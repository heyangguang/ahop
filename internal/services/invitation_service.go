package services

import (
	"ahop/internal/database"
	"ahop/internal/models"
	"ahop/pkg/logger"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// InvitationService 邀请服务
type InvitationService struct {
	db         *gorm.DB
	log        *logrus.Logger
	userService *UserService
}

// NewInvitationService 创建邀请服务
func NewInvitationService() *InvitationService {
	return &InvitationService{
		db:         database.GetDB(),
		log:        logger.GetLogger(),
		userService: NewUserService(),
	}
}

// CreateInvitation 创建邀请
func (s *InvitationService) CreateInvitation(inviterID, tenantID uint, req *CreateInvitationRequest) (*models.TenantInvitation, error) {
	// 验证邀请人权限
	inviter, err := s.userService.GetByID(inviterID)
	if err != nil {
		return nil, fmt.Errorf("邀请人不存在")
	}

	// 检查邀请人是否是该租户的管理员
	if !inviter.IsAdminOfTenant(s.db, tenantID) {
		return nil, fmt.Errorf("只有租户管理员才能邀请用户")
	}

	// 检查是否已有待处理的邀请
	var existingInvitation models.TenantInvitation
	err = s.db.Where("tenant_id = ? AND invitee_email = ? AND status = ?", 
		tenantID, req.Email, models.InvitationStatusPending).First(&existingInvitation).Error
	if err == nil {
		return nil, fmt.Errorf("该邮箱已有待处理的邀请")
	}

	// 检查用户是否已经是该租户的成员
	var existingUser models.User
	err = s.db.Where("email = ?", req.Email).First(&existingUser).Error
	if err == nil {
		// 用户已存在，检查是否已是租户成员
		if existingUser.IsTenantMember(s.db, tenantID) {
			return nil, fmt.Errorf("该用户已是租户成员")
		}
	}

	// 生成邀请令牌
	token, err := s.generateInvitationToken()
	if err != nil {
		return nil, err
	}

	// 创建邀请
	invitation := &models.TenantInvitation{
		TenantID:      tenantID,
		InviterID:     inviterID,
		InviteeEmail:  req.Email,
		RoleID:        req.RoleID,
		IsTenantAdmin: req.IsTenantAdmin,
		Status:        models.InvitationStatusPending,
		Token:         token,
		Message:       req.Message,
		ExpiredAt:     time.Now().Add(7 * 24 * time.Hour), // 7天有效期
	}

	// 如果用户已存在，记录用户ID
	if err == nil {
		invitation.InviteeID = &existingUser.ID
	}

	if err := s.db.Create(invitation).Error; err != nil {
		s.log.Errorf("创建邀请失败: %v", err)
		return nil, fmt.Errorf("创建邀请失败")
	}

	// TODO: 发送邮件通知

	return invitation, nil
}

// AcceptInvitation 接受邀请
func (s *InvitationService) AcceptInvitation(token string, userID uint) error {
	// 查找邀请
	var invitation models.TenantInvitation
	err := s.db.Where("token = ?", token).Preload("Tenant").First(&invitation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("邀请不存在")
		}
		return fmt.Errorf("查询邀请失败")
	}

	// 检查邀请状态
	if !invitation.IsValid() {
		return fmt.Errorf("邀请已失效")
	}

	// 检查用户邮箱是否匹配
	user, err := s.userService.GetByID(userID)
	if err != nil {
		return fmt.Errorf("用户不存在")
	}

	if user.Email != invitation.InviteeEmail {
		return fmt.Errorf("邀请邮箱不匹配")
	}

	// 开始事务
	tx := s.db.Begin()

	// 更新邀请状态
	invitation.Accept()
	invitation.InviteeID = &userID
	if err := tx.Save(&invitation).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("更新邀请状态失败")
	}

	// 创建用户-租户关联
	userTenant := &models.UserTenant{
		UserID:        userID,
		TenantID:      invitation.TenantID,
		RoleID:        invitation.RoleID,
		IsTenantAdmin: invitation.IsTenantAdmin,
		JoinedAt:      time.Now(),
		InvitedBy:     &invitation.InviterID,
	}

	if err := tx.Create(userTenant).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("创建租户关联失败")
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("提交事务失败")
	}

	s.log.WithFields(logrus.Fields{
		"user_id":   userID,
		"tenant_id": invitation.TenantID,
		"inviter_id": invitation.InviterID,
	}).Info("用户接受邀请加入租户")

	return nil
}

// RejectInvitation 拒绝邀请
func (s *InvitationService) RejectInvitation(token string, userID uint) error {
	// 查找邀请
	var invitation models.TenantInvitation
	err := s.db.Where("token = ?", token).First(&invitation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("邀请不存在")
		}
		return fmt.Errorf("查询邀请失败")
	}

	// 检查邀请状态
	if invitation.Status != models.InvitationStatusPending {
		return fmt.Errorf("邀请已处理")
	}

	// 检查用户邮箱是否匹配
	user, err := s.userService.GetByID(userID)
	if err != nil {
		return fmt.Errorf("用户不存在")
	}

	if user.Email != invitation.InviteeEmail {
		return fmt.Errorf("邀请邮箱不匹配")
	}

	// 更新邀请状态
	invitation.Reject()
	invitation.InviteeID = &userID
	if err := s.db.Save(&invitation).Error; err != nil {
		return fmt.Errorf("更新邀请状态失败")
	}

	return nil
}

