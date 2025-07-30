package models

import (
	"time"
)

// HealingExecution 自愈执行实例
type HealingExecution struct {
	BaseModel
	TenantID uint `gorm:"not null;index" json:"tenant_id"`

	// 执行标识
	ExecutionID string `gorm:"size:36;not null;uniqueIndex" json:"execution_id"`

	// 关联信息
	RuleID     *uint `gorm:"index" json:"rule_id"`     // 触发规则ID（手动触发时为空）
	WorkflowID uint  `gorm:"not null;index" json:"workflow_id"`

	// 触发信息
	TriggerType   string `gorm:"size:20;not null" json:"trigger_type"`   // scheduled/manual/api
	TriggerSource JSON   `gorm:"type:jsonb" json:"trigger_source"`       // 触发来源详情
	TriggerUser   *uint  `json:"trigger_user"`                           // 手动触发的用户ID

	// 执行状态
	Status      string     `gorm:"size:20;not null;index" json:"status"`      // pending/running/success/failed/cancelled
	StartTime   time.Time  `gorm:"not null;index" json:"start_time"`
	EndTime     *time.Time `json:"end_time"`
	Duration    int        `json:"duration"`                                   // 执行时长（秒）

	// 执行上下文
	Context     JSON `gorm:"type:jsonb" json:"context"`     // 执行上下文（变量等）
	NodeStates  JSON `gorm:"type:jsonb" json:"node_states"`  // 各节点执行状态

	// 执行结果
	Result      JSON   `gorm:"type:jsonb" json:"result"`      // 执行结果
	ErrorMsg    string `gorm:"type:text" json:"error_msg"`    // 错误信息
	RetryCount  int    `gorm:"default:0" json:"retry_count"`  // 重试次数

	// 关联
	Rule     *HealingRule    `gorm:"foreignKey:RuleID" json:"rule,omitempty"`
	Workflow HealingWorkflow `gorm:"foreignKey:WorkflowID" json:"workflow,omitempty"`
	Tenant   Tenant          `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`

}

// TableName 指定表名
func (HealingExecution) TableName() string {
	return "healing_executions"
}

// HealingExecutionLog 执行日志
type HealingExecutionLog struct {
	ID            uint   `gorm:"primarykey" json:"id"`
	ExecutionID   uint   `gorm:"not null;index" json:"execution_id"` // 关联HealingExecution.ID
	NodeID        string `gorm:"size:100;not null;index" json:"node_id"`

	// 日志信息
	Level     string    `gorm:"size:20;not null" json:"level"`     // debug/info/warn/error
	Message   string    `gorm:"type:text;not null" json:"message"`
	Timestamp time.Time `gorm:"not null;index" json:"timestamp"`

	// 节点执行信息
	NodeType   string     `gorm:"size:50" json:"node_type"`
	NodeName   string     `gorm:"size:200" json:"node_name"`
	StartTime  *time.Time `json:"start_time"`
	EndTime    *time.Time `json:"end_time"`
	Duration   int        `json:"duration"` // 毫秒

	// 输入输出
	Input  JSON `gorm:"type:jsonb" json:"input"`
	Output JSON `gorm:"type:jsonb" json:"output"`
	Error  JSON `gorm:"type:jsonb" json:"error"`

	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名
func (HealingExecutionLog) TableName() string {
	return "healing_execution_logs"
}

// NodeState 节点执行状态
type NodeState struct {
	NodeID    string                 `json:"node_id"`
	Status    string                 `json:"status"`    // pending/running/success/failed/skipped
	StartTime *time.Time             `json:"start_time"`
	EndTime   *time.Time             `json:"end_time"`
	Input     map[string]interface{} `json:"input"`
	Output    map[string]interface{} `json:"output"`
	Error     string                 `json:"error,omitempty"`
}

// 执行状态常量
const (
	ExecutionStatusPending   = "pending"
	ExecutionStatusRunning   = "running"
	ExecutionStatusSuccess   = "success"
	ExecutionStatusFailed    = "failed"
	ExecutionStatusCancelled = "cancelled"
)

// 日志级别常量
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)