package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

// TicketPluginHandler 工单插件处理器
type TicketPluginHandler struct {
	ticketPluginService *services.TicketPluginService
}

// NewTicketPluginHandler 创建工单插件处理器
func NewTicketPluginHandler(ticketPluginService *services.TicketPluginService) *TicketPluginHandler {
	return &TicketPluginHandler{
		ticketPluginService: ticketPluginService,
	}
}

// Create 创建工单插件
func (h *TicketPluginHandler) Create(c *gin.Context) {
	var req services.CreateTicketPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 创建插件
	plugin, err := h.ticketPluginService.CreateTicketPlugin(claims.CurrentTenantID, req)
	if err != nil {
		if err.Error() == "插件编码已存在" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "创建插件失败")
		return
	}

	response.Success(c, plugin)
}

// Update 更新工单插件
func (h *TicketPluginHandler) Update(c *gin.Context) {
	// 获取插件ID
	pluginID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	var req services.UpdateTicketPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 更新插件
	plugin, err := h.ticketPluginService.UpdateTicketPlugin(claims.CurrentTenantID, uint(pluginID), req)
	if err != nil {
		if err.Error() == "插件不存在" {
			response.NotFound(c, err.Error())
			return
		}
		if err.Error() == "插件编码已存在" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "更新插件失败")
		return
	}

	response.Success(c, plugin)
}

// Delete 删除工单插件
func (h *TicketPluginHandler) Delete(c *gin.Context) {
	// 获取插件ID
	pluginID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 删除插件
	if err := h.ticketPluginService.DeleteTicketPlugin(claims.CurrentTenantID, uint(pluginID)); err != nil {
		if err.Error() == "插件不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// GetByID 获取工单插件详情
func (h *TicketPluginHandler) GetByID(c *gin.Context) {
	// 获取插件ID
	pluginID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 获取插件
	plugin, err := h.ticketPluginService.GetTicketPlugin(claims.CurrentTenantID, uint(pluginID))
	if err != nil {
		if err.Error() == "插件不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, "获取插件失败")
		return
	}

	response.Success(c, plugin)
}

// List 获取工单插件列表
func (h *TicketPluginHandler) List(c *gin.Context) {
	// 解析分页参数
	params := pagination.ParsePageParams(c)

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 获取插件列表
	plugins, total, err := h.ticketPluginService.ListTicketPlugins(
		claims.CurrentTenantID,
		params.GetOffset(),
		params.GetLimit(),
	)
	if err != nil {
		response.ServerError(c, "获取插件列表失败")
		return
	}

	// 构建分页信息
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, plugins, pageInfo)
}

// TestConnection 测试插件连接
func (h *TicketPluginHandler) TestConnection(c *gin.Context) {
	// 获取插件ID
	pluginID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 测试连接
	result, err := h.ticketPluginService.TestConnection(claims.CurrentTenantID, uint(pluginID))
	if err != nil {
		if err.Error() == "插件不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, "测试连接失败")
		return
	}

	response.Success(c, result)
}

// Enable 启用插件
func (h *TicketPluginHandler) Enable(c *gin.Context) {
	// 获取插件ID
	pluginID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 启用插件
	if err := h.ticketPluginService.EnablePlugin(claims.CurrentTenantID, uint(pluginID)); err != nil {
		if err.Error() == "插件不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, "启用插件失败")
		return
	}

	response.Success(c, nil)
}

// Disable 禁用插件
func (h *TicketPluginHandler) Disable(c *gin.Context) {
	// 获取插件ID
	pluginID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 禁用插件
	if err := h.ticketPluginService.DisablePlugin(claims.CurrentTenantID, uint(pluginID)); err != nil {
		if err.Error() == "插件不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, "禁用插件失败")
		return
	}

	response.Success(c, nil)
}

// ManualSync 手动触发同步
func (h *TicketPluginHandler) ManualSync(c *gin.Context) {
	// 获取插件ID
	pluginID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 触发同步
	if err := h.ticketPluginService.ManualSync(claims.CurrentTenantID, uint(pluginID)); err != nil {
		if err.Error() == "插件不存在" {
			response.NotFound(c, err.Error())
			return
		}
		if err.Error() == "插件未启用同步" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "触发同步失败")
		return
	}

	response.SuccessWithMessage(c, "已触发同步任务", nil)
}

// GetSyncLogs 获取同步日志
func (h *TicketPluginHandler) GetSyncLogs(c *gin.Context) {
	// 获取插件ID
	pluginID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	// 解析分页参数
	params := pagination.ParsePageParams(c)

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 获取同步日志
	logs, total, err := h.ticketPluginService.GetSyncLogs(
		claims.CurrentTenantID,
		uint(pluginID),
		params.GetOffset(),
		params.GetLimit(),
	)
	if err != nil {
		if err.Error() == "插件不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, "获取同步日志失败")
		return
	}

	// 构建分页信息
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, logs, pageInfo)
}

// TestSync 测试同步
func (h *TicketPluginHandler) TestSync(c *gin.Context) {
	// 获取插件ID
	pluginID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	// 解析请求体
	var req services.TestSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 执行测试同步
	result, err := h.ticketPluginService.TestSync(claims.CurrentTenantID, uint(pluginID), req)
	if err != nil {
		if err.Error() == "插件不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, result)
}

// GetSchedulerStatus 获取工单同步调度器状态
func (h *TicketPluginHandler) GetSchedulerStatus(c *gin.Context) {
	// 获取调度器
	scheduler := services.GetGlobalTicketSyncScheduler()
	if scheduler == nil {
		response.ServerError(c, "工单同步调度器未启动")
		return
	}

	// 获取调度状态
	status := map[string]interface{}{
		"running": scheduler.IsRunning(),
		"scheduled_plugins": scheduler.GetScheduledPlugins(),
	}
	
	response.Success(c, status)
}