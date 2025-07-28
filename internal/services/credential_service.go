package services

import (
	"ahop/internal/models"
	"ahop/pkg/config"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"gorm.io/gorm"
)

// CredentialService 凭证服务
type CredentialService struct {
	db *gorm.DB
}

// NewCredentialService 创建凭证服务实例
func NewCredentialService(db *gorm.DB) *CredentialService {
	return &CredentialService{db: db}
}

// encryptionKey 获取加密密钥（从配置读取）
func (s *CredentialService) encryptionKey() []byte {
	cfg := config.GetConfig()
	key := cfg.Credential.EncryptionKey
	
	// 确保密钥长度为32字节（AES-256要求）
	if len(key) < 32 {
		// 如果密钥不足32字节，用默认值补齐
		defaultKey := "ahop-credential-encryption-key32"
		key = defaultKey
	} else if len(key) > 32 {
		// 如果密钥超过32字节，截取前32字节
		key = key[:32]
	}
	
	return []byte(key)
}

// encrypt 加密敏感数据
func (s *CredentialService) encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key := s.encryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt 解密敏感数据
func (s *CredentialService) decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	key := s.encryptionKey()
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertextBytes) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := ciphertextBytes[:nonceSize], ciphertextBytes[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// Create 创建凭证
func (s *CredentialService) Create(credential *models.Credential) error {
	// 加密敏感字段
	if err := s.encryptCredentialFields(credential); err != nil {
		return err
	}

	// 验证凭证类型和必填字段
	if err := s.validateCredential(credential); err != nil {
		return err
	}

	return s.db.Create(credential).Error
}

// Update 更新凭证
func (s *CredentialService) Update(id uint, tenantID uint, updates map[string]interface{}) error {
	// 先查询原凭证
	var credential models.Credential
	if err := s.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&credential).Error; err != nil {
		return err
	}

	// 处理需要加密的字段
	encryptFields := []string{"password", "private_key", "api_key", "token", "certificate", "passphrase"}
	for _, field := range encryptFields {
		if val, ok := updates[field]; ok && val != nil {
			if strVal, ok := val.(string); ok && strVal != "" {
				encrypted, err := s.encrypt(strVal)
				if err != nil {
					return err
				}
				updates[field] = encrypted
			}
		}
	}

	updates["updated_at"] = time.Now()
	return s.db.Model(&credential).Updates(updates).Error
}

// GetByID 根据ID获取凭证（不返回敏感信息）
func (s *CredentialService) GetByID(id uint, tenantID uint) (*models.Credential, error) {
	var credential models.Credential
	err := s.db.Preload("Tags").Where("id = ? AND tenant_id = ?", id, tenantID).First(&credential).Error
	if err != nil {
		return nil, err
	}

	// 清空敏感字段
	s.clearSensitiveFields(&credential)
	return &credential, nil
}

// GetDecrypted 获取解密后的凭证（需要特殊权限）
func (s *CredentialService) GetDecrypted(id uint, tenantID uint, operator *OperatorInfo, purpose string) (*models.Credential, error) {
	var credential models.Credential
	err := s.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&credential).Error
	if err != nil {
		return nil, err
	}

	// 检查凭证是否可用
	if !credential.IsActive {
		return nil, fmt.Errorf("凭证已禁用")
	}

	// 检查过期时间
	if credential.ExpiresAt != nil && credential.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("凭证已过期")
	}

	// 检查使用次数
	if credential.MaxUsageCount > 0 && credential.UsageCount >= credential.MaxUsageCount {
		return nil, fmt.Errorf("凭证使用次数已达上限")
	}

	// 解密敏感字段
	if err := s.decryptCredentialFields(&credential); err != nil {
		return nil, err
	}

	// 记录使用日志（异步）
	go s.logUsage(credential.ID, tenantID, operator, purpose, "", "", true, "")

	// 更新使用信息
	go s.updateUsageInfo(credential.ID, operator.UserID)

	return &credential, nil
}

// List 获取凭证列表
func (s *CredentialService) List(tenantID uint, page, pageSize int, filters map[string]interface{}) ([]models.Credential, int64, error) {
	query := s.db.Model(&models.Credential{}).Where("tenant_id = ?", tenantID)

	// 应用过滤条件
	if credType, ok := filters["type"]; ok {
		query = query.Where("type = ?", credType)
	}
	if isActive, ok := filters["is_active"]; ok {
		query = query.Where("is_active = ?", isActive)
	}
	if name, ok := filters["name"]; ok {
		query = query.Where("name LIKE ?", "%"+name.(string)+"%")
	}

	var total int64
	query.Count(&total)

	// 计算分页
	offset := (page - 1) * pageSize

	var credentials []models.Credential
	err := query.Preload("Tags").Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&credentials).Error
	if err != nil {
		return nil, 0, err
	}

	// 清空敏感字段
	for i := range credentials {
		s.clearSensitiveFields(&credentials[i])
	}

	return credentials, total, nil
}

