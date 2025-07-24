package executor

import (
	"context"
	"time"
)

// TaskContext 任务执行上下文
type TaskContext struct {
	TaskID   string
	TaskType string
	TenantID uint
	Params   map[string]interface{}
	Timeout  time.Duration
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
type LogCallback func(level, source, message string, hostName string, data interface{})

// Executor 任务执行器接口
type Executor interface {
	// Execute 执行任务
	Execute(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult
	
	// GetSupportedTypes 获取支持的任务类型
	GetSupportedTypes() []string
	
	// ValidateParams 验证任务参数
	ValidateParams(params map[string]interface{}) error
}

// BaseExecutor 基础执行器
type BaseExecutor struct {
	supportedTypes []string
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

// LogProgress 记录进度
func (e *BaseExecutor) LogProgress(onProgress ProgressCallback, progress int, message string) {
	if onProgress != nil {
		onProgress(progress, message)
	}
}

// LogMessage 记录日志
func (e *BaseExecutor) LogMessage(onLog LogCallback, level, source, message string, hostName string, data interface{}) {
	if onLog != nil {
		onLog(level, source, message, hostName, data)
	}
}