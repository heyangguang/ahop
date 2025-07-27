package services

import (
	"ahop/internal/models"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
)

// TicketPluginService 工单插件服务
type TicketPluginService struct {
	db         *gorm.DB
	scheduler  *TicketSyncScheduler
}

// NewTicketPluginService 创建工单插件服务
func NewTicketPluginService(db *gorm.DB) *TicketPluginService {
	return &TicketPluginService{
		db:  db,
		scheduler: GetGlobalTicketSyncScheduler(), // 自动获取全局调度器
	}
}

// CreateTicketPlugin 创建工单插件
func (s *TicketPluginService) CreateTicketPlugin(tenantID uint, req CreateTicketPluginRequest) (*models.TicketPlugin, error) {
	// 检查同租户下code是否重复
	var count int64
	if err := s.db.Model(&models.TicketPlugin{}).Where("tenant_id = ? AND code = ?", tenantID, req.Code).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("插件编码已存在")
	}

	plugin := &models.TicketPlugin{
		TenantID:     tenantID,
		Name:         req.Name,
		Code:         req.Code,
		Description:  req.Description,
		BaseURL:      req.BaseURL,
		AuthType:     req.AuthType,
		SyncEnabled:  req.SyncEnabled,
		SyncInterval: req.SyncInterval,
		SyncWindow:   req.SyncWindow,
		Status:       "active",
	}
	
	// 设置默认值
	if plugin.SyncInterval == 0 {
		plugin.SyncInterval = 5
	}
	if plugin.SyncWindow == 0 {
		plugin.SyncWindow = 60
	}

	// 加密认证令牌
	if req.AuthToken != "" {
		encrypted, err := s.encrypt(req.AuthToken)
		if err != nil {
			return nil, fmt.Errorf("加密认证令牌失败: %v", err)
		}
		plugin.AuthToken = encrypted
	}

	if err := s.db.Create(plugin).Error; err != nil {
		return nil, err
	}

	// 如果启用了同步，添加到调度器
	if plugin.SyncEnabled && s.scheduler != nil {
		if err := s.scheduler.AddPlugin(plugin.ID); err != nil {
			// 记录错误但不影响插件创建
			fmt.Printf("添加插件到调度器失败: %v\n", err)
		}
	}

	return plugin, nil
}

// UpdateTicketPlugin 更新工单插件
func (s *TicketPluginService) UpdateTicketPlugin(tenantID uint, pluginID uint, req UpdateTicketPluginRequest) (*models.TicketPlugin, error) {
	var plugin models.TicketPlugin
	if err := s.db.Where("id = ? AND tenant_id = ?", pluginID, tenantID).First(&plugin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("插件不存在")
		}
		return nil, err
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.BaseURL != nil {
		updates["base_url"] = *req.BaseURL
	}
	if req.AuthType != nil {
		updates["auth_type"] = *req.AuthType
	}
	if req.SyncEnabled != nil {
		updates["sync_enabled"] = *req.SyncEnabled
	}
	if req.SyncInterval != nil {
		updates["sync_interval"] = *req.SyncInterval
	}
	if req.SyncWindow != nil {
		updates["sync_window"] = *req.SyncWindow
	}
	
	// 处理认证令牌更新
	if req.AuthToken != nil {
		if *req.AuthToken == "" {
			updates["auth_token"] = ""
		} else {
			encrypted, err := s.encrypt(*req.AuthToken)
			if err != nil {
				return nil, fmt.Errorf("加密认证令牌失败: %v", err)
			}
			updates["auth_token"] = encrypted
		}
	}

	// 如果code变更，检查重复
	if req.Code != nil && *req.Code != plugin.Code {
		var count int64
		if err := s.db.Model(&models.TicketPlugin{}).Where("tenant_id = ? AND code = ? AND id != ?", tenantID, *req.Code, pluginID).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("插件编码已存在")
		}
		updates["code"] = *req.Code
	}

	if err := s.db.Model(&plugin).Updates(updates).Error; err != nil {
		return nil, err
	}

	// 更新调度器中的任务
	if s.scheduler != nil {
		if err := s.scheduler.UpdatePlugin(pluginID); err != nil {
			// 记录错误但不影响插件更新
			fmt.Printf("更新调度器中的插件失败: %v\n", err)
		}
	}

	return &plugin, nil
}

