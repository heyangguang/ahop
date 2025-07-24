package models

import (
	"time"
)

// WorkerAuth Worker认证授权模型
type WorkerAuth struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	AccessKey   string    `gorm:"size:50;not null;uniqueIndex" json:"access_key"`
	SecretKey   string    `gorm:"size:100;not null" json:"-"` // 不在JSON中返回
	Environment string    `gorm:"size:50" json:"environment"`
	Description string    `gorm:"type:text" json:"description"`
	Status      string    `gorm:"size:20;default:'active'" json:"status"` // active/disabled
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 数据库凭据配置
	DBHost     string `gorm:"size:100;default:'localhost'" json:"db_host"`
	DBPort     int    `gorm:"default:5432" json:"db_port"`
	DBUser     string `gorm:"size:50;default:'ahop_worker'" json:"db_user"`
	DBPassword string `gorm:"size:100" json:"-"` // 不在JSON中返回
	DBName     string `gorm:"size:50;default:'auto_healing_platform'" json:"db_name"`
	
	// Redis配置
	RedisHost     string `gorm:"size:100;default:'localhost'" json:"redis_host"`
	RedisPort     int    `gorm:"default:6379" json:"redis_port"`
	RedisPassword string `gorm:"size:100" json:"-"` // 不在JSON中返回
	RedisDB       int    `gorm:"default:0" json:"redis_db"`
	RedisPrefix   string `gorm:"size:50;default:'ahop:queue'" json:"redis_prefix"`
}

// GetDatabaseConfig 获取数据库配置
func (wa *WorkerAuth) GetDatabaseConfig() map[string]interface{} {
	return map[string]interface{}{
		"host":     wa.DBHost,
		"port":     wa.DBPort,
		"user":     wa.DBUser,
		"password": wa.DBPassword,
		"dbname":   wa.DBName,
		"sslmode":  "disable",
	}
}

// GetRedisConfig 获取Redis配置
func (wa *WorkerAuth) GetRedisConfig() map[string]interface{} {
	return map[string]interface{}{
		"host":     wa.RedisHost,
		"port":     wa.RedisPort,
		"password": wa.RedisPassword,
		"db":       wa.RedisDB,
		"prefix":   wa.RedisPrefix,
	}
}

// IsActive 检查授权是否激活
func (wa *WorkerAuth) IsActive() bool {
	return wa.Status == "active"
}