package services

import (
	"ahop/internal/models"
	"ahop/pkg/logger"
	"ahop/pkg/queue"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)


// BatchSyncRequest 批量同步请求
type BatchSyncRequest struct {
	RepositoryIDs []uint `json:"repository_ids" binding:"required"`
}

// BatchSyncResult 批量同步结果
type BatchSyncResult struct {
	RepositoryID uint   `json:"repository_id"`
	Success      bool   `json:"success"`
	Message      string `json:"message"`
}

// GitRepositoryService Git仓库服务
type GitRepositoryService struct {
	db                *gorm.DB
	queue             *queue.RedisQueue
	credentialService *CredentialService
}

// NewGitRepositoryService 创建Git仓库服务实例
func NewGitRepositoryService(db *gorm.DB) *GitRepositoryService {
	return &GitRepositoryService{
		db:                db,
		queue:             nil, // 暂时设为nil，等待注入
		credentialService: NewCredentialService(db),
	}
}

// SetQueue 设置Redis队列
func (s *GitRepositoryService) SetQueue(q *queue.RedisQueue) {
	s.queue = q
}

// Create 创建Git仓库
func (s *GitRepositoryService) Create(tenantID uint, req *CreateGitRepositoryRequest) (*models.GitRepository, error) {
	// 验证必填字段
	if req.Name == "" || req.Code == "" || req.URL == "" {
		return nil, fmt.Errorf("名称、代码和URL不能为空")
	}

	// 验证代码唯一性
	var count int64
	s.db.Model(&models.GitRepository{}).Where("tenant_id = ? AND code = ?", tenantID, req.Code).Count(&count)
	if count > 0 {
		return nil, fmt.Errorf("仓库代码已存在")
	}

	// 验证URL格式
	if !isValidGitURL(req.URL) {
		return nil, fmt.Errorf("无效的Git URL")
	}

	// 如果是私有仓库，验证凭证
	if !req.IsPublic && req.CredentialID != nil {
		var credential models.Credential
		if err := s.db.Where("id = ? AND tenant_id = ?", *req.CredentialID, tenantID).First(&credential).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("凭证不存在")
			}
			return nil, err
		}

		// 验证凭证类型是否适合Git
		validTypes := []models.CredentialType{models.CredentialTypeSSHKey, models.CredentialTypePassword, models.CredentialTypeToken}
		isValid := false
		for _, t := range validTypes {
			if credential.Type == t {
				isValid = true
				break
			}
		}
		if !isValid {
			return nil, fmt.Errorf("凭证类型不支持Git仓库")
		}
	}

	// 设置默认分支
	if req.Branch == "" {
		req.Branch = "main"
	}

	// 创建仓库
	repo := &models.GitRepository{
		TenantID:     tenantID,
		Name:         req.Name,
		Code:         req.Code,
		Description:  req.Description,
		URL:          req.URL,
		Branch:       req.Branch,
		IsPublic:     req.IsPublic,
		CredentialID: req.CredentialID,
		SyncEnabled:  req.SyncEnabled,
		SyncCron:     req.SyncCron,
		Status:       "active",
	}

	if err := s.db.Create(repo).Error; err != nil {
		logger.GetLogger().Errorf("创建Git仓库失败: %v", err)
		return nil, fmt.Errorf("创建仓库失败")
	}
	
	// 生成本地路径（使用tenant_id/repo_id格式）
	repo.LocalPath = fmt.Sprintf("%d/%d", tenantID, repo.ID)
	if err := s.db.Model(repo).Update("local_path", repo.LocalPath).Error; err != nil {
		logger.GetLogger().Errorf("更新仓库本地路径失败: %v", err)
		// 不影响创建操作
	}

	// 通知Worker同步新仓库
	if err := s.notifyWorkersSync(repo); err != nil {
		logger.GetLogger().Errorf("通知Worker同步失败: %v", err)
		// 不影响创建操作
	}

	// 如果启用了定时同步，添加到调度器
	if repo.SyncEnabled && repo.SyncCron != "" {
		if scheduler := GetGitSyncScheduler(); scheduler != nil {
			if err := scheduler.AddJob(repo); err != nil {
				logger.GetLogger().Errorf("添加定时同步任务失败: %v", err)
				// 不影响创建操作
			}
		}
	}

	return repo, nil
}

