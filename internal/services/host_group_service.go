package services

import (
	"errors"
	"strings"

	"ahop/internal/models"

	"gorm.io/gorm"
)

// HostGroupService 主机组服务
type HostGroupService struct {
	db *gorm.DB
}

// NewHostGroupService 创建主机组服务
func NewHostGroupService(db *gorm.DB) *HostGroupService {
	return &HostGroupService{db: db}
}

// Create 创建主机组
func (s *HostGroupService) Create(tenantID uint, req *models.CreateHostGroupRequest) (*models.HostGroup, error) {
	// 检查同级组名和代码是否重复
	var count int64
	query := s.db.Model(&models.HostGroup{}).Where("tenant_id = ?", tenantID)

	if req.ParentID != nil {
		query = query.Where("parent_id = ?", *req.ParentID)
	} else {
		query = query.Where("parent_id IS NULL")
	}

	if err := query.Where("(name = ? OR code = ?)", req.Name, req.Code).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("同级已存在相同名称或代码的主机组")
	}

	group := &models.HostGroup{
		TenantID:    tenantID,
		ParentID:    req.ParentID,
		Name:        req.Name,
		Code:        req.Code,
		Type:        req.Type,
		Description: req.Description,
		Status:      req.Status,
		Metadata:    req.Metadata,
		IsLeaf:      true,
		Level:       1,
	}

	// 如果没有指定类型，默认为custom
	if group.Type == "" {
		group.Type = models.HostGroupTypeCustom
	}

	// 如果没有指定状态，默认为active
	if group.Status == "" {
		group.Status = models.HostGroupStatusActive
	}

	// 开启事务
	return group, s.db.Transaction(func(tx *gorm.DB) error {
		// 如果有父组
		if req.ParentID != nil {
			parent := &models.HostGroup{}
			if err := tx.First(parent, *req.ParentID).Error; err != nil {
				return errors.New("父组不存在")
			}

			// 检查父组是否属于同一租户
			if parent.TenantID != tenantID {
				return errors.New("父组不属于当前租户")
			}

			// 如果父组是叶子节点且有主机，不允许添加子组
			if parent.IsLeaf && parent.HostCount > 0 {
				return errors.New("父组已包含主机，不能添加子组")
			}

			// 更新父组为非叶子节点
			if parent.IsLeaf {
				if err := tx.Model(parent).Updates(map[string]interface{}{
					"is_leaf": false,
				}).Error; err != nil {
					return err
				}
			}

			// 更新父组的子组数量
			if err := tx.Model(parent).Update("child_count", gorm.Expr("child_count + 1")).Error; err != nil {
				return err
			}

			// 设置层级和路径
			group.Level = parent.Level + 1
			group.Path = parent.Path + "/" + group.Code
		} else {
			// 顶级组
			group.Path = "/" + group.Code
		}

		// 创建组
		if err := tx.Create(group).Error; err != nil {
			return err
		}

		return nil
	})
}

// Update 更新主机组
func (s *HostGroupService) Update(groupID uint, tenantID uint, req *models.UpdateHostGroupRequest) error {
	group := &models.HostGroup{}
	if err := s.db.Where("id = ? AND tenant_id = ?", groupID, tenantID).First(group).Error; err != nil {
		return err
	}

	// 如果要更新名称，检查同级是否有重复
	if req.Name != "" && req.Name != group.Name {
		var count int64
		query := s.db.Model(&models.HostGroup{}).
			Where("tenant_id = ? AND id != ?", tenantID, groupID)

		if group.ParentID != nil {
			query = query.Where("parent_id = ?", *group.ParentID)
		} else {
			query = query.Where("parent_id IS NULL")
		}

		if err := query.Where("name = ?", req.Name).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("同级已存在相同名称的主机组")
		}
	}

	// 更新字段
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Metadata != nil {
		updates["metadata"] = req.Metadata
	}

	return s.db.Model(group).Updates(updates).Error
}

