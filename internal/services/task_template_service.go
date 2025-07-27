package services

import (
	"ahop/internal/database"
	"ahop/internal/models"
	"ahop/pkg/logger"
	"ahop/pkg/queue"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// TaskTemplateService 任务模板服务
type TaskTemplateService struct {
	db    *gorm.DB
	queue *queue.RedisQueue
}

// NewTaskTemplateService 创建任务模板服务
func NewTaskTemplateService(db *gorm.DB) *TaskTemplateService {
	return &TaskTemplateService{
		db:    db,
		queue: database.GetRedisQueue(),
	}
}

// Create 创建任务模板
func (s *TaskTemplateService) Create(tenantID uint, req CreateTaskTemplateRequest, operatorID uint) (*models.TaskTemplate, error) {
	// 通过临时的RepositoryID查询Git仓库信息
	var repo models.GitRepository
	if err := s.db.Where("id = ? AND tenant_id = ?", req.RepositoryID, tenantID).First(&repo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("仓库不存在")
		}
		return nil, err
	}

	// 检查同租户下code是否重复
	var count int64
	s.db.Model(&models.TaskTemplate{}).Where("tenant_id = ? AND code = ?", tenantID, req.Code).Count(&count)
	if count > 0 {
		return nil, fmt.Errorf("模板编码已存在")
	}

	// 构建Git来源信息
	sourceGitInfo := map[string]interface{}{
		"repository_id":   repo.ID,
		"repository_name": repo.Name,
		"repository_url":  repo.URL,
		"branch":          repo.Branch,
		"original_path":   req.OriginalPath,
		"created_at":      time.Now().Format("2006-01-02 15:04:05"),
	}

	sourceGitInfoJSON, err := json.Marshal(sourceGitInfo)
	if err != nil {
		return nil, fmt.Errorf("序列化Git信息失败: %v", err)
	}

	// 转换参数格式
	parameters := s.convertSurveyToTemplateParameters(req.Parameters)

	// 创建模板（不再设置RepositoryID）
	template := &models.TaskTemplate{
		TenantID:      tenantID,
		Name:          req.Name,
		Code:          req.Code,
		ScriptType:    req.ScriptType,
		EntryFile:     req.EntryFile,
		IncludedFiles: models.IncludedFiles(req.IncludedFiles),
		SourceGitInfo: models.JSON(sourceGitInfoJSON),
		Description:   req.Description,
		Parameters:    parameters,
		Timeout:       req.Timeout,
		ExecutionType: req.ExecutionType,
		RequireSudo:   req.RequireSudo,
		CreatedBy:     operatorID,
		UpdatedBy:     operatorID,
	}

	if err := s.db.Create(template).Error; err != nil {
		logger.GetLogger().Errorf("创建任务模板失败: %v", err)
		return nil, fmt.Errorf("创建任务模板失败")
	}

	// 预加载关联数据
	if err := s.db.Preload("Tenant").First(template, template.ID).Error; err != nil {
		return nil, err
	}

	// 转换 IncludedFiles 格式以匹配 Worker 端期望的结构
	workerIncludedFiles := make([]WorkerIncludedFile, len(req.IncludedFiles))
	for i, file := range req.IncludedFiles {
		// 将 file_type 映射为 Worker 期望的 type
		fileType := "file" // 默认为文件类型
		if file.FileType == "directory" {
			fileType = "directory"
		} else if file.FileType == "pattern" {
			fileType = "pattern"
		}
		
		workerIncludedFiles[i] = WorkerIncludedFile{
			Path: file.Path,
			Type: fileType,
		}
	}

	// 通知Worker复制文件到独立目录
	copyMsg := TemplateCopyMessage{
		Action:        "copy",
		TemplateID:    template.ID,
		TenantID:      tenantID,
		TemplateCode:  template.Code,
		RepositoryID:  repo.ID,
		SourcePath:    req.OriginalPath,
		EntryFile:     req.EntryFile,
		IncludedFiles: workerIncludedFiles,
	}

	// 发布到Redis订阅通道（使用模板ID作为频道标识）
	channel := fmt.Sprintf("template:copy:%d", template.ID)
	msgBytes, err := json.Marshal(copyMsg)
	if err != nil {
		return nil, fmt.Errorf("序列化消息失败: %v", err)
	}

	ctx := context.Background()
	if s.queue != nil {
		if err := s.queue.GetClient().Publish(ctx, channel, msgBytes).Err(); err != nil {
			logger.GetLogger().Errorf("发布模板复制通知失败: %v", err)
			// 不影响创建操作，Worker会在执行时处理
		} else {
			logger.GetLogger().Infof("已通知Worker复制模板文件: channel=%s", channel)
		}
	} else {
		logger.GetLogger().Warn("Redis队列未初始化，跳过Worker通知")
	}

	logger.GetLogger().Infof("任务模板 %s (ID: %d) 创建成功", template.Name, template.ID)
	return template, nil
}