// DeleteTicketPlugin 删除工单插件
func (s *TicketPluginService) DeleteTicketPlugin(tenantID uint, pluginID uint) error {
	// 检查是否有关联的工单
	var ticketCount int64
	if err := s.db.Model(&models.Ticket{}).Where("plugin_id = ?", pluginID).Count(&ticketCount).Error; err != nil {
		return err
	}
	if ticketCount > 0 {
		return fmt.Errorf("该插件有 %d 个关联工单，无法删除", ticketCount)
	}

	// 删除相关配置
	// 1. 删除字段映射
	if err := s.db.Where("plugin_id = ?", pluginID).Delete(&models.FieldMapping{}).Error; err != nil {
		return err
	}
	
	// 2. 删除同步规则
	if err := s.db.Where("plugin_id = ?", pluginID).Delete(&models.SyncRule{}).Error; err != nil {
		return err
	}
	
	// 3. 删除同步日志
	if err := s.db.Where("plugin_id = ?", pluginID).Delete(&models.TicketSyncLog{}).Error; err != nil {
		return err
	}

	// 从调度器中移除
	if s.scheduler != nil {
		s.scheduler.RemovePlugin(pluginID)
	}

	// 删除插件
	result := s.db.Where("id = ? AND tenant_id = ?", pluginID, tenantID).Delete(&models.TicketPlugin{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("插件不存在")
	}

	return nil
}

// GetTicketPlugin 获取工单插件详情
func (s *TicketPluginService) GetTicketPlugin(tenantID uint, pluginID uint) (*models.TicketPlugin, error) {
	var plugin models.TicketPlugin
	if err := s.db.Where("id = ? AND tenant_id = ?", pluginID, tenantID).First(&plugin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("插件不存在")
		}
		return nil, err
	}
	return &plugin, nil
}

// ListTicketPlugins 获取工单插件列表
func (s *TicketPluginService) ListTicketPlugins(tenantID uint, offset, limit int) ([]models.TicketPlugin, int64, error) {
	var plugins []models.TicketPlugin
	var total int64

	query := s.db.Model(&models.TicketPlugin{}).Where("tenant_id = ?", tenantID)
	
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&plugins).Error; err != nil {
		return nil, 0, err
	}

	return plugins, total, nil
}

// TestConnection 测试插件连接
func (s *TicketPluginService) TestConnection(tenantID uint, pluginID uint) (*TestConnectionResult, error) {
	plugin, err := s.GetTicketPlugin(tenantID, pluginID)
	if err != nil {
		return nil, err
	}

	result := &TestConnectionResult{
		Success: false,
		Message: "",
	}

	// 构建测试URL
	testURL := fmt.Sprintf("%s/tickets?minutes=1", plugin.BaseURL)
	
	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		result.Message = fmt.Sprintf("创建请求失败: %v", err)
		return result, nil
	}

	// 添加认证
	if plugin.AuthType != "none" && plugin.AuthToken != "" {
		token, err := s.decrypt(plugin.AuthToken)
		if err != nil {
			result.Message = fmt.Sprintf("解密认证令牌失败: %v", err)
			return result, nil
		}

		switch plugin.AuthType {
		case "bearer":
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		case "apikey":
			req.Header.Set("X-API-Key", token)
		}
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		result.Message = fmt.Sprintf("连接失败: %v", err)
		return result, nil
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		result.Message = fmt.Sprintf("响应状态码异常: %d", resp.StatusCode)
		return result, nil
	}

	result.Success = true
	result.Message = "连接成功"
	return result, nil
}

// EnablePlugin 启用插件
func (s *TicketPluginService) EnablePlugin(tenantID uint, pluginID uint) error {
	result := s.db.Model(&models.TicketPlugin{}).
		Where("id = ? AND tenant_id = ?", pluginID, tenantID).
		Updates(map[string]interface{}{
			"sync_enabled": true,
			"status": "active",
			"error_message": "",
		})
	
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("插件不存在")
	}
	
	// 添加到调度器
	if s.scheduler != nil {
		if err := s.scheduler.AddPlugin(pluginID); err != nil {
			fmt.Printf("添加插件到调度器失败: %v\n", err)
		}
	}
	
	return nil
}

// DisablePlugin 禁用插件
func (s *TicketPluginService) DisablePlugin(tenantID uint, pluginID uint) error {
	result := s.db.Model(&models.TicketPlugin{}).
		Where("id = ? AND tenant_id = ?", pluginID, tenantID).
		Update("sync_enabled", false)
	
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("插件不存在")
	}
	
	// 从调度器中移除
	if s.scheduler != nil {
		s.scheduler.RemovePlugin(pluginID)
	}
	
	return nil
}

