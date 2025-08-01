package middleware

import (
	"ahop/internal/models"
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/response"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware 权限中间件
type AuthMiddleware struct {
	userService *services.UserService
	jwtManager  *jwt.JWTManager
}

func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{
		userService: services.NewUserService(),
		jwtManager:  jwt.GetJWTManager(), // 使用全局JWT管理器
	}
}

// RequireLogin 更新RequireLogin方法
func (m *AuthMiddleware) RequireLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从Authorization头获取JWT token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}

		// 检查Bearer格式
		if !strings.HasPrefix(authHeader, "Bearer ") {
			response.Unauthorized(c, "认证头格式错误")
			c.Abort()
			return
		}

		// 提取token
		tokenString := authHeader[7:] // 去掉 "Bearer "

		// 验证token
		claims, err := m.jwtManager.VerifyToken(tokenString)
		if err != nil {
			response.Unauthorized(c, "Token无效或已过期")
			c.Abort()
			return
		}

		// 获取用户信息
		user, err := m.userService.GetByID(claims.UserID)
		if err != nil {
			response.Unauthorized(c, "用户不存在")
			c.Abort()
			return
		}

		// 检查用户状态
		if !m.userService.IsActive(user) {
			response.Unauthorized(c, "用户已被禁用")
			c.Abort()
			return
		}

		// 将用户信息保存到上下文
		c.Set("user", user)
		c.Set("user_id", claims.UserID)
		c.Set("tenant_id", claims.TenantID)
		c.Set("current_tenant_id", claims.CurrentTenantID) // 添加当前操作的租户ID
		c.Set("username", claims.Username)
		c.Set("is_platform_admin", claims.IsPlatformAdmin)
		c.Set("is_tenant_admin", claims.IsTenantAdmin)
		c.Set("claims", claims)

		c.Next()
	}
}