// Update 更新任务模板
func (s *TaskTemplateService) Update(tenantID uint, templateID uint, req UpdateTaskTemplateRequest, operatorID uint) (*models.TaskTemplate, error) {
	// 查找模板
	var template models.TaskTemplate
	if err := s.db.Where("id = ? AND tenant_id = ?", templateID, tenantID).First(&template).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("任务模板不存在")
		}
		return nil, err
	}

	// 如果更新了code，检查是否重复
	if req.Code != "" && req.Code != template.Code {
		var count int64
		s.db.Model(&models.TaskTemplate{}).Where("tenant_id = ? AND code = ? AND id != ?", tenantID, req.Code, templateID).Count(&count)
		if count > 0 {
			return nil, fmt.Errorf("模板编码已存在")
		}
	}

	// 更新字段
	updates := map[string]interface{}{
		"updated_by": operatorID,
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Code != "" {
		updates["code"] = req.Code
	}
	if req.ScriptType != "" {
		updates["script_type"] = req.ScriptType
	}
	if req.EntryFile != "" {
		updates["entry_file"] = req.EntryFile
	}
	if req.IncludedFiles != nil {
		updates["included_files"] = models.IncludedFiles(req.IncludedFiles)
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Parameters != nil {
		updates["parameters"] = s.convertSurveyToTemplateParameters(req.Parameters)
	}
	if req.Timeout > 0 {
		updates["timeout"] = req.Timeout
	}
	if req.ExecutionType != "" {
		updates["execution_type"] = req.ExecutionType
	}
	if req.RequireSudo != nil {
		updates["require_sudo"] = *req.RequireSudo
	}

	// 执行更新
	if err := s.db.Model(&template).Updates(updates).Error; err != nil {
		logger.GetLogger().Errorf("更新任务模板失败: %v", err)
		return nil, fmt.Errorf("更新任务模板失败")
	}

	// 重新加载完整数据
	if err := s.db.Preload("Tenant").First(&template, template.ID).Error; err != nil {
		return nil, err
	}

	logger.GetLogger().Infof("任务模板 %s (ID: %d) 更新成功", template.Name, template.ID)
	return &template, nil
}

// Delete 删除任务模板
func (s *TaskTemplateService) Delete(tenantID uint, templateID uint) error {
	// 查找模板
	var template models.TaskTemplate
	if err := s.db.Where("id = ? AND tenant_id = ?", templateID, tenantID).First(&template).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("任务模板不存在")
		}
		return err
	}

	// 检查是否有正在使用的任务
	var taskCount int64
	if err := s.db.Model(&models.Task{}).
		Where("tenant_id = ? AND task_type = ? AND (params->>'template_id')::integer = ?", 
			tenantID, models.TaskTypeTemplate, templateID).
		Count(&taskCount).Error; err != nil {
		logger.GetLogger().Errorf("检查任务模板使用情况失败: %v", err)
		return fmt.Errorf("检查任务模板使用情况失败")
	}
	
	if taskCount > 0 {
		// 检查是否有正在运行的任务
		var runningCount int64
		if err := s.db.Model(&models.Task{}).
			Where("tenant_id = ? AND task_type = ? AND (params->>'template_id')::integer = ? AND status IN ?", 
				tenantID, models.TaskTypeTemplate, templateID, 
				[]string{"pending", "queued", "locked", "running"}).
			Count(&runningCount).Error; err != nil {
			logger.GetLogger().Errorf("检查运行中的任务失败: %v", err)
			return fmt.Errorf("检查运行中的任务失败")
		}
		
		if runningCount > 0 {
			return fmt.Errorf("任务模板正在被 %d 个运行中的任务使用，无法删除", runningCount)
		}
		
		logger.GetLogger().Warnf("任务模板 %s (ID: %d) 有 %d 个历史任务记录", template.Name, template.ID, taskCount)
	}

	// 删除模板
	if err := s.db.Delete(&template).Error; err != nil {
		logger.GetLogger().Errorf("删除任务模板失败: %v", err)
		return fmt.Errorf("删除任务模板失败")
	}

	// 通知所有 Worker 删除模板目录
	deleteMsg := TemplateCopyMessage{
		Action:       "delete",
		TemplateID:   templateID,
		TenantID:     tenantID,
		TemplateCode: template.Code,
	}
	
	// 发送删除消息到队列（使用同一个队列 template_copy）
	if err := s.queue.PublishMessage("template_copy", deleteMsg); err != nil {
		logger.GetLogger().Errorf("发送模板删除消息失败: %v", err)
		// 不影响删除操作，只记录错误
	} else {
		logger.GetLogger().Infof("已发送模板删除消息: tenant=%d, template=%d, code=%s, action=delete", 
			tenantID, templateID, template.Code)
	}

	logger.GetLogger().Infof("任务模板 %s (ID: %d) 已删除", template.Name, template.ID)
	return nil
}

