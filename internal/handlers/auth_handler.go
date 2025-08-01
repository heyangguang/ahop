package handlers

import (
	"ahop/internal/models"
	"strings"
	"time"

	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/response"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	userService   *services.UserService
	tenantService *services.TenantService
	jwtManager    *jwt.JWTManager
}

func NewAuthHandler(userService *services.UserService, tenantService *services.TenantService) *AuthHandler {
	return &AuthHandler{
		userService:   userService,
		tenantService: tenantService,
		jwtManager:    jwt.GetJWTManager(), // 使用全局JWT管理器
	}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token     string   `json:"token"`
	ExpiresAt int64    `json:"expires_at"`
	User      UserInfo `json:"user"`
}

type UserInfo struct {
	ID              uint   `json:"id"`
	Username        string `json:"username"`
	Email           string `json:"email"`
	Name            string `json:"name"`
	TenantID        uint   `json:"tenant_id"`
	IsPlatformAdmin bool   `json:"is_platform_admin"`
	IsTenantAdmin   bool   `json:"is_tenant_admin"`
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 根据用户名获取用户
	user, err := h.userService.GetByUsername(req.Username)
	if err != nil {
		response.Unauthorized(c, "用户名或密码错误")
		return
	}

	// 检查用户状态
	if !h.userService.IsActive(user) {
		response.Unauthorized(c, "用户已被禁用")
		return
	}

	// 验证密码
	if !user.CheckPassword(req.Password) {
		response.Unauthorized(c, "用户名或密码错误")
		return
	}

	// 获取用户的第一个租户作为默认租户
	userTenants, err := user.GetUserTenants(h.userService.GetDB())
	if err != nil || len(userTenants) == 0 {
		response.Unauthorized(c, "用户未关联任何租户")
		return
	}
	
	// 使用第一个租户作为默认登录租户
	defaultTenant := userTenants[0]
	
	// 生成Token
	token, err := h.jwtManager.GenerateTokenWithTenant(
		user.ID,
		defaultTenant.TenantID,  // 原始租户ID
		defaultTenant.TenantID,  // 当前操作的租户ID
		user.Username,
		user.IsPlatformAdmin,
		defaultTenant.IsTenantAdmin,  // 使用该租户的管理员状态
	)
	if err != nil {
		response.ServerError(c, "生成Token失败")
		return
	}

	// 更新最后登录时间
	if err := h.userService.UpdateLastLogin(user.ID); err != nil {
		// 记录日志但不影响登录流程
	}

	// 计算过期时间
	expiresAt := time.Now().Add(h.jwtManager.GetTokenDuration()).Unix()

	resp := LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User: UserInfo{
			ID:              user.ID,
			Username:        user.Username,
			Email:           user.Email,
			Name:            user.Name,
			TenantID:        defaultTenant.TenantID,
			IsPlatformAdmin: user.IsPlatformAdmin,
			IsTenantAdmin:   defaultTenant.IsTenantAdmin,
		},
	}

	response.Success(c, resp)
}

// Logout 用户登出
func (h *AuthHandler) Logout(c *gin.Context) {
	// 获取并验证当前token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		// 没有token也算登出成功
		response.Success(c, gin.H{
			"message": "登出成功",
		})
		return
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		// token格式错误也算登出成功
		response.Success(c, gin.H{
			"message": "登出成功",
		})
		return
	}

	tokenString := authHeader[7:] // 去掉 "Bearer "

	// 验证token并获取用户信息（用于日志记录）
	claims, err := h.jwtManager.VerifyToken(tokenString)
	if err != nil {
		// token无效也算登出成功
		response.Success(c, gin.H{
			"message": "登出成功",
		})
		return
	}

	// 记录登出日志
	// 在生产环境中，这里应该记录到日志系统：
	// - 用户ID、用户名
	// - 登出时间
	// - IP地址、User-Agent
	// - 租户信息等

	response.Success(c, gin.H{
		"message":     "登出成功",
		"user_id":     claims.UserID,
		"username":    claims.Username,
		"logout_time": time.Now(),
	})

	// 注意：
	// 1. Token会在过期时间后自动失效
	// 2. 前端应该删除本地存储的token
	// 3. 如果需要立即失效，可以考虑更短的token有效期配合refresh token机制
}

