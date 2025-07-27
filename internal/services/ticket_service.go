package services

import (
	"ahop/internal/models"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// TicketService 工单服务
type TicketService struct {
	db *gorm.DB
}

// NewTicketService 创建工单服务
func NewTicketService(db *gorm.DB) *TicketService {
	return &TicketService{
		db: db,
	}
}

// GetTicket 获取工单详情
func (s *TicketService) GetTicket(tenantID uint, ticketID uint) (*models.Ticket, error) {
	var ticket models.Ticket
	err := s.db.Preload("Plugin").Where("id = ? AND tenant_id = ?", ticketID, tenantID).First(&ticket).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("工单不存在")
		}
		return nil, err
	}
	return &ticket, nil
}

// ListTickets 获取工单列表
func (s *TicketService) ListTickets(tenantID uint, filter TicketFilter, offset, limit int) ([]models.Ticket, int64, error) {
	var tickets []models.Ticket
	var total int64

	query := s.db.Model(&models.Ticket{}).Where("tenant_id = ?", tenantID)

	// 应用过滤条件
	if filter.PluginID != nil {
		query = query.Where("plugin_id = ?", *filter.PluginID)
	}
	if filter.Status != nil && *filter.Status != "" {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.Priority != nil && *filter.Priority != "" {
		query = query.Where("priority = ?", *filter.Priority)
	}
	if filter.Type != nil && *filter.Type != "" {
		query = query.Where("type = ?", *filter.Type)
	}
	if filter.Category != nil && *filter.Category != "" {
		query = query.Where("category = ?", *filter.Category)
	}
	if filter.Service != nil && *filter.Service != "" {
		query = query.Where("service = ?", *filter.Service)
	}
	if filter.Assignee != nil && *filter.Assignee != "" {
		query = query.Where("assignee = ?", *filter.Assignee)
	}
	if filter.Search != nil && *filter.Search != "" {
		searchPattern := "%" + *filter.Search + "%"
		query = query.Where("title LIKE ? OR description LIKE ? OR external_id LIKE ?", 
			searchPattern, searchPattern, searchPattern)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取数据
	err := query.
		Preload("Plugin").
		Order("external_updated_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&tickets).Error

	if err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

// GetTicketStats 获取工单统计
func (s *TicketService) GetTicketStats(tenantID uint) (*TicketStats, error) {
	stats := &TicketStats{}

	// 总工单数
	if err := s.db.Model(&models.Ticket{}).Where("tenant_id = ?", tenantID).Count(&stats.Total).Error; err != nil {
		return nil, err
	}

	// 按状态统计
	var statusStats []struct {
		Status string
		Count  int64
	}
	if err := s.db.Model(&models.Ticket{}).
		Select("status, COUNT(*) as count").
		Where("tenant_id = ?", tenantID).
		Group("status").
		Scan(&statusStats).Error; err != nil {
		return nil, err
	}

	stats.ByStatus = make(map[string]int64)
	for _, ss := range statusStats {
		stats.ByStatus[ss.Status] = ss.Count
	}

	// 按优先级统计
	var priorityStats []struct {
		Priority string
		Count    int64
	}
	if err := s.db.Model(&models.Ticket{}).
		Select("priority, COUNT(*) as count").
		Where("tenant_id = ? AND priority IS NOT NULL AND priority != ''", tenantID).
		Group("priority").
		Scan(&priorityStats).Error; err != nil {
		return nil, err
	}

	stats.ByPriority = make(map[string]int64)
	for _, ps := range priorityStats {
		stats.ByPriority[ps.Priority] = ps.Count
	}

	// 按插件统计
	var pluginStats []struct {
		PluginID uint
		Name     string
		Count    int64
	}
	if err := s.db.Table("tickets").
		Select("tickets.plugin_id, ticket_plugins.name, COUNT(*) as count").
		Joins("LEFT JOIN ticket_plugins ON tickets.plugin_id = ticket_plugins.id").
		Where("tickets.tenant_id = ?", tenantID).
		Group("tickets.plugin_id, ticket_plugins.name").
		Scan(&pluginStats).Error; err != nil {
		return nil, err
	}

	stats.ByPlugin = make(map[string]int64)
	for _, ps := range pluginStats {
		stats.ByPlugin[ps.Name] = ps.Count
	}

	// 今日新增
	if err := s.db.Model(&models.Ticket{}).
		Where("tenant_id = ? AND DATE(synced_at) = CURRENT_DATE", tenantID).
		Count(&stats.TodayNew).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// UpdateTicketComment 更新工单评论（回写到插件）
func (s *TicketService) UpdateTicketComment(tenantID uint, ticketID uint, comment string) error {
	// 获取工单信息
	var ticket models.Ticket
	if err := s.db.Preload("Plugin").Where("id = ? AND tenant_id = ?", ticketID, tenantID).First(&ticket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("工单不存在")
		}
		return err
	}

	// TODO: 调用插件接口更新工单
	// 这里需要调用插件的更新接口，将评论回写到原始工单系统
	
	return fmt.Errorf("工单评论更新功能尚未实现")
}

// 过滤条件
type TicketFilter struct {
	PluginID *uint   `form:"plugin_id"`
	Status   *string `form:"status"`
	Priority *string `form:"priority"`
	Type     *string `form:"type"`
	Category *string `form:"category"`
	Service  *string `form:"service"`
	Assignee *string `form:"assignee"`
	Search   *string `form:"search"`
}

// 工单统计
type TicketStats struct {
	Total      int64            `json:"total"`
	TodayNew   int64            `json:"today_new"`
	ByStatus   map[string]int64 `json:"by_status"`
	ByPriority map[string]int64 `json:"by_priority"`
	ByPlugin   map[string]int64 `json:"by_plugin"`
}