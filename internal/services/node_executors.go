package services

import (
	"ahop/internal/models"
	"ahop/pkg/logger"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// StartNodeExecutor 开始节点执行器
type StartNodeExecutor struct{}

func (e *StartNodeExecutor) Execute(node *models.WorkflowNode, context *ExecutionContext) (*NodeResult, error) {
	// 开始节点只是标记工作流开始，不做实际操作
	return &NodeResult{
		Status: "success",
		Output: map[string]interface{}{
			"message": "工作流开始执行",
			"time":    time.Now().Format(time.RFC3339),
		},
	}, nil
}

// EndNodeExecutor 结束节点执行器
type EndNodeExecutor struct{}

func (e *EndNodeExecutor) Execute(node *models.WorkflowNode, context *ExecutionContext) (*NodeResult, error) {
	// 结束节点标记工作流结束
	return &NodeResult{
		Status: "success",
		Output: map[string]interface{}{
			"message": "工作流执行结束",
			"time":    time.Now().Format(time.RFC3339),
		},
	}, nil
}


// ConditionNodeExecutor 条件判断节点执行器
type ConditionNodeExecutor struct{}

func (e *ConditionNodeExecutor) Execute(node *models.WorkflowNode, context *ExecutionContext) (*NodeResult, error) {
	if node.Config == nil {
		return nil, errors.New("条件节点缺少配置")
	}

	expression, ok := node.Config["expression"].(string)
	if !ok {
		return nil, errors.New("条件节点必须有表达式")
	}

	// 评估条件表达式
	result, err := e.evaluateExpression(expression, context.Variables)
	if err != nil {
		return nil, fmt.Errorf("评估条件失败: %v", err)
	}

	// 使用 next_nodes 字段处理分支
	// next_nodes[0] = true分支, next_nodes[1] = false分支
	var nextNodes []string
	if result {
		nextNodes = []string{node.NextNodes[0]}
	} else {
		nextNodes = []string{node.NextNodes[1]}
	}

	// 添加更多上下文信息到输出
	output := map[string]interface{}{
		"expression": expression,
		"result":     result,
		"next":       nextNodes,
	}
	
	// 对于特定的表达式，添加实际值信息
	if expression == "host_count > 0" {
		if hostCount, ok := context.Variables["host_count"]; ok {
			output["host_count"] = hostCount
		}
		if affectedHosts, ok := context.Variables["affected_hosts"]; ok {
			output["affected_hosts"] = affectedHosts
		}
	}
	
	return &NodeResult{
		Status:    "success",
		Output:    output,
		NextNodes: nextNodes,
	}, nil
}

func (e *ConditionNodeExecutor) evaluateExpression(expression string, variables map[string]interface{}) (bool, error) {
	// TODO: 实现完整的表达式评估器
	// 这里只实现简单的示例逻辑

	// 示例1: 检查变量是否存在
	if expression == "len(tickets) > 0" {
		if tickets, ok := variables["tickets"].([]models.Ticket); ok {
			return len(tickets) > 0, nil
		}
		return false, nil
	}

	// 示例2: 检查任务结果
	if expression == `task_result.status == "success"` {
		if taskResult, ok := variables["task_result"].(map[string]interface{}); ok {
			if status, ok := taskResult["status"].(string); ok {
				return status == "success", nil
			}
		}
		return false, nil
	}
	
	// 检查host_count > 0
	if expression == "host_count > 0" {
		if hostCount, ok := variables["host_count"].(int); ok {
			return hostCount > 0, nil
		}
		if hostCount, ok := variables["host_count"].(float64); ok {
			return hostCount > 0, nil
		}
		return false, nil
	}
	
	// 检查cleanup_result.summary.success > 0
	if expression == "cleanup_result.summary.success > 0" {
		if cleanupResult, ok := variables["cleanup_result"].(map[string]interface{}); ok {
			if summary, ok := cleanupResult["summary"].(map[string]interface{}); ok {
				if success, ok := summary["success"].(int); ok {
					return success > 0, nil
				}
				if success, ok := summary["success"].(float64); ok {
					return success > 0, nil
				}
			}
		}
		return false, nil
	}

	// 默认返回false，表示条件不满足
	return false, fmt.Errorf("不支持的表达式: %s", expression)
}

// DataProcessNodeExecutor 数据处理节点执行器
type DataProcessNodeExecutor struct{
	jsonPath     *JSONPath
	transformer  *DataTransformer
}

func (e *DataProcessNodeExecutor) Execute(node *models.WorkflowNode, context *ExecutionContext) (*NodeResult, error) {
	if node.Config == nil {
		return nil, errors.New("数据处理节点缺少配置")
	}

	// 初始化组件
	if e.jsonPath == nil {
		e.jsonPath = NewJSONPath()
	}
	if e.transformer == nil {
		e.transformer = NewDataTransformer()
	}

	processed := make(map[string]interface{})

	// 变量提取
	if extract, ok := node.Config["extract"].(map[string]interface{}); ok {
		for targetVar, sourcePath := range extract {
			pathStr, ok := sourcePath.(string)
			if !ok {
				continue
			}
			
			value := e.jsonPath.Extract(pathStr, context.Variables)
			if value != nil {
				context.Variables[targetVar] = value
				processed[targetVar] = value
			}
		}
	}

	// 数据转换
	if transform, ok := node.Config["transform"].(map[string]interface{}); ok {
		for varName, transformation := range transform {
			transformed := e.transformer.Transform(transformation, context.Variables)
			if transformed != nil {
				context.Variables[varName] = transformed
				processed[varName] = transformed
			}
		}
	}

	return &NodeResult{
		Status: "success",
		Output: map[string]interface{}{
			"processed": true,
			"results":   processed,
		},
	}, nil
}

// TaskExecuteNodeExecutor 任务执行节点执行器
type TaskExecuteNodeExecutor struct {
	taskService      *TaskService
	variableResolver *VariableResolver
	hostService      *HostService
	workflowExecutor *WorkflowExecutor  // 添加引用以便记录日志
}

// logExecution 记录执行日志
func (e *TaskExecuteNodeExecutor) logExecution(executionID uint, nodeID string, level string, message string, 
	nodeType string, nodeName string, output map[string]interface{}) {
	
	// 如果有 workflowExecutor，使用它的详细日志方法
	if e.workflowExecutor != nil {
		now := time.Now()
		e.workflowExecutor.logExecutionWithDetails(executionID, nodeID, level, message, 
			nodeType, nodeName, &now, &now, nil, output, nil)
		return
	}
	
	// 否则直接记录到数据库
	now := time.Now()
	log := &models.HealingExecutionLog{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Level:       level,
		Message:     message,
		Timestamp:   now,
		NodeType:    nodeType,
		NodeName:    nodeName,
		StartTime:   &now,
		EndTime:     &now,
		Duration:    1, // 最小1毫秒
	}
	
	if output != nil {
		outputJSON, _ := json.Marshal(output)
		log.Output = outputJSON
	}
	
	// 如果有数据库连接，记录到数据库
	if e.taskService != nil && e.taskService.db != nil {
		if err := e.taskService.db.Create(log).Error; err != nil {
			logger.GetLogger().WithError(err).Error("记录执行日志失败")
		}
	}
}

func (e *TaskExecuteNodeExecutor) Execute(node *models.WorkflowNode, context *ExecutionContext) (*NodeResult, error) {
	if node.Config == nil {
		return nil, errors.New("任务执行节点缺少配置")
	}

	// 记录节点开始时间（从上下文获取）
	nodeStartTime := time.Now()
	if nodeState, exists := context.NodeStates[node.ID]; exists && nodeState.StartTime != nil {
		nodeStartTime = *nodeState.StartTime
	}

	// 初始化组件
	if e.variableResolver == nil {
		e.variableResolver = NewVariableResolver()
	}
	if e.hostService == nil {
		e.hostService = NewHostService(e.taskService.db)
	}

	// 获取模板ID
	var templateID uint
	if v, ok := node.Config["template_id"]; ok {
		switch val := v.(type) {
		case float64:
			templateID = uint(val)
		case int:
			templateID = uint(val)
		case string:
			id, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("无效的模板ID: %v", val)
			}
			templateID = uint(id)
		default:
			return nil, fmt.Errorf("模板ID类型错误: %T", val)
		}
	} else {
		return nil, errors.New("任务执行节点必须指定template_id")
	}

	// 获取主机配置
	hostsConfig, ok := node.Config["hosts"]
	if !ok {
		return nil, errors.New("任务执行节点必须指定hosts")
	}

	// 解析主机配置（支持变量引用）
	resolvedHosts := e.variableResolver.Resolve(hostsConfig, context.Variables)
	
	// 转换为字符串数组
	var hostIdentifiers []string
	switch v := resolvedHosts.(type) {
	case []string:
		hostIdentifiers = v
	case []interface{}:
		for _, h := range v {
			if str, ok := h.(string); ok {
				hostIdentifiers = append(hostIdentifiers, str)
			}
		}
	case string:
		// 单个主机
		hostIdentifiers = []string{v}
	default:
		return nil, fmt.Errorf("无效的主机配置类型: %T", v)
	}

	if len(hostIdentifiers) == 0 {
		return nil, errors.New("主机列表为空")
	}

	// 获取主机匹配方式
	hostMatchBy, _ := node.Config["host_match_by"].(string)
	if hostMatchBy != "ip" && hostMatchBy != "hostname" {
		return nil, errors.New("host_match_by 必须是 'ip' 或 'hostname'")
	}

	// 查询主机信息（使用新方法获取详细信息）
	hosts, notFound, err := e.hostService.FindHostsByIdentifiersWithDetails(context.Execution.TenantID, hostIdentifiers, hostMatchBy)
	if err != nil {
		return nil, fmt.Errorf("查询主机失败: %v", err)
	}

	// 记录未找到的主机
	var warningMsg string
	if len(notFound) > 0 {
		warningMsg = fmt.Sprintf("以下%s未找到对应主机: %s", hostMatchBy, strings.Join(notFound, ", "))
		context.Logger.Warn(warningMsg)
		
		// 记录到节点输出中（使用 workflowExecutor 的详细日志方法）
		// 构建简化的找到的主机列表（只包含IP）
		var foundHostIPs []string
		for _, host := range hosts {
			foundHostIPs = append(foundHostIPs, host.IPAddress)
		}
		
		// 使用统一的输出结构
		warningOutput := map[string]interface{}{
			"hosts": map[string]interface{}{
				"requested": hostIdentifiers,
				"found":     foundHostIPs,
				"not_found": notFound,
				"count": map[string]interface{}{
					"requested": len(hostIdentifiers),
					"found":     len(hosts),
				},
			},
			"task": map[string]interface{}{
				"template_id": templateID,
			},
		}
		
		if e.workflowExecutor != nil {
			now := time.Now()
			e.workflowExecutor.logExecutionWithDetails(context.Execution.ID, node.ID, models.LogLevelWarn,
				warningMsg, node.Type, node.Name, &nodeStartTime, &now, nil, warningOutput, nil)
		} else {
			e.logExecution(context.Execution.ID, node.ID, models.LogLevelWarn,
				warningMsg, node.Type, node.Name, warningOutput)
		}
	}

	if len(hosts) == 0 {
		// 记录详细的错误信息
		errorDetails := map[string]interface{}{
			"hosts": map[string]interface{}{
				"requested": hostIdentifiers,
				"found":     []string{},
				"not_found": hostIdentifiers, // 所有请求的主机都未找到
				"count": map[string]interface{}{
					"requested": len(hostIdentifiers),
					"found":     0,
				},
			},
			"task": map[string]interface{}{
				"template_id": templateID,
			},
			"error": map[string]interface{}{
				"match_by": hostMatchBy,
				"message":  "数据库中没有找到任何匹配的主机",
			},
		}
		if e.workflowExecutor != nil {
			now := time.Now()
			e.workflowExecutor.logExecutionWithDetails(context.Execution.ID, node.ID, models.LogLevelError,
				fmt.Sprintf("未找到任何主机，无法执行任务。请求的主机: %v", hostIdentifiers),
				node.Type, node.Name, &nodeStartTime, &now, nil, errorDetails, nil)
		} else {
			e.logExecution(context.Execution.ID, node.ID, models.LogLevelError,
				fmt.Sprintf("未找到任何主机，无法执行任务。请求的主机: %v", hostIdentifiers),
				node.Type, node.Name, errorDetails)
		}
		
		return nil, fmt.Errorf("未找到任何主机，无法执行任务")
	}

	// 构建主机ID列表（Worker只需要ID）
	var hostIDs []interface{}
	for _, host := range hosts {
		hostIDs = append(hostIDs, host.ID)
	}

	// 获取任务参数中的变量配置
	variables, _ := node.Config["variables"].(map[string]interface{})
	if variables == nil {
		variables = make(map[string]interface{})
	}
	
	// 解析变量中的引用
	resolvedVars := e.variableResolver.ResolveAll(variables, context.Variables)
	vars, ok := resolvedVars.(map[string]interface{})
	if !ok {
		vars = make(map[string]interface{})
	}

	// 获取超时设置
	timeout := 3600 // 默认1小时
	if t, ok := node.Config["timeout"].(float64); ok {
		timeout = int(t)
	}

	// 创建任务参数
	params := map[string]interface{}{
		"hosts":       hostIDs,      // 只传递主机ID数组
		"variables":   vars,         // 传递解析后的变量
		"timeout":     timeout,
		"template_id": templateID,   // 传递模板ID
	}

	// 构建找到的主机信息（如果还没构建）
	var foundHosts []map[string]interface{}
	for _, host := range hosts {
		foundHosts = append(foundHosts, map[string]interface{}{
			"id": host.ID,
			"name": host.Name,
			"ip": host.IPAddress,
		})
	}
	
	// 记录找到的主机信息
	// 构建简化的找到的主机列表（只包含IP）
	var foundHostIPs []string
	for _, host := range hosts {
		foundHostIPs = append(foundHostIPs, host.IPAddress)
	}
	
	// 构建清晰的输出结构
	outputData := map[string]interface{}{
		"hosts": map[string]interface{}{
			"requested": hostIdentifiers,
			"found":     foundHostIPs,
			"not_found": notFound,
			"count": map[string]interface{}{
				"requested": len(hostIdentifiers),
				"found":     len(hosts),
			},
		},
		"task": map[string]interface{}{
			"template_id": templateID,
			"variables":   vars,  // 包含解析后的变量
		},
	}
	
	if e.workflowExecutor != nil {
		now := time.Now()
		e.workflowExecutor.logExecutionWithDetails(context.Execution.ID, node.ID, models.LogLevelInfo,
			fmt.Sprintf("找到 %d 台主机，准备执行任务", len(hosts)),
			node.Type, node.Name, &nodeStartTime, &now, nil, outputData, nil)
	} else {
		e.logExecution(context.Execution.ID, node.ID, models.LogLevelInfo,
			fmt.Sprintf("找到 %d 台主机，准备执行任务", len(hosts)),
			node.Type, node.Name, outputData)
	}

	// 从global_context中获取工单信息
	var ticketInfo string
	if gc, ok := context.Variables["global_context"].(map[string]interface{}); ok {
		if trigger, ok := gc["trigger"].(map[string]interface{}); ok {
			if ticket, ok := trigger["ticket"].(map[string]interface{}); ok {
				// 从工单数据中获取ID和标题
				ticketID := ""
				ticketTitle := ""
				
				if id, ok := ticket["ticket_id"].(string); ok {
					ticketID = id
				}
				if title, ok := ticket["title"].(string); ok {
					ticketTitle = title
				}
				
				if ticketID != "" {
					if ticketTitle != "" {
						ticketInfo = fmt.Sprintf(" [工单:%s-%s]", ticketID, ticketTitle)
					} else {
						ticketInfo = fmt.Sprintf(" [工单:%s]", ticketID)
					}
				}
			}
		}
	}
	
	// 创建任务，在任务名称中包含工单信息
	taskName := fmt.Sprintf("自愈工作流任务 - %s%s", node.Name, ticketInfo)
	taskDesc := fmt.Sprintf("由工作流 %s 创建", context.Workflow.Name)
	if ticketInfo != "" {
		taskDesc += ticketInfo
	}
	
	// 创建任务
	task, err := e.taskService.CreateTemplateTask(
		context.Execution.TenantID,
		templateID,
		taskName,
		taskDesc,
		params,
		5, // 优先级
	)
	if err != nil {
		return nil, fmt.Errorf("创建任务失败: %v", err)
	}
	
	// 记录任务创建成功
	if e.workflowExecutor != nil {
		now := time.Now()
		e.workflowExecutor.logExecutionWithDetails(context.Execution.ID, node.ID, models.LogLevelInfo,
			fmt.Sprintf("任务创建成功，任务ID: %s", task.TaskID),
			node.Type, node.Name, &nodeStartTime, &now, nil, map[string]interface{}{
				"task_id": task.TaskID,
				"template_id": templateID,
				"hosts": hostIDs,
			}, nil)
	} else {
		e.logExecution(context.Execution.ID, node.ID, models.LogLevelInfo,
			fmt.Sprintf("任务创建成功，任务ID: %s", task.TaskID),
			node.Type, node.Name, map[string]interface{}{
				"task_id": task.TaskID,
				"template_id": templateID,
				"hosts": hostIDs,
			})
	}

	// 等待任务完成
	log := logger.GetLogger()
	log.Infof("等待任务 %s 完成...", task.TaskID)

	// 轮询任务状态，最多等待 timeout 秒
	maxWaitTime := time.Duration(timeout) * time.Second
	startTime := time.Now()
	checkInterval := 2 * time.Second
	
	var completedTask *models.Task
	for {
		// 检查是否超时
		if time.Since(startTime) > maxWaitTime {
			return nil, fmt.Errorf("任务执行超时，任务ID: %s", task.TaskID)
		}
		
		// 获取任务状态
		currentTask, err := e.taskService.GetTask(task.TaskID, context.Execution.TenantID)
		if err != nil {
			return nil, fmt.Errorf("获取任务状态失败: %v", err)
		}
		
		// 检查任务是否完成
		if currentTask.Status == "success" || currentTask.Status == "failed" || currentTask.Status == "cancelled" {
			completedTask = currentTask
			break
		}
		
		// 等待后继续检查
		time.Sleep(checkInterval)
	}
	
	// 解析任务结果
	var taskSummary map[string]interface{}
	if completedTask.Result != nil {
		// 首先尝试从 Result 中获取 summary
		var result map[string]interface{}
		if err := json.Unmarshal(completedTask.Result, &result); err == nil {
			if summary, ok := result["summary"].(map[string]interface{}); ok {
				// 处理两种可能的字段名格式
				success := 0
				failed := 0
				total := 0
				
				// 尝试获取 success/failed/total
				if val, ok := summary["success"].(float64); ok {
					success = int(val)
				}
				if val, ok := summary["failed"].(float64); ok {
					failed = int(val)
				}
				if val, ok := summary["total"].(float64); ok {
					total = int(val)
				}
				
				// 如果上面没有获取到，尝试 success_hosts/failed_hosts/total_hosts
				if success == 0 && failed == 0 && total == 0 {
					if val, ok := summary["success_hosts"].(float64); ok {
						success = int(val)
					}
					if val, ok := summary["failed_hosts"].(float64); ok {
						failed = int(val)
					}
					if val, ok := summary["total_hosts"].(float64); ok {
						total = int(val)
					}
				}
				
				taskSummary = map[string]interface{}{
					"success": success,
					"failed":  failed,
					"total":   total,
				}
			}
		}
		
		// 如果上面的方法失败，尝试解析为 TaskTemplateResult
		if taskSummary == nil {
			var templateResult models.TaskTemplateResult
			if err := json.Unmarshal(completedTask.Result, &templateResult); err == nil {
				taskSummary = map[string]interface{}{
					"success": templateResult.SuccessHosts,
					"failed":  templateResult.FailedHosts,
					"total":   templateResult.TotalHosts,
				}
			}
		}
	}
	
	// 构建任务结果
	taskResult := map[string]interface{}{
		"task_id": task.TaskID,
		"status":  completedTask.Status,
		"hosts_found": len(hosts),
		"hosts_requested": len(hostIdentifiers),
	}
	
	// 添加执行摘要
	if taskSummary != nil {
		// 如果有主机未找到，需要调整统计
		if len(notFound) > 0 && taskSummary["total"] != nil {
			// 实际总数应该包括未找到的主机
			actualTotal := taskSummary["total"].(int) + len(notFound)
			actualFailed := taskSummary["failed"].(int) + len(notFound)
			
			taskSummary["total"] = actualTotal
			taskSummary["failed"] = actualFailed
			taskSummary["not_found"] = len(notFound)
		}
		taskResult["summary"] = taskSummary
	}
	
	// 如果有未找到的主机，添加到结果中
	if len(notFound) > 0 {
		taskResult["hosts_not_found"] = notFound
		taskResult["warning"] = warningMsg
	}
	
	// 添加任务执行详情（用于工单更新）
	if completedTask.Result != nil {
		var result map[string]interface{}
		if err := json.Unmarshal(completedTask.Result, &result); err == nil {
			// 保存原始结果，以便工单更新时使用
			taskResult["execution_details"] = result
		}
	}
	
	// 获取输出变量名（如果配置了）
	outputVar := "task_result"  // 默认变量名
	if outVar, ok := node.Config["output"].(string); ok && outVar != "" {
		outputVar = outVar
	}
	
	// 保存任务结果到指定的变量
	context.Variables[outputVar] = taskResult

	// 构建输出结果
	output := map[string]interface{}{
		"task_id": task.TaskID,
		"status":  "completed",
		"hosts_found": len(hosts),
		"hosts_requested": len(hostIdentifiers),
	}
	
	if len(notFound) > 0 {
		output["hosts_not_found"] = notFound
		output["warning"] = warningMsg
	}

	return &NodeResult{
		Status: "success",
		Output: output,
	}, nil
}

