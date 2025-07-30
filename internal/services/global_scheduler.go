package services

var (
	globalTicketSyncScheduler *TicketSyncScheduler
	globalHealingScheduler    *HealingScheduler
	globalGitSyncScheduler    *GitSyncScheduler
	globalTaskScheduler       *TaskSchedulerService
)

// SetGlobalTicketSyncScheduler 设置全局工单同步调度器
func SetGlobalTicketSyncScheduler(scheduler *TicketSyncScheduler) {
	globalTicketSyncScheduler = scheduler
}

// GetGlobalTicketSyncScheduler 获取全局工单同步调度器
func GetGlobalTicketSyncScheduler() *TicketSyncScheduler {
	return globalTicketSyncScheduler
}

// SetGlobalHealingScheduler 设置全局自愈调度器
func SetGlobalHealingScheduler(scheduler *HealingScheduler) {
	globalHealingScheduler = scheduler
}

// GetGlobalHealingScheduler 获取全局自愈调度器
func GetGlobalHealingScheduler() *HealingScheduler {
	return globalHealingScheduler
}

// SetGlobalGitSyncScheduler 设置全局Git同步调度器
func SetGlobalGitSyncScheduler(scheduler *GitSyncScheduler) {
	globalGitSyncScheduler = scheduler
}

// GetGlobalGitSyncScheduler 获取全局Git同步调度器
func GetGlobalGitSyncScheduler() *GitSyncScheduler {
	return globalGitSyncScheduler
}

// SetGlobalTaskScheduler 设置全局定时任务调度器
func SetGlobalTaskScheduler(scheduler *TaskSchedulerService) {
	globalTaskScheduler = scheduler
}

// GetGlobalTaskScheduler 获取全局定时任务调度器
func GetGlobalTaskScheduler() *TaskSchedulerService {
	return globalTaskScheduler
}