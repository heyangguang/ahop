package services

import (
	"ahop/internal/models"
	"ahop/pkg/logger"
	"fmt"
	"sync"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// TicketSyncScheduler 工单同步调度器
type TicketSyncScheduler struct {
	db         *gorm.DB
	cron       *cron.Cron
	syncService *TicketSyncService
	jobMap     map[uint]cron.EntryID  // pluginID -> cronJobID
	mu         sync.RWMutex
	running    bool
}

// NewTicketSyncScheduler 创建工单同步调度器
func NewTicketSyncScheduler(db *gorm.DB) *TicketSyncScheduler {
	return &TicketSyncScheduler{
		db:          db,
		cron:        cron.New(),
		syncService: NewTicketSyncService(db),
		jobMap:      make(map[uint]cron.EntryID),
	}
}

// Start 启动调度器
func (s *TicketSyncScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	log := logger.GetLogger()
	log.Info("启动工单同步调度器")

	// 加载所有启用同步的插件
	var plugins []models.TicketPlugin
	if err := s.db.Where("sync_enabled = ? AND status = ?", true, "active").Find(&plugins).Error; err != nil {
		return fmt.Errorf("查询插件失败: %v", err)
	}

	// 为每个插件创建定时任务
	for _, plugin := range plugins {
		if err := s.schedulePlugin(&plugin); err != nil {
			log.WithError(err).Errorf("调度插件 %s 失败", plugin.Name)
			continue
		}
	}

	// 启动cron
	s.cron.Start()
	s.running = true

	log.Infof("工单同步调度器启动成功，已加载 %d 个插件任务", len(s.jobMap))
	return nil
}

// Stop 停止调度器
func (s *TicketSyncScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	log := logger.GetLogger()
	log.Info("停止工单同步调度器")

	ctx := s.cron.Stop()
	<-ctx.Done()

	s.running = false
	s.jobMap = make(map[uint]cron.EntryID)
}

// AddPlugin 添加插件的同步任务
func (s *TicketSyncScheduler) AddPlugin(pluginID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已存在，先移除
	if jobID, exists := s.jobMap[pluginID]; exists {
		s.cron.Remove(jobID)
		delete(s.jobMap, pluginID)
	}

	// 获取插件信息
	var plugin models.TicketPlugin
	if err := s.db.First(&plugin, pluginID).Error; err != nil {
		return fmt.Errorf("获取插件失败: %v", err)
	}

	// 检查是否启用
	if !plugin.SyncEnabled || plugin.Status != "active" {
		return nil
	}

	// 添加任务
	return s.schedulePlugin(&plugin)
}

// RemovePlugin 移除插件的同步任务
func (s *TicketSyncScheduler) RemovePlugin(pluginID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if jobID, exists := s.jobMap[pluginID]; exists {
		s.cron.Remove(jobID)
		delete(s.jobMap, pluginID)
		
		log := logger.GetLogger()
		log.Infof("移除插件 %d 的同步任务", pluginID)
	}
}

// UpdatePlugin 更新插件的同步任务
func (s *TicketSyncScheduler) UpdatePlugin(pluginID uint) error {
	// 先移除再添加
	s.RemovePlugin(pluginID)
	return s.AddPlugin(pluginID)
}

// TriggerSync 手动触发同步
func (s *TicketSyncScheduler) TriggerSync(pluginID uint) error {
	log := logger.GetLogger()
	log.Infof("手动触发插件 %d 的同步", pluginID)
	
	// 在新的goroutine中执行，避免阻塞
	go func() {
		if err := s.syncService.SyncTicketsForPlugin(pluginID); err != nil {
			log.WithError(err).Errorf("手动同步插件 %d 失败", pluginID)
		}
	}()
	
	return nil
}

// schedulePlugin 为插件创建定时任务
func (s *TicketSyncScheduler) schedulePlugin(plugin *models.TicketPlugin) error {
	// 构建cron表达式
	// 根据同步间隔（分钟）创建表达式
	cronExpr := fmt.Sprintf("*/%d * * * *", plugin.SyncInterval)
	
	// 创建任务
	jobID, err := s.cron.AddFunc(cronExpr, func() {
		log := logger.GetLogger()
		log.Infof("开始同步插件: %s", plugin.Name)
		
		if err := s.syncService.SyncTicketsForPlugin(plugin.ID); err != nil {
			log.WithError(err).Errorf("同步插件 %s 失败", plugin.Name)
		}
	})
	
	if err != nil {
		return fmt.Errorf("创建定时任务失败: %v", err)
	}
	
	s.jobMap[plugin.ID] = jobID
	
	log := logger.GetLogger()
	log.Infof("已为插件 %s 创建同步任务，间隔: %d 分钟", plugin.Name, plugin.SyncInterval)
	
	return nil
}

// GetScheduledPlugins 获取已调度的插件列表
func (s *TicketSyncScheduler) GetScheduledPlugins() []uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	pluginIDs := make([]uint, 0, len(s.jobMap))
	for pluginID := range s.jobMap {
		pluginIDs = append(pluginIDs, pluginID)
	}
	
	return pluginIDs
}

// IsRunning 检查调度器是否运行中
func (s *TicketSyncScheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}