// Update 更新Git仓库
func (s *GitRepositoryService) Update(tenantID uint, repoID uint, req *UpdateGitRepositoryRequest) (*models.GitRepository, error) {
	var repo models.GitRepository
	if err := s.db.Where("id = ? AND tenant_id = ?", repoID, tenantID).First(&repo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("仓库不存在")
		}
		return nil, err
	}

	// 更新字段
	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["name"] = req.Name
	}

	if req.Description != nil {
		updates["description"] = *req.Description
	}

	if req.Branch != "" {
		updates["branch"] = req.Branch
	}

	if req.IsPublic != nil {
		updates["is_public"] = *req.IsPublic
	}

	if req.CredentialID != nil {
		// 验证凭证
		if *req.CredentialID > 0 {
			var credential models.Credential
			if err := s.db.Where("id = ? AND tenant_id = ?", *req.CredentialID, tenantID).First(&credential).Error; err != nil {
				return nil, fmt.Errorf("凭证不存在")
			}
		}
		updates["credential_id"] = req.CredentialID
	}

	if req.SyncEnabled != nil {
		updates["sync_enabled"] = *req.SyncEnabled
	}

	if req.SyncCron != nil {
		updates["sync_cron"] = *req.SyncCron
	}

	// 更新数据
	if err := s.db.Model(&repo).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新仓库失败")
	}

	// 重新加载完整数据
	s.db.Preload("Credential").First(&repo, repoID)

	// 通知Worker更新仓库
	if err := s.notifyWorkersSync(&repo); err != nil {
		logger.GetLogger().Errorf("通知Worker同步失败: %v", err)
		// 不影响更新操作
	}

	// 更新调度器中的定时任务
	if scheduler := GetGitSyncScheduler(); scheduler != nil {
		if err := scheduler.UpdateJob(&repo); err != nil {
			logger.GetLogger().Errorf("更新定时同步任务失败: %v", err)
			// 不影响更新操作
		}
	}

	return &repo, nil
}

// Delete 删除Git仓库
func (s *GitRepositoryService) Delete(tenantID uint, repoID uint) error {
	// 检查仓库是否存在
	var repo models.GitRepository
	if err := s.db.Where("id = ? AND tenant_id = ?", repoID, tenantID).First(&repo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("仓库不存在")
		}
		return err
	}

	// 检查是否有关联的任务模板
	var templateCount int64
	if err := s.db.Model(&models.TaskTemplate{}).Where("repository_id = ?", repoID).Count(&templateCount).Error; err != nil {
		return fmt.Errorf("检查关联任务模板失败: %v", err)
	}
	
	if templateCount > 0 {
		// 如果有关联的任务模板，先删除它们
		if err := s.db.Where("repository_id = ?", repoID).Delete(&models.TaskTemplate{}).Error; err != nil {
			return fmt.Errorf("删除关联任务模板失败: %v", err)
		}
		logger.GetLogger().Infof("已删除仓库 %d 的 %d 个关联任务模板", repoID, templateCount)
	}

	// 先删除同步日志（避免外键约束）
	if err := s.db.Where("repository_id = ?", repoID).Delete(&models.GitSyncLog{}).Error; err != nil {
		logger.GetLogger().Warnf("删除同步日志失败: %v", err)
		// 继续删除仓库
	}
	
	// 删除仓库
	if err := s.db.Delete(&repo).Error; err != nil {
		return fmt.Errorf("删除仓库失败: %v", err)
	}

	// 通知Worker删除仓库
	if err := s.notifyWorkersDelete(&repo); err != nil {
		logger.GetLogger().Errorf("通知Worker删除失败: %v", err)
		// 不影响删除操作
	}

	// 从调度器中移除定时任务
	if scheduler := GetGitSyncScheduler(); scheduler != nil {
		scheduler.RemoveJob(repoID)
	}

	return nil
}

// GetByID 根据ID获取Git仓库
func (s *GitRepositoryService) GetByID(tenantID uint, repoID uint) (*models.GitRepository, error) {
	var repo models.GitRepository
	err := s.db.Preload("Credential").Where("id = ? AND tenant_id = ?", repoID, tenantID).First(&repo).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("仓库不存在")
		}
		return nil, err
	}

	return &repo, nil
}

