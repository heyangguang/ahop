package services

import (
	"ahop/internal/models"
	"ahop/pkg/pagination"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"time"
)

// HealingWorkflowService 自愈工作流服务
type HealingWorkflowService struct {
	db *gorm.DB
}

// NewHealingWorkflowService 创建自愈工作流服务
func NewHealingWorkflowService(db *gorm.DB) *HealingWorkflowService {
	return &HealingWorkflowService{db: db}
}

// CreateHealingWorkflowRequest 创建自愈工作流请求
type CreateHealingWorkflowRequest struct {
	Name           string                    `json:"name" binding:"required,max=200"`
	Code           string                    `json:"code" binding:"required,max=100"`
	Description    string                    `json:"description" binding:"max=500"`
	Definition     models.WorkflowDefinition `json:"definition" binding:"required"`
	TimeoutMinutes int                       `json:"timeout_minutes"`
	MaxRetries     int                       `json:"max_retries"`
	AllowParallel  bool                      `json:"allow_parallel"`
}

// UpdateHealingWorkflowRequest 更新自愈工作流请求
type UpdateHealingWorkflowRequest struct {
	Name           string                     `json:"name" binding:"max=200"`
	Description    string                     `json:"description" binding:"max=500"`
	Definition     *models.WorkflowDefinition `json:"definition"`
	TimeoutMinutes *int                       `json:"timeout_minutes"`
	MaxRetries     *int                       `json:"max_retries"`
	AllowParallel  *bool                      `json:"allow_parallel"`
}

// Create 创建自愈工作流
func (s *HealingWorkflowService) Create(tenantID uint, userID uint, req CreateHealingWorkflowRequest) (*models.HealingWorkflow, error) {
	// 验证code在租户内唯一
	var count int64
	if err := s.db.Model(&models.HealingWorkflow{}).
		Where("tenant_id = ? AND code = ?", tenantID, req.Code).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("工作流代码已存在")
	}

	// 验证工作流定义
	if err := s.validateWorkflowDefinition(&req.Definition); err != nil {
		return nil, fmt.Errorf("工作流定义验证失败: %v", err)
	}

	// 序列化工作流定义
	definitionJSON, err := json.Marshal(req.Definition)
	if err != nil {
		return nil, fmt.Errorf("序列化工作流定义失败: %v", err)
	}

	// 创建工作流
	workflow := &models.HealingWorkflow{
		TenantID:       tenantID,
		Name:           req.Name,
		Code:           req.Code,
		Description:    req.Description,
		Definition:     definitionJSON,
		TimeoutMinutes: req.TimeoutMinutes,
		MaxRetries:     req.MaxRetries,
		AllowParallel:  req.AllowParallel,
		CreatedBy:      userID,
		UpdatedBy:      userID,
	}

	if workflow.TimeoutMinutes == 0 {
		workflow.TimeoutMinutes = 60
	}

	if err := s.db.Create(workflow).Error; err != nil {
		return nil, err
	}

	return workflow, nil
}

// Update 更新自愈工作流
func (s *HealingWorkflowService) Update(tenantID uint, workflowID uint, userID uint, req UpdateHealingWorkflowRequest) (*models.HealingWorkflow, error) {
	var workflow models.HealingWorkflow
	if err := s.db.Where("id = ? AND tenant_id = ?", workflowID, tenantID).First(&workflow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("工作流不存在")
		}
		return nil, err
	}

	// 检查是否有规则使用此工作流
	var ruleCount int64
	if err := s.db.Model(&models.HealingRule{}).
		Where("workflow_id = ?", workflowID).
		Count(&ruleCount).Error; err != nil {
		return nil, err
	}

	// 构建更新数据
	updates := map[string]interface{}{
		"updated_by": userID,
		"updated_at": time.Now(),
		"version":    gorm.Expr("version + 1"),
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.TimeoutMinutes != nil {
		updates["timeout_minutes"] = *req.TimeoutMinutes
	}
	if req.MaxRetries != nil {
		updates["max_retries"] = *req.MaxRetries
	}
	if req.AllowParallel != nil {
		updates["allow_parallel"] = *req.AllowParallel
	}
	if req.Definition != nil {
		// 验证工作流定义
		if err := s.validateWorkflowDefinition(req.Definition); err != nil {
			return nil, fmt.Errorf("工作流定义验证失败: %v", err)
		}
		
		definitionJSON, err := json.Marshal(req.Definition)
		if err != nil {
			return nil, fmt.Errorf("序列化工作流定义失败: %v", err)
		}
		updates["definition"] = definitionJSON
	}

	// 执行更新
	if err := s.db.Model(&workflow).Updates(updates).Error; err != nil {
		return nil, err
	}

	// 重新加载数据
	if err := s.db.First(&workflow, workflow.ID).Error; err != nil {
		return nil, err
	}

	return &workflow, nil
}

