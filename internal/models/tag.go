package models

// Tag 标签模型
type Tag struct {
	BaseModel
	TenantID uint   `gorm:"not null;index;uniqueIndex:idx_tenant_key_value" json:"tenant_id"`
	Key      string `gorm:"size:50;not null;index;uniqueIndex:idx_tenant_key_value" json:"key"`
	Value    string `gorm:"size:100;not null;index;uniqueIndex:idx_tenant_key_value" json:"value"`
	Color    string `gorm:"size:7;default:'#2196F3'" json:"color"` // 默认蓝色

	// 关联
	Tenant      Tenant       `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Credentials []Credential `gorm:"many2many:credential_tags;" json:"-"`
}

// TableName 指定表名
func (Tag) TableName() string {
	return "tags"
}