// RequirePermission 要求特定权限
func (m *AuthMiddleware) RequirePermission(permissionCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 先检查登录
		userID, exists := c.Get("user_id")
		if !exists {
			response.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}
		
		// 获取当前租户ID
		currentTenantID, exists := c.Get("current_tenant_id")
		if !exists {
			response.ServerError(c, "无法获取当前租户")
			c.Abort()
			return
		}
		
		// 平台管理员拥有所有权限
		isPlatformAdmin, _ := c.Get("is_platform_admin")
		if isPlatformAdmin.(bool) {
			c.Next()
			return
		}

		// 检查用户在当前租户的权限
		hasPermission, err := m.userService.HasPermissionInTenant(userID.(uint), currentTenantID.(uint), permissionCode)
		if err != nil {
			response.ServerError(c, "权限检查失败")
			c.Abort()
			return
		}

		if !hasPermission {
			response.Forbidden(c, "权限不足：需要 "+permissionCode+" 权限")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireRole 要求特定角色
func (m *AuthMiddleware) RequireRole(roleCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 先检查登录
		userID, exists := c.Get("user_id")
		if !exists {
			response.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}

		// 检查角色
		hasRole, err := m.userService.HasRole(userID.(uint), roleCode)
		if err != nil {
			response.ServerError(c, "角色检查失败")
			c.Abort()
			return
		}

		if !hasRole {
			response.Forbidden(c, "权限不足：需要 "+roleCode+" 角色")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequirePlatformAdmin 要求平台管理员
func (m *AuthMiddleware) RequirePlatformAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			response.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}

		if !user.(*models.User).IsPlatformAdmin {
			response.Forbidden(c, "需要平台管理员权限")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireTenantAdmin 要求租户管理员
func (m *AuthMiddleware) RequireTenantAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			response.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}

		userObj := user.(*models.User)
		
		// 平台管理员拥有所有租户的管理权限
		if userObj.IsPlatformAdmin {
			c.Next()
			return
		}
		
		// 获取当前租户ID
		currentTenantID, exists := c.Get("current_tenant_id")
		if !exists {
			response.ServerError(c, "无法获取当前租户")
			c.Abort()
			return
		}
		
		// 检查用户是否是当前租户的管理员
		if !userObj.IsAdminOfTenant(m.userService.GetDB(), currentTenantID.(uint)) {
			response.Forbidden(c, "需要租户管理员权限")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireSameTenant 要求同租户（用于租户数据隔离）
func (m *AuthMiddleware) RequireSameTenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			response.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}

		userObj := user.(*models.User)

		// 平台管理员可以访问所有租户数据
		if userObj.IsPlatformAdmin {
			c.Next()
			return
		}

		// 获取当前操作的租户ID（支持平台管理员切换租户）
		currentTenantID, exists := c.Get("current_tenant_id")
		if !exists {
			response.ServerError(c, "无法获取当前租户ID")
			c.Abort()
			return
		}

		// 从URL参数或查询参数中获取租户ID
		targetTenantIDStr := c.Param("tenant_id")
		if targetTenantIDStr == "" {
			targetTenantIDStr = c.Query("tenant_id")
		}

		if targetTenantIDStr != "" {
			targetTenantID, err := strconv.ParseUint(targetTenantIDStr, 10, 32)
			if err != nil {
				response.BadRequest(c, "租户ID格式错误")
				c.Abort()
				return
			}

			if currentTenantID.(uint) != uint(targetTenantID) {
				response.Forbidden(c, "无权访问其他租户的数据")
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// RequireOwnerOrAdmin 要求是资源所有者或管理员
func (m *AuthMiddleware) RequireOwnerOrAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			response.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}

		userObj := user.(*models.User)

		// 🔧 修复：平台管理员可以访问所有资源
		if userObj.IsPlatformAdmin {
			c.Next()
			return
		}

		// 🔧 修复：检查用户是否是当前租户的管理员
		currentTenantID, exists := c.Get("current_tenant_id")
		if !exists {
			response.ServerError(c, "无法获取当前租户ID")
			c.Abort()
			return
		}
		
		if userObj.IsAdminOfTenant(m.userService.GetDB(), currentTenantID.(uint)) {
			
			// 检查是否是同租户的资源（通过查询目标用户的租户ID）
			resourceUserIDStr := c.Param("id")
			if resourceUserIDStr != "" {
				resourceUserID, err := strconv.ParseUint(resourceUserIDStr, 10, 32)
				if err != nil {
					response.BadRequest(c, "用户ID格式错误")
					c.Abort()
					return
				}

				// 查询目标用户的租户ID
				targetUser, err := m.userService.GetByID(uint(resourceUserID))
				if err != nil {
					response.NotFound(c, "用户不存在")
					c.Abort()
					return
				}

				// 租户管理员只能管理同租户的用户
				// 检查目标用户是否属于当前租户
				if targetUser.IsTenantMember(m.userService.GetDB(), currentTenantID.(uint)) {
					c.Next()
					return
				}
			}
		}

		// 检查是否是资源所有者
		resourceUserIDStr := c.Param("id")
		if resourceUserIDStr != "" {
			resourceUserID, err := strconv.ParseUint(resourceUserIDStr, 10, 32)
			if err != nil {
				response.BadRequest(c, "用户ID格式错误")
				c.Abort()
				return
			}

			if userObj.ID == uint(resourceUserID) {
				c.Next()
				return
			}
		}

		// 既不是管理员，也不是资源所有者
		response.Forbidden(c, "只能操作自己的资源")
		c.Abort()
	}
}

// CombineMiddleware 组合中间件（登录 + 权限）
func (m *AuthMiddleware) CombineMiddleware(permissionCode string) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		m.RequireLogin(),
		m.RequirePermission(permissionCode),
	}
}

// CombineRoleMiddleware 组合中间件（登录 + 角色）
func (m *AuthMiddleware) CombineRoleMiddleware(roleCode string) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		m.RequireLogin(),
		m.RequireRole(roleCode),
	}
}
