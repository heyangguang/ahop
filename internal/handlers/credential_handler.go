package handlers

import (
	"ahop/internal/models"
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CredentialHandler 凭证处理器
type CredentialHandler struct {
	credentialService *services.CredentialService
	tagService        *services.TagService
}

// NewCredentialHandler 创建凭证处理器实例
func NewCredentialHandler(credentialService *services.CredentialService, tagService *services.TagService) *CredentialHandler {
	return &CredentialHandler{
		credentialService: credentialService,
		tagService:        tagService,
	}
}

// CreateCredentialRequest 创建凭证请求
type CreateCredentialRequest struct {
	Name          string                 `json:"name" binding:"required,max=100"`
	Type          models.CredentialType  `json:"type" binding:"required"`
	Description   string                 `json:"description" binding:"max=500"`
	Username      string                 `json:"username" binding:"max=100"`
	Password      string                 `json:"password"`
	PrivateKey    string                 `json:"private_key"`
	PublicKey     string                 `json:"public_key"`
	APIKey        string                 `json:"api_key"`
	Token         string                 `json:"token"`
	Certificate   string                 `json:"certificate"`
	Passphrase    string                 `json:"passphrase"`
	AllowedHosts  string                 `json:"allowed_hosts"`
	AllowedIPs    string                 `json:"allowed_ips"`
	DeniedHosts   string                 `json:"denied_hosts"`
	DeniedIPs     string                 `json:"denied_ips"`
	ExpiresAt     *time.Time             `json:"expires_at"`
	MaxUsageCount int                    `json:"max_usage_count"`
}

// UpdateCredentialRequest 更新凭证请求
type UpdateCredentialRequest struct {
	Name          *string                `json:"name" binding:"omitempty,max=100"`
	Description   *string                `json:"description" binding:"omitempty,max=500"`
	Username      *string                `json:"username" binding:"omitempty,max=100"`
	Password      *string                `json:"password"`
	PrivateKey    *string                `json:"private_key"`
	PublicKey     *string                `json:"public_key"`
	APIKey        *string                `json:"api_key"`
	Token         *string                `json:"token"`
	Certificate   *string                `json:"certificate"`
	Passphrase    *string                `json:"passphrase"`
	AllowedHosts  *string                `json:"allowed_hosts"`
	AllowedIPs    *string                `json:"allowed_ips"`
	DeniedHosts   *string                `json:"denied_hosts"`
	DeniedIPs     *string                `json:"denied_ips"`
	ExpiresAt     *time.Time             `json:"expires_at"`
	MaxUsageCount *int                   `json:"max_usage_count"`
	IsActive      *bool                  `json:"is_active"`
}

// Create 创建凭证
func (h *CredentialHandler) Create(c *gin.Context) {
	var req CreateCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 获取当前用户信息
	claimsInterface, _ := c.Get("claims")
	claims := claimsInterface.(*jwt.JWTClaims)

	// 创建凭证模型
	credential := &models.Credential{
		TenantID:      claims.CurrentTenantID,
		Name:          req.Name,
		Type:          req.Type,
		Description:   req.Description,
		Username:      req.Username,
		Password:      req.Password,
		PrivateKey:    req.PrivateKey,
		PublicKey:     req.PublicKey,
		APIKey:        req.APIKey,
		Token:         req.Token,
		Certificate:   req.Certificate,
		Passphrase:    req.Passphrase,
		AllowedHosts:  req.AllowedHosts,
		AllowedIPs:    req.AllowedIPs,
		DeniedHosts:   req.DeniedHosts,
		DeniedIPs:     req.DeniedIPs,
		MaxUsageCount: req.MaxUsageCount,
		CreatedBy:     claims.UserID,
		UpdatedBy:     claims.UserID,
	}

	if req.ExpiresAt != nil {
		credential.ExpiresAt = req.ExpiresAt
	}

	// 创建凭证
	if err := h.credentialService.Create(credential); err != nil {
		// 处理验证错误
		errMsg := err.Error()
		if strings.Contains(errMsg, "必须提供") || strings.Contains(errMsg, "不支持的凭证类型") {
			response.BadRequest(c, errMsg)
			return
		}
		response.ServerError(c, "创建凭证失败")
		return
	}

	// 清空敏感字段后返回
	credential.Password = ""
	credential.PrivateKey = ""
	credential.APIKey = ""
	credential.Token = ""
	credential.Certificate = ""
	credential.Passphrase = ""

	response.Success(c, credential)
}

// Update 更新凭证
func (h *CredentialHandler) Update(c *gin.Context) {
	// 获取凭证ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	var req UpdateCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 获取当前用户信息
	claimsInterface, _ := c.Get("claims")
	claims := claimsInterface.(*jwt.JWTClaims)

	// 构建更新map
	updates := make(map[string]interface{})
	
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.Password != nil {
		updates["password"] = *req.Password
	}
	if req.PrivateKey != nil {
		updates["private_key"] = *req.PrivateKey
	}
	if req.PublicKey != nil {
		updates["public_key"] = *req.PublicKey
	}
	if req.APIKey != nil {
		updates["api_key"] = *req.APIKey
	}
	if req.Token != nil {
		updates["token"] = *req.Token
	}
	if req.Certificate != nil {
		updates["certificate"] = *req.Certificate
	}
	if req.Passphrase != nil {
		updates["passphrase"] = *req.Passphrase
	}
	if req.AllowedHosts != nil {
		updates["allowed_hosts"] = *req.AllowedHosts
	}
	if req.AllowedIPs != nil {
		updates["allowed_ips"] = *req.AllowedIPs
	}
	if req.DeniedHosts != nil {
		updates["denied_hosts"] = *req.DeniedHosts
	}
	if req.DeniedIPs != nil {
		updates["denied_ips"] = *req.DeniedIPs
	}
	if req.ExpiresAt != nil {
		updates["expires_at"] = req.ExpiresAt
	}
	if req.MaxUsageCount != nil {
		updates["max_usage_count"] = *req.MaxUsageCount
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	updates["updated_by"] = claims.UserID

	// 更新凭证
	if err := h.credentialService.Update(uint(id), claims.CurrentTenantID, updates); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "凭证不存在")
			return
		}
		response.ServerError(c, "更新凭证失败")
		return
	}

	response.SuccessWithMessage(c, "凭证更新成功", nil)
}