// ControlNodeExecutor 控制节点执行器
type ControlNodeExecutor struct{}

func (e *ControlNodeExecutor) Execute(node *models.WorkflowNode, context *ExecutionContext) (*NodeResult, error) {
	if node.Config == nil {
		return nil, errors.New("控制节点缺少配置")
	}

	action, ok := node.Config["action"].(string)
	if !ok {
		return nil, errors.New("控制节点必须指定动作")
	}

	switch action {
	case "wait":
		return e.executeWait(node, context)
	case "terminate":
		return e.executeTerminate(node, context)
	default:
		return nil, fmt.Errorf("不支持的控制动作: %s", action)
	}
}

func (e *ControlNodeExecutor) executeWait(node *models.WorkflowNode, context *ExecutionContext) (*NodeResult, error) {
	duration := 10 // 默认10秒
	if d, ok := node.Config["duration"].(float64); ok {
		duration = int(d)
	}

	time.Sleep(time.Duration(duration) * time.Second)

	return &NodeResult{
		Status: "success",
		Output: map[string]interface{}{
			"action":   "wait",
			"duration": duration,
		},
	}, nil
}

func (e *ControlNodeExecutor) executeTerminate(node *models.WorkflowNode, context *ExecutionContext) (*NodeResult, error) {
	reason, _ := node.Config["reason"].(string)
	
	return &NodeResult{
		Status: "success",
		Output: map[string]interface{}{
			"action": "terminate",
			"reason": reason,
		},
		NextNodes: []string{}, // 不再执行后续节点
	}, nil
}

