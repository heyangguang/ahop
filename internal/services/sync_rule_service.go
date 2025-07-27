package services

import (
	"ahop/internal/models"
	"errors"

	"gorm.io/gorm"
)

// SyncRuleService 同步规则服务
type SyncRuleService struct {
	db *gorm.DB
}

// NewSyncRuleService 创建同步规则服务
func NewSyncRuleService(db *gorm.DB) *SyncRuleService {
	return &SyncRuleService{db: db}
}

// GetByPlugin 获取插件的同步规则
func (s *SyncRuleService) GetByPlugin(tenantID, pluginID uint) ([]models.SyncRule, error) {
	// 验证插件存在且属于当前租户
	var plugin models.TicketPlugin
	if err := s.db.Where("id = ? AND tenant_id = ?", pluginID, tenantID).First(&plugin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("插件不存在")
		}
		return nil, err
	}

	// 获取同步规则
	var rules []models.SyncRule
	if err := s.db.Where("plugin_id = ?", pluginID).Order("priority ASC").Find(&rules).Error; err != nil {
		return nil, err
	}

	return rules, nil
}


// UpdateRules 更新同步规则（统一接口，支持0个或多个）
func (s *SyncRuleService) UpdateRules(tenantID, pluginID uint, req UpdateSyncRulesRequest) ([]models.SyncRule, error) {
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

	// 删除现有规则
	if err := tx.Where("plugin_id = ?", pluginID).Delete(&models.SyncRule{}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// 创建新规则
	var rules []models.SyncRule
	for i, item := range req.Rules {
		rule := models.SyncRule{
			PluginID: pluginID,
			Name:     item.Name,
			Field:    item.Field,
			Operator: item.Operator,
			Value:    item.Value,
			Action:   item.Action,
			Priority: i + 1, // 自动设置优先级
			Enabled:  true,   // 默认启用
		}
		
		// 处理可选字段
		if item.Enabled != nil {
			rule.Enabled = *item.Enabled
		}
		
		if err := tx.Create(&rule).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		rules = append(rules, rule)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return rules, nil
}

// 请求结构体

// UpdateSyncRulesRequest 更新同步规则请求（统一接口）
type UpdateSyncRulesRequest struct {
	Rules []SyncRuleItem `json:"rules"`
}

// SyncRuleItem 同步规则项
type SyncRuleItem struct {
	Name     string `json:"name" binding:"required"`
	Field    string `json:"field" binding:"required"`
	Operator string `json:"operator" binding:"required,oneof=equals not_equals contains not_contains in not_in regex greater less"`
	Value    string `json:"value" binding:"required"`
	Action   string `json:"action" binding:"required,oneof=include exclude"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

