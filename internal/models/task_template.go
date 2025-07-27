package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// TaskTemplate 任务模板
type TaskTemplate struct {
	ID       uint   `gorm:"primarykey" json:"id"`
	TenantID uint   `gorm:"not null;index;uniqueIndex:idx_tenant_code" json:"tenant_id"`
	Name     string `gorm:"size:100;not null" json:"name"`
	Code     string `gorm:"size:50;not null;uniqueIndex:idx_tenant_code" json:"code"`
	
	// 脚本信息
	ScriptType     string         `gorm:"size:20;not null" json:"script_type"`      // shell/ansible
	EntryFile      string         `gorm:"size:500;not null" json:"entry_file"`      // 主执行文件路径
	IncludedFiles  IncludedFiles  `gorm:"type:jsonb" json:"included_files"`         // 包含的文件列表
	
	// Git来源信息（快照，不是外键）
	SourceGitInfo  JSON           `gorm:"type:jsonb" json:"source_git_info"`        // Git仓库来源信息
	
	// 模板配置
	Description string              `gorm:"type:text" json:"description"`
	Parameters  TemplateParameters  `gorm:"type:jsonb" json:"parameters"`
	Timeout     int                 `gorm:"default:300" json:"timeout"` // 超时时间(秒)
	
	// 执行配置
	ExecutionType string `gorm:"size:20;default:'ssh'" json:"execution_type"` // ssh/ansible
	RequireSudo   bool   `gorm:"default:false" json:"require_sudo"`
	
	// 审计字段
	CreatedBy uint      `gorm:"not null" json:"created_by"`
	UpdatedBy uint      `gorm:"not null" json:"updated_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	// 关联
	Tenant     Tenant        `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

// TableName 指定表名
func (TaskTemplate) TableName() string {
	return "task_templates"
}

// TemplateParameter 模板参数定义
type TemplateParameter struct {
	Name         string      `json:"name"`
	Type         string      `json:"type"`         // string, select, multiselect, datetime, password
	Label        string      `json:"label"`        // 显示名称
	Description  string      `json:"description"`
	Required     bool        `json:"required"`
	DefaultValue interface{} `json:"default_value,omitempty"`
	Options      []string    `json:"options,omitempty"`      // select/multiselect的选项
	Placeholder  string      `json:"placeholder,omitempty"`
	Validation   string      `json:"validation,omitempty"`   // 验证规则
}

// TemplateParameters 模板参数列表
type TemplateParameters []TemplateParameter

// Value 实现 driver.Valuer 接口
func (p TemplateParameters) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return json.Marshal(p)
}

// Scan 实现 sql.Scanner 接口
func (p *TemplateParameters) Scan(value interface{}) error {
	if value == nil {
		*p = nil
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), p)
	}
	
	return json.Unmarshal(bytes, p)
}

// IncludedFile 包含的文件信息
type IncludedFile struct {
	Path     string `json:"path"`                  // 文件路径
	FileType string `json:"file_type,omitempty"`   // 文件类型
}

// IncludedFiles 包含的文件列表
type IncludedFiles []IncludedFile

// Value 实现 driver.Valuer 接口
func (f IncludedFiles) Value() (driver.Value, error) {
	if f == nil {
		return nil, nil
	}
	return json.Marshal(f)
}

// Scan 实现 sql.Scanner 接口
func (f *IncludedFiles) Scan(value interface{}) error {
	if value == nil {
		*f = nil
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), f)
	}
	
	return json.Unmarshal(bytes, f)
}