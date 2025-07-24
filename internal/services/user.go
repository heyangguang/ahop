package services

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"ahop/internal/database"
	"ahop/internal/models"

	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

// UserStats 用户统计信息
type UserStats struct {
	Total          int64 `json:"total"`
	Active         int64 `json:"active"`
	Inactive       int64 `json:"inactive"`
	Locked         int64 `json:"locked"`
	PlatformAdmins int64 `json:"platform_admins"`
	TenantAdmins   int64 `json:"tenant_admins"`
}

// UserStatusCount 用户状态分布统计
type UserStatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

func NewUserService() *UserService {
	return &UserService{
		db: database.GetDB(),
	}
}

// ========== 基础CRUD方法 ==========

// Create 创建用户
func (s *UserService) Create(tenantID uint, username, email, password, name string, phone *string) (*models.User, error) {
	// 验证参数
	if err := s.ValidateCreateParams(username, email, password, name); err != nil {
		return nil, err
	}

	// 检查租户是否存在
	var tenantCount int64
	s.db.Model(&models.Tenant{}).Where("id = ?", tenantID).Count(&tenantCount)
	if tenantCount == 0 {
		return nil, fmt.Errorf("租户不存在")
	}

	// 检查用户名是否重复
	var usernameCount int64
	s.db.Model(&models.User{}).Where("username = ?", username).Count(&usernameCount)
	if usernameCount > 0 {
		return nil, fmt.Errorf("用户名已存在")
	}

	// 检查邮箱是否重复
	var emailCount int64
	s.db.Model(&models.User{}).Where("email = ?", email).Count(&emailCount)
	if emailCount > 0 {
		return nil, fmt.Errorf("邮箱已存在")
	}

	user := &models.User{
		TenantID:        tenantID,
		Username:        username,
		Email:           email,
		Name:            name,
		Phone:           phone,
		Status:          models.UserStatusActive,
		IsPlatformAdmin: false,
		IsTenantAdmin:   false,
	}

	// 设置密码
	if err := user.SetPassword(password); err != nil {
		return nil, fmt.Errorf("密码加密失败: %v", err)
	}

	err := s.db.Create(user).Error
	return user, err
}

// CreateWithOptions 创建用户（支持设置租户管理员）
func (s *UserService) CreateWithOptions(tenantID uint, username, email, password, name string, phone *string, isTenantAdmin bool) (*models.User, error) {
	// 验证参数
	if err := s.ValidateCreateParams(username, email, password, name); err != nil {
		return nil, err
	}

	// 检查租户是否存在
	var tenantCount int64
	s.db.Model(&models.Tenant{}).Where("id = ?", tenantID).Count(&tenantCount)
	if tenantCount == 0 {
		return nil, fmt.Errorf("租户不存在")
	}

	// 检查用户名是否重复
	var usernameCount int64
	s.db.Model(&models.User{}).Where("username = ?", username).Count(&usernameCount)
	if usernameCount > 0 {
		return nil, fmt.Errorf("用户名已存在")
	}

	// 检查邮箱是否重复
	var emailCount int64
	s.db.Model(&models.User{}).Where("email = ?", email).Count(&emailCount)
	if emailCount > 0 {
		return nil, fmt.Errorf("邮箱已存在")
	}

	user := &models.User{
		TenantID:        tenantID,
		Username:        username,
		Email:           email,
		Name:            name,
		Phone:           phone,
		Status:          models.UserStatusActive,
		IsPlatformAdmin: false,
		IsTenantAdmin:   isTenantAdmin,
	}

	// 设置密码
	if err := user.SetPassword(password); err != nil {
		return nil, err
	}

	// 创建用户
	if err := s.db.Create(user).Error; err != nil {
		return nil, err
	}

	// 重新加载数据（包含关联）
	if err := s.db.Preload("Tenant").First(user, user.ID).Error; err != nil {
		return nil, err
	}

	return user, nil
}

// GetByID 根据ID获取用户
func (s *UserService) GetByID(id uint) (*models.User, error) {
	var user models.User
	err := s.db.Preload("Tenant").First(&user, id).Error
	return &user, err
}

