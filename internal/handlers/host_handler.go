package handlers

import (
	"ahop/internal/models"
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

// HostHandler 主机处理器
type HostHandler struct {
	hostService *services.HostService
}

// NewHostHandler 创建主机处理器实例
func NewHostHandler(hostService *services.HostService) *HostHandler {
	return &HostHandler{
		hostService: hostService,
	}
}

// Create 创建主机
func (h *HostHandler) Create(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	var req struct {
		Name         string `json:"name" binding:"required,min=1,max=100"`
		IPAddress    string `json:"ip_address" binding:"required,ip"`
		Port         int    `json:"port" binding:"min=1,max=65535"`
		CredentialID uint   `json:"credential_id" binding:"required"`
		Description  string `json:"description" binding:"max=500"`
		Tags         []uint `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 如果没有指定端口，使用默认值
	if req.Port == 0 {
		req.Port = 22
	}

	// 创建主机
	host := &models.Host{
		TenantID:     claims.CurrentTenantID,
		Name:         req.Name,
		IPAddress:    req.IPAddress,
		Port:         req.Port,
		CredentialID: req.CredentialID,
		Description:  req.Description,
		IsActive:     true,
		CreatedBy:    claims.UserID,
		UpdatedBy:    claims.UserID,
	}

	if err := h.hostService.Create(host); err != nil {
		response.ServerError(c, err.Error())
		return
	}

	// 更新标签
	if len(req.Tags) > 0 {
		if err := h.hostService.UpdateTags(host.ID, claims.CurrentTenantID, req.Tags); err != nil {
			// 标签更新失败不影响主机创建
			response.SuccessWithMessage(c, "主机创建成功，但标签更新失败", host)
			return
		}
	}

	// 重新获取包含标签的主机信息
	host, _ = h.hostService.GetByID(host.ID, claims.CurrentTenantID)
	response.Success(c, host)
}

// GetByID 获取主机详情
func (h *HostHandler) GetByID(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的主机ID")
		return
	}

	host, err := h.hostService.GetByID(uint(id), claims.CurrentTenantID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, host)
}

// Update 更新主机
func (h *HostHandler) Update(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的主机ID")
		return
	}

	var req struct {
		Name         *string `json:"name" binding:"omitempty,min=1,max=100"`
		IPAddress    *string `json:"ip_address" binding:"omitempty,ip"`
		Port         *int    `json:"port" binding:"omitempty,min=1,max=65535"`
		CredentialID *uint   `json:"credential_id"`
		Description  *string `json:"description" binding:"omitempty,max=500"`
		IsActive     *bool   `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 构建更新映射
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.IPAddress != nil {
		updates["ip_address"] = *req.IPAddress
	}
	if req.Port != nil {
		updates["port"] = *req.Port
	}
	if req.CredentialID != nil {
		updates["credential_id"] = *req.CredentialID
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		response.BadRequest(c, "没有需要更新的字段")
		return
	}

	updates["updated_by"] = claims.UserID

	if err := h.hostService.Update(uint(id), claims.CurrentTenantID, updates); err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "更新成功", nil)
}

// Delete 删除主机
func (h *HostHandler) Delete(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的主机ID")
		return
	}

	if err := h.hostService.Delete(uint(id), claims.CurrentTenantID); err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}

// List 获取主机列表
func (h *HostHandler) List(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 解析分页参数
	params := pagination.ParsePageParams(c)

	// 构建过滤条件
	filters := make(map[string]interface{})
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if name := c.Query("name"); name != "" {
		filters["name"] = name
	}
	if ipAddress := c.Query("ip_address"); ipAddress != "" {
		filters["ip_address"] = ipAddress
	}
	if osType := c.Query("os_type"); osType != "" {
		filters["os_type"] = osType
	}
	if isActive := c.Query("is_active"); isActive != "" {
		active, _ := strconv.ParseBool(isActive)
		filters["is_active"] = active
	}

	// 获取数据
	hosts, total, err := h.hostService.List(
		claims.CurrentTenantID,
		params.Page,
		params.PageSize,
		filters,
	)

	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	// 构建分页信息
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, hosts, pageInfo)
}

// GetTags 获取主机标签
func (h *HostHandler) GetTags(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的主机ID")
		return
	}

	tags, err := h.hostService.GetTags(uint(id), claims.CurrentTenantID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, tags)
}

// UpdateTags 更新主机标签
func (h *HostHandler) UpdateTags(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的主机ID")
		return
	}

	var req struct {
		TagIDs []uint `json:"tag_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.hostService.UpdateTags(uint(id), claims.CurrentTenantID, req.TagIDs); err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "标签更新成功", nil)
}

// TestConnection 测试主机连接
func (h *HostHandler) TestConnection(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的主机ID")
		return
	}

	result, err := h.hostService.TestConnection(uint(id), claims.CurrentTenantID, claims.UserID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	if result.Success {
		response.SuccessWithMessage(c, "连接测试成功", result)
	} else {
		response.SuccessWithMessage(c, "连接测试失败", result)
	}
}

// TestPing 测试主机网络连通性
func (h *HostHandler) TestPing(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的主机ID")
		return
	}

	result, err := h.hostService.TestPing(uint(id), claims.CurrentTenantID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	if result.Success {
		response.SuccessWithMessage(c, "网络连通性测试成功", result)
	} else {
		response.SuccessWithMessage(c, "网络连通性测试失败", result)
	}
}

// DownloadTemplate 下载CSV导入模板
func (h *HostHandler) DownloadTemplate(c *gin.Context) {
	// CSV模板内容
	csvContent := `ip,port,credential_id,hostname,ssh_user,description,host_group_id,tags
192.168.1.10,22,1,web-server-01,root,生产环境Web服务器,1,"1,2"
192.168.1.20,22,1,db-server-01,ubuntu,数据库服务器,1,"2,3"
192.168.1.30,22,2,app-server-01,centos,测试环境应用服务器,,"1"`

	// 设置响应头
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=host_import_template.csv")
	
	// 写入BOM以支持Excel正确识别UTF-8
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	
	// 写入CSV内容
	c.String(200, csvContent)
}

// Import 批量导入主机
func (h *HostHandler) Import(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	var req struct {
		Hosts []services.BatchImportHost `json:"hosts" binding:"required,min=1,max=5000"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 执行批量导入
	result, err := h.hostService.BatchImport(claims.CurrentTenantID, claims.UserID, req.Hosts)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "批量导入完成", result)
}

// Export 批量导出主机
func (h *HostHandler) Export(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 获取查询参数（用于筛选导出的主机）
	filters := make(map[string]interface{})
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if name := c.Query("name"); name != "" {
		filters["name"] = name
	}
	if osType := c.Query("os_type"); osType != "" {
		filters["os_type"] = osType
	}
	if isActive := c.Query("is_active"); isActive != "" {
		active, _ := strconv.ParseBool(isActive)
		filters["is_active"] = active
	}

	// 导出主机数据为CSV
	csvContent, err := h.hostService.ExportToCSV(claims.CurrentTenantID, filters)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	// 设置响应头
	filename := "hosts_export.csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	
	// 写入BOM以支持Excel正确识别UTF-8
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	
	// 写入CSV内容
	c.String(200, csvContent)
}
