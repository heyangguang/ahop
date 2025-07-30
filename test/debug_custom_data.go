package main

import (
	"ahop/internal/models"
	"ahop/pkg/config"
	"ahop/pkg/database"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 连接数据库
	if err := database.InitDB(cfg.Database); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	db := database.GetDB()

	// 查询一个工单
	var ticket models.Ticket
	if err := db.First(&ticket, "custom_data IS NOT NULL").Error; err != nil {
		log.Fatalf("查询工单失败: %v", err)
	}

	fmt.Printf("工单ID: %d\n", ticket.ID)
	fmt.Printf("工单标题: %s\n", ticket.Title)
	fmt.Printf("CustomData 类型: %T\n", ticket.CustomData)
	fmt.Printf("CustomData 原始值: %s\n", string(ticket.CustomData))

	// 尝试解析为 JSON
	var data map[string]interface{}
	if err := json.Unmarshal(ticket.CustomData, &data); err != nil {
		fmt.Printf("JSON 解析失败: %v\n", err)
		
		// 尝试 base64 解码
		decoded, err := base64.StdEncoding.DecodeString(string(ticket.CustomData))
		if err != nil {
			fmt.Printf("Base64 解码失败: %v\n", err)
		} else {
			fmt.Printf("Base64 解码成功: %s\n", string(decoded))
			
			// 再次尝试 JSON 解析
			if err := json.Unmarshal(decoded, &data); err != nil {
				fmt.Printf("解码后 JSON 解析失败: %v\n", err)
			} else {
				fmt.Printf("解码后 JSON 解析成功: %+v\n", data)
			}
		}
	} else {
		fmt.Printf("JSON 解析成功: %+v\n", data)
	}

	// 直接查询数据库看原始值
	var rawData string
	row := db.Raw("SELECT custom_data FROM tickets WHERE id = ?", ticket.ID).Row()
	if err := row.Scan(&rawData); err != nil {
		fmt.Printf("查询原始数据失败: %v\n", err)
	} else {
		fmt.Printf("\n数据库原始值: %s\n", rawData)
		fmt.Printf("原始值长度: %d\n", len(rawData))
		
		// 检查是否是 base64
		if decoded, err := base64.StdEncoding.DecodeString(rawData); err == nil {
			fmt.Printf("数据库值是 base64 编码的，解码后: %s\n", string(decoded))
		}
	}
}