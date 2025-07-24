package executor

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// TaskContext 任务执行上下文
type TaskContext struct {
	TaskID     string
	TaskType   string
	TenantID   uint
	TenantName string                   // 租户名称
	UserID     uint                     // 发起人ID
	Username   string                   // 发起人用户名
	Source     string                   // 任务来源
	Params     map[string]interface{}
	Timeout    time.Duration
	CreatedAt  time.Time                // 任务创建时间
}

// TaskResult 任务执行结果
type TaskResult struct {
	Success bool                   `json:"success"`
	Result  interface{}            `json:"result,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Logs    []string               `json:"logs,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ProgressCallback 进度回调函数
type ProgressCallback func(progress int, message string)

// LogCallback 日志回调函数
type LogCallback func(level, source, message, hostName, stderr string)


// Executor 任务执行器接口
type Executor interface {
	// Execute 执行任务
	Execute(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult
	
	// GetSupportedTypes 获取支持的任务类型
	GetSupportedTypes() []string
	
	// ValidateParams 验证任务参数
	ValidateParams(params map[string]interface{}) error
	
	// SetRedisClient 设置Redis客户端（用于实时日志发布）
	SetRedisClient(client *redis.Client)
}

// LogEntry 日志条目
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Source    string                 `json:"source"`
	Message   string                 `json:"message"`
	HostName  string                 `json:"host_name,omitempty"`
	Stderr    string                 `json:"stderr,omitempty"`
}

// LogCollector 日志收集器
type LogCollector struct {
	mu          sync.Mutex
	entries     []LogEntry
	stdout      bytes.Buffer
	stderr      bytes.Buffer
	redisClient *redis.Client
	taskID      string
}

// NewLogCollector 创建日志收集器
func NewLogCollector(redisClient *redis.Client, taskID string) *LogCollector {
	return &LogCollector{
		redisClient: redisClient,
		taskID:      taskID,
	}
}

// AddEntry 添加日志条目
func (lc *LogCollector) AddEntry(level, source, message, hostName, stderr string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Source:    source,
		Message:   message,
		HostName:  hostName,
		Stderr:    stderr,
	}
	
	lc.entries = append(lc.entries, entry)
}


// AddStdout 添加标准输出
func (lc *LogCollector) AddStdout(data string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.stdout.WriteString(data)
}

// AddStderr 添加错误输出
func (lc *LogCollector) AddStderr(data string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.stderr.WriteString(data)
}

// GetAll 获取所有日志
func (lc *LogCollector) GetAll() ([]LogEntry, string, string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	return lc.entries, lc.stdout.String(), lc.stderr.String()
}

// BaseExecutor 基础执行器
type BaseExecutor struct {
	supportedTypes []string
	redisClient    *redis.Client
}

// NewBaseExecutor 创建基础执行器
func NewBaseExecutor(supportedTypes []string) *BaseExecutor {
	return &BaseExecutor{
		supportedTypes: supportedTypes,
	}
}

// GetSupportedTypes 获取支持的任务类型
func (e *BaseExecutor) GetSupportedTypes() []string {
	return e.supportedTypes
}

// ValidateParams 默认参数验证
func (e *BaseExecutor) ValidateParams(params map[string]interface{}) error {
	// 基础验证，子类可以重写
	return nil
}

// SetRedisClient 设置Redis客户端
func (e *BaseExecutor) SetRedisClient(client *redis.Client) {
	e.redisClient = client
}

// LogProgress 记录进度
func (e *BaseExecutor) LogProgress(onProgress ProgressCallback, progress int, message string) {
	if onProgress != nil {
		onProgress(progress, message)
	}
}

// LogMessage 记录日志
func (e *BaseExecutor) LogMessage(onLog LogCallback, level, source, message, hostName, stderr string) {
	if onLog != nil {
		onLog(level, source, message, hostName, stderr)
	}
}

// CreateLogCollector 创建日志收集器
func (e *BaseExecutor) CreateLogCollector(taskID string) *LogCollector {
	return NewLogCollector(e.redisClient, taskID)
}