// RefreshToken 刷新Token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	// 从请求头获取当前token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c, "缺少认证头")
		return
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		response.Unauthorized(c, "认证头格式错误")
		return
	}

	tokenString := authHeader[7:] // 去掉 "Bearer "

	// 验证当前token（即使过期也要解析用户信息）
	claims, err := h.jwtManager.VerifyToken(tokenString)
	if err != nil {
		response.Unauthorized(c, "Token无效")
		return
	}

	// 获取用户信息
	user, err := h.userService.GetByID(claims.UserID)
	if err != nil {
		response.Unauthorized(c, "用户不存在")
		return
	}

	// 检查用户状态
	if !h.userService.IsActive(user) {
		response.Unauthorized(c, "用户已被禁用")
		return
	}

	// 保持当前租户状态生成新Token
	newToken, err := h.jwtManager.GenerateTokenWithTenant(
		user.ID,
		claims.TenantID,        // 原始租户ID
		claims.CurrentTenantID, // 当前操作的租户ID
		user.Username,
		user.IsPlatformAdmin,
		claims.IsTenantAdmin,   // 保持当前租户的管理员状态
	)
	if err != nil {
		response.ServerError(c, "生成新Token失败")
		return
	}

	// 计算过期时间
	expiresAt := time.Now().Add(h.jwtManager.GetTokenDuration()).Unix()

	response.Success(c, gin.H{
		"token":      newToken,
		"expires_at": expiresAt,
		"message":    "Token刷新成功",
	})
}

// SwitchTenantRequest 切换租户请求
type SwitchTenantRequest struct {
	TenantID uint `json:"tenant_id" binding:"required"`
}

// SwitchTenant 切换租户（仅平台管理员可用）
func (h *AuthHandler) SwitchTenant(c *gin.Context) {
	// 获取当前用户信息
	claims, exists := c.Get("claims")
	if !exists {
		response.Unauthorized(c, "未登录")
		return
	}
	userClaims := claims.(*jwt.JWTClaims)

	// 获取用户信息
	user, err := h.userService.GetByID(userClaims.UserID)
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	var req SwitchTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 平台管理员可以切换到任何租户
	if !user.IsPlatformAdmin {
		// 非平台管理员只能切换到有权限的租户
		if !user.IsTenantMember(h.userService.GetDB(), req.TenantID) {
			response.Forbidden(c, "无权访问该租户")
			return
		}
	}
	
	// 验证目标租户是否存在且激活
	tenant, err := h.tenantService.GetByID(req.TenantID)
	if err != nil {
		response.NotFound(c, "租户不存在")
		return
	}

	if !h.tenantService.IsActive(tenant) {
		response.BadRequest(c, "目标租户未激活")
		return
	}
	
	// 获取用户在目标租户的角色信息
	var isTenantAdmin bool
	if !user.IsPlatformAdmin {
		userTenant, err := user.GetTenantRole(h.userService.GetDB(), req.TenantID)
		if err == nil {
			isTenantAdmin = userTenant.IsTenantAdmin
		}
	}

	// 生成包含新租户ID的token
	newToken, err := h.jwtManager.GenerateTokenWithTenant(
		user.ID,
		userClaims.TenantID, // 原始租户ID（保持不变）
		req.TenantID,        // 当前操作的租户ID
		user.Username,
		user.IsPlatformAdmin,
		isTenantAdmin,       // 使用目标租户的管理员状态
	)
	if err != nil {
		response.ServerError(c, "生成新Token失败")
		return
	}

	// 计算过期时间
	expiresAt := time.Now().Add(h.jwtManager.GetTokenDuration()).Unix()

	response.Success(c, gin.H{
		"token":      newToken,
		"expires_at": expiresAt,
		"current_tenant": gin.H{
			"id":     tenant.ID,
			"name":   tenant.Name,
			"code":   tenant.Code,
			"status": tenant.Status,
		},
		"message": "切换租户成功",
	})
}

