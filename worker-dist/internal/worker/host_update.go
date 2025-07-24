package worker

import (
	"ahop-worker/internal/executor"
	"ahop-worker/internal/models"
	"ahop-worker/internal/types"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// 需要更新主机信息的任务类型
var tasksRequireHostUpdate = map[string]bool{
	"ping":    true,
	"collect": true,
}

// shouldUpdateHostInfo 判断任务类型是否需要更新主机信息
func (w *Worker) shouldUpdateHostInfo(taskType string) bool {
	return tasksRequireHostUpdate[taskType]
}

// updateHostInfo 根据任务结果更新主机信息
func (w *Worker) updateHostInfo(taskType string, params map[string]interface{}, result *executor.TaskResult) error {
	// 获取主机信息映射
	hostInfoMapInterface, ok := params["_host_info_map"]
	if !ok {
		return fmt.Errorf("找不到主机信息映射")
	}

	hostInfoMap, ok := hostInfoMapInterface.(map[uint]*types.HostInfo)
	if !ok {
		return fmt.Errorf("主机信息映射格式错误")
	}

	// 根据任务类型调用不同的更新方法
	switch taskType {
	case "ping":
		return w.updateHostPingInfo(hostInfoMap, result)
	case "collect":
		return w.updateHostSystemInfo(hostInfoMap, result)
	default:
		return fmt.Errorf("不支持的任务类型: %s", taskType)
	}
}

// updateHostPingInfo 更新主机的 ping 状态
func (w *Worker) updateHostPingInfo(hostInfoMap map[uint]*types.HostInfo, result *executor.TaskResult) error {
	// 解析 ping 结果 - 从result.Result中获取
	resultMap, ok := result.Result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("ping 结果格式错误: result 不是map")
	}
	
	hosts, ok := resultMap["hosts"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("ping 结果格式错误: 找不到hosts字段")
	}

	for hostID, hostInfo := range hostInfoMap {
		// 查找该主机的执行结果
		hostKey := fmt.Sprintf("%s:%d", hostInfo.IP, hostInfo.Port)
		hostResult, ok := hosts[hostKey].(map[string]interface{})
		if !ok {
			w.log.WithField("host_id", hostID).Warn("找不到主机的 ping 结果")
			continue
		}

		// 判断 ping 是否成功
		success, _ := hostResult["success"].(bool)
		status := "offline"
		if success {
			status = "online"
		}

		// 更新主机状态
		if err := w.updateHostStatus(hostID, status); err != nil {
			w.log.WithFields(logrus.Fields{
				"host_id": hostID,
				"error":   err,
			}).Error("更新主机 ping 状态失败")
		} else {
			w.log.WithFields(logrus.Fields{
				"host_id": hostID,
				"status":  status,
			}).Debug("更新主机 ping 状态成功")
		}
	}

	return nil
}

// updateHostSystemInfo 更新主机的系统信息
func (w *Worker) updateHostSystemInfo(hostInfoMap map[uint]*types.HostInfo, result *executor.TaskResult) error {
	// 解析 collect 结果 - 从result.Result中获取
	resultMap, ok := result.Result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("collect 结果格式错误: result 不是map")
	}
	
	hosts, ok := resultMap["hosts"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("collect 结果格式错误: 找不到hosts字段")
	}

	for hostID, hostInfo := range hostInfoMap {
		// 查找该主机的执行结果
		hostKey := fmt.Sprintf("%s:%d", hostInfo.IP, hostInfo.Port)
		hostResult, ok := hosts[hostKey].(map[string]interface{})
		if !ok {
			w.log.WithField("host_id", hostID).Warn("找不到主机的 collect 结果")
			continue
		}

		// 检查是否成功
		if success, ok := hostResult["success"].(bool); !ok || !success {
			w.log.WithField("host_id", hostID).Warn("主机信息收集失败，跳过更新")
			continue
		}

		// 提取 ansible facts
		factsInterface, ok := hostResult["ansible_facts"]
		if !ok {
			w.log.WithField("host_id", hostID).Warn("找不到 ansible_facts")
			continue
		}

		facts, ok := factsInterface.(map[string]interface{})
		if !ok {
			w.log.WithField("host_id", hostID).Warn("ansible_facts 格式错误")
			continue
		}
		
		// 调试日志 - 查看 facts 中的字段
		w.log.WithField("host_id", hostID).Debugf("ansible_facts 字段数量: %d", len(facts))
		for key := range facts {
			if key == "ansible_mounts" || key == "ansible_interfaces" {
				w.log.WithField("host_id", hostID).Debugf("找到字段: %s", key)
			}
		}

		// 使用事务更新主机信息
		err := w.db.Transaction(func(tx *gorm.DB) error {
			// 1. 更新主机基本信息
			if err := w.updateHostBasicInfo(tx, hostID, facts); err != nil {
				return fmt.Errorf("更新主机基本信息失败: %v", err)
			}

			// 2. 更新磁盘信息
			if err := w.updateHostDisks(tx, hostID, facts); err != nil {
				return fmt.Errorf("更新磁盘信息失败: %v", err)
			}

			// 3. 更新网卡信息
			if err := w.updateHostNetworkCards(tx, hostID, facts); err != nil {
				return fmt.Errorf("更新网卡信息失败: %v", err)
			}

			return nil
		})

		if err != nil {
			w.log.WithFields(logrus.Fields{
				"host_id": hostID,
				"error":   err,
			}).Error("更新主机系统信息失败")
		} else {
			w.log.WithField("host_id", hostID).Info("更新主机系统信息成功")
		}
	}

	return nil
}

