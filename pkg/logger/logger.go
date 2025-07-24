package logger

import (
	"ahop/pkg/config"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Logger *logrus.Logger

// Initialize 初始化日志
func Initialize(cfg *config.Config) error {
	Logger = logrus.New()

	// 设置日志等级
	level, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	Logger.SetLevel(level)

	// 设置日志格式
	if cfg.Log.Format == "json" {
		Logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	// 创建日志目录
	if cfg.Log.FilePath != "" {
		logDir := filepath.Dir(cfg.Log.FilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		// 配置日志轮转
		rotateLogger := &lumberjack.Logger{
			Filename:   cfg.Log.FilePath,
			MaxSize:    cfg.Log.MaxSize,
			MaxBackups: cfg.Log.MaxBackups,
			MaxAge:     cfg.Log.MaxAge,
			Compress:   cfg.Log.Compress,
		}

		// 同时输出到文件和控制台
		multiWriter := io.MultiWriter(os.Stdout, rotateLogger)
		Logger.SetOutput(multiWriter)
	}

	return nil

}

// GetLogger 获取日志实例
func GetLogger() *logrus.Logger {
	return Logger
}