// Me 获取当前登录用户的完整信息
func (h *AuthHandler) Me(c *gin.Context) {
	// 获取当前用户信息
	claims, exists := c.Get("claims")
	if !exists {
		response.Unauthorized(c, "未登录")
		return
	}
	userClaims := claims.(*jwt.JWTClaims)

	// 获取用户详细信息
	user, err := h.userService.GetByID(userClaims.UserID)
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	// 获取当前租户信息
	currentTenant, err := h.tenantService.GetByID(userClaims.CurrentTenantID)
	if err != nil {
		response.ServerError(c, "获取租户信息失败")
		return
	}

	// 获取用户角色
	roles, err := h.userService.GetUserRoles(user.ID)
	if err != nil {
		roles = []models.Role{} // 如果获取失败，返回空数组
	}

	// 获取用户权限
	permissions, err := h.userService.GetUserPermissions(user.ID)
	if err != nil {
		permissions = []models.Permission{} // 如果获取失败，返回空数组
	}

	// 构建响应
	responseData := gin.H{
		"user": gin.H{
			"id":                user.ID,
			"username":          user.Username,
			"email":             user.Email,
			"name":              user.Name,
			"phone":             user.Phone,
			"avatar":            user.Avatar,
			"status":            user.Status,
			"is_platform_admin": user.IsPlatformAdmin,
			"created_at":        user.CreatedAt,
			"last_login_at":     user.LastLoginAt,
		},
		"current_tenant": gin.H{
			"id":     currentTenant.ID,
			"name":   currentTenant.Name,
			"code":   currentTenant.Code,
			"status": currentTenant.Status,
		},
		"roles":       h.formatRoles(roles),
		"permissions": h.formatPermissions(permissions),
	}

	// 获取用户的所有租户
	userTenants, err := user.GetUserTenants(h.userService.GetDB())
	if err == nil {
		var switchableTenants []gin.H
		
		// 如果是平台管理员，显示所有活跃租户
		if user.IsPlatformAdmin {
			tenants, err := h.tenantService.GetAllActive()
			if err == nil {
				for _, tenant := range tenants {
					switchableTenants = append(switchableTenants, gin.H{
						"id":               tenant.ID,
						"name":             tenant.Name,
						"code":             tenant.Code,
						"is_current":       tenant.ID == userClaims.CurrentTenantID,
						"is_tenant_admin":  false,  // 平台管理员在所有租户都有管理权限
						"user_count":       tenant.UserCount,
					})
				}
			}
		} else {
			// 普通用户只显示有权限的租户
			for _, ut := range userTenants {
				switchableTenants = append(switchableTenants, gin.H{
					"id":               ut.Tenant.ID,
					"name":             ut.Tenant.Name,
					"code":             ut.Tenant.Code,
					"is_current":       ut.Tenant.ID == userClaims.CurrentTenantID,
					"is_tenant_admin":  ut.IsTenantAdmin,
					"role_name":        ut.Role.Name,
					"joined_at":        ut.JoinedAt,
				})
			}
		}
		
		responseData["switchable_tenants"] = switchableTenants
	}

	response.Success(c, responseData)
}

// 格式化角色列表
func (h *AuthHandler) formatRoles(roles []models.Role) []gin.H {
	var result []gin.H
	for _, role := range roles {
		result = append(result, gin.H{
			"id":          role.ID,
			"name":        role.Name,
			"code":        role.Code,
			"description": role.Description,
		})
	}
	return result
}

// 格式化权限列表
func (h *AuthHandler) formatPermissions(permissions []models.Permission) []string {
	var result []string
	for _, perm := range permissions {
		result = append(result, perm.Code)
	}
	return result
}