// List 获取Git仓库列表
func (s *GitRepositoryService) List(tenantID uint, query *ListGitRepositoryQuery) ([]*models.GitRepository, int64, error) {
	db := s.db.Model(&models.GitRepository{}).Where("tenant_id = ?", tenantID)

	// 搜索条件
	if query.Search != "" {
		search := "%" + query.Search + "%"
		db = db.Where("name LIKE ? OR code LIKE ? OR url LIKE ?", search, search, search)
	}

	// 状态过滤
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	// 同步状态过滤
	if query.SyncEnabled != nil {
		db = db.Where("sync_enabled = ?", *query.SyncEnabled)
	}

	// 获取总数
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	if query.OrderBy == "" {
		query.OrderBy = "created_at DESC"
	}
	db = db.Order(query.OrderBy)

	// 分页
	if query.Page > 0 && query.PageSize > 0 {
		offset := (query.Page - 1) * query.PageSize
		db = db.Offset(offset).Limit(query.PageSize)
	}

	// 查询数据
	var repos []*models.GitRepository
	if err := db.Preload("Credential").Find(&repos).Error; err != nil {
		return nil, 0, err
	}

	return repos, total, nil
}

// GetSyncLogs 获取同步日志
func (s *GitRepositoryService) GetSyncLogs(tenantID uint, repoID uint, query *ListSyncLogsQuery) ([]*models.GitSyncLog, int64, error) {
	// 验证仓库归属
	var repo models.GitRepository
	if err := s.db.Where("id = ? AND tenant_id = ?", repoID, tenantID).First(&repo).Error; err != nil {
		return nil, 0, fmt.Errorf("仓库不存在")
	}

	db := s.db.Model(&models.GitSyncLog{}).Where("repository_id = ?", repoID)

	// 状态过滤
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	// 时间范围过滤
	if query.StartTime != nil {
		db = db.Where("started_at >= ?", *query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("started_at <= ?", *query.EndTime)
	}

	// 获取总数
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序（默认按时间倒序）
	db = db.Order("started_at DESC")

	// 分页
	if query.Page > 0 && query.PageSize > 0 {
		offset := (query.Page - 1) * query.PageSize
		db = db.Offset(offset).Limit(query.PageSize)
	}

	// 查询数据（预加载仓库信息）
	var logs []*models.GitSyncLog
	if err := db.Preload("Repository.Tenant").Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// BatchSync 批量同步仓库
func (s *GitRepositoryService) BatchSync(tenantID uint, repositoryIDs []uint, operatorID uint) []BatchSyncResult {
	results := make([]BatchSyncResult, 0, len(repositoryIDs))
	
	for _, repoID := range repositoryIDs {
		result := BatchSyncResult{
			RepositoryID: repoID,
			Success:      true,
		}
		
		if err := s.ManualSync(tenantID, repoID, operatorID); err != nil {
			result.Success = false
			result.Message = err.Error()
		} else {
			result.Message = "同步任务已触发"
		}
		
		results = append(results, result)
	}
	
	return results
}

// ManualSync 手动同步仓库
func (s *GitRepositoryService) ManualSync(tenantID uint, repoID uint, operatorID uint) error {
	// 验证仓库
	var repo models.GitRepository
	if err := s.db.Where("id = ? AND tenant_id = ?", repoID, tenantID).First(&repo).Error; err != nil {
		return fmt.Errorf("仓库不存在")
	}

	// 注意：不在Master端创建同步日志，由每个Worker自己创建
	// 这样可以确保每个Worker只更新自己的日志

	// 通知所有Worker执行同步，并传递操作者ID
	if err := s.notifyWorkersSyncWithOperator(&repo, &operatorID); err != nil {
		return fmt.Errorf("通知Worker同步失败: %v", err)
	}

	return nil
}

// FileNode 文件节点（与worker端结构一致）
type FileNode struct {
	ID         string     `json:"id"`                  // 唯一标识符
	Name       string     `json:"name"`                // 文件或目录名
	Path       string     `json:"path"`                // 相对于仓库根目录的路径
	Type       string     `json:"type"`                // file/directory
	FileType   string     `json:"file_type,omitempty"` // 文件类型：ansible/shell/template/survey
	Size       int64      `json:"size"`                // 文件大小（字节）
	Selectable bool       `json:"selectable"`          // 是否可选（只有文件可选）
	Children   []FileNode `json:"children,omitempty"`  // 子节点（目录才有）
}

// SurveyFile Survey文件信息
type SurveyFile struct {
	Path        string                  `json:"path"`        // survey文件相对路径
	Name        string                  `json:"name"`        // survey名称
	Description string                  `json:"description"` // survey描述
	Parameters  []ScanTemplateParameter `json:"parameters"`  // 参数列表
}

// ScanResult 扫描结果（增强版）
type ScanResult struct {
	// Survey文件列表
	Surveys []SurveyFile `json:"surveys"`
	
	// 完整文件树
	FileTree *FileNode `json:"file_tree"`
	
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

// ScanTemplateParameter 扫描结果中的模板参数
type ScanTemplateParameter struct {
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

// ScanTemplates 扫描仓库中的模板（通过Redis返回结果）
func (s *GitRepositoryService) ScanTemplates(tenantID uint, repoID uint) (*ScanResult, error) {
	// 验证仓库
	var repo models.GitRepository
	if err := s.db.Where("id = ? AND tenant_id = ?", repoID, tenantID).First(&repo).Error; err != nil {
		return nil, fmt.Errorf("仓库不存在")
	}

	if s.queue == nil {
		return nil, fmt.Errorf("Redis队列未初始化")
	}

	// 生成扫描任务ID
	taskID := fmt.Sprintf("scan_%d_%d_%d", tenantID, repoID, time.Now().Unix())
	
	// 构建扫描消息
	msg := struct {
		TaskID       string            `json:"task_id"`
		Action       string            `json:"action"`
		RepositoryID uint              `json:"repository_id"`
		TenantID     uint              `json:"tenant_id"`
		Repository   struct {
			ID        uint   `json:"id"`
			Name      string `json:"name"`
			LocalPath string `json:"local_path"`
		} `json:"repository"`
		Timestamp time.Time         `json:"timestamp"`
		Metadata  map[string]string `json:"metadata"`
	}{
		TaskID:       taskID,
		Action:       "scan",
		RepositoryID: repo.ID,
		TenantID:     repo.TenantID,
		Repository: struct {
			ID        uint   `json:"id"`
			Name      string `json:"name"`
			LocalPath string `json:"local_path"`
		}{
			ID:        repo.ID,
			Name:      repo.Name,
			LocalPath: repo.LocalPath,
		},
		Timestamp: time.Now(),
		Metadata: map[string]string{
			"enhanced": "true", // 使用增强扫描
		},
	}

	// 发送到扫描队列
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("序列化消息失败: %v", err)
	}

	ctx := context.Background()
	if err := s.queue.GetClient().LPush(ctx, "git:scan:queue", msgBytes).Err(); err != nil {
		return nil, fmt.Errorf("发送扫描任务失败: %v", err)
	}

	// 等待结果（最多等待30秒）
	resultKey := fmt.Sprintf("scan:result:%s", taskID)
	timeout := 30 * time.Second
	startTime := time.Now()

	for {
		// 检查是否超时
		if time.Since(startTime) > timeout {
			return nil, fmt.Errorf("扫描超时")
		}

		// 尝试获取结果
		resultData, err := s.queue.GetClient().Get(ctx, resultKey).Result()
		if err == nil {
			// 解析结果
			var result ScanResult
			if err := json.Unmarshal([]byte(resultData), &result); err != nil {
				return nil, fmt.Errorf("解析扫描结果失败: %v", err)
			}

			// 删除结果键
			s.queue.GetClient().Del(ctx, resultKey)
			
			return &result, nil
		}

		// 等待一小段时间再重试
		time.Sleep(500 * time.Millisecond)
	}
}

// ValidateOwnership 验证仓库归属
func (s *GitRepositoryService) ValidateOwnership(tenantID uint, repoID uint) error {
	var count int64
	s.db.Model(&models.GitRepository{}).Where("id = ? AND tenant_id = ?", repoID, tenantID).Count(&count)
	if count == 0 {
		return fmt.Errorf("仓库不存在")
	}
	return nil
}

// 辅助函数：验证Git URL格式
func isValidGitURL(url string) bool {
	// 简单验证，支持 http(s):// 和 git@ 格式
	url = strings.ToLower(url)
	return strings.HasPrefix(url, "http://") ||
		strings.HasPrefix(url, "https://") ||
		strings.HasPrefix(url, "git@") ||
		strings.HasPrefix(url, "ssh://")
}

// 请求和查询结构体

// CreateGitRepositoryRequest 创建Git仓库请求
type CreateGitRepositoryRequest struct {
	Name         string `json:"name" binding:"required,min=1,max=100"`
	Code         string `json:"code" binding:"required,min=1,max=50"`
	Description  string `json:"description"`
	URL          string `json:"url" binding:"required,min=1,max=500"`
	Branch       string `json:"branch"`
	IsPublic     bool   `json:"is_public"`
	CredentialID *uint  `json:"credential_id"`
	SyncEnabled  bool   `json:"sync_enabled"`
	SyncCron     string `json:"sync_cron"`
}

// UpdateGitRepositoryRequest 更新Git仓库请求
type UpdateGitRepositoryRequest struct {
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	Branch       string  `json:"branch"`
	IsPublic     *bool   `json:"is_public"`
	CredentialID *uint   `json:"credential_id"`
	SyncEnabled  *bool   `json:"sync_enabled"`
	SyncCron     *string `json:"sync_cron"`
}

// ListGitRepositoryQuery 查询Git仓库列表参数
type ListGitRepositoryQuery struct {
	Page        int    `form:"page,default=1"`
	PageSize    int    `form:"page_size,default=10"`
	Search      string `form:"search"`
	Status      string `form:"status"`
	SyncEnabled *bool  `form:"sync_enabled"`
	OrderBy     string `form:"order_by"`
}

// ListSyncLogsQuery 查询同步日志参数
type ListSyncLogsQuery struct {
	Page      int        `form:"page,default=1"`
	PageSize  int        `form:"page_size,default=10"`
	Status    string     `form:"status"`
	StartTime *time.Time `form:"start_time"`
	EndTime   *time.Time `form:"end_time"`
}

// notifyWorkersSync 通知所有Worker同步Git仓库
func (s *GitRepositoryService) notifyWorkersSync(repo *models.GitRepository) error {
	return s.notifyWorkersSyncWithOperator(repo, nil)
}

// notifyWorkersSyncWithOperator 通知所有Worker同步Git仓库（带操作者ID）
func (s *GitRepositoryService) notifyWorkersSyncWithOperator(repo *models.GitRepository, operatorID *uint) error {
	if s.queue == nil {
		logger.GetLogger().Warn("Redis队列未初始化，跳过Worker通知")
		return nil
	}

	// 获取解密后的凭证（如果需要）
	var credentialInfo *struct {
		Type       string `json:"type"`
		Username   string `json:"username"`
		Password   string `json:"password,omitempty"`
		PrivateKey string `json:"private_key,omitempty"`
		Passphrase string `json:"passphrase,omitempty"`
	}
	
	if repo.CredentialID != nil && !repo.IsPublic {
		// 获取解密后的凭证
		credential, err := s.credentialService.GetDecrypted(*repo.CredentialID, repo.TenantID, 0, "Git仓库同步")
		if err != nil {
			logger.GetLogger().Errorf("获取凭证失败: %v", err)
			// 不影响同步，Worker会记录错误
		} else {
			credentialInfo = &struct {
				Type       string `json:"type"`
				Username   string `json:"username"`
				Password   string `json:"password,omitempty"`
				PrivateKey string `json:"private_key,omitempty"`
				Passphrase string `json:"passphrase,omitempty"`
			}{
				Type:       string(credential.Type),
				Username:   credential.Username,
				Password:   credential.Password,
				PrivateKey: credential.PrivateKey,
				Passphrase: credential.Passphrase,
			}
		}
	}
	
	// 构建同步消息（与worker-dist的types.GitSyncMessage结构一致）
	msg := struct {
		Action       string    `json:"action"`
		RepositoryID uint      `json:"repository_id"`
		TenantID     uint      `json:"tenant_id"`
		Repository   struct {
			ID           uint   `json:"id"`
			Name         string `json:"name"`
			URL          string `json:"url"`
			Branch       string `json:"branch"`
			IsPublic     bool   `json:"is_public"`
			CredentialID *uint  `json:"credential_id,omitempty"`
			Credential   *struct {
				Type       string `json:"type"`
				Username   string `json:"username"`
				Password   string `json:"password,omitempty"`
				PrivateKey string `json:"private_key,omitempty"`
				Passphrase string `json:"passphrase,omitempty"`
			} `json:"credential,omitempty"`
			LocalPath    string `json:"local_path"`
		} `json:"repository"`
		OperatorID *uint              `json:"operator_id,omitempty"`
		Timestamp  time.Time          `json:"timestamp"`
		Metadata   map[string]string  `json:"metadata,omitempty"`
	}{
		Action:       "sync",
		RepositoryID: repo.ID,
		TenantID:     repo.TenantID,
		Repository: struct {
			ID           uint   `json:"id"`
			Name         string `json:"name"`
			URL          string `json:"url"`
			Branch       string `json:"branch"`
			IsPublic     bool   `json:"is_public"`
			CredentialID *uint  `json:"credential_id,omitempty"`
			Credential   *struct {
				Type       string `json:"type"`
				Username   string `json:"username"`
				Password   string `json:"password,omitempty"`
				PrivateKey string `json:"private_key,omitempty"`
				Passphrase string `json:"passphrase,omitempty"`
			} `json:"credential,omitempty"`
			LocalPath    string `json:"local_path"`
		}{
			ID:           repo.ID,
			Name:         repo.Name,
			URL:          repo.URL,
			Branch:       repo.Branch,
			IsPublic:     repo.IsPublic,
			CredentialID: repo.CredentialID,
			Credential:   credentialInfo,
			LocalPath:    repo.LocalPath,
		},
		OperatorID: operatorID,
		Timestamp:  time.Now(),
	}

	// 发布到Redis订阅通道
	channel := fmt.Sprintf("git:sync:%d", repo.ID)
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %v", err)
	}
	
	ctx := context.Background()
	if err := s.queue.GetClient().Publish(ctx, channel, msgBytes).Err(); err != nil {
		return fmt.Errorf("发布同步通知失败: %v", err)
	}

	logger.GetLogger().WithFields(map[string]interface{}{
		"repository_id": repo.ID,
		"tenant_id":     repo.TenantID,
		"action":        "sync",
	}).Info("已发布Git仓库同步通知")

	return nil
}


// notifyWorkersDelete 通知所有Worker删除Git仓库
func (s *GitRepositoryService) notifyWorkersDelete(repo *models.GitRepository) error {
	if s.queue == nil {
		logger.GetLogger().Warn("Redis队列未初始化，跳过Worker通知")
		return nil
	}

	// 构建删除消息（与worker-dist的types.GitSyncMessage结构一致）
	msg := struct {
		Action       string    `json:"action"`
		RepositoryID uint      `json:"repository_id"`
		TenantID     uint      `json:"tenant_id"`
		Repository   struct {
			ID           uint   `json:"id"`
			Name         string `json:"name"`
			URL          string `json:"url"`
			Branch       string `json:"branch"`
			IsPublic     bool   `json:"is_public"`
			CredentialID *uint  `json:"credential_id,omitempty"`
			LocalPath    string `json:"local_path"`
		} `json:"repository"`
		OperatorID *uint              `json:"operator_id,omitempty"`
		Timestamp  time.Time          `json:"timestamp"`
		Metadata   map[string]string  `json:"metadata,omitempty"`
	}{
		Action:       "delete",
		RepositoryID: repo.ID,
		TenantID:     repo.TenantID,
		Repository: struct {
			ID           uint   `json:"id"`
			Name         string `json:"name"`
			URL          string `json:"url"`
			Branch       string `json:"branch"`
			IsPublic     bool   `json:"is_public"`
			CredentialID *uint  `json:"credential_id,omitempty"`
			LocalPath    string `json:"local_path"`
		}{
			ID:        repo.ID,
			Name:      repo.Name,
			URL:       repo.URL,
			Branch:    repo.Branch,
			IsPublic:  repo.IsPublic,
			LocalPath: repo.LocalPath,
		},
		Timestamp:    time.Now(),
	}

	// 发布到Redis订阅通道
	channel := fmt.Sprintf("git:sync:%d", repo.ID)
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %v", err)
	}
	
	ctx := context.Background()
	if err := s.queue.GetClient().Publish(ctx, channel, msgBytes).Err(); err != nil {
		return fmt.Errorf("发布删除通知失败: %v", err)
	}

	logger.GetLogger().WithFields(map[string]interface{}{
		"repository_id": repo.ID,
		"tenant_id":     repo.TenantID,
		"action":        "delete",
	}).Info("已发布Git仓库删除通知")

	return nil
}
