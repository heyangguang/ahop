package services

import (
	"ahop/internal/models"
	"ahop/pkg/connector"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// HostService 主机服务
type HostService struct {
	db      *gorm.DB
	credSvc *CredentialService
}

// NewHostService 创建主机服务实例
func NewHostService(db *gorm.DB) *HostService {
	return &HostService{
		db:      db,
		credSvc: NewCredentialService(db),
	}
}

// Create 创建主机
func (s *HostService) Create(host *models.Host) error {
	// 检查同租户下主机名是否已存在
	var count int64
	err := s.db.Model(&models.Host{}).
		Where("tenant_id = ? AND name = ?", host.TenantID, host.Name).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("主机名 %s 已存在", host.Name)
	}

	// 验证凭证是否存在且属于同一租户
	var credential models.Credential
	err = s.db.Where("id = ? AND tenant_id = ?", host.CredentialID, host.TenantID).
		First(&credential).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("凭证不存在或无权限使用")
		}
		return err
	}

	// 设置默认值
	host.Status = "pending"

	// 创建主机
	return s.db.Create(host).Error
}

// Update 更新主机
func (s *HostService) Update(id uint, tenantID uint, updates map[string]interface{}) error {
	// 检查主机是否存在
	var host models.Host
	err := s.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&host).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("主机不存在")
		}
		return err
	}

	// 如果更新凭证，需要验证
	if credentialID, ok := updates["credential_id"]; ok {
		var credential models.Credential
		err = s.db.Where("id = ? AND tenant_id = ?", credentialID, tenantID).
			First(&credential).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("凭证不存在或无权限使用")
			}
			return err
		}
	}

	// 如果更新主机名，检查是否重复
	if name, ok := updates["name"]; ok {
		var count int64
		err = s.db.Model(&models.Host{}).
			Where("tenant_id = ? AND name = ? AND id != ?", tenantID, name, id).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("主机名 %s 已存在", name)
		}
	}

	updates["updated_at"] = time.Now()
	return s.db.Model(&host).Updates(updates).Error
}

// GetByID 根据ID获取主机
func (s *HostService) GetByID(id uint, tenantID uint) (*models.Host, error) {
	var host models.Host
	err := s.db.Preload("Credential").Preload("Tags").
		Preload("Disks").Preload("NetworkCards").
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&host).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("主机不存在")
		}
		return nil, err
	}

	// 清空凭证敏感信息
	host.Credential.Password = ""
	host.Credential.PrivateKey = ""
	host.Credential.APIKey = ""
	host.Credential.Token = ""
	host.Credential.Certificate = ""
	host.Credential.Passphrase = ""

	return &host, nil
}

// List 获取主机列表
func (s *HostService) List(tenantID uint, page, pageSize int, filters map[string]interface{}) ([]models.Host, int64, error) {
	query := s.db.Model(&models.Host{}).Where("tenant_id = ?", tenantID)

	// 应用过滤条件
	if status, ok := filters["status"]; ok {
		query = query.Where("status = ?", status)
	}
	if isActive, ok := filters["is_active"]; ok {
		query = query.Where("is_active = ?", isActive)
	}
	if name, ok := filters["name"]; ok {
		query = query.Where("name LIKE ?", "%"+name.(string)+"%")
	}
	if ipAddress, ok := filters["ip_address"]; ok {
		query = query.Where("ip_address LIKE ?", "%"+ipAddress.(string)+"%")
	}
	if osType, ok := filters["os_type"]; ok {
		query = query.Where("os_type = ?", osType)
	}

	// 计算总数
	var total int64
	query.Count(&total)

	// 分页查询
	offset := (page - 1) * pageSize
	var hosts []models.Host
	err := query.Preload("Tags").
		Offset(offset).Limit(pageSize).
		Order("created_at DESC").
		Find(&hosts).Error

	if err != nil {
		return nil, 0, err
	}

	return hosts, total, nil
}

