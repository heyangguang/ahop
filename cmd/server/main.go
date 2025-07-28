package main

import (
	"ahop/internal/database"
	"ahop/internal/router"
	"ahop/internal/services"
	"ahop/pkg/config"
	"ahop/pkg/logger"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
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

	appLogger := logger.GetLogger()
	appLogger.Info("Starting Auto Healing Platform...")

	// 初始化数据库
	if err := database.Initialize(cfg); err != nil {
		appLogger.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		// 关闭数据库连接
		if err := database.Close(); err != nil {
			appLogger.Error("Failed to close database:", err)
		}
		// 关闭Redis连接
		if err := database.CloseRedisQueue(); err != nil {
			appLogger.Error("Failed to close Redis:", err)
		}
	}()

	// 执行数据库迁移 - 在这里调用migrate
	if err := database.Migrate(); err != nil {
		appLogger.Fatalf("Failed to migrate database: %v", err)
	}

	// 执行种子数据初始化
	if err := seedData(); err != nil {
		appLogger.Fatalf("Failed to initialize seed data: %v", err)
	}

	// 设置Gin模式
	gin.SetMode(cfg.Server.Mode)

	// 启动Git同步调度器（在路由初始化前）
	gitSyncScheduler := services.NewGitSyncScheduler(database.GetDB(), database.GetRedisQueue())
	services.SetGitSyncScheduler(gitSyncScheduler)
	if err := gitSyncScheduler.Start(); err != nil {
		appLogger.Errorf("Failed to start Git sync scheduler: %v", err)
		// 不影响主服务启动
	}
	defer gitSyncScheduler.Stop()
	
	// 启动工单同步调度器（在路由初始化前）
	ticketSyncScheduler := services.NewTicketSyncScheduler(database.GetDB())
	services.SetGlobalTicketSyncScheduler(ticketSyncScheduler)
	if err := ticketSyncScheduler.Start(); err != nil {
		appLogger.Errorf("Failed to start ticket sync scheduler: %v", err)
		// 不影响主服务启动
	}
	defer ticketSyncScheduler.Stop()
	
	// 创建并启动定时任务调度器（必须在路由初始化前）
	taskService := services.NewTaskService(database.GetDB(), database.GetRedisQueue())
	taskTemplateService := services.NewTaskTemplateService(database.GetDB())
	taskScheduler := services.NewTaskSchedulerService(database.GetDB(), taskService, taskTemplateService)
	services.SetGlobalTaskScheduler(taskScheduler)
	if err := taskScheduler.Start(); err != nil {
		appLogger.Errorf("Failed to start task scheduler: %v", err)
		// 不影响主服务启动
	}
	defer taskScheduler.Stop()

	// 设置路由（在所有调度器初始化后）
	r := router.SetupRouter()

	// 启动Worker连接清理任务（每30秒执行一次）
	workerAuthService := services.NewWorkerAuthService(database.GetDB())
	cleanupTicker := time.NewTicker(30 * time.Second)
	go func() {
		for range cleanupTicker.C {
			if err := workerAuthService.CleanupTimeoutConnections(); err != nil {
				appLogger.Errorf("Failed to cleanup timeout connections: %v", err)
			}
		}
	}()
	defer cleanupTicker.Stop()

	// 启动服务器
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 启动服务
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Fatalf("Failed to start server: %v", err)
		}
	}()

	appLogger.Infof("Server started on port %s", cfg.Server.Port)

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")
	if err := server.Close(); err != nil {
		appLogger.Error("Server forced to shutdown:", err)
	}
	appLogger.Info("Server exited")
}