// Delete 删除主机组
func (s *HostGroupService) Delete(groupID uint, tenantID uint, force bool) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		group := &models.HostGroup{}
		if err := tx.Where("id = ? AND tenant_id = ?", groupID, tenantID).First(group).Error; err != nil {
			return err
		}

		// 检查是否有子组
		if group.ChildCount > 0 && !force {
			return errors.New("该组包含子组，不能删除")
		}

		// 检查是否有主机
		if group.HostCount > 0 && !force {
			return errors.New("该组包含主机，不能删除")
		}

		// 如果强制删除，先处理子组和主机
		if force {
			// 递归删除所有子组
			if err := s.deleteChildrenRecursive(tx, groupID); err != nil {
				return err
			}

			// 将组内主机设置为未分组
			if err := tx.Model(&models.Host{}).
				Where("host_group_id = ?", groupID).
				Update("host_group_id", nil).Error; err != nil {
				return err
			}
		}

		// 如果有父组，更新父组的子组数量
		if group.ParentID != nil {
			if err := tx.Model(&models.HostGroup{}).
				Where("id = ?", *group.ParentID).
				Update("child_count", gorm.Expr("child_count - 1")).Error; err != nil {
				return err
			}

			// 检查父组是否还有其他子组，如果没有则设置为叶子节点
			var childCount int64
			tx.Model(&models.HostGroup{}).
				Where("parent_id = ?", *group.ParentID).
				Where("id != ?", groupID).
				Count(&childCount)

			if childCount == 0 {
				tx.Model(&models.HostGroup{}).
					Where("id = ?", *group.ParentID).
					Update("is_leaf", true)
			}
		}

		// 删除组
		return tx.Delete(group).Error
	})
}

// deleteChildrenRecursive 递归删除子组
func (s *HostGroupService) deleteChildrenRecursive(tx *gorm.DB, parentID uint) error {
	var children []models.HostGroup
	if err := tx.Where("parent_id = ?", parentID).Find(&children).Error; err != nil {
		return err
	}

	for _, child := range children {
		// 递归删除子组的子组
		if err := s.deleteChildrenRecursive(tx, child.ID); err != nil {
			return err
		}

		// 将组内主机设置为未分组
		if err := tx.Model(&models.Host{}).
			Where("host_group_id = ?", child.ID).
			Update("host_group_id", nil).Error; err != nil {
			return err
		}

		// 删除子组
		if err := tx.Delete(&child).Error; err != nil {
			return err
		}
	}

	return nil
}

// GetByID 获取主机组详情
func (s *HostGroupService) GetByID(groupID uint, tenantID uint) (*models.HostGroup, error) {
	group := &models.HostGroup{}
	err := s.db.Where("id = ? AND tenant_id = ?", groupID, tenantID).
		Preload("Parent").
		Preload("Children").
		First(group).Error
	return group, err
}

