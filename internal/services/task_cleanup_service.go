package services

import (
	"ahop/internal/models"
	"ahop/pkg/logger"
	"ahop/pkg/queue"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// TaskCleanupService 任务清理服务
type TaskCleanupService struct {
	db    *gorm.DB
	queue *queue.RedisQueue
}

// NewTaskCleanupService 创建任务清理服务
func NewTaskCleanupService(db *gorm.DB, queue *queue.RedisQueue) *TaskCleanupService {
	return &TaskCleanupService{
		db:    db,
		queue: queue,
	}
}

// CleanupZombieTasks 清理僵尸任务
// 僵尸任务定义：
// 1. 状态为 queued 但不在 Redis 队列中的任务
// 2. 数据库状态与Redis状态不一致的任务
// 3. 状态为 pending 超过 30 分钟的任务
func (s *TaskCleanupService) CleanupZombieTasks() error {
	log := logger.GetLogger()
	
	// 1. 清理超时的 pending 任务
	pendingTimeout := time.Now().Add(-30 * time.Minute)
	result := s.db.Model(&models.Task{}).
		Where("status = ? AND created_at < ?", "pending", pendingTimeout).
		Updates(map[string]interface{}{
			"status":      "failed",
			"error_msg":   "任务创建后未能入队，超时失败",
			"finished_at": time.Now(),
		})
	if result.Error == nil && result.RowsAffected > 0 {
		log.Infof("清理了 %d 个超时的 pending 任务", result.RowsAffected)
	}
	
	// 2. 获取所有 queued 状态的任务
	var queuedTasks []models.Task
	err := s.db.Where("status = ?", "queued").
		Where("queued_at < ?", time.Now().Add(-5*time.Minute)). // 只处理超过5分钟的
		Find(&queuedTasks).Error
	if err != nil {
		return err
	}
	
	if len(queuedTasks) == 0 {
		return nil
	}
	
	log.Infof("发现 %d 个可能的僵尸任务", len(queuedTasks))
	
	// 2. 检查每个任务是否在队列中
	for _, task := range queuedTasks {
		// 检查任务是否在 Redis 队列中
		exists, err := s.queue.TaskExists(task.TaskID)
		if err != nil {
			log.Errorf("检查任务 %s 是否在队列中失败: %v", task.TaskID, err)
			continue
		}
		
		if !exists {
			// 任务不在队列中，是僵尸任务
			log.Warnf("发现僵尸任务: %s (创建于 %s)", task.TaskID, task.CreatedAt)
			
			// 更新任务状态为失败
			now := time.Now()
			updates := map[string]interface{}{
				"status":      "failed",
				"error":       "任务在队列中丢失（可能由于处理失败）",
				"finished_at": &now,
				"updated_at":  now,
			}
			
			if err := s.db.Model(&task).Updates(updates).Error; err != nil {
				log.Errorf("更新僵尸任务 %s 状态失败: %v", task.TaskID, err)
			} else {
				log.Infof("已将僵尸任务 %s 标记为失败", task.TaskID)
			}
		} else {
			// 任务存在于Redis中，检查状态是否一致
			redisStatus, err := s.queue.GetTaskStatus(task.TaskID)
			if err != nil {
				log.Errorf("获取任务 %s 的Redis状态失败: %v", task.TaskID, err)
				continue
			}
			
			// 检查Redis中的状态
			if status, ok := redisStatus["status"]; ok && status != "queued" {
				// Redis中的状态不是queued，说明任务已经被处理但数据库未更新
				log.Warnf("发现状态不一致的任务: %s (数据库: queued, Redis: %s)", task.TaskID, status)
				
				// 如果Redis中显示正在运行，但超过30分钟，认为是僵尸任务
				if status == "running" {
					if startedAt, ok := redisStatus["started_at"]; ok {
						// 将字符串转换为时间戳
						var startTime int64
						if _, err := fmt.Sscanf(startedAt, "%d", &startTime); err == nil {
							if time.Now().Unix()-startTime > 1800 { // 30分钟
								log.Warnf("任务 %s 运行超过30分钟，标记为失败", task.TaskID)
								
								now := time.Now()
								updates := map[string]interface{}{
									"status":      "failed",
									"error":       "任务执行超时（运行超过30分钟）",
									"finished_at": &now,
									"updated_at":  now,
								}
								
								if err := s.db.Model(&task).Updates(updates).Error; err != nil {
									log.Errorf("更新超时任务 %s 状态失败: %v", task.TaskID, err)
								} else {
									log.Infof("已将超时任务 %s 标记为失败", task.TaskID)
									// 同时清理Redis中的任务信息
									s.queue.RemoveTask(task.TaskID)
								}
							}
						}
					}
				}
			}
		}
	}
	
	return nil
}

// StartCleanupScheduler 启动定期清理调度器
func (s *TaskCleanupService) StartCleanupScheduler() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // 每5分钟执行一次
		defer ticker.Stop()
		
		log := logger.GetLogger()
		log.Info("启动任务清理调度器")
		
		for range ticker.C {
			if err := s.CleanupZombieTasks(); err != nil {
				log.Errorf("清理僵尸任务失败: %v", err)
			}
		}
	}()
}

// CleanupStuckScheduledTasks 清理卡住的定时任务
func (s *TaskCleanupService) CleanupStuckScheduledTasks() error {
	log := logger.GetLogger()
	
	// 查找超过30分钟还在 running 状态的定时任务
	var stuckTasks []models.ScheduledTask
	err := s.db.Where("last_status = ?", models.ScheduledTaskStatusRunning).
		Where("last_run_at < ?", time.Now().Add(-30*time.Minute)).
		Find(&stuckTasks).Error
	if err != nil {
		return err
	}
	
	for _, st := range stuckTasks {
		log.Warnf("发现卡住的定时任务: %s (ID: %d)", st.Name, st.ID)
		
		// 检查关联的任务状态
		if st.LastTaskID != "" {
			var task models.Task
			if err := s.db.Where("task_id = ?", st.LastTaskID).First(&task).Error; err == nil {
				// 根据任务实际状态更新定时任务状态
				if task.Status == "success" {
					s.db.Model(&st).Update("last_status", models.ScheduledTaskStatusSuccess)
				} else if task.Status == "failed" || task.Status == "timeout" || task.Status == "cancelled" {
					s.db.Model(&st).Update("last_status", models.ScheduledTaskStatusFailed)
				} else if task.Status == "queued" || task.Status == "pending" {
					// 任务还在队列中，可能是僵尸任务
					s.db.Model(&st).Update("last_status", models.ScheduledTaskStatusFailed)
				}
			} else {
				// 找不到任务，标记为失败
				s.db.Model(&st).Update("last_status", models.ScheduledTaskStatusFailed)
			}
		} else {
			// 没有关联任务，直接标记为失败
			s.db.Model(&st).Update("last_status", models.ScheduledTaskStatusFailed)
		}
	}
	
	return nil
}