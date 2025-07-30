package services

import (
	"ahop/internal/models"
	"ahop/pkg/logger"
	"ahop/pkg/pagination"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"strings"
	"time"
)

// WorkflowExecutor 工作流执行器
type WorkflowExecutor struct {
	db                *gorm.DB
	parser            *WorkflowParser
	nodeExecutors     map[string]NodeExecutor
	taskService       *TaskService
	ticketService     *TicketService
}

// NodeExecutor 节点执行器接口
type NodeExecutor interface {
	Execute(node *models.WorkflowNode, context *ExecutionContext) (*NodeResult, error)
}

// ExecutionContext 执行上下文
type ExecutionContext struct {
	Execution  *models.HealingExecution
	Workflow   *models.HealingWorkflow
	Variables  map[string]interface{}
	NodeStates map[string]*models.NodeState
	Logger     *logrus.Logger
}

// NodeResult 节点执行结果
type NodeResult struct {
	Status  string                 `json:"status"`  // success/failed/skipped
	Output  map[string]interface{} `json:"output"`
	Error   string                 `json:"error,omitempty"`
	NextNodes []string             `json:"next_nodes,omitempty"`
}

// NewWorkflowExecutor 创建工作流执行器
func NewWorkflowExecutor(db *gorm.DB, taskService *TaskService, ticketService *TicketService) *WorkflowExecutor {
	executor := &WorkflowExecutor{
		db:            db,
		parser:        NewWorkflowParser(),
		nodeExecutors: make(map[string]NodeExecutor),
		taskService:   taskService,
		ticketService: ticketService,
	}
	
	// 注册节点执行器
	executor.registerNodeExecutors()
	
	return executor
}

// registerNodeExecutors 注册节点执行器
func (e *WorkflowExecutor) registerNodeExecutors() {
	// 注册各种节点类型的执行器
	e.nodeExecutors[models.NodeTypeStart] = &StartNodeExecutor{}
	e.nodeExecutors[models.NodeTypeEnd] = &EndNodeExecutor{}
	e.nodeExecutors[models.NodeTypeCondition] = &ConditionNodeExecutor{}
	e.nodeExecutors[models.NodeTypeDataProcess] = &DataProcessNodeExecutor{}
	e.nodeExecutors[models.NodeTypeTaskExecute] = &TaskExecuteNodeExecutor{
		taskService:      e.taskService,
		variableResolver: NewVariableResolver(),
		hostService:      NewHostService(e.db),
		workflowExecutor: e,
	}
	e.nodeExecutors[models.NodeTypeControl] = &ControlNodeExecutor{}
	e.nodeExecutors[models.NodeTypeTicketUpdate] = &TicketUpdateNodeExecutor{
		ticketService:    e.ticketService,
		variableResolver: NewVariableResolver(),
	}
}

// ExecuteWorkflow 执行工作流
func (e *WorkflowExecutor) ExecuteWorkflow(workflow *models.HealingWorkflow, triggerType string, triggerSource map[string]interface{}, userID *uint) (*models.HealingExecution, error) {
	log := logger.GetLogger()
	
	// 解析工作流
	parsedWorkflow, err := e.parser.Parse(json.RawMessage(workflow.Definition))
	if err != nil {
		return nil, fmt.Errorf("解析工作流失败: %v", err)
	}
	
	// 创建执行实例
	execution := &models.HealingExecution{
		TenantID:      workflow.TenantID,
		ExecutionID:   uuid.New().String(),
		WorkflowID:    workflow.ID,
		TriggerType:   triggerType,
		TriggerSource: nil,
		TriggerUser:   userID,
		Status:        models.ExecutionStatusRunning,
		StartTime:     time.Now(),
		Context:       nil,
		NodeStates:    nil,
	}
	
	// 如果是规则触发，设置规则ID
	if triggerType == "scheduled" && triggerSource != nil {
		if ruleInfo, ok := triggerSource["rule"].(map[string]interface{}); ok {
			if ruleID, ok := ruleInfo["id"].(uint); ok {
				execution.RuleID = &ruleID
			}
		}
	}
	
	if triggerSource != nil {
		triggerSourceJSON, _ := json.Marshal(triggerSource)
		execution.TriggerSource = triggerSourceJSON
	}
	
	// 初始化执行上下文
	context := &ExecutionContext{
		Execution:  execution,
		Workflow:   workflow,
		Variables:  make(map[string]interface{}),
		NodeStates: make(map[string]*models.NodeState),
		Logger:     log,
	}
	
	// 从解析后的工作流定义中初始化变量
	if parsedWorkflow.Definition != nil && parsedWorkflow.Definition.Variables != nil {
		for k, v := range parsedWorkflow.Definition.Variables {
			context.Variables[k] = v
		}
	}
	
	// 创建 global_context 作为所有数据的统一入口
	globalContext := make(map[string]interface{})
	
	// 如果有触发源数据，添加到 global_context
	if triggerSource != nil {
		// 将触发源数据添加到 global_context.trigger
		globalContext["trigger"] = triggerSource
	}
	
	// 添加工作流信息到 global_context
	globalContext["workflow"] = map[string]interface{}{
		"id":   workflow.ID,
		"name": workflow.Name,
		"code": workflow.Code,
	}
	
	// 将 global_context 添加到变量中
	context.Variables["global_context"] = globalContext
	
	// 保存执行实例
	if err := e.db.Create(execution).Error; err != nil {
		return nil, fmt.Errorf("创建执行实例失败: %v", err)
	}
	
	// 开始执行工作流
	go e.executeWorkflowAsync(execution, parsedWorkflow, context)
	
	return execution, nil
}

