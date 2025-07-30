package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HealingExecutionHandler 自愈执行处理器
type HealingExecutionHandler struct {
	workflowExecutor *services.WorkflowExecutor
}

// NewHealingExecutionHandler 创建自愈执行处理器
func NewHealingExecutionHandler(db interface{}, taskService *services.TaskService, ticketService *services.TicketService) *HealingExecutionHandler {
	return &HealingExecutionHandler{
		workflowExecutor: services.NewWorkflowExecutor(db.(*gorm.DB), taskService, ticketService),
	}
}

// List 获取执行历史列表
func (h *HealingExecutionHandler) List(c *gin.Context) {
	// 获取分页参数
	params := pagination.ParsePageParams(c)

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取查询参数
	workflowID, _ := strconv.ParseUint(c.Query("workflow_id"), 10, 32)
	ruleID, _ := strconv.ParseUint(c.Query("rule_id"), 10, 32)
	status := c.Query("status")
	triggerType := c.Query("trigger_type")

	// 查询执行历史
	executions, total, err := h.workflowExecutor.ListExecutions(
		userClaims.CurrentTenantID,
		params,
		uint(workflowID),
		uint(ruleID),
		status,
		triggerType,
	)
	if err != nil {
		response.ServerError(c, "获取执行历史失败")
		return
	}

	// 转换为精简的列表格式
	listItems := make([]*services.ExecutionListItem, 0, len(executions))
	for _, execution := range executions {
		listItems = append(listItems, services.ConvertToExecutionListItem(&execution))
	}

	// 创建分页信息
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, listItems, pageInfo)
}

// GetByID 获取执行详情
func (h *HealingExecutionHandler) GetByID(c *gin.Context) {
	// 获取执行ID
	executionID := c.Param("id")

	// 先获取基本信息进行权限检查
	execution, err := h.workflowExecutor.GetExecution(executionID)
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	// 获取用户信息进行权限检查
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 检查租户权限
	if execution.TenantID != userClaims.CurrentTenantID && !userClaims.IsPlatformAdmin {
		response.Forbidden(c, "无权查看此执行记录")
		return
	}

	// 获取精简的执行详情
	detail, err := h.workflowExecutor.GetExecutionDetail(executionID)
	if err != nil {
		response.ServerError(c, "获取执行详情失败")
		return
	}

	response.Success(c, detail)
}

// GetLogs 获取执行日志
func (h *HealingExecutionHandler) GetLogs(c *gin.Context) {
	// 获取执行ID（这里的id参数是execution_id字符串）
	executionID := c.Param("id")

	// 先获取执行详情进行权限检查
	execution, err := h.workflowExecutor.GetExecution(executionID)
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	// 获取用户信息进行权限检查
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 检查租户权限
	if execution.TenantID != userClaims.CurrentTenantID && !userClaims.IsPlatformAdmin {
		response.Forbidden(c, "无权查看此执行日志")
		return
	}

	// 获取执行日志（使用数据库ID）
	logs, err := h.workflowExecutor.GetExecutionLogs(execution.ID)
	if err != nil {
		response.ServerError(c, "获取执行日志失败")
		return
	}

	response.Success(c, logs)
}