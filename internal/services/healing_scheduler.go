package services

import (
	"ahop/internal/models"
	"ahop/pkg/logger"
	"encoding/json"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// HealingScheduler 自愈规则调度器
type HealingScheduler struct {
	db                     *gorm.DB
	cron                   *cron.Cron
	ruleService           *HealingRuleService
	workflowService       *HealingWorkflowService
	workflowExecutor      *WorkflowExecutor
	taskService           *TaskService
	ticketService         *TicketService
	scheduledJobs         map[uint]cron.EntryID // ruleID -> cronJobID
	logger                *logrus.Logger
	isRunning             bool
}

// NewHealingScheduler 创建自愈调度器
func NewHealingScheduler(db *gorm.DB, taskService *TaskService, ticketService *TicketService) *HealingScheduler {
	return &HealingScheduler{
		db:                db,
		cron:              cron.New(cron.WithSeconds()),
		ruleService:       NewHealingRuleService(db),
		workflowService:   NewHealingWorkflowService(db),
		workflowExecutor:  NewWorkflowExecutor(db, taskService, ticketService),
		taskService:       taskService,
		ticketService:     ticketService,
		scheduledJobs:     make(map[uint]cron.EntryID),
		logger:            logger.GetLogger(),
		isRunning:         false,
	}
}

// Start 启动调度器
func (s *HealingScheduler) Start() error {
	if s.isRunning {
		return fmt.Errorf("调度器已经在运行")
	}

	s.logger.Info("启动自愈规则调度器")

	// 加载所有活跃的定时规则
	if err := s.loadScheduledRules(); err != nil {
		return fmt.Errorf("加载定时规则失败: %v", err)
	}

	// 启动cron调度器
	s.cron.Start()
	s.isRunning = true

	s.logger.Infof("自愈规则调度器启动成功，已加载 %d 个定时规则", len(s.scheduledJobs))
	return nil
}

// Stop 停止调度器
func (s *HealingScheduler) Stop() {
	if !s.isRunning {
		return
	}

	s.logger.Info("停止自愈规则调度器")
	s.cron.Stop()
	s.isRunning = false
	s.scheduledJobs = make(map[uint]cron.EntryID)
}

// loadScheduledRules 加载所有定时规则
func (s *HealingScheduler) loadScheduledRules() error {
	var rules []models.HealingRule
	err := s.db.Where("is_active = ? AND trigger_type = ?", true, "scheduled").
		Find(&rules).Error
	if err != nil {
		return err
	}

	for _, rule := range rules {
		if err := s.scheduleRule(&rule); err != nil {
			s.logger.WithError(err).Errorf("调度规则 %s 失败", rule.Code)
		}
	}

	// 批量更新所有已调度规则的下次执行时间
	for ruleID, jobID := range s.scheduledJobs {
		if entry := s.cron.Entry(jobID); entry.ID != 0 {
			nextRun := entry.Next
			if err := s.db.Model(&models.HealingRule{}).Where("id = ?", ruleID).Update("next_run_at", nextRun).Error; err != nil {
				s.logger.WithError(err).Errorf("更新规则 %d 的下次执行时间失败", ruleID)
			}
		}
	}

	return nil
}

// scheduleRule 调度单个规则
func (s *HealingScheduler) scheduleRule(rule *models.HealingRule) error {
	// 验证cron表达式
	if rule.CronExpr == "" {
		return fmt.Errorf("规则 %s 缺少cron表达式", rule.Code)
	}

	// 如果规则已经被调度，先移除旧的
	if jobID, exists := s.scheduledJobs[rule.ID]; exists {
		s.cron.Remove(jobID)
	}

	// 创建cron任务
	jobID, err := s.cron.AddFunc(rule.CronExpr, func() {
		s.executeRule(rule.ID)
	})
	if err != nil {
		return fmt.Errorf("添加cron任务失败: %v", err)
	}

	s.scheduledJobs[rule.ID] = jobID
	
	// 更新下次执行时间
	if entry := s.cron.Entry(jobID); entry.ID != 0 {
		nextRun := entry.Next
		s.db.Model(&models.HealingRule{}).Where("id = ?", rule.ID).Update("next_run_at", nextRun)
	}
	
	s.logger.Infof("已调度规则 %s，cron表达式: %s", rule.Code, rule.CronExpr)
	return nil
}

// executeRule 执行规则
func (s *HealingScheduler) executeRule(ruleID uint) {
	logger := s.logger.WithField("rule_id", ruleID)
	logger.Info("开始执行自愈规则")
	
	// 记录开始时间
	startTime := time.Now()
	
	// 创建规则执行记录
	ruleExecution := &models.HealingRuleExecution{
		RuleID:        ruleID,
		ExecutionTime: startTime,
		Status:        models.RuleExecStatusSuccess, // 默认成功，有失败时修改
	}

	// 获取规则详情（先获取基本信息）
	var rule models.HealingRule
	if err := s.db.Preload("Workflow").First(&rule, ruleID).Error; err != nil {
		logger.WithError(err).Error("获取规则失败")
		ruleExecution.Status = models.RuleExecStatusFailed
		ruleExecution.ErrorMsg = fmt.Sprintf("获取规则失败: %v", err)
		ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
		s.recordRuleExecution(ruleExecution)
		return
	}
	
	// 设置租户ID
	ruleExecution.TenantID = rule.TenantID

	// 检查规则是否仍然活跃
	if !rule.IsActive {
		logger.Info("规则已禁用，跳过执行")
		ruleExecution.Status = models.RuleExecStatusFailed
		ruleExecution.ErrorMsg = "规则已禁用"
		ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
		s.recordRuleExecution(ruleExecution)
		return
	}

	// 检查冷却时间
	if !s.checkCooldown(&rule) {
		logger.Info("规则仍在冷却期，跳过执行")
		ruleExecution.Status = models.RuleExecStatusFailed
		ruleExecution.ErrorMsg = "规则仍在冷却期"
		ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
		s.recordRuleExecution(ruleExecution)
		return
	}

	// 检查执行次数限制
	if rule.MaxExecutions > 0 && rule.ExecuteCount >= int64(rule.MaxExecutions) {
		logger.Info("规则已达到最大执行次数，跳过执行")
		ruleExecution.Status = models.RuleExecStatusFailed
		ruleExecution.ErrorMsg = "规则已达到最大执行次数"
		ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
		s.recordRuleExecution(ruleExecution)
		return
	}

	// 执行匹配逻辑
	matched, matchData, ticketsScanned, err := s.matchRuleWithStats(&rule)
	if err != nil {
		logger.WithError(err).Error("执行匹配规则失败")
		ruleExecution.Status = models.RuleExecStatusFailed
		ruleExecution.ErrorMsg = fmt.Sprintf("执行匹配规则失败: %v", err)
		ruleExecution.TotalTicketsScanned = ticketsScanned
		ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
		s.recordRuleExecution(ruleExecution)
		return
	}
	
	// 记录扫描的工单数
	ruleExecution.TotalTicketsScanned = ticketsScanned

	if !matched {
		logger.Debug("没有匹配的数据，跳过执行")
		ruleExecution.Status = models.RuleExecStatusNoMatch
		ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
		s.recordRuleExecution(ruleExecution)
		return
	}

	// 工作流已经通过Preload加载
	workflow := &rule.Workflow

	// 检查工作流是否活跃
	if !workflow.IsActive {
		logger.Info("工作流已禁用，跳过执行")
		ruleExecution.Status = models.RuleExecStatusFailed
		ruleExecution.ErrorMsg = "工作流已禁用"
		ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
		s.recordRuleExecution(ruleExecution)
		return
	}

	// 将匹配的数据转换为数组（如果不是）
	var matchedItems []interface{}
	var matchedTicketInfos []models.MatchedTicketInfo
	
	switch v := matchData.(type) {
	case []models.Ticket:
		for _, ticket := range v {
			ticketCopy := ticket // 创建副本避免并发问题
			matchedItems = append(matchedItems, &ticketCopy)
			// 记录匹配的工单信息
			matchedTicketInfos = append(matchedTicketInfos, models.MatchedTicketInfo{
				ID:         ticket.ID,
				ExternalID: ticket.ExternalID,
				Title:      ticket.Title,
				Priority:   ticket.Priority,
				Status:     ticket.Status,
			})
		}
	case []interface{}:
		matchedItems = v
	default:
		matchedItems = []interface{}{matchData}
	}
	
	// 记录匹配的工单数
	ruleExecution.MatchedTickets = len(matchedItems)
	if len(matchedTicketInfos) > 0 {
		ruleExecution.SetMatchedTickets(matchedTicketInfos)
	}

	logger.Infof("规则 %s 匹配到 %d 个项目，开始执行工作流", rule.Code, len(matchedItems))

	// 为每个匹配的项目执行工作流
	successCount := 0
	failCount := 0
	var executionIDs []string
	
	for i, item := range matchedItems {
		// 处理匹配的数据项
		var processedItem interface{}
		
		// 如果是 Ticket 类型，特殊处理
		if ticket, ok := item.(models.Ticket); ok {
			processedItem = s.prepareTicketData(&ticket)
		} else if ticket, ok := item.(*models.Ticket); ok {
			processedItem = s.prepareTicketData(ticket)
		} else {
			processedItem = item
		}
		
		// 准备触发源信息
		triggerSource := map[string]interface{}{
			"rule": map[string]interface{}{
				"id":   rule.ID,
				"code": rule.Code,
				"name": rule.Name,
			},
			"matched_item":  processedItem,  // 使用处理后的数据
			"item_index":    i,
			"total_items":   len(matchedItems),
			"trigger_type":  "scheduled",
			"trigger_time":  time.Now().Format(time.RFC3339),
		}
		
		// 使用 WorkflowExecutor 执行工作流
		execution, err := s.workflowExecutor.ExecuteWorkflow(workflow, "scheduled", triggerSource, nil)
		if err != nil {
			logger.WithError(err).Errorf("执行工作流失败，项目 %d", i+1)
			failCount++
			continue
		}
		
		successCount++
		executionIDs = append(executionIDs, execution.ExecutionID)
		logger.Infof("创建自愈执行成功，执行ID: %s", execution.ExecutionID)
	}
	
	// 记录创建的执行ID
	ruleExecution.ExecutionsCreated = successCount
	if len(executionIDs) > 0 {
		ruleExecution.SetExecutionIDs(executionIDs)
	}
	
	// 根据执行结果设置状态
	if successCount == 0 {
		ruleExecution.Status = models.RuleExecStatusFailed
		ruleExecution.ErrorMsg = "所有工作流执行都失败"
	} else if failCount > 0 {
		ruleExecution.Status = models.RuleExecStatusPartial
		ruleExecution.ErrorMsg = fmt.Sprintf("部分失败: %d 成功, %d 失败", successCount, failCount)
	} else {
		ruleExecution.Status = models.RuleExecStatusSuccess
	}
	
	// 记录执行时间
	ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
	
	// 保存执行记录
	s.recordRuleExecution(ruleExecution)

	// 更新规则执行统计
	if successCount > 0 {
		s.updateRuleExecutionStats(rule.ID, true)
		logger.Infof("自愈规则触发完成，成功创建 %d 个任务，失败 %d 个", successCount, failCount)
	} else {
		s.updateRuleExecutionStats(rule.ID, false)
		logger.Errorf("自愈规则触发失败，无法创建任务")
	}
}

// checkCooldown 检查冷却时间
func (s *HealingScheduler) checkCooldown(rule *models.HealingRule) bool {
	if rule.CooldownMinutes <= 0 || rule.LastExecuteAt == nil {
		return true
	}

	cooldownDuration := time.Duration(rule.CooldownMinutes) * time.Minute
	nextAllowedTime := rule.LastExecuteAt.Add(cooldownDuration)
	
	return time.Now().After(nextAllowedTime)
}

// inferDataSource 从匹配规则中推断数据源
func (s *HealingScheduler) inferDataSource(matchRule *models.HealingRuleMatchCondition) string {
	// 如果当前规则有source，直接返回
	if matchRule.Source != "" {
		return matchRule.Source
	}
	
	// 如果有子条件，递归查找
	if len(matchRule.Conditions) > 0 {
		for _, condition := range matchRule.Conditions {
			source := s.inferDataSource(&condition)
			if source != "" {
				return source
			}
		}
	}
	
	return ""
}

// matchRule 执行规则匹配（原方法，保持兼容）
func (s *HealingScheduler) matchRule(rule *models.HealingRule) (bool, interface{}, error) {
	matched, matchData, _, err := s.matchRuleWithStats(rule)
	return matched, matchData, err
}

// matchRuleWithStats 执行规则匹配并返回统计信息
func (s *HealingScheduler) matchRuleWithStats(rule *models.HealingRule) (bool, interface{}, int, error) {
	// 解析匹配规则
	if len(rule.MatchRules) == 0 {
		return false, nil, 0, fmt.Errorf("规则没有定义匹配条件")
	}

	var matchRules models.HealingRuleMatchCondition
	if err := json.Unmarshal(rule.MatchRules, &matchRules); err != nil {
		return false, nil, 0, fmt.Errorf("解析匹配规则失败: %v", err)
	}

	// 从匹配规则中推断数据源
	source := s.inferDataSource(&matchRules)
	if source == "" {
		return false, nil, 0, fmt.Errorf("无法推断数据源类型")
	}

	// 根据数据源执行匹配
	switch source {
	case "ticket":
		return s.matchTicketsWithStats(rule.TenantID, &matchRules)
	default:
		return false, nil, 0, fmt.Errorf("不支持的数据源: %s", source)
	}
}

// matchTickets 匹配工单（原方法，保持兼容）
func (s *HealingScheduler) matchTickets(tenantID uint, matchRule *models.HealingRuleMatchCondition) (bool, interface{}, error) {
	matched, matchData, _, err := s.matchTicketsWithStats(tenantID, matchRule)
	return matched, matchData, err
}

// matchTicketsWithStats 匹配工单并返回统计信息
func (s *HealingScheduler) matchTicketsWithStats(tenantID uint, matchRule *models.HealingRuleMatchCondition) (bool, interface{}, int, error) {
	// 先获取总工单数
	var totalCount int64
	s.db.Model(&models.Ticket{}).Where("tenant_id = ?", tenantID).Count(&totalCount)
	
	query := s.db.Model(&models.Ticket{}).Where("tenant_id = ?", tenantID)

	// 应用主条件
	query = s.applyMatchCondition(query, matchRule)

	// 应用子条件
	if matchRule.Conditions != nil && len(matchRule.Conditions) > 0 {
		for _, condition := range matchRule.Conditions {
			if matchRule.LogicOp == "or" {
				// OR逻辑
				subQuery := s.db.Model(&models.Ticket{}).Where("tenant_id = ?", tenantID)
				subQuery = s.applyMatchCondition(subQuery, &condition)
				query = query.Or(subQuery)
			} else {
				// AND逻辑（默认）
				query = s.applyMatchCondition(query, &condition)
			}
		}
	}

	// 查询匹配的工单
	var tickets []models.Ticket
	if err := query.Find(&tickets).Error; err != nil {
		return false, nil, int(totalCount), err
	}

	if len(tickets) == 0 {
		return false, nil, int(totalCount), nil
	}

	// 返回匹配的工单数据
	return true, tickets, int(totalCount), nil
}

// applyMatchCondition 应用匹配条件
func (s *HealingScheduler) applyMatchCondition(query *gorm.DB, condition *models.HealingRuleMatchCondition) *gorm.DB {
	field := condition.Field
	operator := condition.Operator
	value := condition.Value

	switch operator {
	case "equals":
		return query.Where(fmt.Sprintf("%s = ?", field), value)
	case "not_equals":
		return query.Where(fmt.Sprintf("%s != ?", field), value)
	case "contains":
		return query.Where(fmt.Sprintf("%s LIKE ?", field), "%"+fmt.Sprint(value)+"%")
	case "not_contains":
		return query.Where(fmt.Sprintf("%s NOT LIKE ?", field), "%"+fmt.Sprint(value)+"%")
	case "starts_with":
		return query.Where(fmt.Sprintf("%s LIKE ?", field), fmt.Sprint(value)+"%")
	case "ends_with":
		return query.Where(fmt.Sprintf("%s LIKE ?", field), "%"+fmt.Sprint(value))
	case "greater_than":
		return query.Where(fmt.Sprintf("%s > ?", field), value)
	case "less_than":
		return query.Where(fmt.Sprintf("%s < ?", field), value)
	case "in":
		return query.Where(fmt.Sprintf("%s IN ?", field), value)
	case "not_in":
		return query.Where(fmt.Sprintf("%s NOT IN ?", field), value)
	default:
		// 未知操作符，返回原查询
		return query
	}
}

// updateRuleExecutionStats 更新规则执行统计
func (s *HealingScheduler) updateRuleExecutionStats(ruleID uint, success bool) error {
	updates := map[string]interface{}{
		"last_execute_at": time.Now(),
		"execute_count":   gorm.Expr("execute_count + 1"),
	}

	if success {
		updates["success_count"] = gorm.Expr("success_count + 1")
	} else {
		updates["failure_count"] = gorm.Expr("failure_count + 1")
	}
	
	// 更新下次执行时间
	if jobID, exists := s.scheduledJobs[ruleID]; exists {
		if entry := s.cron.Entry(jobID); entry.ID != 0 {
			updates["next_run_at"] = entry.Next
		}
	}

	return s.db.Model(&models.HealingRule{}).Where("id = ?", ruleID).Updates(updates).Error
}

// RefreshRule 刷新单个规则的调度
func (s *HealingScheduler) RefreshRule(ruleID uint) error {
	if !s.isRunning {
		return fmt.Errorf("调度器未运行")
	}

	// 获取规则
	var rule models.HealingRule
	if err := s.db.Preload("Workflow").First(&rule, ruleID).Error; err != nil {
		return err
	}

	// 移除旧的调度
	if jobID, exists := s.scheduledJobs[ruleID]; exists {
		s.cron.Remove(jobID)
		delete(s.scheduledJobs, ruleID)
	}

	// 如果规则是活跃的定时规则，重新调度
	if rule.IsActive && rule.TriggerType == "scheduled" {
		return s.scheduleRule(&rule)
	}

	return nil
}

// RemoveRule 移除规则的调度
func (s *HealingScheduler) RemoveRule(ruleID uint) {
	if jobID, exists := s.scheduledJobs[ruleID]; exists {
		s.cron.Remove(jobID)
		delete(s.scheduledJobs, ruleID)
		s.logger.Infof("已移除规则 %d 的调度", ruleID)
	}
}

// GetSchedulerStatus 获取调度器状态信息
func (s *HealingScheduler) GetSchedulerStatus() map[string]interface{} {
	status := map[string]interface{}{
		"running":     s.isRunning,
		"rules_count": len(s.scheduledJobs),
		"rules":       []map[string]interface{}{},
	}

	// 获取所有调度的规则
	entries := s.cron.Entries()
	for ruleID, entryID := range s.scheduledJobs {
		// 查找对应的cron entry
		for _, entry := range entries {
			if entry.ID == entryID {
				// 获取规则信息
				var rule models.HealingRule
				if err := s.db.First(&rule, ruleID).Error; err == nil {
					ruleInfo := map[string]interface{}{
						"rule_id":        rule.ID,
						"rule_name":      rule.Name,
						"rule_code":      rule.Code,
						"cron_expr":      rule.CronExpr,
						"is_active":      rule.IsActive,
						"next_run_at":    entry.Next,
						"last_execute_at": rule.LastExecuteAt,
						"execute_count":  rule.ExecuteCount,
						"success_count":  rule.SuccessCount,
						"failure_count":  rule.FailureCount,
					}
					
					// 计算距离下次执行的时间
					if !entry.Next.IsZero() {
						duration := time.Until(entry.Next)
						ruleInfo["next_run_in"] = formatDuration(duration)
						ruleInfo["next_run_in_seconds"] = int(duration.Seconds())
					}
					
					status["rules"] = append(status["rules"].([]map[string]interface{}), ruleInfo)
				}
				break
			}
		}
	}

	return status
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "已过期"
	}
	
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	
	if days > 0 {
		return fmt.Sprintf("%d天%d小时%d分钟", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟%d秒", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%d分钟%d秒", minutes, seconds)
	} else {
		return fmt.Sprintf("%d秒", seconds)
	}
}

