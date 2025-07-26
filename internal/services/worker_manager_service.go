package services

import (
	"ahop/internal/models"
	"ahop/pkg/config"
	"ahop/pkg/queue"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// WorkerManagerService 分布式Worker管理服务
type WorkerManagerService struct {
	db    *gorm.DB
	queue *queue.RedisQueue
}

// NewWorkerManagerService 创建Worker管理服务
func NewWorkerManagerService(db *gorm.DB) *WorkerManagerService {
	cfg := config.GetConfig()
	redisQueue := queue.NewRedisQueue(&queue.Config{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		Prefix:   cfg.Redis.Prefix,
	})

	return &WorkerManagerService{
		db:    db,
		queue: redisQueue,
	}
}

// WorkerStatus Worker状态信息
type WorkerStatus struct {
	models.Worker
	IsOnline    bool     `json:"is_online"`
	LastSeen    string   `json:"last_seen"`
	OfflineDays int      `json:"offline_days"`
	TaskTypes   []string `json:"task_types_list"`
}

// QueueStats 队列统计信息
type QueueStats struct {
	TotalTasks     int                    `json:"total_tasks"`
	PriorityQueues map[string]int         `json:"priority_queues"`
	Details        map[string]interface{} `json:"details"`
}

// WorkerSummary Worker汇总信息
type WorkerSummary struct {
	TotalWorkers   int     `json:"total_workers"`
	OnlineWorkers  int     `json:"online_workers"`
	OfflineWorkers int     `json:"offline_workers"`
	TotalTasks     int64   `json:"total_tasks"`
	SuccessTasks   int64   `json:"success_tasks"`
	FailedTasks    int64   `json:"failed_tasks"`
	SuccessRate    float64 `json:"success_rate"`
}

// GetAllWorkers 获取所有Worker状态
func (s *WorkerManagerService) GetAllWorkers() ([]WorkerStatus, error) {
	var workers []models.Worker
	if err := s.db.Order("registered_at DESC").Find(&workers).Error; err != nil {
		return nil, fmt.Errorf("查询worker失败: %v", err)
	}

	var result []WorkerStatus
	now := time.Now()

	for _, worker := range workers {
		status := WorkerStatus{
			Worker: worker,
		}

		// 判断是否在线（60秒内有心跳认为在线）
		if now.Sub(worker.LastHeartbeat) <= 60*time.Second {
			status.IsOnline = true
			status.LastSeen = "刚刚"
		} else {
			status.IsOnline = false
			offlineDuration := now.Sub(worker.LastHeartbeat)
			if offlineDuration < 24*time.Hour {
				status.LastSeen = fmt.Sprintf("%.0f分钟前", offlineDuration.Minutes())
			} else {
				days := int(offlineDuration.Hours() / 24)
				status.OfflineDays = days
				status.LastSeen = fmt.Sprintf("%d天前", days)
			}
		}

		// 解析任务类型（简单字符串解析）
		if worker.TaskTypes != "" {
			// 这里可以改进为JSON解析
			status.TaskTypes = []string{worker.TaskTypes}
		}

		result = append(result, status)
	}

	return result, nil
}

// GetWorkerByID 根据ID获取Worker详情
func (s *WorkerManagerService) GetWorkerByID(workerID string) (*WorkerStatus, error) {
	var worker models.Worker
	if err := s.db.Where("worker_id = ?", workerID).First(&worker).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("worker不存在")
		}
		return nil, fmt.Errorf("查询Worker失败: %v", err)
	}

	status := &WorkerStatus{
		Worker: worker,
	}

	// 判断在线状态
	now := time.Now()
	if now.Sub(worker.LastHeartbeat) <= 60*time.Second {
		status.IsOnline = true
		status.LastSeen = "刚刚"
	} else {
		status.IsOnline = false
		offlineDuration := now.Sub(worker.LastHeartbeat)
		if offlineDuration < 24*time.Hour {
			status.LastSeen = fmt.Sprintf("%.0f分钟前", offlineDuration.Minutes())
		} else {
			days := int(offlineDuration.Hours() / 24)
			status.OfflineDays = days
			status.LastSeen = fmt.Sprintf("%d天前", days)
		}
	}

	return status, nil
}

