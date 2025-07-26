package models

import (
	"time"
)

// GitRepository Git仓库模型
type GitRepository struct {
	ID       uint   `gorm:"primarykey" json:"id"`
	TenantID uint   `gorm:"not null;index" json:"tenant_id"`
	
	// 基本信息
	Name        string `gorm:"size:100;not null" json:"name"`
	Code        string `gorm:"size:50;not null;uniqueIndex:idx_tenant_git_repo" json:"code"`
	Description string `gorm:"size:500" json:"description"`
	
	// Git配置
	URL          string `gorm:"size:500;not null" json:"url"`
	Branch       string `gorm:"size:50;default:'main'" json:"branch"`
	IsPublic     bool   `gorm:"default:false" json:"is_public"`
	CredentialID *uint  `gorm:"index" json:"credential_id,omitempty"`
	
	// 同步配置
	SyncEnabled     bool       `gorm:"default:false" json:"sync_enabled"`
	SyncCron        string     `gorm:"size:50" json:"sync_cron,omitempty"`        // cron表达式
	LastScheduledAt *time.Time `json:"last_scheduled_at,omitempty"`
	
	// 状态信息
	Status         string     `gorm:"size:20;default:'active'" json:"status"`
	LastSyncAt     *time.Time `json:"last_sync_at,omitempty"`
	LastSyncCommit string     `gorm:"size:40" json:"last_sync_commit,omitempty"`
	
	// 本地路径（相对路径，如：1/123）
	LocalPath      string     `gorm:"size:200" json:"local_path"`
	
	// 时间戳
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	// 关联
	Credential *Credential `gorm:"foreignKey:CredentialID" json:"credential,omitempty"`
	Tenant     Tenant      `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

// TableName 指定表名
func (GitRepository) TableName() string {
	return "git_repositories"
}