// ExecuteManualRule 手动执行规则
func (s *HealingScheduler) ExecuteManualRule(ruleID uint, userID uint) (*models.HealingExecution, error) {
	// 记录开始时间
	startTime := time.Now()
	
	// 创建规则执行记录
	ruleExecution := &models.HealingRuleExecution{
		RuleID:        ruleID,
		ExecutionTime: startTime,
		Status:        models.RuleExecStatusSuccess, // 默认成功，有失败时修改
	}
	
	// 获取规则
	var rule models.HealingRule
	if err := s.db.Preload("Workflow").First(&rule, ruleID).Error; err != nil {
		ruleExecution.Status = models.RuleExecStatusFailed
		ruleExecution.ErrorMsg = fmt.Sprintf("获取规则失败: %v", err)
		ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
		s.recordRuleExecution(ruleExecution)
		return nil, fmt.Errorf("获取规则失败: %v", err)
	}
	
	// 设置租户ID
	ruleExecution.TenantID = rule.TenantID

	// 检查规则是否允许手动触发
	if rule.TriggerType != "manual" && rule.TriggerType != "scheduled" {
		ruleExecution.Status = models.RuleExecStatusFailed
		ruleExecution.ErrorMsg = "规则不支持手动触发"
		ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
		s.recordRuleExecution(ruleExecution)
		return nil, fmt.Errorf("规则不支持手动触发")
	}

	// 检查规则是否活跃
	if !rule.IsActive {
		ruleExecution.Status = models.RuleExecStatusFailed
		ruleExecution.ErrorMsg = "规则已禁用"
		ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
		s.recordRuleExecution(ruleExecution)
		return nil, fmt.Errorf("规则已禁用")
	}

	// 工作流已经通过Preload加载
	workflow := &rule.Workflow

	// 检查工作流是否活跃
	if !workflow.IsActive {
		ruleExecution.Status = models.RuleExecStatusFailed
		ruleExecution.ErrorMsg = "工作流已禁用"
		ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
		s.recordRuleExecution(ruleExecution)
		return nil, fmt.Errorf("工作流已禁用")
	}

	// 准备触发源数据
	triggerSource := map[string]interface{}{
		"rule_id":    ruleID,
		"rule_code":  rule.Code,
		"manual":     true,
		"user_id":    userID,
	}

	// 执行工作流
	execution, err := s.workflowExecutor.ExecuteWorkflow(workflow, "manual", triggerSource, &userID)
	if err != nil {
		// 执行失败，更新失败统计
		s.updateRuleExecutionStats(rule.ID, false)
		ruleExecution.Status = models.RuleExecStatusFailed
		ruleExecution.ErrorMsg = fmt.Sprintf("执行工作流失败: %v", err)
		ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
		s.recordRuleExecution(ruleExecution)
		return nil, fmt.Errorf("执行工作流失败: %v", err)
	}

	// 记录执行结果
	ruleExecution.ExecutionsCreated = 1
	ruleExecution.SetExecutionIDs([]string{execution.ExecutionID})
	ruleExecution.Duration = int(time.Since(startTime).Milliseconds())
	
	// 根据执行结果更新统计
	success := execution.Status == "success"
	s.updateRuleExecutionStats(rule.ID, success)
	
	// 保存执行记录
	s.recordRuleExecution(ruleExecution)

	return execution, nil
}