// Delete 删除主机
func (s *HostService) Delete(id uint, tenantID uint) error {
	// 检查主机是否存在
	var host models.Host
	err := s.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&host).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("主机不存在")
		}
		return err
	}

	// 开启事务
	tx := s.db.Begin()

	// 删除主机磁盘信息
	if err := tx.Where("host_id = ?", id).Delete(&models.HostDisk{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("删除主机磁盘信息失败: %v", err)
	}

	// 删除主机网卡信息
	if err := tx.Where("host_id = ?", id).Delete(&models.HostNetworkCard{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("删除主机网卡信息失败: %v", err)
	}

	// 删除主机标签关联
	if err := tx.Exec("DELETE FROM host_tags WHERE host_id = ?", id).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("删除主机标签关联失败: %v", err)
	}

	// 删除主机组关联
	if err := tx.Exec("DELETE FROM host_group_members WHERE host_id = ?", id).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("删除主机组关联失败: %v", err)
	}

	// 删除主机
	if err := tx.Delete(&host).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("删除主机失败: %v", err)
	}

	// 提交事务
	return tx.Commit().Error
}

// UpdateTags 更新主机标签
func (s *HostService) UpdateTags(hostID uint, tenantID uint, tagIDs []uint) error {
	// 检查主机是否存在
	var host models.Host
	err := s.db.Where("id = ? AND tenant_id = ?", hostID, tenantID).First(&host).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("主机不存在")
		}
		return err
	}

	// 验证所有标签都属于同一租户
	var tags []models.Tag
	if len(tagIDs) > 0 {
		err = s.db.Where("id IN ? AND tenant_id = ?", tagIDs, tenantID).Find(&tags).Error
		if err != nil {
			return err
		}
		if len(tags) != len(tagIDs) {
			return fmt.Errorf("部分标签不存在或无权限使用")
		}
	}

	// 更新关联
	return s.db.Model(&host).Association("Tags").Replace(tags)
}

// GetTags 获取主机标签
func (s *HostService) GetTags(hostID uint, tenantID uint) ([]models.Tag, error) {
	var host models.Host
	err := s.db.Preload("Tags").
		Where("id = ? AND tenant_id = ?", hostID, tenantID).
		First(&host).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("主机不存在")
		}
		return nil, err
	}

	return host.Tags, nil
}

// GetByIDs 批量获取主机
func (s *HostService) GetByIDs(ids []uint, tenantID uint) ([]models.Host, error) {
	var hosts []models.Host
	err := s.db.Preload("Credential").
		Where("id IN ? AND tenant_id = ?", ids, tenantID).
		Find(&hosts).Error

	if err != nil {
		return nil, err
	}

	// 清空凭证敏感信息
	for i := range hosts {
		hosts[i].Credential.Password = ""
		hosts[i].Credential.PrivateKey = ""
		hosts[i].Credential.APIKey = ""
		hosts[i].Credential.Token = ""
		hosts[i].Credential.Certificate = ""
		hosts[i].Credential.Passphrase = ""
	}

	return hosts, nil
}