// GetWithFiltersAndPage 组合查询（分页版本）
func (s *UserService) GetWithFiltersAndPage(tenantID *uint, status, keyword string, page, pageSize int) ([]*models.User, int64, error) {
	var users []*models.User
	var total int64

	query := s.db.Model(&models.User{})

	// 添加过滤条件
	if tenantID != nil {
		query = query.Where("tenant_id = ?", *tenantID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		searchPattern := fmt.Sprintf("%%%s%%", keyword)
		query = query.Where("username LIKE ? OR email LIKE ? OR name LIKE ?",
			searchPattern, searchPattern, searchPattern)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Preload("Tenant").Offset(offset).Limit(pageSize).Find(&users).Error
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// GetRecentlyCreatedWithPage 最近创建（分页版本）
func (s *UserService) GetRecentlyCreatedWithPage(page, pageSize int) ([]*models.User, int64, error) {
	var users []*models.User
	var total int64

	// 计算总数
	if err := s.db.Model(&models.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询（按创建时间降序）
	offset := (page - 1) * pageSize
	err := s.db.Preload("Tenant").Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&users).Error
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// Update 更新用户
func (s *UserService) Update(id uint, name, email string, phone *string, status string) (*models.User, error) {
	// 验证参数
	if err := s.ValidateUpdateParams(name, email, status); err != nil {
		return nil, err
	}

	var user models.User
	err := s.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}

	// 如果邮箱变更，检查是否重复
	if user.Email != email {
		var emailCount int64
		s.db.Model(&models.User{}).Where("email = ? AND id != ?", email, id).Count(&emailCount)
		if emailCount > 0 {
			return nil, fmt.Errorf("邮箱已存在")
		}
	}

	user.Name = name
	user.Email = email
	user.Phone = phone
	user.Status = status

	err = s.db.Save(&user).Error
	return &user, err
}

// Delete 删除用户
func (s *UserService) Delete(id uint) error {
	return s.db.Delete(&models.User{}, id).Error
}

// ========== 快捷操作方法 ==========

// Activate 激活用户
func (s *UserService) Activate(id uint) (*models.User, error) {
	var user models.User
	err := s.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}

	s.SetActiveStatus(&user)
	err = s.db.Save(&user).Error
	return &user, err
}

// Deactivate 停用用户
func (s *UserService) Deactivate(id uint) (*models.User, error) {
	var user models.User
	err := s.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}

	s.SetInactiveStatus(&user)
	err = s.db.Save(&user).Error
	return &user, err
}

// Lock 锁定用户
func (s *UserService) Lock(id uint) (*models.User, error) {
	var user models.User
	err := s.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}

	s.SetLockedStatus(&user)
	err = s.db.Save(&user).Error
	return &user, err
}

// ResetPassword 重置密码
func (s *UserService) ResetPassword(id uint, newPassword string) (*models.User, error) {
	if err := s.ValidatePassword(newPassword); err != nil {
		return nil, err
	}

	var user models.User
	err := s.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}

	if err := user.SetPassword(newPassword); err != nil {
		return nil, fmt.Errorf("密码加密失败: %v", err)
	}

	err = s.db.Save(&user).Error
	return &user, err
}

// UpdateLastLogin 更新最后登录时间
func (s *UserService) UpdateLastLogin(id uint) error {
	now := time.Now()
	return s.db.Model(&models.User{}).Where("id = ?", id).Update("last_login_at", now).Error
}

// ========== 查询增强方法 ==========

// GetByUsername 根据用户名获取用户
func (s *UserService) GetByUsername(username string) (*models.User, error) {
	var user models.User
	err := s.db.Preload("Tenant").Where("username = ?", username).First(&user).Error
	return &user, err
}

// GetByEmail 根据邮箱获取用户
func (s *UserService) GetByEmail(email string) (*models.User, error) {
	var user models.User
	err := s.db.Preload("Tenant").Where("email = ?", email).First(&user).Error
	return &user, err
}

// ========== 统计相关方法 ==========

// GetStats 获取用户统计
func (s *UserService) GetStats() (*UserStats, error) {
	stats := &UserStats{}

	// 总数
	s.db.Model(&models.User{}).Count(&stats.Total)

	// 各状态数量
	s.db.Model(&models.User{}).Where("status = ?", models.UserStatusActive).Count(&stats.Active)
	s.db.Model(&models.User{}).Where("status = ?", models.UserStatusInactive).Count(&stats.Inactive)
	s.db.Model(&models.User{}).Where("status = ?", models.UserStatusLocked).Count(&stats.Locked)

	// 管理员数量
	s.db.Model(&models.User{}).Where("is_platform_admin = ?", true).Count(&stats.PlatformAdmins)
	s.db.Model(&models.User{}).Where("is_tenant_admin = ?", true).Count(&stats.TenantAdmins)

	return stats, nil
}

