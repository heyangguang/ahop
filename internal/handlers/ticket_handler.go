package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"fmt"
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


// TestWriteback 测试工单回写功能
func (h *TicketHandler) TestWriteback(c *gin.Context) {
	// 获取工单ID
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的工单ID")
		return
	}

	// 解析请求体
	var req struct {
		Status       string                 `json:"status"`
		Comment      string                 `json:"comment"`
		CustomFields map[string]interface{} `json:"custom_fields"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 先验证工单是否存在且属于当前租户
	ticket, err := h.ticketService.GetTicket(claims.CurrentTenantID, uint(ticketID))
	if err != nil {
		if err.Error() == "工单不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, "获取工单失败")
		return
	}

	// 构建更新数据
	updates := make(map[string]interface{})
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Comment != "" {
		updates["comment"] = req.Comment
	}
	if len(req.CustomFields) > 0 {
		updates["custom_fields"] = req.CustomFields
	}

	// 调用回写服务
	if err := h.ticketService.UpdateExternalTicket(uint(ticketID), updates); err != nil {
		response.ServerError(c, fmt.Sprintf("工单回写失败: %v", err))
		return
	}

	response.SuccessWithMessage(c, "工单回写成功", gin.H{
		"ticket_id": ticket.ID,
		"external_id": ticket.ExternalID,
		"plugin_name": ticket.Plugin.Name,
		"updates": updates,
	})
}