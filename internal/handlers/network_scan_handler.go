package handlers

import (
	"ahop/internal/models"
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/response"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// NetworkScanHandler 网络扫描处理器
type NetworkScanHandler struct {
	scanService *services.NetworkScanService
	hostService *services.HostService
}

// NewNetworkScanHandler 创建网络扫描处理器
func NewNetworkScanHandler(scanService *services.NetworkScanService, hostService *services.HostService) *NetworkScanHandler {
	return &NetworkScanHandler{
		scanService: scanService,
		hostService: hostService,
	}
}

// StartScan 开始网络扫描
func (h *NetworkScanHandler) StartScan(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)
	
	var req models.ScanConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	
	// 启动扫描任务（异步，不阻塞）
	task, err := h.scanService.StartScan(claims.CurrentTenantID, claims.UserID, claims.Username, &req)
	if err != nil {
		// 区分配置验证错误和服务器错误
		if strings.Contains(err.Error(), "配置验证失败") || strings.Contains(err.Error(), "网络解析失败") {
			response.BadRequest(c, err.Error())
		} else {
			response.ServerError(c, err.Error())
		}
		return
	}
	
	// 立即返回任务信息
	result := map[string]interface{}{
		"scan_id":     task.ScanID,
		"status":      task.Status,
		"start_time":  task.StartTime,
		"progress":    task.Progress,
		"message":     "扫描任务已启动，请通过WebSocket监听实时结果",
	}
	
	response.SuccessWithMessage(c, "扫描已开始", result)
}

// GetScanStatus 获取扫描状态
func (h *NetworkScanHandler) GetScanStatus(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)
	
	scanID := c.Param("scan_id")
	if scanID == "" {
		response.BadRequest(c, "缺少扫描任务ID")
		return
	}
	
	// 获取任务状态快照
	task, err := h.scanService.GetTaskStatus(scanID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	
	// 验证租户权限
	if task.TenantID != claims.CurrentTenantID {
		response.Forbidden(c, "无权限访问此扫描任务")
		return
	}
	
	response.Success(c, task)
}

// CancelScan 取消扫描任务
func (h *NetworkScanHandler) CancelScan(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)
	
	scanID := c.Param("scan_id")
	if scanID == "" {
		response.BadRequest(c, "缺少扫描任务ID")
		return
	}
	
	// 检查权限
	task, err := h.scanService.GetTaskStatus(scanID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	
	if task.TenantID != claims.CurrentTenantID {
		response.Forbidden(c, "无权限操作此扫描任务")
		return
	}
	
	// 取消任务
	if err := h.scanService.CancelScan(scanID); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	
	response.SuccessWithMessage(c, "扫描任务已取消", nil)
}

// GetActiveTasks 获取活跃扫描任务列表
func (h *NetworkScanHandler) GetActiveTasks(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)
	
	// 获取所有活跃任务
	allTasks := h.scanService.GetActiveTasks()
	
	// 过滤当前租户的任务
	var tenantTasks []*models.ScanTask
	for _, task := range allTasks {
		if task.TenantID == claims.CurrentTenantID {
			tenantTasks = append(tenantTasks, task)
		}
	}
	
	response.Success(c, tenantTasks)
}

// EstimateTargets 估算扫描目标数量
func (h *NetworkScanHandler) EstimateTargets(c *gin.Context) {
	var req models.ScanConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	
	count, err := h.scanService.EstimateTargetCount(&req)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	
	result := map[string]interface{}{
		"target_count": count,
		"networks":     req.Networks,
		"methods":      req.Methods,
		"estimated_duration": estimateDuration(count, req.Concurrency, req.Timeout),
	}
	
	response.Success(c, result)
}

