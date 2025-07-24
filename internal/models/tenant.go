package models

// Tenant 租户模型 - 贫血模型，只包含数据结构
type Tenant struct {
	BaseModel
	Name      string `json:"name" gorm:"not null;size:100"`
	Code      string `json:"code" gorm:"unique;not null;size:50;index"`
	Status    string `json:"status" gorm:"default:'active';size:20"`
	UserCount int    `json:"user_count" gorm:"-"` // 用户数量，不存储在数据库中
}

// TableName 表名
func (t *Tenant) TableName() string {
	return "tenants"
}

// 租户状态常量
const (
	TenantStatusActive   = "active"
	TenantStatusInactive = "inactive"
)
