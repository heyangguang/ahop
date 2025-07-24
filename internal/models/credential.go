package models

import (
	"time"
)

// CredentialType 凭证类型
type CredentialType string

const (
	CredentialTypePassword    CredentialType = "password"    // 用户名密码
	CredentialTypeSSHKey      CredentialType = "ssh_key"     // SSH密钥
	CredentialTypeAPIKey      CredentialType = "api_key"     // API密钥
	CredentialTypeToken       CredentialType = "token"       // Token凭证
	CredentialTypeCertificate CredentialType = "certificate" // 证书
)

// Credential 凭证模型
type Credential struct {
	BaseModel
	TenantID    uint           `gorm:"not null;index:idx_tenant_type" json:"tenant_id"`
	Name        string         `gorm:"size:100;not null" json:"name"`
	Type        CredentialType `gorm:"size:20;not null;index:idx_tenant_type" json:"type"`
	Description string         `gorm:"size:500" json:"description"`

	// 凭证内容（根据类型不同存储不同内容）
	Username    string `gorm:"size:100" json:"username,omitempty"`    // 用户名（password类型）
	Password    string `gorm:"size:500" json:"-"`                     // 密码（加密存储）
	PrivateKey  string `gorm:"type:text" json:"-"`                    // 私钥（ssh_key类型，加密存储）
	PublicKey   string `gorm:"type:text" json:"public_key,omitempty"` // 公钥（ssh_key类型）
	APIKey      string `gorm:"size:500" json:"-"`                     // API密钥（api_key类型，加密存储）
	Token       string `gorm:"size:2000" json:"-"`                    // Token（token类型，加密存储）
	Certificate string `gorm:"type:text" json:"-"`                    // 证书内容（certificate类型，加密存储）
	Passphrase  string `gorm:"size:500" json:"-"`                     // 密钥密码（ssh_key/certificate类型，加密存储）

	// ACL限制
	AllowedHosts  string     `gorm:"type:text" json:"allowed_hosts"`   // 允许的主机（逗号分隔）
	AllowedIPs    string     `gorm:"type:text" json:"allowed_ips"`     // 允许的IP范围（CIDR格式，逗号分隔）
	DeniedHosts   string     `gorm:"type:text" json:"denied_hosts"`    // 禁止的主机（逗号分隔）
	DeniedIPs     string     `gorm:"type:text" json:"denied_ips"`      // 禁止的IP范围（CIDR格式，逗号分隔）
	ExpiresAt     *time.Time `json:"expires_at"`                       // 过期时间
	MaxUsageCount int        `gorm:"default:0" json:"max_usage_count"` // 最大使用次数（0表示无限制）
	UsageCount    int        `gorm:"default:0" json:"usage_count"`     // 已使用次数

	// 审计字段
	LastUsedAt  *time.Time `json:"last_used_at"`                  // 最后使用时间
	LastUsedBy  uint       `json:"last_used_by"`                  // 最后使用者ID
	LastUsedFor string     `gorm:"size:200" json:"last_used_for"` // 最后使用用途

	// 标准字段
	IsActive  bool `gorm:"default:true" json:"is_active"` // 是否启用
	CreatedBy uint `json:"created_by"`
	UpdatedBy uint `json:"updated_by"`

	// 关联
	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
	Tags   []Tag  `gorm:"many2many:credential_tags;" json:"tags,omitempty"`
}

// TableName 指定表名
func (Credential) TableName() string {
	return "credentials"
}

// CredentialUsageLog 凭证使用日志
type CredentialUsageLog struct {
	BaseModel
	TenantID     uint   `gorm:"not null;index" json:"tenant_id"`
	CredentialID uint   `gorm:"not null;index" json:"credential_id"`
	UserID       uint   `gorm:"not null" json:"user_id"`
	HostID       *uint  `json:"host_id,omitempty"`         // 关联的主机ID（如果有）
	HostName     string `gorm:"size:100" json:"host_name"` // 目标主机名
	HostIP       string `gorm:"size:45" json:"host_ip"`    // 目标IP
	Purpose      string `gorm:"size:200" json:"purpose"`   // 使用目的
	Success      bool   `json:"success"`                   // 是否成功
	ErrorMessage string `gorm:"size:500" json:"error_message,omitempty"`

	// 关联
	Credential Credential `gorm:"foreignKey:CredentialID" json:"-"`
	User       User       `gorm:"foreignKey:UserID" json:"-"`
}

// TableName 指定表名
func (CredentialUsageLog) TableName() string {
	return "credential_usage_logs"
}
