package models

import "time"

// Role 角色模型
type Role struct {
	BaseModel
	TenantID    uint   `gorm:"not null;index" json:"tenant_id"`        // 所属租户（0表示系统级角色）
	Code        string `gorm:"size:100;not null" json:"code"`          // 角色代码，如 "tenant_admin"
	Name        string `gorm:"size:100;not null" json:"name"`          // 角色名称，如 "租户管理员"
	Description string `gorm:"size:255" json:"description"`            // 角色描述
	IsSystem    bool   `gorm:"default:false" json:"is_system"`         // 是否系统角色（不可删除）
	Status      string `gorm:"size:20;default:'active'" json:"status"` // 状态：active, inactive

	// 关联关系
	Tenant      *Tenant      `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
}

// 角色状态常量
const (
	RoleStatusActive   = "active"
	RoleStatusInactive = "inactive"
)

// 系统预定义角色常量
const (
	RolePlatformAdmin = "platform_admin" // 平台超级管理员
	RoleTenantAdmin   = "tenant_admin"   // 租户管理员
	RoleTenantUser    = "tenant_user"    // 租户普通用户
)

// RolePermission 角色权限关联表
type RolePermission struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	RoleID       uint      `gorm:"not null;index" json:"role_id"`
	PermissionID uint      `gorm:"not null;index" json:"permission_id"`
	CreatedAt    time.Time `json:"created_at"`
}
