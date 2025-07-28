package services

import (
	"ahop/internal/models"
	"ahop/pkg/config"
	"ahop/pkg/logger"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	// WorkerHeartbeatTimeout Worker心跳超时时间
	WorkerHeartbeatTimeout = 60 * time.Second
)

// WorkerAuthService Worker认证服务
type WorkerAuthService struct {
	db *gorm.DB
}

// NewWorkerAuthService 创建Worker认证服务实例
func NewWorkerAuthService(db *gorm.DB) *WorkerAuthService {
	return &WorkerAuthService{
		db: db,
	}
}

// CreateWorkerAuth 创建Worker授权
func (s *WorkerAuthService) CreateWorkerAuth(environment, description string) (*models.WorkerAuth, error) {
	accessKey := s.generateAccessKey()
	secretKey := s.generateSecretKey()

	// 从配置文件获取数据库配置
	cfg := config.GetConfig()

	// 解析数据库端口
	dbPort := 5432
	if port, err := strconv.Atoi(cfg.Database.Port); err == nil {
		dbPort = port
	}


	auth := &models.WorkerAuth{
		AccessKey:   accessKey,
		SecretKey:   secretKey,
		Environment: environment,
		Description: description,
		Status:      "active",
		// 数据库配置
		DBHost:     cfg.Database.Host,
		DBPort:     dbPort,
		DBUser:     cfg.Database.User,
		DBPassword: cfg.Database.Password,
		DBName:     cfg.Database.DBName,
		// Redis配置
		RedisHost:     cfg.Redis.Host,
		RedisPort:     cfg.Redis.Port,
		RedisPassword: cfg.Redis.Password,
		RedisDB:       cfg.Redis.DB,
		RedisPrefix:   cfg.Redis.Prefix,
	}

	if err := s.db.Create(auth).Error; err != nil {
		return nil, fmt.Errorf("创建Worker授权失败: %v", err)
	}

	return auth, nil
}

// ValidateAccessKey 验证AccessKey并返回认证信息
func (s *WorkerAuthService) ValidateAccessKey(accessKey string) (*models.WorkerAuth, error) {
	var auth models.WorkerAuth
	err := s.db.Where("access_key = ? AND status = ?", accessKey, "active").First(&auth).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("无效的AccessKey")
		}
		return nil, fmt.Errorf("查询AccessKey失败: %v", err)
	}

	return &auth, nil
}

// VerifySignature 验证请求签名
func (s *WorkerAuthService) VerifySignature(accessKey, workerID string, timestamp int64, signature, secretKey string) bool {
	expectedSig := s.calculateSignature(accessKey, workerID, timestamp, secretKey)
	return signature == expectedSig
}

// GetWorkerAuthList 获取Worker授权列表
func (s *WorkerAuthService) GetWorkerAuthList(page, pageSize int) ([]models.WorkerAuth, int64, error) {
	var auths []models.WorkerAuth
	var total int64

	// 计算总数
	if err := s.db.Model(&models.WorkerAuth{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := s.db.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&auths).Error
	if err != nil {
		return nil, 0, err
	}

	return auths, total, nil
}

// UpdateWorkerAuthStatus 更新Worker授权状态
func (s *WorkerAuthService) UpdateWorkerAuthStatus(id uint, status string) error {
	return s.db.Model(&models.WorkerAuth{}).Where("id = ?", id).Update("status", status).Error
}

// DeleteWorkerAuth 删除Worker授权
func (s *WorkerAuthService) DeleteWorkerAuth(id uint) error {
	return s.db.Delete(&models.WorkerAuth{}, id).Error
}

// generateAccessKey 生成AccessKey
func (s *WorkerAuthService) generateAccessKey() string {
	return "AHOP_AK_" + s.randomString(16)
}

// generateSecretKey 生成SecretKey
func (s *WorkerAuthService) generateSecretKey() string {
	return "AHOP_SK_" + s.randomString(32)
}

// randomString 生成随机字符串
func (s *WorkerAuthService) randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

// calculateSignature 计算请求签名
func (s *WorkerAuthService) calculateSignature(accessKey, workerID string, timestamp int64, secretKey string) string {
	// 构造待签名字符串
	stringToSign := fmt.Sprintf("%s|%s|%d", accessKey, workerID, timestamp)

	// HMAC-SHA256签名
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(stringToSign))

	return hex.EncodeToString(h.Sum(nil))
}

