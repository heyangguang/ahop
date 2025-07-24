package models

import (
	"time"
	"gorm.io/datatypes"
)

// HostGroup 主机组模型
type HostGroup struct {
	ID          uint             `gorm:"primarykey" json:"id"`
	TenantID    uint             `gorm:"not null;index:idx_tenant_parent;index:idx_tenant_path" json:"tenant_id"`
	ParentID    *uint            `gorm:"index:idx_tenant_parent" json:"parent_id"`
	Name        string           `gorm:"size:100;not null" json:"name"`
	Code        string           `gorm:"size:50;not null;index:idx_tenant_code" json:"code"`
	Path        string           `gorm:"size:500;index:idx_tenant_path" json:"path"`
	Level       int              `gorm:"default:1" json:"level"`
	Type        string           `gorm:"size:50;default:'custom'" json:"type"`
	Description string           `gorm:"type:text" json:"description"`
	Status      string           `gorm:"size:20;default:'active'" json:"status"`
	IsLeaf      bool             `gorm:"default:true" json:"is_leaf"`
	HostCount   int              `gorm:"default:0" json:"host_count"`
	ChildCount  int              `gorm:"default:0" json:"child_count"`
	Metadata    datatypes.JSON   `gorm:"type:json" json:"metadata"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	
	// 关联
	Parent      *HostGroup       `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Children    []HostGroup      `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Hosts       []Host           `gorm:"foreignKey:HostGroupID" json:"hosts,omitempty"`
	Tenant      *Tenant          `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

// TableName 指定表名
func (HostGroup) TableName() string {
	return "host_groups"
}

// HostGroupType 主机组类型
const (
	HostGroupTypeEnvironment = "environment" // 环境分组
	HostGroupTypeBusiness    = "business"    // 业务分组
	HostGroupTypeRegion      = "region"      // 地域分组
	HostGroupTypeCustom      = "custom"      // 自定义分组
)

// HostGroupStatus 主机组状态
const (
	HostGroupStatusActive   = "active"   // 激活
	HostGroupStatusInactive = "inactive" // 未激活
)

// HostGroupTreeNode 主机组树节点
type HostGroupTreeNode struct {
	*HostGroup
	Children []*HostGroupTreeNode `json:"children,omitempty"`
}

// CreateHostGroupRequest 创建主机组请求
type CreateHostGroupRequest struct {
	ParentID    *uint          `json:"parent_id"`
	Name        string         `json:"name" binding:"required,min=1,max=100"`
	Code        string         `json:"code" binding:"required,min=1,max=50"`
	Type        string         `json:"type" binding:"omitempty,oneof=environment business region custom"`
	Description string         `json:"description" binding:"max=500"`
	Status      string         `json:"status" binding:"omitempty,oneof=active inactive"`
	Metadata    datatypes.JSON `json:"metadata"`
}

// UpdateHostGroupRequest 更新主机组请求
type UpdateHostGroupRequest struct {
	Name        string         `json:"name" binding:"omitempty,min=1,max=100"`
	Description string         `json:"description" binding:"max=500"`
	Status      string         `json:"status" binding:"omitempty,oneof=active inactive"`
	Type        string         `json:"type" binding:"omitempty,oneof=environment business region custom"`
	Metadata    datatypes.JSON `json:"metadata"`
}

// MoveHostGroupRequest 移动主机组请求
type MoveHostGroupRequest struct {
	NewParentID *uint `json:"new_parent_id"`
}

// AssignHostsRequest 分配主机请求
type AssignHostsRequest struct {
	HostIDs []uint `json:"host_ids" binding:"required,min=1"`
}

// HostGroupListResponse 主机组列表响应
type HostGroupListResponse struct {
	ID          uint   `json:"id"`
	ParentID    *uint  `json:"parent_id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Path        string `json:"path"`
	Level       int    `json:"level"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	IsLeaf      bool   `json:"is_leaf"`
	HostCount   int    `json:"host_count"`
	ChildCount  int    `json:"child_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// IsActive 是否激活
func (g *HostGroup) IsActive() bool {
	return g.Status == HostGroupStatusActive
}

// CanContainHosts 是否可以包含主机
func (g *HostGroup) CanContainHosts() bool {
	return g.IsLeaf
}

// CanContainGroups 是否可以包含子组
func (g *HostGroup) CanContainGroups() bool {
	return !g.IsLeaf || g.HostCount == 0
}

// GetFullPath 获取完整路径
func (g *HostGroup) GetFullPath() string {
	if g.Path == "" {
		return "/" + g.Code
	}
	return g.Path
}