// List 获取主机组列表（平铺）
func (s *HostGroupService) List(tenantID uint, parentID *uint, search string) ([]models.HostGroup, error) {
	var groups []models.HostGroup
	query := s.db.Where("tenant_id = ?", tenantID)

	if parentID != nil {
		query = query.Where("parent_id = ?", *parentID)
	}

	if search != "" {
		query = query.Where("name LIKE ? OR code LIKE ? OR description LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	err := query.Order("level, name").Find(&groups).Error
	return groups, err
}

// GetTree 获取主机组树
func (s *HostGroupService) GetTree(tenantID uint, rootID *uint) ([]*models.HostGroupTreeNode, error) {
	// 获取所有组
	var groups []models.HostGroup
	query := s.db.Where("tenant_id = ?", tenantID)
	if rootID != nil {
		// 如果指定了根节点，获取该节点及其所有子孙节点
		root := &models.HostGroup{}
		if err := s.db.First(root, *rootID).Error; err != nil {
			return nil, err
		}
		query = query.Where("path LIKE ?", root.Path+"%")
	}

	if err := query.Order("level, name").Find(&groups).Error; err != nil {
		return nil, err
	}

	// 构建树形结构
	return s.buildTree(groups, rootID), nil
}

// buildTree 构建树形结构
func (s *HostGroupService) buildTree(groups []models.HostGroup, rootID *uint) []*models.HostGroupTreeNode {
	// 创建ID到节点的映射
	nodeMap := make(map[uint]*models.HostGroupTreeNode)
	for i := range groups {
		nodeMap[groups[i].ID] = &models.HostGroupTreeNode{
			HostGroup: &groups[i],
			Children:  make([]*models.HostGroupTreeNode, 0),
		}
	}

	// 构建树
	var roots []*models.HostGroupTreeNode
	for _, group := range groups {
		node := nodeMap[group.ID]
		if group.ParentID == nil && rootID == nil {
			// 顶级节点
			roots = append(roots, node)
		} else if rootID != nil && group.ID == *rootID {
			// 指定的根节点
			roots = append(roots, node)
		} else if group.ParentID != nil {
			// 添加到父节点
			if parent, ok := nodeMap[*group.ParentID]; ok {
				parent.Children = append(parent.Children, node)
			}
		}
	}

	return roots
}

// Move 移动主机组
func (s *HostGroupService) Move(groupID uint, tenantID uint, newParentID *uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 获取要移动的组
		group := &models.HostGroup{}
		if err := tx.Where("id = ? AND tenant_id = ?", groupID, tenantID).First(group).Error; err != nil {
			return err
		}

		// 不能移动到自己
		if newParentID != nil && *newParentID == groupID {
			return errors.New("不能将组移动到自己")
		}

		// 检查是否移动到自己的子孙节点
		if newParentID != nil {
			if isDescendant, err := s.isDescendant(tx, groupID, *newParentID); err != nil {
				return err
			} else if isDescendant {
				return errors.New("不能将组移动到自己的子孙节点")
			}
		}

		// 检查新父组是否存在
		var newParent *models.HostGroup
		if newParentID != nil {
			newParent = &models.HostGroup{}
			if err := tx.First(newParent, *newParentID).Error; err != nil {
				return errors.New("目标父组不存在")
			}

			// 检查新父组是否属于同一租户
			if newParent.TenantID != tenantID {
				return errors.New("目标父组不属于当前租户")
			}

			// 如果新父组是叶子节点且有主机，不允许移入
			if newParent.IsLeaf && newParent.HostCount > 0 {
				return errors.New("目标父组已包含主机，不能添加子组")
			}
		}

		// 检查新位置是否有同名或同代码的组
		var count int64
		checkQuery := tx.Model(&models.HostGroup{}).
			Where("tenant_id = ? AND id != ?", tenantID, groupID).
			Where("(name = ? OR code = ?)", group.Name, group.Code)

		if newParentID != nil {
			checkQuery = checkQuery.Where("parent_id = ?", *newParentID)
		} else {
			checkQuery = checkQuery.Where("parent_id IS NULL")
		}

		if err := checkQuery.Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("目标位置已存在相同名称或代码的主机组")
		}

		// 更新原父组的子组数量
		if group.ParentID != nil {
			if err := tx.Model(&models.HostGroup{}).
				Where("id = ?", *group.ParentID).
				Update("child_count", gorm.Expr("child_count - 1")).Error; err != nil {
				return err
			}

			// 检查原父组是否还有其他子组
			var oldParentChildCount int64
			tx.Model(&models.HostGroup{}).
				Where("parent_id = ? AND id != ?", *group.ParentID, groupID).
				Count(&oldParentChildCount)

			if oldParentChildCount == 0 {
				tx.Model(&models.HostGroup{}).
					Where("id = ?", *group.ParentID).
					Update("is_leaf", true)
			}
		}

		// 更新新父组
		if newParentID != nil {
			// 更新新父组为非叶子节点
			if newParent.IsLeaf {
				if err := tx.Model(newParent).Update("is_leaf", false).Error; err != nil {
					return err
				}
			}

			// 更新新父组的子组数量
			if err := tx.Model(newParent).Update("child_count", gorm.Expr("child_count + 1")).Error; err != nil {
				return err
			}
		}

		// 计算新的层级和路径
		oldPath := group.Path
		var newLevel int
		var newPath string

		if newParentID != nil {
			newLevel = newParent.Level + 1
			newPath = newParent.Path + "/" + group.Code
		} else {
			newLevel = 1
			newPath = "/" + group.Code
		}

		// 更新组的父ID、层级和路径
		if err := tx.Model(group).Updates(map[string]interface{}{
			"parent_id": newParentID,
			"level":     newLevel,
			"path":      newPath,
		}).Error; err != nil {
			return err
		}

		// 更新所有子孙节点的路径和层级
		return s.updateDescendantPaths(tx, groupID, oldPath, newPath, newLevel)
	})
}

