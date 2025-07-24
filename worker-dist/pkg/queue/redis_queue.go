package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisQueue Redis队列实现
type RedisQueue struct {
	client *redis.Client
	prefix string
}

// TaskMessage 队列中的任务消息
type TaskMessage struct {
	TaskID     string                 `json:"task_id"`
	TaskType   string                 `json:"task_type"`
	TenantID   uint                   `json:"tenant_id"`
	TenantName string                 `json:"tenant_name"`  // 租户名称
	UserID     uint                   `json:"user_id"`      // 发起人ID
	Username   string                 `json:"username"`     // 发起人用户名
	Priority   int                    `json:"priority"`
	Params     map[string]interface{} `json:"params"`
	Created    int64                  `json:"created"`
	Source     string                 `json:"source"`       // 任务来源
}

// Config Redis配置
type Config struct {
	Host     string
	Port     int
	Password string
	DB       int
	Prefix   string
}

// NewRedisQueue 创建Redis队列实例
func NewRedisQueue(config *Config) *RedisQueue {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       config.DB,
	})

	prefix := config.Prefix
	if prefix == "" {
		prefix = "ahop:queue"
	}

	return &RedisQueue{
		client: client,
		prefix: prefix,
	}
}

// Close 关闭Redis连接
func (q *RedisQueue) Close() error {
	return q.client.Close()
}

// Ping 测试Redis连接
func (q *RedisQueue) Ping() error {
	ctx := context.Background()
	return q.client.Ping(ctx).Err()
}

// Enqueue 将任务加入队列
func (q *RedisQueue) Enqueue(taskID, taskType string, tenantID uint, tenantName string, userID uint, username string, source string, priority int, params map[string]interface{}) error {
	ctx := context.Background()

	// 创建任务消息
	message := TaskMessage{
		TaskID:     taskID,
		TaskType:   taskType,
		TenantID:   tenantID,
		TenantName: tenantName,
		UserID:     userID,
		Username:   username,
		Priority:   priority,
		Params:     params,
		Created:    time.Now().Unix(),
		Source:     source,
	}

	// 序列化消息
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化任务消息失败: %v", err)
	}

	// 根据优先级选择队列
	queueKey := q.getQueueKey(priority)

	// 加入队列（左侧入队）
	if err := q.client.LPush(ctx, queueKey, data).Err(); err != nil {
		return fmt.Errorf("任务入队失败: %v", err)
	}

	// 记录任务到集合中（用于状态查询）
	taskKey := q.getTaskKey(taskID)
	taskInfo := map[string]interface{}{
		"task_id":   taskID,
		"task_type": taskType,
		"tenant_id": tenantID,
		"priority":  priority,
		"status":    "queued",
		"queued_at": time.Now().Unix(),
	}
	if err := q.client.HSet(ctx, taskKey, taskInfo).Err(); err != nil {
		return fmt.Errorf("记录任务状态失败: %v", err)
	}

	// 设置任务过期时间（24小时）
	q.client.Expire(ctx, taskKey, 24*time.Hour)

	return nil
}

// Dequeue 从队列获取任务（阻塞式）- 使用BLMOVE确保任务不丢失
func (q *RedisQueue) Dequeue(timeout time.Duration) (*TaskMessage, error) {
	ctx := context.Background()
	processingQueue := q.getProcessingQueueKey()

	// 按优先级顺序检查队列（优先级越小越高）
	for priority := 1; priority <= 10; priority++ {
		queueKey := q.getQueueKey(priority)
		
		// 使用BLMOVE原子性地从队列移动到处理中队列
		// 这样即使Worker崩溃，任务也不会丢失
		result, err := q.client.BLMove(ctx, queueKey, processingQueue, "RIGHT", "LEFT", 1*time.Second).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue // 这个优先级队列为空，检查下一个
			}
			return nil, fmt.Errorf("获取任务失败: %v", err)
		}

		// 解析消息
		var message TaskMessage
		if err := json.Unmarshal([]byte(result), &message); err != nil {
			// 解析失败，将任务标记为处理中但失败，避免死循环
			q.markTaskAsFailed(message.TaskID, fmt.Sprintf("解析任务消息失败: %v", err))
			return nil, fmt.Errorf("解析任务消息失败: %v", err)
		}

		// 更新任务状态为处理中
		taskKey := q.getTaskKey(message.TaskID)
		q.client.HSet(ctx, taskKey, map[string]interface{}{
			"status":      "processing",
			"dequeued_at": time.Now().Unix(),
		})

		return &message, nil
	}

	// 所有队列都为空，返回nil
	return nil, nil
}

