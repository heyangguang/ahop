package services

import "sync"

var (
	gitSyncSchedulerInstance *GitSyncScheduler
	gitSyncSchedulerOnce     sync.Once
)

// SetGitSyncScheduler 设置全局Git同步调度器实例
func SetGitSyncScheduler(scheduler *GitSyncScheduler) {
	gitSyncSchedulerOnce.Do(func() {
		gitSyncSchedulerInstance = scheduler
	})
}

// GetGitSyncScheduler 获取全局Git同步调度器实例
func GetGitSyncScheduler() *GitSyncScheduler {
	return gitSyncSchedulerInstance
}