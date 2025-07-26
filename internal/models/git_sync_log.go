package models

import (
	"time"
)

// GitSyncLog Git同步日志
type GitSyncLog struct {
	ID           uint `gorm:"primarykey" json:"id"`
	RepositoryID uint `gorm:"not null;index" json:"repository_id"`
	TenantID     uint `gorm:"not null;index" json:"tenant_id"`
	
	// 任务信息
	TaskID   string `gorm:"size:36;index" json:"task_id,omitempty"`
	TaskType string `gorm:"size:20;not null" json:"task_type"` // scheduled/manual/initial
	
	// Worker信息
	WorkerID string `gorm:"size:100;not null" json:"worker_id"`
	
	// 操作者信息（用于manual类型）
	OperatorID *uint `gorm:"index" json:"operator_id,omitempty"` // 触发同步的用户ID
	
	// 时间信息
	StartedAt  time.Time  `gorm:"not null" json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Duration   int        `json:"duration,omitempty"` // 秒
	
	// 同步结果
	Status       string `gorm:"size:20;not null" json:"status"` // success/failed
	FromCommit   string `gorm:"size:40" json:"from_commit,omitempty"`
	ToCommit     string `gorm:"size:40" json:"to_commit,omitempty"`
	ErrorMessage string `gorm:"type:text" json:"error_message,omitempty"`
	
	// 同步详情
	LocalPath     string `gorm:"size:500" json:"local_path,omitempty"`     // 本地仓库路径
	CommandOutput string `gorm:"type:text" json:"command_output,omitempty"` // Git命令输出
	
	// 时间戳
	CreatedAt time.Time `json:"created_at"`
	
	// 关联
	Repository GitRepository `gorm:"foreignKey:RepositoryID" json:"repository,omitempty"`
}

// TableName 指定表名
func (GitSyncLog) TableName() string {
	return "git_sync_logs"
}