// isDescendant 检查targetID是否是groupID的子孙节点
func (s *HostGroupService) isDescendant(tx *gorm.DB, groupID, targetID uint) (bool, error) {
	target := &models.HostGroup{}
	if err := tx.First(target, targetID).Error; err != nil {
		return false, err
	}

	group := &models.HostGroup{}
	if err := tx.First(group, groupID).Error; err != nil {
		return false, err
	}

	// 检查目标节点的路径是否以当前节点路径开头
	return strings.HasPrefix(target.Path, group.Path+"/"), nil
}

// updateDescendantPaths 更新子孙节点的路径和层级
func (s *HostGroupService) updateDescendantPaths(tx *gorm.DB, groupID uint, oldPath, newPath string, newLevel int) error {
	// 获取所有子孙节点
	var descendants []models.HostGroup
	if err := tx.Where("path LIKE ? AND id != ?", oldPath+"/%", groupID).Find(&descendants).Error; err != nil {
		return err
	}

	// 更新每个子孙节点
	for _, desc := range descendants {
		// 计算新路径
		relativePath := strings.TrimPrefix(desc.Path, oldPath)
		desc.Path = newPath + relativePath

		// 计算新层级
		desc.Level = newLevel + strings.Count(relativePath, "/")

		// 更新
		if err := tx.Model(&desc).Updates(map[string]interface{}{
			"path":  desc.Path,
			"level": desc.Level,
		}).Error; err != nil {
			return err
		}
	}

	return nil
}

// AssignHosts 分配主机到组
func (s *HostGroupService) AssignHosts(groupID uint, tenantID uint, hostIDs []uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 检查组是否存在且是叶子节点
		group := &models.HostGroup{}
		if err := tx.Where("id = ? AND tenant_id = ?", groupID, tenantID).First(group).Error; err != nil {
			return err
		}

		if !group.IsLeaf {
			return errors.New("只能将主机分配到叶子节点")
		}

		// 检查主机是否都属于同一租户
		var count int64
		if err := tx.Model(&models.Host{}).
			Where("tenant_id = ? AND id IN ?", tenantID, hostIDs).
			Count(&count).Error; err != nil {
			return err
		}

		if int(count) != len(hostIDs) {
			return errors.New("部分主机不存在或不属于当前租户")
		}

		// 更新主机的组ID
		if err := tx.Model(&models.Host{}).
			Where("id IN ?", hostIDs).
			Update("host_group_id", groupID).Error; err != nil {
			return err
		}

		// 更新组的主机数量
		var newCount int64
		if err := tx.Model(&models.Host{}).
			Where("host_group_id = ?", groupID).
			Count(&newCount).Error; err != nil {
			return err
		}

		return tx.Model(group).Update("host_count", newCount).Error
	})
}

// RemoveHosts 从组中移除主机
func (s *HostGroupService) RemoveHosts(groupID uint, tenantID uint, hostIDs []uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 检查组是否存在
		group := &models.HostGroup{}
		if err := tx.Where("id = ? AND tenant_id = ?", groupID, tenantID).First(group).Error; err != nil {
			return err
		}

		// 移除主机（设置host_group_id为NULL）
		if err := tx.Model(&models.Host{}).
			Where("id IN ? AND host_group_id = ?", hostIDs, groupID).
			Update("host_group_id", nil).Error; err != nil {
			return err
		}

		// 更新组的主机数量
		var newCount int64
		if err := tx.Model(&models.Host{}).
			Where("host_group_id = ?", groupID).
			Count(&newCount).Error; err != nil {
			return err
		}

		return tx.Model(group).Update("host_count", newCount).Error
	})
}

