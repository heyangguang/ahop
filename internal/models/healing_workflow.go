package models

import (
	"time"
)

// HealingWorkflow 自愈工作流
type HealingWorkflow struct {
	BaseModel
	TenantID uint `gorm:"not null;index" json:"tenant_id"`

	// 基本信息
	Name        string `gorm:"size:200;not null" json:"name"`
	Code        string `gorm:"size:100;not null;uniqueIndex:idx_tenant_wf_code" json:"code"`
	Description string `gorm:"size:500" json:"description"`
	Version     int    `gorm:"default:1" json:"version"`
	IsActive    bool   `gorm:"default:true;index" json:"is_active"`

	// 工作流定义
	Definition JSON `gorm:"type:jsonb;not null" json:"definition"` // 工作流定义JSON

	// 执行配置
	TimeoutMinutes int  `gorm:"default:60" json:"timeout_minutes"`      // 执行超时（分钟）
	MaxRetries     int  `gorm:"default:0" json:"max_retries"`           // 最大重试次数
	AllowParallel  bool `gorm:"default:false" json:"allow_parallel"`    // 允许并行执行

	// 执行统计
	LastExecuteAt *time.Time `json:"last_execute_at"`
	ExecuteCount  int64      `gorm:"default:0" json:"execute_count"`
	SuccessCount  int64      `gorm:"default:0" json:"success_count"`
	FailureCount  int64      `gorm:"default:0" json:"failure_count"`
	AvgDuration   int        `gorm:"default:0" json:"avg_duration"` // 平均执行时长（秒）

	// 审计
	CreatedBy uint `gorm:"not null" json:"created_by"`
	UpdatedBy uint `json:"updated_by"`

	// 关联
	Tenant Tenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

// TableName 指定表名
func (HealingWorkflow) TableName() string {
	return "healing_workflows"
}

// WorkflowDefinition 工作流定义结构
type WorkflowDefinition struct {
	Nodes       []WorkflowNode       `json:"nodes"`       // 节点列表
	Connections []WorkflowConnection `json:"connections"` // 节点连接
	Variables   map[string]interface{} `json:"variables"` // 全局变量
}

// WorkflowNode 工作流节点
type WorkflowNode struct {
	ID          string                 `json:"id"`          // 节点唯一ID
	Name        string                 `json:"name"`        // 节点名称
	Type        string                 `json:"type"`        // 节点类型
	Config      map[string]interface{} `json:"config"`      // 节点配置
	NextNodes   []string               `json:"next_nodes"`  // 下一个节点ID列表
	ErrorHandle string                 `json:"error_handle"` // 错误处理：stop/continue/retry
}

// WorkflowConnection 节点连接
type WorkflowConnection struct {
	From      string `json:"from"`      // 源节点ID
	To        string `json:"to"`        // 目标节点ID
	Condition string `json:"condition"` // 连接条件
}

// 节点类型常量
const (
	NodeTypeStart        = "start"         // 开始节点
	NodeTypeEnd          = "end"           // 结束节点
	NodeTypeCondition    = "condition"     // 条件判断
	NodeTypeDataProcess  = "data_process"  // 数据处理
	NodeTypeTaskExecute  = "task_execute"  // 任务执行
	NodeTypeControl      = "control"       // 控制节点（等待、终止等）
	NodeTypeTicketUpdate = "ticket_update" // 工单更新
)

// 错误处理方式
const (
	ErrorHandleStop     = "stop"     // 停止执行
	ErrorHandleContinue = "continue" // 继续执行
	ErrorHandleRetry    = "retry"    // 重试
)