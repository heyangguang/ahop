package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"github.com/gin-gonic/gin"
	"strconv"
)

// HealingRuleHandler 自愈规则处理器
type HealingRuleHandler struct {
	healingRuleService *services.HealingRuleService
}

// NewHealingRuleHandler 创建自愈规则处理器
func NewHealingRuleHandler(healingRuleService *services.HealingRuleService) *HealingRuleHandler {
	return &HealingRuleHandler{
		healingRuleService: healingRuleService,
	}
}

// Create 创建自愈规则
func (h *HealingRuleHandler) Create(c *gin.Context) {
	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 解析请求
	var req services.CreateHealingRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 创建规则
	rule, err := h.healingRuleService.Create(userClaims.CurrentTenantID, userClaims.UserID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, rule)
}

// Update 更新自愈规则
func (h *HealingRuleHandler) Update(c *gin.Context) {
	// 获取规则ID
	ruleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的规则ID")
		return
	}

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 解析请求
	var req services.UpdateHealingRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 更新规则
	rule, err := h.healingRuleService.Update(userClaims.CurrentTenantID, uint(ruleID), userClaims.UserID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, rule)
}

// Delete 删除自愈规则
func (h *HealingRuleHandler) Delete(c *gin.Context) {
	// 获取规则ID
	ruleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的规则ID")
		return
	}

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 删除规则
	if err := h.healingRuleService.Delete(userClaims.CurrentTenantID, uint(ruleID)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// GetByID 根据ID获取自愈规则
func (h *HealingRuleHandler) GetByID(c *gin.Context) {
	// 获取规则ID
	ruleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的规则ID")
		return
	}

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取规则
	rule, err := h.healingRuleService.GetByID(userClaims.CurrentTenantID, uint(ruleID))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, rule)
}

// List 获取自愈规则列表
func (h *HealingRuleHandler) List(c *gin.Context) {
	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取分页参数
	params := pagination.ParsePageParams(c)
	search := c.Query("search")

	// 获取规则列表
	rules, total, err := h.healingRuleService.List(userClaims.CurrentTenantID, params, search)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	// 构建分页信息
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, rules, pageInfo)
}

// Enable 启用规则
func (h *HealingRuleHandler) Enable(c *gin.Context) {
	// 获取规则ID
	ruleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的规则ID")
		return
	}

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 启用规则
	if err := h.healingRuleService.Enable(userClaims.CurrentTenantID, uint(ruleID), userClaims.UserID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// Disable 禁用规则
func (h *HealingRuleHandler) Disable(c *gin.Context) {
	// 获取规则ID
	ruleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的规则ID")
		return
	}

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 禁用规则
	if err := h.healingRuleService.Disable(userClaims.CurrentTenantID, uint(ruleID), userClaims.UserID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// Execute 手动执行规则
func (h *HealingRuleHandler) Execute(c *gin.Context) {
	// 获取规则ID
	ruleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的规则ID")
		return
	}

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取调度器
	scheduler := services.GetGlobalHealingScheduler()
	if scheduler == nil {
		response.ServerError(c, "自愈调度器未启动")
		return
	}

	// 手动执行规则
	execution, err := scheduler.ExecuteManualRule(uint(ruleID), userClaims.UserID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, execution)
}

// GetSchedulerStatus 获取调度器状态
func (h *HealingRuleHandler) GetSchedulerStatus(c *gin.Context) {
	// 获取调度器
	scheduler := services.GetGlobalHealingScheduler()
	if scheduler == nil {
		response.ServerError(c, "自愈调度器未启动")
		return
	}

	// 获取调度状态
	status := scheduler.GetSchedulerStatus()
	response.Success(c, status)
}