// Get 获取凭证详情（不含敏感信息）
func (h *CredentialHandler) Get(c *gin.Context) {
	// 获取凭证ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 获取当前用户信息
	claimsInterface, _ := c.Get("claims")
	claims := claimsInterface.(*jwt.JWTClaims)

	// 获取凭证
	credential, err := h.credentialService.GetByID(uint(id), claims.CurrentTenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "凭证不存在")
			return
		}
		response.ServerError(c, "查询凭证失败")
		return
	}

	response.Success(c, credential)
}

// GetDecrypted 获取解密的凭证（需要特殊权限）
func (h *CredentialHandler) GetDecrypted(c *gin.Context) {
	// 获取凭证ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 获取用途说明
	purpose := c.Query("purpose")
	if purpose == "" {
		purpose = "API调用获取凭证"
	}

	// 获取当前用户信息
	claimsInterface, _ := c.Get("claims")
	claims := claimsInterface.(*jwt.JWTClaims)

	// 获取解密的凭证
	credential, err := h.credentialService.GetDecrypted(uint(id), claims.CurrentTenantID, &services.OperatorInfo{
		Type: "user",
		UserID: &claims.UserID,
		Info: "api-decrypt",
	}, purpose)
	if err != nil {
		errMsg := err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "凭证不存在")
			return
		}
		if strings.Contains(errMsg, "已禁用") || strings.Contains(errMsg, "已过期") || strings.Contains(errMsg, "次数已达上限") {
			response.BadRequest(c, errMsg)
			return
		}
		response.ServerError(c, "获取凭证失败")
		return
	}

	// 构建包含敏感信息的响应
	decryptedData := gin.H{
		"id":            credential.ID,
		"created_at":    credential.CreatedAt,
		"updated_at":    credential.UpdatedAt,
		"tenant_id":     credential.TenantID,
		"name":          credential.Name,
		"type":          credential.Type,
		"description":   credential.Description,
		"username":      credential.Username,
		"allowed_hosts": credential.AllowedHosts,
		"allowed_ips":   credential.AllowedIPs,
		"denied_hosts":  credential.DeniedHosts,
		"denied_ips":    credential.DeniedIPs,
		"expires_at":    credential.ExpiresAt,
		"is_active":     credential.IsActive,
	}

	// 根据类型添加敏感字段
	switch credential.Type {
	case models.CredentialTypePassword:
		decryptedData["password"] = credential.Password
	case models.CredentialTypeSSHKey:
		decryptedData["private_key"] = credential.PrivateKey
		decryptedData["public_key"] = credential.PublicKey
		if credential.Passphrase != "" {
			decryptedData["passphrase"] = credential.Passphrase
		}
	case models.CredentialTypeAPIKey:
		decryptedData["api_key"] = credential.APIKey
	case models.CredentialTypeToken:
		decryptedData["token"] = credential.Token
	case models.CredentialTypeCertificate:
		decryptedData["certificate"] = credential.Certificate
		if credential.Passphrase != "" {
			decryptedData["passphrase"] = credential.Passphrase
		}
	}

	// 设置安全响应头
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	
	response.Success(c, decryptedData)
}

