package services

import (
	"ahop/internal/models"
	"ahop/pkg/logger"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// TaskTemplateService 任务模板服务
type TaskTemplateService struct {
	db *gorm.DB
}

// NewTaskTemplateService 创建任务模板服务
func NewTaskTemplateService(db *gorm.DB) *TaskTemplateService {
	return &TaskTemplateService{
		db: db,
	}
}

// Create 创建任务模板
func (s *TaskTemplateService) Create(tenantID uint, req CreateTaskTemplateRequest, operatorID uint) (*models.TaskTemplate, error) {
	// 验证仓库是否存在
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


	// 转换参数格式
	parameters := s.convertSurveyToTemplateParameters(req.Parameters)

	// 创建模板
	template := &models.TaskTemplate{
		TenantID:      tenantID,
		Name:          req.Name,
		Code:          req.Code,
		ScriptType:    req.ScriptType,
		EntryFile:     req.EntryFile,
		IncludedFiles: models.IncludedFiles(req.IncludedFiles),
		RepositoryID:  req.RepositoryID,
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
	if err := s.db.Preload("Repository").Preload("Tenant").First(template, template.ID).Error; err != nil {
		return nil, err
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

	// 如果更新了仓库ID，验证仓库是否存在
	if req.RepositoryID != 0 && req.RepositoryID != template.RepositoryID {
		var repo models.GitRepository
		if err := s.db.Where("id = ? AND tenant_id = ?", req.RepositoryID, tenantID).First(&repo).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("仓库不存在")
			}
			return nil, err
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
	if req.RepositoryID != 0 {
		updates["repository_id"] = req.RepositoryID
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
	if err := s.db.Preload("Repository").Preload("Tenant").First(&template, template.ID).Error; err != nil {
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

	// TODO: 检查是否有正在使用的任务

	// 删除模板
	if err := s.db.Delete(&template).Error; err != nil {
		logger.GetLogger().Errorf("删除任务模板失败: %v", err)
		return fmt.Errorf("删除任务模板失败")
	}

	logger.GetLogger().Infof("任务模板 %s (ID: %d) 已删除", template.Name, template.ID)
	return nil
}

// GetByID 根据ID获取任务模板
func (s *TaskTemplateService) GetByID(tenantID uint, templateID uint) (*models.TaskTemplate, error) {
	var template models.TaskTemplate
	if err := s.db.Preload("Repository").Preload("Tenant").
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
	if req.RepositoryID != 0 {
		query = query.Where("repository_id = ?", req.RepositoryID)
	}
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
	if err := query.Preload("Repository").
		Order("created_at DESC").
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
			Name:         sp.Name,
			Type:         sp.Type,
			Label:        sp.Name, // 使用参数名作为默认标签
			Description:  sp.Description,
			Required:     sp.Required,
			DefaultValue: sp.DefaultValue,
			Options:      sp.Options,
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

// SyncFromWorker 处理Worker上报的任务模板
func (s *TaskTemplateService) SyncFromWorker(repositoryID uint, templates []TemplateInfo, workerID string) error {
	// 验证仓库是否存在
	var repo models.GitRepository
	if err := s.db.First(&repo, repositoryID).Error; err != nil {
		return fmt.Errorf("仓库不存在")
	}
	
	// 开始事务
	tx := s.db.Begin()
	
	// 获取该仓库的所有现有模板
	oldTemplates := make(map[string]*models.TaskTemplate)
	var existingTemplates []models.TaskTemplate
	if err := tx.Where("repository_id = ? AND tenant_id = ?", repositoryID, repo.TenantID).Find(&existingTemplates).Error; err != nil {
		tx.Rollback()
		return err
	}
	
	for i := range existingTemplates {
		key := existingTemplates[i].EntryFile
		oldTemplates[key] = &existingTemplates[i]
	}
	
	// 处理上报的模板
	now := time.Now()
	for _, templateInfo := range templates {
		// 转换参数格式
		params := make(models.TemplateParameters, len(templateInfo.Parameters))
		for i, p := range templateInfo.Parameters {
			params[i] = models.TemplateParameter{
				Name:         p.Name,
				Type:         p.Type,
				Label:        p.Name, // 使用Name作为默认Label
				Description:  p.Description,
				Required:     p.Required,
				DefaultValue: p.DefaultValue,
				Options:      p.Options,
			}
		}
		
		if existing, exists := oldTemplates[templateInfo.Path]; exists {
			// 更新现有模板
			existing.Name = templateInfo.Name
			existing.ScriptType = templateInfo.ScriptType
			existing.Description = templateInfo.Description
			existing.Parameters = params
			existing.UpdatedBy = 0 // 系统更新
			existing.UpdatedAt = now
			
			if err := tx.Save(existing).Error; err != nil {
				tx.Rollback()
				return err
			}
			
			// 从待删除列表中移除
			delete(oldTemplates, templateInfo.Path)
		} else {
			// 创建新模板
			template := &models.TaskTemplate{
				TenantID:      repo.TenantID,
				Name:          templateInfo.Name,
				Code:          templateInfo.Code,
				ScriptType:    templateInfo.ScriptType,
				EntryFile:     templateInfo.Path,
				RepositoryID:  repositoryID,
				Description:   templateInfo.Description,
				Parameters:    params,
				Timeout:       300, // 默认5分钟
				ExecutionType: s.getExecutionTypeFromString(templateInfo.ScriptType),
				CreatedBy:     0, // 系统创建
				UpdatedBy:     0,
			}
			
			if err := tx.Create(template).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	
	// 删除不再存在的模板
	for _, template := range oldTemplates {
		if err := tx.Delete(template).Error; err != nil {
			tx.Rollback()
			return err
		}
		logger.GetLogger().Debugf("删除不存在的模板: %s", template.Name)
	}
	
	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return err
	}
	
	logger.GetLogger().Infof("成功处理 %d 个任务模板 (仓库ID: %d, Worker: %s)", len(templates), repositoryID, workerID)
	return nil
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

// GetByEntryFile 根据入口文件路径获取模板
func (s *TaskTemplateService) GetByEntryFile(tenantID uint, repositoryID uint, entryFile string) (*models.TaskTemplate, error) {
	var template models.TaskTemplate
	err := s.db.Where("tenant_id = ? AND repository_id = ? AND entry_file = ?", tenantID, repositoryID, entryFile).
		Preload("Repository").
		First(&template).Error
	return &template, err
}

// CreateTaskTemplateRequest 创建任务模板请求
type CreateTaskTemplateRequest struct {
	Name          string                     `json:"name" binding:"required"`
	Code          string                     `json:"code" binding:"required"`
	ScriptType    string                     `json:"script_type" binding:"required,oneof=shell ansible"`
	EntryFile     string                     `json:"entry_file" binding:"required"`        // 主执行文件路径
	IncludedFiles []models.IncludedFile      `json:"included_files"`                       // 包含的文件列表
	RepositoryID  uint                       `json:"repository_id" binding:"required"`
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
	RepositoryID  uint                       `json:"repository_id"`
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
	RepositoryID uint   `form:"repository_id"`
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

// SurveyParameter 扫描器返回的参数格式
type SurveyParameter struct {
	Variable            string      `json:"variable"`
	Type                string      `json:"type"`
	QuestionName        string      `json:"question_name"`
	QuestionDescription string      `json:"question_description"`
	Required            bool        `json:"required"`
	Default             interface{} `json:"default,omitempty"`
	Choices             []string    `json:"choices,omitempty"`
	Min                 *int        `json:"min,omitempty"`
	Max                 *int        `json:"max,omitempty"`
	MinLength           *int        `json:"min_length,omitempty"`
	MaxLength           *int        `json:"max_length,omitempty"`
}

// convertSurveyToTemplateParameters 转换扫描器参数格式到模板参数格式
func (s *TaskTemplateService) convertSurveyToTemplateParameters(surveyParams []SurveyParameter) models.TemplateParameters {
	params := make([]models.TemplateParameter, len(surveyParams))
	for i, sp := range surveyParams {
		params[i] = models.TemplateParameter{
			Name:         sp.Variable,
			Type:         s.mapSurveyTypeToTemplateType(sp.Type),
			Label:        sp.QuestionName,
			Description:  sp.QuestionDescription,
			Required:     sp.Required,
			DefaultValue: sp.Default,
			Options:      sp.Choices,
		}
	}
	return params
}

// mapSurveyTypeToTemplateType 映射扫描器类型到模板类型
func (s *TaskTemplateService) mapSurveyTypeToTemplateType(surveyType string) string {
	typeMap := map[string]string{
		"text":           "string",
		"textarea":       "string",
		"password":       "password",
		"integer":        "string", // 在模板中统一作为字符串处理
		"float":          "string",
		"multiplechoice": "select",
		"multiselect":    "multiselect",
	}
	
	if templateType, ok := typeMap[surveyType]; ok {
		return templateType
	}
	return "string"
}