package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ScheduledTaskHandler 定时任务处理器
type ScheduledTaskHandler struct {
	schedulerService *services.TaskSchedulerService
}

// NewScheduledTaskHandler 创建定时任务处理器
func NewScheduledTaskHandler() *ScheduledTaskHandler {
	return &ScheduledTaskHandler{
		schedulerService: services.GetGlobalTaskScheduler(),
	}
}

// Create 创建定时任务
func (h *ScheduledTaskHandler) Create(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	var req services.CreateScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 创建定时任务
	scheduledTask, err := h.schedulerService.CreateScheduledTask(claims.CurrentTenantID, claims.UserID, &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "创建成功", scheduledTask)
}

// List 获取定时任务列表
func (h *ScheduledTaskHandler) List(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)
	params := pagination.ParsePageParams(c)

	// 解析过滤条件
	filters := make(map[string]interface{})
	
	// 是否启用
	if isActive := c.Query("is_active"); isActive != "" {
		if isActive == "true" {
			filters["is_active"] = true
		} else if isActive == "false" {
			filters["is_active"] = false
		}
	}
	
	// 任务模板ID
	if templateID := c.Query("template_id"); templateID != "" {
		if id, err := strconv.ParseUint(templateID, 10, 32); err == nil {
			filters["template_id"] = uint(id)
		}
	}
	
	// 执行状态
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	
	// 名称搜索
	if name := c.Query("name"); name != "" {
		filters["name"] = name
	}

	// 获取列表
	tasks, total, err := h.schedulerService.List(claims.CurrentTenantID, params.Page, params.PageSize, filters)
	if err != nil {
		response.ServerError(c, "获取列表失败: "+err.Error())
		return
	}

	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, tasks, pageInfo)
}

// GetByID 获取定时任务详情
func (h *ScheduledTaskHandler) GetByID(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 解析ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	// 获取详情
	scheduledTask, err := h.schedulerService.GetByID(uint(id), claims.CurrentTenantID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, scheduledTask)
}

// Update 更新定时任务
func (h *ScheduledTaskHandler) Update(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 解析ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	var req services.UpdateScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 更新定时任务
	scheduledTask, err := h.schedulerService.UpdateScheduledTask(uint(id), claims.CurrentTenantID, claims.UserID, &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "更新成功", scheduledTask)
}

// Delete 删除定时任务
func (h *ScheduledTaskHandler) Delete(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 解析ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	// 删除定时任务
	if err := h.schedulerService.DeleteScheduledTask(uint(id), claims.CurrentTenantID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}

// Enable 启用定时任务
func (h *ScheduledTaskHandler) Enable(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 解析ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	// 启用定时任务
	if err := h.schedulerService.EnableScheduledTask(uint(id), claims.CurrentTenantID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "启用成功", nil)
}

// Disable 禁用定时任务
func (h *ScheduledTaskHandler) Disable(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 解析ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	// 禁用定时任务
	if err := h.schedulerService.DisableScheduledTask(uint(id), claims.CurrentTenantID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "禁用成功", nil)
}

// RunNow 立即执行定时任务
func (h *ScheduledTaskHandler) RunNow(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 解析ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	// 立即执行
	task, err := h.schedulerService.RunScheduledTaskNow(uint(id), claims.CurrentTenantID, claims.UserID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "任务已触发", gin.H{
		"task_id": task.TaskID,
		"task_name": task.Name,
	})
}

// GetExecutions 获取执行历史
func (h *ScheduledTaskHandler) GetExecutions(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)
	params := pagination.ParsePageParams(c)

	// 解析ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	// 获取执行历史
	executions, total, err := h.schedulerService.GetExecutionHistory(uint(id), claims.CurrentTenantID, params.Page, params.PageSize)
	if err != nil {
		response.ServerError(c, "获取执行历史失败: "+err.Error())
		return
	}

	// 构建响应数据
	type ExecutionResponse struct {
		ID          uint   `json:"id"`
		TaskID      string `json:"task_id"`
		TriggeredAt string `json:"triggered_at"`
		TaskName    string `json:"task_name,omitempty"`
		TaskStatus  string `json:"task_status,omitempty"`
		StartedAt   string `json:"started_at,omitempty"`
		FinishedAt  string `json:"finished_at,omitempty"`
		Duration    int64  `json:"duration,omitempty"` // 执行时长（秒）
	}

	results := make([]ExecutionResponse, len(executions))
	for i, exec := range executions {
		resp := ExecutionResponse{
			ID:          exec.ID,
			TaskID:      exec.TaskID,
			TriggeredAt: exec.TriggeredAt.Format("2006-01-02 15:04:05"),
		}

		// 如果关联了任务，添加任务信息
		if exec.Task != nil && exec.Task.ID > 0 {
			resp.TaskName = exec.Task.Name
			resp.TaskStatus = exec.Task.Status
			
			if exec.Task.StartedAt != nil {
				resp.StartedAt = exec.Task.StartedAt.Format("2006-01-02 15:04:05")
			}
			
			if exec.Task.FinishedAt != nil {
				resp.FinishedAt = exec.Task.FinishedAt.Format("2006-01-02 15:04:05")
				
				// 计算执行时长
				if exec.Task.StartedAt != nil {
					duration := exec.Task.FinishedAt.Sub(*exec.Task.StartedAt)
					resp.Duration = int64(duration.Seconds())
				}
			}
		}

		results[i] = resp
	}

	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, results, pageInfo)
}

// GetSchedulerStatus 获取调度器统计信息
func (h *ScheduledTaskHandler) GetSchedulerStatus(c *gin.Context) {
	stats, err := h.schedulerService.GetSchedulerStatistics()
	if err != nil {
		response.ServerError(c, "获取统计信息失败: "+err.Error())
		return
	}
	response.Success(c, stats)
}

// GetLogs 获取定时任务的执行日志
func (h *ScheduledTaskHandler) GetLogs(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)
	
	// 获取路径参数
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的任务ID")
		return
	}
	
	// 分页参数
	params := pagination.ParsePageParams(c)
	
	// 查询过滤条件
	filters := make(map[string]interface{})
	
	// 特定执行ID的日志
	if executionID := c.Query("execution_id"); executionID != "" {
		if execID, err := strconv.ParseUint(executionID, 10, 64); err == nil {
			filters["execution_id"] = uint(execID)
		}
	}
	
	// 日志级别过滤
	if level := c.Query("level"); level != "" {
		filters["level"] = level
	}
	
	// 主机名过滤
	if host := c.Query("host"); host != "" {
		filters["host"] = host
	}
	
	// 关键词搜索
	if keyword := c.Query("keyword"); keyword != "" {
		filters["keyword"] = keyword
	}
	
	// 时间范围过滤
	if startTime := c.Query("start_time"); startTime != "" {
		filters["start_time"] = startTime
	}
	if endTime := c.Query("end_time"); endTime != "" {
		filters["end_time"] = endTime
	}
	
	// 获取日志
	logs, total, err := h.schedulerService.GetTaskLogs(uint(id), claims.CurrentTenantID, params.Page, params.PageSize, filters)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	
	// 返回分页数据
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, logs, pageInfo)
}