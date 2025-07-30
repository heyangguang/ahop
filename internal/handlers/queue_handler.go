package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

// QueueHandler 队列处理器
type QueueHandler struct {
	queueService *services.QueueService
}

// NewQueueHandler 创建队列处理器
func NewQueueHandler(queueService *services.QueueService) *QueueHandler {
	return &QueueHandler{
		queueService: queueService,
	}
}

// GetQueueStatus 获取队列状态
func (h *QueueHandler) GetQueueStatus(c *gin.Context) {
	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取队列状态
	status, err := h.queueService.GetQueueStatus(userClaims.CurrentTenantID)
	if err != nil {
		response.ServerError(c, "获取队列状态失败")
		return
	}

	response.Success(c, status)
}

// GetQueueTasks 获取队列任务列表
func (h *QueueHandler) GetQueueTasks(c *gin.Context) {
	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取查询参数
	taskType := c.Query("type")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// 获取任务列表
	tasks, err := h.queueService.GetQueueTasksByType(userClaims.CurrentTenantID, taskType, limit)
	if err != nil {
		response.ServerError(c, "获取任务列表失败")
		return
	}

	response.Success(c, tasks)
}

// CancelQueueTask 取消队列任务
func (h *QueueHandler) CancelQueueTask(c *gin.Context) {
	// 获取任务ID
	taskID := c.Param("id")
	if taskID == "" {
		response.BadRequest(c, "任务ID不能为空")
		return
	}

	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 取消任务
	if err := h.queueService.CancelQueueTask(userClaims.CurrentTenantID, taskID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "任务已取消", nil)
}

// GetQueueStatsByType 按类型获取队列统计
func (h *QueueHandler) GetQueueStatsByType(c *gin.Context) {
	// 获取用户信息
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)

	// 获取统计数据
	status, err := h.queueService.GetQueueStatus(userClaims.CurrentTenantID)
	if err != nil {
		response.ServerError(c, "获取统计数据失败")
		return
	}

	// 构建响应
	stats := map[string]interface{}{
		"by_type": status.TasksByType,
		"by_priority": status.TasksByPriority,
		"summary": map[string]int64{
			"total": status.TotalTasks,
			"pending": status.PendingTasks,
			"running": status.RunningTasks,
			"completed": status.CompletedTasks,
			"failed": status.FailedTasks,
		},
	}

	response.Success(c, stats)
}