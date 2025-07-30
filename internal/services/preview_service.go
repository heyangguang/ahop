package services

import (
	"ahop/internal/models"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Knetic/govaluate"
	"gorm.io/gorm"
)

// PreviewService 预览服务
type PreviewService struct {
	db              *gorm.DB
	workflowParser  *WorkflowParser
	variableResolver *VariableResolver
}

// NewPreviewService 创建预览服务
func NewPreviewService(db *gorm.DB) *PreviewService {
	return &PreviewService{
		db:              db,
		workflowParser:  NewWorkflowParser(),
		variableResolver: NewVariableResolver(),
	}
}

// TestTicket 测试工单数据
type TestTicket struct {
	ExternalID   string                 `json:"external_id"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Priority     string                 `json:"priority"`
	Status       string                 `json:"status"`
	Type         string                 `json:"type"`
	CustomData   map[string]interface{} `json:"custom_data"`
}

// RulePreviewRequest 规则预览请求
type RulePreviewRequest struct {
	TestTickets []TestTicket `json:"test_tickets"`
}

// RulePreviewResult 规则预览结果
type RulePreviewResult struct {
	MatchResults []TicketMatchResult `json:"match_results"`
	Summary      PreviewSummary      `json:"summary"`
}

// TicketMatchResult 工单匹配结果
type TicketMatchResult struct {
	TicketID     string            `json:"ticket_id"`
	TicketTitle  string            `json:"ticket_title"`
	Matched      bool              `json:"matched"`
	MatchProcess []ConditionResult `json:"match_process"`
}

// ConditionResult 条件判断结果
type ConditionResult struct {
	Field       string      `json:"field"`
	Operator    string      `json:"operator"`
	Expected    interface{} `json:"expected"`
	Actual      interface{} `json:"actual"`
	Expression  string      `json:"expression"`
	Result      bool        `json:"result"`
	Reason      string      `json:"reason"`
}

// PreviewSummary 预览摘要
type PreviewSummary struct {
	TotalTested           int    `json:"total_tested"`
	Matched               int    `json:"matched"`
	NotMatched            int    `json:"not_matched"`
	WouldTriggerWorkflow  string `json:"would_trigger_workflow,omitempty"`
	WouldCreateExecutions int    `json:"would_create_executions"`
}

// PreviewRule 预览规则匹配
func (s *PreviewService) PreviewRule(rule *models.HealingRule, req *RulePreviewRequest) (*RulePreviewResult, error) {
	result := &RulePreviewResult{
		MatchResults: make([]TicketMatchResult, 0),
		Summary: PreviewSummary{
			TotalTested: len(req.TestTickets),
		},
	}

	// 解析匹配规则
	var matchRules models.HealingRuleMatchCondition
	if err := json.Unmarshal(rule.MatchRules, &matchRules); err != nil {
		return nil, fmt.Errorf("解析匹配规则失败: %v", err)
	}

	// 预加载工作流信息
	if rule.WorkflowID > 0 {
		var workflow models.HealingWorkflow
		if err := s.db.First(&workflow, rule.WorkflowID).Error; err == nil {
			result.Summary.WouldTriggerWorkflow = workflow.Name
		}
	}

	// 对每个测试工单进行匹配
	for _, testTicket := range req.TestTickets {
		matchResult := s.evaluateTicket(testTicket, &matchRules)
		result.MatchResults = append(result.MatchResults, matchResult)
		
		if matchResult.Matched {
			result.Summary.Matched++
		} else {
			result.Summary.NotMatched++
		}
	}

	result.Summary.WouldCreateExecutions = result.Summary.Matched

	return result, nil
}

// evaluateTicket 评估单个工单
func (s *PreviewService) evaluateTicket(ticket TestTicket, matchRule *models.HealingRuleMatchCondition) TicketMatchResult {
	result := TicketMatchResult{
		TicketID:     ticket.ExternalID,
		TicketTitle:  ticket.Title,
		MatchProcess: make([]ConditionResult, 0),
	}

	// 将工单转换为map以便于访问字段
	ticketData := s.ticketToMap(ticket)
	
	// 评估条件
	matched := s.evaluateCondition(ticketData, matchRule, &result.MatchProcess)
	result.Matched = matched

	return result
}

// ticketToMap 将工单转换为map
func (s *PreviewService) ticketToMap(ticket TestTicket) map[string]interface{} {
	data := map[string]interface{}{
		"external_id":  ticket.ExternalID,
		"title":        ticket.Title,
		"description":  ticket.Description,
		"priority":     ticket.Priority,
		"status":       ticket.Status,
		"type":         ticket.Type,
	}

	// 合并custom_data
	if ticket.CustomData != nil {
		for k, v := range ticket.CustomData {
			data["custom_data."+k] = v
		}
	}

	return data
}

// evaluateCondition 评估条件
func (s *PreviewService) evaluateCondition(data map[string]interface{}, condition *models.HealingRuleMatchCondition, results *[]ConditionResult) bool {
	// 如果有具体的字段条件
	if condition.Field != "" && condition.Operator != "" {
		condResult := s.evaluateFieldCondition(data, condition)
		*results = append(*results, condResult)
		
		// 如果当前条件不满足且是AND逻辑，直接返回false
		if !condResult.Result && condition.LogicOp != "or" {
			return false
		}
		
		// 如果当前条件满足且是OR逻辑，直接返回true
		if condResult.Result && condition.LogicOp == "or" {
			return true
		}
	}

	// 如果有子条件
	if len(condition.Conditions) > 0 {
		if condition.LogicOp == "or" {
			// OR逻辑：任一子条件满足即可
			for _, subCondition := range condition.Conditions {
				if s.evaluateCondition(data, &subCondition, results) {
					return true
				}
			}
			return false
		} else {
			// AND逻辑：所有子条件都必须满足
			for _, subCondition := range condition.Conditions {
				if !s.evaluateCondition(data, &subCondition, results) {
					return false
				}
			}
			return true
		}
	}

	// 如果没有条件，默认匹配
	return true
}

// evaluateFieldCondition 评估字段条件
func (s *PreviewService) evaluateFieldCondition(data map[string]interface{}, condition *models.HealingRuleMatchCondition) ConditionResult {
	result := ConditionResult{
		Field:    condition.Field,
		Operator: condition.Operator,
		Expected: condition.Value,
	}

	// 获取实际值
	actualValue, exists := s.getFieldValue(data, condition.Field)
	result.Actual = actualValue

	// 构建表达式
	result.Expression = fmt.Sprintf("%s %s %v", condition.Field, condition.Operator, condition.Value)

	// 如果字段不存在
	if !exists {
		result.Result = false
		result.Reason = fmt.Sprintf("字段 '%s' 不存在", condition.Field)
		return result
	}

	// 根据操作符进行比较
	switch condition.Operator {
	case "equals":
		result.Result = s.compareEquals(actualValue, condition.Value)
		if result.Result {
			result.Reason = fmt.Sprintf("%v 等于 %v", actualValue, condition.Value)
		} else {
			result.Reason = fmt.Sprintf("%v 不等于 %v", actualValue, condition.Value)
		}

	case "not_equals":
		result.Result = !s.compareEquals(actualValue, condition.Value)
		if result.Result {
			result.Reason = fmt.Sprintf("%v 不等于 %v", actualValue, condition.Value)
		} else {
			result.Reason = fmt.Sprintf("%v 等于 %v", actualValue, condition.Value)
		}

	case "contains":
		result.Result = s.compareContains(actualValue, condition.Value)
		if result.Result {
			result.Reason = fmt.Sprintf("%v 包含 %v", actualValue, condition.Value)
		} else {
			result.Reason = fmt.Sprintf("%v 不包含 %v", actualValue, condition.Value)
		}

	case "greater_than":
		result.Result = s.compareGreaterThan(actualValue, condition.Value)
		if result.Result {
			result.Reason = fmt.Sprintf("%v 大于 %v", actualValue, condition.Value)
		} else {
			result.Reason = fmt.Sprintf("%v 不大于 %v", actualValue, condition.Value)
		}

	case "less_than":
		result.Result = s.compareLessThan(actualValue, condition.Value)
		if result.Result {
			result.Reason = fmt.Sprintf("%v 小于 %v", actualValue, condition.Value)
		} else {
			result.Reason = fmt.Sprintf("%v 不小于 %v", actualValue, condition.Value)
		}

	case "in":
		result.Result = s.compareIn(actualValue, condition.Value)
		if result.Result {
			result.Reason = fmt.Sprintf("%v 在列表 %v 中", actualValue, condition.Value)
		} else {
			result.Reason = fmt.Sprintf("%v 不在列表 %v 中", actualValue, condition.Value)
		}

	default:
		result.Result = false
		result.Reason = fmt.Sprintf("不支持的操作符: %s", condition.Operator)
	}

	return result
}

// getFieldValue 获取字段值
func (s *PreviewService) getFieldValue(data map[string]interface{}, field string) (interface{}, bool) {
	// 直接查找
	if val, ok := data[field]; ok {
		return val, true
	}

	// 尝试处理嵌套字段
	parts := strings.Split(field, ".")
	current := data
	
	for i, part := range parts {
		if i == len(parts)-1 {
			// 最后一个部分
			if val, ok := current[part]; ok {
				return val, true
			}
		} else {
			// 中间部分，必须是map
			if next, ok := current[part]; ok {
				if nextMap, isMap := next.(map[string]interface{}); isMap {
					current = nextMap
				} else {
					return nil, false
				}
			} else {
				return nil, false
			}
		}
	}

	return nil, false
}

// 比较函数
func (s *PreviewService) compareEquals(actual, expected interface{}) bool {
	return fmt.Sprint(actual) == fmt.Sprint(expected)
}

func (s *PreviewService) compareContains(actual, expected interface{}) bool {
	return strings.Contains(fmt.Sprint(actual), fmt.Sprint(expected))
}

func (s *PreviewService) compareGreaterThan(actual, expected interface{}) bool {
	actualFloat, err1 := s.toFloat64(actual)
	expectedFloat, err2 := s.toFloat64(expected)
	if err1 == nil && err2 == nil {
		return actualFloat > expectedFloat
	}
	return false
}

func (s *PreviewService) compareLessThan(actual, expected interface{}) bool {
	actualFloat, err1 := s.toFloat64(actual)
	expectedFloat, err2 := s.toFloat64(expected)
	if err1 == nil && err2 == nil {
		return actualFloat < expectedFloat
	}
	return false
}

func (s *PreviewService) compareIn(actual, expected interface{}) bool {
	// 如果expected是slice
	expectedSlice, ok := expected.([]interface{})
	if !ok {
		// 尝试将字符串转换为slice
		if str, ok := expected.(string); ok {
			var slice []interface{}
			if err := json.Unmarshal([]byte(str), &slice); err == nil {
				expectedSlice = slice
			}
		}
	}

	if expectedSlice != nil {
		actualStr := fmt.Sprint(actual)
		for _, item := range expectedSlice {
			if fmt.Sprint(item) == actualStr {
				return true
			}
		}
	}
	
	return false
}

func (s *PreviewService) toFloat64(val interface{}) (float64, error) {
	switch v := val.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		var f float64
		_, err := fmt.Sscanf(v, "%f", &f)
		return f, err
	default:
		return 0, fmt.Errorf("cannot convert to float64")
	}
}

// WorkflowPreviewRequest 工作流预览请求
type WorkflowPreviewRequest struct {
	TriggerData map[string]interface{} `json:"trigger_data"`
}

// WorkflowPreviewResult 工作流预览结果
type WorkflowPreviewResult struct {
	ExecutionFlow []NodePreview          `json:"execution_flow"`
	VariableTrace map[string]VariableInfo `json:"variable_trace"`
	Summary       WorkflowSummary         `json:"summary"`
}

// NodePreview 节点预览
type NodePreview struct {
	Step         int                    `json:"step"`
	NodeID       string                 `json:"node_id"`
	NodeName     string                 `json:"node_name"`
	NodeType     string                 `json:"node_type"`
	Status       string                 `json:"status"` // would_execute, would_skip, would_fail
	Details      map[string]interface{} `json:"details,omitempty"`
}

// VariableInfo 变量信息
type VariableInfo struct {
	Value  interface{} `json:"value"`
	Source string      `json:"source"`
}

// WorkflowSummary 工作流摘要
type WorkflowSummary struct {
	TotalNodes      int      `json:"total_nodes"`
	WouldExecute    int      `json:"would_execute"`
	WouldSkip       int      `json:"would_skip"`
	TasksToExecute  []string `json:"tasks_to_execute"`
	TargetHosts     []string `json:"target_hosts"`
}

// PreviewWorkflow 预览工作流执行
func (s *PreviewService) PreviewWorkflow(workflow *models.HealingWorkflow, req *WorkflowPreviewRequest) (*WorkflowPreviewResult, error) {
	result := &WorkflowPreviewResult{
		ExecutionFlow: make([]NodePreview, 0),
		VariableTrace: make(map[string]VariableInfo),
		Summary:       WorkflowSummary{},
	}

	// 解析工作流
	parsedWorkflow, err := s.workflowParser.Parse(json.RawMessage(workflow.Definition))
	if err != nil {
		return nil, fmt.Errorf("解析工作流失败: %v", err)
	}

	// 初始化变量上下文
	context := &WorkflowContext{
		Variables:    make(map[string]interface{}),
		TriggerData:  req.TriggerData,
	}

	// 记录初始变量
	for k, v := range req.TriggerData {
		result.VariableTrace[k] = VariableInfo{
			Value:  v,
			Source: "trigger_data",
		}
	}

	// 模拟执行工作流
	step := 1
	currentNode := parsedWorkflow.StartNode
	visitedNodes := make(map[string]bool)

	for currentNode != nil {
		// 防止循环
		if visitedNodes[currentNode.ID] {
			break
		}
		visitedNodes[currentNode.ID] = true

		// 预览节点执行
		nodePreview := s.previewNode(currentNode, context, step)
		result.ExecutionFlow = append(result.ExecutionFlow, nodePreview)
		result.Summary.TotalNodes++

		// 更新统计
		switch nodePreview.Status {
		case "would_execute":
			result.Summary.WouldExecute++
			// 如果是任务节点，记录任务信息
			if currentNode.Type == "task" && nodePreview.Details != nil {
				if taskName, ok := nodePreview.Details["task_name"].(string); ok {
					result.Summary.TasksToExecute = append(result.Summary.TasksToExecute, taskName)
				}
				if hosts, ok := nodePreview.Details["target_hosts"].([]string); ok {
					result.Summary.TargetHosts = append(result.Summary.TargetHosts, hosts...)
				}
			}
		case "would_skip":
			result.Summary.WouldSkip++
		}

		// 获取下一个节点
		nextNodeID := s.getNextNodeID(currentNode, nodePreview, parsedWorkflow)
		if nextNodeID == "" {
			break
		}

		if node, exists := parsedWorkflow.NodeMap[nextNodeID]; exists {
			currentNode = node
		} else {
			break
		}
		step++
	}

	// 去重目标主机
	result.Summary.TargetHosts = s.uniqueStrings(result.Summary.TargetHosts)

	return result, nil
}

// previewNode 预览节点执行
func (s *PreviewService) previewNode(node *models.WorkflowNode, context *WorkflowContext, step int) NodePreview {
	preview := NodePreview{
		Step:     step,
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: node.Type,
		Status:   "would_execute",
		Details:  make(map[string]interface{}),
	}

	switch node.Type {
	case "start":
		preview.Details["action"] = "开始执行工作流"

	case "condition":
		// 评估条件
		expression := node.Config["expression"].(string)
		result, evaluated := s.evaluateExpression(expression, context)
		
		preview.Details["expression"] = expression
		preview.Details["evaluated_expression"] = evaluated
		preview.Details["result"] = result
		
		if result {
			preview.Details["next_path"] = "true分支"
		} else {
			preview.Details["next_path"] = "false分支"
		}

	case "task":
		// 获取任务信息
		taskID := node.Config["task_id"]
		var task models.Task
		if id, ok := taskID.(float64); ok {
			if err := s.db.First(&task, uint(id)).Error; err == nil {
				preview.Details["task_name"] = task.Name
				preview.Details["task_type"] = task.TaskType
				
				// 解析变量
				variables := make(map[string]interface{})
				if varConfig, ok := node.Config["variables"].(map[string]interface{}); ok {
					for k, v := range varConfig {
						resolvedValue := s.resolveVariable(fmt.Sprint(v), context.GetAllVariables())
						variables[k] = resolvedValue
						
						// 记录变量来源
						context.VariableTrace[k] = VariableInfo{
							Value:  resolvedValue,
							Source: fmt.Sprint(v),
						}
					}
				}
				preview.Details["resolved_variables"] = variables
				
				// 预测目标主机
				if hosts, ok := s.predictTargetHosts(&task, variables); ok {
					preview.Details["target_hosts"] = hosts
				}
			}
		}

	case "parallel":
		branches := make([]string, 0)
		if branchIDs, ok := node.Config["branches"].([]interface{}); ok {
			for _, id := range branchIDs {
				branches = append(branches, fmt.Sprint(id))
			}
		}
		preview.Details["branches"] = branches
		preview.Details["execution_mode"] = "并行执行"

	case "end":
		preview.Details["action"] = "工作流执行完成"
	}

	return preview
}

// evaluateExpression 评估表达式
func (s *PreviewService) evaluateExpression(expression string, context *WorkflowContext) (bool, string) {
	// 替换变量
	evaluatedExpr := s.resolveVariable(expression, context.GetAllVariables())
	
	// 使用govaluate评估表达式
	expr, err := govaluate.NewEvaluableExpression(evaluatedExpr)
	if err != nil {
		return false, fmt.Sprintf("表达式解析错误: %v", err)
	}

	result, err := expr.Evaluate(context.GetAllVariables())
	if err != nil {
		return false, fmt.Sprintf("表达式评估错误: %v", err)
	}

	if boolResult, ok := result.(bool); ok {
		return boolResult, evaluatedExpr
	}

	return false, evaluatedExpr
}

// getNextNodeID 获取下一个节点ID
func (s *PreviewService) getNextNodeID(currentNode *models.WorkflowNode, preview NodePreview, workflow *ParsedWorkflow) string {
	// 对于条件节点，根据结果选择分支
	if currentNode.Type == "condition" {
		if result, ok := preview.Details["result"].(bool); ok {
			// 从配置中获取true/false分支
			if result {
				if trueBranch, ok := currentNode.Config["true_branch"].(string); ok && trueBranch != "" {
					return trueBranch
				}
			} else {
				if falseBranch, ok := currentNode.Config["false_branch"].(string); ok && falseBranch != "" {
					return falseBranch
				}
			}
		}
	}
	
	// 其他节点使用NextNodes列表
	if len(currentNode.NextNodes) > 0 {
		return currentNode.NextNodes[0]
	}
	
	return ""
}

// predictTargetHosts 预测目标主机
func (s *PreviewService) predictTargetHosts(task *models.Task, variables map[string]interface{}) ([]string, bool) {
	hosts := make([]string, 0)
	
	// 从变量中提取主机信息
	if hostVar, ok := variables["host"]; ok {
		hosts = append(hosts, fmt.Sprint(hostVar))
		return hosts, true
	}
	
	if hostsVar, ok := variables["hosts"]; ok {
		switch v := hostsVar.(type) {
		case []interface{}:
			for _, h := range v {
				hosts = append(hosts, fmt.Sprint(h))
			}
			return hosts, true
		case []string:
			return v, true
		case string:
			// 可能是逗号分隔的字符串
			parts := strings.Split(v, ",")
			for _, p := range parts {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					hosts = append(hosts, trimmed)
				}
			}
			return hosts, true
		}
	}

	// 从server_ip等常见字段提取
	if serverIP, ok := variables["server_ip"]; ok {
		hosts = append(hosts, fmt.Sprint(serverIP))
		return hosts, true
	}

	return hosts, false
}

// uniqueStrings 字符串数组去重
func (s *PreviewService) uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	
	for _, str := range strs {
		if !seen[str] {
			seen[str] = true
			result = append(result, str)
		}
	}
	
	return result
}

// WorkflowContext 工作流执行上下文
type WorkflowContext struct {
	Variables     map[string]interface{}
	TriggerData   map[string]interface{}
	VariableTrace map[string]VariableInfo
}

// GetAllVariables 获取所有变量
func (c *WorkflowContext) GetAllVariables() map[string]interface{} {
	all := make(map[string]interface{})
	
	// 先添加触发数据
	for k, v := range c.TriggerData {
		all[k] = v
	}
	
	// 再添加工作流变量（可能覆盖）
	for k, v := range c.Variables {
		all[k] = v
	}
	
	return all
}

// resolveVariable 解析变量
func (s *PreviewService) resolveVariable(template string, variables map[string]interface{}) string {
	result := template
	
	// 简单的变量替换实现
	for key, value := range variables {
		placeholder := fmt.Sprintf("${%s}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprint(value))
		
		// 也支持 {{key}} 格式
		placeholder = fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprint(value))
	}
	
	return result
}