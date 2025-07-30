package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

func main() {
	// 获取 JWT token
	cmd := exec.Command("curl", "-s", "-X", "POST", "http://localhost:8080/api/v1/auth/login",
		"-H", "Content-Type: application/json",
		"-d", `{"username":"admin","password":"Admin@123"}`)
	
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("登录失败: %v", err)
	}
	
	var loginResp map[string]interface{}
	if err := json.Unmarshal(output, &loginResp); err != nil {
		log.Fatalf("解析登录响应失败: %v", err)
	}
	
	token := loginResp["data"].(map[string]interface{})["token"].(string)
	fmt.Printf("Token 获取成功\n\n")
	
	// 获取工单列表
	req, _ := http.NewRequest("GET", "http://localhost:8080/api/v1/tickets?page=1&page_size=3&sort=-created_at", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	var ticketsResp map[string]interface{}
	if err := json.Unmarshal(body, &ticketsResp); err != nil {
		log.Fatalf("解析响应失败: %v", err)
	}
	
	tickets := ticketsResp["data"].([]interface{})
	
	for i, t := range tickets {
		ticket := t.(map[string]interface{})
		
		fmt.Printf("=== 工单 %d ===\n", i+1)
		fmt.Printf("External ID: %v\n", ticket["external_id"])
		fmt.Printf("Title: %v\n", ticket["title"])
		
		customData := ticket["custom_data"]
		fmt.Printf("\nCustomData 信息:\n")
		fmt.Printf("类型: %T\n", customData)
		
		// 尝试转换为字符串
		switch v := customData.(type) {
		case string:
			fmt.Printf("是字符串，长度: %d\n", len(v))
			fmt.Printf("前100字符: %s\n", v[:min(100, len(v))])
			
			// 尝试 base64 解码
			decoded, err := base64.StdEncoding.DecodeString(v)
			if err == nil {
				fmt.Printf("\n✓ Base64 解码成功！\n")
				fmt.Printf("解码后内容:\n%s\n", string(decoded))
			} else {
				fmt.Printf("\n✗ 不是 base64: %v\n", err)
			}
			
		case map[string]interface{}:
			fmt.Printf("是 JSON 对象\n")
			pretty, _ := json.MarshalIndent(v, "", "  ")
			fmt.Printf("内容:\n%s\n", pretty)
			
		default:
			fmt.Printf("未知类型: %T\n", v)
		}
		
		fmt.Printf("\n%s\n\n", strings.Repeat("=", 80))
	}
	
	// 测试原始数据同步
	fmt.Println("测试从插件获取原始数据...")
	
	// 先生成一个测试工单
	genCmd := exec.Command("curl", "-s", "-X", "POST", "http://localhost:5002/generate-test-tickets",
		"-H", "Content-Type: application/json",
		"-d", `{"count": 1, "type": "disk"}`)
	
	genOutput, _ := genCmd.Output()
	fmt.Printf("生成测试工单: %s\n", genOutput)
	
	// 获取原始数据
	rawCmd := exec.Command("curl", "-s", "http://localhost:5002/tickets?minutes=5")
	rawOutput, _ := rawCmd.Output()
	
	var rawResp map[string]interface{}
	json.Unmarshal(rawOutput, &rawResp)
	
	if data, ok := rawResp["data"].([]interface{}); ok && len(data) > 0 {
		firstTicket := data[0].(map[string]interface{})
		fmt.Printf("\n原始工单数据:\n")
		pretty, _ := json.MarshalIndent(firstTicket, "", "  ")
		fmt.Printf("%s\n", pretty)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}