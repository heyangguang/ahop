package services

import (
	"ahop/internal/models"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// FieldMappingService 字段映射服务
type FieldMappingService struct {
	db *gorm.DB
}

// NewFieldMappingService 创建字段映射服务
func NewFieldMappingService(db *gorm.DB) *FieldMappingService {
	return &FieldMappingService{db: db}
}

// GetByPlugin 获取插件的字段映射
func (s *FieldMappingService) GetByPlugin(tenantID, pluginID uint) ([]models.FieldMapping, error) {
	// 验证插件存在且属于当前租户
	var plugin models.TicketPlugin
	if err := s.db.Where("id = ? AND tenant_id = ?", pluginID, tenantID).First(&plugin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("插件不存在")
		}
		return nil, err
	}

	// 获取字段映射
	var mappings []models.FieldMapping
	if err := s.db.Where("plugin_id = ?", pluginID).Order("created_at ASC").Find(&mappings).Error; err != nil {
		return nil, err
	}

	return mappings, nil
}


// UpdateMappings 更新字段映射（统一接口，支持0个或多个）
func (s *FieldMappingService) UpdateMappings(tenantID, pluginID uint, req UpdateFieldMappingsRequest) ([]models.FieldMapping, error) {
	// 验证插件存在且属于当前租户
	var plugin models.TicketPlugin
	if err := s.db.Where("id = ? AND tenant_id = ?", pluginID, tenantID).First(&plugin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("插件不存在")
		}
		return nil, err
	}

	// 开启事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 删除现有映射
	if err := tx.Where("plugin_id = ?", pluginID).Delete(&models.FieldMapping{}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// 创建新映射
	var mappings []models.FieldMapping
	validTargetFields := map[string]bool{
		"external_id": true, "title": true, "description": true,
		"status": true, "priority": true, "type": true,
		"reporter": true, "assignee": true, "category": true,
		"service": true, "tags": true,
	}
	
	for _, item := range req.Mappings {
		// 验证目标字段
		if !validTargetFields[item.TargetField] {
			tx.Rollback()
			return nil, fmt.Errorf("无效的目标字段: %s", item.TargetField)
		}
		
		mapping := models.FieldMapping{
			PluginID:    pluginID,
			SourceField: item.SourceField,
			TargetField: item.TargetField,
		}
		
		// 处理可选字段
		if item.DefaultValue != nil {
			mapping.DefaultValue = *item.DefaultValue
		}
		if item.Required != nil {
			mapping.Required = *item.Required
		}
		
		if err := tx.Create(&mapping).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		mappings = append(mappings, mapping)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return mappings, nil
}

// 请求结构体

// UpdateFieldMappingsRequest 更新字段映射请求（统一接口）
type UpdateFieldMappingsRequest struct {
	Mappings []FieldMappingItem `json:"mappings"`
}

// FieldMappingItem 字段映射项
type FieldMappingItem struct {
	SourceField  string  `json:"source_field" binding:"required"`
	TargetField  string  `json:"target_field" binding:"required"`
	DefaultValue *string `json:"default_value,omitempty"`
	Required     *bool   `json:"required,omitempty"`
}