// IsValidTimestamp 验证时间戳有效性（防重放攻击）
func (s *WorkerAuthService) IsValidTimestamp(timestamp int64) bool {
	now := time.Now().Unix()
	// 允许5分钟的时钟偏差
	return now-timestamp <= 300 && timestamp-now <= 300
}

// LogWorkerAuth 记录Worker认证日志
func (s *WorkerAuthService) LogWorkerAuth(workerID, accessKey, result, ipAddress string) {
	log := logger.GetLogger()
	log.WithFields(logrus.Fields{
		"worker_id":  workerID,
		"access_key": accessKey,
		"result":     result,
		"ip_address": ipAddress,
		"timestamp":  time.Now(),
		"action":     "worker_auth",
	}).Info("Worker认证记录")
}

// RegisterWorkerConnection 注册Worker连接，确保Worker ID唯一性
func (s *WorkerAuthService) RegisterWorkerConnection(workerID, accessKey, ipAddress string) error {
	// 开启事务
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 查看是否有活跃的同名Worker
		var existingConn models.WorkerConnection
		err := tx.Where("worker_id = ? AND status = ?", workerID, "active").
			First(&existingConn).Error
		
		if err == nil {
			// 如果存在活跃连接，检查心跳时间
			if time.Since(existingConn.LastHeartbeat) > WorkerHeartbeatTimeout {
				// 心跳超时，标记为断开
				if err := tx.Model(&existingConn).Update("status", "disconnected").Error; err != nil {
					return err
				}
			} else {
				// 心跳正常，拒绝新连接
				return fmt.Errorf("Worker ID '%s' 已经被其他实例使用（IP: %s）", workerID, existingConn.IPAddress)
			}
		}
		
		// 2. 创建或更新连接记录
		now := time.Now()
		newConn := models.WorkerConnection{
			WorkerID:      workerID,
			IPAddress:     ipAddress,
			ConnectedAt:   now,
			LastHeartbeat: now,
			Status:        "active",
			AccessKey:     accessKey,
		}
		
		// 使用Upsert（存在则更新，不存在则创建）
		if err := tx.Where("worker_id = ?", workerID).FirstOrCreate(&newConn).Error; err != nil {
			return err
		}
		
		// 更新连接信息
		if err := tx.Model(&newConn).Updates(map[string]interface{}{
			"ip_address":     ipAddress,
			"connected_at":   now,
			"last_heartbeat": now,
			"status":         "active",
			"access_key":     accessKey,
		}).Error; err != nil {
			return err
		}
		
		return nil
	})
}

// UpdateWorkerHeartbeat 更新Worker心跳
func (s *WorkerAuthService) UpdateWorkerHeartbeat(workerID string) error {
	return s.db.Model(&models.WorkerConnection{}).
		Where("worker_id = ? AND status = ?", workerID, "active").
		Update("last_heartbeat", time.Now()).Error
}

// DisconnectWorker 断开Worker连接
func (s *WorkerAuthService) DisconnectWorker(workerID string) error {
	return s.db.Model(&models.WorkerConnection{}).
		Where("worker_id = ?", workerID).
		Update("status", "disconnected").Error
}

// CleanupTimeoutConnections 清理超时的Worker连接
func (s *WorkerAuthService) CleanupTimeoutConnections() error {
	cutoffTime := time.Now().Add(-WorkerHeartbeatTimeout)
	
	// 将超时的活跃连接标记为断开
	result := s.db.Model(&models.WorkerConnection{}).
		Where("status = ? AND last_heartbeat < ?", "active", cutoffTime).
		Update("status", "disconnected")
	
	if result.Error != nil {
		return result.Error
	}
	
	if result.RowsAffected > 0 {
		logger.GetLogger().WithFields(logrus.Fields{
			"count": result.RowsAffected,
			"timeout": WorkerHeartbeatTimeout,
		}).Info("清理了超时的Worker连接")
	}
	
	return nil
}

