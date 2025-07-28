package models

import (
	"time"
)

// ScheduledTask 定时任务配置表
type ScheduledTask struct {
	BaseModel
	TenantID uint `gorm:"not null;index" json:"tenant_id"`

	// 基本信息
	Name        string `gorm:"size:200;not null" json:"name"`
	Description string `gorm:"size:500" json:"description"`
	CronExpr    string `gorm:"size:100;not null" json:"cron_expr"`  // cron表达式
	IsActive    bool   `gorm:"default:true;index" json:"is_active"` // 是否启用

	// 关联任务模板
	TemplateID uint `gorm:"not null;index" json:"template_id"` // 关联task_templates表

	// 执行配置
	HostIDs     JSON `gorm:"type:jsonb" json:"host_ids"`     // JSON数组，存储[]uint
	Variables   JSON `gorm:"type:jsonb" json:"variables"`     // 模板变量值
	TimeoutMins int  `gorm:"default:60" json:"timeout_mins"` // 执行超时（分钟）

	// 执行状态
	LastRunAt  *time.Time `gorm:"index" json:"last_run_at"`        // 上次执行时间
	NextRunAt  *time.Time `gorm:"index" json:"next_run_at"`        // 下次执行时间
	LastStatus string     `gorm:"size:20" json:"last_status"`      // idle/running/success/failed
	LastTaskID string     `gorm:"size:36" json:"last_task_id"`     // 最后创建的任务ID
	RunCount   int64      `gorm:"default:0" json:"run_count"`      // 总执行次数

	// 审计
	CreatedBy uint `gorm:"not null" json:"created_by"`
	UpdatedBy uint `json:"updated_by"`

	// 关联
	Template TaskTemplate `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
}

// TableName 指定表名
func (ScheduledTask) TableName() string {
	return "scheduled_tasks"
}

// ScheduledTaskExecution 定时任务执行历史表
type ScheduledTaskExecution struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	ScheduledTaskID uint      `gorm:"not null;index" json:"scheduled_task_id"`
	TaskID          string    `gorm:"size:36;not null;index" json:"task_id"` // 关联tasks表
	TriggeredAt     time.Time `gorm:"not null;index" json:"triggered_at"`

	// 关联
	ScheduledTask ScheduledTask `gorm:"foreignKey:ScheduledTaskID;constraint:OnDelete:CASCADE" json:"scheduled_task,omitempty"`
	Task          *Task         `gorm:"-" json:"task,omitempty"` // 使用 gorm:"-" 阻止GORM自动管理
}

// TableName 指定表名
func (ScheduledTaskExecution) TableName() string {
	return "scheduled_task_executions"
}

// 定时任务状态常量
const (
	ScheduledTaskStatusIdle    = "idle"    // 空闲
	ScheduledTaskStatusRunning = "running" // 执行中
	ScheduledTaskStatusSuccess = "success" // 上次执行成功
	ScheduledTaskStatusFailed  = "failed"  // 上次执行失败
)