// TicketUpdateNodeExecutor 工单更新节点执行器
type TicketUpdateNodeExecutor struct {
	ticketService    *TicketService
	variableResolver *VariableResolver
}

func (e *TicketUpdateNodeExecutor) Execute(node *models.WorkflowNode, context *ExecutionContext) (*NodeResult, error) {
	if node.Config == nil {
		return nil, errors.New("工单更新节点缺少配置")
	}

	// 初始化组件
	if e.variableResolver == nil {
		e.variableResolver = NewVariableResolver()
	}

	// 获取工单对象
	var ticketID uint
	
	// 优先从变量获取工单
	if ticketVar, ok := node.Config["ticket_var"].(string); ok {
		ticketObj := context.Variables[ticketVar]
		switch v := ticketObj.(type) {
		case models.Ticket:
			ticketID = v.ID
		case *models.Ticket:
			ticketID = v.ID
		case map[string]interface{}:
			// 从map中提取ID
			if id, ok := v["id"].(float64); ok {
				ticketID = uint(id)
			}
		}
	}
	
	// 备选：从配置中获取工单ID（可能是数字或变量引用）
	if ticketID == 0 {
		if idConfig := node.Config["ticket_id"]; idConfig != nil {
			switch v := idConfig.(type) {
			case float64:
				ticketID = uint(v)
			case string:
				// 解析变量引用
				resolved := e.variableResolver.Resolve(v, context.Variables)
				switch id := resolved.(type) {
				case float64:
					ticketID = uint(id)
				case int:
					ticketID = uint(id)
				case uint:
					ticketID = id
				}
			}
		}
	}

	if ticketID == 0 {
		return nil, errors.New("未找到要更新的工单")
	}

	// 获取更新内容配置
	updatesConfig, ok := node.Config["updates"].(map[string]interface{})
	if !ok || updatesConfig == nil {
		return nil, errors.New("工单更新节点必须指定更新内容")
	}

	// 解析更新内容中的变量
	resolvedUpdates := e.variableResolver.ResolveAll(updatesConfig, context.Variables)
	updates, ok := resolvedUpdates.(map[string]interface{})
	if !ok {
		return nil, errors.New("解析更新内容失败")
	}

	// 处理 include_logs 选项
	if comment, ok := updates["comment"].(map[string]interface{}); ok {
		if includeLogs, ok := comment["include_logs"].(bool); ok && includeLogs {
			// 获取执行日志
			executionLogs := e.buildExecutionLogs(context)
			
			// 将日志添加到评论模板中
			if template, ok := comment["template"].(string); ok {
				// 添加执行日志部分
				template += "\n\n【执行日志】\n" + executionLogs
				comment["template"] = template
			}
		}
	}

	// 调用工单服务更新外部工单
	if err := e.ticketService.UpdateExternalTicket(ticketID, updates); err != nil {
		// 记录错误但不阻止工作流
		context.Logger.WithError(err).Errorf("更新工单失败: ticketID=%d", ticketID)
		
		return &NodeResult{
			Status: "success", // 即使更新失败也返回成功，避免阻塞工作流
			Output: map[string]interface{}{
				"ticket_id": ticketID,
				"updated":   false,
				"error":     err.Error(),
			},
		}, nil
	}

	return &NodeResult{
		Status: "success",
		Output: map[string]interface{}{
			"ticket_id": ticketID,
			"updated":   true,
			"updates":   updates,
		},
	}, nil
}

