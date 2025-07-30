package services

import (
	"ahop/internal/models"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ExecutionListItem 执行列表项（精简版）
type ExecutionListItem struct {
	ID           uint   `json:"id"`
	ExecutionID  string `json:"execution_id"`
	DisplayName  string `json:"display_name"`
	WorkflowName string `json:"workflow_name"`
	Status       string `json:"status"`
	StatusText   string `json:"status_text"`
	StartTime    string `json:"start_time"`
	Duration     string `json:"duration"`
	HasError     bool   `json:"has_error"`
}

// ExecutionDetailResponse 执行详情响应结构
type ExecutionDetailResponse struct {
	Basic    ExecutionBasicInfo    `json:"basic"`
	Workflow ExecutionWorkflowInfo `json:"workflow"`
	Rule     *ExecutionRuleInfo    `json:"rule,omitempty"`
	Trigger  ExecutionTriggerInfo  `json:"trigger"`
	Progress ExecutionProgressInfo `json:"progress"`
	Stats    ExecutionStatsInfo    `json:"stats"`
}

// ExecutionBasicInfo 基本信息
type ExecutionBasicInfo struct {
	ID          uint    `json:"id"`
	ExecutionID string  `json:"execution_id"`
	Status      string  `json:"status"`
	StartTime   string  `json:"start_time"`
	EndTime     *string `json:"end_time"`
	Duration    string  `json:"duration"`
	ErrorMsg    string  `json:"error_msg"`
}

// ExecutionWorkflowInfo 工作流信息
type ExecutionWorkflowInfo struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// ExecutionRuleInfo 规则信息
type ExecutionRuleInfo struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	TriggerType string `json:"trigger_type"`
	Cron        string `json:"cron,omitempty"`
}

// ExecutionTriggerInfo 触发信息
type ExecutionTriggerInfo struct {
	Type   string                 `json:"type"`
	Ticket *TriggerTicketInfo     `json:"ticket,omitempty"`
	User   *TriggerUserInfo       `json:"user,omitempty"`
	Extra  map[string]interface{} `json:"extra,omitempty"`
}

// TriggerTicketInfo 触发工单信息
type TriggerTicketInfo struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	AffectedHosts []string `json:"affected_hosts,omitempty"`
	DiskUsage     string   `json:"disk_usage,omitempty"`
	Partition     string   `json:"partition,omitempty"`
}

// TriggerUserInfo 触发用户信息
type TriggerUserInfo struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// ExecutionProgressInfo 执行进度信息
type ExecutionProgressInfo struct {
	CurrentStep int    `json:"current_step"`
	TotalSteps  int    `json:"total_steps"`
	CurrentNode string `json:"current_node"`
	Percentage  int    `json:"percentage"`
}

// ExecutionStatsInfo 执行统计信息
type ExecutionStatsInfo struct {
	NodesExecuted int `json:"nodes_executed"`
	NodesSuccess  int `json:"nodes_success"`
	NodesFailed   int `json:"nodes_failed"`
}

// ConvertToExecutionDetail 转换为执行详情响应
func ConvertToExecutionDetail(execution *models.HealingExecution, logs []models.HealingExecutionLog) *ExecutionDetailResponse {
	response := &ExecutionDetailResponse{
		Basic: ExecutionBasicInfo{
			ID:          execution.ID,
			ExecutionID: execution.ExecutionID,
			Status:      execution.Status,
			StartTime:   execution.StartTime.Format("2006-01-02 15:04:05"),
			ErrorMsg:    execution.ErrorMsg,
		},
		Workflow: ExecutionWorkflowInfo{
			ID:      execution.WorkflowID,
			Name:    "",
			Version: 0,
		},
		Trigger: ExecutionTriggerInfo{
			Type: execution.TriggerType,
		},
		Progress: ExecutionProgressInfo{
			CurrentStep: 0,
			TotalSteps:  0,
			CurrentNode: "",
			Percentage:  0,
		},
		Stats: ExecutionStatsInfo{
			NodesExecuted: 0,
			NodesSuccess:  0,
			NodesFailed:   0,
		},
	}

	// 设置结束时间和持续时间
	if execution.EndTime != nil {
		endTimeStr := execution.EndTime.Format("2006-01-02 15:04:05")
		response.Basic.EndTime = &endTimeStr
		response.Basic.Duration = formatExecutionDuration(execution.EndTime.Sub(execution.StartTime))
	} else {
		response.Basic.Duration = formatExecutionDuration(time.Since(execution.StartTime))
	}

	// 设置工作流信息
	if execution.Workflow.ID > 0 {
		response.Workflow.Name = execution.Workflow.Name
		response.Workflow.Version = execution.Workflow.Version
	}

	// 设置规则信息
	if execution.Rule != nil && execution.RuleID != nil {
		response.Rule = &ExecutionRuleInfo{
			ID:          execution.Rule.ID,
			Name:        execution.Rule.Name,
			TriggerType: execution.Rule.TriggerType,
			Cron:        execution.Rule.CronExpr,
		}
	}

	// 解析触发源信息
	if execution.TriggerSource != nil {
		parseTriggerSource(execution, response)
	}

	// 解析节点状态以计算进度
	if execution.NodeStates != nil {
		parseNodeStates(execution, response)
	} else {
		// 如果 NodeStates 为空，尝试从日志推断进度
		inferProgressFromLogs(logs, response)
	}

	// 从日志中统计执行情况
	calculateStats(logs, response)

	return response
}

