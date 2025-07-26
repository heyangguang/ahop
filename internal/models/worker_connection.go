package models

import (
	"time"
)

// WorkerConnection Worker连接状态
type WorkerConnection struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	WorkerID     string    `gorm:"uniqueIndex;size:100;not null" json:"worker_id"`
	IPAddress    string    `gorm:"size:50" json:"ip_address"`
	ConnectedAt  time.Time `gorm:"not null" json:"connected_at"`
	LastHeartbeat time.Time `gorm:"not null" json:"last_heartbeat"`
	Status       string    `gorm:"size:20;default:'active'" json:"status"` // active, disconnected
	AccessKey    string    `gorm:"size:100" json:"access_key"`
	Metadata     string    `gorm:"type:text" json:"metadata"` // 额外信息，如版本号等
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 指定表名
func (WorkerConnection) TableName() string {
	return "worker_connections"
}