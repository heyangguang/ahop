package services

import (
	"ahop/internal/models"
	"ahop/pkg/logger"
	"ahop/pkg/queue"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// GitSyncScheduler Git仓库同步调度器
type GitSyncScheduler struct {
	db       *gorm.DB
	queue    *queue.RedisQueue
	cron     *cron.Cron
	jobs     map[uint]cron.EntryID // repositoryID -> cronJobID
	jobsLock sync.RWMutex
	running  bool
}

// NewGitSyncScheduler 创建Git同步调度器
func NewGitSyncScheduler(db *gorm.DB, queue *queue.RedisQueue) *GitSyncScheduler {
	return &GitSyncScheduler{
		db:    db,
		queue: queue,
		cron:  cron.New(),
		jobs:  make(map[uint]cron.EntryID),
	}
}

// Start 启动调度器
func (s *GitSyncScheduler) Start() error {
	if s.running {
		return fmt.Errorf("调度器已经在运行")
	}

	logger.GetLogger().Info("启动Git仓库同步调度器")

	// 加载所有启用了定时同步的仓库
	var repositories []models.GitRepository
	err := s.db.Where("sync_enabled = ? AND sync_cron IS NOT NULL AND sync_cron != ''", true).Find(&repositories).Error
	if err != nil {
		return fmt.Errorf("加载仓库列表失败: %v", err)
	}

	// 为每个仓库创建定时任务
	for _, repo := range repositories {
		if err := s.AddJob(&repo); err != nil {
			logger.GetLogger().Errorf("添加仓库 %s (ID: %d) 的定时任务失败: %v", repo.Name, repo.ID, err)
		}
	}

	// 启动cron调度器
	s.cron.Start()
	s.running = true

	logger.GetLogger().Infof("Git仓库同步调度器启动成功，已加载 %d 个定时任务", len(s.jobs))
	return nil
}

// Stop 停止调度器
func (s *GitSyncScheduler) Stop() {
	if !s.running {
		return
	}

	logger.GetLogger().Info("停止Git仓库同步调度器")
	s.cron.Stop()
	s.running = false
}

// AddJob 添加定时任务
func (s *GitSyncScheduler) AddJob(repo *models.GitRepository) error {
	if !repo.SyncEnabled || repo.SyncCron == "" {
		return fmt.Errorf("仓库未启用定时同步或未设置cron表达式")
	}

	// 验证cron表达式
	if !isValidCron(repo.SyncCron) {
		return fmt.Errorf("无效的cron表达式: %s", repo.SyncCron)
	}

	// 创建任务函数
	jobFunc := func() {
		s.executeSyncJob(repo)
	}

	// 添加到cron调度器
	entryID, err := s.cron.AddFunc(repo.SyncCron, jobFunc)
	if err != nil {
		return fmt.Errorf("添加定时任务失败: %v", err)
	}

	// 记录任务ID
	s.jobsLock.Lock()
	s.jobs[repo.ID] = entryID
	s.jobsLock.Unlock()
	
	// 更新下次执行时间
	if entry := s.cron.Entry(entryID); entry.ID != 0 {
		nextRun := entry.Next
		s.db.Model(&models.GitRepository{}).Where("id = ?", repo.ID).Update("next_run_at", nextRun)
	}

	logger.GetLogger().Infof("已添加Git仓库 %s (ID: %d) 的定时同步任务，cron: %s", repo.Name, repo.ID, repo.SyncCron)
	return nil
}

// RemoveJob 移除定时任务
func (s *GitSyncScheduler) RemoveJob(repositoryID uint) {
	s.jobsLock.Lock()
	defer s.jobsLock.Unlock()

	if entryID, exists := s.jobs[repositoryID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobs, repositoryID)
		logger.GetLogger().Infof("已移除Git仓库 ID: %d 的定时同步任务", repositoryID)
	}
}

// UpdateJob 更新定时任务
func (s *GitSyncScheduler) UpdateJob(repo *models.GitRepository) error {
	// 先移除旧任务
	s.RemoveJob(repo.ID)

	// 如果仍然启用定时同步，添加新任务
	if repo.SyncEnabled && repo.SyncCron != "" {
		return s.AddJob(repo)
	}

	return nil
}

