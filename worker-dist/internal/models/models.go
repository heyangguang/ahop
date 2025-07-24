package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// BaseModel 基础模型
type BaseModel struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Task 任务模型（只读）
type Task struct {
	BaseModel
	TenantID uint `gorm:"not null;index" json:"tenant_id"`

	// 任务标识
	TaskID   string `gorm:"size:36;not null;uniqueIndex" json:"task_id"`
	TaskType string `gorm:"size:50;not null;index" json:"task_type"`

	Name        string `gorm:"size:200;not null" json:"name"`
	Description string `gorm:"size:500" json:"description,omitempty"` // 任务描述
	Priority    int    `gorm:"default:5" json:"priority"`

	// 动态参数
	Params JSON `gorm:"type:jsonb" json:"params"`

	// 执行控制
	MaxRetries int `gorm:"default:0" json:"max_retries"`
	RetryCount int `gorm:"default:0" json:"retry_count"`
	Timeout    int `gorm:"default:3600" json:"timeout"`

	// 状态
	Status   string `gorm:"size:20;default:'pending';index" json:"status"`
	Progress int    `json:"progress"`
	WorkerID string `gorm:"size:100;index" json:"worker_id"`

	// 结果
	Result JSON   `gorm:"type:jsonb" json:"result,omitempty"`
	Error  string `gorm:"type:text" json:"error,omitempty"`

	// 时间
	QueuedAt   *time.Time `json:"queued_at,omitempty"`
	LockedAt   *time.Time `json:"locked_at,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	// 审计
	CreatedBy   uint  `json:"created_by"`
	CancelledBy *uint `json:"cancelled_by,omitempty"`
}

// TaskLog 任务日志
type TaskLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	TaskID    string    `gorm:"size:36;not null;index" json:"task_id"`
	Timestamp time.Time `gorm:"not null;index" json:"timestamp"`
	Level     string    `gorm:"size:20" json:"level"`
	Source    string    `gorm:"size:50" json:"source"`
	HostName  string    `gorm:"size:100;index" json:"host_name,omitempty"`
	Message   string    `gorm:"type:text" json:"message"`
	Data      JSON      `gorm:"type:jsonb" json:"data,omitempty"`
}

// Host 主机模型（只读）
type Host struct {
	BaseModel
	TenantID uint `gorm:"not null;index" json:"tenant_id"`

	// 核心字段
	Name         string `gorm:"size:100;not null;uniqueIndex:idx_tenant_host" json:"name"`
	IPAddress    string `gorm:"size:45;not null" json:"ip_address"`
	Port         int    `gorm:"default:22" json:"port"`
	CredentialID uint   `gorm:"not null" json:"credential_id"`

	// 系统信息（自动采集）
	Hostname     string `gorm:"size:100" json:"hostname"`
	OSType       string `gorm:"size:50" json:"os_type"` // centos/ubuntu/windows
	OSVersion    string `gorm:"size:50" json:"os_version"`
	Kernel       string `gorm:"size:100" json:"kernel"`
	Architecture string `gorm:"size:20" json:"architecture"` // x86_64/aarch64

	// 硬件概览（自动采集）
	CPUModel      string `gorm:"size:200" json:"cpu_model"`
	CPUCores      int    `json:"cpu_cores"`
	MemoryTotalMB int64  `json:"memory_total_mb"`

	// 状态
	Status      string     `gorm:"size:20;default:'pending'" json:"status"` // pending/online/offline/unreachable
	LastCheckAt *time.Time `json:"last_check_at"`

	// 元数据
	Description string `gorm:"size:500" json:"description"`
	IsActive    bool   `gorm:"default:true" json:"is_active"`
	
	// 审计字段
	CreatedBy uint `json:"created_by"`
	UpdatedBy uint `json:"updated_by"`

	// 关联
	Credential   Credential        `gorm:"foreignKey:CredentialID" json:"credential,omitempty"`
	Tags         []Tag             `gorm:"many2many:host_tags;" json:"tags,omitempty"`
	Disks        []HostDisk        `gorm:"foreignKey:HostID" json:"disks,omitempty"`
	NetworkCards []HostNetworkCard `gorm:"foreignKey:HostID" json:"network_cards,omitempty"`
}

// Credential 凭证模型（只读）
type Credential struct {
	BaseModel
	TenantID uint `gorm:"not null;index" json:"tenant_id"`

	Name        string `gorm:"size:100;not null" json:"name"`
	Type        string `gorm:"size:20;not null" json:"type"`
	Description string `gorm:"type:text" json:"description"`

	Username   string `gorm:"size:100" json:"username,omitempty"`
	Password   []byte `gorm:"type:bytea" json:"-"`
	PrivateKey []byte `gorm:"type:bytea" json:"-"`
	PublicKey  []byte `gorm:"type:bytea" json:"-"`
	Passphrase []byte `gorm:"type:bytea" json:"-"`
	APIKey     []byte `gorm:"type:bytea" json:"-"`
	Token      []byte `gorm:"type:bytea" json:"-"`
	SecretKey  []byte `gorm:"type:bytea" json:"-"`

	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	
	CreatedBy uint `json:"created_by"`
	UpdatedBy uint `json:"updated_by"`
}

// Tag 标签模型
type Tag struct {
	BaseModel
	TenantID uint `gorm:"not null;index" json:"tenant_id"`

	Key   string `gorm:"size:50;not null" json:"key"`
	Value string `gorm:"size:100;not null" json:"value"`
	Color string `gorm:"size:7" json:"color"`

	Description string `gorm:"type:text" json:"description"`
	CreatedBy   uint   `json:"created_by"`
	UpdatedBy   uint   `json:"updated_by"`
}

// Worker Worker注册信息
type Worker struct {
	ID         uint   `gorm:"primarykey" json:"id"`
	WorkerID   string `gorm:"size:100;not null;uniqueIndex" json:"worker_id"`
	WorkerType string `gorm:"size:50" json:"worker_type"`
	Hostname   string `gorm:"size:100" json:"hostname"`
	IPAddress  string `gorm:"size:45" json:"ip_address"`

	// 能力信息
	Concurrent int    `json:"concurrent"`
	TaskTypes  string `gorm:"size:500" json:"task_types"`
	Version    string `gorm:"size:20" json:"version"`

	// 状态信息
	Status        string  `gorm:"size:20" json:"status"`
	CPUUsage      float64 `json:"cpu_usage"`
	MemoryUsage   float64 `json:"memory_usage"`
	TaskCount     int     `json:"task_count"`

	// 时间信息
	RegisteredAt  time.Time `json:"registered_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`

	// 统计信息
	TotalTasks   int64 `json:"total_tasks"`
	SuccessTasks int64 `json:"success_tasks"`
	FailedTasks  int64 `json:"failed_tasks"`
}

