package services

import (
	"ahop/internal/models"
	"ahop/pkg/queue"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TaskService 任务服务
type TaskService struct {
	db    *gorm.DB
	queue *queue.RedisQueue
}

// NewTaskService 创建任务服务实例
func NewTaskService(db *gorm.DB, redisQueue *queue.RedisQueue) *TaskService {
	return &TaskService{
		db:    db,
		queue: redisQueue,
	}
}

// CreateTask 创建任务
func (s *TaskService) CreateTask(task *models.Task) error {
	// 生成任务ID
	if task.TaskID == "" {
		task.TaskID = uuid.New().String()
	}

	// 设置默认值
	task.Status = "pending"
	if task.Priority == 0 {
		task.Priority = 5 // 默认优先级
	}

	// 如果没有提供source，默认为api
	if task.Source == "" {
		task.Source = "api"
	}

	// 获取租户名称
	var tenantName string
	if task.TenantID > 0 {
		var tenant models.Tenant
		if err := s.db.Select("name").Where("id = ?", task.TenantID).First(&tenant).Error; err == nil {
			tenantName = tenant.Name
			task.TenantName = tenantName
		}
	}

	// 保存到数据库
	if err := s.db.Create(task).Error; err != nil {
		return fmt.Errorf("保存任务到数据库失败: %v", err)
	}

	// 准备队列参数
	params := make(map[string]interface{})
	if len(task.Params) > 0 {
		if err := json.Unmarshal(task.Params, &params); err != nil {
			return fmt.Errorf("解析任务参数失败: %v", err)
		}
	}

	// 加入Redis队列（使用新的签名）
	if err := s.queue.Enqueue(
		task.TaskID,
		task.TaskType,
		task.TenantID,
		tenantName,
		task.CreatedBy,
		task.Username,
		task.Source,
		task.Priority,
		params,
	); err != nil {
		// 队列入队失败，删除数据库记录
		s.db.Delete(task)
		return fmt.Errorf("任务入队失败: %v", err)
	}

	// 更新任务状态为已入队
	now := time.Now()
	updates := map[string]interface{}{
		"status":    "queued",
		"queued_at": &now,
	}
	s.db.Model(task).Updates(updates)

	return nil
}

// GetTask 获取任务详情
func (s *TaskService) GetTask(taskID string, tenantID uint) (*models.Task, error) {
	var task models.Task
	err := s.db.Preload("Logs").
		Where("task_id = ? AND tenant_id = ?", taskID, tenantID).
		First(&task).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("任务不存在")
		}
		return nil, err
	}

	return &task, nil
}

// ListTasks 获取任务列表
func (s *TaskService) ListTasks(tenantID uint, page, pageSize int, filters map[string]interface{}) ([]models.Task, int64, error) {
	query := s.db.Model(&models.Task{}).Where("tenant_id = ?", tenantID)

	// 应用过滤条件
	if taskType, ok := filters["task_type"]; ok {
		query = query.Where("task_type = ?", taskType)
	}
	if status, ok := filters["status"]; ok {
		query = query.Where("status = ?", status)
	}
	if priority, ok := filters["priority"]; ok {
		query = query.Where("priority = ?", priority)
	}
	if createdBy, ok := filters["created_by"]; ok {
		query = query.Where("created_by = ?", createdBy)
	}

	// 计算总数
	var total int64
	query.Count(&total)

	// 分页查询
	offset := (page - 1) * pageSize
	var tasks []models.Task
	err := query.Offset(offset).Limit(pageSize).
		Order("created_at DESC").
		Find(&tasks).Error

	if err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// CreateTemplateTask 创建模板任务
func (s *TaskService) CreateTemplateTask(task *models.Task, templateID uint, variables map[string]interface{}, hostIDs []uint) error {
	// 验证任务模板是否存在
	var template models.TaskTemplate
	if err := s.db.Where("id = ? AND tenant_id = ?", templateID, task.TenantID).First(&template).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("任务模板不存在")
		}
		return err
	}

	// 验证参数是否符合模板定义
	templateService := NewTaskTemplateService(s.db)
	if err := templateService.ValidateTemplateVariables(&template, variables); err != nil {
		return fmt.Errorf("参数验证失败: %v", err)
	}

	// 验证主机是否存在且属于当前租户
	var hosts []models.Host
	if err := s.db.Where("id IN ? AND tenant_id = ?", hostIDs, task.TenantID).Find(&hosts).Error; err != nil {
		return err
	}
	if len(hosts) != len(hostIDs) {
		return fmt.Errorf("部分主机不存在或不属于当前租户")
	}

	// 构建任务参数，将主机ID数组转换为 interface{} 数组
	hostsInterface := make([]interface{}, len(hostIDs))
	for i, id := range hostIDs {
		hostsInterface[i] = id
	}

	taskParams := models.TaskParams{
		TemplateID: templateID,
		Variables:  variables,
		Hosts:      hostsInterface, // 使用 hosts 字段传递主机ID
	}

	paramsJSON, err := json.Marshal(taskParams)
	if err != nil {
		return fmt.Errorf("序列化任务参数失败: %v", err)
	}

	// 设置任务属性
	task.TaskType = models.TaskTypeTemplate
	task.Params = paramsJSON
	
	// 如果没有设置任务名称，使用模板名称
	if task.Name == "" {
		task.Name = fmt.Sprintf("%s - %s", template.Name, time.Now().Format("2006-01-02 15:04:05"))
	}

	// 如果没有设置超时时间，使用模板的超时时间
	if task.Timeout == 0 && template.Timeout > 0 {
		task.Timeout = template.Timeout
	}

	// 调用基础的创建任务方法
	return s.CreateTask(task)
}


