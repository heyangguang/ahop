package main

import (
	"ahop/internal/models"
	"ahop/pkg/config"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	
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

	// 查询最新的工单
	var tickets []models.Ticket
	if err := db.Order("created_at DESC").Limit(3).Find(&tickets).Error; err != nil {
		log.Fatalf("查询工单失败: %v", err)
	}

	for i, ticket := range tickets {
		fmt.Printf("\n=== 工单 %d ===\n", i+1)
		fmt.Printf("External ID: %s\n", ticket.ExternalID)
		fmt.Printf("Title: %s\n", ticket.Title)
		
		// 显示 custom_data 的原始值
		fmt.Printf("\nCustomData 原始值:\n")
		fmt.Printf("类型: %T\n", ticket.CustomData)
		fmt.Printf("长度: %d\n", len(ticket.CustomData))
		
		// 尝试直接显示
		fmt.Printf("直接显示: %s\n", ticket.CustomData)
		
		// 尝试 base64 解码
		fmt.Printf("\n尝试 base64 解码:\n")
		decoded, err := base64.StdEncoding.DecodeString(string(ticket.CustomData))
		if err == nil {
			fmt.Printf("✓ Base64 解码成功！\n")
			fmt.Printf("解码后: %s\n", string(decoded))
			
			// 尝试解析解码后的 JSON
			var data map[string]interface{}
			if err := json.Unmarshal(decoded, &data); err == nil {
				fmt.Printf("✓ JSON 解析成功！\n")
				pretty, _ := json.MarshalIndent(data, "", "  ")
				fmt.Printf("解析后的 JSON:\n%s\n", pretty)
			} else {
				fmt.Printf("✗ JSON 解析失败: %v\n", err)
			}
		} else {
			fmt.Printf("✗ 不是 base64 编码: %v\n", err)
			
			// 尝试直接解析为 JSON
			var data map[string]interface{}
			if err := json.Unmarshal(ticket.CustomData, &data); err == nil {
				fmt.Printf("✓ 直接 JSON 解析成功！\n")
				pretty, _ := json.MarshalIndent(data, "", "  ")
				fmt.Printf("解析后的 JSON:\n%s\n", pretty)
			} else {
				fmt.Printf("✗ 直接 JSON 解析失败: %v\n", err)
			}
		}
		
		fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	}
}