// ImportDiscoveredHosts 批量导入发现的主机
func (h *NetworkScanHandler) ImportDiscoveredHosts(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)
	
	var req models.NetworkScanImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	
	// 检查扫描任务是否存在且属于当前租户
	task, err := h.scanService.GetTaskStatus(req.ScanID)
	if err != nil {
		response.NotFound(c, "扫描任务不存在")
		return
	}
	
	if task.TenantID != claims.CurrentTenantID {
		response.Forbidden(c, "无权限操作此扫描任务")
		return
	}
	
	// 验证选择的IP是否在扫描结果中
	aliveIPs := task.GetAliveIPs()
	aliveIPSet := make(map[string]bool)
	for _, ip := range aliveIPs {
		aliveIPSet[ip] = true
	}
	
	var validIPs []string
	for _, ip := range req.IPs {
		if aliveIPSet[ip] {
			validIPs = append(validIPs, ip)
		}
	}
	
	if len(validIPs) == 0 {
		response.BadRequest(c, "所选IP不在扫描结果中或未存活")
		return
	}
	
	// 构建主机导入数据（使用统一的BatchImportHost结构）
	var hosts []services.BatchImportHost
	port := req.Port
	if port <= 0 {
		port = 22 // 默认SSH端口
	}
	
	description := req.Description
	if description == "" {
		description = "网络扫描发现" // 默认描述
	}
	
	for _, ip := range validIPs {
		hosts = append(hosts, services.BatchImportHost{
			IP:           ip,
			Port:         port,
			CredentialID: req.CredentialID,
			Hostname:     ip,               // 使用IP作为主机名（可为空，服务会自动设置）
			SSHUser:      "",               // 使用默认用户（服务会设置为root）
			Description:  description,
			HostGroupID:  req.HostGroupID,
			Tags:         req.Tags,
		})
	}
	
	// 调用主机服务执行批量导入
	result, err := h.hostService.BatchImport(claims.CurrentTenantID, claims.UserID, hosts)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	
	// 返回导入结果
	importResult := map[string]interface{}{
		"scan_id":          req.ScanID,
		"selected_count":   len(req.IPs),
		"valid_count":      len(validIPs),
		"import_result":    result,
	}
	
	response.SuccessWithMessage(c, "批量导入完成", importResult)
}

// GetScanResult 获取完整扫描结果
func (h *NetworkScanHandler) GetScanResult(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)
	
	scanID := c.Param("scan_id")
	if scanID == "" {
		response.BadRequest(c, "缺少扫描任务ID")
		return
	}
	
	// 获取任务
	task, err := h.scanService.GetTaskStatus(scanID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	
	// 验证租户权限
	if task.TenantID != claims.CurrentTenantID {
		response.Forbidden(c, "无权限访问此扫描任务")
		return
	}
	
	// 解析查询参数
	onlyAlive, _ := strconv.ParseBool(c.DefaultQuery("only_alive", "false"))
	protocol := c.Query("protocol")
	
	// 过滤结果
	results := task.GetResults()
	var filteredResults []*models.ScanResult
	
	for _, result := range results {
		// 过滤存活状态
		if onlyAlive && result.Status != models.ScanResultStatusAlive {
			continue
		}
		
		// 过滤协议
		if protocol != "" && result.Protocol != protocol {
			continue
		}
		
		filteredResults = append(filteredResults, result)
	}
	
	// 构建响应
	resultData := map[string]interface{}{
		"scan_info": map[string]interface{}{
			"scan_id":     task.ScanID,
			"status":      task.Status,
			"progress":    task.Progress,
			"start_time":  task.StartTime,
			"end_time":    task.EndTime,
			"total_found": task.GetAliveCount(),
		},
		"results": filteredResults,
		"filters": map[string]interface{}{
			"only_alive": onlyAlive,
			"protocol":   protocol,
		},
		"summary": generateResultSummary(filteredResults),
	}
	
	response.Success(c, resultData)
}

// generateResultSummary 生成扫描结果摘要
func generateResultSummary(results []*models.ScanResult) map[string]interface{} {
	summary := map[string]interface{}{
		"total_scans": len(results),
		"by_status": make(map[string]int),
		"by_protocol": make(map[string]int),
		"alive_ips": make(map[string]bool),
	}
	
	statusCount := make(map[string]int)
	protocolCount := make(map[string]int)
	aliveIPs := make(map[string]bool)
	
	for _, result := range results {
		statusCount[result.Status]++
		protocolCount[result.Protocol]++
		
		if result.Status == models.ScanResultStatusAlive {
			aliveIPs[result.IP] = true
		}
	}
	
	summary["by_status"] = statusCount
	summary["by_protocol"] = protocolCount
	summary["unique_alive_ips"] = len(aliveIPs)
	
	// 转换存活IP映射为切片
	var aliveIPList []string
	for ip := range aliveIPs {
		aliveIPList = append(aliveIPList, ip)
	}
	summary["alive_ip_list"] = aliveIPList
	
	return summary
}

// estimateDuration 估算扫描持续时间
func estimateDuration(targetCount, concurrency, timeoutSeconds int) string {
	if concurrency <= 0 {
		concurrency = 50
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 5
	}
	
	// 简单估算：考虑并发和超时
	estimatedSeconds := (targetCount * timeoutSeconds) / concurrency
	
	if estimatedSeconds < 60 {
		return strconv.Itoa(estimatedSeconds) + "秒"
	} else if estimatedSeconds < 3600 {
		return strconv.Itoa(estimatedSeconds/60) + "分钟"
	} else {
		return strconv.Itoa(estimatedSeconds/3600) + "小时"
	}
}