// executeWorkflowAsync 异步执行工作流
func (e *WorkflowExecutor) executeWorkflowAsync(execution *models.HealingExecution, workflow *ParsedWorkflow, context *ExecutionContext) {
	log := context.Logger
	startTime := time.Now()
	
	// 记录开始日志（包含触发信息）
	startOutput := map[string]interface{}{
		"trigger_type": execution.TriggerType,
		"workflow_id": context.Workflow.ID,
		"workflow_name": context.Workflow.Name,
	}
	if execution.TriggerSource != nil {
		var triggerData map[string]interface{}
		json.Unmarshal(execution.TriggerSource, &triggerData)
		startOutput["trigger_source"] = triggerData
	}
	endTime := time.Now()
	e.logExecutionWithDetails(execution.ID, "start", models.LogLevelInfo, "工作流开始执行", 
		"workflow", "工作流", &startTime, &endTime, nil, startOutput, nil)
	
	// 从开始节点开始执行
	err := e.executeNode(workflow.StartNode, workflow, context)
	
	// 计算执行时长
	duration := int(time.Since(startTime).Seconds())
	
	// 更新执行状态
	status := models.ExecutionStatusSuccess
	var errorMsg string
	if err != nil {
		status = models.ExecutionStatusFailed
		errorMsg = err.Error()
		log.WithError(err).Error("工作流执行失败")
	}
	
	// 保存节点状态
	nodeStatesJSON, _ := json.Marshal(context.NodeStates)
	
	// 更新执行实例
	execution.Status = status
	execution.EndTime = &[]time.Time{time.Now()}[0]
	execution.Duration = duration
	execution.NodeStates = nodeStatesJSON
	execution.ErrorMsg = errorMsg
	
	updates := map[string]interface{}{
		"status":      status,
		"end_time":    execution.EndTime,
		"duration":    duration,
		"node_states": nodeStatesJSON,
		"error_msg":   errorMsg,
	}
	
	if err := e.db.Model(&models.HealingExecution{}).Where("id = ?", execution.ID).Updates(updates).Error; err != nil {
		log.WithError(err).Error("更新执行实例失败")
	}
	
	// 生成执行摘要并添加到上下文
	executionSummary := e.GenerateExecutionSummary(execution)
	context.Variables["execution_summary"] = executionSummary
	context.Variables["execution_status"] = string(status)
	
	// 添加执行信息到上下文
	executionInfo := map[string]interface{}{
		"id":         execution.ExecutionID,
		"status":     string(status),
		"start_time": execution.StartTime.Format("2006-01-02 15:04:05"),
		"end_time":   execution.EndTime.Format("2006-01-02 15:04:05"),
		"duration":   duration,
	}
	
	// 更新 global_context
	if gc, ok := context.Variables["global_context"].(map[string]interface{}); ok {
		gc["execution"] = executionInfo
	}
	
	// 记录结束日志
	endTime = time.Now()
	endOutput := map[string]interface{}{
		"status": string(status),
		"duration": duration,
		"total_nodes": len(context.NodeStates),
	}
	
	// 统计各状态节点数
	statusCount := make(map[string]int)
	for _, state := range context.NodeStates {
		statusCount[state.Status]++
	}
	endOutput["node_status_count"] = statusCount
	
	if err != nil {
		errorData := map[string]interface{}{
			"error": errorMsg,
			"failed_at": time.Now().Format(time.RFC3339),
		}
		e.logExecutionWithDetails(execution.ID, "end", models.LogLevelError, "工作流执行失败", 
			"workflow", "工作流", &startTime, &endTime, nil, endOutput, errorData)
	} else {
		e.logExecutionWithDetails(execution.ID, "end", models.LogLevelInfo, "工作流执行成功", 
			"workflow", "工作流", &startTime, &endTime, nil, endOutput, nil)
	}
	
	// 更新工作流执行统计
	e.updateWorkflowStats(context.Workflow.ID, status == models.ExecutionStatusSuccess, duration)
}

