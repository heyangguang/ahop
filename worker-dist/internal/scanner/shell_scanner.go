package scanner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	
	"github.com/sirupsen/logrus"
)

// ShellScanner Shell 脚本扫描器
type ShellScanner struct {
	paramRegex *regexp.Regexp
	log        *logrus.Logger
}

// NewShellScanner 创建 Shell 扫描器
func NewShellScanner(log *logrus.Logger) *ShellScanner {
	return &ShellScanner{
		// 支持两种格式：
		// 1. 简单格式: @param 参数名 描述
		// 2. 完整格式: @param VAR_NAME type default "question" "description" [options]
		paramRegex: regexp.MustCompile(`^#\s*@param\s+(\w+)(?:\s+(\w+)\s+([^\s"]*)\s+"([^"]+)"(?:\s+"([^"]+)")?(?:\s+(.*))?|\s+(.*))?`),
		log:        log,
	}
}

// Scan 扫描 Shell 脚本
func (s *ShellScanner) Scan(projectPath string) ([]ScanResult, error) {
	var results []ScanResult
	
	// 查找所有 .sh 文件
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		// 跳过隐藏目录
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}
		
		// 只处理 .sh 文件
		if !strings.HasSuffix(path, ".sh") {
			return nil
		}
		
		s.log.WithField("path", path).Debug("扫描 Shell 脚本")
		
		// 解析脚本
		result, err := s.parseShellScript(path)
		if err == nil && result != nil {
			results = append(results, *result)
		}
		
		return nil
	})
	
	return results, err
}

// parseShellScript 解析 Shell 脚本
func (s *ShellScanner) parseShellScript(scriptPath string) (*ScanResult, error) {
	file, err := os.Open(scriptPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var (
		name        string
		description string
		params      []SurveyItem
	)
	
	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		
		// 停止扫描的条件
		if lineNum > 100 || (!strings.HasPrefix(line, "#") && line != "") {
			break
		}
		
		// 提取脚本名称
		if strings.HasPrefix(line, "# Name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "# Name:"))
			continue
		}
		
		// 提取脚本描述
		if strings.HasPrefix(line, "# Description:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "# Description:"))
			continue
		}
		
		// 解析 @param 注释
		if matches := s.paramRegex.FindStringSubmatch(line); len(matches) > 0 {
			param := s.parseParam(matches)
			if param != nil {
				params = append(params, *param)
			}
		}
	}
	
	// 如果没有参数定义，跳过
	if len(params) == 0 {
		s.log.WithField("path", scriptPath).Debug("脚本没有参数定义，跳过")
		return nil, fmt.Errorf("no parameters found")
	}
	
	// 如果没有名称，使用文件名
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(scriptPath), ".sh")
	}
	
	return &ScanResult{
		Type: "shell",
		Name: name,
		Path: scriptPath,
		Survey: &Survey{
			Name:        name,
			Description: description,
			Spec:        params,
		},
	}, nil
}

