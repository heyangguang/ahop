package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// GitRepositoryHandler Git仓库处理器
type GitRepositoryHandler struct {
	gitRepoService *services.GitRepositoryService
}

// NewGitRepositoryHandler 创建Git仓库处理器
func NewGitRepositoryHandler(gitRepoService *services.GitRepositoryService) *GitRepositoryHandler {
	return &GitRepositoryHandler{
		gitRepoService: gitRepoService,
	}
}

// Create 创建Git仓库
func (h *GitRepositoryHandler) Create(c *gin.Context) {
	// 解析请求
	var req services.CreateGitRepositoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 调用Service创建
	repo, err := h.gitRepoService.Create(claims.CurrentTenantID, &req)
	if err != nil {
		if strings.Contains(err.Error(), "已存在") {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "创建失败")
		return
	}

	response.SuccessWithMessage(c, "创建成功", repo)
}

// Update 更新Git仓库
func (h *GitRepositoryHandler) Update(c *gin.Context) {
	// 解析仓库ID
	repoID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 解析请求
	var req services.UpdateGitRepositoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 调用Service更新
	repo, err := h.gitRepoService.Update(claims.CurrentTenantID, uint(repoID), &req)
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			response.NotFound(c, "仓库不存在")
			return
		}
		if strings.Contains(err.Error(), "已存在") {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "更新失败")
		return
	}

	response.SuccessWithMessage(c, "更新成功", repo)
}

// Delete 删除Git仓库
func (h *GitRepositoryHandler) Delete(c *gin.Context) {
	// 解析仓库ID
	repoID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 调用Service删除
	if err := h.gitRepoService.Delete(claims.CurrentTenantID, uint(repoID)); err != nil {
		if strings.Contains(err.Error(), "不存在") {
			response.NotFound(c, "仓库不存在")
			return
		}
		response.ServerError(c, "删除失败")
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}

// GetByID 获取单个Git仓库
func (h *GitRepositoryHandler) GetByID(c *gin.Context) {
	// 解析仓库ID
	repoID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 调用Service获取
	repo, err := h.gitRepoService.GetByID(claims.CurrentTenantID, uint(repoID))
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			response.NotFound(c, "仓库不存在")
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, repo)
}

// List 获取Git仓库列表
func (h *GitRepositoryHandler) List(c *gin.Context) {
	// 解析分页参数
	var query services.ListGitRepositoryQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		// 如果绑定失败，使用默认值
		query.Page = 1
		query.PageSize = 10
	}

	// 处理分页参数
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 10
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 调用Service查询
	repos, total, err := h.gitRepoService.List(claims.CurrentTenantID, &query)
	if err != nil {
		response.ServerError(c, "查询失败")
		return
	}

	// 构建分页响应
	pageInfo := pagination.NewPageInfo(query.Page, query.PageSize, total)
	response.SuccessWithPage(c, repos, pageInfo)
}

// ManualSync 手动同步仓库
func (h *GitRepositoryHandler) ManualSync(c *gin.Context) {
	// 解析仓库ID
	repoID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 调用Service触发同步
	if err := h.gitRepoService.ManualSync(claims.CurrentTenantID, uint(repoID), claims.UserID); err != nil {
		if strings.Contains(err.Error(), "不存在") {
			response.NotFound(c, "仓库不存在")
			return
		}
		response.ServerError(c, "触发同步失败")
		return
	}

	response.SuccessWithMessage(c, "已触发同步任务", nil)
}

// GetSyncLogs 获取同步日志
func (h *GitRepositoryHandler) GetSyncLogs(c *gin.Context) {
	// 解析仓库ID
	repoID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 构建查询参数
	var query services.ListSyncLogsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		// 如果绑定失败，使用默认值
		query.Page = 1
		query.PageSize = 10
	}

	// 处理分页参数
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 10
	}

	// 调用Service查询
	logs, total, err := h.gitRepoService.GetSyncLogs(claims.CurrentTenantID, uint(repoID), &query)
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			response.NotFound(c, "仓库不存在")
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	// 构建分页响应
	pageInfo := pagination.NewPageInfo(query.Page, query.PageSize, total)
	response.SuccessWithPage(c, logs, pageInfo)
}

// BatchSync 批量同步仓库
func (h *GitRepositoryHandler) BatchSync(c *gin.Context) {
	// 解析请求
	var req services.BatchSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 调用Service批量同步
	results := h.gitRepoService.BatchSync(claims.CurrentTenantID, req.RepositoryIDs, claims.UserID)

	response.Success(c, results)
}

// ScanTemplates 扫描仓库中的任务模板
func (h *GitRepositoryHandler) ScanTemplates(c *gin.Context) {
	// 解析仓库ID
	repoID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 调用Service扫描
	result, err := h.gitRepoService.ScanTemplates(claims.CurrentTenantID, uint(repoID))
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			response.NotFound(c, err.Error())
			return
		}
		if strings.Contains(err.Error(), "超时") {
			response.ServerError(c, "扫描超时，请稍后重试")
			return
		}
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, result)
}

// GetSchedulerStatus 获取Git同步调度器状态
func (h *GitRepositoryHandler) GetSchedulerStatus(c *gin.Context) {
	// 获取调度器
	scheduler := services.GetGlobalGitSyncScheduler()
	if scheduler == nil {
		response.ServerError(c, "Git同步调度器未启动")
		return
	}

	// 获取调度状态
	status := scheduler.GetJobStatus()
	response.Success(c, status)
}