// GetUserInvitations 获取用户的邀请列表
func (s *InvitationService) GetUserInvitations(email string, status string) ([]models.TenantInvitation, error) {
	query := s.db.Where("invitee_email = ?", email).
		Preload("Tenant").
		Preload("Inviter").
		Preload("Role")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var invitations []models.TenantInvitation
	if err := query.Order("created_at DESC").Find(&invitations).Error; err != nil {
		return nil, fmt.Errorf("查询邀请列表失败")
	}

	return invitations, nil
}

// GetTenantInvitations 获取租户的邀请列表
func (s *InvitationService) GetTenantInvitations(tenantID uint, status string) ([]models.TenantInvitation, error) {
	query := s.db.Where("tenant_id = ?", tenantID).
		Preload("Inviter").
		Preload("Invitee").
		Preload("Role")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var invitations []models.TenantInvitation
	if err := query.Order("created_at DESC").Find(&invitations).Error; err != nil {
		return nil, fmt.Errorf("查询邀请列表失败")
	}

	return invitations, nil
}

// CancelInvitation 取消邀请（邀请人操作）
func (s *InvitationService) CancelInvitation(invitationID, inviterID, tenantID uint) error {
	var invitation models.TenantInvitation
	err := s.db.Where("id = ? AND tenant_id = ?", invitationID, tenantID).First(&invitation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("邀请不存在")
		}
		return fmt.Errorf("查询邀请失败")
	}

	// 检查权限：只有邀请人或租户管理员可以取消
	inviter, err := s.userService.GetByID(inviterID)
	if err != nil {
		return fmt.Errorf("用户不存在")
	}

	if invitation.InviterID != inviterID && !inviter.IsAdminOfTenant(s.db, tenantID) {
		return fmt.Errorf("无权取消该邀请")
	}

	// 检查状态
	if invitation.Status != models.InvitationStatusPending {
		return fmt.Errorf("只能取消待处理的邀请")
	}

	// 标记为过期
	invitation.MarkExpired()
	if err := s.db.Save(&invitation).Error; err != nil {
		return fmt.Errorf("取消邀请失败")
	}

	return nil
}

// CleanupExpiredInvitations 清理过期的邀请
func (s *InvitationService) CleanupExpiredInvitations() error {
	result := s.db.Model(&models.TenantInvitation{}).
		Where("status = ? AND expired_at < ?", models.InvitationStatusPending, time.Now()).
		Update("status", models.InvitationStatusExpired)
	
	if result.Error != nil {
		return result.Error
	}

	s.log.Infof("清理过期邀请 %d 条", result.RowsAffected)
	return nil
}

// generateInvitationToken 生成邀请令牌
func (s *InvitationService) generateInvitationToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateInvitationRequest 创建邀请请求
type CreateInvitationRequest struct {
	Email         string `json:"email" binding:"required,email"`
	RoleID        *uint  `json:"role_id"`
	IsTenantAdmin bool   `json:"is_tenant_admin"`
	Message       string `json:"message"`
}

// InvitationResponse 邀请响应
type InvitationResponse struct {
	ID            uint      `json:"id"`
	TenantName    string    `json:"tenant_name"`
	InviterName   string    `json:"inviter_name"`
	InviteeEmail  string    `json:"invitee_email"`
	RoleName      string    `json:"role_name,omitempty"`
	IsTenantAdmin bool      `json:"is_tenant_admin"`
	Status        string    `json:"status"`
	Message       string    `json:"message,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiredAt     time.Time `json:"expired_at"`
}

// GetInvitationByToken 根据令牌获取邀请详情
func (s *InvitationService) GetInvitationByToken(token string) (*models.TenantInvitation, error) {
	var invitation models.TenantInvitation
	err := s.db.Where("token = ?", token).
		Preload("Tenant").
		Preload("Inviter").
		Preload("Invitee").
		Preload("Role").
		First(&invitation).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("邀请不存在")
		}
		return nil, fmt.Errorf("查询邀请失败")
	}

	return &invitation, nil
}

// ToResponse 转换为响应格式
func (s *InvitationService) ToResponse(invitation *models.TenantInvitation) *InvitationResponse {
	resp := &InvitationResponse{
		ID:            invitation.ID,
		InviteeEmail:  invitation.InviteeEmail,
		IsTenantAdmin: invitation.IsTenantAdmin,
		Status:        invitation.Status,
		Message:       invitation.Message,
		CreatedAt:     invitation.CreatedAt,
		ExpiredAt:     invitation.ExpiredAt,
	}

	if invitation.Tenant.ID != 0 {
		resp.TenantName = invitation.Tenant.Name
	}

	if invitation.Inviter.ID != 0 {
		resp.InviterName = invitation.Inviter.Name
	}

	if invitation.Role != nil {
		resp.RoleName = invitation.Role.Name
	}

	return resp
}