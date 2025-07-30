package services

import (
	"ahop/internal/models"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
)

// QueueService 队列服务
type QueueService struct {
	db *gorm.DB
}

// NewQueueService 创建队列服务
func NewQueueService(db *gorm.DB) *QueueService {
	return &QueueService{
		db: db,
	}
}

// QueueStatus 队列状态
type QueueStatus struct {
	TotalTasks      int64                    `json:"total_tasks"`       // 总任务数
	PendingTasks    int64                    `json:"pending_tasks"`     // 待执行任务数
	RunningTasks    int64                    `json:"running_tasks"`     // 执行中任务数
	CompletedTasks  int64                    `json:"completed_tasks"`   // 已完成任务数
	FailedTasks     int64                    `json:"failed_tasks"`      // 失败任务数
	TasksByType     map[string]int64         `json:"tasks_by_type"`     // 按类型统计
	TasksByPriority map[int]int64            `json:"tasks_by_priority"` // 按优先级统计
	RecentTasks     []QueueTaskInfo          `json:"recent_tasks"`      // 最近的任务
}

// QueueTaskInfo 队列任务信息
type QueueTaskInfo struct {
	TaskID      string    `json:"task_id"`
	TaskType    string    `json:"task_type"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"` // 添加描述字段
	Status      string    `json:"status"`
	Priority    int       `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	WorkerID    string    `json:"worker_id,omitempty"`
	TicketInfo  string    `json:"ticket_info,omitempty"` // 工单信息
}

// GetQueueStatus 获取队列状态
func (s *QueueService) GetQueueStatus(tenantID uint) (*QueueStatus, error) {
	status := &QueueStatus{
		TasksByType:     make(map[string]int64),
		TasksByPriority: make(map[int]int64),
	}
	
	// 基础查询
	baseQuery := s.db.Model(&models.Task{})
	if tenantID > 0 {
		baseQuery = baseQuery.Where("tenant_id = ?", tenantID)
	}
	
	// 总任务数
	if err := baseQuery.Count(&status.TotalTasks).Error; err != nil {
		return nil, err
	}
	
	// 按状态统计
	var statusCounts []struct {
		Status string
		Count  int64
	}
	if err := baseQuery.
		Select("status, COUNT(*) as count").
		Group("status").
		Find(&statusCounts).Error; err != nil {
		return nil, err
	}
	
	for _, sc := range statusCounts {
		switch sc.Status {
		case models.TaskStatusPending, models.TaskStatusQueued:
			status.PendingTasks += sc.Count
		case models.TaskStatusRunning, models.TaskStatusLocked:
			status.RunningTasks += sc.Count
		case models.TaskStatusSuccess:
			status.CompletedTasks += sc.Count
		case models.TaskStatusFailed, models.TaskStatusTimeout, models.TaskStatusCancelled:
			status.FailedTasks += sc.Count
		}
	}
	
	// 按类型统计
	var typeCounts []struct {
		TaskType string
		Count    int64
	}
	if err := baseQuery.
		Where("status IN ?", []string{models.TaskStatusPending, models.TaskStatusQueued, models.TaskStatusRunning}).
		Select("task_type, COUNT(*) as count").
		Group("task_type").
		Find(&typeCounts).Error; err != nil {
		return nil, err
	}
	
	for _, tc := range typeCounts {
		status.TasksByType[tc.TaskType] = tc.Count
	}
	
	// 按优先级统计
	var priorityCounts []struct {
		Priority int
		Count    int64
	}
	if err := baseQuery.
		Where("status IN ?", []string{models.TaskStatusPending, models.TaskStatusQueued}).
		Select("priority, COUNT(*) as count").
		Group("priority").
		Find(&priorityCounts).Error; err != nil {
		return nil, err
	}
	
	for _, pc := range priorityCounts {
		status.TasksByPriority[pc.Priority] = pc.Count
	}
	
	// 获取最近的任务（创建新的查询，不要重用baseQuery）
	var recentTasks []models.Task
	recentQuery := s.db.Model(&models.Task{}).Where("tenant_id = ?", tenantID)
	if err := recentQuery.
		Order("created_at DESC").
		Limit(20).
		Find(&recentTasks).Error; err != nil {
		return nil, err
	}
	
	for _, task := range recentTasks {
		info := QueueTaskInfo{
			TaskID:      task.TaskID,
			TaskType:    task.TaskType,
			Name:        task.Name,
			Description: task.Description,
			Status:      task.Status,
			Priority:    task.Priority,
			CreatedAt:   task.CreatedAt,
			WorkerID:    task.WorkerID,
		}
		
		// 从描述中提取工单信息
		if task.Description != "" {
			info.TicketInfo = s.extractTicketInfo(task.Description)
		}
		
		if task.LockedAt != nil {
			info.StartedAt = task.LockedAt
		}
		if task.FinishedAt != nil {
			info.FinishedAt = task.FinishedAt
		}
		
		status.RecentTasks = append(status.RecentTasks, info)
	}
	
	return status, nil
}

// GetQueueTasksByType 按类型获取队列任务
func (s *QueueService) GetQueueTasksByType(tenantID uint, taskType string, limit int) ([]QueueTaskInfo, error) {
	query := s.db.Model(&models.Task{})
	
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	
	if taskType != "" {
		query = query.Where("task_type = ?", taskType)
	}
	
	// 只查询待处理的任务
	query = query.Where("status IN ?", []string{models.TaskStatusPending, models.TaskStatusQueued})
	
	var tasks []models.Task
	if err := query.
		Order("priority ASC, created_at ASC").
		Limit(limit).
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	
	var result []QueueTaskInfo
	for _, task := range tasks {
		info := QueueTaskInfo{
			TaskID:      task.TaskID,
			TaskType:    task.TaskType,
			Name:        task.Name,
			Description: task.Description,
			Status:      task.Status,
			Priority:    task.Priority,
			CreatedAt:   task.CreatedAt,
		}
		
		// 从描述中提取工单信息
		if task.Description != "" {
			info.TicketInfo = s.extractTicketInfo(task.Description)
		}
		
		result = append(result, info)
	}
	
	return result, nil
}

// CancelQueueTask 取消队列中的任务
func (s *QueueService) CancelQueueTask(tenantID uint, taskID string) error {
	// 检查任务状态
	var task models.Task
	query := s.db.Where("task_id = ?", taskID)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	
	if err := query.First(&task).Error; err != nil {
		return fmt.Errorf("任务不存在")
	}
	
	// 只能取消待执行的任务
	if task.Status != models.TaskStatusPending && task.Status != models.TaskStatusQueued {
		return fmt.Errorf("任务状态为 %s，无法取消", task.Status)
	}
	
	// 更新任务状态
	now := time.Now()
	return s.db.Model(&task).Updates(map[string]interface{}{
		"status": models.TaskStatusCancelled,
		"finished_at": &now,
		"error": "用户取消",
	}).Error
}

// extractTicketInfo 从描述中提取工单信息
func (s *QueueService) extractTicketInfo(description string) string {
	// 查找 [工单:xxx] 格式的信息
	start := strings.Index(description, "[工单:")
	if start == -1 {
		return ""
	}
	
	end := strings.Index(description[start:], "]")
	if end == -1 {
		return ""
	}
	
	// 提取工单信息
	ticketInfo := description[start+len("[工单:") : start+end]
	return ticketInfo
}