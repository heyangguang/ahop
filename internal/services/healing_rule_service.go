package services

import (
	"ahop/internal/models"
	"ahop/pkg/logger"
	"ahop/pkg/pagination"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// HealingRuleService 自愈规则服务
type HealingRuleService struct {
	db *gorm.DB
}

// NewHealingRuleService 创建自愈规则服务
func NewHealingRuleService(db *gorm.DB) *HealingRuleService {
	return &HealingRuleService{db: db}
}

// CreateHealingRuleRequest 创建自愈规则请求
type CreateHealingRuleRequest struct {
	Name            string                         `json:"name" binding:"required,max=200"`
	Code            string                         `json:"code" binding:"required,max=100"`
	Description     string                         `json:"description" binding:"max=500"`
	TriggerType     string                         `json:"trigger_type" binding:"required,oneof=scheduled manual"`
	CronExpr        string                         `json:"cron_expr"`
	MatchRules      models.HealingRuleMatchCondition `json:"match_rules" binding:"required"`
	Priority        int                            `json:"priority"`
	WorkflowID      uint                           `json:"workflow_id" binding:"required"`
	MaxExecutions   int                            `json:"max_executions"`
	CooldownMinutes int                            `json:"cooldown_minutes"`
}

// UpdateHealingRuleRequest 更新自愈规则请求
type UpdateHealingRuleRequest struct {
	Name            string                         `json:"name" binding:"max=200"`
	Description     string                         `json:"description" binding:"max=500"`
	CronExpr        string                         `json:"cron_expr"`
	MatchRules      *models.HealingRuleMatchCondition `json:"match_rules"`
	Priority        *int                           `json:"priority"`
	WorkflowID      *uint                          `json:"workflow_id"`
	MaxExecutions   *int                           `json:"max_executions"`
	CooldownMinutes *int                           `json:"cooldown_minutes"`
}

// Create 创建自愈规则
func (s *HealingRuleService) Create(tenantID uint, userID uint, req CreateHealingRuleRequest) (*models.HealingRule, error) {
	// 验证code在租户内唯一
	var count int64
	if err := s.db.Model(&models.HealingRule{}).
		Where("tenant_id = ? AND code = ?", tenantID, req.Code).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("规则代码已存在")
	}

	// 验证工作流存在
	var workflow models.HealingWorkflow
	if err := s.db.Where("id = ? AND tenant_id = ?", req.WorkflowID, tenantID).
		First(&workflow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("工作流不存在")
		}
		return nil, err
	}

	// 定时触发需要cron表达式
	if req.TriggerType == models.HealingTriggerScheduled {
		if req.CronExpr == "" {
			return nil, errors.New("定时触发必须设置cron表达式")
		}
		// 验证cron表达式格式（需要6个字段，包含秒）
		if err := validateCronExpression(req.CronExpr); err != nil {
			return nil, fmt.Errorf("cron表达式格式错误: %v", err)
		}
	}

	// 序列化匹配规则
	matchRulesJSON, err := json.Marshal(req.MatchRules)
	if err != nil {
		return nil, fmt.Errorf("序列化匹配规则失败: %v", err)
	}

	// 创建规则
	rule := &models.HealingRule{
		TenantID:        tenantID,
		Name:            req.Name,
		Code:            req.Code,
		Description:     req.Description,
		TriggerType:     req.TriggerType,
		CronExpr:        req.CronExpr,
		MatchRules:      matchRulesJSON,
		Priority:        req.Priority,
		WorkflowID:      req.WorkflowID,
		MaxExecutions:   req.MaxExecutions,
		CooldownMinutes: req.CooldownMinutes,
		CreatedBy:       userID,
		UpdatedBy:       userID,
	}

	if rule.Priority == 0 {
		rule.Priority = 100
	}
	if rule.CooldownMinutes == 0 {
		rule.CooldownMinutes = 30
	}

	if err := s.db.Create(rule).Error; err != nil {
		return nil, err
	}

	// 预加载关联数据
	if err := s.db.Preload("Workflow").First(rule, rule.ID).Error; err != nil {
		return nil, err
	}

	// 如果是活跃的定时规则，添加到调度器
	if rule.IsActive && rule.TriggerType == "scheduled" {
		if scheduler := GetGlobalHealingScheduler(); scheduler != nil {
			if err := scheduler.RefreshRule(rule.ID); err != nil {
				// 仅记录错误，不影响主流程
				logger.GetLogger().WithError(err).Errorf("添加规则 %d 到调度器失败", rule.ID)
			}
		}
	}

	return rule, nil
}

