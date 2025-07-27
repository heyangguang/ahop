package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	
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
	Path         string                 `json:"path"`         // survey文件相对路径
	Name         string                 `json:"name"`         // survey名称
	Description  string                 `json:"description"`  // survey描述
	Parameters   []TemplateParameter    `json:"parameters"`   // 参数列表
	
	// 模板元信息（自动生成）
	Code          string                 `json:"code"`          // 建议的模板代码
	ScriptType    string                 `json:"script_type"`   // 脚本类型 shell/ansible
	ExecutionType string                 `json:"execution_type"` // 执行类型 ssh/ansible（智能推断）
	EntryFile     string                 `json:"entry_file"`    // 入口文件
	OriginalPath  string                 `json:"original_path"`  // 原始路径（相对于仓库根目录）
	IncludedFiles []IncludedFile        `json:"included_files"` // 包含的文件列表
	RepositoryID  uint                   `json:"repository_id"` // 仓库ID（智能推断）
}


// TemplateParameter 模板参数（兼容前端）
type TemplateParameter struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Label       string            `json:"label,omitempty"`       // 智能推断的显示标签
	Description string            `json:"description"`
	Required    bool              `json:"required"`
	Default     string            `json:"default,omitempty"`
	Options     []string          `json:"options,omitempty"`
	MinValue    *string           `json:"min_value,omitempty"`
	MaxValue    *string           `json:"max_value,omitempty"`
	MinLength   *int              `json:"min_length,omitempty"`
	MaxLength   *int              `json:"max_length,omitempty"`
	Validation  *ValidationRules  `json:"validation,omitempty"`  // 智能推断的验证规则
	Source      string            `json:"source,omitempty"`
}

