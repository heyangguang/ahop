package main

import (
	"ahop/internal/models"
	"encoding/json"
	"fmt"
)

func main() {
	// 创建测试数据
	testData := map[string]interface{}{
		"id":     "TEST-001",
		"title":  "测试工单",
		"status": "open",
		"custom_fields": map[string]interface{}{
			"affected_hosts": []string{"192.168.1.1", "192.168.1.2"},
			"disk_usage":     90,
		},
	}

	// 序列化为 JSON
	jsonData, err := json.Marshal(testData)
	if err != nil {
		fmt.Printf("序列化失败: %v\n", err)
		return
	}

	fmt.Printf("原始 JSON: %s\n", jsonData)
	fmt.Printf("JSON 长度: %d\n", len(jsonData))

	// 创建 models.JSON
	var customData models.JSON = models.JSON(jsonData)
	
	fmt.Printf("\nmodels.JSON 类型: %T\n", customData)
	fmt.Printf("models.JSON 值: %s\n", customData)

	// 测试 Value() 方法
	value, err := customData.Value()
	if err != nil {
		fmt.Printf("Value() 失败: %v\n", err)
		return
	}
	
	fmt.Printf("\nValue() 返回类型: %T\n", value)
	fmt.Printf("Value() 返回值: %v\n", value)

	// 测试 MarshalJSON
	marshaled, err := customData.MarshalJSON()
	if err != nil {
		fmt.Printf("MarshalJSON 失败: %v\n", err)
		return
	}
	
	fmt.Printf("\nMarshalJSON 结果: %s\n", marshaled)

	// 尝试解析回来
	var parsed map[string]interface{}
	if err := json.Unmarshal(marshaled, &parsed); err != nil {
		fmt.Printf("解析失败: %v\n", err)
	} else {
		fmt.Printf("解析成功: %+v\n", parsed)
	}
}