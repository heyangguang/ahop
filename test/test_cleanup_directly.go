package main

import (
	"ahop/internal/database"
	"ahop/internal/services"
	"ahop/pkg/config"
	"ahop/pkg/logger"
	"log"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	if err := logger.Initialize(cfg); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// 初始化数据库
	if err := database.Initialize(cfg); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()
	defer database.CloseRedisQueue()

	// 创建清理服务
	cleanupService := services.NewTaskCleanupService(database.GetDB(), database.GetRedisQueue())

	log.Println("开始执行任务清理...")
	
	// 执行僵尸任务清理
	if err := cleanupService.CleanupZombieTasks(); err != nil {
		log.Printf("清理僵尸任务失败: %v", err)
	} else {
		log.Println("僵尸任务清理完成")
	}

	// 执行卡住的定时任务清理
	if err := cleanupService.CleanupStuckScheduledTasks(); err != nil {
		log.Printf("清理卡住的定时任务失败: %v", err)
	} else {
		log.Println("卡住的定时任务清理完成")
	}

	log.Println("清理任务执行完成")
}