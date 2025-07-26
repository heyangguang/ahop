package executor

import (
	"ahop-worker/internal/scanner"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	
	"github.com/sirupsen/logrus"
)

// TemplateType 模板类型
type TemplateType string

const (
	TemplateTypeShell           TemplateType = "shell"
	TemplateTypeAnsibleRole     TemplateType = "ansible_role"
	TemplateTypeAnsibleProject  TemplateType = "ansible_project"
	TemplateTypeAnsiblePlaybook TemplateType = "ansible_playbook"
)

// TemplateInfo 模板信息（用于上报给Server创建任务模板）
type TemplateInfo struct {
	Path         string               `json:"path"`         // 脚本路径
	Name         string               `json:"name"`         // 模板名称
	Code         string               `json:"code"`         // 模板编码（唯一标识）
	ScriptType   string               `json:"script_type"`  // shell/ansible_role/ansible_project/ansible_playbook
	Description  string               `json:"description"`
	Parameters   []TemplateParameter  `json:"parameters"`
	Content      string               `json:"content"`      // 脚本内容
	
	// 扩展信息
	TemplateType TemplateType         `json:"template_type"`
	EntryPoint   string               `json:"entry_point"`  // 执行入口（文件或role名）
	Components   []string             `json:"components"`   // 包含的组件（对项目而言）
	Context      string               `json:"context"`      // standalone/part_of_project
}


// ScannerAdapter 适配新的扫描器到旧的接口
type ScannerAdapter struct {
	scanner *scanner.Scanner
	log     *logrus.Logger
}

// NewScannerAdapter 创建扫描器适配器
func NewScannerAdapter() *ScannerAdapter {
	log := logrus.StandardLogger()
	return &ScannerAdapter{
		scanner: scanner.NewScanner(log),
		log:     log,
	}
}

// ScanRepository 扫描仓库（兼容旧接口）
func (a *ScannerAdapter) ScanRepository(repoPath string) ([]*TemplateInfo, error) {
	// 使用新扫描器
	results, err := a.scanner.ScanProject(repoPath)
	if err != nil {
		return nil, err
	}
	
	// 转换结果格式
	var templates []*TemplateInfo
	for _, result := range results {
		template := a.convertToTemplateInfo(result, repoPath)
		if template != nil {
			templates = append(templates, template)
		}
	}
	
	return templates, nil
}

// convertToTemplateInfo 转换扫描结果到旧格式
func (a *ScannerAdapter) convertToTemplateInfo(result scanner.ScanResult, repoPath string) *TemplateInfo {
	// 生成唯一代码
	relPath, _ := filepath.Rel(repoPath, result.Path)
	code := strings.ReplaceAll(relPath, string(filepath.Separator), "_")
	code = strings.TrimSuffix(code, filepath.Ext(code))
	
	// 转换类型
	var templateType TemplateType
	var scriptType string
	
	switch result.Type {
	case "ansible":
		templateType = TemplateTypeAnsiblePlaybook
		scriptType = "ansible_playbook"
	case "shell":
		templateType = TemplateTypeShell
		scriptType = "shell"
	default:
		return nil
	}
	
	// 转换参数
	parameters := a.convertParameters(result.Survey)
	
	// 读取文件内容
	content := ""
	if data, err := ioutil.ReadFile(result.Path); err == nil {
		content = string(data)
	} else {
		a.log.WithError(err).WithField("path", result.Path).Warn("读取脚本文件失败")
	}
	
	return &TemplateInfo{
		Path:         result.Path,
		Name:         result.Name,
		Code:         code,
		ScriptType:   scriptType,
		Description:  result.Survey.Description,
		Parameters:   parameters,
		Content:      content,
		TemplateType: templateType,
		EntryPoint:   result.Path,
		Context:      "standalone",
	}
}

// convertParameters 转换 Survey 参数到旧格式
func (a *ScannerAdapter) convertParameters(survey *scanner.Survey) []TemplateParameter {
	if survey == nil || len(survey.Spec) == 0 {
		return nil
	}
	
	var params []TemplateParameter
	
	for _, item := range survey.Spec {
		param := TemplateParameter{
			Name:         item.Variable,
			Description:  item.QuestionDescription,
			Required:     item.Required,
		}
		
		// 处理默认值
		if item.Default != nil {
			param.Default = fmt.Sprintf("%v", item.Default)
		}
		
		// 转换类型
		switch item.Type {
		case "text", "textarea", "password":
			param.Type = "string"
		case "integer":
			param.Type = "integer"
		case "float":
			param.Type = "float"
		case "multiplechoice":
			param.Type = "string"
			param.Options = item.Choices
		case "multiselect":
			param.Type = "multiselect"
			param.Options = item.Choices
		default:
			param.Type = "string"
		}
		
		
		params = append(params, param)
	}
	
	return params
}