// Update 更新自愈规则
func (s *HealingRuleService) Update(tenantID uint, ruleID uint, userID uint, req UpdateHealingRuleRequest) (*models.HealingRule, error) {
	var rule models.HealingRule
	if err := s.db.Where("id = ? AND tenant_id = ?", ruleID, tenantID).First(&rule).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("规则不存在")
		}
		return nil, err
	}

	// 构建更新数据
	updates := map[string]interface{}{
		"updated_by": userID,
		"updated_at": time.Now(),
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.CronExpr != "" {
		// 如果是定时触发，验证cron表达式
		if rule.TriggerType == models.HealingTriggerScheduled {
			if err := validateCronExpression(req.CronExpr); err != nil {
				return nil, fmt.Errorf("cron表达式格式错误: %v", err)
			}
		}
		updates["cron_expr"] = req.CronExpr
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.WorkflowID != nil {
		// 验证工作流存在
		var workflow models.HealingWorkflow
		if err := s.db.Where("id = ? AND tenant_id = ?", *req.WorkflowID, tenantID).
			First(&workflow).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("工作流不存在")
			}
			return nil, err
		}
		updates["workflow_id"] = *req.WorkflowID
	}
	if req.MaxExecutions != nil {
		updates["max_executions"] = *req.MaxExecutions
	}
	if req.CooldownMinutes != nil {
		updates["cooldown_minutes"] = *req.CooldownMinutes
	}
	if req.MatchRules != nil {
		matchRulesJSON, err := json.Marshal(req.MatchRules)
		if err != nil {
			return nil, fmt.Errorf("序列化匹配规则失败: %v", err)
		}
		updates["match_rules"] = matchRulesJSON
	}

	// 执行更新
	if err := s.db.Model(&rule).Updates(updates).Error; err != nil {
		return nil, err
	}

	// 重新加载数据
	if err := s.db.Preload("Workflow").First(&rule, rule.ID).Error; err != nil {
		return nil, err
	}

	// 刷新调度器（如果是定时规则）
	if rule.TriggerType == "scheduled" {
		if scheduler := GetGlobalHealingScheduler(); scheduler != nil {
			if err := scheduler.RefreshRule(ruleID); err != nil {
				// 仅记录错误，不影响主流程
				logger.GetLogger().WithError(err).Errorf("刷新规则 %d 调度失败", ruleID)
			}
		}
	}

	return &rule, nil
}

// Delete 删除自愈规则
func (s *HealingRuleService) Delete(tenantID uint, ruleID uint) error {
	// 检查是否有执行历史
	var count int64
	if err := s.db.Model(&models.HealingExecution{}).
		Where("rule_id = ?", ruleID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该规则有执行历史，不能删除")
	}

	// 删除规则
	result := s.db.Where("id = ? AND tenant_id = ?", ruleID, tenantID).
		Delete(&models.HealingRule{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("规则不存在")
	}

	// 从调度器中移除规则
	if scheduler := GetGlobalHealingScheduler(); scheduler != nil {
		scheduler.RemoveRule(ruleID)
	}

	return nil
}

// GetByID 根据ID获取自愈规则
func (s *HealingRuleService) GetByID(tenantID uint, ruleID uint) (*models.HealingRule, error) {
	var rule models.HealingRule
	if err := s.db.Where("id = ? AND tenant_id = ?", ruleID, tenantID).
		Preload("Workflow").
		Preload("Workflow.Tenant").
		Preload("Tenant").
		First(&rule).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("规则不存在")
		}
		return nil, err
	}
	return &rule, nil
}

