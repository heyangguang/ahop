package main

import (
	"ahop/internal/database"
	"ahop/internal/models"
	"ahop/pkg/config"
	"ahop/pkg/logger"
	"fmt"
	"log"
	"time"
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

	// 查询自愈规则
	var rule models.HealingRule
	if err := db.First(&rule, 12).Error; err != nil {
		log.Fatalf("Failed to get rule: %v", err)
	}

	fmt.Printf("规则信息：\n")
	fmt.Printf("ID: %d\n", rule.ID)
	fmt.Printf("Name: %s\n", rule.Name)
	fmt.Printf("CronExpr: %s\n", rule.CronExpr)
	fmt.Printf("IsActive: %v\n", rule.IsActive)
	fmt.Printf("NextRunAt: %v\n", rule.NextRunAt)

	// 手动更新 next_run_at
	nextRun := time.Now().Add(30 * time.Minute)
	if err := db.Model(&rule).Update("next_run_at", nextRun).Error; err != nil {
		log.Printf("更新失败: %v", err)
	} else {
		log.Printf("更新成功，新的 NextRunAt: %v", nextRun)
	}

	// 重新查询验证
	var updatedRule models.HealingRule
	if err := db.First(&updatedRule, 12).Error; err != nil {
		log.Fatalf("Failed to get updated rule: %v", err)
	}
	fmt.Printf("\n更新后的 NextRunAt: %v\n", updatedRule.NextRunAt)
}