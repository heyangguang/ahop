package services

import (
	"ahop/internal/database"
	"ahop/internal/models"
	"fmt"
	"unicode/utf8"

	"gorm.io/gorm"
)

type TenantService struct {
	db *gorm.DB
}

// TenantStats 租户统计信息
type TenantStats struct {
	Total    int64 `json:"total"`
	Active   int64 `json:"active"`
	Inactive int64 `json:"inactive"`
}

// StatusCount 状态分布统计
type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

func NewTenantService() *TenantService {
	return &TenantService{
		db: database.GetDB(),
	}
}

// GetWithFiltersAndPage 组合查询（分页版本）
func (s *TenantService) GetWithFiltersAndPage(status, keyword string, page, pageSize int) ([]*models.Tenant, int64, error) {
	var tenants []*models.Tenant
	var total int64

	query := s.db.Model(&models.Tenant{})

	// 添加过滤条件
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		searchPattern := fmt.Sprintf("%%%s%%", keyword)
		query = query.Where("name LIKE ? OR code LIKE ?", searchPattern, searchPattern)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Find(&tenants).Error
	if err != nil {
		return nil, 0, err
	}

	return tenants, total, nil
}

// Create 创建租户
func (s *TenantService) Create(name, code string) (*models.Tenant, error) {
	// 验证参数
	if err := s.ValidateCreateParams(name, code); err != nil {
		return nil, err
	}

	// 检查代码是否重复
	var count int64
	s.db.Model(&models.Tenant{}).Where("code = ?", code).Count(&count)
	if count > 0 {
		return nil, gorm.ErrDuplicatedKey
	}

	tenant := &models.Tenant{
		Name:   name,
		Code:   code,
		Status: models.TenantStatusActive,
	}

	err := s.db.Create(tenant).Error
	return tenant, err
}

// GetByID 根据ID获取租户
func (s *TenantService) GetByID(id uint) (*models.Tenant, error) {
	var tenant models.Tenant
	err := s.db.First(&tenant, id).Error
	return &tenant, err
}

// GetAllActive 获取所有激活的租户
func (s *TenantService) GetAllActive() ([]*models.Tenant, error) {
	var tenants []*models.Tenant
	// 查询激活的租户，并预加载用户数量
	err := s.db.Model(&models.Tenant{}).
		Where("status = ?", models.TenantStatusActive).
		Order("created_at DESC").
		Find(&tenants).Error
	
	// 统计每个租户的用户数量
	for i := range tenants {
		var userCount int64
		s.db.Model(&models.User{}).Where("tenant_id = ?", tenants[i].ID).Count(&userCount)
		tenants[i].UserCount = int(userCount)
	}
	
	return tenants, err
}

// GetRecentlyCreatedWithPage 最近创建的租户（分页）
func (s *TenantService) GetRecentlyCreatedWithPage(page, pageSize int) ([]*models.Tenant, int64, error) {
	var tenants []*models.Tenant
	var total int64

	// 计算总数
	if err := s.db.Model(&models.Tenant{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询（按创建时间降序）
	offset := (page - 1) * pageSize
	err := s.db.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tenants).Error
	if err != nil {
		return nil, 0, err
	}

	return tenants, total, nil
}

// Update 更新租户
func (s *TenantService) Update(id uint, name, status string) (*models.Tenant, error) {
	var tenant models.Tenant
	err := s.db.First(&tenant, id).Error
	if err != nil {
		return nil, err
	}

	tenant.Name = name
	tenant.Status = status

	err = s.db.Save(&tenant).Error
	return &tenant, err
}

// Delete 删除租户
func (s *TenantService) Delete(id uint) error {
	return s.db.Delete(&models.Tenant{}, id).Error
}

// Activate 激活租户
func (s *TenantService) Activate(id uint) (*models.Tenant, error) {
	var tenant models.Tenant
	err := s.db.First(&tenant, id).Error
	if err != nil {
		return nil, err
	}

	s.SetActiveStatus(&tenant) // 使用业务方法
	err = s.db.Save(&tenant).Error
	return &tenant, err
}

// Deactivate 停用租户
func (s *TenantService) Deactivate(id uint) (*models.Tenant, error) {
	var tenant models.Tenant
	err := s.db.First(&tenant, id).Error
	if err != nil {
		return nil, err
	}

	s.SetInactiveStatus(&tenant) // 使用业务方法
	err = s.db.Save(&tenant).Error
	return &tenant, err
}

// GetStats 获取租户统计
func (s *TenantService) GetStats() (*TenantStats, error) {
	stats := &TenantStats{}

	// 总数
	s.db.Model(&models.Tenant{}).Count(&stats.Total)

	// 各状态数量
	s.db.Model(&models.Tenant{}).Where("status = ?", models.TenantStatusActive).Count(&stats.Active)
	s.db.Model(&models.Tenant{}).Where("status = ?", models.TenantStatusInactive).Count(&stats.Inactive)

	return stats, nil
}

// GetStatusDistribution 统计租户方法
func (s *TenantService) GetStatusDistribution() ([]*StatusCount, error) {
	var results []*StatusCount
	err := s.db.Model(&models.Tenant{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Find(&results).Error
	return results, err
}

// IsValidStatus 检查租户状态是否有效
func (s *TenantService) IsValidStatus(status string) bool {
	switch status {
	case models.TenantStatusActive, models.TenantStatusInactive:
		return true
	default:
		return false
	}
}

// IsActive 检查租户是否激活
func (s *TenantService) IsActive(tenant *models.Tenant) bool {
	return tenant.Status == models.TenantStatusActive
}

// IsInactive 检查租户是否停用
func (s *TenantService) IsInactive(tenant *models.Tenant) bool {
	return tenant.Status == models.TenantStatusInactive
}

// SetActiveStatus 设置租户状态为激活
func (s *TenantService) SetActiveStatus(tenant *models.Tenant) {
	tenant.Status = models.TenantStatusActive
}

// SetInactiveStatus 设置租户状态为停用
func (s *TenantService) SetInactiveStatus(tenant *models.Tenant) {
	tenant.Status = models.TenantStatusInactive
}

// ========== 验证相关方法 ==========

// ValidateName 验证方法中的字符长度计算
func (s *TenantService) ValidateName(name string) bool {
	// 使用 utf8.RuneCountInString 替代 len
	runeCount := utf8.RuneCountInString(name)
	return runeCount >= 2 && runeCount <= 50
}

// ValidateCode 保持不变
func (s *TenantService) ValidateCode(code string) bool {
	if len(code) < 2 || len(code) > 20 {
		return false
	}
	for _, r := range code {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// ValidateCreateParams 保持不变（可选：改进错误信息）
func (s *TenantService) ValidateCreateParams(name, code string) error {
	if !s.ValidateName(name) {
		return fmt.Errorf("租户名称长度必须在2-50个字符之间")
	}
	if !s.ValidateCode(code) {
		return fmt.Errorf("租户代码长度必须在2-20个字符之间，且只能包含字母和数字")
	}
	return nil
}
