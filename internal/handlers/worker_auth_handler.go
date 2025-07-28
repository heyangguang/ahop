package handlers

import (
	"ahop/internal/services"
	"ahop/pkg/response"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// WorkerAuthHandler Worker认证处理器
type WorkerAuthHandler struct {
	workerAuthService *services.WorkerAuthService
}

// NewWorkerAuthHandler 创建Worker认证处理器
func NewWorkerAuthHandler(workerAuthService *services.WorkerAuthService) *WorkerAuthHandler {
	return &WorkerAuthHandler{
		workerAuthService: workerAuthService,
	}
}

// AuthRequest Worker认证请求
type AuthRequest struct {
	AccessKey string `json:"access_key" binding:"required"`
	WorkerID  string `json:"worker_id" binding:"required"`
	Timestamp int64  `json:"timestamp" binding:"required"`
	Signature string `json:"signature" binding:"required"`
}

// AuthResponse Worker认证响应
type AuthResponse struct {
	Success        bool                   `json:"success"`
	DatabaseConfig map[string]interface{} `json:"database_config,omitempty"`
	Message        string                 `json:"message,omitempty"`
}

// CreateWorkerAuthRequest 创建Worker授权请求
type CreateWorkerAuthRequest struct {
	Environment string `json:"environment"`
	Description string `json:"description"`
}

// CreateWorkerAuthResponse 创建Worker授权响应
type CreateWorkerAuthResponse struct {
	ID          uint   `json:"id"`
	AccessKey   string `json:"access_key"`
	SecretKey   string `json:"secret_key"`
	Environment string `json:"environment"`
	Description string `json:"description"`
}

// Authenticate Worker认证接口
func (h *WorkerAuthHandler) Authenticate(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 1. 验证AccessKey
	auth, err := h.workerAuthService.ValidateAccessKey(req.AccessKey)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	// 2. 验证时间戳（防重放攻击）
	if !h.workerAuthService.IsValidTimestamp(req.Timestamp) {
		response.Unauthorized(c, "请求已过期")
		return
	}

	// 3. 验证签名
	if !h.workerAuthService.VerifySignature(req.AccessKey, req.WorkerID, req.Timestamp, req.Signature, auth.SecretKey) {
		response.Unauthorized(c, "签名验证失败")
		return
	}

	// 4. 检查Worker ID唯一性
	if err := h.workerAuthService.RegisterWorkerConnection(req.WorkerID, req.AccessKey, c.ClientIP()); err != nil {
		response.Conflict(c, err.Error())
		return
	}

	// 5. 记录认证日志
	clientIP := c.ClientIP()
	h.workerAuthService.LogWorkerAuth(req.WorkerID, req.AccessKey, "success", clientIP)

	// 6. 返回数据库和Redis配置
	response.Success(c, map[string]interface{}{
		"database_config": auth.GetDatabaseConfig(),
		"redis_config":    auth.GetRedisConfig(),
	})
}

// CreateWorkerAuth 创建Worker授权（管理员接口）
func (h *WorkerAuthHandler) CreateWorkerAuth(c *gin.Context) {
	var req CreateWorkerAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 创建授权
	auth, err := h.workerAuthService.CreateWorkerAuth(req.Environment, req.Description)
	if err != nil {
		response.ServerError(c, "创建Worker授权失败: "+err.Error())
		return
	}

	// 返回创建的授权信息（包含SecretKey，只有这一次会返回）
	response.Success(c, CreateWorkerAuthResponse{
		ID:          auth.ID,
		AccessKey:   auth.AccessKey,
		SecretKey:   auth.SecretKey,
		Environment: auth.Environment,
		Description: auth.Description,
	})
}

// ListWorkerAuths 获取Worker授权列表（管理员接口）
func (h *WorkerAuthHandler) ListWorkerAuths(c *gin.Context) {
	// 解析分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	// 获取授权列表
	auths, total, err := h.workerAuthService.GetWorkerAuthList(page, pageSize)
	if err != nil {
		response.ServerError(c, "获取授权列表失败: "+err.Error())
		return
	}

	// 构建分页响应
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    auths,
		"pagination": gin.H{
			"page":       page,
			"page_size":  pageSize,
			"total":      total,
			"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// Heartbeat Worker心跳接口
func (h *WorkerAuthHandler) Heartbeat(c *gin.Context) {
	var req struct {
		WorkerID  string `json:"worker_id" binding:"required"`
		AccessKey string `json:"-"`
		Timestamp int64  `json:"-"`
		Signature string `json:"-"`
	}
	
	// 解析请求体
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	
	// 从头部获取认证信息
	req.AccessKey = c.GetHeader("X-Access-Key")
	timestampStr := c.GetHeader("X-Timestamp")
	req.Signature = c.GetHeader("X-Signature")
	
	if req.AccessKey == "" || timestampStr == "" || req.Signature == "" {
		response.Unauthorized(c, "缺少认证信息")
		return
	}
	
	// 解析时间戳
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		response.Unauthorized(c, "无效的时间戳")
		return
	}
	req.Timestamp = timestamp
	
	// 1. 验证AccessKey
	auth, err := h.workerAuthService.ValidateAccessKey(req.AccessKey)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	
	// 2. 验证时间戳（防重放攻击）
	if !h.workerAuthService.IsValidTimestamp(req.Timestamp) {
		response.Unauthorized(c, "请求已过期")
		return
	}
	
	// 3. 验证签名
	if !h.workerAuthService.VerifySignature(req.AccessKey, "/api/v1/worker/heartbeat", req.Timestamp, req.Signature, auth.SecretKey) {
		response.Unauthorized(c, "签名验证失败")
		return
	}
	
	// 4. 更新心跳
	if err := h.workerAuthService.UpdateWorkerHeartbeat(req.WorkerID); err != nil {
		response.ServerError(c, "更新心跳失败: "+err.Error())
		return
	}
	
	response.Success(c, gin.H{
		"worker_id": req.WorkerID,
		"timestamp": time.Now().Unix(),
	})
}

// DisconnectRequest Worker断开连接请求
type DisconnectRequest struct {
	WorkerID string `json:"worker_id" binding:"required"`
}

// Disconnect Worker主动断开连接
func (h *WorkerAuthHandler) Disconnect(c *gin.Context) {
	var req DisconnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 从头部获取认证信息
	accessKey := c.GetHeader("X-Access-Key")
	timestampStr := c.GetHeader("X-Timestamp")
	signature := c.GetHeader("X-Signature")
	
	if accessKey == "" || timestampStr == "" || signature == "" {
		response.Unauthorized(c, "缺少认证信息")
		return
	}
	
	// 解析时间戳
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		response.Unauthorized(c, "无效的时间戳")
		return
	}
	
	// 1. 验证AccessKey
	auth, err := h.workerAuthService.ValidateAccessKey(accessKey)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	
	// 2. 验证时间戳（防重放攻击）
	if !h.workerAuthService.IsValidTimestamp(timestamp) {
		response.Unauthorized(c, "请求已过期")
		return
	}
	
	// 3. 验证签名
	if !h.workerAuthService.VerifySignature(accessKey, req.WorkerID, timestamp, signature, auth.SecretKey) {
		response.Unauthorized(c, "签名验证失败")
		return
	}

	// 4. 断开Worker连接
	if err := h.workerAuthService.DisconnectWorker(req.WorkerID); err != nil {
		response.ServerError(c, "断开连接失败: "+err.Error())
		return
	}

	// 5. 记录断开日志
	clientIP := c.ClientIP()
	h.workerAuthService.LogWorkerAuth(req.WorkerID, accessKey, "disconnect", clientIP)

	response.Success(c, "Worker已正常断开连接")
}

// UpdateWorkerAuthStatus 更新Worker栈权状态（管理员接口）
func (h *WorkerAuthHandler) UpdateWorkerAuthStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的授权ID")
		return
	}

	var req struct {
		Status string `json:"status" binding:"required,oneof=active disabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.workerAuthService.UpdateWorkerAuthStatus(uint(id), req.Status); err != nil {
		response.ServerError(c, "更新授权状态失败: "+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "授权状态更新成功"})
}

// DeleteWorkerAuth 删除Worker授权（管理员接口）
func (h *WorkerAuthHandler) DeleteWorkerAuth(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的授权ID")
		return
	}

	if err := h.workerAuthService.DeleteWorkerAuth(uint(id)); err != nil {
		response.ServerError(c, "删除授权失败: "+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "授权删除成功"})
}

// GetInitializationData 获取Worker初始化数据
func (h *WorkerAuthHandler) GetInitializationData(c *gin.Context) {
	// 从头部获取认证信息
	accessKey := c.GetHeader("X-Access-Key")
	timestampStr := c.GetHeader("X-Timestamp")
	signature := c.GetHeader("X-Signature")
	
	if accessKey == "" || timestampStr == "" || signature == "" {
		response.Unauthorized(c, "缺少认证信息")
		return
	}
	
	// 解析时间戳
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		response.Unauthorized(c, "无效的时间戳")
		return
	}
	
	// 1. 验证AccessKey
	auth, err := h.workerAuthService.ValidateAccessKey(accessKey)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	
	// 2. 验证时间戳（防重放攻击）
	if !h.workerAuthService.IsValidTimestamp(timestamp) {
		response.Unauthorized(c, "请求已过期")
		return
	}
	
	// 3. 验证签名
	if !h.workerAuthService.VerifySignature(accessKey, "/api/v1/worker/initialization", timestamp, signature, auth.SecretKey) {
		response.Unauthorized(c, "签名验证失败")
		return
	}

	// 4. 获取初始化数据
	initData, err := h.workerAuthService.GetWorkerInitializationData()
	if err != nil {
		response.ServerError(c, "获取初始化数据失败: "+err.Error())
		return
	}

	response.Success(c, initData)
}