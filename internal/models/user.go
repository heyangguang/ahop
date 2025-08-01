package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	BaseModel
	Username        string     `json:"username" gorm:"unique;not null;size:50;index"`
	Email           string     `json:"email" gorm:"unique;not null;size:100;index"`
	PasswordHash    string     `json:"-" gorm:"not null;size:255"`
	Name            string     `json:"name" gorm:"not null;size:100"`
	Phone           *string    `json:"phone" gorm:"size:20"`
	Avatar          *string    `json:"avatar" gorm:"size:255"`
	Status          string     `json:"status" gorm:"default:'active';size:20"`
	IsPlatformAdmin bool       `json:"is_platform_admin" gorm:"default:false"`
	LastLoginAt     *time.Time `json:"last_login_at"`

	// 多对多关联
	Tenants []Tenant `gorm:"many2many:user_tenants;" json:"tenants,omitempty"`
	Roles   []Role   `gorm:"many2many:user_roles;" json:"roles,omitempty"`
}

type UserRole struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	RoleID    uint      `gorm:"not null;index" json:"role_id"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy uint      `json:"created_by"` // 谁分配的角色
}

// TableName 表名
func (u *User) TableName() string {
	return "users"
}

// 用户状态常量
const (
	UserStatusActive   = "active"
	UserStatusInactive = "inactive"
	UserStatusLocked   = "locked"
)

// SetPassword 设置密码 - 数据操作方法
func (u *User) SetPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hashedPassword)
	return nil
}

// CheckPassword 验证密码 - 数据操作方法
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// GetUserTenants 获取用户的所有租户信息
func (u *User) GetUserTenants(db *gorm.DB) ([]UserTenant, error) {
	var userTenants []UserTenant
	err := db.Where("user_id = ?", u.ID).
		Preload("Tenant").
		Preload("Role").
		Find(&userTenants).Error
	return userTenants, err
}

// GetTenantRole 获取用户在指定租户的角色
func (u *User) GetTenantRole(db *gorm.DB, tenantID uint) (*UserTenant, error) {
	var userTenant UserTenant
	err := db.Where("user_id = ? AND tenant_id = ?", u.ID, tenantID).
		Preload("Role").
		First(&userTenant).Error
	if err != nil {
		return nil, err
	}
	return &userTenant, nil
}

// IsTenantMember 检查用户是否是指定租户的成员
func (u *User) IsTenantMember(db *gorm.DB, tenantID uint) bool {
	var count int64
	db.Model(&UserTenant{}).
		Where("user_id = ? AND tenant_id = ?", u.ID, tenantID).
		Count(&count)
	return count > 0
}

// IsAdminOfTenant 检查用户是否是指定租户的管理员
func (u *User) IsAdminOfTenant(db *gorm.DB, tenantID uint) bool {
	// 平台管理员对所有租户都有管理权限
	if u.IsPlatformAdmin {
		return true
	}
	
	var userTenant UserTenant
	err := db.Where("user_id = ? AND tenant_id = ? AND is_tenant_admin = ?", u.ID, tenantID, true).
		First(&userTenant).Error
	return err == nil
}
