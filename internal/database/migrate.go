package database

import (
	"ahop/internal/models"
	"ahop/pkg/logger"
)

// Migrate 执行数据库迁移
func Migrate() error {
	appLogger := logger.GetLogger()
	appLogger.Info("Starting database migration...")

	// 先只迁移Tenant模型
	err := DB.AutoMigrate(
		&models.Tenant{},
		&models.User{},
		&models.Permission{},
		&models.Role{},
		&models.RolePermission{},
		&models.UserRole{},
		&models.Credential{},
		&models.CredentialUsageLog{},
		&models.Tag{},
		&models.Host{},
		&models.HostDisk{},
		&models.HostNetworkCard{},
		&models.HostGroup{},
		&models.Task{},
		&models.TaskLog{},
		&models.Worker{},
		&models.WorkerAuth{},
		&models.GitRepository{},
		&models.GitSyncLog{},
		&models.TaskTemplate{},
		&models.WorkerConnection{},
		// 工单系统集成
		&models.TicketPlugin{},
		&models.FieldMapping{},
		&models.SyncRule{},
		&models.Ticket{},
		&models.TicketSyncLog{},
		// 定时任务
		&models.ScheduledTask{},
		&models.ScheduledTaskExecution{},
	)

	if err != nil {
		appLogger.Errorf("Database migration failed: %v", err)
		return err
	}

	appLogger.Info("Database migration completed successfully")
	
	// 种子数据初始化将在 main.go 中单独调用，避免循环依赖
	
	return nil
}
