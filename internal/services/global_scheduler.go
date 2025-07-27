package services

var globalTicketSyncScheduler *TicketSyncScheduler

// SetGlobalTicketSyncScheduler 设置全局工单同步调度器
func SetGlobalTicketSyncScheduler(scheduler *TicketSyncScheduler) {
	globalTicketSyncScheduler = scheduler
}

// GetGlobalTicketSyncScheduler 获取全局工单同步调度器
func GetGlobalTicketSyncScheduler() *TicketSyncScheduler {
	return globalTicketSyncScheduler
}