// updateHostBasicInfo 更新主机基本信息
func (w *Worker) updateHostBasicInfo(tx *gorm.DB, hostID uint, facts map[string]interface{}) error {
	host := models.Host{
		LastCheckAt: &[]time.Time{time.Now()}[0],
	}

	// 操作系统信息
	if val, ok := facts["ansible_system"].(string); ok {
		host.OSType = val
	}
	if val, ok := facts["ansible_distribution_version"].(string); ok {
		host.OSVersion = val
	}
	if val, ok := facts["ansible_hostname"].(string); ok {
		host.Hostname = val
	}
	if val, ok := facts["ansible_architecture"].(string); ok {
		host.Architecture = val
	}
	if val, ok := facts["ansible_kernel"].(string); ok {
		host.Kernel = val
	}

	// CPU 信息
	if processors, ok := facts["ansible_processor"].([]interface{}); ok && len(processors) > 0 {
		if cpuModel, ok := processors[len(processors)-1].(string); ok {
			host.CPUModel = cpuModel
		}
	}
	if val, ok := facts["ansible_processor_vcpus"].(float64); ok {
		host.CPUCores = int(val)
	}

	// 内存信息（MB）
	if val, ok := facts["ansible_memtotal_mb"].(float64); ok {
		host.MemoryTotalMB = int64(val)
	}

	// 执行更新
	return tx.Model(&models.Host{}).
		Where("id = ?", hostID).
		Select("hostname", "os_type", "os_version", "architecture", "kernel",
			"cpu_model", "cpu_cores", "memory_total_mb", "last_check_at").
		Updates(&host).Error
}

// updateHostDisks 更新主机磁盘信息
func (w *Worker) updateHostDisks(tx *gorm.DB, hostID uint, facts map[string]interface{}) error {
	// 删除旧的磁盘信息
	if err := tx.Where("host_id = ?", hostID).Delete(&models.HostDisk{}).Error; err != nil {
		return err
	}

	// 使用 ansible_mounts 获取磁盘信息
	mounts, ok := facts["ansible_mounts"].([]interface{})
	if !ok {
		w.log.WithField("host_id", hostID).Debug("没有找到挂载点信息")
		return nil
	}

	for _, mountInfo := range mounts {
		mount, ok := mountInfo.(map[string]interface{})
		if !ok {
			continue
		}

		// 跳过特殊文件系统
		fstype, _ := mount["fstype"].(string)
		if fstype == "tmpfs" || fstype == "devtmpfs" || fstype == "proc" || fstype == "sysfs" {
			continue
		}

		// 创建磁盘记录
		disk := models.HostDisk{
			HostID: hostID,
		}

		// 设备名
		if device, ok := mount["device"].(string); ok {
			disk.Device = device
		}

		// 挂载点
		if mountPoint, ok := mount["mount"].(string); ok {
			disk.MountPoint = mountPoint
		}

		// 文件系统类型
		disk.FileSystem = fstype

		// 磁盘大小信息（字节转换为MB）
		if sizeTotal, ok := mount["size_total"].(float64); ok {
			disk.TotalMB = int64(sizeTotal / 1024 / 1024)
		}
		if sizeAvailable, ok := mount["size_available"].(float64); ok {
			disk.FreeMB = int64(sizeAvailable / 1024 / 1024)
		}
		if disk.TotalMB > 0 && disk.FreeMB >= 0 {
			disk.UsedMB = disk.TotalMB - disk.FreeMB
			if disk.TotalMB > 0 {
				disk.UsagePercent = float64(disk.UsedMB) * 100 / float64(disk.TotalMB)
			}
		}

		// 插入磁盘记录
		if err := tx.Create(&disk).Error; err != nil {
			w.log.WithFields(logrus.Fields{
				"host_id":     hostID,
				"device":      disk.Device,
				"mount_point": disk.MountPoint,
				"error":       err,
			}).Error("创建磁盘记录失败")
		}
	}

	return nil
}

