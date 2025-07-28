package services

import "sync"

// 全局任务调度器实例
var (
	globalTaskScheduler     *TaskSchedulerService
	taskSchedulerMutex      sync.RWMutex
)

// SetGlobalTaskScheduler 设置全局任务调度器实例
func SetGlobalTaskScheduler(scheduler *TaskSchedulerService) {
	taskSchedulerMutex.Lock()
	defer taskSchedulerMutex.Unlock()
	globalTaskScheduler = scheduler
}

// GetGlobalTaskScheduler 获取全局任务调度器实例
func GetGlobalTaskScheduler() *TaskSchedulerService {
	taskSchedulerMutex.RLock()
	defer taskSchedulerMutex.RUnlock()
	return globalTaskScheduler
}