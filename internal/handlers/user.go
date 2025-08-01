package handlers

import (
	"ahop/internal/models"
	"ahop/pkg/pagination"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ahop/internal/services"
	"ahop/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CreateUserRequest struct {
	TenantID      uint    `json:"tenant_id"`
	Username      string  `json:"username"`
	Email         string  `json:"email"`
	Password      string  `json:"password"`
	Name          string  `json:"name"`
	Phone         *string `json:"phone"`
	IsTenantAdmin bool    `json:"is_tenant_admin"`
}

type UpdateUserRequest struct {
	Name   string  `json:"name"`
	Email  string  `json:"email"`
	Phone  *string `json:"phone"`
	Status string  `json:"status"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

type AssignRolesRequest struct {
	RoleIDs []uint `json:"role_ids"`
}

type AddRoleRequest struct {
	RoleID uint `json:"role_id"`
}

type UserHandler struct {
	service *services.UserService
}

func NewUserHandler() *UserHandler {
	return &UserHandler{
		service: services.NewUserService(),
	}
}

// ========== 基础CRUD方法 ==========

// Create 创建用户
func (h *UserHandler) Create(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	user, err := h.service.CreateWithOptions(req.TenantID, req.Username, req.Email, req.Password, req.Name, req.Phone, req.IsTenantAdmin)
	if err != nil {
		errMsg := err.Error()

		// 🚨 统一处理：所有参数验证错误 -> 400
		if strings.Contains(errMsg, "用户名长度") ||
			strings.Contains(errMsg, "邮箱格式") ||
			strings.Contains(errMsg, "密码长度") ||
			strings.Contains(errMsg, "姓名长度") {
			response.BadRequest(c, errMsg)
			return
		}

		// 🚨 统一处理：所有业务逻辑错误 -> 400
		if errMsg == "用户名已存在" ||
			errMsg == "邮箱已存在" ||
			errMsg == "租户不存在" {
			response.BadRequest(c, errMsg)
			return
		}

		// 系统错误 -> 500
		response.ServerError(c, "创建失败")
		return
	}

	response.Success(c, user)
}

// GetByID 获取用户
func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	user, err := h.service.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "用户不存在") // 404
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, user)
}

// GetAll 获取所有用户
func (h *UserHandler) GetAll(c *gin.Context) {
	// 解析分页参数
	pageParams := pagination.ParsePageParams(c)

	// 支持按状态筛选、关键词搜索和租户筛选
	status := c.Query("status")
	keyword := c.Query("keyword")
	tenantIDStr := c.Query("tenant_id")

	var users []*models.User
	var total int64
	var err error

	// 解析租户ID
	var tenantID *uint
	if tenantIDStr != "" {
		if id, parseErr := strconv.ParseUint(tenantIDStr, 10, 32); parseErr == nil {
			tenantIDVal := uint(id)
			tenantID = &tenantIDVal
		} else {
			response.BadRequest(c, "租户ID格式错误")
			return
		}
	}

	// 使用组合查询（最灵活的方案）
	users, total, err = h.service.GetWithFiltersAndPage(tenantID, status, keyword, pageParams.Page, pageParams.PageSize)

	if err != nil {
		response.ServerError(c, "查询失败")
		return
	}

	// 计算分页信息
	pageInfo := pagination.NewPageInfo(pageParams.Page, pageParams.PageSize, total)
	response.SuccessWithPage(c, users, pageInfo)
}

// Update 更新用户
func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	user, err := h.service.Update(uint(id), req.Name, req.Email, req.Phone, req.Status)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "用户不存在") // 404
			return
		}

		errMsg := err.Error()
		// 参数验证错误和业务逻辑错误都返回400
		if strings.Contains(errMsg, "姓名长度") ||
			strings.Contains(errMsg, "邮箱格式") ||
			strings.Contains(errMsg, "状态只能") ||
			errMsg == "邮箱已存在" {
			response.BadRequest(c, errMsg)
			return
		}

		response.ServerError(c, "更新失败")
		return
	}

	response.Success(c, user)
}

// Delete 删除用户
func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	if err := h.service.Delete(uint(id)); err != nil {
		response.ServerError(c, "删除失败")
		return
	}

	response.Success(c, nil)
}

// ========== 快捷操作方法 ==========

// Activate 激活用户
func (h *UserHandler) Activate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	user, err := h.service.Activate(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "用户不存在") // 404
			return
		}
		response.ServerError(c, "激活失败")
		return
	}

	response.SuccessWithMessage(c, "用户激活成功", user)
}

// Deactivate 停用用户
func (h *UserHandler) Deactivate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	user, err := h.service.Deactivate(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "用户不存在") // 404
			return
		}
		response.ServerError(c, "停用失败")
		return
	}

	response.SuccessWithMessage(c, "用户停用成功", user)
}

// Lock 锁定用户
func (h *UserHandler) Lock(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	user, err := h.service.Lock(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "用户不存在") // 404
			return
		}
		response.ServerError(c, "锁定失败")
		return
	}

	response.SuccessWithMessage(c, "用户锁定成功", user)
}

// ResetPassword 重置密码
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	user, err := h.service.ResetPassword(uint(id), req.NewPassword)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "用户不存在") // 404
			return
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "密码长度") {
			response.BadRequest(c, errMsg) // 400
			return
		}

		response.ServerError(c, "重置密码失败")
		return
	}

	response.SuccessWithMessage(c, "密码重置成功", user)
}

// ========== 查询方法 ==========

// GetByUsername 根据用户名获取用户
func (h *UserHandler) GetByUsername(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		response.BadRequest(c, "用户名不能为空")
		return
	}

	user, err := h.service.GetByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "用户不存在") // 404
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, user)
}

// GetByEmail 根据邮箱获取用户
func (h *UserHandler) GetByEmail(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		response.BadRequest(c, "邮箱不能为空")
		return
	}

	user, err := h.service.GetByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "用户不存在") // 404
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, user)
}

// ========== 统计相关方法 ==========

// GetStats 获取用户统计
func (h *UserHandler) GetStats(c *gin.Context) {
	stats, err := h.service.GetStats()
	if err != nil {
		response.ServerError(c, "获取统计失败")
		return
	}

	response.Success(c, stats)
}

// GetRecentlyCreated 方法已经支持limit，可以改为分页版本
func (h *UserHandler) GetRecentlyCreated(c *gin.Context) {
	// 解析分页参数
	pageParams := pagination.ParsePageParams(c)

	users, total, err := h.service.GetRecentlyCreatedWithPage(pageParams.Page, pageParams.PageSize)
	if err != nil {
		response.ServerError(c, "查询失败")
		return
	}

	// 计算分页信息
	pageInfo := pagination.NewPageInfo(pageParams.Page, pageParams.PageSize, total)
	response.SuccessWithPage(c, users, pageInfo)
}

// GetStatusDistribution 获取状态分布
func (h *UserHandler) GetStatusDistribution(c *gin.Context) {
	distribution, err := h.service.GetStatusDistribution()
	if err != nil {
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, distribution)
}

// ========== 用户角色管理方法 ==========

// AssignRoles 为用户分配角色
func (h *UserHandler) AssignRoles(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	var req AssignRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 获取当前租户ID
	currentTenantID, _ := c.Get("current_tenant_id")
	
	err = h.service.AssignRoles(uint(id), req.RoleIDs, currentTenantID.(uint))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "用户不存在")
			return
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "不存在") || strings.Contains(errMsg, "不属于") {
			response.BadRequest(c, errMsg)
			return
		}

		response.ServerError(c, "角色分配失败")
		return
	}

	response.Success(c, "角色分配成功")
}

// AddRole 为用户添加角色
func (h *UserHandler) AddRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	var req AddRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 获取当前租户ID
	currentTenantID, _ := c.Get("current_tenant_id")
	
	err = h.service.AddRole(uint(id), req.RoleID, currentTenantID.(uint))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "用户不存在")
			return
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "不存在") || strings.Contains(errMsg, "不属于") || strings.Contains(errMsg, "已拥有") {
			response.BadRequest(c, errMsg)
			return
		}

		response.ServerError(c, "添加角色失败")
		return
	}

	response.Success(c, "添加角色成功")
}

// RemoveRole 移除用户角色
func (h *UserHandler) RemoveRole(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "用户ID格式错误")
		return
	}

	roleID, err := strconv.ParseUint(c.Param("role_id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "角色ID格式错误")
		return
	}

	err = h.service.RemoveRole(uint(userID), uint(roleID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "用户或角色不存在")
			return
		}
		response.ServerError(c, "移除角色失败")
		return
	}

	response.Success(c, "移除角色成功")
}

// GetUserRoles 获取用户角色
func (h *UserHandler) GetUserRoles(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	roles, err := h.service.GetUserRoles(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "用户不存在")
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, roles)
}

// GetUserPermissions 获取用户权限
func (h *UserHandler) GetUserPermissions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	permissions, err := h.service.GetUserPermissions(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "用户不存在")
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, permissions)
}

// CheckPermission 检查用户权限
func (h *UserHandler) CheckPermission(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	permissionCode := c.Query("permission")
	if permissionCode == "" {
		response.BadRequest(c, "权限代码不能为空")
		return
	}

	hasPermission, err := h.service.HasPermission(uint(id), permissionCode)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "用户不存在")
			return
		}
		response.ServerError(c, "检查失败")
		return
	}

	response.Success(c, map[string]interface{}{
		"user_id":    uint(id),
		"permission": permissionCode,
		"has_access": hasPermission,
	})
}

// CheckRole 检查用户角色
func (h *UserHandler) CheckRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	roleCode := c.Query("role")
	if roleCode == "" {
		response.BadRequest(c, "角色代码不能为空")
		return
	}

	hasRole, err := h.service.HasRole(uint(id), roleCode)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "用户不存在")
			return
		}
		response.ServerError(c, "检查失败")
		return
	}

	response.Success(c, map[string]interface{}{
		"user_id":  uint(id),
		"role":     roleCode,
		"has_role": hasRole,
	})
}

// ========== 个人设置管理方法 ==========

// UpdateProfileRequest 更新个人信息请求
type UpdateProfileRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
	Phone string `json:"phone" binding:"required"`
}

// UpdateProfile 更新个人信息
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	// 从JWT获取当前用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "未登录")
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 更新用户信息（不包含status，普通用户不能修改自己的状态）
	user, err := h.service.GetByID(userID.(uint))
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	// 更新信息
	updatedUser, err := h.service.Update(userID.(uint), req.Name, req.Email, &req.Phone, user.Status)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "邮箱已存在") {
			response.BadRequest(c, errMsg)
			return
		}
		response.ServerError(c, "更新失败")
		return
	}

	response.Success(c, updatedUser)
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ChangePassword 修改密码
func (h *UserHandler) ChangePassword(c *gin.Context) {
	// 从JWT获取当前用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "未登录")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 获取用户信息
	user, err := h.service.GetByID(userID.(uint))
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	// 验证旧密码
	if !user.CheckPassword(req.OldPassword) {
		response.BadRequest(c, "原密码错误")
		return
	}

	// 修改密码
	_, err = h.service.ResetPassword(userID.(uint), req.NewPassword)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "密码长度") {
			response.BadRequest(c, errMsg)
			return
		}
		response.ServerError(c, "修改密码失败")
		return
	}

	response.SuccessWithMessage(c, "密码修改成功", nil)
}

// UpdateAvatar 更新头像
func (h *UserHandler) UpdateAvatar(c *gin.Context) {
	// 从JWT获取当前用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "未登录")
		return
	}

	// 处理文件上传
	file, err := c.FormFile("avatar")
	if err != nil {
		response.BadRequest(c, "请选择要上传的文件")
		return
	}

	// 验证文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}

	if !allowedExts[ext] {
		response.BadRequest(c, "不支持的文件格式，仅支持 jpg/jpeg/png/gif/webp")
		return
	}

	// 验证文件大小（最大5MB）
	if file.Size > 5*1024*1024 {
		response.BadRequest(c, "文件大小不能超过5MB")
		return
	}

	// 生成文件名
	filename := fmt.Sprintf("avatar_%d_%d%s", userID.(uint), time.Now().Unix(), ext)
	uploadPath := fmt.Sprintf("uploads/avatars/%s", filename)

	// 确保目录存在
	if err := os.MkdirAll("uploads/avatars", 0755); err != nil {
		response.ServerError(c, "创建上传目录失败")
		return
	}

	// 保存文件
	if err := c.SaveUploadedFile(file, uploadPath); err != nil {
		response.ServerError(c, "保存文件失败")
		return
	}

	// 更新用户头像路径
	user, err := h.service.UpdateAvatar(userID.(uint), "/"+uploadPath)
	if err != nil {
		// 如果更新失败，删除已上传的文件
		os.Remove(uploadPath)
		response.ServerError(c, "更新头像失败")
		return
	}

	response.Success(c, gin.H{
		"avatar": user.Avatar,
		"message": "头像更新成功",
	})
}

// GetAvatar 获取当前用户头像
func (h *UserHandler) GetAvatar(c *gin.Context) {
	// 从JWT获取当前用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "未登录")
		return
	}

	// 获取用户信息
	user, err := h.service.GetByID(userID.(uint))
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	response.Success(c, gin.H{
		"user_id": user.ID,
		"username": user.Username,
		"name": user.Name,
		"avatar": user.Avatar,
	})
}

// GetUserAvatar 获取指定用户头像（公开接口，任何登录用户都可以查看其他用户头像）
func (h *UserHandler) GetUserAvatar(c *gin.Context) {
	// 获取用户ID参数
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "用户ID格式错误")
		return
	}

	// 获取用户信息
	user, err := h.service.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "用户不存在")
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, gin.H{
		"user_id": user.ID,
		"username": user.Username,
		"name": user.Name,
		"avatar": user.Avatar,
	})
}
