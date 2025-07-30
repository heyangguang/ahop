package services

import (
	"fmt"
	"reflect"
	"strings"
)

// DataTransformer 数据转换器
type DataTransformer struct {
	jsonPath *JSONPath
}

// NewDataTransformer 创建数据转换器
func NewDataTransformer() *DataTransformer {
	return &DataTransformer{
		jsonPath: NewJSONPath(),
	}
}

// Transform 执行数据转换
func (t *DataTransformer) Transform(transformation interface{}, variables map[string]interface{}) interface{} {
	switch v := transformation.(type) {
	case string:
		// 简单函数调用格式: "len(array)"
		return t.parseAndExecute(v, variables)
	case map[string]interface{}:
		// 结构化格式: {"function": "len", "args": ["array"]}
		return t.executeStructured(v, variables)
	default:
		return transformation
	}
}

// parseAndExecute 解析并执行简单函数调用
func (t *DataTransformer) parseAndExecute(expr string, variables map[string]interface{}) interface{} {
	// 简单解析 function(arg1, arg2, ...)
	if idx := strings.Index(expr, "("); idx > 0 {
		funcName := strings.TrimSpace(expr[:idx])
		argsStr := strings.TrimSuffix(strings.TrimPrefix(expr[idx:], "("), ")")
		
		// 解析参数
		args := t.parseArgs(argsStr, variables)
		
		return t.executeFunction(funcName, args, variables)
	}
	
	// 不是函数调用，尝试作为变量名
	return variables[expr]
}

// parseArgs 解析函数参数
func (t *DataTransformer) parseArgs(argsStr string, variables map[string]interface{}) []interface{} {
	if argsStr == "" {
		return nil
	}
	
	var args []interface{}
	parts := strings.Split(argsStr, ",")
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		
		// 检查是否是字符串字面量
		if strings.HasPrefix(part, "'") && strings.HasSuffix(part, "'") {
			args = append(args, strings.Trim(part, "'"))
		} else if strings.HasPrefix(part, "\"") && strings.HasSuffix(part, "\"") {
			args = append(args, strings.Trim(part, "\""))
		} else {
			// 作为变量名处理
			if val, ok := variables[part]; ok {
				args = append(args, val)
			} else {
				args = append(args, part)
			}
		}
	}
	
	return args
}

// executeStructured 执行结构化格式的转换
func (t *DataTransformer) executeStructured(config map[string]interface{}, variables map[string]interface{}) interface{} {
	funcName, _ := config["function"].(string)
	
	// 支持多种参数格式
	var args []interface{}
	
	// 1. 优先使用 input 字段（变量引用格式）
	if input, ok := config["input"].(string); ok {
		// 解析变量引用，如 {{affected_hosts}}
		resolver := NewVariableResolver()
		resolved := resolver.Resolve(input, variables)
		args = append(args, resolved)
	} else if argsConfig, ok := config["args"].([]interface{}); ok {
		// 2. 使用 args 字段（数组格式）
		for _, arg := range argsConfig {
			switch v := arg.(type) {
			case string:
				// 检查是否是变量引用
				if val, ok := variables[v]; ok {
					args = append(args, val)
				} else {
					args = append(args, v)
				}
			case map[string]interface{}:
				// 嵌套函数调用
				args = append(args, t.executeStructured(v, variables))
			default:
				args = append(args, v)
			}
		}
	}
	
	// 处理特殊函数
	switch funcName {
	case "default":
		// default 函数需要额外的默认值参数
		if defaultVal, ok := config["default"]; ok {
			args = append(args, defaultVal)
		}
	case "format":
		// format 函数使用 template 字段
		if template, ok := config["template"].(string); ok {
			resolver := NewVariableResolver()
			return resolver.Resolve(template, variables)
		}
	}
	
	return t.executeFunction(funcName, args, variables)
}

// executeFunction 执行具体的转换函数
func (t *DataTransformer) executeFunction(funcName string, args []interface{}, variables map[string]interface{}) interface{} {
	switch funcName {
	case "len", "count":
		return t.funcLen(args)
	case "join":
		return t.funcJoin(args)
	case "first":
		return t.funcFirst(args)
	case "last":
		return t.funcLast(args)
	case "toString", "str":
		return t.funcToString(args)
	case "default":
		return t.funcDefault(args)
	case "format":
		return t.funcFormat(args)
	case "contains":
		return t.funcContains(args)
	case "unique":
		return t.funcUnique(args)
	default:
		return nil
	}
}

