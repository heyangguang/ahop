package scanner

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateSurvey 验证 Survey 定义
func ValidateSurvey(survey *Survey) error {
	if survey == nil {
		return fmt.Errorf("survey is nil")
	}
	
	if survey.Name == "" {
		return fmt.Errorf("survey name is required")
	}
	
	if len(survey.Spec) == 0 {
		return fmt.Errorf("survey spec is empty")
	}
	
	// 验证每个 item
	for i, item := range survey.Spec {
		if err := ValidateSurveyItem(&item); err != nil {
			return fmt.Errorf("survey spec[%d]: %v", i, err)
		}
	}
	
	return nil
}

// ValidateSurveyItem 验证 Survey Item
func ValidateSurveyItem(item *SurveyItem) error {
	if item.Variable == "" {
		return fmt.Errorf("variable is required")
	}
	
	// 验证变量名格式
	if strings.Contains(item.Variable, " ") {
		return fmt.Errorf("variable name cannot contain spaces")
	}
	
	// 验证类型
	validTypes := map[string]bool{
		"text":           true,
		"textarea":       true,
		"password":       true,
		"integer":        true,
		"float":          true,
		"multiplechoice": true,
		"multiselect":    true,
	}
	
	if item.Type == "" {
		item.Type = "text" // 默认类型
	} else if !validTypes[item.Type] {
		return fmt.Errorf("invalid type: %s", item.Type)
	}
	
	// 验证选择类型必须有选项
	if (item.Type == "multiplechoice" || item.Type == "multiselect") && len(item.Choices) == 0 {
		return fmt.Errorf("%s type requires choices", item.Type)
	}
	
	// 验证默认值类型
	if item.Default != nil {
		if err := validateDefaultValue(item); err != nil {
			return fmt.Errorf("invalid default value: %v", err)
		}
	}
	
	return nil
}

// validateDefaultValue 验证默认值
func validateDefaultValue(item *SurveyItem) error {
	switch item.Type {
	case "integer":
		switch v := item.Default.(type) {
		case int, int32, int64, float64:
			// 数字类型都可以
		case string:
			// 尝试转换
			if _, err := fmt.Sscanf(v, "%d", new(int)); err != nil {
				return fmt.Errorf("default value must be an integer")
			}
		default:
			return fmt.Errorf("default value must be an integer")
		}
		
	case "float":
		switch v := item.Default.(type) {
		case float32, float64, int, int32, int64:
			// 数字类型都可以
		case string:
			// 尝试转换
			if _, err := fmt.Sscanf(v, "%f", new(float64)); err != nil {
				return fmt.Errorf("default value must be a float")
			}
		default:
			return fmt.Errorf("default value must be a float")
		}
		
	case "multiplechoice":
		// 必须是字符串且在选项中
		defaultStr, ok := item.Default.(string)
		if !ok {
			return fmt.Errorf("default value must be a string")
		}
		found := false
		for _, choice := range item.Choices {
			if choice == defaultStr {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("default value must be one of the choices")
		}
		
	case "multiselect":
		// 必须是字符串数组且都在选项中
		var defaultArr []string
		switch v := item.Default.(type) {
		case []string:
			defaultArr = v
		case []interface{}:
			for _, item := range v {
				if str, ok := item.(string); ok {
					defaultArr = append(defaultArr, str)
				} else {
					return fmt.Errorf("default value items must be strings")
				}
			}
		default:
			return fmt.Errorf("default value must be an array")
		}
		
		// 验证每个值都在选项中
		for _, val := range defaultArr {
			found := false
			for _, choice := range item.Choices {
				if choice == val {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("default value '%s' is not in choices", val)
			}
		}
	}
	
	return nil
}

// ConvertToJSON 将 Survey 转换为 JSON 字符串
func ConvertToJSON(survey *Survey) (string, error) {
	data, err := json.MarshalIndent(survey, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}