// Delete 删除凭证（硬删除）
func (s *CredentialService) Delete(id uint, tenantID uint) error {
	// 先检查凭证是否存在
	var credential models.Credential
	if err := s.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&credential).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("凭证不存在")
		}
		return err
	}
	
	// 检查是否有Git仓库引用此凭证
	var repoCount int64
	if err := s.db.Model(&models.GitRepository{}).Where("credential_id = ?", id).Count(&repoCount).Error; err != nil {
		return fmt.Errorf("检查关联Git仓库失败: %v", err)
	}
	
	if repoCount > 0 {
		return fmt.Errorf("该凭证被 %d 个Git仓库引用，请先解除引用关系", repoCount)
	}
	
	// 开始事务
	tx := s.db.Begin()
	
	// 先清理凭证标签关联
	if err := tx.Exec("DELETE FROM credential_tags WHERE credential_id = ?", id).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("清理凭证标签失败: %v", err)
	}
	
	// 硬删除凭证
	if err := tx.Unscoped().Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&models.Credential{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("删除凭证失败: %v", err)
	}
	
	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("提交事务失败: %v", err)
	}
	
	return nil
}

// CheckACL 检查凭证的ACL限制
func (s *CredentialService) CheckACL(credential *models.Credential, hostName string, hostIP string) error {
	// 检查允许的主机
	if credential.AllowedHosts != "" {
		allowed := false
		hosts := strings.Split(credential.AllowedHosts, ",")
		for _, h := range hosts {
			h = strings.TrimSpace(h)
			if h == hostName || (h != "" && strings.Contains(hostName, h)) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("主机不在允许列表中")
		}
	}

	// 检查允许的IP
	if credential.AllowedIPs != "" && hostIP != "" {
		allowed := false
		ips := strings.Split(credential.AllowedIPs, ",")
		for _, ipRange := range ips {
			ipRange = strings.TrimSpace(ipRange)
			if s.isIPInRange(hostIP, ipRange) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("IP不在允许范围内")
		}
	}

	// 检查禁止的主机
	if credential.DeniedHosts != "" {
		hosts := strings.Split(credential.DeniedHosts, ",")
		for _, h := range hosts {
			h = strings.TrimSpace(h)
			if h == hostName || (h != "" && strings.Contains(hostName, h)) {
				return fmt.Errorf("主机在禁止列表中")
			}
		}
	}

	// 检查禁止的IP
	if credential.DeniedIPs != "" && hostIP != "" {
		ips := strings.Split(credential.DeniedIPs, ",")
		for _, ipRange := range ips {
			ipRange = strings.TrimSpace(ipRange)
			if s.isIPInRange(hostIP, ipRange) {
				return fmt.Errorf("IP在禁止范围内")
			}
		}
	}

	return nil
}

// 辅助方法

// encryptCredentialFields 加密凭证的敏感字段
func (s *CredentialService) encryptCredentialFields(credential *models.Credential) error {
	var err error
	
	if credential.Password != "" {
		credential.Password, err = s.encrypt(credential.Password)
		if err != nil {
			return err
		}
	}
	
	if credential.PrivateKey != "" {
		credential.PrivateKey, err = s.encrypt(credential.PrivateKey)
		if err != nil {
			return err
		}
	}
	
	if credential.APIKey != "" {
		credential.APIKey, err = s.encrypt(credential.APIKey)
		if err != nil {
			return err
		}
	}
	
	if credential.Token != "" {
		credential.Token, err = s.encrypt(credential.Token)
		if err != nil {
			return err
		}
	}
	
	if credential.Certificate != "" {
		credential.Certificate, err = s.encrypt(credential.Certificate)
		if err != nil {
			return err
		}
	}
	
	if credential.Passphrase != "" {
		credential.Passphrase, err = s.encrypt(credential.Passphrase)
		if err != nil {
			return err
		}
	}
	
	return nil
}

// decryptCredentialFields 解密凭证的敏感字段
func (s *CredentialService) decryptCredentialFields(credential *models.Credential) error {
	var err error
	
	if credential.Password != "" {
		credential.Password, err = s.decrypt(credential.Password)
		if err != nil {
			return err
		}
	}
	
	if credential.PrivateKey != "" {
		credential.PrivateKey, err = s.decrypt(credential.PrivateKey)
		if err != nil {
			return err
		}
	}
	
	if credential.APIKey != "" {
		credential.APIKey, err = s.decrypt(credential.APIKey)
		if err != nil {
			return err
		}
	}
	
	if credential.Token != "" {
		credential.Token, err = s.decrypt(credential.Token)
		if err != nil {
			return err
		}
	}
	
	if credential.Certificate != "" {
		credential.Certificate, err = s.decrypt(credential.Certificate)
		if err != nil {
			return err
		}
	}
	
	if credential.Passphrase != "" {
		credential.Passphrase, err = s.decrypt(credential.Passphrase)
		if err != nil {
			return err
		}
	}
	
	return nil
}

// clearSensitiveFields 清空敏感字段
func (s *CredentialService) clearSensitiveFields(credential *models.Credential) {
	credential.Password = ""
	credential.PrivateKey = ""
	credential.APIKey = ""
	credential.Token = ""
	credential.Certificate = ""
	credential.Passphrase = ""
}

