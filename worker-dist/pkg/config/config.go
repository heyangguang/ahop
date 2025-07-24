package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Duration 自定义Duration类型，支持JSON字符串解析
type Duration time.Duration

// UnmarshalJSON 实现JSON解析
func (d *Duration) UnmarshalJSON(data []byte) error {
	// 移除引号
	s := strings.Trim(string(data), `"`)
	
	// 解析duration字符串
	duration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	
	*d = Duration(duration)
	return nil
}

// MarshalJSON 实现JSON序列化
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// String 返回字符串表示
func (d Duration) String() string {
	return time.Duration(d).String()
}

// ToDuration 转换为time.Duration
func (d Duration) ToDuration() time.Duration {
	return time.Duration(d)
}

// Config Worker配置
type Config struct {
	Worker WorkerConfig `json:"worker"`
	Master MasterConfig `json:"master"`
	Log    LogConfig    `json:"log"`
}

// WorkerConfig Worker配置
type WorkerConfig struct {
	WorkerID    string `json:"worker_id"`   // Worker唯一标识（必须配置）
	AccessKey   string `json:"access_key"`  // 认证密钥
	SecretKey   string `json:"secret_key"`  // 认证密钥
	Name        string `json:"name"`        // Worker名称
	Concurrency int    `json:"concurrency"` // 并发任务数
}

// MasterConfig AHOP Master服务器配置
type MasterConfig struct {
	URL string `json:"url"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	Prefix   string `json:"prefix"`
}

// DatabaseConfig 数据库配置（运行时从Master获取）
type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
	SSLMode  string `json:"sslmode"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `json:"level"`
	FilePath   string `json:"file_path"`
	MaxSize    int    `json:"max_size"`
	MaxBackups int    `json:"max_backups"`
	MaxAge     int    `json:"max_age"`
	Compress   bool   `json:"compress"`
	Format     string `json:"format"`
}

// LoadConfig 加载配置
func LoadConfig(configFile string) (*Config, error) {
	cfg := defaultConfig()

	// 如果指定了配置文件，先加载文件配置
	if configFile != "" {
		if err := loadFromFile(cfg, configFile); err != nil {
			return nil, fmt.Errorf("加载配置文件失败: %v", err)
		}
	}

	// 环境变量覆盖配置
	loadFromEnv(cfg)

	// 验证配置
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %v", err)
	}

	return cfg, nil
}

// defaultConfig 默认配置
func defaultConfig() *Config {
	hostname, _ := os.Hostname()
	
	return &Config{
		Worker: WorkerConfig{
			WorkerID:    "", // 必须通过配置文件提供
			AccessKey:   "", // 必须通过配置文件或环境变量提供
			SecretKey:   "", // 必须通过配置文件或环境变量提供
			Name:        fmt.Sprintf("AHOP Worker on %s", hostname),
			Concurrency: 2,
		},
		Master: MasterConfig{
			URL: "http://localhost:8080",
		},
		Log: LogConfig{
			Level:      "info",
			FilePath:   "logs/worker.log",
			MaxSize:    100,
			MaxBackups: 7,
			MaxAge:     30,
			Compress:   true,
			Format:     "json",
		},
	}
}

// loadFromFile 从文件加载配置
func loadFromFile(cfg *Config, filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, cfg)
}

// loadFromEnv 从环境变量加载配置
func loadFromEnv(cfg *Config) {
	// Worker配置
	if v := os.Getenv("WORKER_ACCESS_KEY"); v != "" {
		cfg.Worker.AccessKey = v
	}
	if v := os.Getenv("WORKER_SECRET_KEY"); v != "" {
		cfg.Worker.SecretKey = v
	}
	if v := os.Getenv("WORKER_NAME"); v != "" {
		cfg.Worker.Name = v
	}
	if v := os.Getenv("WORKER_CONCURRENCY"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Worker.Concurrency = i
		}
	}

	// Master配置
	if v := os.Getenv("AHOP_MASTER_URL"); v != "" {
		cfg.Master.URL = v
	}

	// 日志配置
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("LOG_FILE_PATH"); v != "" {
		cfg.Log.FilePath = v
	}
}

// validateConfig 验证配置
func validateConfig(cfg *Config) error {
	if cfg.Worker.AccessKey == "" {
		return fmt.Errorf("Worker AccessKey不能为空")
	}
	if cfg.Worker.SecretKey == "" {
		return fmt.Errorf("Worker SecretKey不能为空")
	}
	if cfg.Worker.Concurrency <= 0 {
		return fmt.Errorf("Worker并发数必须大于0")
	}
	if cfg.Worker.Concurrency > 50 {
		return fmt.Errorf("Worker并发数不能超过50")
	}
	if cfg.Master.URL == "" {
		return fmt.Errorf("Master服务器URL不能为空")
	}
	return nil
}