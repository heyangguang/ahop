package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"github.com/gin-gonic/gin"
	"strconv"
)

// HealingWorkflowHandler 自愈工作流处理器
type HealingWorkflowHandler struct {
	healingWorkflowService *services.HealingWorkflowService
}

// NewHealingWorkflowHandler 创建自愈工作流处理器
func NewHealingWorkflowHandler(healingWorkflowService *services.HealingWorkflowService) *HealingWorkflowHandler {
	return &HealingWorkflowHandler{
		healingWorkflowService: healingWorkflowService,
	}
}

// Create 创建自愈工作流
func (h *HealingWorkflowHandler) Create(c *gin.Context) {
	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 解析请求
	var req services.CreateHealingWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 创建工作流
	workflow, err := h.healingWorkflowService.Create(userClaims.CurrentTenantID, userClaims.UserID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, workflow)
}

// Update 更新自愈工作流
func (h *HealingWorkflowHandler) Update(c *gin.Context) {
	// 获取工作流ID
	workflowID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的工作流ID")
		return
	}

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 解析请求
	var req services.UpdateHealingWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 更新工作流
	workflow, err := h.healingWorkflowService.Update(userClaims.CurrentTenantID, uint(workflowID), userClaims.UserID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, workflow)
}

// Delete 删除自愈工作流
func (h *HealingWorkflowHandler) Delete(c *gin.Context) {
	// 获取工作流ID
	workflowID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的工作流ID")
		return
	}

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 删除工作流
	if err := h.healingWorkflowService.Delete(userClaims.CurrentTenantID, uint(workflowID)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// GetByID 根据ID获取自愈工作流
func (h *HealingWorkflowHandler) GetByID(c *gin.Context) {
	// 获取工作流ID
	workflowID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的工作流ID")
		return
	}

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取工作流
	workflow, err := h.healingWorkflowService.GetByID(userClaims.CurrentTenantID, uint(workflowID))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, workflow)
}

// List 获取自愈工作流列表
func (h *HealingWorkflowHandler) List(c *gin.Context) {
	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取分页参数
	params := pagination.ParsePageParams(c)
	search := c.Query("search")

	// 获取工作流列表
	workflows, total, err := h.healingWorkflowService.List(userClaims.CurrentTenantID, params, search)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	// 构建分页信息
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, workflows, pageInfo)
}

// Enable 启用工作流
func (h *HealingWorkflowHandler) Enable(c *gin.Context) {
	// 获取工作流ID
	workflowID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的工作流ID")
		return
	}

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 启用工作流
	if err := h.healingWorkflowService.Enable(userClaims.CurrentTenantID, uint(workflowID), userClaims.UserID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// Disable 禁用工作流
func (h *HealingWorkflowHandler) Disable(c *gin.Context) {
	// 获取工作流ID
	workflowID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的工作流ID")
		return
	}

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 禁用工作流
	if err := h.healingWorkflowService.Disable(userClaims.CurrentTenantID, uint(workflowID), userClaims.UserID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// Clone 克隆工作流
func (h *HealingWorkflowHandler) Clone(c *gin.Context) {
	// 获取工作流ID
	workflowID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的工作流ID")
		return
	}

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 解析请求
	var req struct {
		Code string `json:"code" binding:"required,max=100"`
		Name string `json:"name" binding:"required,max=200"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 克隆工作流
	workflow, err := h.healingWorkflowService.Clone(userClaims.CurrentTenantID, uint(workflowID), userClaims.UserID, req.Code, req.Name)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, workflow)
}

// Execute 手动执行工作流
func (h *HealingWorkflowHandler) Execute(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	// 请求参数
	var req struct {
		TriggerType   string                 `json:"trigger_type" binding:"required"`
		TriggerSource map[string]interface{} `json:"trigger_source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 从 JWT 中获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取工作流
	workflow, err := h.healingWorkflowService.GetByID(userClaims.CurrentTenantID, uint(id))
	if err != nil {
		if err.Error() == "工作流不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, err.Error())
		return
	}

	// 检查工作流状态
	if !workflow.IsActive {
		response.BadRequest(c, "工作流未启用")
		return
	}

	// 执行工作流
	execution, err := h.healingWorkflowService.Execute(workflow, req.TriggerType, req.TriggerSource, &userClaims.UserID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, execution)
}