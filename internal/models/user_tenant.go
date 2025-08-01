package models

import (
	"time"
	"gorm.io/gorm"
)

// UserTenant 用户-租户关联表
type UserTenant struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	UserID        uint      `gorm:"not null;uniqueIndex:idx_user_tenant" json:"user_id"`
	TenantID      uint      `gorm:"not null;uniqueIndex:idx_user_tenant" json:"tenant_id"`
	RoleID        *uint     `gorm:"index" json:"role_id"`                     // 在该租户的角色
	IsTenantAdmin bool      `gorm:"default:false" json:"is_tenant_admin"`     // 是否为该租户管理员
	JoinedAt      time.Time `gorm:"not null" json:"joined_at"`                // 加入时间
	InvitedBy     *uint     `json:"invited_by"`                                // 邀请人ID
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// 关联
	User     User   `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Tenant   Tenant `gorm:"foreignKey:TenantID;constraint:OnDelete:CASCADE" json:"tenant,omitempty"`
	Role     *Role  `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	Inviter  *User  `gorm:"foreignKey:InvitedBy" json:"inviter,omitempty"`
}

// TableName 指定表名
func (UserTenant) TableName() string {
	return "user_tenants"
}

// GetPermissions 获取用户在该租户的权限
func (ut *UserTenant) GetPermissions(db *gorm.DB) ([]Permission, error) {
	var permissions []Permission
	
	// 如果是租户管理员，获取所有权限
	if ut.IsTenantAdmin {
		err := db.Find(&permissions).Error
		return permissions, err
	}
	
	// 否则根据角色获取权限
	if ut.RoleID != nil {
		err := db.
			Joins("JOIN role_permissions ON permissions.id = role_permissions.permission_id").
			Where("role_permissions.role_id = ?", *ut.RoleID).
			Find(&permissions).Error
		return permissions, err
	}
	
	return permissions, nil
}

// UserTenantStats 用户租户统计
type UserTenantStats struct {
	TotalTenants  int64 `json:"total_tenants"`
	AdminTenants  int64 `json:"admin_tenants"`  // 作为管理员的租户数
	ActiveTenants int64 `json:"active_tenants"` // 活跃的租户数
}