// funcLen 获取长度
func (t *DataTransformer) funcLen(args []interface{}) interface{} {
	if len(args) < 1 {
		return 0
	}
	
	v := reflect.ValueOf(args[0])
	switch v.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
		return v.Len()
	default:
		return 0
	}
}

// funcJoin 连接数组为字符串
func (t *DataTransformer) funcJoin(args []interface{}) interface{} {
	if len(args) < 1 {
		return ""
	}
	
	separator := ","
	if len(args) >= 2 {
		separator = fmt.Sprintf("%v", args[1])
	}
	
	v := reflect.ValueOf(args[0])
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return fmt.Sprintf("%v", args[0])
	}
	
	var parts []string
	for i := 0; i < v.Len(); i++ {
		parts = append(parts, fmt.Sprintf("%v", v.Index(i).Interface()))
	}
	
	return strings.Join(parts, separator)
}

// funcFirst 获取第一个元素
func (t *DataTransformer) funcFirst(args []interface{}) interface{} {
	if len(args) < 1 {
		return nil
	}
	
	v := reflect.ValueOf(args[0])
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		if v.Len() > 0 {
			return v.Index(0).Interface()
		}
	case reflect.String:
		if v.Len() > 0 {
			return string(v.String()[0])
		}
	}
	
	return nil
}

// funcLast 获取最后一个元素
func (t *DataTransformer) funcLast(args []interface{}) interface{} {
	if len(args) < 1 {
		return nil
	}
	
	v := reflect.ValueOf(args[0])
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		if v.Len() > 0 {
			return v.Index(v.Len() - 1).Interface()
		}
	case reflect.String:
		if v.Len() > 0 {
			return string(v.String()[v.Len()-1])
		}
	}
	
	return nil
}

// funcToString 转换为字符串
func (t *DataTransformer) funcToString(args []interface{}) interface{} {
	if len(args) < 1 {
		return ""
	}
	
	return fmt.Sprintf("%v", args[0])
}

// funcDefault 提供默认值
func (t *DataTransformer) funcDefault(args []interface{}) interface{} {
	if len(args) < 2 {
		return nil
	}
	
	// 检查第一个参数是否为空
	if args[0] == nil || args[0] == "" {
		return args[1]
	}
	
	// 检查是否是空切片/数组
	v := reflect.ValueOf(args[0])
	if (v.Kind() == reflect.Slice || v.Kind() == reflect.Array) && v.Len() == 0 {
		return args[1]
	}
	
	return args[0]
}

// funcFormat 格式化字符串
func (t *DataTransformer) funcFormat(args []interface{}) interface{} {
	if len(args) < 1 {
		return ""
	}
	
	format, ok := args[0].(string)
	if !ok {
		return ""
	}
	
	// 替换 {} 占位符
	result := format
	for i := 1; i < len(args); i++ {
		placeholder := "{}"
		if idx := strings.Index(result, placeholder); idx >= 0 {
			result = result[:idx] + fmt.Sprintf("%v", args[i]) + result[idx+len(placeholder):]
		}
	}
	
	return result
}

// funcContains 判断是否包含
func (t *DataTransformer) funcContains(args []interface{}) interface{} {
	if len(args) < 2 {
		return false
	}
	
	// 字符串包含
	if str, ok := args[0].(string); ok {
		search := fmt.Sprintf("%v", args[1])
		return strings.Contains(str, search)
	}
	
	// 数组包含
	v := reflect.ValueOf(args[0])
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		for i := 0; i < v.Len(); i++ {
			if reflect.DeepEqual(v.Index(i).Interface(), args[1]) {
				return true
			}
		}
	}
	
	return false
}

// funcUnique 数组去重
func (t *DataTransformer) funcUnique(args []interface{}) interface{} {
	if len(args) < 1 {
		return nil
	}
	
	v := reflect.ValueOf(args[0])
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return args[0]
	}
	
	seen := make(map[string]bool)
	var result []interface{}
	
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i).Interface()
		key := fmt.Sprintf("%v", item)
		
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}
	
	return result
}