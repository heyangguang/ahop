package services

import (
	"ahop/internal/database"
	"ahop/internal/models"
	"fmt"
	"unicode/utf8"

	"gorm.io/gorm"
)

type RoleService struct {
	db *gorm.DB
}

func NewRoleService() *RoleService {
	return &RoleService{
		db: database.GetDB(),
	}
}

// ========== 基础CRUD方法 ==========

// Create 创建角色
func (s *RoleService) Create(tenantID uint, code, name, description string) (*models.Role, error) {
	// 验证参数
	if err := s.ValidateCreateParams(code, name); err != nil {
		return nil, err
	}

	// 检查角色代码是否重复（在同一租户内）
	var count int64
	s.db.Model(&models.Role{}).Where("tenant_id = ? AND code = ?", tenantID, code).Count(&count)
	if count > 0 {
		return nil, fmt.Errorf("角色代码已存在")
	}

	role := &models.Role{
		TenantID:    tenantID,
		Code:        code,
		Name:        name,
		Description: description,
		Status:      models.RoleStatusActive,
		IsSystem:    false,
	}

	err := s.db.Create(role).Error
	return role, err
}

// GetByID 根据ID获取角色
func (s *RoleService) GetByID(id uint) (*models.Role, error) {
	var role models.Role
	err := s.db.Preload("Tenant").Preload("Permissions").First(&role, id).Error
	return &role, err
}

// GetByTenant 根据租户获取角色列表
func (s *RoleService) GetByTenant(tenantID uint) ([]*models.Role, error) {
	var roles []*models.Role
	err := s.db.Where("tenant_id = ?", tenantID).Preload("Permissions").Find(&roles).Error
	return roles, err
}

// Update 更新角色
func (s *RoleService) Update(id uint, name, description, status string) (*models.Role, error) {
	// 验证参数
	if err := s.ValidateUpdateParams(name, status); err != nil {
		return nil, err
	}

	var role models.Role
	err := s.db.First(&role, id).Error
	if err != nil {
		return nil, err
	}

	// 系统角色不能修改某些字段
	if role.IsSystem {
		return nil, fmt.Errorf("系统角色不允许修改")
	}

	role.Name = name
	role.Description = description
	role.Status = status

	err = s.db.Save(&role).Error
	return &role, err
}

// Delete 删除角色
func (s *RoleService) Delete(id uint) error {
	var role models.Role
	err := s.db.First(&role, id).Error
	if err != nil {
		return err
	}

	// 系统角色不能删除
	if role.IsSystem {
		return fmt.Errorf("系统角色不允许删除")
	}

	return s.db.Delete(&role).Error
}

// ========== 权限管理方法 ==========

// AssignPermissions 为角色分配权限
func (s *RoleService) AssignPermissions(roleID uint, permissionIDs []uint) error {
	var role models.Role
	err := s.db.First(&role, roleID).Error
	if err != nil {
		return err
	}

	// 获取权限
	var permissions []models.Permission
	err = s.db.Where("id IN ?", permissionIDs).Find(&permissions).Error
	if err != nil {
		return err
	}

	// 清除现有权限，重新分配
	err = s.db.Model(&role).Association("Permissions").Replace(permissions)
	return err
}

// GetRolePermissions 获取角色的权限
func (s *RoleService) GetRolePermissions(roleID uint) ([]models.Permission, error) {
	var role models.Role
	err := s.db.Preload("Permissions").First(&role, roleID).Error
	if err != nil {
		return nil, err
	}
	return role.Permissions, nil
}

// ========== 验证方法 ==========

// ValidateCode 验证角色代码
func (s *RoleService) ValidateCode(code string) bool {
	if len(code) < 2 || len(code) > 50 {
		return false
	}
	// 只允许字母、数字和下划线
	for _, r := range code {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// ValidateName 验证角色名称
func (s *RoleService) ValidateName(name string) bool {
	runeCount := utf8.RuneCountInString(name)
	return runeCount >= 2 && runeCount <= 50
}

// ValidateStatus 验证角色状态
func (s *RoleService) ValidateStatus(status string) bool {
	return status == models.RoleStatusActive || status == models.RoleStatusInactive
}

// ValidateCreateParams 验证创建角色的参数
func (s *RoleService) ValidateCreateParams(code, name string) error {
	if !s.ValidateCode(code) {
		return fmt.Errorf("角色代码长度必须在2-50个字符之间，且只能包含字母、数字和下划线")
	}
	if !s.ValidateName(name) {
		return fmt.Errorf("角色名称长度必须在2-50个字符之间")
	}
	return nil
}

// ValidateUpdateParams 验证更新角色的参数
func (s *RoleService) ValidateUpdateParams(name, status string) error {
	if !s.ValidateName(name) {
		return fmt.Errorf("角色名称长度必须在2-50个字符之间")
	}
	if !s.ValidateStatus(status) {
		return fmt.Errorf("状态只能是active或inactive")
	}
	return nil
}

// GetByTenantWithPage 分页获取租户角色
func (s *RoleService) GetByTenantWithPage(tenantID uint, status string, page, pageSize int) ([]*models.Role, int64, error) {
	var roles []*models.Role
	var total int64

	query := s.db.Model(&models.Role{}).Where("tenant_id = ?", tenantID)

	// 按状态筛选
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Preload("Permissions").Offset(offset).Limit(pageSize).Find(&roles).Error
	if err != nil {
		return nil, 0, err
	}

	return roles, total, nil
}
