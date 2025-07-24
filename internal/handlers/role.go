package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CreateRoleRequest struct {
	TenantID    uint   `json:"tenant_id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type AssignPermissionsRequest struct {
	PermissionIDs []uint `json:"permission_ids"`
}

type RoleHandler struct {
	service *services.RoleService
}

func NewRoleHandler(service *services.RoleService) *RoleHandler {
	return &RoleHandler{
		service: service,
	}
}

// ========== 基础CRUD方法 ==========

// Create 创建角色
func (h *RoleHandler) Create(c *gin.Context) {
	var req CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	role, err := h.service.Create(req.TenantID, req.Code, req.Name, req.Description)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "角色代码长度") || strings.Contains(errMsg, "角色名称长度") {
			response.BadRequest(c, errMsg)
			return
		}
		if errMsg == "角色代码已存在" {
			response.BadRequest(c, errMsg)
			return
		}
		response.ServerError(c, "创建失败")
		return
	}

	response.Success(c, role)
}

// GetByID 获取角色
func (h *RoleHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	role, err := h.service.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "角色不存在")
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, role)
}

// GetByTenant 根据租户获取角色列表（支持分页）
func (h *RoleHandler) GetByTenant(c *gin.Context) {
	tenantIDStr := c.Param("tenant_id")
	tenantID, err := strconv.ParseUint(tenantIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "租户ID格式错误")
		return
	}

	// 解析分页参数
	pageParams := pagination.ParsePageParams(c)

	// 支持按状态筛选
	status := c.Query("status")

	roles, total, err := h.service.GetByTenantWithPage(uint(tenantID), status, pageParams.Page, pageParams.PageSize)
	if err != nil {
		response.ServerError(c, "查询失败")
		return
	}

	// 计算分页信息
	pageInfo := pagination.NewPageInfo(pageParams.Page, pageParams.PageSize, total)
	response.SuccessWithPage(c, roles, pageInfo)
}

// Update 更新角色
func (h *RoleHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	role, err := h.service.Update(uint(id), req.Name, req.Description, req.Status)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "角色不存在")
			return
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "角色名称长度") || strings.Contains(errMsg, "状态只能") || errMsg == "系统角色不允许修改" {
			response.BadRequest(c, errMsg)
			return
		}

		response.ServerError(c, "更新失败")
		return
	}

	response.Success(c, role)
}

// Delete 删除角色
func (h *RoleHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	if err := h.service.Delete(uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "角色不存在")
			return
		}
		if err.Error() == "系统角色不允许删除" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "删除失败")
		return
	}

	response.Success(c, nil)
}

// ========== 权限管理方法 ==========

// AssignPermissions 为角色分配权限
func (h *RoleHandler) AssignPermissions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	var req AssignPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	err = h.service.AssignPermissions(uint(id), req.PermissionIDs)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "角色不存在")
			return
		}
		response.ServerError(c, "分配权限失败")
		return
	}

	response.Success(c, "权限分配成功")
}

// GetPermissions 获取角色的权限
func (h *RoleHandler) GetPermissions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	permissions, err := h.service.GetRolePermissions(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "角色不存在")
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, permissions)
}