// Delete 删除自愈工作流
func (s *HealingWorkflowService) Delete(tenantID uint, workflowID uint) error {
	// 检查是否有规则使用此工作流
	var ruleCount int64
	if err := s.db.Model(&models.HealingRule{}).
		Where("workflow_id = ?", workflowID).
		Count(&ruleCount).Error; err != nil {
		return err
	}
	if ruleCount > 0 {
		return errors.New("该工作流被规则引用，不能删除")
	}

	// 检查是否有执行历史
	var execCount int64
	if err := s.db.Model(&models.HealingExecution{}).
		Where("workflow_id = ?", workflowID).
		Count(&execCount).Error; err != nil {
		return err
	}
	if execCount > 0 {
		return errors.New("该工作流有执行历史，不能删除")
	}

	// 删除工作流
	result := s.db.Where("id = ? AND tenant_id = ?", workflowID, tenantID).
		Delete(&models.HealingWorkflow{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("工作流不存在")
	}

	return nil
}

// GetByID 根据ID获取自愈工作流
func (s *HealingWorkflowService) GetByID(tenantID uint, workflowID uint) (*models.HealingWorkflow, error) {
	var workflow models.HealingWorkflow
	if err := s.db.Where("id = ? AND tenant_id = ?", workflowID, tenantID).
		First(&workflow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("工作流不存在")
		}
		return nil, err
	}
	return &workflow, nil
}

// List 获取自愈工作流列表
func (s *HealingWorkflowService) List(tenantID uint, params *pagination.PageParams, search string) ([]models.HealingWorkflow, int64, error) {
	var workflows []models.HealingWorkflow
	var total int64

	query := s.db.Model(&models.HealingWorkflow{}).Where("tenant_id = ?", tenantID)

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
		Order("created_at DESC").
		Find(&workflows).Error; err != nil {
		return nil, 0, err
	}

	return workflows, total, nil
}

