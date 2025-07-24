package handlers

import (
	"strconv"
	
	"ahop/internal/models"
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	
	"github.com/gin-gonic/gin"
)

// HostGroupHandler 主机组处理器
type HostGroupHandler struct {
	hostGroupService *services.HostGroupService
	hostService      *services.HostService
}

// NewHostGroupHandler 创建主机组处理器
func NewHostGroupHandler(hostGroupService *services.HostGroupService, hostService *services.HostService) *HostGroupHandler {
	return &HostGroupHandler{
		hostGroupService: hostGroupService,
		hostService:      hostService,
	}
}

// Create 创建主机组
func (h *HostGroupHandler) Create(c *gin.Context) {
	var req models.CreateHostGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	group, err := h.hostGroupService.Create(userClaims.CurrentTenantID, &req)
	if err != nil {
		// 检查是否是业务逻辑错误
		if err.Error() == "同级已存在相同名称或代码的主机组" || 
		   err.Error() == "父组不存在" ||
		   err.Error() == "父组不属于当前租户" ||
		   err.Error() == "父组已包含主机，不能添加子组" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "创建主机组失败: "+err.Error())
		return
	}
	
	response.Success(c, group)
}

// Update 更新主机组
func (h *HostGroupHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	
	var req models.UpdateHostGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	if err := h.hostGroupService.Update(uint(id), userClaims.CurrentTenantID, &req); err != nil {
		// 检查是否是记录不存在
		if err.Error() == "record not found" {
			response.NotFound(c, "主机组不存在")
			return
		}
		// 检查是否是业务逻辑错误
		if err.Error() == "同级已存在相同名称的主机组" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "更新主机组失败: "+err.Error())
		return
	}
	
	response.Success(c, nil)
}

// Delete 删除主机组
func (h *HostGroupHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	
	// 获取force参数
	force := c.Query("force") == "true"
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	if err := h.hostGroupService.Delete(uint(id), userClaims.CurrentTenantID, force); err != nil {
		// 检查是否是记录不存在
		if err.Error() == "record not found" {
			response.NotFound(c, "主机组不存在")
			return
		}
		// 检查是否是业务逻辑错误
		if err.Error() == "该组包含子组，不能删除" || 
		   err.Error() == "该组包含主机，不能删除" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "删除主机组失败: "+err.Error())
		return
	}
	
	response.Success(c, nil)
}

// GetByID 获取主机组详情
func (h *HostGroupHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	group, err := h.hostGroupService.GetByID(uint(id), userClaims.CurrentTenantID)
	if err != nil {
		response.NotFound(c, "主机组不存在")
		return
	}
	
	response.Success(c, group)
}

// List 获取主机组列表（平铺）
func (h *HostGroupHandler) List(c *gin.Context) {
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	// 解析查询参数
	search := c.Query("search")
	parentIDStr := c.Query("parent_id")
	
	var parentID *uint
	if parentIDStr != "" {
		if id, err := strconv.ParseUint(parentIDStr, 10, 32); err == nil {
			pid := uint(id)
			parentID = &pid
		}
	}
	
	groups, err := h.hostGroupService.List(userClaims.CurrentTenantID, parentID, search)
	if err != nil {
		response.ServerError(c, "获取主机组列表失败: "+err.Error())
		return
	}
	
	// 转换为响应格式
	var result []models.HostGroupListResponse
	for _, g := range groups {
		result = append(result, models.HostGroupListResponse{
			ID:         g.ID,
			ParentID:   g.ParentID,
			Name:       g.Name,
			Code:       g.Code,
			Path:       g.Path,
			Level:      g.Level,
			Type:       g.Type,
			Status:     g.Status,
			IsLeaf:     g.IsLeaf,
			HostCount:  g.HostCount,
			ChildCount: g.ChildCount,
			CreatedAt:  g.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:  g.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	
	response.Success(c, result)
}

// GetTree 获取主机组树
func (h *HostGroupHandler) GetTree(c *gin.Context) {
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	// 解析根节点ID
	rootIDStr := c.Query("root_id")
	var rootID *uint
	if rootIDStr != "" {
		if id, err := strconv.ParseUint(rootIDStr, 10, 32); err == nil {
			rid := uint(id)
			rootID = &rid
		}
	}
	
	tree, err := h.hostGroupService.GetTree(userClaims.CurrentTenantID, rootID)
	if err != nil {
		response.ServerError(c, "获取主机组树失败: "+err.Error())
		return
	}
	
	response.Success(c, tree)
}

// GetSubTree 获取子树
func (h *HostGroupHandler) GetSubTree(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	rootID := uint(id)
	tree, err := h.hostGroupService.GetTree(userClaims.CurrentTenantID, &rootID)
	if err != nil {
		response.ServerError(c, "获取子树失败: "+err.Error())
		return
	}
	
	response.Success(c, tree)
}

// Move 移动主机组
func (h *HostGroupHandler) Move(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	
	var req models.MoveHostGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	if err := h.hostGroupService.Move(uint(id), userClaims.CurrentTenantID, req.NewParentID); err != nil {
		// 检查是否是业务逻辑错误
		if err.Error() == "不能将组移动到自己" ||
		   err.Error() == "不能将组移动到自己的子孙节点" ||
		   err.Error() == "目标父组不存在" ||
		   err.Error() == "目标父组不属于当前租户" ||
		   err.Error() == "目标父组已包含主机，不能添加子组" ||
		   err.Error() == "目标位置已存在相同名称或代码的主机组" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "移动主机组失败: "+err.Error())
		return
	}
	
	response.Success(c, nil)
}

// GetHosts 获取组内主机
func (h *HostGroupHandler) GetHosts(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	hosts, err := h.hostGroupService.GetHostsByGroup(uint(id), userClaims.CurrentTenantID)
	if err != nil {
		response.ServerError(c, "获取组内主机失败: "+err.Error())
		return
	}
	
	response.Success(c, hosts)
}

// AssignHosts 批量分配主机到组
func (h *HostGroupHandler) AssignHosts(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	
	var req models.AssignHostsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	if err := h.hostGroupService.AssignHosts(uint(id), userClaims.CurrentTenantID, req.HostIDs); err != nil {
		// 检查是否是业务逻辑错误
		if err.Error() == "只能将主机分配到叶子节点" ||
		   err.Error() == "部分主机不存在或不属于当前租户" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "分配主机失败: "+err.Error())
		return
	}
	
	response.Success(c, nil)
}

// RemoveHosts 批量移除主机
func (h *HostGroupHandler) RemoveHosts(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	
	var req models.AssignHostsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	if err := h.hostGroupService.RemoveHosts(uint(id), userClaims.CurrentTenantID, req.HostIDs); err != nil {
		response.ServerError(c, "移除主机失败: "+err.Error())
		return
	}
	
	response.Success(c, nil)
}

// UpdateHostsGroup 更新组内所有主机（全量替换）
func (h *HostGroupHandler) UpdateHostsGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	
	var req models.AssignHostsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	// 先获取当前组内的所有主机
	currentHosts, err := h.hostGroupService.GetHostsByGroup(uint(id), userClaims.CurrentTenantID)
	if err != nil {
		response.ServerError(c, "获取当前主机失败: "+err.Error())
		return
	}
	
	// 提取当前主机ID列表
	currentHostIDs := make([]uint, len(currentHosts))
	for i, host := range currentHosts {
		currentHostIDs[i] = host.ID
	}
	
	// 先移除所有现有主机
	if len(currentHostIDs) > 0 {
		if err := h.hostGroupService.RemoveHosts(uint(id), userClaims.CurrentTenantID, currentHostIDs); err != nil {
			response.ServerError(c, "移除现有主机失败: "+err.Error())
			return
		}
	}
	
	// 再添加新的主机
	if len(req.HostIDs) > 0 {
		if err := h.hostGroupService.AssignHosts(uint(id), userClaims.CurrentTenantID, req.HostIDs); err != nil {
			response.ServerError(c, "分配新主机失败: "+err.Error())
			return
		}
	}
	
	response.Success(c, nil)
}