// UpdateStatus 更新主机状态
func (s *HostService) UpdateStatus(id uint, tenantID uint, status string) error {
	updates := map[string]interface{}{
		"status":        status,
		"last_check_at": time.Now(),
		"updated_at":    time.Now(),
	}

	result := s.db.Model(&models.Host{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("主机不存在")
	}

	return nil
}

// TestConnection 测试主机连接
func (s *HostService) TestConnection(hostID uint, tenantID uint, userID uint) (*connector.TestResult, error) {
	// 获取主机信息
	host, err := s.GetByID(hostID, tenantID)
	if err != nil {
		return nil, err
	}

	// 获取解密后的凭证
	credential, err := s.credSvc.GetDecrypted(
		host.CredentialID,
		tenantID,
		&OperatorInfo{
			Type: "user",
			UserID: &userID,
			Info: "host-connection-test",
		},
		"host_connection_test",
	)
	if err != nil {
		return nil, fmt.Errorf("获取凭证失败: %v", err)
	}

	// 创建SSH连接器
	sshConnector := connector.NewSSHConnector(
		host.IPAddress,
		host.Port,
		credential.Username,
		credential.Password,
		credential.PrivateKey,
	)

	// 执行连接测试
	result := sshConnector.TestConnection()

	// 更新主机状态
	status := "offline"
	if result.Success {
		status = "online"
	}
	if err := s.UpdateStatus(hostID, tenantID, status); err != nil {
		// 状态更新失败不影响连接测试结果，但应该记录日志
		// 这里可以添加日志记录，暂时忽略错误
	}

	return result, nil
}

// TestPing 测试主机网络连通性
func (s *HostService) TestPing(hostID uint, tenantID uint) (*connector.TestResult, error) {
	// 获取主机信息
	host, err := s.GetByID(hostID, tenantID)
	if err != nil {
		return nil, err
	}

	// 创建连接器进行ping测试
	sshConnector := connector.NewSSHConnector(
		host.IPAddress,
		host.Port,
		"", // ping测试不需要凭证
		"",
		"",
	)

	// 执行ping测试
	result := sshConnector.TestPing()

	// 更新主机状态
	status := "unreachable"
	if result.Success {
		status = "online" // ping通了，但可能SSH不通
	}
	if err := s.UpdateStatus(hostID, tenantID, status); err != nil {
		// 状态更新失败不影响ping测试结果，但应该记录日志
		// 这里可以添加日志记录，暂时忽略错误
	}

	return result, nil
}

// BatchImportHost 批量导入主机数据结构
type BatchImportHost struct {
	// 核心信息（网络扫描可提供）
	IP           string `json:"ip" binding:"required,ip"`        // 必须：IP地址
	Port         int    `json:"port" binding:"min=1,max=65535"`  // 可选：端口（默认22）
	
	// 用户必须指定
	CredentialID uint   `json:"credential_id" binding:"required"` // 必须：凭证ID
	
	// 用户可选指定
	Hostname     string `json:"hostname"`                        // 可选：主机名（默认使用IP）
	SSHUser      string `json:"ssh_user"`                        // 可选：SSH用户（默认root）
	Description  string `json:"description"`                     // 可选：描述
	HostGroupID  *uint  `json:"host_group_id"`                   // 可选：主机组ID
	Tags         []uint `json:"tags"`                            // 可选：标签ID列表
}

// BatchImportResult 批量导入结果
type BatchImportResult struct {
	Total     int                    `json:"total"`
	Success   int                    `json:"success"`
	Failed    int                    `json:"failed"`
	Errors    []string              `json:"errors,omitempty"`
	Imported  []models.Host         `json:"imported,omitempty"`
}

// BatchImport 批量导入主机
func (s *HostService) BatchImport(tenantID uint, userID uint, hosts []BatchImportHost) (*BatchImportResult, error) {
	result := &BatchImportResult{
		Total:    len(hosts),
		Success:  0,
		Failed:   0,
		Errors:   []string{},
		Imported: []models.Host{},
	}

	// 使用事务处理
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for i, hostData := range hosts {
			// 设置默认值
			hostname := hostData.Hostname
			if hostname == "" {
				hostname = hostData.IP // 使用IP作为主机名
			}
			
			port := hostData.Port
			if port == 0 {
				port = 22 // 默认SSH端口
			}
			
			sshUser := hostData.SSHUser
			if sshUser == "" {
				sshUser = "root" // 默认用户
			}
			
			// 检查主机名是否已存在
			var count int64
			err := tx.Model(&models.Host{}).
				Where("tenant_id = ? AND name = ?", tenantID, hostname).
				Count(&count).Error
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 检查主机名失败 - %v", i+1, err))
				continue
			}
			if count > 0 {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 主机名 %s 已存在", i+1, hostname))
				continue
			}

			// 验证凭证是否存在
			var credential models.Credential
			err = tx.Where("id = ? AND tenant_id = ?", hostData.CredentialID, tenantID).
				First(&credential).Error
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 凭证ID %d 不存在或无权限", i+1, hostData.CredentialID))
				continue
			}
			
			// 验证主机组是否存在（如果提供）
			if hostData.HostGroupID != nil && *hostData.HostGroupID > 0 {
				var hostGroup models.HostGroup
				err = tx.Where("id = ? AND tenant_id = ?", *hostData.HostGroupID, tenantID).
					First(&hostGroup).Error
				if err != nil {
					result.Failed++
					result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 主机组ID %d 不存在或无权限", i+1, *hostData.HostGroupID))
					continue
				}
			}

			// 创建主机
			host := &models.Host{
				TenantID:     tenantID,
				Name:         hostname,
				IPAddress:    hostData.IP,
				Port:         port,
				CredentialID: hostData.CredentialID,
				Description:  hostData.Description,
				Status:       "unknown",
				IsActive:     true,
				CreatedBy:    userID,
				UpdatedBy:    userID,
			}

			if err := tx.Create(host).Error; err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 创建主机失败 - %v", i+1, err))
				continue
			}

			// 更新标签
			if len(hostData.Tags) > 0 {
				// 使用主机服务的UpdateTags方法
				hostService := &HostService{db: tx}
				if err := hostService.UpdateTags(host.ID, tenantID, hostData.Tags); err != nil {
					// 标签更新失败不影响主机创建，记录错误
					result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 主机创建成功但标签更新失败 - %v", i+1, err))
				}
			}
			
			// 添加到主机组
			if hostData.HostGroupID != nil && *hostData.HostGroupID > 0 {
				// 创建主机组成员关联
				if err := tx.Exec("INSERT INTO host_group_members (host_group_id, host_id) VALUES (?, ?)", *hostData.HostGroupID, host.ID).Error; err != nil {
					// 主机组关联失败不影响主机创建，记录错误
					result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 主机创建成功但加入主机组失败 - %v", i+1, err))
				}
			}

			// 重新获取包含关联数据的主机信息
			var createdHost models.Host
			err = tx.Preload("Credential").Preload("Tags").Preload("Groups").
				Where("id = ?", host.ID).First(&createdHost).Error
			if err == nil {
				result.Imported = append(result.Imported, createdHost)
			} else {
				result.Imported = append(result.Imported, *host)
			}

			result.Success++
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// ExportToCSV 导出主机数据为CSV格式
func (s *HostService) ExportToCSV(tenantID uint, filters map[string]interface{}) (string, error) {
	// 构建查询
	query := s.db.Model(&models.Host{}).Where("tenant_id = ?", tenantID)

	// 应用筛选条件
	for key, value := range filters {
		switch key {
		case "status":
			if v, ok := value.(string); ok && v != "" {
				query = query.Where("status = ?", v)
			}
		case "name":
			if v, ok := value.(string); ok && v != "" {
				query = query.Where("name ILIKE ?", "%"+v+"%")
			}
		case "os_type":
			if v, ok := value.(string); ok && v != "" {
				query = query.Where("os_type = ?", v)
			}
		case "is_active":
			if v, ok := value.(bool); ok {
				query = query.Where("is_active = ?", v)
			}
		}
	}

	// 获取主机数据
	var hosts []models.Host
	err := query.Preload("Credential").Preload("Tags").Find(&hosts).Error
	if err != nil {
		return "", err
	}

	// 创建CSV缓冲区
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)

	// 写入表头
	headers := []string{
		"序号", "主机名", "IP地址", "端口", "状态", "操作系统",
		"内核版本", "CPU核心数", "内存(GB)", "凭证名称", "凭证类型",
		"是否启用", "描述", "标签", "创建时间", "最后检查时间",
	}
	if err := writer.Write(headers); err != nil {
		return "", err
	}

	// 写入数据行
	for i, host := range hosts {
		// 处理标签
		var tagNames []string
		for _, tag := range host.Tags {
			tagNames = append(tagNames, fmt.Sprintf("%s:%s", tag.Key, tag.Value))
		}
		tagString := strings.Join(tagNames, ",")

		// 处理时间格式
		createdAt := host.CreatedAt.Format("2006-01-02 15:04:05")
		lastCheckAt := ""
		if host.LastCheckAt != nil {
			lastCheckAt = host.LastCheckAt.Format("2006-01-02 15:04:05")
		}

		// 处理凭证信息（脱敏处理）
		credentialName := ""
		credentialType := ""
		if host.Credential.ID != 0 {
			credentialName = fmt.Sprintf("凭证#%d", host.Credential.ID)
			credentialType = string(host.Credential.Type)
		}

		// 处理内存大小（转换为GB）
		memoryGB := ""
		if host.MemoryTotalMB > 0 {
			memoryGB = fmt.Sprintf("%.2f", float64(host.MemoryTotalMB)/1024.0)
		}

		row := []string{
			strconv.Itoa(i + 1),
			host.Name,
			host.IPAddress,
			strconv.Itoa(host.Port),
			host.Status,
			host.OSType,
			host.Kernel,
			strconv.Itoa(host.CPUCores),
			memoryGB,
			credentialName,
			credentialType,
			fmt.Sprintf("%t", host.IsActive),
			host.Description,
			tagString,
			createdAt,
			lastCheckAt,
		}

		if err := writer.Write(row); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return buffer.String(), nil
}
