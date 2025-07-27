package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

// TicketHandler 工单处理器
type TicketHandler struct {
	ticketService *services.TicketService
}

// NewTicketHandler 创建工单处理器
func NewTicketHandler(ticketService *services.TicketService) *TicketHandler {
	return &TicketHandler{
		ticketService: ticketService,
	}
}

// GetByID 获取工单详情
func (h *TicketHandler) GetByID(c *gin.Context) {
	// 获取工单ID
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的工单ID")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 获取工单
	ticket, err := h.ticketService.GetTicket(claims.CurrentTenantID, uint(ticketID))
	if err != nil {
		if err.Error() == "工单不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, "获取工单失败")
		return
	}

	response.Success(c, ticket)
}

// List 获取工单列表
func (h *TicketHandler) List(c *gin.Context) {
	// 解析过滤条件
	var filter services.TicketFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 解析分页参数
	params := pagination.ParsePageParams(c)

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 获取工单列表
	tickets, total, err := h.ticketService.ListTickets(
		claims.CurrentTenantID,
		filter,
		params.GetOffset(),
		params.GetLimit(),
	)
	if err != nil {
		response.ServerError(c, "获取工单列表失败")
		return
	}

	// 构建分页信息
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, tickets, pageInfo)
}

// GetStats 获取工单统计
func (h *TicketHandler) GetStats(c *gin.Context) {
	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 获取统计信息
	stats, err := h.ticketService.GetTicketStats(claims.CurrentTenantID)
	if err != nil {
		response.ServerError(c, "获取统计信息失败")
		return
	}

	response.Success(c, stats)
}

// AddComment 添加工单评论
func (h *TicketHandler) AddComment(c *gin.Context) {
	// 获取工单ID
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的工单ID")
		return
	}

	// 解析请求体
	var req struct {
		Comment string `json:"comment" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 更新评论
	if err := h.ticketService.UpdateTicketComment(claims.CurrentTenantID, uint(ticketID), req.Comment); err != nil {
		if err.Error() == "工单不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "评论添加成功", nil)
}