// buildExecutionLogs 构建执行日志
func (e *TicketUpdateNodeExecutor) buildExecutionLogs(context *ExecutionContext) string {
	var logs strings.Builder
	
	// 1. 添加执行参数信息
	if cleanupResult, ok := context.Variables["cleanup_result"].(map[string]interface{}); ok {
		// 获取任务执行详情
		if details, ok := cleanupResult["execution_details"].(map[string]interface{}); ok {
			// 添加脚本信息
			if template, ok := details["template"].(map[string]interface{}); ok {
				logs.WriteString(fmt.Sprintf("执行脚本: %s (%s)\n", 
					template["name"], template["code"]))
			}
			
			// 添加执行参数
			logs.WriteString("\n执行参数:\n")
			paramFound := false
			
			// 方法1：从 context.NodeStates 中查找
			if taskID, ok := cleanupResult["task_id"].(string); ok {
				for _, nodeState := range context.NodeStates {
					if nodeState != nil && nodeState.Output != nil {
						output := nodeState.Output
						if tid, ok := output["task_id"].(string); ok && tid == taskID {
							// 找到了对应的任务节点，提取参数
							if task, ok := output["task"].(map[string]interface{}); ok {
								if vars, ok := task["variables"].(map[string]interface{}); ok {
									paramFound = true
									for k, v := range vars {
										logs.WriteString(fmt.Sprintf("  - %s: %v\n", k, v))
									}
								}
							}
							break
						}
					}
				}
			}
			
			// 方法2：如果上面没找到，尝试从工作流节点配置中获取
			if !paramFound {
				// 查找执行清理的节点
				for nodeID := range context.NodeStates {
					if strings.Contains(nodeID, "cleanup") || strings.Contains(nodeID, "execute") {
						// 从工作流定义中获取该节点的配置
						if workflow := context.Workflow; workflow != nil && workflow.Definition != nil {
							var def map[string]interface{}
							if err := json.Unmarshal(workflow.Definition, &def); err == nil {
								if nodes, ok := def["nodes"].([]interface{}); ok {
									for _, n := range nodes {
										if node, ok := n.(map[string]interface{}); ok {
											if nid, ok := node["id"].(string); ok && nid == nodeID {
												if config, ok := node["config"].(map[string]interface{}); ok {
													if vars, ok := config["variables"].(map[string]interface{}); ok {
														paramFound = true
														logs.WriteString("  （从工作流配置获取）\n")
														for k, v := range vars {
															logs.WriteString(fmt.Sprintf("  - %s: %v\n", k, v))
														}
													}
												}
												break
											}
										}
									}
								}
							}
						}
						if paramFound {
							break
						}
					}
				}
			}
			
			if !paramFound {
				logs.WriteString("  （未找到执行参数）\n")
			}
			
			// 添加主机执行结果
			logs.WriteString("\n主机执行结果:\n")
			
			// 先添加未找到的主机信息
			if notFound, ok := cleanupResult["hosts_not_found"].([]interface{}); ok && len(notFound) > 0 {
				logs.WriteString("\n【未找到的主机】\n")
				for _, host := range notFound {
					if hostStr, ok := host.(string); ok {
						logs.WriteString(fmt.Sprintf("- %s: 主机未在数据库中找到\n", hostStr))
					}
				}
			}
			
			// 然后添加执行成功/失败的主机信息
			if hosts, ok := details["hosts"].(map[string]interface{}); ok {
				for hostIP, hostResult := range hosts {
					if result, ok := hostResult.(map[string]interface{}); ok {
						logs.WriteString(fmt.Sprintf("\n=== %s ===\n", hostIP))
						
						// 状态
						if success, ok := result["success"].(bool); ok {
							if success {
								logs.WriteString("状态: 成功\n")
							} else {
								logs.WriteString("状态: 失败\n")
								// 如果失败，显示失败原因
								if msg, ok := result["message"].(string); ok {
									logs.WriteString(fmt.Sprintf("失败原因: %s\n", msg))
								}
							}
						}
						
						// 退出码
						if exitCode, ok := result["exit_code"].(float64); ok {
							logs.WriteString(fmt.Sprintf("退出码: %d\n", int(exitCode)))
						}
						
						// 输出内容（最多5000行）
						if output, ok := result["output"].(string); ok {
							logs.WriteString("\n执行输出:\n")
							lines := strings.Split(output, "\n")
							if len(lines) > 5000 {
								// 只保留前5000行
								for i := 0; i < 5000; i++ {
									logs.WriteString(lines[i])
									logs.WriteString("\n")
								}
								logs.WriteString(fmt.Sprintf("\n... (输出已截断，共 %d 行)\n", len(lines)))
							} else {
								logs.WriteString(output)
								if !strings.HasSuffix(output, "\n") {
									logs.WriteString("\n")
								}
							}
						}
						
						// 错误信息
						if stderr, ok := result["stderr"].(string); ok && stderr != "" {
							logs.WriteString("\n错误输出:\n")
							logs.WriteString(stderr)
							logs.WriteString("\n")
						}
					}
				}
			}
		}
	}
	
	// 如果没有找到详细日志，返回简单信息
	if logs.Len() == 0 {
		logs.WriteString("（未找到详细执行日志）")
	}
	
	return logs.String()
}