// ValidationRules 参数验证规则
type ValidationRules struct {
	Min     *int   `json:"min,omitempty"`     // 最小值/最小长度
	Max     *int   `json:"max,omitempty"`     // 最大值/最大长度
	Pattern string `json:"pattern,omitempty"` // 正则表达式
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
		// 构建完整路径
		fullSurveyPath := filepath.Join(repoPath, survey.Path)
		
		// 自动生成模板元信息
		code := a.generateTemplateCode(fullSurveyPath)
		scriptType := a.detectScriptType(fullSurveyPath, repoPath)
		entryFile, originalPath := a.findEntryFile(fullSurveyPath, repoPath, scriptType)
		includedFiles := a.detectIncludedFiles(fullSurveyPath, repoPath, scriptType, fileTree)
		
		// 如果入口文件不在 included_files 中，添加它
		hasEntryFile := false
		for _, f := range includedFiles {
			if f.Path == entryFile {
				hasEntryFile = true
				break
			}
		}
		if !hasEntryFile && entryFile != "" {
			includedFiles = append(includedFiles, IncludedFile{
				Path: entryFile,
				Type: "file",
			})
			a.log.WithField("added", entryFile).Debug("Added entry file to included files")
		}
		
		// 调试日志
		a.log.WithFields(logrus.Fields{
			"survey_path":    survey.Path,
			"code":           code,
			"script_type":    scriptType,
			"entry_file":     entryFile,
			"original_path":  originalPath,
			"included_files": len(includedFiles),
		}).Debug("生成模板元信息")
		
		surveyFile := SurveyFile{
			Path:          survey.Path,
			Name:          survey.Name,
			Description:   survey.Description,
			Parameters:    []TemplateParameter{},
			Code:          code,
			ScriptType:    scriptType,
			ExecutionType: a.inferExecutionType(scriptType), // 智能推断执行类型
			EntryFile:     entryFile,
			OriginalPath:  originalPath,
			IncludedFiles: includedFiles,
			RepositoryID:  repoID, // 智能推断仓库ID
		}

		// 转换参数
		for _, param := range survey.Parameters {
			templateParam := TemplateParameter{
				Name:        param.Name,
				Type:        param.Type,
				Label:       a.inferLabel(param.Name, param.Description), // 智能推断标签
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
			
			// 智能推断验证规则
			templateParam.Validation = a.inferValidation(&templateParam)
			
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
				// 自动生成模板元信息
				code := a.generateTemplateCode(script.Path)
				originalPath := a.getRelativePath(script.Path, repoPath)
				entryFile := filepath.Base(script.Path)
				
				surveyFile := SurveyFile{
					Path:          originalPath, // 使用相对路径
					Name:          script.Survey.Name,
					Description:   script.Survey.Description,
					Parameters:    []TemplateParameter{},
					Code:          code,
					ScriptType:    "shell",
					ExecutionType: "ssh", // Shell脚本默认使用ssh执行
					EntryFile:     entryFile,
					OriginalPath:  filepath.Dir(originalPath),
					IncludedFiles: []IncludedFile{}, // Shell脚本通常没有依赖文件
					RepositoryID:  repoID, // 智能推断仓库ID
				}

				// 转换参数
				for _, param := range script.Survey.Spec {
					templateParam := TemplateParameter{
						Name:        param.Variable,
						Type:        param.Type,
						Label:       a.inferLabel(param.Variable, param.QuestionDescription), // 智能推断标签
						Description: param.QuestionDescription,
						Required:    param.Required,
						Default:     a.interfaceToString(param.Default),
						Options:     param.Choices,
						MinValue:    a.intToString(param.Min),
						MaxValue:    a.intToString(param.Max),
						Source:      "shell",
					}
					
					// 智能推断验证规则
					templateParam.Validation = a.inferValidation(&templateParam)
					
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

// inferExecutionType 根据脚本类型推断执行类型
func (a *EnhancedScannerAdapter) inferExecutionType(scriptType string) string {
	if scriptType == "ansible" {
		return "ansible"
	}
	return "ssh"
}

// inferLabel 智能推断参数标签
func (a *EnhancedScannerAdapter) inferLabel(name, description string) string {
	// 如果有描述，直接使用描述作为标签
	if description != "" {
		return description
	}
	
	// 基于参数名智能生成标签
	labelMap := map[string]string{
		// 数据库相关
		"db_host":     "数据库主机",
		"db_port":     "数据库端口",
		"db_name":     "数据库名称",
		"db_user":     "数据库用户",
		"db_password": "数据库密码",
		"db_type":     "数据库类型",
		
		// 服务器相关
		"server_name": "服务器名称",
		"server_ip":   "服务器IP",
		"server_port": "服务器端口",
		"host":        "主机地址",
		"hostname":    "主机名",
		"port":        "端口号",
		
		// 认证相关
		"username":    "用户名",
		"password":    "密码",
		"token":       "令牌",
		"api_key":     "API密钥",
		"secret_key":  "密钥",
		
		// 路径相关
		"path":        "路径",
		"file_path":   "文件路径",
		"dir_path":    "目录路径",
		"backup_path": "备份路径",
		"log_path":    "日志路径",
		
		// SSL/TLS相关
		"enable_ssl":    "启用SSL",
		"ssl_cert_path": "SSL证书路径",
		"ssl_key_path":  "SSL密钥路径",
		
		// 通用配置
		"timeout":     "超时时间",
		"retry_count": "重试次数",
		"max_size":    "最大大小",
		"min_size":    "最小大小",
		"enabled":     "是否启用",
		"debug":       "调试模式",
		"verbose":     "详细输出",
		
		// 特定应用
		"nginx_version":    "Nginx版本",
		"worker_processes": "工作进程数",
		"listen_port":      "监听端口",
		"retention_days":   "保留天数",
		"output_format":    "输出格式",
		"show_disk":        "显示磁盘信息",
		"show_network":     "显示网络信息",
	}
	
	if label, ok := labelMap[name]; ok {
		return label
	}
	
	// 尝试基于名称模式生成
	// 转换下划线为空格，首字母大写
	parts := strings.Split(name, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

// inferValidation 智能推断验证规则
func (a *EnhancedScannerAdapter) inferValidation(param *TemplateParameter) *ValidationRules {
	var validation ValidationRules
	hasValidation := false
	
	// 1. 从现有的 min_value/max_value 转换
	if param.MinValue != nil || param.MaxValue != nil {
		if param.Type == "number" || param.Type == "integer" {
			if param.MinValue != nil {
				if v, err := strconv.Atoi(*param.MinValue); err == nil {
					validation.Min = &v
					hasValidation = true
				}
			}
			if param.MaxValue != nil {
				if v, err := strconv.Atoi(*param.MaxValue); err == nil {
					validation.Max = &v
					hasValidation = true
				}
			}
		}
	}
	
	// 2. 从 min_length/max_length 转换
	if param.MinLength != nil {
		validation.Min = param.MinLength
		hasValidation = true
	}
	if param.MaxLength != nil {
		validation.Max = param.MaxLength
		hasValidation = true
	}
	
	// 3. 基于参数名智能推断
	name := strings.ToLower(param.Name)
	
	// 端口号验证
	if strings.Contains(name, "port") && (param.Type == "number" || param.Type == "integer") {
		if validation.Min == nil {
			min := 1
			validation.Min = &min
			hasValidation = true
		}
		if validation.Max == nil {
			max := 65535
			validation.Max = &max
			hasValidation = true
		}
	}
	
	// Email验证
	if strings.Contains(name, "email") && param.Type == "string" {
		validation.Pattern = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
		hasValidation = true
	}
	
	// IP地址验证
	if (strings.Contains(name, "ip") || strings.Contains(name, "address")) && 
		!strings.Contains(name, "description") && param.Type == "string" {
		validation.Pattern = `^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`
		hasValidation = true
	}
	
	// URL验证
	if strings.Contains(name, "url") && param.Type == "string" {
		validation.Pattern = `^https?://[^\s/$.?#].[^\s]*$`
		hasValidation = true
	}
	
	// 百分比验证
	if strings.Contains(name, "percent") && (param.Type == "number" || param.Type == "integer") {
		if validation.Min == nil {
			min := 0
			validation.Min = &min
		}
		if validation.Max == nil {
			max := 100
			validation.Max = &max
		}
		hasValidation = true
	}
	
	// 天数验证（通常是正数）
	if strings.Contains(name, "days") && (param.Type == "number" || param.Type == "integer") {
		if validation.Min == nil {
			min := 0
			validation.Min = &min
			hasValidation = true
		}
	}
	
	// 超时验证（通常有最小值）
	if strings.Contains(name, "timeout") && (param.Type == "number" || param.Type == "integer") {
		if validation.Min == nil {
			min := 1
			validation.Min = &min
			hasValidation = true
		}
	}
	
	if hasValidation {
		return &validation
	}
	return nil
}

// generateTemplateCode 根据文件路径生成模板代码
func (a *EnhancedScannerAdapter) generateTemplateCode(filePath string) string {
	// 获取目录路径
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)
	
	a.log.WithFields(logrus.Fields{
		"file_path": filePath,
		"dir":       dir,
		"base":      base,
	}).Debug("generateTemplateCode called")
	
	// 如果是 survey.yml，尝试根据目录或同目录的其他文件生成更好的名称
	if strings.ToLower(base) == "survey.yml" || strings.ToLower(base) == "survey.yaml" {
		// 查找同目录下的主要文件
		var betterName string
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || path == filePath {
				return nil
			}
			
			fileName := filepath.Base(path)
			ext := strings.ToLower(filepath.Ext(fileName))
			
			// 优先使用 .yml/.yaml 文件（排除 survey）
			if (ext == ".yml" || ext == ".yaml") && !strings.Contains(strings.ToLower(fileName), "survey") {
				betterName = strings.TrimSuffix(fileName, ext)
				return filepath.SkipDir
			}
			// 其次使用 .sh 文件
			if ext == ".sh" && betterName == "" {
				betterName = strings.TrimSuffix(fileName, ext)
			}
			return nil
		})
		
		if betterName != "" {
			name := betterName
			// 替换特殊字符为下划线
			name = strings.ReplaceAll(name, "-", "_")
			name = strings.ReplaceAll(name, " ", "_")
			name = strings.ReplaceAll(name, ".", "_")
			return strings.ToLower(name)
		}
	}
	
	// 默认处理：使用文件名（不含扩展名）
	name := strings.TrimSuffix(base, filepath.Ext(base))
	
	// 替换特殊字符为下划线
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, ".", "_")
	
	// 转换为小写
	return strings.ToLower(name)
}

// detectScriptType 检测脚本类型
func (a *EnhancedScannerAdapter) detectScriptType(surveyPath, repoPath string) string {
	// 获取survey文件所在目录
	surveyDir := filepath.Dir(surveyPath)
	
	// 在同目录或附近查找 .yml/.yaml 文件（排除survey文件本身）
	var hasAnsibleFiles bool
	filepath.Walk(surveyDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// 跳过目录
		if info.IsDir() {
			return nil
		}
		
		ext := strings.ToLower(filepath.Ext(path))
		baseName := strings.ToLower(filepath.Base(path))
		
		// 排除 survey 文件本身
		if (ext == ".yml" || ext == ".yaml") && !strings.Contains(baseName, "survey") {
			hasAnsibleFiles = true
			return filepath.SkipDir // 找到就停止
		}
		return nil
	})
	
	if hasAnsibleFiles {
		return "ansible"
	}
	return "shell"
}

// findEntryFile 查找入口文件
func (a *EnhancedScannerAdapter) findEntryFile(surveyPath, repoPath, scriptType string) (entryFile, originalPath string) {
	surveyDir := filepath.Dir(surveyPath)
	
	// 获取相对路径
	relPath, _ := filepath.Rel(repoPath, surveyDir)
	if relPath == "" {
		relPath = "."
	}
	
	if scriptType == "ansible" {
		// 先尝试其他常见的 playbook 文件名
		patterns := []string{
			filepath.Join(surveyDir, "playbook.yml"),
			filepath.Join(surveyDir, "main.yml"),
			filepath.Join(surveyDir, "site.yml"),
			filepath.Join(surveyDir, "deploy.yml"),
		}
		
		for _, pattern := range patterns {
			if _, err := os.Stat(pattern); err == nil {
				return filepath.Base(pattern), relPath
			}
		}
		
		// 查找任意 .yml 文件（排除 survey 文件）
		filepath.Walk(surveyDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			baseName := strings.ToLower(filepath.Base(path))
			if !info.IsDir() && (ext == ".yml" || ext == ".yaml") && !strings.Contains(baseName, "survey") {
				entryFile = filepath.Base(path)
				return filepath.SkipDir
			}
			return nil
		})
	} else {
		// Shell 脚本：查找 .sh 文件
		filepath.Walk(surveyDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".sh" {
				entryFile = filepath.Base(path)
				return filepath.SkipDir
			}
			return nil
		})
	}
	
	return entryFile, relPath
}

