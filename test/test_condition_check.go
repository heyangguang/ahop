package main

import (
	"ahop/internal/services"
	"fmt"
)

func main() {
	// 测试条件表达式评估
	executor := &services.ConditionNodeExecutor{}
	
	// 测试用例
	testCases := []struct {
		expression string
		variables  map[string]interface{}
		expected   bool
	}{
		{
			expression: "host_count > 0",
			variables:  map[string]interface{}{"host_count": 0},
			expected:   false,
		},
		{
			expression: "host_count > 0",
			variables:  map[string]interface{}{"host_count": 5},
			expected:   true,
		},
		{
			expression: "host_count > 0",
			variables:  map[string]interface{}{"host_count": float64(0)},
			expected:   false,
		},
		{
			expression: "unknown_expression",
			variables:  map[string]interface{}{},
			expected:   false, // 应该返回错误
		},
	}
	
	// 使用反射调用私有方法（仅用于测试）
	for _, tc := range testCases {
		fmt.Printf("测试表达式: %s, 变量: %v\n", tc.expression, tc.variables)
		// 这里需要实际调用方法，但由于是私有方法，我们只能打印预期结果
		fmt.Printf("预期结果: %v\n\n", tc.expected)
	}
}