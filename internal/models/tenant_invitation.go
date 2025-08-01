package models

import (
	"time"
)

// TenantInvitation 租户邀请
type TenantInvitation struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	TenantID     uint      `gorm:"not null;index" json:"tenant_id"`
	InviterID    uint      `gorm:"not null" json:"inviter_id"`                          // 邀请人
	InviteeEmail string    `gorm:"size:200;not null;index" json:"invitee_email"`        // 被邀请人邮箱
	InviteeID    *uint     `json:"invitee_id"`                                           // 被邀请人ID（如果已注册）
	RoleID       *uint     `json:"role_id"`                                              // 分配的角色
	IsTenantAdmin bool     `gorm:"default:false" json:"is_tenant_admin"`                // 是否邀请为管理员
	Status       string    `gorm:"size:20;not null;default:'pending'" json:"status"`    // pending/accepted/rejected/expired
	Token        string    `gorm:"size:100;uniqueIndex" json:"token"`                   // 邀请令牌
	Message      string    `gorm:"size:500" json:"message,omitempty"`                   // 邀请留言
	ExpiredAt    time.Time `gorm:"not null" json:"expired_at"`                          // 过期时间
	AcceptedAt   *time.Time `json:"accepted_at,omitempty"`                               // 接受时间
	RejectedAt   *time.Time `json:"rejected_at,omitempty"`                               // 拒绝时间
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// 关联
	Tenant   Tenant `gorm:"foreignKey:TenantID;constraint:OnDelete:CASCADE" json:"tenant,omitempty"`
	Inviter  User   `gorm:"foreignKey:InviterID" json:"inviter,omitempty"`
	Invitee  *User  `gorm:"foreignKey:InviteeID" json:"invitee,omitempty"`
	Role     *Role  `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

// TableName 指定表名
func (TenantInvitation) TableName() string {
	return "tenant_invitations"
}

// 邀请状态常量
const (
	InvitationStatusPending  = "pending"
	InvitationStatusAccepted = "accepted"
	InvitationStatusRejected = "rejected"
	InvitationStatusExpired  = "expired"
)

// IsValid 检查邀请是否有效
func (ti *TenantInvitation) IsValid() bool {
	return ti.Status == InvitationStatusPending && time.Now().Before(ti.ExpiredAt)
}

// Accept 接受邀请
func (ti *TenantInvitation) Accept() {
	now := time.Now()
	ti.Status = InvitationStatusAccepted
	ti.AcceptedAt = &now
}

// Reject 拒绝邀请
func (ti *TenantInvitation) Reject() {
	now := time.Now()
	ti.Status = InvitationStatusRejected
	ti.RejectedAt = &now
}

// MarkExpired 标记为过期
func (ti *TenantInvitation) MarkExpired() {
	ti.Status = InvitationStatusExpired
}