package types

import "time"

// HostInfo 主机连接信息
type HostInfo struct {
	ID         uint            `json:"id"`
	IP         string          `json:"ip"`
	Port       int             `json:"port"`
	Hostname   string          `json:"hostname"`
	Credential *CredentialInfo `json:"credential"`
}

// CredentialInfo 凭证信息（解密后）
type CredentialInfo struct {
	Type       string `json:"type"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

// GitSyncMessage Git同步消息
type GitSyncMessage struct {
	Action       string `json:"action"` // sync/delete
	TenantID     uint   `json:"tenant_id"`
	RepositoryID uint   `json:"repository_id"`
	Repository   RepositoryInfo `json:"repository"`
	TaskID       string `json:"task_id,omitempty"`   // 任务ID
	TaskType     string `json:"task_type,omitempty"` // 任务类型: scheduled/manual/initial
	OperatorID *uint              `json:"operator_id,omitempty"`
	Timestamp  time.Time          `json:"timestamp"`
	Metadata   map[string]string  `json:"metadata,omitempty"` // 额外的元数据
}

// RepositoryInfo 仓库信息
type RepositoryInfo struct {
	ID           uint            `json:"id"`
	Name         string          `json:"name"`
	URL          string          `json:"url"`
	Branch       string          `json:"branch"`
	IsPublic     bool            `json:"is_public"`
	CredentialID *uint           `json:"credential_id,omitempty"`
	Credential   *CredentialInfo `json:"credential,omitempty"`
	LocalPath    string          `json:"local_path"`
}

// TemplateCopyMessage 模板复制消息
type TemplateCopyMessage struct {
	Action       string   `json:"action"`       // copy/delete
	TemplateID   uint     `json:"template_id"`
	TenantID     uint     `json:"tenant_id"`
	TemplateCode string   `json:"template_code"`
	SourceRepo   string   `json:"source_repo"`
	SourceFiles  []string `json:"source_files"`
}