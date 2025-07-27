package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

// SyncRuleHandler 同步规则处理器
type SyncRuleHandler struct {
	syncRuleService *services.SyncRuleService
}

// NewSyncRuleHandler 创建同步规则处理器
func NewSyncRuleHandler(syncRuleService *services.SyncRuleService) *SyncRuleHandler {
	return &SyncRuleHandler{
		syncRuleService: syncRuleService,
	}
}

// GetByPlugin 获取插件的同步规则
func (h *SyncRuleHandler) GetByPlugin(c *gin.Context) {
	// 获取插件ID
	pluginID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 获取同步规则
	rules, err := h.syncRuleService.GetByPlugin(claims.CurrentTenantID, uint(pluginID))
	if err != nil {
		response.ServerError(c, "获取同步规则失败")
		return
	}

	response.Success(c, rules)
}

// UpdateRules 更新同步规则（统一接口）
func (h *SyncRuleHandler) UpdateRules(c *gin.Context) {
	// 获取插件ID
	pluginID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	// 解析请求
	var req services.UpdateSyncRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: " + err.Error())
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 更新规则
	rules, err := h.syncRuleService.UpdateRules(claims.CurrentTenantID, uint(pluginID), req)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, rules)
}