// GetByID 根据ID获取任务模板
func (s *TaskTemplateService) GetByID(tenantID uint, templateID uint) (*models.TaskTemplate, error) {
	var template models.TaskTemplate
	if err := s.db.Preload("Tenant").
		Where("id = ? AND tenant_id = ?", templateID, tenantID).
		First(&template).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("任务模板不存在")
		}
		return nil, err
	}

	return &template, nil
}

// List 获取任务模板列表
func (s *TaskTemplateService) List(tenantID uint, req ListTaskTemplateRequest) ([]models.TaskTemplate, int64, error) {
	query := s.db.Model(&models.TaskTemplate{}).Where("tenant_id = ?", tenantID)

	// 添加过滤条件
	if req.ScriptType != "" {
		query = query.Where("script_type = ?", req.ScriptType)
	}
	if req.Search != "" {
		search := "%" + req.Search + "%"
		query = query.Where("name LIKE ? OR code LIKE ? OR description LIKE ?", search, search, search)
	}

	// 计算总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	var templates []models.TaskTemplate
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("created_at DESC").
		Offset(offset).
		Limit(req.PageSize).
		Find(&templates).Error; err != nil {
		return nil, 0, err
	}

	return templates, total, nil
}

// generateTemplateCode 生成模板编码
func (s *TaskTemplateService) generateTemplateCode(scriptName string) string {
	// 移除扩展名
	name := strings.TrimSuffix(scriptName, ".sh")
	name = strings.TrimSuffix(name, ".yml")
	name = strings.TrimSuffix(name, ".yaml")

	// 替换特殊字符
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")

	// 转换为小写
	return strings.ToLower(name)
}

// convertScriptParameters 转换脚本参数为模板参数
func (s *TaskTemplateService) convertScriptParameters(scriptParams []ScriptParameter) models.TemplateParameters {
	params := make([]models.TemplateParameter, len(scriptParams))
	for i, sp := range scriptParams {
		params[i] = models.TemplateParameter{
			Name:        sp.Name,
			Type:        s.normalizeParameterType(sp.Type),
			Label:       sp.Name, // 使用参数名作为默认标签
			Description: sp.Description,
			Required:    sp.Required,
			Default:     sp.DefaultValue,
			Options:     sp.Options,
			Source:      "script",
		}
	}
	return params
}

// getDefaultExecutionType 获取默认执行类型
func (s *TaskTemplateService) getDefaultExecutionType(scriptType ScriptType) string {
	switch scriptType {
	case ScriptTypeAnsible:
		return "ansible"
	default:
		return "ssh"
	}
}


// getExecutionTypeFromString 从字符串获取执行类型
func (s *TaskTemplateService) getExecutionTypeFromString(scriptType string) string {
	switch scriptType {
	case "ansible":
		return "ansible"
	default:
		return "ssh"
	}
}


// CreateTaskTemplateRequest 创建任务模板请求
type CreateTaskTemplateRequest struct {
	Name          string                     `json:"name" binding:"required"`
	Code          string                     `json:"code" binding:"required"`
	ScriptType    string                     `json:"script_type" binding:"required,oneof=shell ansible"`
	EntryFile     string                     `json:"entry_file" binding:"required"`        // 主执行文件路径
	IncludedFiles []models.IncludedFile      `json:"included_files"`                       // 包含的文件列表
	RepositoryID  uint                       `json:"repository_id" binding:"required"`     // 临时使用，用于查找Git仓库
	OriginalPath  string                     `json:"original_path" binding:"required"`     // Git仓库中的原始路径
	Description   string                     `json:"description"`
	Parameters    []SurveyParameter          `json:"parameters"`                           // 使用扫描器返回的格式
	Timeout       int                        `json:"timeout"`
	ExecutionType string                     `json:"execution_type" binding:"oneof=ssh ansible"`
	RequireSudo   bool                       `json:"require_sudo"`
}