// GetWorkerSummary 获取Worker汇总统计
func (s *WorkerManagerService) GetWorkerSummary() (*WorkerSummary, error) {
	var workers []models.Worker
	if err := s.db.Find(&workers).Error; err != nil {
		return nil, fmt.Errorf("查询Worker失败: %v", err)
	}

	summary := &WorkerSummary{
		TotalWorkers: len(workers),
	}

	now := time.Now()
	var totalTasks, successTasks, failedTasks int64

	for _, worker := range workers {
		// 统计在线/离线
		if now.Sub(worker.LastHeartbeat) <= 60*time.Second {
			summary.OnlineWorkers++
		} else {
			summary.OfflineWorkers++
		}

		// 统计任务数
		totalTasks += worker.TotalTasks
		successTasks += worker.SuccessTasks
		failedTasks += worker.FailedTasks
	}

	summary.TotalTasks = totalTasks
	summary.SuccessTasks = successTasks
	summary.FailedTasks = failedTasks

	// 计算成功率
	if totalTasks > 0 {
		summary.SuccessRate = float64(successTasks) / float64(totalTasks) * 100
	}

	return summary, nil
}

// GetQueueStats 获取队列统计信息
func (s *WorkerManagerService) GetQueueStats() (*QueueStats, error) {
	stats, err := s.queue.GetQueueStats()
	if err != nil {
		return nil, fmt.Errorf("获取队列统计失败: %v", err)
	}

	queueStats := &QueueStats{
		PriorityQueues: make(map[string]int),
		Details:        make(map[string]interface{}),
	}

	for key, count := range stats {
		if key == "total" {
			queueStats.TotalTasks = count
		} else {
			queueStats.PriorityQueues[key] = count
		}
	}

	// 添加详细信息
	queueStats.Details["high_priority"] = stats["priority_1"] + stats["priority_2"]
	queueStats.Details["normal_priority"] = stats["priority_5"]
	queueStats.Details["low_priority"] = stats["priority_9"] + stats["priority_10"]

	return queueStats, nil
}

// RemoveOfflineWorker 移除离线Worker记录
func (s *WorkerManagerService) RemoveOfflineWorker(workerID string) error {
	var worker models.Worker
	if err := s.db.Where("worker_id = ?", workerID).First(&worker).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("worker不存在")
		}
		return fmt.Errorf("查询Worker失败: %v", err)
	}

	// 检查是否真的离线（超过1小时没有心跳）
	if time.Since(worker.LastHeartbeat) < time.Hour {
		return fmt.Errorf("worker仍在线，无法移除")
	}

	if err := s.db.Delete(&worker).Error; err != nil {
		return fmt.Errorf("删除Worker失败: %v", err)
	}

	return nil
}

// GetWorkerTasks 获取Worker的任务执行历史
func (s *WorkerManagerService) GetWorkerTasks(workerID string, page, pageSize int) ([]models.Task, int64, error) {
	var tasks []models.Task
	var total int64

	query := s.db.Model(&models.Task{}).Where("worker_id = ?", workerID)

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计任务数失败: %v", err)
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("查询任务失败: %v", err)
	}

	return tasks, total, nil
}

// GetActiveWorkers 获取活跃Worker（最近5分钟内有心跳）
func (s *WorkerManagerService) GetActiveWorkers() ([]models.Worker, error) {
	var workers []models.Worker
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)

	if err := s.db.Where("last_heartbeat > ?", fiveMinutesAgo).
		Order("last_heartbeat DESC").Find(&workers).Error; err != nil {
		return nil, fmt.Errorf("查询活跃Worker失败: %v", err)
	}

	return workers, nil
}

// GetWorkersByStatus 根据状态获取Worker
func (s *WorkerManagerService) GetWorkersByStatus(online bool) ([]models.Worker, error) {
	var workers []models.Worker

	if online {
		// 获取在线Worker（60秒内有心跳）
		threeMinutesAgo := time.Now().Add(-3 * time.Minute)
		if err := s.db.Where("last_heartbeat > ?", threeMinutesAgo).
			Order("last_heartbeat DESC").Find(&workers).Error; err != nil {
			return nil, fmt.Errorf("查询在线Worker失败: %v", err)
		}
	} else {
		// 获取离线Worker（60秒内无心跳）
		threeMinutesAgo := time.Now().Add(-3 * time.Minute)
		if err := s.db.Where("last_heartbeat <= ?", threeMinutesAgo).
			Order("last_heartbeat DESC").Find(&workers).Error; err != nil {
			return nil, fmt.Errorf("查询离线Worker失败: %v", err)
		}
	}

	return workers, nil
}