// GetWorkerInitializationData 获取Worker初始化数据
func (s *WorkerAuthService) GetWorkerInitializationData() (map[string]interface{}, error) {
	log := logger.GetLogger()
	
	// 获取所有需要同步的Git仓库
	var repositories []models.GitRepository
	err := s.db.Where("status = ? AND (sync_enabled = ? OR id IN (SELECT DISTINCT (source_git_info->>'repository_id')::bigint FROM task_templates WHERE source_git_info IS NOT NULL))", 
		"active", true).
		Preload("Credential").
		Find(&repositories).Error
	if err != nil {
		log.WithError(err).Error("获取Git仓库列表失败")
		return nil, err
	}

	// 获取所有活跃的任务模板
	var templates []models.TaskTemplate
	err = s.db.Where("id IN (SELECT DISTINCT template_id FROM scheduled_tasks WHERE is_active = ? AND template_id IS NOT NULL)", true).
		Find(&templates).Error
	if err != nil {
		log.WithError(err).Error("获取任务模板列表失败")
		return nil, err
	}

	// 构建响应数据
	repoList := make([]map[string]interface{}, 0, len(repositories))
	for _, repo := range repositories {
		repoData := map[string]interface{}{
			"id":          repo.ID,
			"tenant_id":   repo.TenantID,
			"name":        repo.Name,
			"url":         repo.URL,
			"branch":      repo.Branch,
			"is_public":   repo.IsPublic,
			"local_path":  repo.LocalPath,
		}
		
		// 如果有凭证，添加凭证信息（包含解密后的数据供Worker使用）
		if repo.CredentialID != nil && repo.Credential.ID > 0 {
			repoData["credential_id"] = *repo.CredentialID
			
			// 解密凭证供Worker使用
			credService := NewCredentialService(s.db)
			decryptedCred, err := credService.DecryptCredential(*repo.CredentialID, repo.TenantID)
			if err != nil {
				log.WithError(err).WithField("credential_id", *repo.CredentialID).Warn("解密凭证失败")
			} else {
				// 构建凭证信息
				credInfo := map[string]interface{}{
					"type": decryptedCred["type"],
				}
				
				// 根据类型添加相应字段
				switch decryptedCred["type"] {
				case "password":
					credInfo["username"] = decryptedCred["username"]
					credInfo["password"] = decryptedCred["password"]
				case "ssh_key":
					credInfo["username"] = decryptedCred["username"]
					credInfo["private_key"] = decryptedCred["private_key"]
					if passphrase, ok := decryptedCred["passphrase"]; ok {
						credInfo["passphrase"] = passphrase
					}
				}
				
				repoData["credential"] = credInfo
			}
		}
		
		repoList = append(repoList, repoData)
	}

	// 构建模板列表
	templateList := make([]map[string]interface{}, 0, len(templates))
	for _, tmpl := range templates {
		templateData := map[string]interface{}{
			"id":             tmpl.ID,
			"tenant_id":      tmpl.TenantID,
			"code":           tmpl.Code,
			"name":           tmpl.Name,
			"script_type":    tmpl.ScriptType,
			"entry_file":     tmpl.EntryFile,
			"included_files": tmpl.IncludedFiles,
		}
		
		// 添加Git来源信息
		if tmpl.SourceGitInfo != nil {
			var gitInfo map[string]interface{}
			if err := tmpl.SourceGitInfo.Unmarshal(&gitInfo); err == nil {
				templateData["source_git_info"] = gitInfo
			}
		}
		
		templateList = append(templateList, templateData)
	}

	log.WithFields(logrus.Fields{
		"repositories": len(repoList),
		"templates":    len(templateList),
	}).Info("准备Worker初始化数据")

	return map[string]interface{}{
		"repositories": repoList,
		"templates":    templateList,
		"timestamp":    time.Now().Unix(),
	}, nil
}
