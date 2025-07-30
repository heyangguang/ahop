package services

import (
	"ahop/internal/models"
	"time"
	"gorm.io/gorm"
)

// SchedulerMonitor 统一调度器监控服务
type SchedulerMonitor struct {
	db *gorm.DB
}

// NewSchedulerMonitor 创建调度器监控服务
func NewSchedulerMonitor(db *gorm.DB) *SchedulerMonitor {
	return &SchedulerMonitor{
		db: db,
	}
}

// SchedulerInfo 调度器信息
type SchedulerInfo struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Running     bool                   `json:"running"`
	TotalJobs   int                    `json:"total_jobs"`
	ActiveJobs  int                    `json:"active_jobs"`
	NextJobs    []NextJobInfo          `json:"next_jobs"`
	Details     map[string]interface{} `json:"details"`
}

// NextJobInfo 下次执行任务信息
type NextJobInfo struct {
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	NextRunAt   time.Time  `json:"next_run_at"`
	NextRunIn   string     `json:"next_run_in"`
}

// GetAllSchedulersStatus 获取所有调度器状态
func (m *SchedulerMonitor) GetAllSchedulersStatus() map[string]interface{} {
	result := map[string]interface{}{
		"timestamp": time.Now(),
		"schedulers": []SchedulerInfo{},
	}
	
	schedulers := []SchedulerInfo{}
	
	// 1. 定时任务调度器
	if scheduler := GetGlobalTaskScheduler(); scheduler != nil {
		stats, _ := scheduler.GetSchedulerStatistics()
		info := SchedulerInfo{
			Name:       "定时任务调度器",
			Type:       "scheduled_tasks",
			Running:    stats.SchedulerStatus.Running,
			TotalJobs:  stats.SchedulerStatus.JobsCount,
			ActiveJobs: int(stats.TaskOverview.ActiveTasks),
			NextJobs:   []NextJobInfo{},
			Details:    map[string]interface{}{
				"uptime": stats.SchedulerStatus.Uptime,
				"stats":  stats,
			},
		}
		
		// 获取下次执行的任务
		var tasks []models.ScheduledTask
		m.db.Where("is_active = ? AND next_run_at IS NOT NULL", true).
			Order("next_run_at ASC").
			Limit(5).
			Find(&tasks)
			
		for _, task := range tasks {
			if task.NextRunAt != nil {
				duration := time.Until(*task.NextRunAt)
				info.NextJobs = append(info.NextJobs, NextJobInfo{
					ID:        task.ID,
					Name:      task.Name,
					NextRunAt: *task.NextRunAt,
					NextRunIn: formatDuration(duration),
				})
			}
		}
		
		schedulers = append(schedulers, info)
	}
	
	// 2. 自愈规则调度器
	if scheduler := GetGlobalHealingScheduler(); scheduler != nil {
		status := scheduler.GetSchedulerStatus()
		running := status["running"].(bool)
		rulesCount := status["rules_count"].(int)
		
		info := SchedulerInfo{
			Name:       "自愈规则调度器",
			Type:       "healing_rules",
			Running:    running,
			TotalJobs:  rulesCount,
			ActiveJobs: rulesCount,
			NextJobs:   []NextJobInfo{},
			Details:    status,
		}
		
		// 获取下次执行的规则
		var rules []models.HealingRule
		m.db.Where("is_active = ? AND trigger_type = ? AND next_run_at IS NOT NULL", true, "scheduled").
			Order("next_run_at ASC").
			Limit(5).
			Find(&rules)
			
		for _, rule := range rules {
			if rule.NextRunAt != nil {
				duration := time.Until(*rule.NextRunAt)
				info.NextJobs = append(info.NextJobs, NextJobInfo{
					ID:        rule.ID,
					Name:      rule.Name,
					NextRunAt: *rule.NextRunAt,
					NextRunIn: formatDuration(duration),
				})
			}
		}
		
		schedulers = append(schedulers, info)
	}
	
	// 3. Git同步调度器
	if scheduler := GetGlobalGitSyncScheduler(); scheduler != nil {
		status := scheduler.GetJobStatus()
		running := status["running"].(bool)
		totalJobs := status["total_jobs"].(int)
		
		info := SchedulerInfo{
			Name:       "Git同步调度器",
			Type:       "git_sync",
			Running:    running,
			TotalJobs:  totalJobs,
			ActiveJobs: totalJobs,
			NextJobs:   []NextJobInfo{},
			Details:    status,
		}
		
		// 获取下次执行的仓库
		var repos []models.GitRepository
		m.db.Where("sync_enabled = ? AND sync_cron IS NOT NULL AND sync_cron != '' AND next_run_at IS NOT NULL", true).
			Order("next_run_at ASC").
			Limit(5).
			Find(&repos)
			
		for _, repo := range repos {
			if repo.NextRunAt != nil {
				duration := time.Until(*repo.NextRunAt)
				info.NextJobs = append(info.NextJobs, NextJobInfo{
					ID:        repo.ID,
					Name:      repo.Name,
					NextRunAt: *repo.NextRunAt,
					NextRunIn: formatDuration(duration),
				})
			}
		}
		
		schedulers = append(schedulers, info)
	}
	
	// 4. 工单同步调度器
	if scheduler := GetGlobalTicketSyncScheduler(); scheduler != nil {
		scheduledPlugins := scheduler.GetScheduledPlugins()
		
		info := SchedulerInfo{
			Name:       "工单同步调度器",
			Type:       "ticket_sync",
			Running:    scheduler.IsRunning(),
			TotalJobs:  len(scheduledPlugins),
			ActiveJobs: len(scheduledPlugins),
			NextJobs:   []NextJobInfo{},
			Details: map[string]interface{}{
				"scheduled_plugins": scheduledPlugins,
			},
		}
		
		// 获取下次执行的插件
		var plugins []models.TicketPlugin
		m.db.Where("sync_enabled = ? AND status = ? AND next_run_at IS NOT NULL", true, "active").
			Order("next_run_at ASC").
			Limit(5).
			Find(&plugins)
			
		for _, plugin := range plugins {
			if plugin.NextRunAt != nil {
				duration := time.Until(*plugin.NextRunAt)
				info.NextJobs = append(info.NextJobs, NextJobInfo{
					ID:        plugin.ID,
					Name:      plugin.Name,
					NextRunAt: *plugin.NextRunAt,
					NextRunIn: formatDuration(duration),
				})
			}
		}
		
		schedulers = append(schedulers, info)
	}
	
	result["schedulers"] = schedulers
	result["total_schedulers"] = len(schedulers)
	
	// 统计总体情况
	totalJobs := 0
	activeJobs := 0
	runningCount := 0
	
	for _, s := range schedulers {
		totalJobs += s.TotalJobs
		activeJobs += s.ActiveJobs
		if s.Running {
			runningCount++
		}
	}
	
	result["summary"] = map[string]interface{}{
		"total_jobs": totalJobs,
		"active_jobs": activeJobs,
		"running_schedulers": runningCount,
		"total_schedulers": len(schedulers),
	}
	
	return result
}