// executeSyncJob 执行同步任务
func (s *GitSyncScheduler) executeSyncJob(repo *models.GitRepository) {
	logger.GetLogger().Infof("开始执行Git仓库 %s (ID: %d) 的定时同步", repo.Name, repo.ID)

	// 重新加载仓库信息（确保使用最新数据）
	var currentRepo models.GitRepository
	if err := s.db.First(&currentRepo, repo.ID).Error; err != nil {
		logger.GetLogger().Errorf("加载仓库信息失败: %v", err)
		return
	}

	// 检查是否仍然启用定时同步
	if !currentRepo.SyncEnabled {
		logger.GetLogger().Warnf("仓库 %s (ID: %d) 已禁用定时同步", currentRepo.Name, currentRepo.ID)
		return
	}

	// 构建同步消息
	msg := struct {
		Action       string    `json:"action"`
		RepositoryID uint      `json:"repository_id"`
		TenantID     uint      `json:"tenant_id"`
		Repository   struct {
			ID           uint   `json:"id"`
			Name         string `json:"name"`
			URL          string `json:"url"`
			Branch       string `json:"branch"`
			IsPublic     bool   `json:"is_public"`
			CredentialID *uint  `json:"credential_id,omitempty"`
		} `json:"repository"`
		OperatorID *uint     `json:"operator_id,omitempty"`
		Timestamp  time.Time `json:"timestamp"`
	}{
		Action:       "sync",
		RepositoryID: currentRepo.ID,
		TenantID:     currentRepo.TenantID,
		Repository: struct {
			ID           uint   `json:"id"`
			Name         string `json:"name"`
			URL          string `json:"url"`
			Branch       string `json:"branch"`
			IsPublic     bool   `json:"is_public"`
			CredentialID *uint  `json:"credential_id,omitempty"`
		}{
			ID:           currentRepo.ID,
			Name:         currentRepo.Name,
			URL:          currentRepo.URL,
			Branch:       currentRepo.Branch,
			IsPublic:     currentRepo.IsPublic,
			CredentialID: currentRepo.CredentialID,
		},
		Timestamp: time.Now(),
	}

	// 发布同步通知到Redis订阅通道
	channel := fmt.Sprintf("git:sync:%d", currentRepo.ID)
	msgBytes, _ := json.Marshal(msg)
	ctx := context.Background()
	if err := s.queue.GetClient().Publish(ctx, channel, msgBytes).Err(); err != nil {
		logger.GetLogger().Errorf("发布同步通知失败: %v", err)
		
		// 记录失败日志
		now := time.Now()
		syncLog := &models.GitSyncLog{
			RepositoryID: currentRepo.ID,
			TenantID:     currentRepo.TenantID,
			TaskType:     "scheduled",
			WorkerID:     "scheduler",
			Status:       "failed",
			StartedAt:    now,
			FinishedAt:   &now,
			ErrorMessage: fmt.Sprintf("发布同步通知失败: %v", err),
		}
		s.db.Create(syncLog)
		return
	}

	// 记录调度日志
	syncLog := &models.GitSyncLog{
		RepositoryID: currentRepo.ID,
		TenantID:     currentRepo.TenantID,
		TaskType:     "scheduled",
		WorkerID:     "scheduler",
		Status:       "pending",
		StartedAt:    time.Now(),
	}
	s.db.Create(syncLog)
	
	// 更新下次执行时间和最后调度时间
	s.jobsLock.RLock()
	if entryID, exists := s.jobs[currentRepo.ID]; exists {
		if entry := s.cron.Entry(entryID); entry.ID != 0 {
			now := time.Now()
			s.db.Model(&models.GitRepository{}).Where("id = ?", currentRepo.ID).Updates(map[string]interface{}{
				"last_scheduled_at": now,
				"next_run_at": entry.Next,
			})
		}
	}
	s.jobsLock.RUnlock()

	logger.GetLogger().Infof("已发布Git仓库 %s (ID: %d) 的定时同步任务", currentRepo.Name, currentRepo.ID)
}

// GetJobStatus 获取任务状态
func (s *GitSyncScheduler) GetJobStatus() map[string]interface{} {
	s.jobsLock.RLock()
	defer s.jobsLock.RUnlock()

	entries := s.cron.Entries()
	jobs := make([]map[string]interface{}, 0)

	for repoID, entryID := range s.jobs {
		for _, entry := range entries {
			if entry.ID == entryID {
				jobs = append(jobs, map[string]interface{}{
					"repository_id": repoID,
					"next_run":      entry.Next,
					"prev_run":      entry.Prev,
				})
				break
			}
		}
	}

	return map[string]interface{}{
		"running":     s.running,
		"total_jobs":  len(s.jobs),
		"jobs":        jobs,
		"current_time": time.Now(),
	}
}

// 验证cron表达式是否有效
func isValidCron(cronExpr string) bool {
	// 支持标准cron格式和一些预定义的表达式
	predefined := map[string]bool{
		"@yearly":   true,
		"@annually": true,
		"@monthly":  true,
		"@weekly":   true,
		"@daily":    true,
		"@midnight": true,
		"@hourly":   true,
	}

	if predefined[cronExpr] {
		return true
	}

	// 简单验证：检查是否包含5个或6个字段
	fields := strings.Fields(cronExpr)
	return len(fields) == 5 || len(fields) == 6
}