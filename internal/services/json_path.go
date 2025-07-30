package services

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// JSONPath 简单的JSON路径解析器
type JSONPath struct{}

// NewJSONPath 创建JSON路径解析器
func NewJSONPath() *JSONPath {
	return &JSONPath{}
}

// Extract 从数据中提取指定路径的值
// 支持的路径格式：
// - "field" - 简单字段
// - "object.field" - 嵌套字段
// - "array[0]" - 数组索引
// - "array[*]" - 数组所有元素
// - "array[*].field" - 数组所有元素的字段
func (j *JSONPath) Extract(path string, data interface{}) interface{} {
	if path == "" || data == nil {
		return nil
	}

	// 分割路径
	parts := j.parsePath(path)
	current := data

	for _, part := range parts {
		if part.isArray {
			// 处理数组访问
			current = j.accessArray(current, part)
		} else {
			// 处理对象字段访问
			current = j.accessField(current, part.field)
		}

		if current == nil {
			return nil
		}
	}

	return current
}

// pathPart 路径片段
type pathPart struct {
	field      string
	isArray    bool
	arrayIndex int    // -1 表示 [*]
}

// parsePath 解析路径
func (j *JSONPath) parsePath(path string) []pathPart {
	var parts []pathPart
	
	// 使用正则表达式分割路径
	// 匹配模式：field[index] 或 field
	re := regexp.MustCompile(`([^.\[]+)(?:\[([0-9]+|\*)\])?`)
	
	// 按点分割
	segments := strings.Split(path, ".")
	
	for _, segment := range segments {
		matches := re.FindAllStringSubmatch(segment, -1)
		for _, match := range matches {
			part := pathPart{
				field: match[1],
			}
			
			if len(match) > 2 && match[2] != "" {
				part.isArray = true
				if match[2] == "*" {
					part.arrayIndex = -1
				} else {
					part.arrayIndex, _ = strconv.Atoi(match[2])
				}
			}
			
			parts = append(parts, part)
		}
	}
	
	return parts
}

// accessField 访问对象字段
func (j *JSONPath) accessField(data interface{}, field string) interface{} {
	if data == nil {
		return nil
	}

	// 处理 map[string]interface{} 类型
	if m, ok := data.(map[string]interface{}); ok {
		return m[field]
	}

	// 使用反射处理结构体
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	// 查找字段（支持tag）
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		fieldType := t.Field(i)
		
		// 检查字段名
		if fieldType.Name == field {
			return v.Field(i).Interface()
		}
		
		// 检查json tag
		tag := fieldType.Tag.Get("json")
		if tag != "" {
			tagName := strings.Split(tag, ",")[0]
			if tagName == field {
				return v.Field(i).Interface()
			}
		}
	}

	return nil
}

// accessArray 访问数组
func (j *JSONPath) accessArray(data interface{}, part pathPart) interface{} {
	if data == nil {
		return nil
	}

	v := reflect.ValueOf(data)
	
	// 先访问字段
	if part.field != "" && part.field != "." {
		data = j.accessField(data, part.field)
		if data == nil {
			return nil
		}
		v = reflect.ValueOf(data)
	}

	// 确保是数组或切片
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return nil
	}

	// 处理索引
	if part.arrayIndex == -1 {
		// [*] 返回所有元素
		result := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = v.Index(i).Interface()
		}
		return result
	}

	// 具体索引
	if part.arrayIndex >= 0 && part.arrayIndex < v.Len() {
		return v.Index(part.arrayIndex).Interface()
	}

	return nil
}

// ExtractMultiple 提取多个值并返回数组
func (j *JSONPath) ExtractMultiple(path string, data interface{}) []interface{} {
	result := j.Extract(path, data)
	if result == nil {
		return nil
	}

	// 如果已经是数组，直接返回
	if arr, ok := result.([]interface{}); ok {
		return arr
	}

	// 否则包装成数组
	return []interface{}{result}
}

// ExtractString 提取字符串值
func (j *JSONPath) ExtractString(path string, data interface{}) string {
	result := j.Extract(path, data)
	if result == nil {
		return ""
	}

	switch v := result.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ExtractInt 提取整数值
func (j *JSONPath) ExtractInt(path string, data interface{}) (int, bool) {
	result := j.Extract(path, data)
	if result == nil {
		return 0, false
	}

	switch v := result.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case uint:
		return int(v), true
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	}

	return 0, false
}