// UpdateTaskTemplateRequest 更新任务模板请求
type UpdateTaskTemplateRequest struct {
	Name          string                     `json:"name"`
	Code          string                     `json:"code"`
	ScriptType    string                     `json:"script_type" binding:"omitempty,oneof=shell ansible"`
	EntryFile     string                     `json:"entry_file"`
	IncludedFiles []models.IncludedFile      `json:"included_files"`
	Description   string                     `json:"description"`
	Parameters    []SurveyParameter          `json:"parameters"`
	Timeout       int                        `json:"timeout"`
	ExecutionType string                     `json:"execution_type" binding:"omitempty,oneof=sh ansible"`
	RequireSudo   *bool                      `json:"require_sudo"`
}

// ListTaskTemplateRequest 列表请求
type ListTaskTemplateRequest struct {
	Page         int    `form:"page,default=1"`
	PageSize     int    `form:"page_size,default=10"`
	ScriptType   string `form:"script_type"`
	Search       string `form:"search"`
}


// TemplateInfo Worker上报的模板信息
type TemplateInfo struct {
	Path        string               `json:"path"`
	Name        string               `json:"name"`
	Code        string               `json:"code"`
	ScriptType  string               `json:"script_type"`
	Description string               `json:"description"`
	Parameters  []TemplateParameter  `json:"parameters"`
}

// TemplateParameter Worker上报的模板参数
type TemplateParameter struct {
	Name         string      `json:"name"`
	Type         string      `json:"type"`
	Description  string      `json:"description"`
	Required     bool        `json:"required"`
	DefaultValue interface{} `json:"default_value,omitempty"`
	Options      []string    `json:"options,omitempty"`
}

// SurveyParameter 扫描器返回的参数格式（统一格式）
type SurveyParameter struct {
	Name        string                   `json:"name"`
	Type        string                   `json:"type"`
	Label       string                   `json:"label"`
	Description string                   `json:"description"`
	Required    bool                     `json:"required"`
	Default     interface{}              `json:"default,omitempty"`
	Options     []string                 `json:"options,omitempty"`
	MinValue    *string                  `json:"min_value,omitempty"`
	MaxValue    *string                  `json:"max_value,omitempty"`
	MinLength   *int                     `json:"min_length,omitempty"`
	MaxLength   *int                     `json:"max_length,omitempty"`
	Validation  *models.ValidationRules  `json:"validation,omitempty"`
	Source      string                   `json:"source,omitempty"`
}

// convertSurveyToTemplateParameters 转换扫描器参数格式到模板参数格式
func (s *TaskTemplateService) convertSurveyToTemplateParameters(surveyParams []SurveyParameter) models.TemplateParameters {
	params := make([]models.TemplateParameter, len(surveyParams))
	for i, sp := range surveyParams {
		param := models.TemplateParameter{
			Name:        sp.Name,
			Type:        s.normalizeParameterType(sp.Type),
			Label:       sp.Label,
			Description: sp.Description,
			Required:    sp.Required,
			Default:     sp.Default,
			Options:     sp.Options,
			Source:      sp.Source,
		}
		
		// 如果 Source 为空，默认为 "scanner"
		if param.Source == "" {
			param.Source = "scanner"
		}
		
		// 设置验证规则
		if sp.Validation != nil {
			// 直接使用传入的验证规则
			param.Validation = sp.Validation
		} else if sp.MinValue != nil || sp.MaxValue != nil {
			// 从 min_value/max_value 转换
			validation := &models.ValidationRules{}
			if sp.MinValue != nil {
				if val, err := strconv.Atoi(*sp.MinValue); err == nil {
					validation.Min = &val
				}
			}
			if sp.MaxValue != nil {
				if val, err := strconv.Atoi(*sp.MaxValue); err == nil {
					validation.Max = &val
				}
			}
			if validation.Min != nil || validation.Max != nil {
				param.Validation = validation
			}
		}
		
		params[i] = param
	}
	return params
}

