package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

// FieldMappingHandler 字段映射处理器
type FieldMappingHandler struct {
	fieldMappingService *services.FieldMappingService
}

// NewFieldMappingHandler 创建字段映射处理器
func NewFieldMappingHandler(fieldMappingService *services.FieldMappingService) *FieldMappingHandler {
	return &FieldMappingHandler{
		fieldMappingService: fieldMappingService,
	}
}

// GetByPlugin 获取插件的字段映射
func (h *FieldMappingHandler) GetByPlugin(c *gin.Context) {
	// 获取插件ID
	pluginID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 获取字段映射
	mappings, err := h.fieldMappingService.GetByPlugin(claims.CurrentTenantID, uint(pluginID))
	if err != nil {
		response.ServerError(c, "获取字段映射失败")
		return
	}

	response.Success(c, mappings)
}


// UpdateMappings 更新字段映射（统一接口）
func (h *FieldMappingHandler) UpdateMappings(c *gin.Context) {
	// 获取插件ID
	pluginID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的插件ID")
		return
	}

	// 解析请求
	var req services.UpdateFieldMappingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: " + err.Error())
		return
	}

	// 获取用户信息
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 更新映射
	mappings, err := h.fieldMappingService.UpdateMappings(claims.CurrentTenantID, uint(pluginID), req)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, mappings)
}