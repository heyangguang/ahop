package config

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	JWT        JWTConfig `mapstructure:"jwt"`
	Log        LogConfig
	Redis      RedisConfig
	Credential CredentialConfig
	CORS       CORSConfig
}

type ServerConfig struct {
	Port string
	Mode string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type JWTConfig struct {
	SecretKey       string `mapstructure:"secret_key"`       // JWT密钥
	TokenDuration   string `mapstructure:"token_duration"`   // 令牌有效期，如 "24h"
	RefreshDuration string `mapstructure:"refresh_duration"` // 刷新令牌有效期
}

type LogConfig struct {
	Level      string
	FilePath   string
	MaxSize    int    // MB
	MaxBackups int    // 保留的备份文件数
	MaxAge     int    // 保留天数
	Compress   bool   // 是否压缩
	Format     string // json 或 text
}

type RedisConfig struct {
	Host     string // Redis主机地址
	Port     int    // Redis端口
	Password string // Redis密码
	DB       int    // Redis数据库编号
	Prefix   string // 队列键前缀
}

type CredentialConfig struct {
	EncryptionKey string // 凭证加密密钥（32字节用于AES-256）
}

type CORSConfig struct {
	AllowOrigins     []string // 允许的源
	AllowMethods     []string // 允许的HTTP方法
	AllowHeaders     []string // 允许的请求头
	ExposeHeaders    []string // 暴露的响应头
	AllowCredentials bool     // 是否允许携带凭证
	MaxAge           int      // 预检请求缓存时间（小时）
}

// 全局配置实例和同步锁
var (
	globalConfig *Config
	once         sync.Once
)

func GetConfig() *Config {
	once.Do(func() {
		var err error
		globalConfig, err = LoadConfig()
		if err != nil {
			// 如果加载失败，可以panic或使用默认配置
			panic("Failed to load config: " + err.Error())
		}
	})
	return globalConfig
}

// 获取环境变量，如果不存在则使用默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// 获取环境变量转换为int
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// 获取环境变量转换为bool
func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return strings.ToLower(value) == "true"
	}
	return defaultValue
}

// 获取环境变量转换为字符串数组（逗号分隔）
func getEnvAsStringArray(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// 处理逗号分隔的字符串，去除空格
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return defaultValue
}

func LoadConfig() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
	}

	config := &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Mode: getEnv("SERVER_MODE", "debug"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			DBName:   getEnv("DB_NAME", "auto_healing_platform"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			SecretKey:       getEnv("JWT_SECRET_KEY", "default-secret-change-me"),
			TokenDuration:   getEnv("JWT_TOKEN_DURATION", "24h"),
			RefreshDuration: getEnv("JWT_REFRESH_DURATION", "7d"),
		},
		Log: LogConfig{
			Level:      getEnv("LOG_LEVEL", "info"),
			FilePath:   getEnv("LOG_FILE_PATH", "logs/app.log"),
			MaxSize:    getEnvAsInt("LOG_MAX_SIZE", 100),
			MaxBackups: getEnvAsInt("LOG_MAX_BACKUPS", 7),
			MaxAge:     getEnvAsInt("LOG_MAX_AGE", 30),
			Compress:   getEnvAsBool("LOG_COMPRESS", true),
			Format:     getEnv("LOG_FORMAT", "json"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvAsInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
			Prefix:   getEnv("REDIS_PREFIX", "ahop:queue"),
		},
		Credential: CredentialConfig{
			EncryptionKey: getEnv("CREDENTIAL_ENCRYPTION_KEY", "ahop-credential-encryption-key32"),
		},
		CORS: CORSConfig{
			AllowOrigins: getEnvAsStringArray("CORS_ALLOW_ORIGINS", []string{"*"}),
			AllowMethods: getEnvAsStringArray("CORS_ALLOW_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}),
			AllowHeaders: getEnvAsStringArray("CORS_ALLOW_HEADERS", []string{"Origin", "Content-Type", "Authorization", "Accept", "X-Requested-With"}),
			ExposeHeaders: getEnvAsStringArray("CORS_EXPOSE_HEADERS", []string{"Content-Length", "Content-Type"}),
			AllowCredentials: getEnvAsBool("CORS_ALLOW_CREDENTIALS", false),
			MaxAge: getEnvAsInt("CORS_MAX_AGE", 12),
		},
	}

	return config, nil
}
