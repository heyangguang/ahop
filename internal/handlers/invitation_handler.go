package handlers

import (
	"ahop/internal/models"
	"ahop/internal/services"
	"ahop/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

// InvitationHandler 邀请处理器
type InvitationHandler struct {
	invitationService *services.InvitationService
}

// NewInvitationHandler 创建邀请处理器
func NewInvitationHandler() *InvitationHandler {
	return &InvitationHandler{
		invitationService: services.NewInvitationService(),
	}
}

// CreateInvitation 创建邀请
// @Summary 创建租户邀请
// @Description 租户管理员邀请用户加入租户
// @Tags 邀请管理
// @Accept json
// @Produce json
// @Param tenantId path int true "租户ID"
// @Param request body services.CreateInvitationRequest true "邀请信息"
// @Success 200 {object} response.Response{data=models.TenantInvitation}
// @Router /api/v1/tenants/{tenantId}/invitations [post]
func (h *InvitationHandler) CreateInvitation(c *gin.Context) {
	// 获取租户ID
	tenantIDStr := c.Param("id")
	tenantID, err := strconv.ParseUint(tenantIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "租户ID格式错误")
		return
	}

	// 获取当前用户ID
	inviterID, _ := c.Get("user_id")

	// 解析请求
	var req services.CreateInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 创建邀请
	invitation, err := h.invitationService.CreateInvitation(inviterID.(uint), uint(tenantID), &req)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, invitation)
}

// GetTenantInvitations 获取租户的邀请列表
// @Summary 获取租户邀请列表
// @Description 获取指定租户的所有邀请记录
// @Tags 邀请管理
// @Accept json
// @Produce json
// @Param tenantId path int true "租户ID"
// @Param status query string false "邀请状态(pending/accepted/rejected/expired)"
// @Success 200 {object} response.Response{data=[]models.TenantInvitation}
// @Router /api/v1/tenants/{tenantId}/invitations [get]
func (h *InvitationHandler) GetTenantInvitations(c *gin.Context) {
	// 获取租户ID
	tenantIDStr := c.Param("id")
	tenantID, err := strconv.ParseUint(tenantIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "租户ID格式错误")
		return
	}

	// 获取状态过滤
	status := c.Query("status")

	// 获取邀请列表
	invitations, err := h.invitationService.GetTenantInvitations(uint(tenantID), status)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	// 转换为响应格式
	var responses []*services.InvitationResponse
	for _, inv := range invitations {
		responses = append(responses, h.invitationService.ToResponse(&inv))
	}

	response.Success(c, responses)
}

// GetMyInvitations 获取我的邀请
// @Summary 获取我的邀请列表
// @Description 获取当前用户收到的所有邀请
// @Tags 邀请管理
// @Accept json
// @Produce json
// @Param status query string false "邀请状态(pending/accepted/rejected/expired)"
// @Success 200 {object} response.Response{data=[]models.TenantInvitation}
// @Router /api/v1/invitations/my [get]
func (h *InvitationHandler) GetMyInvitations(c *gin.Context) {
	// 获取当前用户信息
	userObj, _ := c.Get("user")
	user := userObj.(*models.User)

	// 获取状态过滤
	status := c.Query("status")

	// 获取邀请列表
	invitations, err := h.invitationService.GetUserInvitations(user.Email, status)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	// 转换为响应格式
	var responses []*services.InvitationResponse
	for _, inv := range invitations {
		responses = append(responses, h.invitationService.ToResponse(&inv))
	}

	response.Success(c, responses)
}

// AcceptInvitation 接受邀请
// @Summary 接受邀请
// @Description 用户接受加入租户的邀请
// @Tags 邀请管理
// @Accept json
// @Produce json
// @Param token path string true "邀请令牌"
// @Success 200 {object} response.Response
// @Router /api/v1/invitations/{token}/accept [post]
func (h *InvitationHandler) AcceptInvitation(c *gin.Context) {
	// 获取邀请令牌
	token := c.Param("token")
	
	// 获取当前用户ID
	userID, _ := c.Get("user_id")

	// 接受邀请
	if err := h.invitationService.AcceptInvitation(token, userID.(uint)); err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "邀请已接受", nil)
}

// RejectInvitation 拒绝邀请
// @Summary 拒绝邀请
// @Description 用户拒绝加入租户的邀请
// @Tags 邀请管理
// @Accept json
// @Produce json
// @Param token path string true "邀请令牌"
// @Success 200 {object} response.Response
// @Router /api/v1/invitations/{token}/reject [post]
func (h *InvitationHandler) RejectInvitation(c *gin.Context) {
	// 获取邀请令牌
	token := c.Param("token")
	
	// 获取当前用户ID
	userID, _ := c.Get("user_id")

	// 拒绝邀请
	if err := h.invitationService.RejectInvitation(token, userID.(uint)); err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "邀请已拒绝", nil)
}

// CancelInvitation 取消邀请
// @Summary 取消邀请
// @Description 邀请人或租户管理员取消邀请
// @Tags 邀请管理
// @Accept json
// @Produce json
// @Param tenantId path int true "租户ID"
// @Param invitationId path int true "邀请ID"
// @Success 200 {object} response.Response
// @Router /api/v1/tenants/{tenantId}/invitations/{invitationId}/cancel [post]
func (h *InvitationHandler) CancelInvitation(c *gin.Context) {
	// 获取租户ID
	tenantIDStr := c.Param("id")
	tenantID, err := strconv.ParseUint(tenantIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "租户ID格式错误")
		return
	}

	// 获取邀请ID
	invitationIDStr := c.Param("invitationId")
	invitationID, err := strconv.ParseUint(invitationIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "邀请ID格式错误")
		return
	}

	// 获取当前用户ID
	inviterID, _ := c.Get("user_id")

	// 取消邀请
	if err := h.invitationService.CancelInvitation(uint(invitationID), inviterID.(uint), uint(tenantID)); err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "邀请已取消", nil)
}

// GetInvitationByToken 根据令牌获取邀请详情
// @Summary 获取邀请详情
// @Description 根据令牌获取邀请的详细信息（用于邀请页面展示）
// @Tags 邀请管理
// @Accept json
// @Produce json
// @Param token path string true "邀请令牌"
// @Success 200 {object} response.Response{data=services.InvitationResponse}
// @Router /api/v1/invitations/{token} [get]
func (h *InvitationHandler) GetInvitationByToken(c *gin.Context) {
	// 获取邀请令牌
	token := c.Param("token")

	// 查询邀请
	invitation, err := h.invitationService.GetInvitationByToken(token)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	// 转换为响应格式
	resp := h.invitationService.ToResponse(invitation)
	response.Success(c, resp)
}