// normalizeParameterType 标准化参数类型
func (s *TaskTemplateService) normalizeParameterType(scannerType string) string {
	// 统一类型映射
	switch scannerType {
	case "text", "string":
		return "text"
	case "textarea":
		return "textarea"
	case "integer", "int", "float":
		return "number"
	case "password", "secret":
		return "password"
	case "multiplechoice":
		return "select"
	case "multiselect":
		return "multiselect"
	default:
		return "text"
	}
}

// TemplateCopyMessage Worker模板文件复制消息
type TemplateCopyMessage struct {
	Action        string                      `json:"action"` // copy/delete
	TemplateID    uint                        `json:"template_id"`
	TenantID      uint                        `json:"tenant_id"`
	TemplateCode  string                      `json:"template_code"`
	RepositoryID  uint                        `json:"repository_id"`
	SourcePath    string                      `json:"source_path"`    // Git仓库中的原始路径
	EntryFile     string                      `json:"entry_file"`     // 相对于模板目录的入口文件
	IncludedFiles []WorkerIncludedFile        `json:"included_files"` // 包含的文件列表
}

// WorkerIncludedFile Worker端期望的文件格式
type WorkerIncludedFile struct {
	Path    string `json:"path"`             // 文件路径
	Type    string `json:"type"`             // file/directory/pattern
	Pattern string `json:"pattern,omitempty"` // 当type为pattern时的模式
}

// ValidateTemplateVariables 验证任务创建时的变量是否符合模板参数定义
func (s *TaskTemplateService) ValidateTemplateVariables(template *models.TaskTemplate, variables map[string]interface{}) error {
	// 构建参数映射，使用 name 字段作为 key
	paramMap := make(map[string]models.TemplateParameter)
	for _, param := range template.Parameters {
		paramMap[param.Name] = param
	}
	
	// 检查必填参数
	for _, param := range template.Parameters {
		if param.Required {
			if _, exists := variables[param.Name]; !exists {
				// 改进错误提示
				paramDesc := param.Name
				if param.Label != "" {
					paramDesc = fmt.Sprintf("%s (%s)", param.Name, param.Label)
				} else if param.Description != "" {
					paramDesc = fmt.Sprintf("%s (%s)", param.Name, param.Description)
				}
				return fmt.Errorf("缺少必填参数: %s", paramDesc)
			}
		}
	}
	
	// 验证提供的参数
	for varName, varValue := range variables {
		param, exists := paramMap[varName]
		if !exists {
			// 忽略未定义的参数，允许额外参数
			continue
		}
		
		// 类型验证
		if err := s.validateParameterValue(param, varValue); err != nil {
			return fmt.Errorf("参数 %s 验证失败: %v", varName, err)
		}
	}
	
	return nil
}

// validateParameterValue 验证单个参数值
func (s *TaskTemplateService) validateParameterValue(param models.TemplateParameter, value interface{}) error {
	switch param.Type {
	case "number":
		// 验证数字类型
		var numValue float64
		switch v := value.(type) {
		case float64:
			numValue = v
		case int:
			numValue = float64(v)
		case string:
			// 尝试解析字符串
			fmt.Sscanf(v, "%f", &numValue)
		default:
			return fmt.Errorf("期望数字类型，得到 %T", value)
		}
		
		// 验证范围
		if param.Validation != nil {
			if param.Validation.Min != nil && int(numValue) < *param.Validation.Min {
				return fmt.Errorf("值必须大于等于 %d", *param.Validation.Min)
			}
			if param.Validation.Max != nil && int(numValue) > *param.Validation.Max {
				return fmt.Errorf("值必须小于等于 %d", *param.Validation.Max)
			}
		}
		
	case "select":
		// 验证单选
		strValue := fmt.Sprintf("%v", value)
		if len(param.Options) > 0 {
			found := false
			for _, opt := range param.Options {
				if opt == strValue {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("值必须是以下选项之一: %v", param.Options)
			}
		}
		
	case "multiselect":
		// 验证多选
		// TODO: 实现多选验证
		
	case "text", "textarea", "password":
		// 验证字符串长度
		strValue := fmt.Sprintf("%v", value)
		if param.Validation != nil {
			if param.Validation.Min != nil && len(strValue) < *param.Validation.Min {
				return fmt.Errorf("长度必须大于等于 %d", *param.Validation.Min)
			}
			if param.Validation.Max != nil && len(strValue) > *param.Validation.Max {
				return fmt.Errorf("长度必须小于等于 %d", *param.Validation.Max)
			}
		}
	}
	
	return nil
}