// Enable 启用工作流
func (s *HealingWorkflowService) Enable(tenantID uint, workflowID uint, userID uint) error {
	result := s.db.Model(&models.HealingWorkflow{}).
		Where("id = ? AND tenant_id = ?", workflowID, tenantID).
		Updates(map[string]interface{}{
			"is_active":  true,
			"updated_by": userID,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("工作流不存在")
	}
	return nil
}

// Disable 禁用工作流
func (s *HealingWorkflowService) Disable(tenantID uint, workflowID uint, userID uint) error {
	result := s.db.Model(&models.HealingWorkflow{}).
		Where("id = ? AND tenant_id = ?", workflowID, tenantID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"updated_by": userID,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("工作流不存在")
	}
	return nil
}

// validateWorkflowDefinition 验证工作流定义
func (s *HealingWorkflowService) validateWorkflowDefinition(def *models.WorkflowDefinition) error {
	if len(def.Nodes) == 0 {
		return errors.New("工作流必须包含至少一个节点")
	}

	// 检查节点ID唯一性
	nodeMap := make(map[string]bool)
	for _, node := range def.Nodes {
		if node.ID == "" {
			return errors.New("节点ID不能为空")
		}
		if nodeMap[node.ID] {
			return fmt.Errorf("节点ID重复: %s", node.ID)
		}
		nodeMap[node.ID] = true

		// 验证节点类型
		if !isValidNodeType(node.Type) {
			return fmt.Errorf("无效的节点类型: %s", node.Type)
		}

		// 验证节点名称
		if node.Name == "" {
			return fmt.Errorf("节点 %s 的名称不能为空", node.ID)
		}
	}

	// 验证连接
	for _, conn := range def.Connections {
		if !nodeMap[conn.From] {
			return fmt.Errorf("连接的源节点不存在: %s", conn.From)
		}
		if !nodeMap[conn.To] {
			return fmt.Errorf("连接的目标节点不存在: %s", conn.To)
		}
	}

	// 检查是否有开始节点
	hasStart := false
	for _, node := range def.Nodes {
		if node.Type == models.NodeTypeStart {
			hasStart = true
			break
		}
	}
	if !hasStart {
		return errors.New("工作流必须包含开始节点")
	}

	return nil
}

// isValidNodeType 检查节点类型是否有效
func isValidNodeType(nodeType string) bool {
	validTypes := []string{
		models.NodeTypeStart,
		models.NodeTypeEnd,
		models.NodeTypeCondition,
		models.NodeTypeDataProcess,
		models.NodeTypeTaskExecute,
		models.NodeTypeControl,
		models.NodeTypeTicketUpdate,
	}

	for _, validType := range validTypes {
		if nodeType == validType {
			return true
		}
	}
	return false
}

// UpdateExecutionStats 更新执行统计
func (s *HealingWorkflowService) UpdateExecutionStats(workflowID uint, success bool, duration int) error {
	// 获取当前工作流统计
	var workflow models.HealingWorkflow
	if err := s.db.Select("execute_count, success_count, failure_count, avg_duration").
		Where("id = ?", workflowID).
		First(&workflow).Error; err != nil {
		return err
	}

	// 计算新的平均执行时长
	totalDuration := workflow.AvgDuration * int(workflow.ExecuteCount)
	newAvgDuration := (totalDuration + duration) / int(workflow.ExecuteCount+1)

	updates := map[string]interface{}{
		"execute_count":  gorm.Expr("execute_count + 1"),
		"last_execute_at": time.Now(),
		"avg_duration":   newAvgDuration,
	}
	
	if success {
		updates["success_count"] = gorm.Expr("success_count + 1")
	} else {
		updates["failure_count"] = gorm.Expr("failure_count + 1")
	}

	return s.db.Model(&models.HealingWorkflow{}).
		Where("id = ?", workflowID).
		Updates(updates).Error
}

// Clone 克隆工作流
func (s *HealingWorkflowService) Clone(tenantID uint, workflowID uint, userID uint, newCode string, newName string) (*models.HealingWorkflow, error) {
	// 获取源工作流
	var sourceWorkflow models.HealingWorkflow
	if err := s.db.Where("id = ? AND tenant_id = ?", workflowID, tenantID).
		First(&sourceWorkflow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("源工作流不存在")
		}
		return nil, err
	}

	// 验证新code不存在
	var count int64
	if err := s.db.Model(&models.HealingWorkflow{}).
		Where("tenant_id = ? AND code = ?", tenantID, newCode).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("工作流代码已存在")
	}

	// 创建新工作流
	newWorkflow := &models.HealingWorkflow{
		TenantID:       tenantID,
		Name:           newName,
		Code:           newCode,
		Description:    sourceWorkflow.Description + " (克隆自 " + sourceWorkflow.Name + ")",
		Definition:     sourceWorkflow.Definition,
		TimeoutMinutes: sourceWorkflow.TimeoutMinutes,
		MaxRetries:     sourceWorkflow.MaxRetries,
		AllowParallel:  sourceWorkflow.AllowParallel,
		Version:        1,
		IsActive:       true,
		CreatedBy:      userID,
		UpdatedBy:      userID,
	}

	if err := s.db.Create(newWorkflow).Error; err != nil {
		return nil, err
	}

	return newWorkflow, nil
}

// Execute 执行工作流
func (s *HealingWorkflowService) Execute(workflow *models.HealingWorkflow, triggerType string, triggerSource map[string]interface{}, userID *uint) (*models.HealingExecution, error) {
	// 创建工作流执行器
	// TODO: 需要注入 RedisQueue 依赖
	executor := NewWorkflowExecutor(s.db, nil, NewTicketService())
	
	// 执行工作流
	return executor.ExecuteWorkflow(workflow, triggerType, triggerSource, userID)
}