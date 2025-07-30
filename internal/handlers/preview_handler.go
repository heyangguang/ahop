package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PreviewHandler 预览处理器
type PreviewHandler struct {
	previewService  *services.PreviewService
	ruleService     *services.HealingRuleService
	workflowService *services.HealingWorkflowService
}

// NewPreviewHandler 创建预览处理器
func NewPreviewHandler(db *gorm.DB) *PreviewHandler {
	return &PreviewHandler{
		previewService:  services.NewPreviewService(db),
		ruleService:     services.NewHealingRuleService(db),
		workflowService: services.NewHealingWorkflowService(db),
	}
}

// PreviewRule 预览规则匹配
func (h *PreviewHandler) PreviewRule(c *gin.Context) {
	// 解析规则ID
	ruleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的规则ID")
		return
	}

	// 获取当前用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取规则
	rule, err := h.ruleService.GetByID(userClaims.CurrentTenantID, uint(ruleID))
	if err != nil {
		response.NotFound(c, "规则不存在")
		return
	}

	// 解析请求
	var req services.RulePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	// 验证测试数据
	if len(req.TestTickets) == 0 {
		response.BadRequest(c, "请提供测试工单数据")
		return
	}

	// 预览规则
	result, err := h.previewService.PreviewRule(rule, &req)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, result)
}

// PreviewWorkflow 预览工作流执行
func (h *PreviewHandler) PreviewWorkflow(c *gin.Context) {
	// 解析工作流ID
	workflowID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的工作流ID")
		return
	}

	// 获取当前用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取工作流
	workflow, err := h.workflowService.GetByID(userClaims.CurrentTenantID, uint(workflowID))
	if err != nil {
		response.NotFound(c, "工作流不存在")
		return
	}

	// 解析请求
	var req services.WorkflowPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	// 验证触发数据
	if req.TriggerData == nil {
		req.TriggerData = make(map[string]interface{})
	}

	// 预览工作流
	result, err := h.previewService.PreviewWorkflow(workflow, &req)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, result)
}