// SaveTaskLogs 批量保存任务日志
func (s *TaskService) SaveTaskLogs(logs []models.TaskLog) error {
	if len(logs) == 0 {
		return nil
	}
	return s.db.CreateInBatches(logs, 100).Error
}

// UpdateTaskStatus 更新任务状态
func (s *TaskService) UpdateTaskStatus(taskID string, status string, progress int, workerID string) error {
	// 更新数据库
	updates := map[string]interface{}{
		"status":     status,
		"progress":   progress,
		"updated_at": time.Now(),
	}

	if workerID != "" {
		updates["worker_id"] = workerID
	}

	now := time.Now()
	if status == "locked" {
		updates["locked_at"] = &now
	} else if status == "running" {
		updates["started_at"] = &now
	} else if status == "success" || status == "failed" || status == "cancelled" || status == "timeout" {
		updates["finished_at"] = &now
	}

	result := s.db.Model(&models.Task{}).
		Where("task_id = ?", taskID).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("任务不存在")
	}

	// 同步更新Redis状态
	if err := s.queue.UpdateTaskStatus(taskID, status, progress, workerID); err != nil {
		// Redis更新失败不影响主要流程，记录日志即可
	}

	return nil
}

// UpdateTemplateTaskResult 更新模板任务执行结果
func (s *TaskService) UpdateTemplateTaskResult(taskID string, result *models.TaskTemplateResult) error {
	if result == nil {
		return nil
	}
	
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("序列化任务结果失败: %v", err)
	}
	
	return s.db.Model(&models.Task{}).
		Where("task_id = ?", taskID).
		Update("result", resultJSON).Error
}

// SetTaskResult 设置任务结果
func (s *TaskService) SetTaskResult(taskID string, result interface{}, errorMsg string) error {
	updates := map[string]interface{}{
		"updated_at":  time.Now(),
		"finished_at": time.Now(),
	}

	if errorMsg != "" {
		updates["status"] = "failed"
		updates["error"] = errorMsg
	} else {
		updates["status"] = "success"
		if result != nil {
			resultData, err := json.Marshal(result)
			if err != nil {
				return fmt.Errorf("序列化任务结果失败: %v", err)
			}
			updates["result"] = resultData
		}
	}

	dbResult := s.db.Model(&models.Task{}).
		Where("task_id = ?", taskID).
		Updates(updates)

	if dbResult.Error != nil {
		return dbResult.Error
	}

	if dbResult.RowsAffected == 0 {
		return fmt.Errorf("任务不存在")
	}

	// 同步更新Redis结果
	if err := s.queue.SetTaskResult(taskID, result, errorMsg); err != nil {
		// Redis更新失败不影响主要流程
	}

	return nil
}

