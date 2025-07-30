package handlers

import (
	"ahop/internal/models"
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// HealingRuleExecutionHandler 自愈规则执行记录处理器
type HealingRuleExecutionHandler struct {
	executionService *services.HealingRuleExecutionService
	ruleService     *services.HealingRuleService
}

// NewHealingRuleExecutionHandler 创建处理器实例
func NewHealingRuleExecutionHandler(executionService *services.HealingRuleExecutionService, ruleService *services.HealingRuleService) *HealingRuleExecutionHandler {
	return &HealingRuleExecutionHandler{
		executionService: executionService,
		ruleService:     ruleService,
	}
}

// GetByID 获取执行记录详情
func (h *HealingRuleExecutionHandler) GetByID(c *gin.Context) {
	// 解析ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	// 获取当前用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取执行记录
	execution, err := h.executionService.GetByID(uint(id))
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	// 验证租户权限
	if execution.TenantID != userClaims.CurrentTenantID && !userClaims.IsPlatformAdmin {
		response.Forbidden(c, "无权访问该执行记录")
		return
	}

	response.Success(c, execution)
}

// GetByRuleID 获取指定规则的执行记录
func (h *HealingRuleExecutionHandler) GetByRuleID(c *gin.Context) {
	// 解析规则ID
	ruleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的规则ID")
		return
	}

	// 获取当前用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 验证规则是否存在并属于当前租户
	rule, err := h.ruleService.GetByID(userClaims.CurrentTenantID, uint(ruleID))
	if err != nil {
		response.NotFound(c, "规则不存在")
		return
	}

	// 分页参数
	params := pagination.ParsePageParams(c)

	// 获取执行记录
	executions, total, err := h.executionService.GetByRuleID(rule.ID, params)
	if err != nil {
		response.ServerError(c, "获取执行记录失败")
		return
	}

	// 简化返回数据，重点展示匹配的工单
	type SimpleExecution struct {
		ID                  uint                         `json:"id"`
		ExecutionTime       time.Time                    `json:"execution_time"`
		Status              string                       `json:"status"`
		TotalTicketsScanned int                          `json:"total_tickets_scanned"`
		MatchedTickets      int                          `json:"matched_tickets"`
		ExecutionsCreated   int                          `json:"executions_created"`
		MatchedTicketList   []models.MatchedTicketInfo   `json:"matched_ticket_list,omitempty"`
		Duration            int                          `json:"duration"`
		ErrorMsg            string                       `json:"error_msg,omitempty"`
	}

	simpleExecutions := make([]SimpleExecution, 0, len(executions))
	for _, exec := range executions {
		matchedTickets, _ := exec.GetMatchedTickets()
		
		simple := SimpleExecution{
			ID:                  exec.ID,
			ExecutionTime:       exec.ExecutionTime,
			Status:              exec.Status,
			TotalTicketsScanned: exec.TotalTicketsScanned,
			MatchedTickets:      exec.MatchedTickets,
			ExecutionsCreated:   exec.ExecutionsCreated,
			MatchedTicketList:   matchedTickets,
			Duration:            exec.Duration,
			ErrorMsg:            exec.ErrorMsg,
		}
		
		simpleExecutions = append(simpleExecutions, simple)
	}

	// 构建分页信息
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, simpleExecutions, pageInfo)
}

// GetList 获取执行记录列表
func (h *HealingRuleExecutionHandler) GetList(c *gin.Context) {
	// 获取当前用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 分页参数
	params := pagination.ParsePageParams(c)

	// 获取执行记录
	executions, total, err := h.executionService.GetByTenant(userClaims.CurrentTenantID, params)
	if err != nil {
		response.ServerError(c, "获取执行记录失败")
		return
	}

	// 构建分页信息
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, executions, pageInfo)
}

// GetStats 获取执行统计
func (h *HealingRuleExecutionHandler) GetStats(c *gin.Context) {
	// 获取当前用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 解析时间范围
	var startTime, endTime *time.Time
	
	if start := c.Query("start_time"); start != "" {
		t, err := time.Parse(time.RFC3339, start)
		if err == nil {
			startTime = &t
		}
	}
	
	if end := c.Query("end_time"); end != "" {
		t, err := time.Parse(time.RFC3339, end)
		if err == nil {
			endTime = &t
		}
	}

	// 获取统计数据
	stats, err := h.executionService.GetRuleExecutionStats(userClaims.CurrentTenantID, startTime, endTime)
	if err != nil {
		response.ServerError(c, "获取统计数据失败")
		return
	}

	response.Success(c, stats)
}

// GetRecent 获取最近的执行记录
func (h *HealingRuleExecutionHandler) GetRecent(c *gin.Context) {
	// 获取当前用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 解析限制数量
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsedLimit, err := strconv.Atoi(l); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// 获取最近的执行记录
	executions, err := h.executionService.GetRecentExecutions(userClaims.CurrentTenantID, limit)
	if err != nil {
		response.ServerError(c, "获取执行记录失败")
		return
	}

	response.Success(c, executions)
}

// GetDetail 获取执行详情（包含匹配的工单和创建的执行）
func (h *HealingRuleExecutionHandler) GetDetail(c *gin.Context) {
	// 解析ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	// 获取当前用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取执行记录
	execution, err := h.executionService.GetByID(uint(id))
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	// 验证租户权限
	if execution.TenantID != userClaims.CurrentTenantID && !userClaims.IsPlatformAdmin {
		response.Forbidden(c, "无权访问该执行记录")
		return
	}

	// 获取详细信息
	detail, err := h.executionService.GetExecutionDetail(uint(id))
	if err != nil {
		response.ServerError(c, "获取执行详情失败")
		return
	}

	response.Success(c, detail)
}