package executor

import (
	"fmt"
	
	"ahop-worker/internal/scanner"

	"github.com/sirupsen/logrus"
)

// EnhancedScanResult 增强的扫描结果（包含文件树）
type EnhancedScanResult struct {
	// Survey文件列表
	Surveys []SurveyFile `json:"surveys"`
	
	// 相关文件树（只包含相关文件）
	FileTree *scanner.RelevantFileNode `json:"file_tree"`
	
	// 仓库信息
	Repository struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"repository"`
	
	// 统计信息
	Stats struct {
		AnsibleFiles  int `json:"ansible_files"`  // .yml/.yaml文件数
		TemplateFiles int `json:"template_files"` // .j2文件数
		ShellFiles    int `json:"shell_files"`    // .sh文件数
		SurveyFiles   int `json:"survey_files"`   // survey文件数
		TotalFiles    int `json:"total_files"`    // 相关文件总数
	} `json:"stats"`
}

// SurveyFile Survey文件信息
type SurveyFile struct {
	Path        string                 `json:"path"`        // survey文件相对路径
	Name        string                 `json:"name"`        // survey名称
	Description string                 `json:"description"` // survey描述
	Parameters  []TemplateParameter    `json:"parameters"`  // 参数列表
}

// TemplateParameter 模板参数（兼容前端）
type TemplateParameter struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	MinValue    *string  `json:"min_value,omitempty"`
	MaxValue    *string  `json:"max_value,omitempty"`
	MinLength   *int     `json:"min_length,omitempty"`
	MaxLength   *int     `json:"max_length,omitempty"`
	Source      string   `json:"source,omitempty"`
}

// EnhancedScannerAdapter 增强的扫描适配器
type EnhancedScannerAdapter struct {
	surveyScanner       *scanner.SurveyScanner
	relevantFileScanner *scanner.RelevantFileScanner
	shellScanner        *scanner.ShellScanner
	log                 *logrus.Logger
}

// NewEnhancedScannerAdapter 创建增强的扫描适配器
func NewEnhancedScannerAdapter(log *logrus.Logger) *EnhancedScannerAdapter {
	return &EnhancedScannerAdapter{
		surveyScanner:       scanner.NewSurveyScanner(log),
		relevantFileScanner: scanner.NewRelevantFileScanner(log),
		shellScanner:        scanner.NewShellScanner(log),
		log:                 log,
	}
}

// ScanRepository 扫描仓库，返回增强的结果
func (a *EnhancedScannerAdapter) ScanRepository(repoPath string, repoID uint, repoName string) (*EnhancedScanResult, error) {
	a.log.WithFields(logrus.Fields{
		"path": repoPath,
		"id":   repoID,
		"name": repoName,
	}).Info("开始增强扫描")

	result := &EnhancedScanResult{
		Surveys: []SurveyFile{},
	}

	// 设置仓库信息
	result.Repository.ID = repoID
	result.Repository.Name = repoName
	result.Repository.Path = repoPath

	// 1. 扫描相关文件树
	fileTree, err := a.relevantFileScanner.ScanRelevantFiles(repoPath)
	if err != nil {
		a.log.WithError(err).Error("扫描相关文件失败")
		// 不中断，继续扫描survey
	} else {
		result.FileTree = fileTree
		// 统计文件信息
		a.countRelevantFiles(fileTree, &result.Stats)
	}

	// 2. 扫描survey文件
	surveys, err := a.surveyScanner.ScanSurveys(repoPath)
	if err != nil {
		a.log.WithError(err).Error("扫描survey文件失败")
		return nil, err
	}

	// 3. 转换survey信息
	for _, survey := range surveys {
		surveyFile := SurveyFile{
			Path:        survey.Path,
			Name:        survey.Name,
			Description: survey.Description,
			Parameters:  []TemplateParameter{},
		}

		// 转换参数
		for _, param := range survey.Parameters {
			templateParam := TemplateParameter{
				Name:        param.Name,
				Type:        param.Type,
				Description: param.Description,
				Required:    param.Required,
				Default:     param.Default,
				Options:     param.Options,
				MinValue:    param.MinValue,
				MaxValue:    param.MaxValue,
				MinLength:   param.MinLength,
				MaxLength:   param.MaxLength,
				Source:      param.Source,
			}
			surveyFile.Parameters = append(surveyFile.Parameters, templateParam)
		}

		result.Surveys = append(result.Surveys, surveyFile)
	}

	// 3. 扫描Shell脚本参数
	shellScripts, err := a.shellScanner.Scan(repoPath)
	if err != nil {
		a.log.WithError(err).Warn("扫描Shell脚本失败")
		// 不中断扫描
	} else {
		// 将Shell脚本参数信息添加到surveys中
		for _, script := range shellScripts {
			if script.Survey != nil {
				surveyFile := SurveyFile{
					Path:        script.Path,
					Name:        script.Survey.Name,
					Description: script.Survey.Description,
					Parameters:  []TemplateParameter{},
				}

				// 转换参数
				for _, param := range script.Survey.Spec {
					templateParam := TemplateParameter{
						Name:        param.Variable,
						Type:        param.Type,
						Description: param.QuestionDescription,
						Required:    param.Required,
						Default:     a.interfaceToString(param.Default),
						Options:     param.Choices,
						MinValue:    a.intToString(param.Min),
						MaxValue:    a.intToString(param.Max),
						Source:      "shell",
					}
					surveyFile.Parameters = append(surveyFile.Parameters, templateParam)
				}

				result.Surveys = append(result.Surveys, surveyFile)
			}
		}
	}

	result.Stats.SurveyFiles = len(result.Surveys)

	a.log.WithFields(logrus.Fields{
		"surveys":        len(result.Surveys),
		"ansible_files":  result.Stats.AnsibleFiles,
		"template_files": result.Stats.TemplateFiles,
		"shell_files":    result.Stats.ShellFiles,
		"total_files":    result.Stats.TotalFiles,
	}).Info("增强扫描完成")

	return result, nil
}

// countRelevantFiles 递归统计相关文件信息
func (a *EnhancedScannerAdapter) countRelevantFiles(node *scanner.RelevantFileNode, stats *struct {
	AnsibleFiles  int `json:"ansible_files"`
	TemplateFiles int `json:"template_files"`
	ShellFiles    int `json:"shell_files"`
	SurveyFiles   int `json:"survey_files"`
	TotalFiles    int `json:"total_files"`
}) {
	if node == nil {
		return
	}

	if node.Type == "file" {
		stats.TotalFiles++
		
		// 根据文件类型统计
		switch node.FileType {
		case "ansible":
			stats.AnsibleFiles++
		case "template":
			stats.TemplateFiles++
		case "shell":
			stats.ShellFiles++
		case "survey":
			// survey文件会在专门的扫描中统计
		}
	}

	// 递归处理子节点
	for i := range node.Children {
		a.countRelevantFiles(&node.Children[i], stats)
	}
}

// intToString 将整数指针转换为字符串指针
func (a *EnhancedScannerAdapter) intToString(i *int) *string {
	if i == nil {
		return nil
	}
	s := fmt.Sprintf("%d", *i)
	return &s
}

// interfaceToString 将interface{}转换为字符串
func (a *EnhancedScannerAdapter) interfaceToString(i interface{}) string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%v", i)
}