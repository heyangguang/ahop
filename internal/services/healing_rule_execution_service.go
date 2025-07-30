package services

import (
	"ahop/internal/models"
	"ahop/pkg/pagination"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// HealingRuleExecutionService 自愈规则执行记录服务
type HealingRuleExecutionService struct {
	db *gorm.DB
}

// NewHealingRuleExecutionService 创建服务实例
func NewHealingRuleExecutionService(db *gorm.DB) *HealingRuleExecutionService {
	return &HealingRuleExecutionService{db: db}
}

// GetByID 根据ID获取执行记录
func (s *HealingRuleExecutionService) GetByID(id uint) (*models.HealingRuleExecution, error) {
	var execution models.HealingRuleExecution
	err := s.db.Preload("Rule").Preload("Tenant").First(&execution, id).Error
	if err != nil {
		return nil, err
	}
	return &execution, nil
}

// GetByRuleID 获取指定规则的执行记录列表
func (s *HealingRuleExecutionService) GetByRuleID(ruleID uint, params *pagination.PageParams) ([]models.HealingRuleExecution, int64, error) {
	var executions []models.HealingRuleExecution
	var total int64
	
	query := s.db.Model(&models.HealingRuleExecution{}).Where("rule_id = ?", ruleID)
	
	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// 分页查询
	offset := params.GetOffset()
	limit := params.GetLimit()
	
	err := query.Order("execution_time DESC").
		Offset(offset).
		Limit(limit).
		Find(&executions).Error
		
	return executions, total, err
}

// GetByTenant 获取租户的执行记录列表
func (s *HealingRuleExecutionService) GetByTenant(tenantID uint, params *pagination.PageParams) ([]models.HealingRuleExecution, int64, error) {
	var executions []models.HealingRuleExecution
	var total int64
	
	query := s.db.Model(&models.HealingRuleExecution{}).Where("tenant_id = ?", tenantID)
	
	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// 分页查询
	offset := params.GetOffset()
	limit := params.GetLimit()
	
	err := query.Preload("Rule").
		Order("execution_time DESC").
		Offset(offset).
		Limit(limit).
		Find(&executions).Error
		
	return executions, total, err
}

// GetRuleExecutionStats 获取规则执行统计
func (s *HealingRuleExecutionService) GetRuleExecutionStats(tenantID uint, startTime, endTime *time.Time) ([]models.RuleExecutionStats, error) {
	query := s.db.Table("healing_rule_executions").
		Select(`
			rule_id,
			COUNT(*) as total_executions,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_executions,
			SUM(CASE WHEN status = 'failed' OR status = 'partial' THEN 1 ELSE 0 END) as failed_executions,
			SUM(total_tickets_scanned) as total_tickets_scanned,
			SUM(matched_tickets) as total_matched,
			SUM(executions_created) as total_created,
			AVG(duration) as avg_duration,
			MAX(execution_time) as last_execution_time
		`).
		Where("tenant_id = ?", tenantID).
		Group("rule_id")
		
	// 时间范围过滤
	if startTime != nil {
		query = query.Where("execution_time >= ?", startTime)
	}
	if endTime != nil {
		query = query.Where("execution_time <= ?", endTime)
	}
	
	// 定义结果结构
	type statsResult struct {
		RuleID             uint       `gorm:"column:rule_id"`
		TotalExecutions    int64      `gorm:"column:total_executions"`
		SuccessExecutions  int64      `gorm:"column:success_executions"`
		FailedExecutions   int64      `gorm:"column:failed_executions"`
		TotalTicketsScanned int64     `gorm:"column:total_tickets_scanned"`
		TotalMatched       int64      `gorm:"column:total_matched"`
		TotalCreated       int64      `gorm:"column:total_created"`
		AvgDuration        float64    `gorm:"column:avg_duration"`
		LastExecutionTime  *time.Time `gorm:"column:last_execution_time"`
	}
	
	var results []statsResult
	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}
	
	// 获取规则名称
	ruleIDs := make([]uint, 0, len(results))
	for _, r := range results {
		ruleIDs = append(ruleIDs, r.RuleID)
	}
	
	var rules []models.HealingRule
	ruleMap := make(map[uint]string)
	if len(ruleIDs) > 0 {
		s.db.Where("id IN ?", ruleIDs).Find(&rules)
		for _, rule := range rules {
			ruleMap[rule.ID] = rule.Name
		}
	}
	
	// 转换为返回结构
	stats := make([]models.RuleExecutionStats, 0, len(results))
	for _, r := range results {
		stat := models.RuleExecutionStats{
			RuleID:             r.RuleID,
			RuleName:           ruleMap[r.RuleID],
			TotalExecutions:    r.TotalExecutions,
			SuccessExecutions:  r.SuccessExecutions,
			FailedExecutions:   r.FailedExecutions,
			TotalTicketsScanned: r.TotalTicketsScanned,
			TotalMatched:       r.TotalMatched,
			TotalCreated:       r.TotalCreated,
			AvgDuration:        r.AvgDuration,
			LastExecutionTime:  r.LastExecutionTime,
		}
		
		// 计算成功率
		if stat.TotalExecutions > 0 {
			stat.SuccessRate = float64(stat.SuccessExecutions) / float64(stat.TotalExecutions) * 100
		}
		
		// 计算匹配率
		if stat.TotalTicketsScanned > 0 {
			stat.MatchRate = float64(stat.TotalMatched) / float64(stat.TotalTicketsScanned) * 100
		}
		
		stats = append(stats, stat)
	}
	
	return stats, nil
}

// GetRecentExecutions 获取最近的执行记录
func (s *HealingRuleExecutionService) GetRecentExecutions(tenantID uint, limit int) ([]models.HealingRuleExecution, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	
	var executions []models.HealingRuleExecution
	err := s.db.Where("tenant_id = ?", tenantID).
		Preload("Rule").
		Order("execution_time DESC").
		Limit(limit).
		Find(&executions).Error
		
	return executions, err
}

// GetExecutionDetail 获取执行详情（包含匹配的工单和创建的执行）
func (s *HealingRuleExecutionService) GetExecutionDetail(id uint) (map[string]interface{}, error) {
	var execution models.HealingRuleExecution
	if err := s.db.Preload("Rule").First(&execution, id).Error; err != nil {
		return nil, err
	}
	
	// 解析匹配的工单
	matchedTickets, _ := execution.GetMatchedTickets()
	
	// 解析创建的执行ID
	executionIDs, _ := execution.GetExecutionIDs()
	
	// 获取创建的执行详情
	var healingExecutions []models.HealingExecution
	if len(executionIDs) > 0 {
		s.db.Where("execution_id IN ?", executionIDs).Find(&healingExecutions)
	}
	
	return map[string]interface{}{
		"execution":          execution,
		"matched_tickets":    matchedTickets,
		"created_executions": healingExecutions,
	}, nil
}

// CleanupOldExecutions 清理旧的执行记录
func (s *HealingRuleExecutionService) CleanupOldExecutions(daysToKeep int) (int64, error) {
	if daysToKeep <= 0 {
		return 0, fmt.Errorf("保留天数必须大于0")
	}
	
	cutoffTime := time.Now().AddDate(0, 0, -daysToKeep)
	
	result := s.db.Where("execution_time < ?", cutoffTime).Delete(&models.HealingRuleExecution{})
	return result.RowsAffected, result.Error
}