// parseTriggerSource 解析触发源
func parseTriggerSource(execution *models.HealingExecution, response *ExecutionDetailResponse) {
	var triggerSource map[string]interface{}
	if err := json.Unmarshal(execution.TriggerSource, &triggerSource); err != nil {
		return
	}

	// 解析工单触发
	if matchedItem, ok := triggerSource["matched_item"].(map[string]interface{}); ok {
		ticket := &TriggerTicketInfo{}
		
		if id, ok := matchedItem["external_id"].(string); ok {
			ticket.ID = id
		}
		if title, ok := matchedItem["title"].(string); ok {
			ticket.Title = title
		}
		
		// 解析自定义数据
		if customData, ok := matchedItem["custom_data"].(map[string]interface{}); ok {
			if hosts, ok := customData["affected_hosts"].([]interface{}); ok {
				for _, host := range hosts {
					if h, ok := host.(string); ok {
						ticket.AffectedHosts = append(ticket.AffectedHosts, h)
					}
				}
			}
			if usage, ok := customData["disk_usage"].(float64); ok {
				ticket.DiskUsage = fmt.Sprintf("%.0f%%", usage)
			}
			if partition, ok := customData["partition"].(string); ok {
				ticket.Partition = partition
			}
		}
		
		response.Trigger.Ticket = ticket
	}

	// 解析用户触发
	if execution.TriggerUser != nil {
		response.Trigger.User = &TriggerUserInfo{
			ID:   *execution.TriggerUser,
			Name: fmt.Sprintf("用户%d", *execution.TriggerUser),
		}
	}
}

// parseNodeStates 解析节点状态
func parseNodeStates(execution *models.HealingExecution, response *ExecutionDetailResponse) {
	var nodeStates map[string]interface{}
	if err := json.Unmarshal(execution.NodeStates, &nodeStates); err != nil {
		return
	}

	// 计算总节点数和已执行节点数
	totalNodes := len(nodeStates)
	executedNodes := 0
	currentNode := ""
	
	for nodeID, state := range nodeStates {
		if stateMap, ok := state.(map[string]interface{}); ok {
			if status, ok := stateMap["status"].(string); ok {
				if status != "pending" {
					executedNodes++
				}
				if status == "running" {
					currentNode = nodeID
				}
			}
		}
	}

	response.Progress.TotalSteps = totalNodes
	response.Progress.CurrentStep = executedNodes
	response.Progress.CurrentNode = currentNode
	if totalNodes > 0 {
		response.Progress.Percentage = (executedNodes * 100) / totalNodes
	}
}

// inferProgressFromLogs 从日志推断进度
func inferProgressFromLogs(logs []models.HealingExecutionLog, response *ExecutionDetailResponse) {
	// 收集所有节点
	nodeSet := make(map[string]bool)
	nodeNames := make(map[string]string)
	nodeOrder := []string{}
	
	for _, log := range logs {
		if log.NodeID != "" && !nodeSet[log.NodeID] {
			nodeSet[log.NodeID] = true
			nodeOrder = append(nodeOrder, log.NodeID)
			if log.NodeName != "" {
				nodeNames[log.NodeID] = log.NodeName
			}
		}
	}
	
	// 找出最后执行的节点
	lastNodeID := ""
	lastNodeName := ""
	for i := len(logs) - 1; i >= 0; i-- {
		if logs[i].NodeID != "" {
			lastNodeID = logs[i].NodeID
			lastNodeName = logs[i].NodeName
			break
		}
	}
	
	// 计算进度
	response.Progress.TotalSteps = len(nodeSet)
	response.Progress.CurrentStep = len(nodeSet) // 如果能看到日志，说明节点都执行过了
	
	// 设置当前节点名称
	if lastNodeName != "" {
		response.Progress.CurrentNode = lastNodeName
	} else if lastNodeID != "" {
		response.Progress.CurrentNode = lastNodeID
	}
	
	// 计算百分比
	if response.Progress.TotalSteps > 0 {
		response.Progress.Percentage = (response.Progress.CurrentStep * 100) / response.Progress.TotalSteps
	}
}

