package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/pagination"
	"ahop/pkg/response"

	"github.com/gin-gonic/gin"
)

type PermissionHandler struct {
	service *services.PermissionService
}

func NewPermissionHandler(service *services.PermissionService) *PermissionHandler {
	return &PermissionHandler{
		service: service,
	}
}

// GetAll 获取所有权限（支持分页）
func (h *PermissionHandler) GetAll(c *gin.Context) {
	// 解析分页参数
	pageParams := pagination.ParsePageParams(c)

	// 支持按模块筛选
	module := c.Query("module")

	permissions, total, err := h.service.GetWithPage(module, pageParams.Page, pageParams.PageSize)
	if err != nil {
		response.ServerError(c, "查询失败")
		return
	}

	// 计算分页信息
	pageInfo := pagination.NewPageInfo(pageParams.Page, pageParams.PageSize, total)
	response.SuccessWithPage(c, permissions, pageInfo)
}

// GetByModule 根据模块获取权限
func (h *PermissionHandler) GetByModule(c *gin.Context) {
	module := c.Param("module")
	if module == "" {
		response.BadRequest(c, "模块名称不能为空")
		return
	}

	// 使用统一的分页方法，只是不传分页参数
	permissions, _, err := h.service.GetWithPage(module, 1, 1000) // 获取该模块的所有权限
	if err != nil {
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, permissions)
}