// validateCredential 验证凭证必填字段
func (s *CredentialService) validateCredential(credential *models.Credential) error {
	switch credential.Type {
	case models.CredentialTypePassword:
		if credential.Username == "" || credential.Password == "" {
			return fmt.Errorf("用户名密码类型凭证必须提供用户名和密码")
		}
	case models.CredentialTypeSSHKey:
		if credential.PrivateKey == "" {
			return fmt.Errorf("SSH密钥类型凭证必须提供私钥")
		}
	case models.CredentialTypeAPIKey:
		if credential.APIKey == "" {
			return fmt.Errorf("API密钥类型凭证必须提供API密钥")
		}
	case models.CredentialTypeToken:
		if credential.Token == "" {
			return fmt.Errorf("Token类型凭证必须提供Token")
		}
	case models.CredentialTypeCertificate:
		if credential.Certificate == "" {
			return fmt.Errorf("证书类型凭证必须提供证书内容")
		}
	default:
		return fmt.Errorf("不支持的凭证类型")
	}
	return nil
}

// isIPInRange 检查IP是否在CIDR范围内
func (s *CredentialService) isIPInRange(ip string, cidr string) bool {
	// 如果不是CIDR格式，尝试作为单个IP比较
	if !strings.Contains(cidr, "/") {
		return ip == cidr
	}

	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}

	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return false
	}

	return ipnet.Contains(ipAddr)
}

// OperatorInfo 操作者信息
type OperatorInfo struct {
	Type string // user/system/worker/integration
	UserID *uint // 用户ID（user类型时使用）
	Info string // 详细信息
}

// logUsage 记录凭证使用日志
func (s *CredentialService) logUsage(credentialID, tenantID uint, operator *OperatorInfo, purpose, hostName, hostIP string, success bool, errorMsg string) {
	log := &models.CredentialUsageLog{
		TenantID:     tenantID,
		CredentialID: credentialID,
		UserID:       operator.UserID,
		OperatorType: operator.Type,
		OperatorInfo: operator.Info,
		HostName:     hostName,
		HostIP:       hostIP,
		Purpose:      purpose,
		Success:      success,
		ErrorMessage: errorMsg,
	}
	s.db.Create(log)
}

// updateUsageInfo 更新凭证使用信息
func (s *CredentialService) updateUsageInfo(credentialID uint, userID *uint) {
	updates := map[string]interface{}{
		"last_used_at": time.Now(),
		"usage_count":  gorm.Expr("usage_count + 1"),
	}
	// 只有当userID不为空时才更新last_used_by
	if userID != nil {
		updates["last_used_by"] = *userID
	}
	s.db.Model(&models.Credential{}).Where("id = ?", credentialID).Updates(updates)
}

// GetUsageLogs 获取凭证使用日志
func (s *CredentialService) GetUsageLogs(credentialID, tenantID uint, page, pageSize int) ([]models.CredentialUsageLog, int64, error) {
	query := s.db.Model(&models.CredentialUsageLog{}).
		Where("credential_id = ? AND tenant_id = ?", credentialID, tenantID)

	var total int64
	query.Count(&total)

	// 计算分页
	offset := (page - 1) * pageSize

	var logs []models.CredentialUsageLog
	err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&logs).Error
	
	return logs, total, err
}

// DecryptCredential 解密凭证并返回map格式（供Worker使用）
func (s *CredentialService) DecryptCredential(credentialID uint, tenantID uint) (map[string]string, error) {
	// 获取凭证
	var credential models.Credential
	if err := s.db.Where("id = ? AND tenant_id = ?", credentialID, tenantID).First(&credential).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("凭证不存在")
		}
		return nil, err
	}

	// 检查凭证是否激活
	if !credential.IsActive {
		return nil, fmt.Errorf("凭证已禁用")
	}

	// 解密凭证字段
	if err := s.decryptCredentialFields(&credential); err != nil {
		return nil, fmt.Errorf("解密凭证失败: %v", err)
	}

	// 构建返回的map
	result := map[string]string{
		"type": string(credential.Type),
		"name": credential.Name,
	}

	// 根据类型返回相应字段
	switch credential.Type {
	case models.CredentialTypePassword:
		result["username"] = credential.Username
		result["password"] = credential.Password
	case models.CredentialTypeSSHKey:
		result["username"] = credential.Username
		result["private_key"] = credential.PrivateKey
		if credential.Passphrase != "" {
			result["passphrase"] = credential.Passphrase
		}
	case models.CredentialTypeAPIKey:
		result["api_key"] = credential.APIKey
	case models.CredentialTypeToken:
		result["token"] = credential.Token
	case models.CredentialTypeCertificate:
		result["certificate"] = credential.Certificate
		if credential.PrivateKey != "" {
			result["private_key"] = credential.PrivateKey
		}
	}

	// 记录使用（Worker使用）
	go s.logUsage(credentialID, tenantID, &OperatorInfo{
		Type: "worker",
		UserID: nil,
		Info: "worker-api",
	}, "git_sync", "worker", "", true, "")
	go s.updateUsageInfo(credentialID, nil)

	return result, nil
}