// updateHostNetworkCards 更新主机网卡信息
func (w *Worker) updateHostNetworkCards(tx *gorm.DB, hostID uint, facts map[string]interface{}) error {
	// 删除旧的网卡信息
	if err := tx.Where("host_id = ?", hostID).Delete(&models.HostNetworkCard{}).Error; err != nil {
		return err
	}

	// 获取网络接口列表
	interfaces, ok := facts["ansible_interfaces"].([]interface{})
	if !ok {
		w.log.WithField("host_id", hostID).Debug("没有找到网络接口信息")
		return nil
	}

	for _, iface := range interfaces {
		ifaceName, ok := iface.(string)
		if !ok {
			continue
		}

		// 跳过 lo 接口
		if ifaceName == "lo" {
			continue
		}

		// 获取接口详细信息
		ifaceKey := fmt.Sprintf("ansible_%s", ifaceName)
		ifaceInfo, ok := facts[ifaceKey].(map[string]interface{})
		if !ok {
			continue
		}

		// 创建网卡记录
		nic := models.HostNetworkCard{
			HostID: hostID,
			Name:   ifaceName, // 主项目使用 Name 字段
		}

		// 提取网卡信息
		if mac, ok := ifaceInfo["macaddress"].(string); ok {
			nic.MACAddress = mac
		}
		// MTU
		if mtu, ok := ifaceInfo["mtu"].(float64); ok {
			nic.MTU = int(mtu)
		}
		if active, ok := ifaceInfo["active"].(bool); ok {
			nic.State = "down" // 主项目使用 State 字段
			if active {
				nic.State = "up"
			}
		}
		if speed, ok := ifaceInfo["speed"].(float64); ok {
			nic.Speed = int(speed)
		}

		// 提取 IPv4 地址
		var ipAddresses []string
		if ipv4Info, ok := ifaceInfo["ipv4"].(map[string]interface{}); ok {
			if address, ok := ipv4Info["address"].(string); ok {
				nic.IPAddress = address // 主IP
				ipAddresses = append(ipAddresses, address)
			}
		}

		// 提取所有IPv4地址
		if ipv4Addresses, ok := ifaceInfo["ipv4_secondaries"].([]interface{}); ok {
			for _, addr := range ipv4Addresses {
				if addrMap, ok := addr.(map[string]interface{}); ok {
					if address, ok := addrMap["address"].(string); ok {
						ipAddresses = append(ipAddresses, address)
					}
				}
			}
		}

		// 设置所有IP地址
		if len(ipAddresses) > 0 {
			nic.IPAddresses = strings.Join(ipAddresses, ",")
		}

		// 插入网卡记录
		if err := tx.Create(&nic).Error; err != nil {
			w.log.WithFields(logrus.Fields{
				"host_id": hostID,
				"name":    ifaceName,
				"error":   err,
			}).Error("创建网卡记录失败")
		}
	}

	return nil
}

// updateHostStatus 更新主机状态 - 优雅的GORM更新方式
func (w *Worker) updateHostStatus(hostID uint, status string) error {
	now := time.Now()
	return w.db.Model(&models.Host{}).
		Where("id = ?", hostID).
		Select("status", "last_check_at").
		Updates(models.Host{
			Status:      status,
			LastCheckAt: &now,
		}).Error
}