// List 获取自愈规则列表
func (s *HealingRuleService) List(tenantID uint, params *pagination.PageParams, search string) ([]models.HealingRule, int64, error) {
	var rules []models.HealingRule
	var total int64

	query := s.db.Model(&models.HealingRule{}).Where("tenant_id = ?", tenantID)

	// 搜索条件
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("name LIKE ? OR code LIKE ? OR description LIKE ?", 
			searchPattern, searchPattern, searchPattern)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := params.GetOffset()
	limit := params.GetLimit()
	if err := query.Offset(offset).Limit(limit).
		Order("priority ASC, created_at DESC").
		Preload("Workflow").
		Find(&rules).Error; err != nil {
		return nil, 0, err
	}

	return rules, total, nil
}

// Enable 启用规则
func (s *HealingRuleService) Enable(tenantID uint, ruleID uint, userID uint) error {
	result := s.db.Model(&models.HealingRule{}).
		Where("id = ? AND tenant_id = ?", ruleID, tenantID).
		Updates(map[string]interface{}{
			"is_active":  true,
			"updated_by": userID,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("规则不存在")
	}

	// 刷新调度器
	if scheduler := GetGlobalHealingScheduler(); scheduler != nil {
		if err := scheduler.RefreshRule(ruleID); err != nil {
			// 仅记录错误，不影响主流程
			logger.GetLogger().WithError(err).Errorf("刷新规则 %d 调度失败", ruleID)
		}
	}

	return nil
}

// Disable 禁用规则
func (s *HealingRuleService) Disable(tenantID uint, ruleID uint, userID uint) error {
	result := s.db.Model(&models.HealingRule{}).
		Where("id = ? AND tenant_id = ?", ruleID, tenantID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"updated_by": userID,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("规则不存在")
	}

	// 从调度器中移除规则
	if scheduler := GetGlobalHealingScheduler(); scheduler != nil {
		scheduler.RemoveRule(ruleID)
	}

	return nil
}

// GetActiveRules 获取所有激活的规则（供调度器使用）
func (s *HealingRuleService) GetActiveRules() ([]models.HealingRule, error) {
	var rules []models.HealingRule
	if err := s.db.Where("is_active = ? AND trigger_type = ?", true, models.HealingTriggerScheduled).
		Preload("Workflow").
		Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// CanExecute 检查规则是否可以执行
func (s *HealingRuleService) CanExecute(rule *models.HealingRule) bool {
	// 检查最大执行次数
	if rule.MaxExecutions > 0 && rule.ExecuteCount >= int64(rule.MaxExecutions) {
		return false
	}

	// 检查冷却时间
	if rule.LastExecuteAt != nil {
		cooldownDuration := time.Duration(rule.CooldownMinutes) * time.Minute
		if time.Since(*rule.LastExecuteAt) < cooldownDuration {
			return false
		}
	}

	return true
}

// UpdateExecutionStats 更新执行统计
func (s *HealingRuleService) UpdateExecutionStats(ruleID uint, success bool) error {
	updates := map[string]interface{}{
		"execute_count":  gorm.Expr("execute_count + 1"),
		"last_execute_at": time.Now(),
	}
	
	if success {
		updates["success_count"] = gorm.Expr("success_count + 1")
	} else {
		updates["failure_count"] = gorm.Expr("failure_count + 1")
	}

	return s.db.Model(&models.HealingRule{}).
		Where("id = ?", ruleID).
		Updates(updates).Error
}

// validateCronExpression 验证cron表达式格式
func validateCronExpression(expr string) error {
	// 创建一个带秒的cron解析器来验证表达式
	parser := cron.New(cron.WithSeconds())
	_, err := parser.AddFunc(expr, func() {})
	if err != nil {
		// 提供更友好的错误信息
		if strings.Contains(err.Error(), "expected exactly 6 fields") {
			return fmt.Errorf("需要6个字段（秒 分 时 日 月 周），例如: '0 */30 * * * *' 表示每30分钟执行")
		}
		return err
	}
	return nil
}