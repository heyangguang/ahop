package models

import (
	"time"
)

// Host 主机模型
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

	// 主机组关联
	HostGroupID *uint `gorm:"index" json:"host_group_id"`
	
	// 关联
	Credential   Credential        `gorm:"foreignKey:CredentialID" json:"credential,omitempty"`
	Tags         []Tag             `gorm:"many2many:host_tags;" json:"tags,omitempty"`
	Disks        []HostDisk        `gorm:"foreignKey:HostID" json:"disks,omitempty"`
	NetworkCards []HostNetworkCard `gorm:"foreignKey:HostID" json:"network_cards,omitempty"`
	HostGroup    *HostGroup        `gorm:"foreignKey:HostGroupID" json:"host_group,omitempty"`
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