// prepareTicketData 准备工单数据，确保 custom_data 正确解析
func (s *HealingScheduler) prepareTicketData(ticket *models.Ticket) map[string]interface{} {
	// 转换为 map
	ticketData := map[string]interface{}{
		"id":                   ticket.ID,
		"tenant_id":            ticket.TenantID,
		"plugin_id":            ticket.PluginID,
		"external_id":          ticket.ExternalID,
		"title":                ticket.Title,
		"description":          ticket.Description,
		"status":               ticket.Status,
		"priority":             ticket.Priority,
		"type":                 ticket.Type,
		"reporter":             ticket.Reporter,
		"assignee":             ticket.Assignee,
		"category":             ticket.Category,
		"service":              ticket.Service,
		"external_created_at":  ticket.ExternalCreatedAt,
		"external_updated_at":  ticket.ExternalUpdatedAt,
		"tags":                 ticket.Tags,
		"synced_at":            ticket.SyncedAt,
		"created_at":           ticket.CreatedAt,
		"updated_at":           ticket.UpdatedAt,
	}
	
	// 特殊处理 custom_data 字段
	if ticket.CustomData != nil && len(ticket.CustomData) > 0 {
		var customData interface{}
		if err := json.Unmarshal(ticket.CustomData, &customData); err == nil {
			ticketData["custom_data"] = customData
		} else {
			// 如果解析失败，保持原样
			ticketData["custom_data"] = ticket.CustomData
		}
	}
	
	return ticketData
}

// recordRuleExecution 记录规则执行信息
func (s *HealingScheduler) recordRuleExecution(execution *models.HealingRuleExecution) {
	if err := s.db.Create(execution).Error; err != nil {
		s.logger.WithError(err).Error("保存规则执行记录失败")
	} else {
		s.logger.WithFields(logrus.Fields{
			"rule_id":        execution.RuleID,
			"status":         execution.Status,
			"scanned":        execution.TotalTicketsScanned,
			"matched":        execution.MatchedTickets,
			"created":        execution.ExecutionsCreated,
			"duration_ms":    execution.Duration,
		}).Info("规则执行记录已保存")
	}
}