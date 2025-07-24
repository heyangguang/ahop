package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// 任务类型常量
const (
	TaskTypePing    = "ping"    // 主机探活（网络ping）
	TaskTypeCollect = "collect" // 信息采集（Ansible setup）

	// TaskTypeTemplate 未来的任务类型
	TaskTypeTemplate = "template" // 任务模板（TODO: 未来实现）
)

// 执行器类型常量
const (
	ExecutorTypeAnsible = "ansible" // Ansible执行器
	ExecutorTypeShell   = "shell"   // Shell执行器（SSH）
)

// TaskParams 任务参数结构
type TaskParams struct {
	// 基础参数
	Hosts []TaskHost `json:"hosts" binding:"required"`

	// 任务模板参数（未来使用）
	TemplateID int                    `json:"template_id,omitempty"`
	Variables  map[string]interface{} `json:"variables,omitempty"`

	// Ansible执行选项
	AnsibleOptions *AnsibleOptions `json:"ansible_options,omitempty"`
}

// TaskHost 任务目标主机
type TaskHost struct {
	IP           string `json:"ip" binding:"required,ip"`
	Port         int    `json:"port" binding:"required,min=1,max=65535"`
	CredentialID uint   `json:"credential_id" binding:"required,min=1"` // 所有任务都需要凭证ID
}

// AnsibleOptions Ansible执行选项
type AnsibleOptions struct {
	Verbosity  int                    `json:"verbosity,omitempty"`   // 详细级别 0-4
	Check      bool                   `json:"check,omitempty"`       // 检查模式
	Diff       bool                   `json:"diff,omitempty"`        // 显示差异
	Forks      int                    `json:"forks,omitempty"`       // 并发数
	Timeout    int                    `json:"timeout,omitempty"`     // 超时秒数
	Become     bool                   `json:"become,omitempty"`      // 提权
	BecomeUser string                 `json:"become_user,omitempty"` // 提权用户
	ExtraVars  map[string]interface{} `json:"extra_vars,omitempty"`  // 额外变量
}

// Task 通用任务模型
type Task struct {
	BaseModel
	TenantID uint `gorm:"not null;index" json:"tenant_id"`

	// 任务标识
	TaskID   string `gorm:"size:36;not null;uniqueIndex" json:"task_id"`
	TaskType string `gorm:"size:50;not null;index" json:"task_type"` // ping/collect/template

	Name        string `gorm:"size:200;not null" json:"name"`
	Description string `gorm:"size:500" json:"description,omitempty"` // 任务描述
	Priority    int    `gorm:"default:5" json:"priority"`             // 1-10，数字越小优先级越高

	// 动态参数
	Params JSON `gorm:"type:jsonb" json:"params"`

	// 执行控制
	MaxRetries int `gorm:"default:0" json:"max_retries"`
	RetryCount int `gorm:"default:0" json:"retry_count"`
	Timeout    int `gorm:"default:3600" json:"timeout"` // 秒

	// 状态
	Status string `gorm:"size:20;default:'pending';index" json:"status"`
	// pending: 待执行
	// queued: 已入队
	// locked: 已锁定（被Worker获取）
	// running: 执行中
	// success: 成功
	// failed: 失败
	// timeout: 超时
	// cancelled: 已取消

	Progress int    `json:"progress"` // 进度百分比 0-100
	WorkerID string `gorm:"size:100;index" json:"worker_id"`

	// 结果
	Result JSON   `gorm:"type:jsonb" json:"result,omitempty"`
	Error  string `gorm:"type:text" json:"error,omitempty"`

	// 时间
	QueuedAt   *time.Time `json:"queued_at,omitempty"`
	LockedAt   *time.Time `json:"locked_at,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	// 审计和追踪
	CreatedBy   uint   `json:"created_by"`
	Username    string `gorm:"size:100" json:"username"`    // 发起人用户名
	TenantName  string `gorm:"size:100" json:"tenant_name"` // 租户名称
	Source      string `gorm:"size:50" json:"source"`       // 任务来源（api/ui/schedule）
	CancelledBy *uint  `json:"cancelled_by,omitempty"`

	// 关联
	Logs []TaskLog `gorm:"foreignKey:TaskID;references:TaskID" json:"logs,omitempty"`
}

// TaskLog 任务执行日志（持久化存储）
type TaskLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	TaskID    string    `gorm:"size:36;not null;index" json:"task_id"`
	Timestamp time.Time `gorm:"not null;index" json:"timestamp"`
	Level     string    `gorm:"size:20" json:"level"`  // debug/info/warning/error
	Source    string    `gorm:"size:50" json:"source"` // worker/ansible/system
	HostName  string    `gorm:"size:100;index" json:"host_name,omitempty"`
	Message   string    `gorm:"type:text" json:"message"`
	Data      JSON      `gorm:"type:jsonb" json:"data,omitempty"` // 结构化数据
}

// Worker 工作节点注册信息
type Worker struct {
	ID         uint   `gorm:"primarykey" json:"id"`
	WorkerID   string `gorm:"size:100;not null;uniqueIndex" json:"worker_id"`
	WorkerType string `gorm:"size:50" json:"worker_type"` // ansible/shell/all
	Hostname   string `gorm:"size:100" json:"hostname"`
	IPAddress  string `gorm:"size:45" json:"ip_address"`

	// 能力信息
	Concurrent int    `json:"concurrent"`                 // 并发能力
	TaskTypes  string `gorm:"size:500" json:"task_types"` // 支持的任务类型，逗号分隔
	Version    string `gorm:"size:20" json:"version"`     // Worker版本

	// 状态信息
	Status      string  `gorm:"size:20" json:"status"` // online/offline
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	TaskCount   int     `json:"task_count"` // 当前执行任务数

	// 时间信息
	RegisteredAt  time.Time `json:"registered_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`

	// 统计信息
	TotalTasks   int64 `json:"total_tasks"` // 总执行任务数
	SuccessTasks int64 `json:"success_tasks"`
	FailedTasks  int64 `json:"failed_tasks"`
}

// JSON 类型定义（用于JSONB字段）
type JSON json.RawMessage

// Scan 实现 sql.Scanner 接口
func (j *JSON) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	*j = bytes
	return nil
}

// Value 实现 driver.Valuer 接口
func (j *JSON) Value() (driver.Value, error) {
	if len(*j) == 0 {
		return nil, nil
	}
	return json.RawMessage(*j), nil
}

// MarshalJSON 实现 json.Marshaler 接口
func (j *JSON) MarshalJSON() ([]byte, error) {
	if j == nil || len(*j) == 0 {
		return []byte("null"), nil
	}
	return *j, nil
}

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (j *JSON) UnmarshalJSON(data []byte) error {
	if j == nil {
		return nil
	}
	*j = data
	return nil
}