// UpdateTaskStatus 更新任务状态
func (q *RedisQueue) UpdateTaskStatus(taskID, status string, progress int, workerID string) error {
	ctx := context.Background()
	taskKey := q.getTaskKey(taskID)

	updates := map[string]interface{}{
		"status":     status,
		"progress":   progress,
		"updated_at": time.Now().Unix(),
	}

	if workerID != "" {
		updates["worker_id"] = workerID
	}

	if status == "running" {
		updates["started_at"] = time.Now().Unix()
	} else if status == "success" || status == "failed" || status == "cancelled" {
		updates["finished_at"] = time.Now().Unix()
	}

	return q.client.HSet(ctx, taskKey, updates).Err()
}

// GetTaskStatus 获取任务状态
func (q *RedisQueue) GetTaskStatus(taskID string) (map[string]string, error) {
	ctx := context.Background()
	taskKey := q.getTaskKey(taskID)

	result, err := q.client.HGetAll(ctx, taskKey).Result()
	if err != nil {
		return nil, fmt.Errorf("获取任务状态失败: %v", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("任务不存在")
	}

	return result, nil
}

// SetTaskResult 设置任务结果
func (q *RedisQueue) SetTaskResult(taskID string, result interface{}, errorMsg string) error {
	ctx := context.Background()
	taskKey := q.getTaskKey(taskID)

	updates := map[string]interface{}{
		"finished_at": time.Now().Unix(),
	}

	if errorMsg != "" {
		updates["error"] = errorMsg
		updates["status"] = "failed"
	} else {
		updates["status"] = "success"
		if result != nil {
			resultData, err := json.Marshal(result)
			if err != nil {
				return fmt.Errorf("序列化任务结果失败: %v", err)
			}
			updates["result"] = string(resultData)
		}
	}

	// 更新任务状态（处理队列的清理由恢复服务处理）
	if err := q.client.HSet(ctx, taskKey, updates).Err(); err != nil {
		return fmt.Errorf("设置任务结果失败: %v", err)
	}

	return nil
}

// GetQueueStats 获取队列统计信息
func (q *RedisQueue) GetQueueStats() (map[string]int, error) {
	ctx := context.Background()
	stats := make(map[string]int)

	// 统计各优先级队列长度
	for priority := 1; priority <= 10; priority++ {
		queueKey := q.getQueueKey(priority)
		length, err := q.client.LLen(ctx, queueKey).Result()
		if err != nil {
			return nil, fmt.Errorf("获取队列长度失败: %v", err)
		}
		stats[fmt.Sprintf("priority_%d", priority)] = int(length)
	}

	// 计算总任务数
	total := 0
	for _, count := range stats {
		total += count
	}
	stats["total"] = total

	return stats, nil
}

// ClearQueue 清空指定优先级的队列
func (q *RedisQueue) ClearQueue(priority int) error {
	ctx := context.Background()
	queueKey := q.getQueueKey(priority)
	return q.client.Del(ctx, queueKey).Err()
}

// RemoveTask 从队列中移除任务
func (q *RedisQueue) RemoveTask(taskID string) error {
	ctx := context.Background()
	taskKey := q.getTaskKey(taskID)
	return q.client.Del(ctx, taskKey).Err()
}

// 辅助方法

// getQueueKey 获取队列键名
func (q *RedisQueue) getQueueKey(priority int) string {
	return fmt.Sprintf("%s:priority:%d", q.prefix, priority)
}

// getTaskKey 获取任务键名
func (q *RedisQueue) getTaskKey(taskID string) string {
	return fmt.Sprintf("%s:task:%s", q.prefix, taskID)
}

// getProcessingKey 获取处理中任务键名
func (q *RedisQueue) getProcessingKey(taskID string) string {
	return fmt.Sprintf("%s:processing:%s", q.prefix, taskID)
}

// getHeartbeatKey 获取任务心跳键名
func (q *RedisQueue) getHeartbeatKey(taskID string) string {
	return fmt.Sprintf("%s:heartbeat:%s", q.prefix, taskID)
}

// getProcessingQueueKey 获取处理中队列键名
func (q *RedisQueue) getProcessingQueueKey() string {
	return fmt.Sprintf("%s:processing", q.prefix)
}

// markTaskAsFailed 标记任务为失败状态
func (q *RedisQueue) markTaskAsFailed(taskID, errorMsg string) {
	ctx := context.Background()
	taskKey := q.getTaskKey(taskID)
	
	q.client.HSet(ctx, taskKey, map[string]interface{}{
		"status":       "failed",
		"error":        errorMsg,
		"finished_at":  time.Now().Unix(),
	})
}

// RecoverOrphanedTasks 恢复孤儿任务（定期扫描处理中队列）
func (q *RedisQueue) RecoverOrphanedTasks() error {
	ctx := context.Background()
	processingQueue := q.getProcessingQueueKey()
	
	// 获取所有处理中的任务
	tasks, err := q.client.LRange(ctx, processingQueue, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("获取处理中任务失败: %v", err)
	}
	
	currentTime := time.Now().Unix()
	
	for _, taskStr := range tasks {
		var message TaskMessage
		if err := json.Unmarshal([]byte(taskStr), &message); err != nil {
			continue // 跳过无效任务
		}
		
		taskKey := q.getTaskKey(message.TaskID)
		
		// 检查任务状态
		status, err := q.client.HGet(ctx, taskKey, "status").Result()
		if err != nil {
			continue // 任务状态不存在，跳过
		}
		
		// 如果任务已完成，从处理队列中清理
		if status == "success" || status == "failed" {
			q.client.LRem(ctx, processingQueue, 1, taskStr)
			continue
		}
		
		// 如果任务仍在处理中，检查是否超时
		if status == "processing" {
			dequeuedAtStr, err := q.client.HGet(ctx, taskKey, "dequeued_at").Result()
			if err != nil {
				continue
			}
			
			dequeuedAt, err := strconv.ParseInt(dequeuedAtStr, 10, 64)
			if err != nil {
				continue
			}
			
			// 如果任务处理时间超过5分钟，认为Worker已经挂了
			if currentTime-dequeuedAt > 5*60 {
				q.recoverSingleOrphanedTask(taskStr, message)
			}
		}
	}
	
	return nil
}

// recoverSingleOrphanedTask 恢复单个孤儿任务
func (q *RedisQueue) recoverSingleOrphanedTask(taskStr string, message TaskMessage) {
	ctx := context.Background()
	processingQueue := q.getProcessingQueueKey()
	queueKey := q.getQueueKey(message.Priority)
	taskKey := q.getTaskKey(message.TaskID)
	
	pipeline := q.client.Pipeline()
	
	// 从处理中队列移除
	pipeline.LRem(ctx, processingQueue, 1, taskStr)
	
	// 重新入队
	pipeline.LPush(ctx, queueKey, taskStr)
	
	// 更新任务状态
	pipeline.HSet(ctx, taskKey, map[string]interface{}{
		"status":       "queued",
		"recovered_at": time.Now().Unix(),
		"error":        "任务超时，Worker可能已崩溃，重新入队",
	})
	
	pipeline.Exec(ctx)
}

// RenewTaskHeartbeat 续约任务心跳
func (q *RedisQueue) RenewTaskHeartbeat(taskID string) error {
	ctx := context.Background()
	heartbeatKey := q.getHeartbeatKey(taskID)
	
	// 延长TTL到2分钟
	return q.client.Expire(ctx, heartbeatKey, 2*time.Minute).Err()
}

// GetClient 获取Redis客户端（用于高级操作）
func (q *RedisQueue) GetClient() *redis.Client {
	return q.client
}
