package services

import (
	"ahop/internal/database"
	"ahop/internal/models"

	"gorm.io/gorm"
)

type PermissionService struct {
	db *gorm.DB
}

func NewPermissionService() *PermissionService {
	return &PermissionService{
		db: database.GetDB(),
	}
}

// ========== 基础CRUD方法 ==========

// GetWithPage 分页获取权限
func (s *PermissionService) GetWithPage(module string, page, pageSize int) ([]*models.Permission, int64, error) {
	var permissions []*models.Permission
	var total int64

	query := s.db.Model(&models.Permission{})

	// 按模块筛选
	if module != "" {
		query = query.Where("module = ?", module)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Find(&permissions).Error
	if err != nil {
		return nil, 0, err
	}

	return permissions, total, nil
}

// GetByID 根据ID获取权限
func (s *PermissionService) GetByID(id uint) (*models.Permission, error) {
	var permission models.Permission
	err := s.db.First(&permission, id).Error
	return &permission, err
}

// Create 创建权限（系统级操作，一般预设）
func (s *PermissionService) Create(code, name, description, module, action string) (*models.Permission, error) {
	permission := &models.Permission{
		Code:        code,
		Name:        name,
		Description: description,
		Module:      module,
		Action:      action,
	}

	err := s.db.Create(permission).Error
	return permission, err
}

