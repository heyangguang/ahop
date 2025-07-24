package worker

import (
	"ahop-worker/internal/models"
	"ahop-worker/internal/types"
	"ahop-worker/pkg/crypto"
	"fmt"

	"github.com/sirupsen/logrus"
)

// prepareHostsInfo 批量查询主机信息（新版本）
func (w *Worker) prepareHostsInfo(tenantID uint, params map[string]interface{}) (map[uint]*types.HostInfo, error) {
	// 1. 解析主机ID列表
	hostIDs, err := w.extractHostIDs(params)
	if err != nil {
		return nil, err
	}

	if len(hostIDs) == 0 {
		return nil, fmt.Errorf("没有指定主机")
	}

	w.log.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"host_ids":  hostIDs,
		"count":     len(hostIDs),
	}).Debug("开始查询主机信息")

	// 2. 批量查询主机及其凭证
	var hosts []models.Host
	err = w.db.Preload("Credential").
		Where("id IN ? AND tenant_id = ?", hostIDs, tenantID).
		Find(&hosts).Error
	if err != nil {
		return nil, fmt.Errorf("查询主机信息失败: %v", err)
	}

	// 检查是否所有主机都找到了
	if len(hosts) != len(hostIDs) {
		w.log.WithFields(logrus.Fields{
			"requested": len(hostIDs),
			"found":     len(hosts),
		}).Warn("部分主机未找到")
	}

	// 3. 构建主机信息映射
	hostInfoMap := make(map[uint]*types.HostInfo)
	for _, host := range hosts {
		// 检查主机是否有凭证
		if host.CredentialID == 0 {
			w.log.WithFields(logrus.Fields{
				"host_id":   host.ID,
				"host_name": host.Name,
				"host_ip":   host.IPAddress,
			}).Warn("主机没有绑定凭证，跳过")
			continue
		}

		// 准备凭证信息
		credInfo, err := w.prepareCredentialInfo(&host.Credential)
		if err != nil {
			w.log.WithFields(logrus.Fields{
				"host_id":       host.ID,
				"credential_id": host.CredentialID,
				"error":         err,
			}).Error("准备凭证信息失败")
			continue
		}

		// 构建主机信息
		info := &types.HostInfo{
			ID:         host.ID,
			IP:         host.IPAddress,
			Port:       host.Port,
			Hostname:   host.Hostname,
			Credential: credInfo,
		}

		hostInfoMap[host.ID] = info

		w.log.WithFields(logrus.Fields{
			"host_id":         host.ID,
			"host_ip":         host.IPAddress,
			"host_name":       host.Hostname,
			"credential_type": host.Credential.Type,
		}).Debug("准备主机连接信息")
	}

	w.log.WithFields(logrus.Fields{
		"total_requested": len(hostIDs),
		"total_prepared":  len(hostInfoMap),
	}).Debug("主机信息准备完成")

	return hostInfoMap, nil
}

// extractHostIDs 从参数中提取主机ID列表
func (w *Worker) extractHostIDs(params map[string]interface{}) ([]uint, error) {
	hostsInterface, ok := params["hosts"]
	if !ok {
		return nil, fmt.Errorf("缺少 hosts 参数")
	}

	hostsArray, ok := hostsInterface.([]interface{})
	if !ok {
		return nil, fmt.Errorf("hosts 参数格式错误")
	}

	var hostIDs []uint
	for _, h := range hostsArray {
		switch v := h.(type) {
		case float64:
			hostIDs = append(hostIDs, uint(v))
		case int:
			hostIDs = append(hostIDs, uint(v))
		default:
			return nil, fmt.Errorf("主机ID类型错误: %T", v)
		}
	}

	return hostIDs, nil
}

// prepareCredentialInfo 准备凭证信息（解密敏感字段）
func (w *Worker) prepareCredentialInfo(cred *models.Credential) (*types.CredentialInfo, error) {
	if cred == nil {
		return nil, fmt.Errorf("凭证为空")
	}

	info := &types.CredentialInfo{
		Type:     cred.Type,
		Username: cred.Username,
	}

	// 根据凭证类型解密相应字段
	switch cred.Type {
	case "password":
		if len(cred.Password) > 0 {
			decrypted, err := crypto.Decrypt(string(cred.Password))
			if err != nil {
				return nil, fmt.Errorf("解密密码失败: %v", err)
			}
			info.Password = decrypted
		}

	case "ssh_key":
		if len(cred.PrivateKey) > 0 {
			decrypted, err := crypto.Decrypt(string(cred.PrivateKey))
			if err != nil {
				return nil, fmt.Errorf("解密私钥失败: %v", err)
			}
			info.PrivateKey = decrypted
		}
		if len(cred.Passphrase) > 0 {
			decrypted, err := crypto.Decrypt(string(cred.Passphrase))
			if err != nil {
				return nil, fmt.Errorf("解密密钥密码失败: %v", err)
			}
			info.Passphrase = decrypted
		}

	default:
		return nil, fmt.Errorf("不支持的凭证类型: %s", cred.Type)
	}

	return info, nil
}

// 兼容旧的调用方式
func (w *Worker) prepareHostInfoV2(tenantID uint, params map[string]interface{}) error {
	// 新版本不再修改原始参数，而是将主机信息存储在独立的字段中
	hostInfoMap, err := w.prepareHostsInfo(tenantID, params)
	if err != nil {
		return err
	}

	// 将主机信息存储在参数中供执行器使用
	params["_host_info_map"] = hostInfoMap
	return nil
}