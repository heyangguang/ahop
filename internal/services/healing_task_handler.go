package services

import (
	"ahop/internal/models"
	"ahop/pkg/logger"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
)

// HealingTaskHandler 处理自愈工作流任务
type HealingTaskHandler struct {
	db               *gorm.DB
	workflowExecutor *WorkflowExecutor
}

// NewHealingTaskHandler 创建自愈任务处理器
func NewHealingTaskHandler(db *gorm.DB, taskService *TaskService, ticketService *TicketService) *HealingTaskHandler {
	return &HealingTaskHandler{
		db:               db,
		workflowExecutor: NewWorkflowExecutor(db, taskService, ticketService),
	}
}

// HandleHealingWorkflowTask 处理自愈工作流任务
func (h *HealingTaskHandler) HandleHealingWorkflowTask(task *models.Task) error {
	log := logger.GetLogger()
	log.Infof("开始处理自愈工作流任务: %s", task.TaskID)
	
	// 解析任务参数
	var params models.TaskParams
	if err := json.Unmarshal(task.Params, &params); err != nil {
		return fmt.Errorf("解析任务参数失败: %v", err)
	}
	
	// 从参数中提取变量
	vars := params.Variables
	if vars == nil {
		return fmt.Errorf("任务参数缺少变量")
	}
	
	// 获取工作流ID
	workflowIDFloat, ok := vars["workflow_id"].(float64)
	if !ok {
		return fmt.Errorf("任务参数缺少workflow_id")
	}
	workflowID := uint(workflowIDFloat)
	
	// 获取工作流
	var workflow models.HealingWorkflow
	if err := h.db.Where("id = ?", workflowID).First(&workflow).Error; err != nil {
		return fmt.Errorf("获取工作流失败: %v", err)
	}
	
	// 检查工作流是否活跃
	if !workflow.IsActive {
		return fmt.Errorf("工作流已禁用")
	}
	
	// 准备触发源数据
	triggerSource := make(map[string]interface{})
	for k, v := range vars {
		triggerSource[k] = v
	}
	
	// 获取触发用户ID（系统触发为0）
	var triggerUserID *uint
	if userID, ok := vars["trigger_user_id"].(float64); ok && userID > 0 {
		uid := uint(userID)
		triggerUserID = &uid
	}
	
	// 获取触发类型
	triggerType, _ := vars["trigger_type"].(string)
	if triggerType == "" {
		triggerType = "task_queue"
	}
	
	// 执行工作流
	execution, err := h.workflowExecutor.ExecuteWorkflow(&workflow, triggerType, triggerSource, triggerUserID)
	if err != nil {
		return fmt.Errorf("执行工作流失败: %v", err)
	}
	
	log.Infof("自愈工作流执行成功，执行ID: %s", execution.ExecutionID)
	
	// 更新任务结果
	result := map[string]interface{}{
		"execution_id": execution.ExecutionID,
		"workflow_id":  workflow.ID,
		"workflow_name": workflow.Name,
	}
	
	return h.updateTaskResult(task.TaskID, result)
}

// updateTaskResult 更新任务结果
func (h *HealingTaskHandler) updateTaskResult(taskID string, result interface{}) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("序列化结果失败: %v", err)
	}
	
	return h.db.Model(&models.Task{}).
		Where("task_id = ?", taskID).
		Update("result", resultJSON).Error
}