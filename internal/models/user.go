package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User 用户模型
type User struct {
	BaseModel
	TenantID        uint       `json:"tenant_id" gorm:"not null;index"`
	Username        string     `json:"username" gorm:"unique;not null;size:50;index"`
	Email           string     `json:"email" gorm:"unique;not null;size:100;index"`
	PasswordHash    string     `json:"-" gorm:"not null;size:255"`
	Name            string     `json:"name" gorm:"not null;size:100"`
	Phone           *string    `json:"phone" gorm:"size:20"`
	Avatar          *string    `json:"avatar" gorm:"size:255"`
	Status          string     `json:"status" gorm:"default:'active';size:20"`
	IsPlatformAdmin bool       `json:"is_platform_admin" gorm:"default:false"`
	IsTenantAdmin   bool       `json:"is_tenant_admin" gorm:"default:false"`
	LastLoginAt     *time.Time `json:"last_login_at"`

	// 关联关系
	Tenant *Tenant `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`

	// 新增：角色关联
	Roles []Role `gorm:"many2many:user_roles;" json:"roles,omitempty"`
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
