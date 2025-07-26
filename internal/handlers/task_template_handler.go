package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

// TaskTemplateHandler 任务模板处理器
type TaskTemplateHandler struct {
	taskTemplateService *services.TaskTemplateService
}

// NewTaskTemplateHandler 创建任务模板处理器
func NewTaskTemplateHandler(taskTemplateService *services.TaskTemplateService) *TaskTemplateHandler {
	return &TaskTemplateHandler{
		taskTemplateService: taskTemplateService,
	}
}

// Create 创建任务模板
func (h *TaskTemplateHandler) Create(c *gin.Context) {
	var req services.CreateTaskTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 创建模板
	template, err := h.taskTemplateService.Create(claims.CurrentTenantID, req, claims.UserID)
	if err != nil {
		if err.Error() == "仓库不存在" || err.Error() == "模板编码已存在" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "创建任务模板失败")
		return
	}

	response.Success(c, template)
}

// Update 更新任务模板
func (h *TaskTemplateHandler) Update(c *gin.Context) {
	// 获取模板ID
	templateID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的模板ID")
		return
	}

	var req services.UpdateTaskTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 更新模板
	template, err := h.taskTemplateService.Update(claims.CurrentTenantID, uint(templateID), req, claims.UserID)
	if err != nil {
		if err.Error() == "任务模板不存在" {
			response.NotFound(c, err.Error())
			return
		}
		if err.Error() == "仓库不存在" || err.Error() == "模板编码已存在" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "更新任务模板失败")
		return
	}

	response.Success(c, template)
}

// Delete 删除任务模板
func (h *TaskTemplateHandler) Delete(c *gin.Context) {
	// 获取模板ID
	templateID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的模板ID")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 删除模板
	if err := h.taskTemplateService.Delete(claims.CurrentTenantID, uint(templateID)); err != nil {
		if err.Error() == "任务模板不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, "删除任务模板失败")
		return
	}

	response.Success(c, nil)
}

// GetByID 获取任务模板详情
func (h *TaskTemplateHandler) GetByID(c *gin.Context) {
	// 获取模板ID
	templateID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的模板ID")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 获取模板
	template, err := h.taskTemplateService.GetByID(claims.CurrentTenantID, uint(templateID))
	if err != nil {
		if err.Error() == "任务模板不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, "获取任务模板失败")
		return
	}

	response.Success(c, template)
}

// List 获取任务模板列表
func (h *TaskTemplateHandler) List(c *gin.Context) {
	var req services.ListTaskTemplateRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 参数验证
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 10
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 获取列表
	templates, total, err := h.taskTemplateService.List(claims.CurrentTenantID, req)
	if err != nil {
		response.ServerError(c, "获取任务模板列表失败")
		return
	}

	// 返回分页响应
	pageInfo := pagination.NewPageInfo(req.Page, req.PageSize, total)
	response.SuccessWithPage(c, templates, pageInfo)
}

// SyncFromWorker 接收Worker上报的任务模板
func (h *TaskTemplateHandler) SyncFromWorker(c *gin.Context) {
	var req struct {
		RepositoryID uint                       `json:"repository_id" binding:"required"`
		Templates    []services.TemplateInfo    `json:"templates" binding:"required"`
		WorkerID     string                     `json:"worker_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 调用服务处理上报
	if err := h.taskTemplateService.SyncFromWorker(req.RepositoryID, req.Templates, req.WorkerID); err != nil {
		if err.Error() == "仓库不存在" {
			response.NotFound(c, err.Error())
			return
		}
		response.ServerError(c, "处理任务模板同步失败")
		return
	}

	response.Success(c, gin.H{
		"message": fmt.Sprintf("成功同步 %d 个任务模板", len(req.Templates)),
	})
}