package main

import (
	"ahop/internal/database"
	"ahop/internal/models"
	"ahop/pkg/config"
	"ahop/pkg/logger"
	"fmt"
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

	db := database.GetDB()

	// 查询租户数据
	var tenant models.Tenant
	if err := db.First(&tenant, 1).Error; err != nil {
		log.Fatalf("Failed to get tenant: %v", err)
	}
	fmt.Printf("租户信息：ID=%d, Name=%s, Code=%s\n", tenant.ID, tenant.Name, tenant.Code)

	// 查询自愈规则，预加载Tenant
	var rule models.HealingRule
	if err := db.Preload("Tenant").First(&rule, 12).Error; err != nil {
		log.Fatalf("Failed to get rule: %v", err)
	}
	fmt.Printf("\n规则 Tenant 信息：ID=%d, Name=%s, Code=%s\n", rule.Tenant.ID, rule.Tenant.Name, rule.Tenant.Code)

	// 查询工作流，预加载Tenant
	var workflow models.HealingWorkflow
	if err := db.Preload("Tenant").First(&workflow, 23).Error; err != nil {
		log.Fatalf("Failed to get workflow: %v", err)
	}
	fmt.Printf("\n工作流 Tenant 信息：ID=%d, Name=%s, Code=%s\n", workflow.Tenant.ID, workflow.Tenant.Name, workflow.Tenant.Code)
}