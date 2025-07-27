package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// TicketPlugin 工单插件配置
type TicketPlugin struct {
	ID          uint   `gorm:"primarykey" json:"id"`
	TenantID    uint   `gorm:"not null;index;uniqueIndex:idx_tenant_code" json:"tenant_id"`
	Name        string `gorm:"size:100;not null" json:"name"`                            // 插件名称
	Code        string `gorm:"size:50;not null;uniqueIndex:idx_tenant_code" json:"code"` // 唯一标识
	Description string `gorm:"type:text" json:"description"`                             // 描述信息

	// 接口配置
	BaseURL   string `gorm:"size:500;not null" json:"base_url"`       // 插件地址
	AuthType  string `gorm:"size:20;default:'none'" json:"auth_type"` // none/bearer/apikey
	AuthToken string `gorm:"size:500" json:"-"`                       // 加密存储，不返回给前端

	// 同步配置
	SyncEnabled  bool       `gorm:"default:true" json:"sync_enabled"`   // 是否启用同步
	SyncInterval int        `gorm:"default:5" json:"sync_interval"`     // 同步间隔(分钟)
	SyncWindow   int        `gorm:"default:60" json:"sync_window"`      // 数据获取时间窗口(分钟)
	LastSyncAt   *time.Time `json:"last_sync_at"`                       // 最后同步时间

	// 状态信息
	Status       string `gorm:"size:20;default:'active'" json:"status"` // active/inactive/error
	ErrorMessage string `gorm:"type:text" json:"error_message"`         // 错误信息

	// 租户和时间戳
	Tenant    *Tenant   `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FieldMapping 字段映射配置
type FieldMapping struct {
	ID           uint   `gorm:"primarykey" json:"id"`
	PluginID     uint   `gorm:"not null;index" json:"plugin_id"`
	SourceField  string `gorm:"size:100;not null" json:"source_field"` // 源字段路径
	TargetField  string `gorm:"size:100;not null" json:"target_field"` // 目标字段
	DefaultValue string `gorm:"size:200" json:"default_value"`         // 默认值
	Required     bool   `gorm:"default:false" json:"required"`         // 是否必需

	// 关联
	Plugin    *TicketPlugin `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// SyncRule 同步过滤规则
type SyncRule struct {
	ID       uint   `gorm:"primarykey" json:"id"`
	PluginID uint   `gorm:"not null;index" json:"plugin_id"`
	Name     string `gorm:"size:100;not null" json:"name"`    // 规则名称
	Field    string `gorm:"size:100;not null" json:"field"`   // 检查字段
	Operator string `gorm:"size:20;not null" json:"operator"` // contains/equals/not_equals/in/regex
	Value    string `gorm:"type:text;not null" json:"value"`  // 匹配值
	Action   string `gorm:"size:20;not null" json:"action"`   // include/exclude
	Priority int    `gorm:"default:0" json:"priority"`        // 优先级(越小越先执行)
	Enabled  bool   `gorm:"default:true" json:"enabled"`      // 是否启用

	// 关联
	Plugin    *TicketPlugin `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// Ticket 工单信息
type Ticket struct {
	ID         uint   `gorm:"primarykey" json:"id"`
	TenantID   uint   `gorm:"not null;index" json:"tenant_id"`
	PluginID   uint   `gorm:"not null;index" json:"plugin_id"`
	ExternalID string `gorm:"size:100;not null;uniqueIndex:idx_plugin_external" json:"external_id"` // 外部ID

	// 核心字段
	Title       string `gorm:"size:500;not null" json:"title"`       // 工单标题
	Description string `gorm:"type:text" json:"description"`         // 详细描述
	Status      string `gorm:"size:50;not null;index" json:"status"` // open/in_progress/resolved/closed
	Priority    string `gorm:"size:20;index" json:"priority"`        // critical/high/medium/low
	Type        string `gorm:"size:50" json:"type"`                  // incident/problem/change

	// 人员信息
	Reporter string `gorm:"size:100" json:"reporter"` // 报告人
	Assignee string `gorm:"size:100" json:"assignee"` // 处理人

	// 分类信息
	Category string `gorm:"size:100;index" json:"category"` // 分类
	Service  string `gorm:"size:100;index" json:"service"`  // 影响的服务

	// 时间信息
	ExternalCreatedAt time.Time `json:"external_created_at"` // 工单系统中的创建时间
	ExternalUpdatedAt time.Time `json:"external_updated_at"` // 工单系统中的更新时间

	// 扩展信息
	Tags       StringArray `gorm:"type:text" json:"tags"`         // 标签
	CustomData JSON        `gorm:"type:jsonb" json:"custom_data"` // 自定义数据

	// 系统信息
	SyncedAt time.Time `json:"synced_at"` // 同步时间

	// 关联
	Tenant *Tenant       `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Plugin *TicketPlugin `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}


// StringArray 字符串数组类型，用于PostgreSQL
type StringArray []string

// Value 实现 driver.Valuer 接口
func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "[]", nil
	}
	return json.Marshal(a)
}

// Scan 实现 sql.Scanner 接口
func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = []string{}
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	
	return json.Unmarshal(bytes, a)
}

