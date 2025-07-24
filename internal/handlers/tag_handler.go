package handlers

import (
	"errors"
	"strconv"

	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TagHandler struct {
	service *services.TagService
}

func NewTagHandler(service *services.TagService) *TagHandler {
	return &TagHandler{
		service: service,
	}
}

// CreateTagRequest 创建标签请求
type CreateTagRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value" binding:"required"`
	Color string `json:"color"`
}

// UpdateTagRequest 更新标签请求
type UpdateTagRequest struct {
	Color string `json:"color" binding:"required"`
}

// Create 创建标签
func (h *TagHandler) Create(c *gin.Context) {
	// 获取当前用户的租户ID
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	tenantID := userClaims.CurrentTenantID

	var req CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	tag, err := h.service.Create(tenantID, req.Key, req.Value, req.Color)
	if err != nil {
		if err.Error() == "标签已存在" {
			response.BadRequest(c, err.Error())
			return
		}
		if err.Error() == "标签键长度必须在1-50个字符之间" ||
			err.Error() == "标签值长度必须在1-100个字符之间" ||
			err.Error() == "标签键只能包含字母、数字、中文、下划线和连字符" ||
			err.Error() == "标签值只能包含字母、数字、中文、下划线和连字符" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "创建标签失败")
		return
	}

	response.Success(c, tag)
}

// GetByID 获取标签详情
func (h *TagHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	tag, err := h.service.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "标签不存在")
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	// 验证标签属于当前租户
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	if tag.TenantID != userClaims.CurrentTenantID && !userClaims.IsPlatformAdmin {
		response.Forbidden(c, "无权访问此标签")
		return
	}

	response.Success(c, tag)
}

// GetAll 获取标签列表
func (h *TagHandler) GetAll(c *gin.Context) {
	// 获取当前用户的租户ID
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	tenantID := userClaims.CurrentTenantID

	// 支持按key过滤
	key := c.Query("key")

	tags, err := h.service.GetByTenant(tenantID, key)
	if err != nil {
		response.ServerError(c, "查询失败")
		return
	}

	// 使用分页响应格式保持一致性
	pageParams := pagination.ParsePageParams(c)
	pageInfo := pagination.NewPageInfo(pageParams.Page, pageParams.PageSize, int64(len(tags)))
	response.SuccessWithPage(c, tags, pageInfo)
}

// GetGroupedByKey 按key分组获取标签
func (h *TagHandler) GetGroupedByKey(c *gin.Context) {
	// 获取当前用户的租户ID
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	tenantID := userClaims.CurrentTenantID

	grouped, err := h.service.GetGroupedByKey(tenantID)
	if err != nil {
		response.ServerError(c, "查询失败")
		return
	}

	response.Success(c, grouped)
}

// Update 更新标签
func (h *TagHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 先获取标签验证权限
	tag, err := h.service.GetByID(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "标签不存在")
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	// 验证标签属于当前租户
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	if tag.TenantID != userClaims.CurrentTenantID && !userClaims.IsPlatformAdmin {
		response.Forbidden(c, "无权更新此标签")
		return
	}

	var req UpdateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	updatedTag, err := h.service.Update(uint(id), req.Color)
	if err != nil {
		response.ServerError(c, "更新失败")
		return
	}

	response.Success(c, updatedTag)
}

// Delete 删除标签
func (h *TagHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 先获取标签验证权限
	tag, err := h.service.GetByID(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "标签不存在")
			return
		}
		response.ServerError(c, "查询失败")
		return
	}

	// 验证标签属于当前租户
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	if tag.TenantID != userClaims.CurrentTenantID && !userClaims.IsPlatformAdmin {
		response.Forbidden(c, "无权删除此标签")
		return
	}

	if err := h.service.Delete(uint(id)); err != nil {
		if err.Error() == "标签正在使用中，无法删除" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "删除失败")
		return
	}

	response.Success(c, nil)
}