// CancelTask 取消任务
func (s *TaskService) CancelTask(taskID string, tenantID uint, userID uint) error {
	var task models.Task
	err := s.db.Where("task_id = ? AND tenant_id = ?", taskID, tenantID).First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("任务不存在")
		}
		return err
	}

	// 只能取消待执行或执行中的任务
	if task.Status != "pending" && task.Status != "queued" && task.Status != "running" {
		return fmt.Errorf("任务状态为 %s，无法取消", task.Status)
	}

	// 更新任务状态
	updates := map[string]interface{}{
		"status":       "cancelled",
		"cancelled_by": userID,
		"finished_at":  time.Now(),
		"updated_at":   time.Now(),
	}

	return s.db.Model(&task).Updates(updates).Error
}

// AddTaskLog 添加任务日志
func (s *TaskService) AddTaskLog(taskID, level, source, message string, hostName string, data interface{}) error {
	taskLog := &models.TaskLog{
		TaskID:    taskID,
		Timestamp: time.Now(),
		Level:     level,
		Source:    source,
		HostName:  hostName,
		Message:   message,
	}

	if data != nil {
		dataBytes, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("序列化日志数据失败: %v", err)
		}
		taskLog.Data = dataBytes
	}

	return s.db.Create(taskLog).Error
}

// GetTaskLogs 获取任务日志
func (s *TaskService) GetTaskLogs(taskID string, tenantID uint, page, pageSize int) ([]models.TaskLog, int64, error) {
	// 验证任务是否属于当前租户
	var task models.Task
	err := s.db.Where("task_id = ? AND tenant_id = ?", taskID, tenantID).First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, fmt.Errorf("任务不存在")
		}
		return nil, 0, err
	}

	// 查询日志
	query := s.db.Model(&models.TaskLog{}).Where("task_id = ?", taskID)

	var total int64
	query.Count(&total)

	offset := (page - 1) * pageSize
	var logs []models.TaskLog
	err = query.Offset(offset).Limit(pageSize).
		Order("timestamp ASC").
		Find(&logs).Error

	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// DeleteTask 删除任务及其日志
func (s *TaskService) DeleteTask(taskID string, tenantID uint) error {
	// 检查任务是否存在
	var task models.Task
	if err := s.db.Where("task_id = ? AND tenant_id = ?", taskID, tenantID).First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("任务不存在")
		}
		return err
	}

	// 检查任务状态
	if task.Status == "running" || task.Status == "queued" || task.Status == "locked" {
		return fmt.Errorf("任务正在执行中，无法删除")
	}

	// 使用事务级联删除
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 删除任务日志
		if err := tx.Where("task_id = ?", taskID).Delete(&models.TaskLog{}).Error; err != nil {
			return fmt.Errorf("删除任务日志失败: %v", err)
		}

		// 2. 删除任务
		if err := tx.Delete(&task).Error; err != nil {
			return fmt.Errorf("删除任务失败: %v", err)
		}

		return nil
	})
}

// DeleteTasksByIDs 批量删除任务（供其他服务调用）
func (s *TaskService) DeleteTasksByIDs(taskIDs []string) error {
	if len(taskIDs) == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 删除日志
		if err := tx.Where("task_id IN ?", taskIDs).Delete(&models.TaskLog{}).Error; err != nil {
			return fmt.Errorf("删除任务日志失败: %v", err)
		}

		// 删除任务
		if err := tx.Where("task_id IN ?", taskIDs).Delete(&models.Task{}).Error; err != nil {
			return fmt.Errorf("删除任务失败: %v", err)
		}

		return nil
	})
}

// GetQueueStats 获取队列统计信息
func (s *TaskService) GetQueueStats() (map[string]interface{}, error) {
	// 从Redis获取队列统计
	queueStats, err := s.queue.GetQueueStats()
	if err != nil {
		return nil, fmt.Errorf("获取队列统计失败: %v", err)
	}

	// 从数据库获取任务状态统计
	var dbStats []struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}

	err = s.db.Model(&models.Task{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&dbStats).Error

	if err != nil {
		return nil, fmt.Errorf("获取数据库统计失败: %v", err)
	}

	// 合并统计信息
	result := map[string]interface{}{
		"queue": queueStats,
		"task_status": map[string]int64{
			"pending":   0,
			"queued":    0,
			"locked":    0,
			"running":   0,
			"success":   0,
			"failed":    0,
			"cancelled": 0,
			"timeout":   0,
		},
	}

	for _, stat := range dbStats {
		if statusMap, ok := result["task_status"].(map[string]int64); ok {
			statusMap[stat.Status] = stat.Count
		}
	}

	return result, nil
}
