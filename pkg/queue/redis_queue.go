package queue

import (
	"context"
	"encoding/json"
	"fmt"
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
	TenantName string                 `json:"tenant_name"` // 租户名称
	UserID     uint                   `json:"user_id"`     // 发起人ID
	Username   string                 `json:"username"`    // 发起人用户名
	Priority   int                    `json:"priority"`
	Params     map[string]interface{} `json:"params"`
	Created    int64                  `json:"created"`
	Source     string                 `json:"source"` // 任务来源
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


// GetClient 获取Redis客户端（用于高级操作）
func (q *RedisQueue) GetClient() *redis.Client {
	return q.client
}

// PublishMessage 发布消息到指定频道
func (q *RedisQueue) PublishMessage(channel string, message interface{}) error {
	ctx := context.Background()
	
	// 序列化消息
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %v", err)
	}
	
	// 发布消息
	channelKey := fmt.Sprintf("%s:channel:%s", q.prefix, channel)
	if err := q.client.Publish(ctx, channelKey, data).Err(); err != nil {
		return fmt.Errorf("发布消息失败: %v", err)
	}
	
	return nil
}

// SubscribeChannel 订阅指定频道
func (q *RedisQueue) SubscribeChannel(channel string) *redis.PubSub {
	ctx := context.Background()
	channelKey := fmt.Sprintf("%s:channel:%s", q.prefix, channel)
	return q.client.Subscribe(ctx, channelKey)
}