// GetStatusDistribution 获取状态分布统计
func (s *UserService) GetStatusDistribution() ([]*UserStatusCount, error) {
	var results []*UserStatusCount
	err := s.db.Model(&models.User{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Find(&results).Error
	return results, err
}

// ========== 业务逻辑方法 ==========

// SetActiveStatus 设置用户状态为激活
func (s *UserService) SetActiveStatus(user *models.User) {
	user.Status = models.UserStatusActive
}

// SetInactiveStatus 设置用户状态为停用
func (s *UserService) SetInactiveStatus(user *models.User) {
	user.Status = models.UserStatusInactive
}

// SetLockedStatus 设置用户状态为锁定
func (s *UserService) SetLockedStatus(user *models.User) {
	user.Status = models.UserStatusLocked
}

// IsActive 检查用户是否激活
func (s *UserService) IsActive(user *models.User) bool {
	return user.Status == models.UserStatusActive
}

// IsValidStatus 检查用户状态是否有效
func (s *UserService) IsValidStatus(status string) bool {
	switch status {
	case models.UserStatusActive, models.UserStatusInactive, models.UserStatusLocked:
		return true
	default:
		return false
	}
}

// ========== 验证相关方法 ==========

// ValidateUsername 验证用户名
func (s *UserService) ValidateUsername(username string) bool {
	if len(username) < 3 || len(username) > 50 {
		return false
	}
	// 检查是否只包含字母、数字和下划线
	for _, r := range username {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// ValidateEmail 验证邮箱
func (s *UserService) ValidateEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".") && len(email) >= 5 && len(email) <= 100
}

// ValidatePassword 验证密码
func (s *UserService) ValidatePassword(password string) error {
	if len(password) < 6 {
		return fmt.Errorf("密码长度不能少于6位")
	}
	if len(password) > 50 {
		return fmt.Errorf("密码长度不能超过50位")
	}
	return nil
}

// ValidateName 验证姓名
func (s *UserService) ValidateName(name string) bool {
	// 🚨 关键修复：使用 utf8.RuneCountInString 正确计算中文字符数
	runeCount := utf8.RuneCountInString(name)
	return runeCount >= 2 && runeCount <= 50
}

// ValidateCreateParams 验证创建用户的参数
func (s *UserService) ValidateCreateParams(username, email, password, name string) error {
	if !s.ValidateUsername(username) {
		return fmt.Errorf("用户名长度必须在3-50个字符之间，且只能包含字母、数字和下划线")
	}
	if !s.ValidateEmail(email) {
		return fmt.Errorf("邮箱格式不正确")
	}
	if err := s.ValidatePassword(password); err != nil {
		return err
	}
	if !s.ValidateName(name) {
		return fmt.Errorf("姓名长度必须在2-50个字符之间")
	}
	return nil
}

// ValidateUpdateParams 验证更新用户的参数
func (s *UserService) ValidateUpdateParams(name, email, status string) error {
	if !s.ValidateName(name) {
		return fmt.Errorf("姓名长度必须在2-50个字符之间")
	}
	if !s.ValidateEmail(email) {
		return fmt.Errorf("邮箱格式不正确")
	}
	if !s.IsValidStatus(status) {
		return fmt.Errorf("状态只能是active、inactive或locked")
	}
	return nil
}

// ========== 角色管理方法 ==========

// AssignRoles 为用户分配角色
func (s *UserService) AssignRoles(userID uint, roleIDs []uint) error {
	var user models.User
	err := s.db.First(&user, userID).Error
	if err != nil {
		return err
	}

	// 获取角色（确保角色存在且属于同一租户）
	var roles []models.Role
	err = s.db.Where("id IN ? AND tenant_id = ?", roleIDs, user.TenantID).Find(&roles).Error
	if err != nil {
		return err
	}

	// 验证所有角色都找到了
	if len(roles) != len(roleIDs) {
		return fmt.Errorf("部分角色不存在或不属于该用户的租户")
	}

	// 清除现有角色，重新分配
	err = s.db.Model(&user).Association("Roles").Replace(roles)
	return err
}

// AddRole 为用户添加单个角色
func (s *UserService) AddRole(userID, roleID uint) error {
	var user models.User
	err := s.db.First(&user, userID).Error
	if err != nil {
		return err
	}

	var role models.Role
	err = s.db.Where("id = ? AND tenant_id = ?", roleID, user.TenantID).First(&role).Error
	if err != nil {
		return fmt.Errorf("角色不存在或不属于该用户的租户")
	}

	// 检查用户是否已有该角色
	var count int64
	s.db.Table("user_roles").Where("user_id = ? AND role_id = ?", userID, roleID).Count(&count)
	if count > 0 {
		return fmt.Errorf("用户已拥有该角色")
	}

	err = s.db.Model(&user).Association("Roles").Append(&role)
	return err
}

// RemoveRole 移除用户的角色
func (s *UserService) RemoveRole(userID, roleID uint) error {
	var user models.User
	err := s.db.First(&user, userID).Error
	if err != nil {
		return err
	}

	var role models.Role
	err = s.db.First(&role, roleID).Error
	if err != nil {
		return err
	}

	err = s.db.Model(&user).Association("Roles").Delete(&role)
	return err
}

// GetUserRoles 获取用户的角色列表
func (s *UserService) GetUserRoles(userID uint) ([]models.Role, error) {
	var user models.User
	err := s.db.Preload("Roles.Permissions").First(&user, userID).Error
	if err != nil {
		return nil, err
	}
	return user.Roles, nil
}

// GetUserPermissions 获取用户的所有权限
func (s *UserService) GetUserPermissions(userID uint) ([]models.Permission, error) {
	var user models.User
	err := s.db.Preload("Roles.Permissions").First(&user, userID).Error
	if err != nil {
		return nil, err
	}

	// 收集所有权限（去重）
	permissionMap := make(map[string]models.Permission)

	// 平台管理员拥有所有权限
	if user.IsPlatformAdmin {
		var allPermissions []models.Permission
		s.db.Find(&allPermissions)
		return allPermissions, nil
	}

	// 租户管理员拥有本租户内的管理权限
	if user.IsTenantAdmin {
		var allPermissions []models.Permission
		s.db.Find(&allPermissions)
		
		// 过滤掉平台级权限（tenant:*）
		filteredPermissions := make([]models.Permission, 0)
		for _, permission := range allPermissions {
			if !strings.HasPrefix(permission.Code, "tenant:") {
				filteredPermissions = append(filteredPermissions, permission)
			}
		}
		
		// 合并角色权限（如果有的话）
		for _, role := range user.Roles {
			if role.Status == models.RoleStatusActive {
				for _, permission := range role.Permissions {
					permissionMap[permission.Code] = permission
				}
			}
		}
		
		// 将过滤后的权限也加入到map中（去重）
		for _, permission := range filteredPermissions {
			permissionMap[permission.Code] = permission
		}
		
		// 转换为切片
		permissions := make([]models.Permission, 0, len(permissionMap))
		for _, permission := range permissionMap {
			permissions = append(permissions, permission)
		}
		
		return permissions, nil
	}

	// 普通用户：收集角色权限
	for _, role := range user.Roles {
		if role.Status == models.RoleStatusActive {
			for _, permission := range role.Permissions {
				permissionMap[permission.Code] = permission
			}
		}
	}

	// 转换为切片
	permissions := make([]models.Permission, 0, len(permissionMap))
	for _, permission := range permissionMap {
		permissions = append(permissions, permission)
	}

	return permissions, nil
}

// HasPermission 检查用户是否有特定权限
func (s *UserService) HasPermission(userID uint, permissionCode string) (bool, error) {
	var user models.User
	err := s.db.Preload("Roles.Permissions").First(&user, userID).Error
	if err != nil {
		return false, err
	}

	// 平台管理员拥有所有权限
	if user.IsPlatformAdmin {
		return true, nil
	}

	// 租户管理员拥有本租户内的管理权限
	if user.IsTenantAdmin {
		// 租户管理员不应该拥有平台级权限（如租户管理）
		if strings.HasPrefix(permissionCode, "tenant:") {
			// 租户管理权限仅限平台管理员
			return false, nil
		}
		// 租户管理员拥有其他所有租户级资源的管理权限
		return true, nil
	}

	// 检查角色权限
	for _, role := range user.Roles {
		if role.Status == models.RoleStatusActive {
			for _, permission := range role.Permissions {
				if permission.Code == permissionCode {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// HasRole 检查用户是否有特定角色
func (s *UserService) HasRole(userID uint, roleCode string) (bool, error) {
	var user models.User
	err := s.db.Preload("Roles").First(&user, userID).Error
	if err != nil {
		return false, err
	}

	for _, role := range user.Roles {
		if role.Code == roleCode && role.Status == models.RoleStatusActive {
			return true, nil
		}
	}

	return false, nil
}
