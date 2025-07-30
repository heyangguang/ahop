package handlers

import (
	"ahop/internal/database"
	"ahop/internal/services"
	"ahop/pkg/response"
	"github.com/gin-gonic/gin"
)

// SystemHandler 系统处理器
type SystemHandler struct {
	schedulerMonitor *services.SchedulerMonitor
}

// NewSystemHandler 创建系统处理器
func NewSystemHandler() *SystemHandler {
	return &SystemHandler{
		schedulerMonitor: services.NewSchedulerMonitor(database.GetDB()),
	}
}

// GetAllSchedulersStatus 获取所有调度器状态
func (h *SystemHandler) GetAllSchedulersStatus(c *gin.Context) {
	status := h.schedulerMonitor.GetAllSchedulersStatus()
	response.Success(c, status)
}