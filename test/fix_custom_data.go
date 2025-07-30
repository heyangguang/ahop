package main

import (
	"ahop/internal/models"
	"ahop/pkg/config"
	"encoding/json"
	"fmt"
	"log"
	
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// 加载配置
	config.LoadConfig()
	
	// 连接数据库
	cfg := config.GetConfig()
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.Port,
		cfg.Database.SSLMode,
	)
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	
	// 查询所有工单
	var tickets []models.Ticket
	if err := db.Find(&tickets).Error; err != nil {
		log.Fatalf("查询工单失败: %v", err)
	}
	
	fmt.Printf("找到 %d 个工单\n", len(tickets))
	
	fixedCount := 0
	for _, ticket := range tickets {
		// 尝试解析 custom_data
		var data map[string]interface{}
		if err := json.Unmarshal(ticket.CustomData, &data); err != nil {
			fmt.Printf("工单 %s: 解析失败，跳过\n", ticket.ExternalID)
			continue
		}
		
		// 检查是否包含了整个工单对象
		if _, hasID := data["id"]; hasID {
			if _, hasTitle := data["title"]; hasTitle {
				if customFields, hasCustomFields := data["custom_fields"]; hasCustomFields {
					fmt.Printf("工单 %s: 需要修复\n", ticket.ExternalID)
					
					// 只保留 custom_fields
					newCustomData, err := json.Marshal(customFields)
					if err != nil {
						fmt.Printf("  序列化失败: %v\n", err)
						continue
					}
					
					// 更新数据库
					if err := db.Model(&models.Ticket{}).
						Where("id = ?", ticket.ID).
						Update("custom_data", models.JSON(newCustomData)).Error; err != nil {
						fmt.Printf("  更新失败: %v\n", err)
						continue
					}
					
					fmt.Printf("  ✓ 修复成功\n")
					fixedCount++
				}
			}
		} else {
			fmt.Printf("工单 %s: 数据格式正确，无需修复\n", ticket.ExternalID)
		}
	}
	
	fmt.Printf("\n修复完成！共修复 %d 个工单\n", fixedCount)
}