// HostDisk 主机磁盘信息
type HostDisk struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	HostID       uint      `gorm:"not null;index" json:"host_id"`
	Device       string    `gorm:"size:50;not null" json:"device"` // /dev/sda1
	MountPoint   string    `gorm:"size:200" json:"mount_point"`    // /
	FileSystem   string    `gorm:"size:50" json:"file_system"`     // ext4/xfs
	TotalMB      int64     `json:"total_mb"`
	UsedMB       int64     `json:"used_mb"`
	FreeMB       int64     `json:"free_mb"`
	UsagePercent float64   `json:"usage_percent"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// HostNetworkCard 主机网卡信息
type HostNetworkCard struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	HostID      uint      `gorm:"not null;index" json:"host_id"`
	Name        string    `gorm:"size:50;not null" json:"name"` // eth0/ens33
	MACAddress  string    `gorm:"size:20" json:"mac_address"`
	IPAddress   string    `gorm:"size:45" json:"ip_address"`    // 主IP
	IPAddresses string    `gorm:"size:500" json:"ip_addresses"` // 所有IP，逗号分隔
	MTU         int       `json:"mtu"`
	Speed       int       `json:"speed"`                // Mbps
	State       string    `gorm:"size:20" json:"state"` // up/down
	UpdatedAt   time.Time `json:"updated_at"`
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
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return json.RawMessage(j), nil
}

// MarshalJSON 实现 json.Marshaler 接口
func (j JSON) MarshalJSON() ([]byte, error) {
	if j == nil || len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (j *JSON) UnmarshalJSON(data []byte) error {
	if j == nil {
		return nil
	}
	*j = data
	return nil
}