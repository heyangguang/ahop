package main

import (
	"ahop/internal/models"
	"ahop/internal/services"
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
	
	// 创建测试数据
	testData := map[string]interface{}{
		"id":          "TEST-MANUAL-001",
		"title":       "手动测试工单",
		"description": "这是一个手动测试工单",
		"status":      "open",
		"priority":    "high",
		"custom_fields": map[string]interface{}{
			"affected_hosts": []string{"192.168.1.1"},
			"disk_usage":     95,
		},
	}
	
	// 序列化为 JSON
	jsonData, _ := json.Marshal(testData)
	fmt.Printf("原始 JSON 数据:\n%s\n\n", jsonData)
	
	// 创建 models.JSON
	ticketData := models.JSON(jsonData)
	
	// 创建工单对象
	ticket := &models.Ticket{
		TenantID:   1,
		PluginID:   1,
		ExternalID: "TEST-MANUAL-001",
		Title:      "手动测试工单",
		CustomData: ticketData,
		Status:     "open",
		Priority:   "high",
	}
	
	fmt.Printf("创建前 CustomData 类型: %T\n", ticket.CustomData)
	fmt.Printf("创建前 CustomData 内容: %s\n\n", ticket.CustomData)
	
	// 保存到数据库
	if err := db.Create(ticket).Error; err != nil {
		log.Fatalf("创建工单失败: %v", err)
	}
	
	fmt.Printf("工单创建成功，ID: %d\n\n", ticket.ID)
	
	// 重新查询
	var savedTicket models.Ticket
	if err := db.First(&savedTicket, ticket.ID).Error; err != nil {
		log.Fatalf("查询工单失败: %v", err)
	}
	
	fmt.Printf("查询后 CustomData 类型: %T\n", savedTicket.CustomData)
	fmt.Printf("查询后 CustomData 内容: %s\n\n", savedTicket.CustomData)
	
	// 尝试解析
	var parsedData map[string]interface{}
	if err := json.Unmarshal(savedTicket.CustomData, &parsedData); err != nil {
		fmt.Printf("解析失败: %v\n", err)
	} else {
		pretty, _ := json.MarshalIndent(parsedData, "", "  ")
		fmt.Printf("解析后内容:\n%s\n", pretty)
	}
	
	// 调用同步服务测试
	fmt.Println("\n=== 测试同步服务 ===")
	
	syncService := services.NewTicketSyncService(db)
	
	// 获取插件
	var plugin models.TicketPlugin
	if err := db.First(&plugin, 1).Error; err != nil {
		log.Fatalf("获取插件失败: %v", err)
	}
	
	// 调用映射方法
	mappedTicket, err := syncService.MapTicketFields(&plugin, ticketData, []models.FieldMapping{})
	if err != nil {
		log.Fatalf("映射字段失败: %v", err)
	}
	
	fmt.Printf("\n映射后 CustomData 类型: %T\n", mappedTicket.CustomData)
	fmt.Printf("映射后 CustomData 内容: %s\n", mappedTicket.CustomData)
}