// ManualSync 手动触发同步
func (s *TicketPluginService) ManualSync(tenantID uint, pluginID uint) error {
	// 验证插件存在且属于当前租户
	var plugin models.TicketPlugin
	if err := s.db.Where("id = ? AND tenant_id = ?", pluginID, tenantID).First(&plugin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("插件不存在")
		}
		return err
	}

	if !plugin.SyncEnabled {
		return errors.New("插件未启用同步")
	}

	// 创建同步服务并执行同步
	syncService := NewTicketSyncService(s.db)
	
	// 在新的goroutine中执行，避免阻塞
	go func() {
		if err := syncService.SyncTicketsForPlugin(pluginID); err != nil {
			// 错误会记录在同步日志中
		}
	}()
	
	return nil
}

// GetSyncLogs 获取同步日志
func (s *TicketPluginService) GetSyncLogs(tenantID uint, pluginID uint, offset, limit int) ([]models.TicketSyncLog, int64, error) {
	// 验证插件存在且属于当前租户
	var plugin models.TicketPlugin
	if err := s.db.Where("id = ? AND tenant_id = ?", pluginID, tenantID).First(&plugin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, errors.New("插件不存在")
		}
		return nil, 0, err
	}

	var logs []models.TicketSyncLog
	var total int64

	query := s.db.Model(&models.TicketSyncLog{}).Where("plugin_id = ?", pluginID)
	
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// TestSync 测试同步
func (s *TicketPluginService) TestSync(tenantID uint, pluginID uint, req TestSyncRequest) (*TestSyncResult, error) {
	// 获取插件
	plugin, err := s.GetTicketPlugin(tenantID, pluginID)
	if err != nil {
		return nil, err
	}

	// 设置默认值
	if req.TestOptions.SampleSize == 0 {
		req.TestOptions.SampleSize = 5
	}

	// 创建同步服务
	syncService := NewTicketSyncService(s.db)
	
	// 获取字段映射
	var fieldMappings []models.FieldMapping
	if err := s.db.Where("plugin_id = ?", pluginID).Find(&fieldMappings).Error; err != nil {
		return nil, fmt.Errorf("获取字段映射失败: %v", err)
	}

	// 获取过滤规则
	var syncRules []models.SyncRule
	if err := s.db.Where("plugin_id = ? AND enabled = ?", pluginID, true).
		Order("priority ASC").Find(&syncRules).Error; err != nil {
		return nil, fmt.Errorf("获取同步规则失败: %v", err)
	}

	// 构建插件请求URL
	url := plugin.BaseURL + "/tickets"
	if len(req.PluginParams) > 0 {
		// 添加查询参数
		params := make([]string, 0)
		for k, v := range req.PluginParams {
			params = append(params, fmt.Sprintf("%s=%v", k, v))
		}
		if len(params) > 0 {
			url += "?" + strings.Join(params, "&")
		}
	}

	// 调用插件获取数据
	tickets, err := s.fetchTicketsForTest(plugin, url)
	if err != nil {
		return nil, fmt.Errorf("获取工单数据失败: %v", err)
	}

	// 准备结果
	result := &TestSyncResult{
		Success: true,
		Summary: TestSyncSummary{
			TotalFetched: len(tickets),
			FilterRulesApplied: len(syncRules),
		},
		Samples: TestSyncSamples{
			RawData:     []models.JSON{},
			FilteredOut: []FilteredSample{},
			MappedData:  []MappedSample{},
		},
		Errors: []TestSyncError{},
	}

	// 处理每个工单
	filtered := 0
	mapped := 0
	
	for i, ticketData := range tickets {
		// 添加原始数据样本
		if i < req.TestOptions.SampleSize {
			result.Samples.RawData = append(result.Samples.RawData, ticketData)
		}

		// 应用过滤规则
		shouldSync, filterReason := s.testSyncRules(ticketData, syncRules)
		if !shouldSync {
			filtered++
			if req.TestOptions.ShowFiltered && len(result.Samples.FilteredOut) < req.TestOptions.SampleSize {
				result.Samples.FilteredOut = append(result.Samples.FilteredOut, FilteredSample{
					Data:   ticketData,
					Reason: filterReason,
				})
			}
			continue
		}

		// 映射字段
		mappedTicket, mappingInfo, err := s.mapTicketWithDetails(plugin, ticketData, fieldMappings)
		if err != nil {
			externalID := syncService.GetExternalID(ticketData)
			result.Errors = append(result.Errors, TestSyncError{
				ExternalID: externalID,
				Error:      err.Error(),
				Data:       ticketData,
			})
			continue
		}

		mapped++
		
		// 添加映射后的样本
		if len(result.Samples.MappedData) < req.TestOptions.SampleSize {
			sample := MappedSample{
				Ticket: mappedTicket,
			}
			if req.TestOptions.ShowMappingDetails {
				sample.MappingInfo = mappingInfo
			}
			result.Samples.MappedData = append(result.Samples.MappedData, sample)
		}
	}

	result.Summary.TotalFilteredOut = filtered
	result.Summary.TotalProcessed = result.Summary.TotalFetched - filtered
	result.Summary.TotalMapped = mapped

	return result, nil
}

