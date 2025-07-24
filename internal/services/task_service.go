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

	if status == "locked" {
		updates["locked_at"] = time.Now()
	} else if status == "running" {
		updates["started_at"] = time.Now()
	} else if status == "success" || status == "failed" || status == "cancelled" || status == "timeout" {
		updates["finished_at"] = time.Now()
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