// GetUngroupedHosts 获取未分组的主机
func (h *HostGroupHandler) GetUngroupedHosts(c *gin.Context) {
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	// 解析分页参数
	params := pagination.ParsePageParams(c)
	
	hosts, err := h.hostGroupService.GetUngroupedHosts(userClaims.CurrentTenantID)
	if err != nil {
		response.ServerError(c, "获取未分组主机失败: "+err.Error())
		return
	}
	
	// 手动分页
	total := len(hosts)
	start := params.GetOffset()
	end := start + params.GetLimit()
	
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	
	paginatedHosts := hosts[start:end]
	
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, int64(total))
	response.SuccessWithPage(c, paginatedHosts, pageInfo)
}

// UpdateHostGroup 更新主机所属组
func (h *HostGroupHandler) UpdateHostGroup(c *gin.Context) {
	hostID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的主机ID")
		return
	}
	
	var req struct {
		GroupID *uint `json:"group_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	if err := h.hostGroupService.UpdateHostGroup(userClaims.CurrentTenantID, uint(hostID), req.GroupID); err != nil {
		response.ServerError(c, "更新主机组失败: "+err.Error())
		return
	}
	
	response.Success(c, nil)
}

// GetHostGroups 获取主机所属组
func (h *HostGroupHandler) GetHostGroups(c *gin.Context) {
	hostID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的主机ID")
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	// 获取主机信息
	host, err := h.hostService.GetByID(uint(hostID), userClaims.CurrentTenantID)
	if err != nil {
		response.NotFound(c, "主机不存在")
		return
	}
	
	// 如果主机有组，获取组信息
	if host.HostGroupID != nil {
		group, err := h.hostGroupService.GetByID(*host.HostGroupID, userClaims.CurrentTenantID)
		if err != nil {
			response.ServerError(c, "获取主机组失败: "+err.Error())
			return
		}
		response.Success(c, group)
	} else {
		response.Success(c, nil)
	}
}

// GetByPath 根据路径获取组
func (h *HostGroupHandler) GetByPath(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		response.BadRequest(c, "路径参数不能为空")
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	group, err := h.hostGroupService.GetByPath(userClaims.CurrentTenantID, path)
	if err != nil {
		response.NotFound(c, "主机组不存在")
		return
	}
	
	response.Success(c, group)
}

// GetAncestors 获取祖先节点
func (h *HostGroupHandler) GetAncestors(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	ancestors, err := h.hostGroupService.GetAncestors(uint(id), userClaims.CurrentTenantID)
	if err != nil {
		response.ServerError(c, "获取祖先节点失败: "+err.Error())
		return
	}
	
	response.Success(c, ancestors)
}

// GetDescendants 获取后代节点
func (h *HostGroupHandler) GetDescendants(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	
	claims, _ := c.Get("claims")
	userClaims := claims.(*jwt.JWTClaims)
	
	descendants, err := h.hostGroupService.GetDescendants(uint(id), userClaims.CurrentTenantID)
	if err != nil {
		response.ServerError(c, "获取后代节点失败: "+err.Error())
		return
	}
	
	response.Success(c, descendants)
}