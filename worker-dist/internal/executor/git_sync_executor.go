package executor

import (
	"ahop-worker/internal/types"
	"ahop-worker/pkg/auth"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GitSyncLog Git同步日志（与Master的models.GitSyncLog对应）
type GitSyncLog struct {
	ID           uint       `gorm:"primarykey"`
	RepositoryID uint       `gorm:"not null;index"`
	TenantID     uint       `gorm:"not null;index"`
	TaskID       string     `gorm:"size:36;index"`
	TaskType     string     `gorm:"size:20;not null"`
	WorkerID     string     `gorm:"size:100;not null"`
	OperatorID   *uint      `gorm:"index"`
	StartedAt    time.Time  `gorm:"not null"`
	FinishedAt   *time.Time
	Duration     int
	Status       string     `gorm:"size:20;not null;default:'pending'"`
	FromCommit   string     `gorm:"size:40"`
	ToCommit     string     `gorm:"size:40"`
	ErrorMessage string     `gorm:"type:text"`
	LocalPath    string     `gorm:"size:500"`
	CommandOutput string    `gorm:"type:text"`
	CreatedAt    time.Time
}

func (GitSyncLog) TableName() string {
	return "git_sync_logs"
}

// CredentialUsageLog 凭证使用日志
type CredentialUsageLog struct {
	ID           uint      `gorm:"primarykey"`
	CredentialID uint      `gorm:"not null;index"`
	TenantID     uint      `gorm:"not null;index"`
	UserID       uint      `gorm:"not null;index;column:user_id"`
	HostID       *uint     `gorm:"index;column:host_id"`
	HostName     string    `gorm:"size:255;column:host_name"`
	HostIP       string    `gorm:"size:45;column:host_ip"`
	Purpose      string    `gorm:"size:100;not null"`
	Success      bool      `gorm:"not null"`
	ErrorMessage string    `gorm:"type:text;column:error_message"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (CredentialUsageLog) TableName() string {
	return "credential_usage_logs"
}

// GitSyncExecutor Git同步执行器
type GitSyncExecutor struct {
	BaseExecutor
	db                     *gorm.DB
	baseDir                string
	authClient             *auth.AuthClient
	templateScanner        *ScannerAdapter
	enhancedScanner        *EnhancedScannerAdapter
	log                    *logrus.Logger // 日志实例
}


// ScanRepository 扫描仓库返回模板信息
func (e *GitSyncExecutor) ScanRepository(repoPath string) ([]*TemplateInfo, error) {
	return e.templateScanner.ScanRepository(repoPath)
}

// ScanRepositoryEnhanced 增强扫描仓库（返回文件树和survey信息）
func (e *GitSyncExecutor) ScanRepositoryEnhanced(repoPath string, repoID uint, repoName string) (*EnhancedScanResult, error) {
	return e.enhancedScanner.ScanRepository(repoPath, repoID, repoName)
}

// NewGitSyncExecutor 创建Git同步执行器
func NewGitSyncExecutor(db *gorm.DB, authClient *auth.AuthClient, baseDir string, log *logrus.Logger) *GitSyncExecutor {
	if baseDir == "" {
		baseDir = "/data/ahop/repos"
	}

	// 确保基础目录存在
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		log.Errorf("创建Git仓库基础目录失败: %v", err)
	}

	return &GitSyncExecutor{
		db:              db,
		baseDir:         baseDir,
		authClient:      authClient,
		templateScanner: NewScannerAdapter(),
		enhancedScanner: NewEnhancedScannerAdapter(log),
		log:             log,
	}
}

// GetSupportedTypes 获取支持的任务类型
func (e *GitSyncExecutor) GetSupportedTypes() []string {
	return []string{"git_sync"}
}

// ValidateParams 验证参数
func (e *GitSyncExecutor) ValidateParams(params map[string]interface{}) error {
	// Git同步参数通过消息传递，这里不需要验证
	return nil
}

// Execute 执行Git同步（这个方法用于普通任务队列，Git同步通常通过订阅消息触发）
func (e *GitSyncExecutor) Execute(ctx context.Context, task *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
	return &TaskResult{
		Success: false,
		Error:   "Git同步应通过订阅消息触发，而不是任务队列",
		Details: make(map[string]interface{}),
		Logs:    []string{"错误：Git同步应通过订阅消息触发"},
	}
}

// ProcessGitSyncMessage 处理Git同步消息
func (e *GitSyncExecutor) ProcessGitSyncMessage(msg *types.GitSyncMessage, workerID string) error {
	log := e.log.WithFields(logrus.Fields{
		"repository_id": msg.RepositoryID,
		"tenant_id":     msg.TenantID,
		"action":        msg.Action,
		"worker_id":     workerID,
	})

	// 忽略scan动作，scan通过队列处理
	if msg.Action == "scan" {
		log.Info("忽略scan动作消息，scan通过队列处理")
		return nil
	}

	startTime := time.Now()
	
	// 查找最新的pending状态的同步日志
	var syncLog GitSyncLog
	e.log.WithFields(logrus.Fields{
		"repository_id": msg.RepositoryID,
		"tenant_id": msg.TenantID,
	}).Debug("开始查找pending状态的同步日志")
	
	// 先查询所有的同步日志看看
	var allLogs []GitSyncLog
	if err := e.db.Where("repository_id = ?", msg.RepositoryID).Find(&allLogs).Error; err == nil {
		e.log.WithFields(logrus.Fields{
			"count": len(allLogs),
			"logs": allLogs,
		}).Debug("查询到的所有同步日志")
	}
	
	findLogErr := e.db.Where("repository_id = ? AND status = ? AND tenant_id = ? AND worker_id = ?", 
		msg.RepositoryID, "pending", msg.TenantID, workerID).
		Order("created_at DESC").
		First(&syncLog).Error
	
	if findLogErr != nil {
		// 如果找不到属于自己的同步日志，创建一个
		e.log.Info("未找到属于本 Worker 的同步日志，创建新日志")
		
		// 确定任务类型
		taskType := "scheduled" // 默认为定时任务
		if msg.OperatorID != nil {
			taskType = "manual" // 有操作者ID表示手动触发
			if msg.Metadata != nil && msg.Metadata["scan"] == "true" {
				taskType = "manual_scan" // 手动触发并扫描
			}
		}
		
		syncLog = GitSyncLog{
			RepositoryID: msg.RepositoryID,
			TenantID:     msg.TenantID,
			TaskType:     taskType,
			WorkerID:     workerID,
			OperatorID:   msg.OperatorID,
			Status:       "pending",
			StartedAt:    time.Now(),
			CreatedAt:    time.Now(),
		}
		
		if createErr := e.db.Create(&syncLog).Error; createErr != nil {
			e.log.WithError(createErr).Error("创建同步日志失败")
			// 不影响同步执行
		} else {
			e.log.WithFields(logrus.Fields{
				"sync_log_id": syncLog.ID,
				"worker_id":   syncLog.WorkerID,
			}).Info("成功创建同步日志")
			findLogErr = nil // 标记找到了日志
		}
	} else {
		e.log.WithFields(logrus.Fields{
			"sync_log_id": syncLog.ID,
			"worker_id": syncLog.WorkerID,
		}).Debug("找到pending状态的同步日志")
	}

	// 创建同步日志（简化版，实际的日志记录应该在主服务器端）
	e.log.Info("开始处理Git同步任务")

	// 执行同步
	var err error
	var repoPath, fromCommit, toCommit string
	
	switch msg.Action {
	case "sync":
		repoPath, fromCommit, toCommit, err = e.syncRepository(msg, workerID, log)
	case "delete":
		err = e.deleteRepository(msg, log)
	default:
		err = fmt.Errorf("未知的同步动作: %s", msg.Action)
	}

	duration := time.Since(startTime)
	
	// 更新同步日志
	if findLogErr == nil {
		finishedAt := time.Now()
		updateData := map[string]interface{}{
			"worker_id":   workerID,
			"finished_at": finishedAt,
			"duration":    int(duration.Milliseconds()), // 使用毫秒更精确
		}
		
		if err != nil {
			updateData["status"] = "failed"
			updateData["error_message"] = err.Error()
		} else {
			updateData["status"] = "success"
			updateData["error_message"] = "" // 清空错误信息
		}
		
		// 如果是同步操作，记录详细信息
		if msg.Action == "sync" && err == nil {
			updateData["local_path"] = repoPath
			if fromCommit != "" {
				updateData["from_commit"] = fromCommit
			}
			if toCommit != "" {
				updateData["to_commit"] = toCommit
			}
		}
		
		e.log.WithFields(logrus.Fields{
			"sync_log_id": syncLog.ID,
			"update_data": updateData,
		}).Debug("准备更新同步日志")
		
		if updateErr := e.db.Model(&syncLog).Updates(updateData).Error; updateErr != nil {
			e.log.WithError(updateErr).Error("更新同步日志失败")
		} else {
			e.log.WithFields(logrus.Fields{
				"sync_log_id": syncLog.ID,
				"status": updateData["status"],
			}).Info("成功更新同步日志")
		}
	} else {
		e.log.WithError(findLogErr).Warn("无法更新同步日志，因为未找到对应的pending日志")
	}
	
	if err != nil {
		log.WithError(err).Errorf("Git仓库同步失败，耗时: %s", duration)
		return err
	}

	e.log.Infof("Git仓库同步成功，耗时: %s", duration)
	return nil
}

// syncRepository 同步仓库
func (e *GitSyncExecutor) syncRepository(msg *types.GitSyncMessage, workerID string, log *logrus.Entry) (string, string, string, error) {
	// 使用Master指定的LocalPath
	if msg.Repository.LocalPath == "" {
		return "", "", "", fmt.Errorf("仓库本地路径为空")
	}
	
	// 构建完整路径：基础目录 + Master指定的相对路径
	repoPath := filepath.Join(e.baseDir, msg.Repository.LocalPath)
	var needClone bool
	
	// 检查仓库是否已存在
	gitDirPath := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDirPath); os.IsNotExist(err) {
		// .git目录不存在
		needClone = true
		
		// 检查目录是否存在但不是Git仓库
		if _, dirErr := os.Stat(repoPath); dirErr == nil {
			// 目录存在但不是Git仓库，删除它
			log.Warnf("目录存在但不是Git仓库，将删除: %s", repoPath)
			if removeErr := os.RemoveAll(repoPath); removeErr != nil {
				return "", "", "", fmt.Errorf("删除非Git目录失败: %v", removeErr)
			}
		}
		
		log.Infof("仓库不存在，将克隆到: %s", repoPath)
	} else {
		// 仓库已存在，拉取更新
		needClone = false
		log.Infof("仓库已存在，将拉取更新: %s", repoPath)
	}

	// 确保基础目录存在
	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		return "", "", "", fmt.Errorf("创建基础目录失败: %v", err)
	}

	// 执行同步
	var syncErr error
	var fromCommit, toCommit string
	
	if needClone {
		// 需要克隆
		toCommit, syncErr = e.cloneRepository(msg, repoPath, log)
		fromCommit = ""
	} else {
		// 需要拉取更新
		fromCommit, toCommit, syncErr = e.pullRepository(msg, repoPath, log)
	}
	
	if syncErr != nil {
		return "", "", "", syncErr
	}
	
	// 如果消息中包含扫描标记，则执行扫描
	if msg.Metadata != nil && msg.Metadata["scan"] == "true" {
		e.log.Info("开始扫描仓库脚本...")
		templates, err := e.templateScanner.ScanRepository(repoPath)
		if err != nil {
			log.WithError(err).Error("扫描仓库脚本失败")
			// 扫描失败不影响同步成功状态，只记录日志
		} else {
			log.Infof("扫描完成，发现 %d 个脚本模板", len(templates))
			// TODO: 后续这里可以根据需要处理扫描结果
		}
	}
	
	return repoPath, fromCommit, toCommit, nil
}

// cloneRepository 克隆仓库
func (e *GitSyncExecutor) cloneRepository(msg *types.GitSyncMessage, repoPath string, log *logrus.Entry) (string, error) {
	// 创建目录
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return "", fmt.Errorf("创建仓库目录失败: %v", err)
	}

	// 构建克隆命令
	args := []string{"clone", "--progress"}

	// 添加分支参数
	if msg.Repository.Branch != "" && msg.Repository.Branch != "main" && msg.Repository.Branch != "master" {
		args = append(args, "-b", msg.Repository.Branch)
	}

	// 处理认证
	cloneURL := msg.Repository.URL
	var tempCleanup func()

	var credUsageLog *CredentialUsageLog
	if !msg.Repository.IsPublic && msg.Repository.CredentialID != nil {
		authURL, cleanup, usageLog, err := e.buildAuthURL(msg)
		if err != nil {
			return "", fmt.Errorf("构建认证失败: %v", err)
		}
		cloneURL = authURL
		tempCleanup = cleanup
		credUsageLog = usageLog
		defer func() {
			if tempCleanup != nil {
				tempCleanup()
			}
		}()
	}

	args = append(args, cloneURL, repoPath)

	// 执行克隆（不重试）
	cmd := exec.Command("git", args...)
	cmd.Env = os.Environ() // 继承环境变量（包括GIT_SSH）

	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	output, lastErr := cmd.CombinedOutput()

	// 更新凭证使用日志
	if credUsageLog != nil && lastErr != nil {
		// 如果克隆失败，更新凭证使用日志为失败
		credUsageLog.Success = false
		credUsageLog.ErrorMessage = fmt.Sprintf("git clone failed: %v", lastErr)
		e.db.Model(credUsageLog).Updates(map[string]interface{}{
			"success": false,
			"error_message": credUsageLog.ErrorMessage,
		})
	}

	if lastErr != nil {
		// 清理失败的目录
		os.RemoveAll(repoPath)
		return "", fmt.Errorf("git clone 失败: %v, 输出: %s", lastErr, output)
	}

	e.log.Infof("成功克隆仓库到: %s", repoPath)
	
	// 获取最新的commit hash
	commitCmd := exec.Command("git", "rev-parse", "HEAD")
	commitCmd.Dir = repoPath
	commitOutput, err := commitCmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取commit hash失败: %v", err)
	}
	
	return strings.TrimSpace(string(commitOutput)), nil
}

// pullRepository 拉取仓库更新
func (e *GitSyncExecutor) pullRepository(msg *types.GitSyncMessage, repoPath string, log *logrus.Entry) (string, string, error) {
	// 处理认证（如果需要）
	var tempCleanup func()
	var credUsageLog *CredentialUsageLog
	if !msg.Repository.IsPublic && msg.Repository.CredentialID != nil {
		_, cleanup, usageLog, err := e.buildAuthURL(msg)
		if err != nil {
			return "", "", fmt.Errorf("构建认证失败: %v", err)
		}
		tempCleanup = cleanup
		credUsageLog = usageLog
		defer func() {
			if tempCleanup != nil {
				tempCleanup()
			}
		}()
	}

	// 先获取当前的commit hash
	currentCommitCmd := exec.Command("git", "rev-parse", "HEAD")
	currentCommitCmd.Dir = repoPath
	currentCommitOutput, _ := currentCommitCmd.Output()
	fromCommit := strings.TrimSpace(string(currentCommitOutput))
	
	// 先fetch再merge，这样更安全
	fetchCmd := exec.Command("git", "fetch", "origin", msg.Repository.Branch)
	fetchCmd.Dir = repoPath
	fetchCmd.Env = os.Environ() // 继承环境变量（包括GIT_SSH）

	var fetchErr error
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		fetchErr = fmt.Errorf("git fetch 失败: %v, 输出: %s", err, output)
	}

	if fetchErr == nil {
		// 执行merge
		mergeCmd := exec.Command("git", "merge", fmt.Sprintf("origin/%s", msg.Repository.Branch))
		mergeCmd.Dir = repoPath

		if output, err := mergeCmd.CombinedOutput(); err != nil {
			fetchErr = fmt.Errorf("git merge 失败: %v, 输出: %s", err, output)
		}
	}

	// 更新凭证使用日志
	if credUsageLog != nil && fetchErr != nil {
		// 如果拉取失败，更新凭证使用日志为失败
		credUsageLog.Success = false
		credUsageLog.ErrorMessage = fetchErr.Error()
		e.db.Model(credUsageLog).Updates(map[string]interface{}{
			"success": false,
			"error_message": credUsageLog.ErrorMessage,
		})
	}

	if fetchErr != nil {
		return "", "", fetchErr
	}

	e.log.Info("成功更新仓库")
	
	// 获取更新后的commit hash
	commitCmd := exec.Command("git", "rev-parse", "HEAD")
	commitCmd.Dir = repoPath
	commitOutput, err := commitCmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("获取commit hash失败: %v", err)
	}
	
	return fromCommit, strings.TrimSpace(string(commitOutput)), nil
}

// buildAuthURL 构建带认证的URL（返回URL和清理函数）
func (e *GitSyncExecutor) buildAuthURL(msg *types.GitSyncMessage) (string, func(), *CredentialUsageLog, error) {
	if msg.Repository.CredentialID == nil {
		return msg.Repository.URL, nil, nil, nil
	}

	// 优先使用消息中的凭证
	var credential *types.CredentialInfo
	if msg.Repository.Credential != nil {
		// 使用消息中已解密的凭证
		credential = msg.Repository.Credential
	} else {
		// 如果消息中没有凭证，从主服务器获取（兼容旧版本）
		cred, err := e.authClient.GetDecryptedCredential(*msg.Repository.CredentialID, msg.TenantID)
		if err != nil {
			// 记录凭证使用失败日志
			usageLog := CredentialUsageLog{
				CredentialID: *msg.Repository.CredentialID,
				TenantID:     msg.TenantID,
				UserID:       1, // TODO: 从消息中获取操作者ID
				Purpose:      fmt.Sprintf("Git仓库同步 - %s", msg.Repository.Name),
				HostName:     msg.Repository.URL,
				Success:      false,
				ErrorMessage: err.Error(),
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			e.db.Create(&usageLog)
			return "", nil, &usageLog, fmt.Errorf("获取凭证失败: %v", err)
		}
		credential = cred
	}
	
	// 记录凭证使用成功日志（延迟记录，因为还不知道最终是否成功）
	usageLog := CredentialUsageLog{
		CredentialID: *msg.Repository.CredentialID,
		TenantID:     msg.TenantID,
		UserID:       1, // TODO: 从消息中获取操作者ID
		Purpose:      fmt.Sprintf("Git仓库同步 - %s", msg.Repository.Name),
		HostName:     msg.Repository.URL,
		Success:      true, // 先假设成功，如果失败会在外层更新
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	
	// 如果有操作者ID，使用它
	if msg.OperatorID != nil {
		usageLog.UserID = *msg.OperatorID
	}

	// 根据凭证类型构建认证URL
	switch credential.Type {
	case "password":
		// HTTPS with username/password
		username := credential.Username
		password := credential.Password

		// 解析URL并插入认证信息
		if strings.HasPrefix(msg.Repository.URL, "https://") {
			authPart := fmt.Sprintf("%s:%s@", username, password)
			authURL := strings.Replace(msg.Repository.URL, "https://", "https://"+authPart, 1)
			e.db.Create(&usageLog)
			return authURL, nil, &usageLog, nil
		}
		return "", nil, &usageLog, fmt.Errorf("密码认证仅支持HTTPS协议")

	case "ssh_key":
		// SSH认证需要配置SSH
		url, cleanup, err := e.setupSSHAuth(msg, credential)
		if err == nil {
			e.db.Create(&usageLog)
		} else {
			usageLog.Success = false
			usageLog.ErrorMessage = err.Error()
			usageLog.UpdatedAt = time.Now()
			e.db.Create(&usageLog)
		}
		return url, cleanup, &usageLog, err

	default:
		usageLog.Success = false
		usageLog.ErrorMessage = fmt.Sprintf("不支持的凭证类型: %s", credential.Type)
		usageLog.UpdatedAt = time.Now()
		e.db.Create(&usageLog)
		return "", nil, &usageLog, fmt.Errorf("不支持的凭证类型: %s", credential.Type)
	}
}

// setupSSHAuth 设置SSH认证（返回URL和清理函数）
func (e *GitSyncExecutor) setupSSHAuth(msg *types.GitSyncMessage, credential *types.CredentialInfo) (string, func(), error) {
	privateKey := credential.PrivateKey
	if privateKey == "" {
		return "", nil, fmt.Errorf("SSH凭证缺少私钥")
	}

	// 创建临时目录存放SSH密钥
	tempDir, err := os.MkdirTemp("", "git-ssh-*")
	if err != nil {
		return "", nil, fmt.Errorf("创建临时目录失败: %v", err)
	}

	// 写入私钥文件
	keyPath := filepath.Join(tempDir, "id_rsa")
	if err := os.WriteFile(keyPath, []byte(privateKey), 0600); err != nil {
		os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("写入私钥文件失败: %v", err)
	}

	// 创建SSH配置脚本
	sshConfig := fmt.Sprintf(`#!/bin/bash
ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null $@
`, keyPath)

	sshScriptPath := filepath.Join(tempDir, "git-ssh.sh")
	if err := os.WriteFile(sshScriptPath, []byte(sshConfig), 0700); err != nil {
		os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("创建SSH脚本失败: %v", err)
	}

	// 设置环境变量（在执行git命令时使用）
	os.Setenv("GIT_SSH", sshScriptPath)

	// 创建清理函数
	cleanup := func() {
		os.Unsetenv("GIT_SSH")
		os.RemoveAll(tempDir)
		e.log.Debugf("清理SSH临时目录: %s", tempDir)
	}

	// 确保URL是SSH格式
	if !strings.HasPrefix(msg.Repository.URL, "git@") && !strings.Contains(msg.Repository.URL, "ssh://") {
		cleanup()
		return "", nil, fmt.Errorf("SSH认证需要SSH格式的Git URL")
	}

	return msg.Repository.URL, cleanup, nil
}

// deleteRepository 删除仓库
func (e *GitSyncExecutor) deleteRepository(msg *types.GitSyncMessage, log *logrus.Entry) error {
	// 使用Master指定的LocalPath
	if msg.Repository.LocalPath == "" {
		// 如果没有LocalPath，使用旧的模式匹配方式（兼容性）
		pattern := filepath.Join(e.baseDir, fmt.Sprintf("%d", msg.TenantID), fmt.Sprintf("%d", msg.RepositoryID), "*")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("查找仓库目录失败: %v", err)
		}
		
		// 删除所有匹配的目录
		for _, match := range matches {
			if err := os.RemoveAll(match); err != nil {
				e.log.Errorf("删除仓库目录 %s 失败: %v", match, err)
			} else {
				e.log.Infof("已删除仓库目录: %s", match)
			}
		}
	} else {
		// 使用Master指定的LocalPath
		repoPath := filepath.Join(e.baseDir, msg.Repository.LocalPath)
		
		// 检查目录是否存在
		if _, err := os.Stat(repoPath); err == nil {
			// 删除仓库目录
			if err := os.RemoveAll(repoPath); err != nil {
				e.log.Errorf("删除仓库目录 %s 失败: %v", repoPath, err)
				return fmt.Errorf("删除仓库目录失败: %v", err)
			}
			e.log.Infof("已删除仓库目录: %s", repoPath)
		} else if !os.IsNotExist(err) {
			// 如果不是"文件不存在"错误，返回错误
			return fmt.Errorf("检查仓库目录失败: %v", err)
		} else {
			// 目录不存在，不需要删除
			e.log.Infof("仓库目录不存在，无需删除: %s", repoPath)
		}
	}

	return nil
}