// parseParam 解析参数定义
func (s *ShellScanner) parseParam(matches []string) *SurveyItem {
	if len(matches) < 2 {
		return nil
	}
	
	varName := matches[1]
	
	// 检查是否是简单格式（只有参数名和描述）
	if len(matches) > 7 && matches[7] != "" {
		// 简单格式: @param 参数名 描述 或 @param 参数名 [类型] 描述
		description := strings.TrimSpace(matches[7])
		paramType := "text"
		
		// 检查是否有明确指定的类型 [类型]
		typeRegex := regexp.MustCompile(`^\[([^\]]+)\]\s*(.*)`)
		if typeMatches := typeRegex.FindStringSubmatch(description); len(typeMatches) > 2 {
			// 用户明确指定了类型
			specifiedType := strings.ToLower(strings.TrimSpace(typeMatches[1]))
			description = strings.TrimSpace(typeMatches[2])
			
			// 映射用户指定的类型
			switch specifiedType {
			case "text", "string", "str":
				paramType = "text"
			case "password", "pwd", "密码":
				paramType = "password"
			case "integer", "int", "number", "整数", "数字":
				paramType = "integer"
			case "float", "decimal", "浮点数", "小数":
				paramType = "float"
			case "select", "choice", "单选", "选择":
				paramType = "choice"
			case "multiselect", "multichoice", "多选":
				paramType = "multiselect"
			case "textarea", "multiline", "多行文本", "文本域":
				paramType = "textarea"
			default:
				// 未识别的类型，使用默认并记录警告
				s.log.WithField("type", specifiedType).Warn("未识别的参数类型，使用默认类型 text")
				paramType = "text"
			}
		} else {
			// 没有明确指定类型，使用智能推断
			if strings.Contains(strings.ToLower(description), "password") || 
			   strings.Contains(strings.ToLower(description), "密码") {
				paramType = "password"
			} else if strings.Contains(strings.ToLower(description), "number") || 
			          strings.Contains(strings.ToLower(description), "数字") ||
			          strings.Contains(strings.ToLower(description), "天数") ||
			          strings.Contains(strings.ToLower(description), "端口") {
				paramType = "integer"
			} else if (strings.Contains(strings.ToLower(description), "类型") ||
			          strings.Contains(strings.ToLower(description), "type")) &&
			          strings.Contains(description, "/") {
				paramType = "choice"
			} else if (strings.Contains(description, "（") && strings.Contains(description, "）") &&
			          strings.Contains(description, "/")) ||
			         (strings.Contains(description, "(") && strings.Contains(description, ")") &&
			          strings.Contains(description, "/")) {
				// 如果描述中有括号并且包含"/"分隔的选项，也识别为choice
				paramType = "choice"
			}
		}
		
		// 判断是否必填：只有描述末尾有 (required) 才是必填，其他默认可选
		required := false
		cleanDescription := description
		if strings.HasSuffix(strings.TrimSpace(description), "(required)") {
			required = true
			// 去掉 (required) 标记，保持描述干净
			cleanDescription = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(description), "(required)"))
		}
		
		item := &SurveyItem{
			Type:                s.mapShellTypeToSurvey(paramType),
			Variable:            varName,
			QuestionName:        cleanDescription,
			QuestionDescription: cleanDescription,
			Required:            required,
		}
		
		// 如果是选择类型，尝试从描述中提取选项
		if paramType == "choice" {
			// 支持中文括号和英文括号
			var start, end int
			var choicesStr string
			
			// 先尝试中文括号
			if strings.Contains(description, "（") && strings.Contains(description, "）") {
				start = strings.Index(description, "（")
				end = strings.Index(description, "）")
				if start != -1 && end != -1 && end > start {
					choicesStr = description[start+3:end] // +3 for UTF-8 "（"
				}
			} else if strings.Contains(description, "(") && strings.Contains(description, ")") {
				// 再尝试英文括号
				start = strings.Index(description, "(")
				end = strings.Index(description, ")")
				if start != -1 && end != -1 && end > start {
					choicesStr = description[start+1:end] // +1 for ASCII "("
				}
			}
			
			// 提取选项
			if choicesStr != "" {
				choices := strings.Split(choicesStr, "/")
				for i := range choices {
					choices[i] = strings.TrimSpace(choices[i])
				}
				item.Choices = choices
			}
		}
		
		return item
	}
	
	// 完整格式处理
	if len(matches) < 5 {
		return nil
	}
	
	paramType := matches[2]
	defaultValue := matches[3]
	questionName := matches[4]
	description := ""
	if len(matches) > 5 {
		description = matches[5]
	}
	options := ""
	if len(matches) > 6 {
		options = matches[6]
	}
	
	// 类型映射
	surveyType := s.mapShellTypeToSurvey(paramType)
	
	item := &SurveyItem{
		Type:                surveyType,
		Variable:            varName,
		QuestionName:        questionName,
		QuestionDescription: description,
		Required:            defaultValue == "required",
	}
	
	// 处理默认值
	if defaultValue != "" && defaultValue != "required" {
		item.Default = s.parseDefaultValue(surveyType, defaultValue)
	}
	
	// 解析选项
	s.parseOptions(item, options)
	
	return item
}

// mapShellTypeToSurvey 映射 Shell 类型到 Survey 类型
func (s *ShellScanner) mapShellTypeToSurvey(shellType string) string {
	typeMap := map[string]string{
		"text":        "text",
		"string":      "text",
		"password":    "password",
		"secret":      "password",
		"integer":     "integer",
		"int":         "integer",
		"number":      "integer",
		"float":       "float",
		"choice":      "multiplechoice",
		"select":      "multiplechoice",
		"multiselect": "multiselect",
		"multichoice": "multiselect",
		"textarea":    "textarea",
		"json":        "textarea",
	}
	
	if surveyType, ok := typeMap[shellType]; ok {
		return surveyType
	}
	return "text"
}

// parseDefaultValue 解析默认值
func (s *ShellScanner) parseDefaultValue(surveyType, value string) interface{} {
	switch surveyType {
	case "integer":
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	case "float":
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	case "multiselect":
		// 逗号分隔的默认值
		if value != "" {
			parts := strings.Split(value, ",")
			// 清理空格
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			return parts
		}
	}
	return value
}

// parseOptions 解析额外选项
func (s *ShellScanner) parseOptions(item *SurveyItem, options string) {
	if options == "" {
		return
	}
	
	// 解析 key=value 格式的选项
	parts := strings.Fields(options)
	for _, part := range parts {
		if strings.HasPrefix(part, "choices=") {
			choicesStr := strings.TrimPrefix(part, "choices=")
			choices := strings.Split(choicesStr, ",")
			// 清理空格
			for i := range choices {
				choices[i] = strings.TrimSpace(choices[i])
			}
			item.Choices = choices
		} else if strings.HasPrefix(part, "min=") {
			if v, err := strconv.Atoi(strings.TrimPrefix(part, "min=")); err == nil {
				item.Min = &v
			}
		} else if strings.HasPrefix(part, "max=") {
			if v, err := strconv.Atoi(strings.TrimPrefix(part, "max=")); err == nil {
				item.Max = &v
			}
		}
	}
}