// executeNode 执行节点
func (e *WorkflowExecutor) executeNode(node *models.WorkflowNode, workflow *ParsedWorkflow, context *ExecutionContext) error {
	// 检查节点是否已执行
	if state, exists := context.NodeStates[node.ID]; exists && state.Status != models.ExecutionStatusPending {
		return nil // 节点已执行，跳过
	}
	
	// 获取节点执行器
	executor, exists := e.nodeExecutors[node.Type]
	if !exists {
		return fmt.Errorf("未知的节点类型: %s", node.Type)
	}
	
	// 更新节点状态为执行中
	startTime := time.Now()
	context.NodeStates[node.ID] = &models.NodeState{
		NodeID:    node.ID,
		Status:    models.ExecutionStatusRunning,
		StartTime: &startTime,
	}
	
	// 准备节点输入数据（从当前上下文变量中提取相关数据）
	nodeInput := make(map[string]interface{})
	if node.Config != nil {
		// 记录节点配置作为输入
		nodeInput["config"] = node.Config
	}
	// 记录当前变量上下文的快照（避免记录过多数据）
	if len(context.Variables) <= 10 {
		nodeInput["context_vars"] = context.Variables
	} else {
		nodeInput["context_vars_count"] = len(context.Variables)
	}
	
	// 不再记录"开始执行"日志，只在执行完成时记录完整日志
	
	// 执行节点
	result, err := executor.Execute(node, context)
	
	// 更新节点状态
	endTime := time.Now()
	nodeState := context.NodeStates[node.ID]
	nodeState.EndTime = &endTime
	nodeState.Input = nodeInput
	
	if err != nil {
		nodeState.Status = models.ExecutionStatusFailed
		nodeState.Error = err.Error()
		
		// 记录错误日志
		errorData := map[string]interface{}{
			"error": err.Error(),
			"type": fmt.Sprintf("%T", err),
		}
		e.logExecutionWithDetails(context.Execution.ID, node.ID, models.LogLevelError,
			fmt.Sprintf("%s 失败: %v", node.Name, err), node.Type, node.Name,
			&startTime, &endTime, nodeInput, nil, errorData)
		
		// 根据错误处理策略决定是否继续
		if node.ErrorHandle == models.ErrorHandleContinue {
			// 继续执行下一个节点
		} else if node.ErrorHandle == models.ErrorHandleRetry {
			// TODO: 实现重试逻辑
			return err
		} else {
			// 默认停止执行
			return err
		}
	} else {
		nodeState.Status = models.ExecutionStatusSuccess
		var outputData map[string]interface{}
		if result != nil {
			nodeState.Output = result.Output
			outputData = result.Output
		}
		
		// 记录成功日志
		e.logExecutionWithDetails(context.Execution.ID, node.ID, models.LogLevelInfo,
			fmt.Sprintf("%s", node.Name), node.Type, node.Name,
			&startTime, &endTime, nodeInput, outputData, nil)
	}
	
	// 执行下一个节点
	var nextNodes []string
	if result != nil && len(result.NextNodes) > 0 {
		nextNodes = result.NextNodes
	} else {
		nextNodes = e.parser.GetNextNodes(node, context.Variables)
	}
	
	for _, nextID := range nextNodes {
		if nextNode, exists := workflow.NodeMap[nextID]; exists {
			if err := e.executeNode(nextNode, workflow, context); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// logExecution 记录执行日志
func (e *WorkflowExecutor) logExecution(executionID uint, nodeID string, level string, message string, 
	nodeType string, nodeName string, output map[string]interface{}) {
	
	log := &models.HealingExecutionLog{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Level:       level,
		Message:     message,
		Timestamp:   time.Now(),
		NodeType:    nodeType,
		NodeName:    nodeName,
	}
	
	if output != nil {
		outputJSON, _ := json.Marshal(output)
		log.Output = outputJSON
	}
	
	if err := e.db.Create(log).Error; err != nil {
		logger.GetLogger().WithError(err).Error("记录执行日志失败")
	}
}

// logExecutionWithDetails 记录详细的执行日志
func (e *WorkflowExecutor) logExecutionWithDetails(executionID uint, nodeID string, level string, message string, 
	nodeType string, nodeName string, startTime, endTime *time.Time, input, output, errorData map[string]interface{}) {
	
	log := &models.HealingExecutionLog{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Level:       level,
		Message:     message,
		Timestamp:   time.Now(),
		NodeType:    nodeType,
		NodeName:    nodeName,
		StartTime:   startTime,
		EndTime:     endTime,
	}
	
	// 计算执行时长（毫秒）
	if startTime != nil && endTime != nil {
		duration := int(endTime.Sub(*startTime).Milliseconds())
		// 最小记录1毫秒
		if duration == 0 {
			duration = 1
		}
		log.Duration = duration
	}
	
	// 序列化输入数据
	if input != nil {
		inputJSON, _ := json.Marshal(input)
		log.Input = inputJSON
	}
	
	// 序列化输出数据
	if output != nil {
		outputJSON, _ := json.Marshal(output)
		log.Output = outputJSON
	}
	
	// 序列化错误数据
	if errorData != nil {
		errorJSON, _ := json.Marshal(errorData)
		log.Error = errorJSON
	}
	
	if err := e.db.Create(log).Error; err != nil {
		logger.GetLogger().WithError(err).Error("记录执行日志失败")
	}
}

// updateWorkflowStats 更新工作流执行统计
func (e *WorkflowExecutor) updateWorkflowStats(workflowID uint, success bool, duration int) {
	svc := NewHealingWorkflowService(e.db)
	if err := svc.UpdateExecutionStats(workflowID, success, duration); err != nil {
		logger.GetLogger().WithError(err).Error("更新工作流统计失败")
	}
}

// GetExecution 获取执行实例
func (e *WorkflowExecutor) GetExecution(executionID string) (*models.HealingExecution, error) {
	var execution models.HealingExecution
	if err := e.db.Where("execution_id = ?", executionID).
		Preload("Workflow").
		Preload("Workflow.Tenant").
		Preload("Rule").
		Preload("Rule.Workflow").
		Preload("Rule.Tenant").
		Preload("Tenant").
		First(&execution).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("执行实例不存在")
		}
		return nil, err
	}
	return &execution, nil
}

// GetExecutionLogs 获取执行日志
func (e *WorkflowExecutor) GetExecutionLogs(executionID uint) ([]models.HealingExecutionLog, error) {
	var logs []models.HealingExecutionLog
	if err := e.db.Where("execution_id = ?", executionID).
		Order("timestamp ASC").
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// GetExecutionDetail 获取执行详情（返回精简的结构）
func (e *WorkflowExecutor) GetExecutionDetail(executionID string) (*ExecutionDetailResponse, error) {
	// 获取执行记录
	execution, err := e.GetExecution(executionID)
	if err != nil {
		return nil, err
	}
	
	// 获取执行日志
	logs, err := e.GetExecutionLogs(execution.ID)
	if err != nil {
		logger.GetLogger().WithError(err).Error("获取执行日志失败")
		logs = []models.HealingExecutionLog{} // 即使获取失败也继续
	}
	
	// 转换为精简的响应结构
	return ConvertToExecutionDetail(execution, logs), nil
}

// ListExecutions 获取执行历史列表
func (e *WorkflowExecutor) ListExecutions(tenantID uint, params *pagination.PageParams, workflowID uint, ruleID uint, status string, triggerType string) ([]models.HealingExecution, int64, error) {
	var executions []models.HealingExecution
	var total int64

	query := e.db.Model(&models.HealingExecution{}).Where("tenant_id = ?", tenantID)

	// 应用过滤条件
	if workflowID > 0 {
		query = query.Where("workflow_id = ?", workflowID)
	}
	if ruleID > 0 {
		query = query.Where("rule_id = ?", ruleID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if triggerType != "" {
		query = query.Where("trigger_type = ?", triggerType)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := params.GetOffset()
	limit := params.GetLimit()
	if err := query.Offset(offset).Limit(limit).
		Order("start_time DESC").
		Preload("Workflow").
		Preload("Workflow.Tenant").
		Preload("Rule").
		Preload("Tenant").
		Find(&executions).Error; err != nil {
		return nil, 0, err
	}

	return executions, total, nil
}

// GenerateExecutionSummary 生成执行摘要
func (e *WorkflowExecutor) GenerateExecutionSummary(execution *models.HealingExecution) string {
	// 获取所有执行日志
	logs, err := e.GetExecutionLogs(execution.ID)
	if err != nil {
		return fmt.Sprintf("获取执行日志失败: %v", err)
	}

	var summary strings.Builder
	
	// 标题
	if execution.Status == models.ExecutionStatusSuccess {
		summary.WriteString("✅ 自动修复成功\n\n")
	} else if execution.Status == models.ExecutionStatusFailed {
		summary.WriteString("❌ 自动修复失败\n\n")
	} else {
		summary.WriteString("⏳ 工作流执行中\n\n")
	}
	
	// 执行信息
	summary.WriteString("【执行信息】\n")
	// 确保工作流信息已加载
	if execution.WorkflowID > 0 {
		if execution.Workflow.ID == 0 {
			// 如果没有预加载，尝试加载
			e.db.First(&execution.Workflow, execution.WorkflowID)
		}
		if execution.Workflow.ID > 0 {
			summary.WriteString(fmt.Sprintf("- 工作流：%s\n", execution.Workflow.Name))
		}
	}
	summary.WriteString(fmt.Sprintf("- 执行ID：%s\n", execution.ExecutionID))
	summary.WriteString(fmt.Sprintf("- 开始时间：%s\n", execution.StartTime.Format("2006-01-02 15:04:05")))
	if execution.EndTime != nil {
		summary.WriteString(fmt.Sprintf("- 结束时间：%s\n", execution.EndTime.Format("2006-01-02 15:04:05")))
		summary.WriteString(fmt.Sprintf("- 执行耗时：%d秒\n", execution.Duration))
	}
	
	// 执行步骤
	summary.WriteString("\n【执行步骤】\n")
	for _, log := range logs {
		// 格式化图标
		icon := "▶"
		if log.Level == models.LogLevelError {
			icon = "✗"
		} else if log.NodeType != models.NodeTypeStart && log.Level == models.LogLevelInfo {
			icon = "✓"
		}
		
		// 格式化日志行
		summary.WriteString(fmt.Sprintf("[%s] %s %s\n", 
			log.Timestamp.Format("15:04:05"),
			icon,
			log.Message))
			
		// 添加详细输出（如果有）
		if log.Output != nil && len(log.Output) > 0 {
			var output map[string]interface{}
			if err := json.Unmarshal(log.Output, &output); err == nil {
				// 格式化输出内容
				for key, value := range output {
					// 跳过一些内部字段
					if key == "processed" || key == "results" {
						continue
					}
					summary.WriteString(fmt.Sprintf("  - %s: %v\n", key, value))
				}
			}
		}
	}
	
	// 执行结果
	summary.WriteString("\n【执行结果】\n")
	if execution.Status == models.ExecutionStatusSuccess {
		summary.WriteString("所有任务执行成功，问题已自动修复。\n")
	} else if execution.Status == models.ExecutionStatusFailed {
		if execution.ErrorMsg != "" {
			summary.WriteString(fmt.Sprintf("执行失败：%s\n", execution.ErrorMsg))
		}
		
		// 分析失败原因
		var failedNode string
		for _, log := range logs {
			if log.Level == models.LogLevelError && log.NodeName != "" {
				failedNode = log.NodeName
				break
			}
		}
		
		if failedNode != "" {
			summary.WriteString(fmt.Sprintf("\n失败节点：%s\n", failedNode))
		}
		
		summary.WriteString("\n【建议操作】\n")
		summary.WriteString("请人工介入处理，检查失败原因并手动修复。\n")
	}
	
	return summary.String()
}