// List 获取凭证列表
func (h *CredentialHandler) List(c *gin.Context) {
	// 解析分页参数
	params := pagination.ParsePageParams(c)

	// 获取当前用户信息
	claimsInterface, _ := c.Get("claims")
	claims := claimsInterface.(*jwt.JWTClaims)

	// 构建过滤条件
	filters := make(map[string]interface{})
	if credType := c.Query("type"); credType != "" {
		filters["type"] = credType
	}
	if name := c.Query("name"); name != "" {
		filters["name"] = name
	}
	if isActive := c.Query("is_active"); isActive != "" {
		if isActive == "true" {
			filters["is_active"] = true
		} else if isActive == "false" {
			filters["is_active"] = false
		}
	}

	// 获取凭证列表
	credentials, total, err := h.credentialService.List(claims.CurrentTenantID, params.Page, params.PageSize, filters)
	if err != nil {
		response.ServerError(c, "查询凭证列表失败")
		return
	}

	// 创建分页信息
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)

	response.SuccessWithPage(c, credentials, pageInfo)
}

// Delete 删除凭证
func (h *CredentialHandler) Delete(c *gin.Context) {
	// 获取凭证ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 获取当前用户信息
	claimsInterface, _ := c.Get("claims")
	claims := claimsInterface.(*jwt.JWTClaims)

	// 删除凭证
	if err := h.credentialService.Delete(uint(id), claims.CurrentTenantID); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "不存在") {
			response.NotFound(c, "凭证不存在")
			return
		}
		// 返回具体的错误信息
		response.ServerError(c, errMsg)
		return
	}

	response.SuccessWithMessage(c, "凭证删除成功", nil)
}

// GetUsageLogs 获取凭证使用日志
func (h *CredentialHandler) GetUsageLogs(c *gin.Context) {
	// 获取凭证ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 解析分页参数
	params := pagination.ParsePageParams(c)

	// 获取当前用户信息
	claimsInterface, _ := c.Get("claims")
	claims := claimsInterface.(*jwt.JWTClaims)

	// 获取使用日志
	logs, total, err := h.credentialService.GetUsageLogs(uint(id), claims.CurrentTenantID, params.Page, params.PageSize)
	if err != nil {
		response.ServerError(c, "查询使用日志失败")
		return
	}

	// 创建分页信息
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)

	response.SuccessWithPage(c, logs, pageInfo)
}

// UpdateTagsRequest 更新标签请求
type UpdateTagsRequest struct {
	TagIDs []uint `json:"tag_ids" binding:"required"`
}

// GetTags 获取凭证的标签
func (h *CredentialHandler) GetTags(c *gin.Context) {
	// 获取凭证ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 获取标签列表
	tags, err := h.tagService.GetByCredential(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "凭证不存在")
			return
		}
		response.ServerError(c, "获取标签失败")
		return
	}

	response.Success(c, tags)
}

// UpdateTags 更新凭证的标签
func (h *CredentialHandler) UpdateTags(c *gin.Context) {
	// 获取凭证ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	// 获取当前用户信息
	claimsInterface, _ := c.Get("claims")
	claims := claimsInterface.(*jwt.JWTClaims)

	// 验证凭证是否存在且属于当前租户
	credential, err := h.credentialService.GetByID(uint(id), claims.CurrentTenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "凭证不存在")
			return
		}
		response.ServerError(c, "查询凭证失败")
		return
	}

	// 解析请求
	var req UpdateTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 更新标签
	if err := h.tagService.UpdateCredentialTags(credential.ID, req.TagIDs); err != nil {
		if err.Error() == "部分标签不存在或不属于当前租户" {
			response.BadRequest(c, err.Error())
			return
		}
		response.ServerError(c, "更新标签失败")
		return
	}

	// 重新获取凭证（包含标签）
	credential, err = h.credentialService.GetByID(uint(id), claims.CurrentTenantID)
	if err != nil {
		response.ServerError(c, "获取凭证信息失败")
		return
	}

	response.Success(c, credential)
}