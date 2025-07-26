package services

import (
	"bufio"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ScriptType 脚本类型
type ScriptType string

const (
	ScriptTypeShell   ScriptType = "shell"
	ScriptTypeAnsible ScriptType = "ansible"
	ScriptTypeUnknown ScriptType = "unknown"
)

// ScriptParameter 脚本参数
type ScriptParameter struct {
	Name         string      `json:"name"`
	Type         string      `json:"type"`         // string, select, multiselect, datetime, password
	Description  string      `json:"description"`
	Required     bool        `json:"required"`
	DefaultValue interface{} `json:"default_value,omitempty"`
	Options      []string    `json:"options,omitempty"` // 用于select/multiselect类型
}

// ScriptInfo 脚本信息
type ScriptInfo struct {
	Path        string             `json:"path"`
	Name        string             `json:"name"`
	Type        ScriptType         `json:"type"`
	Description string             `json:"description"`
	Parameters  []ScriptParameter  `json:"parameters"`
}

// ScriptIdentifier 脚本识别器
type ScriptIdentifier struct {
	// 正则表达式
	shellParamRegex   *regexp.Regexp
	ansibleVarRegex   *regexp.Regexp
	playbookIndicator *regexp.Regexp
}

// NewScriptIdentifier 创建脚本识别器
func NewScriptIdentifier() *ScriptIdentifier {
	return &ScriptIdentifier{
		// Shell脚本参数注释格式: # @param name type "description" [required] [default:value]
		shellParamRegex: regexp.MustCompile(`^\s*#\s*@param\s+(\w+)\s+(\w+)\s+"([^"]+)"(.*)$`),
		// Ansible变量使用: {{ var_name }}
		ansibleVarRegex: regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`),
		// Ansible playbook标识
		playbookIndicator: regexp.MustCompile(`^\s*-?\s*(hosts|tasks|vars|name):`),
	}
}

// IdentifyScript 识别脚本类型和参数
func (si *ScriptIdentifier) IdentifyScript(content string, filename string) (*ScriptInfo, error) {
	info := &ScriptInfo{
		Path:       filename,
		Name:       filepath.Base(filename),
		Parameters: make([]ScriptParameter, 0),
	}

	// 根据文件扩展名初步判断
	ext := strings.ToLower(filepath.Ext(filename))
	
	// 判断脚本类型
	scriptType := si.detectScriptType(content, ext)
	info.Type = scriptType

	// 根据类型提取参数
	switch scriptType {
	case ScriptTypeShell:
		si.extractShellParameters(content, info)
	case ScriptTypeAnsible:
		si.extractAnsibleParameters(content, info)
	default:
		return info, nil
	}

	return info, nil
}

// detectScriptType 检测脚本类型
func (si *ScriptIdentifier) detectScriptType(content string, ext string) ScriptType {
	// 检查是否是Ansible Playbook
	if ext == ".yml" || ext == ".yaml" {
		if si.playbookIndicator.MatchString(content) {
			return ScriptTypeAnsible
		}
	}

	// 检查Shell脚本
	if ext == ".sh" || strings.HasPrefix(content, "#!/bin/bash") || strings.HasPrefix(content, "#!/bin/sh") {
		return ScriptTypeShell
	}

	// 检查内容中的Ansible特征
	if strings.Contains(content, "- hosts:") || strings.Contains(content, "- name:") {
		return ScriptTypeAnsible
	}

	return ScriptTypeUnknown
}

// extractShellParameters 提取Shell脚本参数
func (si *ScriptIdentifier) extractShellParameters(content string, info *ScriptInfo) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	
	// 提取脚本描述（第一个非shebang的注释行）
	foundDescription := false
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// 跳过空行
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		// 跳过shebang
		if strings.HasPrefix(line, "#!") {
			continue
		}
		
		// 提取脚本描述
		if !foundDescription && strings.HasPrefix(strings.TrimSpace(line), "#") {
			desc := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
			if desc != "" && !strings.HasPrefix(desc, "@") {
				info.Description = desc
				foundDescription = true
			}
		}
		
		// 匹配参数定义
		if matches := si.shellParamRegex.FindStringSubmatch(line); len(matches) > 0 {
			param := ScriptParameter{
				Name:        matches[1],
				Type:        matches[2],
				Description: matches[3],
			}
			
			// 解析额外选项
			extras := strings.TrimSpace(matches[4])
			if strings.Contains(extras, "required") {
				param.Required = true
			}
			
			// 解析默认值
			if defaultMatch := regexp.MustCompile(`default:(\S+)`).FindStringSubmatch(extras); len(defaultMatch) > 1 {
				param.DefaultValue = defaultMatch[1]
			}
			
			// 解析选项（用于select类型）
			if optionsMatch := regexp.MustCompile(`options:\[([^\]]+)\]`).FindStringSubmatch(extras); len(optionsMatch) > 1 {
				options := strings.Split(optionsMatch[1], ",")
				for i, opt := range options {
					options[i] = strings.TrimSpace(opt)
				}
				param.Options = options
			}
			
			info.Parameters = append(info.Parameters, param)
		}
	}
}

// extractAnsibleParameters 提取Ansible Playbook参数
func (si *ScriptIdentifier) extractAnsibleParameters(content string, info *ScriptInfo) {
	// 查找所有变量使用
	varMap := make(map[string]bool)
	matches := si.ansibleVarRegex.FindAllStringSubmatch(content, -1)
	
	for _, match := range matches {
		if len(match) > 1 {
			varName := match[1]
			// 过滤掉一些内置变量
			if !isAnsibleBuiltinVar(varName) {
				varMap[varName] = true
			}
		}
	}
	
	// 提取vars部分的定义
	varsSection := extractVarsSection(content)
	
	// 为每个变量创建参数
	for varName := range varMap {
		param := ScriptParameter{
			Name:        varName,
			Type:        "string", // 默认类型
			Description: fmt.Sprintf("Variable %s", varName),
			Required:    true,
		}
		
		// 尝试从vars部分获取默认值和类型提示
		if defaultValue, found := getVarDefault(varsSection, varName); found {
			param.DefaultValue = defaultValue
			param.Required = false
		}
		
		// 根据变量名推测类型
		param.Type = guessParameterType(varName)
		
		info.Parameters = append(info.Parameters, param)
	}
}

// isAnsibleBuiltinVar 判断是否是Ansible内置变量
func isAnsibleBuiltinVar(varName string) bool {
	builtins := []string{
		"inventory_hostname", "ansible_hostname", "ansible_host",
		"ansible_user", "ansible_port", "ansible_connection",
		"item", "hostvars", "groups", "group_names",
	}
	
	for _, builtin := range builtins {
		if varName == builtin || strings.HasPrefix(varName, "ansible_") {
			return true
		}
	}
	
	return false
}

// extractVarsSection 提取vars部分
func extractVarsSection(content string) string {
	// 简单实现，查找vars:部分
	lines := strings.Split(content, "\n")
	inVars := false
	varsContent := ""
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "vars:" {
			inVars = true
			continue
		}
		
		if inVars {
			// 检查是否退出vars部分
			if !strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "\t") && strings.TrimSpace(line) != "" {
				break
			}
			varsContent += line + "\n"
		}
	}
	
	return varsContent
}

// getVarDefault 获取变量默认值
func getVarDefault(varsSection string, varName string) (interface{}, bool) {
	pattern := fmt.Sprintf(`\s*%s:\s*(.+)`, varName)
	re := regexp.MustCompile(pattern)
	
	if matches := re.FindStringSubmatch(varsSection); len(matches) > 1 {
		value := strings.TrimSpace(matches[1])
		// 去除引号
		value = strings.Trim(value, `"'`)
		return value, true
	}
	
	return nil, false
}

// guessParameterType 根据变量名推测参数类型
func guessParameterType(varName string) string {
	lowerName := strings.ToLower(varName)
	
	// 密码类型
	if strings.Contains(lowerName, "password") || strings.Contains(lowerName, "passwd") ||
		strings.Contains(lowerName, "secret") || strings.Contains(lowerName, "token") {
		return "password"
	}
	
	// 日期时间类型
	if strings.Contains(lowerName, "date") || strings.Contains(lowerName, "time") {
		return "datetime"
	}
	
	// 布尔类型（转换为select）
	if strings.HasPrefix(lowerName, "is_") || strings.HasPrefix(lowerName, "enable_") ||
		strings.HasPrefix(lowerName, "disable_") || strings.Contains(lowerName, "flag") {
		return "select"
	}
	
	// 端口号（数字类型，但我们用string处理）
	if strings.Contains(lowerName, "port") || strings.Contains(lowerName, "count") ||
		strings.Contains(lowerName, "num") {
		return "string"
	}
	
	return "string"
}