package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/pagination"
	"ahop/pkg/response"

	"github.com/gin-gonic/gin"
)

// WorkerManagerHandler 分布式Worker管理处理器
type WorkerManagerHandler struct {
	workerManagerService *services.WorkerManagerService
}

// NewWorkerManagerHandler 创建Worker管理处理器
func NewWorkerManagerHandler(workerManagerService *services.WorkerManagerService) *WorkerManagerHandler {
	return &WorkerManagerHandler{
		workerManagerService: workerManagerService,
	}
}

// GetAllWorkers 获取所有Worker状态
func (h *WorkerManagerHandler) GetAllWorkers(c *gin.Context) {
	workers, err := h.workerManagerService.GetAllWorkers()
	if err != nil {
		response.ServerError(c, "获取Worker列表失败: "+err.Error())
		return
	}

	response.Success(c, workers)
}

// GetWorkerByID 获取单个Worker详情
func (h *WorkerManagerHandler) GetWorkerByID(c *gin.Context) {
	workerID := c.Param("worker_id")
	if workerID == "" {
		response.BadRequest(c, "缺少worker_id参数")
		return
	}

	worker, err := h.workerManagerService.GetWorkerByID(workerID)
	if err != nil {
		if err.Error() == "Worker不存在" {
			response.NotFound(c, "Worker不存在")
			return
		}
		response.ServerError(c, "获取Worker详情失败: "+err.Error())
		return
	}

	response.Success(c, worker)
}

// GetWorkerSummary 获取Worker汇总统计
func (h *WorkerManagerHandler) GetWorkerSummary(c *gin.Context) {
	summary, err := h.workerManagerService.GetWorkerSummary()
	if err != nil {
		response.ServerError(c, "获取Worker统计失败: "+err.Error())
		return
	}

	response.Success(c, summary)
}

// GetQueueStats 获取队列统计信息
func (h *WorkerManagerHandler) GetQueueStats(c *gin.Context) {
	stats, err := h.workerManagerService.GetQueueStats()
	if err != nil {
		response.ServerError(c, "获取队列统计失败: "+err.Error())
		return
	}

	response.Success(c, stats)
}

// RemoveOfflineWorker 移除离线Worker
func (h *WorkerManagerHandler) RemoveOfflineWorker(c *gin.Context) {
	workerID := c.Param("worker_id")
	if workerID == "" {
		response.BadRequest(c, "缺少worker_id参数")
		return
	}

	err := h.workerManagerService.RemoveOfflineWorker(workerID)
	if err != nil {
		if err.Error() == "Worker不存在" {
			response.NotFound(c, "Worker不存在")
			return
		}
		if err.Error() == "Worker仍在线，无法移除" {
			response.BadRequest(c, "Worker仍在线，无法移除")
			return
		}
		response.ServerError(c, "移除Worker失败: "+err.Error())
		return
	}

	response.SuccessWithMessage(c, "Worker已移除", map[string]interface{}{
		"worker_id": workerID,
	})
}

// GetWorkerTasks 获取Worker任务执行历史
func (h *WorkerManagerHandler) GetWorkerTasks(c *gin.Context) {
	workerID := c.Param("worker_id")
	if workerID == "" {
		response.BadRequest(c, "缺少worker_id参数")
		return
	}

	// 解析分页参数
	params := pagination.ParsePageParams(c)

	tasks, total, err := h.workerManagerService.GetWorkerTasks(workerID, params.Page, params.PageSize)
	if err != nil {
		response.ServerError(c, "获取Worker任务历史失败: "+err.Error())
		return
	}

	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, tasks, pageInfo)
}

// GetActiveWorkers 获取活跃Worker
func (h *WorkerManagerHandler) GetActiveWorkers(c *gin.Context) {
	workers, err := h.workerManagerService.GetActiveWorkers()
	if err != nil {
		response.ServerError(c, "获取活跃Worker失败: "+err.Error())
		return
	}

	response.Success(c, workers)
}

// GetWorkersByStatus 根据状态获取Worker
func (h *WorkerManagerHandler) GetWorkersByStatus(c *gin.Context) {
	statusParam := c.Query("status")
	if statusParam == "" {
		response.BadRequest(c, "缺少status参数 (online/offline)")
		return
	}

	var online bool
	switch statusParam {
	case "online":
		online = true
	case "offline":
		online = false
	default:
		response.BadRequest(c, "status参数只能是online或offline")
		return
	}

	workers, err := h.workerManagerService.GetWorkersByStatus(online)
	if err != nil {
		response.ServerError(c, "获取Worker失败: "+err.Error())
		return
	}

	response.Success(c, workers)
}

// GetDashboardData 获取Worker仪表盘数据
func (h *WorkerManagerHandler) GetDashboardData(c *gin.Context) {
	// 获取Worker汇总
	summary, err := h.workerManagerService.GetWorkerSummary()
	if err != nil {
		response.ServerError(c, "获取Worker统计失败: "+err.Error())
		return
	}

	// 获取队列统计
	queueStats, err := h.workerManagerService.GetQueueStats()
	if err != nil {
		response.ServerError(c, "获取队列统计失败: "+err.Error())
		return
	}

	// 获取活跃Worker
	activeWorkers, err := h.workerManagerService.GetActiveWorkers()
	if err != nil {
		response.ServerError(c, "获取活跃Worker失败: "+err.Error())
		return
	}

	// 组装仪表盘数据
	dashboardData := map[string]interface{}{
		"worker_summary":  summary,
		"queue_stats":     queueStats,
		"active_workers":  activeWorkers,
		"recent_workers":  activeWorkers[:min(len(activeWorkers), 5)], // 最近5个活跃Worker
	}

	response.Success(c, dashboardData)
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}