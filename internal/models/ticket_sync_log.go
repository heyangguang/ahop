package models

import (
	"time"
)

// TicketSyncLog 工单同步日志
type TicketSyncLog struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	PluginID      uint      `gorm:"not null;index" json:"plugin_id"`
	TenantID      uint      `gorm:"not null;index" json:"tenant_id"`
	
	// 同步信息
	StartTime     time.Time `json:"start_time"`                      // 开始时间
	EndTime       time.Time `json:"end_time"`                        // 结束时间
	Duration      int       `json:"duration"`                        // 耗时(秒)
	
	// 同步结果
	Status          string    `gorm:"size:20;not null" json:"status"`    // success/failed/partial
	TotalFetched    int       `json:"total_fetched"`                     // 获取的工单总数
	TotalFilteredOut int      `json:"total_filtered_out"`                // 被过滤掉的工单数
	TotalProcessed  int       `json:"total_processed"`                   // 处理的工单数（过滤后）
	TotalCreated    int       `json:"total_created"`                     // 新创建的工单数
	TotalUpdated    int       `json:"total_updated"`                     // 更新的工单数
	TotalFailed     int       `json:"total_failed"`                      // 失败的工单数
	
	// 错误信息
	ErrorMessage  string    `gorm:"type:text" json:"error_message"`
	ErrorDetails  JSON      `gorm:"type:jsonb" json:"error_details"` // 详细错误信息
	
	// 关联
	Plugin        *TicketPlugin `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
	
	CreatedAt     time.Time `json:"created_at"`
}

// TicketSyncError 工单同步错误详情
type TicketSyncError struct {
	ExternalID string `json:"external_id"`
	Error      string `json:"error"`
	Data       JSON   `json:"data,omitempty"`
}