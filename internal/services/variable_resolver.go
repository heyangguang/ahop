package services

import (
	"fmt"
	"regexp"
	"strings"
)

// VariableResolver 变量解析器
type VariableResolver struct {
	jsonPath *JSONPath
}

// NewVariableResolver 创建变量解析器
func NewVariableResolver() *VariableResolver {
	return &VariableResolver{
		jsonPath: NewJSONPath(),
	}
}

// Resolve 解析变量表达式
// 支持的格式：
// - {{variable_name}} - 简单变量
// - {{global_context.ticket.id}} - 嵌套路径
// - {{hosts[0]}} - 数组索引
// - {{service_name|default:unknown}} - 默认值
func (r *VariableResolver) Resolve(expr interface{}, context map[string]interface{}) interface{} {
	// 如果不是字符串，直接返回
	strExpr, ok := expr.(string)
	if !ok {
		return expr
	}
	
	if strExpr == "" {
		return strExpr
	}

	// 如果不包含变量引用，直接返回
	if !strings.Contains(strExpr, "{{") {
		return strExpr
	}

	// 查找所有变量引用
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	matches := re.FindAllStringSubmatch(strExpr, -1)

	// 如果只有一个变量引用且占据整个字符串，返回解析后的值
	if len(matches) == 1 && strExpr == matches[0][0] {
		return r.resolveVariable(matches[0][1], context)
	}

	// 否则进行字符串替换
	result := strExpr
	for _, match := range matches {
		fullMatch := match[0]
		varExpr := match[1]
		
		value := r.resolveVariable(varExpr, context)
		// 转换为字符串进行替换
		strValue := toString(value)
		result = strings.Replace(result, fullMatch, strValue, 1)
	}

	return result
}

// ResolveAll 解析所有变量
func (r *VariableResolver) ResolveAll(data interface{}, context map[string]interface{}) interface{} {
	switch v := data.(type) {
	case string:
		return r.Resolve(v, context)
	case map[string]interface{}:
		// 递归解析map
		resolved := make(map[string]interface{})
		for k, val := range v {
			resolved[k] = r.ResolveAll(val, context)
		}
		return resolved
	case []interface{}:
		// 递归解析数组
		resolved := make([]interface{}, len(v))
		for i, val := range v {
			resolved[i] = r.ResolveAll(val, context)
		}
		return resolved
	default:
		return v
	}
}

// resolveVariable 解析单个变量
func (r *VariableResolver) resolveVariable(varExpr string, context map[string]interface{}) interface{} {
	varExpr = strings.TrimSpace(varExpr)

	// 处理默认值语法: variable|default:value
	parts := strings.SplitN(varExpr, "|", 2)
	varPath := parts[0]
	var defaultValue interface{}
	hasDefault := false

	if len(parts) > 1 {
		defaultPart := strings.TrimSpace(parts[1])
		if strings.HasPrefix(defaultPart, "default:") {
			hasDefault = true
			defaultStr := strings.TrimPrefix(defaultPart, "default:")
			defaultStr = strings.TrimSpace(defaultStr)
			
			// 去除引号
			if (strings.HasPrefix(defaultStr, "'") && strings.HasSuffix(defaultStr, "'")) ||
			   (strings.HasPrefix(defaultStr, "\"") && strings.HasSuffix(defaultStr, "\"")) {
				defaultValue = defaultStr[1 : len(defaultStr)-1]
			} else {
				defaultValue = defaultStr
			}
		}
	}

	// 使用 JSONPath 解析变量路径
	value := r.jsonPath.Extract(varPath, context)
	
	// 如果值为空且有默认值，返回默认值
	if value == nil && hasDefault {
		return defaultValue
	}

	return value
}

// toString 将值转换为字符串
func toString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case []string:
		return strings.Join(v, ",")
	case []interface{}:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = toString(item)
		}
		return strings.Join(parts, ",")
	default:
		return strings.TrimSpace(strings.Trim(toJSON(value), "\""))
	}
}

// toJSON 简单的JSON序列化
func toJSON(v interface{}) string {
	// 使用 fmt.Sprintf 作为简单的序列化方案
	return fmt.Sprintf("%v", v)
}