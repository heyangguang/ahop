package services

import (
	"fmt"

	"ahop/internal/database"
	"ahop/internal/models"

	"gorm.io/gorm"
)

type TagService struct {
	db *gorm.DB
}

func NewTagService() *TagService {
	return &TagService{
		db: database.GetDB(),
	}
}

// Create 创建标签
func (s *TagService) Create(tenantID uint, key, value, color string) (*models.Tag, error) {
	// 验证参数
	if err := s.ValidateTag(key, value); err != nil {
		return nil, err
	}

	// 如果没有指定颜色，使用默认值
	if color == "" {
		color = "#2196F3"
	}

	// 检查是否已存在
	var existingTag models.Tag
	err := s.db.Where("tenant_id = ? AND key = ? AND value = ?", tenantID, key, value).First(&existingTag).Error
	if err == nil {
		// 标签已存在
		return nil, fmt.Errorf("标签已存在")
	}

	tag := &models.Tag{
		TenantID: tenantID,
		Key:      key,
		Value:    value,
		Color:    color,
	}

	if err := s.db.Create(tag).Error; err != nil {
		return nil, err
	}

	return tag, nil
}

// GetByID 根据ID获取标签
func (s *TagService) GetByID(id uint) (*models.Tag, error) {
	var tag models.Tag
	err := s.db.Preload("Tenant").First(&tag, id).Error
	return &tag, err
}

// GetByTenant 获取租户的所有标签
func (s *TagService) GetByTenant(tenantID uint, key string) ([]models.Tag, error) {
	var tags []models.Tag
	query := s.db.Where("tenant_id = ?", tenantID)

	// 如果指定了key，则过滤
	if key != "" {
		query = query.Where("key = ?", key)
	}

	err := query.Order("key, value").Find(&tags).Error
	return tags, err
}

// GetGroupedByKey 按key分组获取标签
func (s *TagService) GetGroupedByKey(tenantID uint) (map[string][]models.Tag, error) {
	var tags []models.Tag
	err := s.db.Where("tenant_id = ?", tenantID).Order("key, value").Find(&tags).Error
	if err != nil {
		return nil, err
	}

	// 按key分组
	grouped := make(map[string][]models.Tag)
	for _, tag := range tags {
		grouped[tag.Key] = append(grouped[tag.Key], tag)
	}

	return grouped, nil
}

// Update 更新标签（只能更新颜色）
func (s *TagService) Update(id uint, color string) (*models.Tag, error) {
	var tag models.Tag
	if err := s.db.First(&tag, id).Error; err != nil {
		return nil, err
	}

	// 更新颜色
	tag.Color = color
	if err := s.db.Save(&tag).Error; err != nil {
		return nil, err
	}

	return &tag, nil
}

// Delete 删除标签
func (s *TagService) Delete(id uint) error {
	// 检查标签是否被使用
	var count int64
	s.db.Table("credential_tags").Where("tag_id = ?", id).Count(&count)
	if count > 0 {
		return fmt.Errorf("标签正在使用中，无法删除")
	}

	return s.db.Delete(&models.Tag{}, id).Error
}

// GetOrCreate 获取或创建标签
func (s *TagService) GetOrCreate(tenantID uint, key, value string) (*models.Tag, error) {
	// 先尝试获取
	var tag models.Tag
	err := s.db.Where("tenant_id = ? AND key = ? AND value = ?", tenantID, key, value).First(&tag).Error
	if err == nil {
		return &tag, nil
	}

	// 不存在则创建
	return s.Create(tenantID, key, value, "")
}

// ValidateTag 验证标签参数
func (s *TagService) ValidateTag(key, value string) error {
	// 验证key
	if len(key) == 0 || len(key) > 50 {
		return fmt.Errorf("标签键长度必须在1-50个字符之间")
	}

	// 验证value
	if len(value) == 0 || len(value) > 100 {
		return fmt.Errorf("标签值长度必须在1-100个字符之间")
	}

	// 验证字符（只允许字母、数字、中文、下划线、连字符）
	if !s.isValidTagString(key) {
		return fmt.Errorf("标签键只能包含字母、数字、中文、下划线和连字符")
	}

	if !s.isValidTagString(value) {
		return fmt.Errorf("标签值只能包含字母、数字、中文、下划线和连字符")
	}

	return nil
}

// isValidTagString 检查标签字符串是否合法
func (s *TagService) isValidTagString(str string) bool {
	for _, r := range str {
		if !((r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-' ||
			r >= 0x4e00 && r <= 0x9fa5) { // 中文字符范围
			return false
		}
	}
	return true
}

// GetByCredential 获取凭证的标签
func (s *TagService) GetByCredential(credentialID uint) ([]models.Tag, error) {
	var credential models.Credential
	err := s.db.Preload("Tags").First(&credential, credentialID).Error
	if err != nil {
		return nil, err
	}
	return credential.Tags, nil
}

// UpdateCredentialTags 更新凭证的标签（全量替换）
func (s *TagService) UpdateCredentialTags(credentialID uint, tagIDs []uint) error {
	var credential models.Credential
	if err := s.db.First(&credential, credentialID).Error; err != nil {
		return err
	}

	// 获取要关联的标签（确保都是同租户的）
	var tags []models.Tag
	if len(tagIDs) > 0 {
		if err := s.db.Where("id IN ? AND tenant_id = ?", tagIDs, credential.TenantID).Find(&tags).Error; err != nil {
			return err
		}

		if len(tags) != len(tagIDs) {
			return fmt.Errorf("部分标签不存在或不属于当前租户")
		}
	}

	// 替换标签
	return s.db.Model(&credential).Association("Tags").Replace(tags)
}

// GetAllKeys 获取所有标签键
func (s *TagService) GetAllKeys(tenantID uint) ([]string, error) {
	var keys []string
	err := s.db.Model(&models.Tag{}).
		Where("tenant_id = ?", tenantID).
		Distinct("key").
		Pluck("key", &keys).Error
	return keys, err
}

// GetValuesByKey 根据key获取所有值
func (s *TagService) GetValuesByKey(tenantID uint, key string) ([]string, error) {
	var values []string
	err := s.db.Model(&models.Tag{}).
		Where("tenant_id = ? AND key = ?", tenantID, key).
		Pluck("value", &values).Error
	return values, err
}
