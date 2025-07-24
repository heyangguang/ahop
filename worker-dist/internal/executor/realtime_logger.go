package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	
	"github.com/go-redis/redis/v8"
)

// RealtimeLogger 实时日志记录器（专门为WebSocket设计）
type RealtimeLogger struct {
	redisClient *redis.Client
	taskID      string
	enabled     bool
}

// NewRealtimeLogger 创建实时日志记录器
func NewRealtimeLogger(redisClient *redis.Client, taskID string) *RealtimeLogger {
	return &RealtimeLogger{
		redisClient: redisClient,
		taskID:      taskID,
		enabled:     redisClient != nil && taskID != "",
	}
}

// LogMessage 发送实时日志消息
func (rl *RealtimeLogger) LogMessage(level, source, message, hostName, stderr string) {
	if !rl.enabled {
		return
	}
	
	// 构建日志消息，始终包含stderr字段
	logMsg := map[string]interface{}{
		"task_id":   rl.taskID,
		"timestamp": time.Now().Unix(),
		"level":     level,
		"source":    source,
		"message":   message,
		"host_name": hostName,
		"stderr":    stderr, // 始终包含，即使为空
	}
	
	// 同步发送到Redis（确保实时性）
	rl.publish(logMsg)
}

// LogOutput 记录命令输出（用于Ansible等详细输出）
func (rl *RealtimeLogger) LogOutput(source, line, hostName string) {
	if !rl.enabled || line == "" {
		return
	}
	
	// 根据内容判断日志级别
	level := "output"
	if contains(line, "TASK", "PLAY") {
		level = "info"
	} else if contains(line, "ok:", "changed:") {
		level = "success"
	} else if contains(line, "failed:", "FAILED", "UNREACHABLE") {
		level = "error"
	}
	
	rl.LogMessage(level, source, line, hostName, "")
}

// LogError 记录错误输出
func (rl *RealtimeLogger) LogError(source, line, hostName string) {
	if !rl.enabled || line == "" {
		return
	}
	
	// 错误输出同时放在message和stderr中
	rl.LogMessage("error", source, line, hostName, line)
}

// publish 发布消息到Redis
func (rl *RealtimeLogger) publish(logMsg map[string]interface{}) {
	channel := fmt.Sprintf("task:logs:%s", rl.taskID)
	
	if msgData, err := json.Marshal(logMsg); err == nil {
		// 使用较短的超时时间，确保快速发送
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		rl.redisClient.Publish(ctx, channel, msgData)
	}
}

// contains 检查字符串是否包含任意一个子串
func contains(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}