// calculateStats 计算执行统计
func calculateStats(logs []models.HealingExecutionLog, response *ExecutionDetailResponse) {
	nodeResults := make(map[string]string)
	
	for _, log := range logs {
		if log.NodeID != "" {
			// 记录每个节点的最终状态
			if log.Level == models.LogLevelError {
				nodeResults[log.NodeID] = "failed"
			} else if log.Level == models.LogLevelInfo && nodeResults[log.NodeID] != "failed" {
				nodeResults[log.NodeID] = "success"
			}
		}
		
		// 更新当前节点（如果还没有设置）
		if response.Progress.CurrentNode == "" && log.NodeID != "" && log.NodeName != "" {
			response.Progress.CurrentNode = log.NodeName
		}
	}

	// 统计结果
	for _, status := range nodeResults {
		response.Stats.NodesExecuted++
		if status == "success" {
			response.Stats.NodesSuccess++
		} else if status == "failed" {
			response.Stats.NodesFailed++
		}
	}
}

// formatExecutionDuration 格式化执行持续时间
func formatExecutionDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		if seconds == 0 {
			return fmt.Sprintf("%d分钟", minutes)
		}
		return fmt.Sprintf("%d分%d秒", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%d小时", hours)
	}
	return fmt.Sprintf("%d小时%d分钟", hours, minutes)
}

// ConvertToExecutionListItem 转换为列表项
func ConvertToExecutionListItem(execution *models.HealingExecution) *ExecutionListItem {
	item := &ExecutionListItem{
		ID:          execution.ID,
		ExecutionID: execution.ExecutionID,
		Status:      execution.Status,
		StartTime:   execution.StartTime.Format("2006-01-02 15:04:05"),
		HasError:    execution.Status == models.ExecutionStatusFailed,
	}

	// 设置持续时间
	if execution.EndTime != nil {
		item.Duration = formatExecutionDuration(execution.EndTime.Sub(execution.StartTime))
	} else {
		item.Duration = formatExecutionDuration(time.Since(execution.StartTime))
	}

	// 设置工作流名称
	if execution.Workflow.ID > 0 {
		item.WorkflowName = execution.Workflow.Name
	}

	// 生成显示名称和状态文本
	generateDisplayInfo(execution, item)

	return item
}

// generateDisplayInfo 生成显示信息
func generateDisplayInfo(execution *models.HealingExecution, item *ExecutionListItem) {
	var displayName strings.Builder
	
	// 基础名称
	if execution.Workflow.ID > 0 {
		displayName.WriteString(execution.Workflow.Name)
	} else {
		displayName.WriteString("自愈任务")
	}

	// 添加触发信息
	if execution.TriggerSource != nil {
		var triggerSource map[string]interface{}
		if err := json.Unmarshal(execution.TriggerSource, &triggerSource); err == nil {
			// 尝试获取工单信息
			if matchedItem, ok := triggerSource["matched_item"].(map[string]interface{}); ok {
				if ticketID, ok := matchedItem["external_id"].(string); ok {
					displayName.WriteString(" - 工单")
					displayName.WriteString(ticketID)
				}
			}
		}
	}

	item.DisplayName = displayName.String()

	// 生成状态文本
	switch execution.Status {
	case models.ExecutionStatusRunning:
		item.StatusText = "正在执行"
		// 尝试从上下文获取当前节点信息
		if execution.Context != nil {
			var context map[string]interface{}
			if err := json.Unmarshal(execution.Context, &context); err == nil {
				if currentNode, ok := context["current_node"].(string); ok {
					item.StatusText = fmt.Sprintf("正在执行: %s", currentNode)
				}
			}
		}
	case models.ExecutionStatusSuccess:
		item.StatusText = "执行成功"
	case models.ExecutionStatusFailed:
		item.StatusText = "执行失败"
		if execution.ErrorMsg != "" {
			// 截取错误信息的前50个字符
			errMsg := execution.ErrorMsg
			if len(errMsg) > 50 {
				errMsg = errMsg[:50] + "..."
			}
			item.StatusText = fmt.Sprintf("失败: %s", errMsg)
		}
	case models.ExecutionStatusCancelled:
		item.StatusText = "已取消"
	default:
		item.StatusText = execution.Status
	}
}