// GetHostsByGroup 获取组内主机
func (s *HostGroupService) GetHostsByGroup(groupID uint, tenantID uint) ([]models.Host, error) {
	var hosts []models.Host
	err := s.db.Where("tenant_id = ? AND host_group_id = ?", tenantID, groupID).
		Preload("Credential").
		Preload("Tags").
		Find(&hosts).Error
	return hosts, err
}

// GetUngroupedHosts 获取未分组的主机
func (s *HostGroupService) GetUngroupedHosts(tenantID uint) ([]models.Host, error) {
	var hosts []models.Host
	err := s.db.Where("tenant_id = ? AND host_group_id IS NULL", tenantID).
		Preload("Credential").
		Preload("Tags").
		Find(&hosts).Error
	return hosts, err
}

// GetByPath 根据路径获取组
func (s *HostGroupService) GetByPath(tenantID uint, path string) (*models.HostGroup, error) {
	group := &models.HostGroup{}
	err := s.db.Where("tenant_id = ? AND path = ?", tenantID, path).First(group).Error
	return group, err
}

// GetAncestors 获取祖先节点
func (s *HostGroupService) GetAncestors(groupID uint, tenantID uint) ([]models.HostGroup, error) {
	group := &models.HostGroup{}
	if err := s.db.Where("id = ? AND tenant_id = ?", groupID, tenantID).First(group).Error; err != nil {
		return nil, err
	}

	var ancestors []models.HostGroup

	// 根据路径获取所有祖先
	parts := strings.Split(strings.Trim(group.Path, "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		ancestorPath := "/" + strings.Join(parts[:i+1], "/")
		ancestor := &models.HostGroup{}
		if err := s.db.Where("tenant_id = ? AND path = ?", tenantID, ancestorPath).First(ancestor).Error; err == nil {
			ancestors = append(ancestors, *ancestor)
		}
	}

	return ancestors, nil
}

// GetDescendants 获取所有后代节点
func (s *HostGroupService) GetDescendants(groupID uint, tenantID uint) ([]models.HostGroup, error) {
	group := &models.HostGroup{}
	if err := s.db.Where("id = ? AND tenant_id = ?", groupID, tenantID).First(group).Error; err != nil {
		return nil, err
	}

	var descendants []models.HostGroup
	err := s.db.Where("tenant_id = ? AND path LIKE ? AND id != ?",
		tenantID, group.Path+"/%", groupID).
		Order("level, name").
		Find(&descendants).Error

	return descendants, err
}

// UpdateHostGroup 批量更新组内主机的组ID
func (s *HostGroupService) UpdateHostGroup(tenantID uint, hostID uint, groupID *uint) error {
	// 检查主机是否存在
	host := &models.Host{}
	if err := s.db.Where("id = ? AND tenant_id = ?", hostID, tenantID).First(host).Error; err != nil {
		return errors.New("主机不存在")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 如果要分配到新组
		if groupID != nil {
			// 检查目标组是否存在且是叶子节点
			group := &models.HostGroup{}
			if err := tx.Where("id = ? AND tenant_id = ?", *groupID, tenantID).First(group).Error; err != nil {
				return errors.New("目标组不存在")
			}

			if !group.IsLeaf {
				return errors.New("只能将主机分配到叶子节点")
			}
		}

		// 更新原组的主机数量
		if host.HostGroupID != nil {
			if err := tx.Model(&models.HostGroup{}).
				Where("id = ?", *host.HostGroupID).
				Update("host_count", gorm.Expr("host_count - 1")).Error; err != nil {
				return err
			}
		}

		// 更新主机的组ID
		if err := tx.Model(host).Update("host_group_id", groupID).Error; err != nil {
			return err
		}

		// 更新新组的主机数量
		if groupID != nil {
			if err := tx.Model(&models.HostGroup{}).
				Where("id = ?", *groupID).
				Update("host_count", gorm.Expr("host_count + 1")).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
