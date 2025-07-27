package services

import (
	"ahop/internal/models"
	"ahop/pkg/logger"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// TicketSyncService 工单同步服务
type TicketSyncService struct {
	db                  *gorm.DB
	ticketPluginService *TicketPluginService
}

// NewTicketSyncService 创建工单同步服务
func NewTicketSyncService(db *gorm.DB) *TicketSyncService {
	return &TicketSyncService{
		db:                  db,
		ticketPluginService: NewTicketPluginService(db),
	}
}

// SyncTicketsForPlugin 为指定插件同步工单
func (s *TicketSyncService) SyncTicketsForPlugin(pluginID uint) error {
	log := logger.GetLogger()
	
	// 获取插件配置
	var plugin models.TicketPlugin
	if err := s.db.First(&plugin, pluginID).Error; err != nil {
		return fmt.Errorf("获取插件失败: %v", err)
	}

	// 检查插件是否启用
	if !plugin.SyncEnabled {
		return errors.New("插件未启用同步")
	}

	// 创建同步日志
	syncLog := &models.TicketSyncLog{
		PluginID:  pluginID,
		TenantID:  plugin.TenantID,
		StartTime: time.Now(),
		Status:    "running",
	}

	// 执行同步
	err := s.performSync(&plugin, syncLog)
	
	// 更新同步完成时间
	syncLog.EndTime = time.Now()
	syncLog.Duration = int(syncLog.EndTime.Sub(syncLog.StartTime).Seconds())
	
	if err != nil {
		syncLog.Status = "failed"
		syncLog.ErrorMessage = err.Error()
		log.WithError(err).Errorf("插件 %s 同步失败", plugin.Name)
	} else {
		syncLog.Status = "success"
		log.Infof("插件 %s 同步成功: 获取 %d, 过滤掉 %d, 处理 %d, 新建 %d, 更新 %d", 
			plugin.Name, syncLog.TotalFetched, syncLog.TotalFilteredOut, 
			syncLog.TotalProcessed, syncLog.TotalCreated, syncLog.TotalUpdated)
	}

	// 保存同步日志
	if err := s.db.Create(syncLog).Error; err != nil {
		log.WithError(err).Error("保存同步日志失败")
	}

	// 更新插件最后同步时间
	now := time.Now()
	updates := map[string]interface{}{
		"last_sync_at": &now,
	}
	if err != nil {
		updates["status"] = "error"
		updates["error_message"] = err.Error()
	} else {
		updates["status"] = "active"
		updates["error_message"] = ""
	}
	
	s.db.Model(&plugin).Updates(updates)

	return err
}

// performSync 执行实际的同步操作
func (s *TicketSyncService) performSync(plugin *models.TicketPlugin, syncLog *models.TicketSyncLog) error {
	// 1. 获取字段映射配置
	var fieldMappings []models.FieldMapping
	if err := s.db.Where("plugin_id = ?", plugin.ID).Find(&fieldMappings).Error; err != nil {
		return fmt.Errorf("获取字段映射失败: %v", err)
	}

	// 2. 获取过滤规则
	var syncRules []models.SyncRule
	if err := s.db.Where("plugin_id = ? AND enabled = ?", plugin.ID, true).
		Order("priority ASC").Find(&syncRules).Error; err != nil {
		return fmt.Errorf("获取同步规则失败: %v", err)
	}

	// 3. 调用插件接口获取工单
	tickets, err := s.fetchTicketsFromPlugin(plugin)
	if err != nil {
		return fmt.Errorf("获取工单数据失败: %v", err)
	}
	
	syncLog.TotalFetched = len(tickets)

	// 4. 处理每个工单
	var errors []models.TicketSyncError
	filtered := 0
	created := 0
	updated := 0

	for _, ticketData := range tickets {
		// 应用过滤规则
		if !s.shouldSyncTicket(ticketData, syncRules) {
			filtered++
			continue
		}

		// 映射字段
		mappedTicket, err := s.MapTicketFields(plugin, ticketData, fieldMappings)
		if err != nil {
			errors = append(errors, models.TicketSyncError{
				ExternalID: s.GetExternalID(ticketData),
				Error:      err.Error(),
				Data:       ticketData,
			})
			syncLog.TotalFailed++
			continue
		}

		// 保存或更新工单
		isNew, err := s.saveTicket(mappedTicket)
		if err != nil {
			errors = append(errors, models.TicketSyncError{
				ExternalID: mappedTicket.ExternalID,
				Error:      err.Error(),
			})
			syncLog.TotalFailed++
			continue
		}

		if isNew {
			created++
		} else {
			updated++
		}
	}

	syncLog.TotalFilteredOut = filtered
	syncLog.TotalProcessed = syncLog.TotalFetched - filtered
	syncLog.TotalCreated = created
	syncLog.TotalUpdated = updated

	// 记录错误详情
	if len(errors) > 0 {
		errorDetails, _ := json.Marshal(errors)
		syncLog.ErrorDetails = errorDetails
	}

	return nil
}

// fetchTicketsFromPlugin 从插件获取工单数据
func (s *TicketSyncService) fetchTicketsFromPlugin(plugin *models.TicketPlugin) ([]models.JSON, error) {
	// 构建请求URL
	// 使用 SyncWindow 作为数据获取的时间窗口
	minutes := plugin.SyncWindow
	if minutes == 0 {
		minutes = 60 // 默认获取最近60分钟的数据
	}
	
	// 如果是首次同步（没有上次同步时间），可以使用更大的时间窗口
	if plugin.LastSyncAt == nil {
		// 首次同步时，获取更多历史数据（最多30天）
		if minutes < 43200 {
			minutes = 43200
		}
	}
	
	url := fmt.Sprintf("%s/tickets?minutes=%d", plugin.BaseURL, minutes)

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 添加认证
	if plugin.AuthType != "none" && plugin.AuthToken != "" {
		token, err := s.ticketPluginService.decrypt(plugin.AuthToken)
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

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("插件返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
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

// shouldSyncTicket 判断工单是否应该同步
func (s *TicketSyncService) shouldSyncTicket(ticketData models.JSON, rules []models.SyncRule) bool {
	// 如果没有规则，默认同步所有
	if len(rules) == 0 {
		return true
	}

	// 解析工单数据
	var data map[string]interface{}
	if err := json.Unmarshal(ticketData, &data); err != nil {
		return false
	}

	// 应用规则
	for _, rule := range rules {
		fieldValue := s.GetFieldValue(data, rule.Field)
		match := s.MatchRule(fieldValue, rule.Operator, rule.Value)
		
		if rule.Action == "exclude" && match {
			return false
		}
		if rule.Action == "include" && !match {
			return false
		}
	}

	return true
}

// GetFieldValue 获取字段值
func (s *TicketSyncService) GetFieldValue(data map[string]interface{}, field string) string {
	// 支持嵌套字段，如 "issue.fields.priority" 或 "items.0.name"
	parts := strings.Split(field, ".")
	var current interface{} = data
	
	for _, part := range parts {
		// 检查是否是数组索引访问
		if index, err := strconv.Atoi(part); err == nil {
			// 是数字，尝试作为数组索引
			if arr, ok := current.([]interface{}); ok {
				if index >= 0 && index < len(arr) {
					current = arr[index]
				} else {
					return ""
				}
			} else {
				return ""
			}
		} else {
			// 不是数字，作为字段名访问
			switch v := current.(type) {
			case map[string]interface{}:
				if val, ok := v[part]; ok {
					current = val
				} else {
					return ""
				}
			default:
				return ""
			}
		}
	}
	
	// 将最终值转换为字符串
	switch v := current.(type) {
	case string:
		return v
	case []interface{}:
		// 如果是数组，将其转换为逗号分隔的字符串
		var items []string
		for _, item := range v {
			items = append(items, fmt.Sprintf("%v", item))
		}
		return strings.Join(items, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// MatchRule 匹配规则
func (s *TicketSyncService) MatchRule(value, operator, pattern string) bool {
	switch operator {
	case "equals":
		return value == pattern
	case "not_equals":
		return value != pattern
	case "contains":
		return strings.Contains(value, pattern)
	case "in":
		// pattern 应该是逗号分隔的值列表
		values := strings.Split(pattern, ",")
		for _, v := range values {
			if strings.TrimSpace(v) == value {
				return true
			}
		}
		return false
	case "regex":
		// TODO: 实现正则匹配
		return false
	default:
		return false
	}
}

// MapTicketFields 映射工单字段
func (s *TicketSyncService) MapTicketFields(plugin *models.TicketPlugin, ticketData models.JSON, mappings []models.FieldMapping) (*models.Ticket, error) {
	// 解析工单数据
	var data map[string]interface{}
	if err := json.Unmarshal(ticketData, &data); err != nil {
		return nil, fmt.Errorf("解析工单数据失败: %v", err)
	}

	// 创建工单对象
	ticket := &models.Ticket{
		TenantID:   plugin.TenantID,
		PluginID:   plugin.ID,
		CustomData: ticketData,
		SyncedAt:   time.Now(),
	}

	// 如果没有映射配置，使用默认映射
	if len(mappings) == 0 {
		return s.applyDefaultMapping(ticket, data)
	}

	// 应用字段映射
	for _, mapping := range mappings {
		value := s.GetFieldValue(data, mapping.SourceField)
		if value == "" && mapping.DefaultValue != "" {
			value = mapping.DefaultValue
		}
		
		if mapping.Required && value == "" {
			return nil, fmt.Errorf("必需字段 %s 为空", mapping.SourceField)
		}

		// 根据目标字段设置值
		s.setTicketField(ticket, mapping.TargetField, value)
	}

	// 验证必需字段
	if ticket.ExternalID == "" {
		return nil, errors.New("缺少外部工单ID")
	}
	if ticket.Title == "" {
		return nil, errors.New("缺少工单标题")
	}

	return ticket, nil
}

// applyDefaultMapping 应用默认字段映射
func (s *TicketSyncService) applyDefaultMapping(ticket *models.Ticket, data map[string]interface{}) (*models.Ticket, error) {
	// 尝试常见的字段名
	ticket.ExternalID = s.tryGetString(data, "id", "ticket_id", "issue_id")
	ticket.Title = s.tryGetString(data, "title", "summary", "subject")
	ticket.Description = s.tryGetString(data, "description", "details", "body")
	ticket.Status = s.tryGetString(data, "status", "state")
	ticket.Priority = s.tryGetString(data, "priority", "urgency")
	ticket.Type = s.tryGetString(data, "type", "issue_type", "ticket_type")
	ticket.Reporter = s.tryGetString(data, "reporter", "created_by", "requester")
	ticket.Assignee = s.tryGetString(data, "assignee", "assigned_to")
	ticket.Category = s.tryGetString(data, "category", "classification")
	ticket.Service = s.tryGetString(data, "service", "application")
	
	// 处理时间字段
	if createdAt := s.tryGetString(data, "created_at", "created", "create_time"); createdAt != "" {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			ticket.ExternalCreatedAt = t
		}
	}
	
	if updatedAt := s.tryGetString(data, "updated_at", "updated", "update_time"); updatedAt != "" {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			ticket.ExternalUpdatedAt = t
		}
	}
	
	// 处理标签
	if tags, ok := data["tags"].([]interface{}); ok {
		for _, tag := range tags {
			ticket.Tags = append(ticket.Tags, fmt.Sprintf("%v", tag))
		}
	}

	// 验证必需字段
	if ticket.ExternalID == "" {
		return nil, errors.New("无法识别工单ID字段")
	}
	if ticket.Title == "" {
		return nil, errors.New("无法识别工单标题字段")
	}

	return ticket, nil
}

// tryGetString 尝试从多个可能的字段名获取字符串值
func (s *TicketSyncService) tryGetString(data map[string]interface{}, fields ...string) string {
	for _, field := range fields {
		if val, ok := data[field]; ok && val != nil {
			return fmt.Sprintf("%v", val)
		}
	}
	return ""
}

// setTicketField 设置工单字段
func (s *TicketSyncService) setTicketField(ticket *models.Ticket, field, value string) {
	switch field {
	case "external_id":
		ticket.ExternalID = value
	case "title":
		ticket.Title = value
	case "description":
		ticket.Description = value
	case "status":
		ticket.Status = value
	case "priority":
		ticket.Priority = value
	case "type":
		ticket.Type = value
	case "reporter":
		ticket.Reporter = value
	case "assignee":
		ticket.Assignee = value
	case "category":
		ticket.Category = value
	case "service":
		ticket.Service = value
	case "tags":
		// 假设value是逗号分隔的标签列表
		tags := strings.Split(value, ",")
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				ticket.Tags = append(ticket.Tags, tag)
			}
		}
	}
}

// GetExternalID 从工单数据中获取外部ID
func (s *TicketSyncService) GetExternalID(ticketData models.JSON) string {
	var data map[string]interface{}
	if err := json.Unmarshal(ticketData, &data); err != nil {
		return "unknown"
	}
	
	return s.tryGetString(data, "id", "ticket_id", "issue_id")
}

// saveTicket 保存或更新工单
func (s *TicketSyncService) saveTicket(ticket *models.Ticket) (bool, error) {
	// 检查工单是否已存在
	var existing models.Ticket
	err := s.db.Where("plugin_id = ? AND external_id = ?", ticket.PluginID, ticket.ExternalID).First(&existing).Error
	
	if err == gorm.ErrRecordNotFound {
		// 创建新工单
		if err := s.db.Create(ticket).Error; err != nil {
			return false, err
		}
		return true, nil
	}
	
	if err != nil {
		return false, err
	}
	
	// 更新现有工单
	updates := map[string]interface{}{
		"title":               ticket.Title,
		"description":         ticket.Description,
		"status":              ticket.Status,
		"priority":            ticket.Priority,
		"type":                ticket.Type,
		"reporter":            ticket.Reporter,
		"assignee":            ticket.Assignee,
		"category":            ticket.Category,
		"service":             ticket.Service,
		"tags":                ticket.Tags,
		"external_updated_at": ticket.ExternalUpdatedAt,
		"custom_data":         ticket.CustomData,
		"synced_at":           ticket.SyncedAt,
	}
	
	if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
		return false, err
	}
	
	return false, nil
}