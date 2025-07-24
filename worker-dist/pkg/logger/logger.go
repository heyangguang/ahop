package logger

import (
	"ahop-worker/pkg/config"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// InitLogger 初始化日志
func InitLogger(cfg config.LogConfig) *logrus.Logger {
	log := logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	// 设置日志格式
	if cfg.Format == "json" {
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		})
	}

	// 配置日志输出
	if cfg.FilePath != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.WithError(err).Warn("创建日志目录失败，使用标准输出")
			return log
		}

		// 配置滚动日志
		lumber := &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}

		// 使用 MultiWriter 同时输出到文件和控制台
		multiWriter := io.MultiWriter(os.Stdout, lumber)
		log.SetOutput(multiWriter)
	} else {
		// 如果没有配置文件路径，只输出到控制台
		log.SetOutput(os.Stdout)
	}

	return log
}