// fetchTicketsForTest 为测试获取工单数据
func (s *TicketPluginService) fetchTicketsForTest(plugin *models.TicketPlugin, url string) ([]models.JSON, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 添加认证
	if plugin.AuthType != "none" && plugin.AuthToken != "" {
		token, err := s.decrypt(plugin.AuthToken)
		if err != nil {
			return nil, fmt.Errorf("解密认证令牌失败: %v", err)
		}

		switch plugin.AuthType {
		case "bearer":
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		case "apikey":
			req.Header.Set("X-API-Key", token)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("插件返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Success bool          `json:"success"`
		Data    []models.JSON `json:"data"`
		Message string        `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("插件返回失败: %s", response.Message)
	}

	return response.Data, nil
}

// testSyncRules 测试同步规则
func (s *TicketPluginService) testSyncRules(ticketData models.JSON, rules []models.SyncRule) (bool, string) {
	if len(rules) == 0 {
		return true, ""
	}

	syncService := NewTicketSyncService(s.db)
	
	var data map[string]interface{}
	if err := json.Unmarshal(ticketData, &data); err != nil {
		return false, "无法解析工单数据"
	}

	for _, rule := range rules {
		fieldValue := syncService.GetFieldValue(data, rule.Field)
		match := syncService.MatchRule(fieldValue, rule.Operator, rule.Value)
		
		if rule.Action == "exclude" && match {
			return false, fmt.Sprintf("规则'%s': %s %s %s", rule.Name, rule.Field, rule.Operator, rule.Value)
		}
		if rule.Action == "include" && !match {
			return false, fmt.Sprintf("规则'%s': %s 不满足 %s %s", rule.Name, rule.Field, rule.Operator, rule.Value)
		}
	}

	return true, ""
}

// mapTicketWithDetails 映射工单并返回映射详情
func (s *TicketPluginService) mapTicketWithDetails(plugin *models.TicketPlugin, ticketData models.JSON, mappings []models.FieldMapping) (*models.Ticket, map[string]MappingDetail, error) {
	syncService := NewTicketSyncService(s.db)
	
	// 使用同步服务的映射方法
	ticket, err := syncService.MapTicketFields(plugin, ticketData, mappings)
	if err != nil {
		return nil, nil, err
	}

	// 构建映射详情
	mappingInfo := make(map[string]MappingDetail)
	
	var data map[string]interface{}
	if err := json.Unmarshal(ticketData, &data); err != nil {
		return ticket, mappingInfo, nil
	}

	// 记录每个字段的映射信息
	if len(mappings) > 0 {
		for _, mapping := range mappings {
			value := syncService.GetFieldValue(data, mapping.SourceField)
			if targetField := getTargetFieldName(mapping.TargetField); targetField != "" {
				mappingInfo[targetField] = MappingDetail{
					SourceField: mapping.SourceField,
					SourceValue: value,
				}
			}
		}
	} else {
		// 默认映射的详情
		mappingInfo["external_id"] = MappingDetail{
			SourceField: "id",
			SourceValue: ticket.ExternalID,
		}
		mappingInfo["title"] = MappingDetail{
			SourceField: "title",
			SourceValue: ticket.Title,
		}
		// ... 可以添加更多默认映射的详情
	}

	return ticket, mappingInfo, nil
}

// getTargetFieldName 获取目标字段的显示名称
func getTargetFieldName(field string) string {
	fieldMap := map[string]string{
		"external_id": "external_id",
		"title":       "title",
		"description": "description",
		"status":      "status",
		"priority":    "priority",
		"type":        "type",
		"reporter":    "reporter",
		"assignee":    "assignee",
		"category":    "category",
		"service":     "service",
		"tags":        "tags",
	}
	return fieldMap[field]
}

// 加密方法
func (s *TicketPluginService) encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	
	key := s.encryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// 解密方法
func (s *TicketPluginService) decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	key := s.encryptionKey()
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("密文太短")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// 获取加密密钥
func (s *TicketPluginService) encryptionKey() []byte {
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	if key == "" {
		// 使用默认密钥（仅用于开发）
		key = "12345678901234567890123456789012"
	}
	
	// 确保密钥长度为32字节
	keyBytes, _ := hex.DecodeString(hex.EncodeToString([]byte(key))[:64])
	if len(keyBytes) < 32 {
		paddedKey := make([]byte, 32)
		copy(paddedKey, keyBytes)
		return paddedKey
	}
	
	return keyBytes[:32]
}

// 请求结构体定义

// CreateTicketPluginRequest 创建工单插件请求
type CreateTicketPluginRequest struct {
	Name         string `json:"name" binding:"required,min=1,max=100"`
	Code         string `json:"code" binding:"required,min=1,max=50"`
	Description  string `json:"description"`
	BaseURL      string `json:"base_url" binding:"required,url"`
	AuthType     string `json:"auth_type" binding:"oneof=none bearer apikey"`
	AuthToken    string `json:"auth_token"`
	SyncEnabled  bool   `json:"sync_enabled"`
	SyncInterval int    `json:"sync_interval" binding:"min=1,max=1440"`  // 1分钟到24小时
	SyncWindow   int    `json:"sync_window" binding:"min=1,max=43200"`   // 1分钟到30天
}

// UpdateTicketPluginRequest 更新工单插件请求
type UpdateTicketPluginRequest struct {
	Name         *string `json:"name"`
	Code         *string `json:"code"`
	Description  *string `json:"description"`
	BaseURL      *string `json:"base_url"`
	AuthType     *string `json:"auth_type"`
	AuthToken    *string `json:"auth_token"`
	SyncEnabled  *bool   `json:"sync_enabled"`
	SyncInterval *int    `json:"sync_interval"`
	SyncWindow   *int    `json:"sync_window"`
}

// TestConnectionResult 测试连接结果
type TestConnectionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// TestSyncRequest 测试同步请求
type TestSyncRequest struct {
	PluginParams map[string]interface{} `json:"plugin_params"` // 传递给插件的参数
	TestOptions  TestSyncOptions        `json:"test_options"`  // AHOP内部测试选项
}

// TestSyncOptions 测试同步选项
type TestSyncOptions struct {
	SampleSize          int  `json:"sample_size" binding:"min=1,max=50"`           // 样本数量，默认5
	ShowFiltered        bool `json:"show_filtered"`                                // 显示被过滤的数据
	ShowMappingDetails  bool `json:"show_mapping_details"`                         // 显示映射详情
}

// TestSyncResult 测试同步结果
type TestSyncResult struct {
	Success bool                `json:"success"`
	Summary TestSyncSummary     `json:"summary"`
	Samples TestSyncSamples     `json:"samples"`
	Errors  []TestSyncError     `json:"errors"`
}

// TestSyncSummary 测试同步摘要
type TestSyncSummary struct {
	TotalFetched       int `json:"total_fetched"`        // 获取的总数
	TotalFilteredOut   int `json:"total_filtered_out"`   // 被过滤掉的数量
	TotalProcessed     int `json:"total_processed"`      // 处理的数量（过滤后）
	TotalMapped        int `json:"total_mapped"`         // 成功映射的数量
	FilterRulesApplied int `json:"filter_rules_applied"` // 应用的规则数
}

// TestSyncSamples 测试同步样本
type TestSyncSamples struct {
	RawData      []models.JSON           `json:"raw_data"`      // 原始数据样本
	FilteredOut  []FilteredSample        `json:"filtered_out"`  // 被过滤的样本
	MappedData   []MappedSample          `json:"mapped_data"`   // 映射后的样本
}

// FilteredSample 被过滤的样本
type FilteredSample struct {
	Data   models.JSON `json:"data"`
	Reason string      `json:"reason"`
}

// MappedSample 映射后的样本
type MappedSample struct {
	*models.Ticket
	MappingInfo map[string]MappingDetail `json:"_mapping_info,omitempty"`
}

// MappingDetail 映射详情
type MappingDetail struct {
	SourceField string      `json:"source_field"`
	SourceValue interface{} `json:"source_value"`
}

// TestSyncError 测试同步错误
type TestSyncError struct {
	ExternalID string `json:"external_id"`
	Error      string `json:"error"`
	Data       models.JSON `json:"data,omitempty"`
}