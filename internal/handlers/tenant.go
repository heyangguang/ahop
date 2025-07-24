package handlers

import (
	"ahop/pkg/pagination"
	"errors"
	"strconv"
	"strings"

	"ahop/internal/services"
	"ahop/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CreateTenantRequest 请求结构体
type CreateTenantRequest struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type UpdateTenantRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type TenantHandler struct {
	service *services.TenantService
}

func NewTenantHandler(service *services.TenantService) *TenantHandler {
	return &TenantHandler{
		service: service,
	}
}

// Create 创建租户
func (h *TenantHandler) Create(c *gin.Context) {
	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	tenant, err := h.service.Create(req.Name, req.Code)
	if err != nil {
		// 🔧 统一处理：重复代码错误 -> 400
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			response.BadRequest(c, "租户代码已存在")
			return
		}

		// 🔧 统一处理：验证错误 -> 400
		errMsg := err.Error()
		if strings.Contains(errMsg, "租户名称长度") || strings.Contains(errMsg, "租户代码长度") {
			response.BadRequest(c, errMsg)
			return
		}

		// 系统错误 -> 500
		response.ServerError(c, "创建失败")
		return
	}

	response.Success(c, tenant)
}

// GetByID 获取租户
func (h *TenantHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	tenant, err := h.service.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "租户不存在") // 🔧 改为404
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, tenant)
}

// GetAll 替换现有的GetAll方法
func (h *TenantHandler) GetAll(c *gin.Context) {
	// 解析分页参数
	pageParams := pagination.ParsePageParams(c)

	// 支持按状态筛选、关键词搜索
	status := c.Query("status")
	keyword := c.Query("keyword")

	// 使用万能查询方法
	tenants, total, err := h.service.GetWithFiltersAndPage(status, keyword, pageParams.Page, pageParams.PageSize)

	if err != nil {
		response.ServerError(c, "查询失败")
		return
	}

	// 计算分页信息
	pageInfo := pagination.NewPageInfo(pageParams.Page, pageParams.PageSize, total)
	response.SuccessWithPage(c, tenants, pageInfo)
}

// Update 更新租户
func (h *TenantHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	var req UpdateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	tenant, err := h.service.Update(uint(id), req.Name, req.Status)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "租户不存在") // 🔧 改为404
			return
		}

		// 🔧 新增：处理验证错误
		errMsg := err.Error()
		if strings.Contains(errMsg, "租户名称长度") || strings.Contains(errMsg, "状态只能") {
			response.BadRequest(c, errMsg)
			return
		}

		response.ServerError(c, "更新失败")
		return
	}

	response.Success(c, tenant)
}

// Delete 删除租户
func (h *TenantHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	if err := h.service.Delete(uint(id)); err != nil {
		// 🔧 可以考虑区分是资源不存在还是系统错误
		response.ServerError(c, "删除失败")
		return
	}

	response.Success(c, nil)
}

// Activate 激活租户
func (h *TenantHandler) Activate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	tenant, err := h.service.Activate(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "租户不存在") // 🔧 改为404
			return
		}
		response.ServerError(c, "激活失败")
		return
	}

	response.SuccessWithMessage(c, "租户激活成功", tenant)
}

// Deactivate 停用租户
func (h *TenantHandler) Deactivate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	tenant, err := h.service.Deactivate(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "租户不存在") // 🔧 改为404
			return
		}
		response.ServerError(c, "停用失败")
		return
	}

	response.SuccessWithMessage(c, "租户停用成功", tenant)
}

// ========== 统计相关方法 ==========

// GetStats 获取租户统计
func (h *TenantHandler) GetStats(c *gin.Context) {
	stats, err := h.service.GetStats()
	if err != nil {
		response.ServerError(c, "获取统计失败")
		return
	}

	response.Success(c, stats)
}

// GetRecentlyCreated 获取最近创建的租户
func (h *TenantHandler) GetRecentlyCreated(c *gin.Context) {
	// 解析分页参数
	pageParams := pagination.ParsePageParams(c)

	tenants, total, err := h.service.GetRecentlyCreatedWithPage(pageParams.Page, pageParams.PageSize)
	if err != nil {
		response.ServerError(c, "查询失败")
		return
	}

	// 计算分页信息
	pageInfo := pagination.NewPageInfo(pageParams.Page, pageParams.PageSize, total)
	response.SuccessWithPage(c, tenants, pageInfo)
}

// GetStatusDistribution 获取状态分布
func (h *TenantHandler) GetStatusDistribution(c *gin.Context) {
	distribution, err := h.service.GetStatusDistribution()
	if err != nil {
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, distribution)
}