// detectIncludedFiles 检测包含的文件
func (a *EnhancedScannerAdapter) detectIncludedFiles(surveyPath, repoPath, scriptType string, fileTree *scanner.RelevantFileNode) []IncludedFile {
	includedFiles := []IncludedFile{}
	surveyDir := filepath.Dir(surveyPath)
	
	// 调试日志
	a.log.WithFields(logrus.Fields{
		"surveyPath": surveyPath,
		"surveyDir":  surveyDir,
		"scriptType": scriptType,
	}).Debug("detectIncludedFiles called")
	
	// 总是包含 survey.yml 文件本身
	surveyFileName := filepath.Base(surveyPath)
	if strings.ToLower(surveyFileName) == "survey.yml" || strings.ToLower(surveyFileName) == "survey.yaml" {
		includedFiles = append(includedFiles, IncludedFile{
			Path: surveyFileName,
			Type: "file",
		})
		a.log.WithField("added", surveyFileName).Debug("Added survey file")
	}
	
	if scriptType == "ansible" {
		// 查找模板文件
		templatesDir := filepath.Join(surveyDir, "templates")
		if info, err := os.Stat(templatesDir); err == nil && info.IsDir() {
			filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if !info.IsDir() {
					relPath, _ := filepath.Rel(surveyDir, path)
					includedFiles = append(includedFiles, IncludedFile{
						Path: relPath,
						Type: "file",
					})
					a.log.WithField("added", relPath).Debug("Added template file")
				}
				return nil
			})
		}
		
		// 查找 vars 文件
		varsFiles := []string{"vars.yml", "vars/main.yml", "defaults/main.yml"}
		for _, varsFile := range varsFiles {
			fullPath := filepath.Join(surveyDir, varsFile)
			if _, err := os.Stat(fullPath); err == nil {
				includedFiles = append(includedFiles, IncludedFile{
					Path: varsFile,
					Type: "file",
				})
				a.log.WithField("added", varsFile).Debug("Added vars file")
			}
		}
		
		// 查找roles目录
		rolesDir := filepath.Join(surveyDir, "roles")
		if info, err := os.Stat(rolesDir); err == nil && info.IsDir() {
			includedFiles = append(includedFiles, IncludedFile{
				Path: "roles",
				Type: "directory",
			})
			a.log.WithField("added", "roles").Debug("Added roles directory")
		}
		
		// 查找files目录
		filesDir := filepath.Join(surveyDir, "files")
		if info, err := os.Stat(filesDir); err == nil && info.IsDir() {
			includedFiles = append(includedFiles, IncludedFile{
				Path: "files", 
				Type: "directory",
			})
			a.log.WithField("added", "files").Debug("Added files directory")
		}
		
		// 查找handlers目录
		handlersDir := filepath.Join(surveyDir, "handlers")
		if info, err := os.Stat(handlersDir); err == nil && info.IsDir() {
			includedFiles = append(includedFiles, IncludedFile{
				Path: "handlers",
				Type: "directory",
			})
			a.log.WithField("added", "handlers").Debug("Added handlers directory")
		}
		
		// 查找tasks目录
		tasksDir := filepath.Join(surveyDir, "tasks")
		if info, err := os.Stat(tasksDir); err == nil && info.IsDir() {
			includedFiles = append(includedFiles, IncludedFile{
				Path: "tasks",
				Type: "directory",
			})
			a.log.WithField("added", "tasks").Debug("Added tasks directory")
		}
		
		// 查找inventory文件
		inventoryFiles := []string{"inventory", "hosts", "inventory.ini", "hosts.ini"}
		for _, invFile := range inventoryFiles {
			fullPath := filepath.Join(surveyDir, invFile)
			if _, err := os.Stat(fullPath); err == nil {
				includedFiles = append(includedFiles, IncludedFile{
					Path: invFile,
					Type: "file",
				})
				a.log.WithField("added", invFile).Debug("Added inventory file")
			}
		}
		
		// 查找ansible.cfg
		cfgPath := filepath.Join(surveyDir, "ansible.cfg")
		if _, err := os.Stat(cfgPath); err == nil {
			includedFiles = append(includedFiles, IncludedFile{
				Path: "ansible.cfg",
				Type: "file",
			})
			a.log.WithField("added", "ansible.cfg").Debug("Added ansible.cfg")
		}
		
		// 查找requirements.yml
		reqPath := filepath.Join(surveyDir, "requirements.yml")
		if _, err := os.Stat(reqPath); err == nil {
			includedFiles = append(includedFiles, IncludedFile{
				Path: "requirements.yml",
				Type: "file",
			})
			a.log.WithField("added", "requirements.yml").Debug("Added requirements.yml")
		}
	} else if scriptType == "shell" {
		// Shell脚本可能需要的其他文件
		// 查找配置文件
		configFiles := []string{"config", "config.sh", "config.conf", ".env"}
		for _, cfgFile := range configFiles {
			fullPath := filepath.Join(surveyDir, cfgFile)
			if _, err := os.Stat(fullPath); err == nil {
				includedFiles = append(includedFiles, IncludedFile{
					Path: cfgFile,
					Type: "file",
				})
				a.log.WithField("added", cfgFile).Debug("Added config file")
			}
		}
		
		// 查找lib目录
		libDir := filepath.Join(surveyDir, "lib")
		if info, err := os.Stat(libDir); err == nil && info.IsDir() {
			includedFiles = append(includedFiles, IncludedFile{
				Path: "lib",
				Type: "directory",
			})
			a.log.WithField("added", "lib").Debug("Added lib directory")
		}
	}
	
	a.log.WithField("total_files", len(includedFiles)).Debug("detectIncludedFiles completed")
	
	return includedFiles
}

// getRelativePath 获取相对路径
func (a *EnhancedScannerAdapter) getRelativePath(fullPath, repoPath string) string {
	relPath, err := filepath.Rel(repoPath, fullPath)
	if err != nil {
		return fullPath
	}
	return relPath
}