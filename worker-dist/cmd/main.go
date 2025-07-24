package main

import (
	"ahop-worker/internal/worker"
	"ahop-worker/pkg/config"
	"ahop-worker/pkg/logger"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	configFile = flag.String("config", "", "配置文件路径")
	masterURL  = flag.String("master", "", "AHOP Master服务器地址")
)

func main() {
	flag.Parse()

	// 初始化配置
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 命令行参数覆盖配置
	if *masterURL != "" {
		cfg.Master.URL = *masterURL
	}

	// 初始化日志
	log := logger.InitLogger(cfg.Log)
	log.WithFields(logrus.Fields{
		"worker_name": cfg.Worker.Name,
		"master_url":  cfg.Master.URL,
		"version":     "1.0.0",
	}).Info("启动AHOP Worker")

	// 创建Worker实例
	w, err := worker.NewWorker(cfg, log)
	if err != nil {
		log.WithError(err).Fatal("创建Worker失败")
	}

	// 启动Worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动Worker服务
	if err := w.Start(ctx); err != nil {
		log.WithError(err).Fatal("启动Worker失败")
	}

	log.Info("Worker启动成功，正在等待任务...")

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Info("收到退出信号，正在停止Worker...")

	// 优雅停止
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := w.Stop(shutdownCtx); err != nil {
		log.WithError(err).Error("停止Worker时出错")
	} else {
		log.Info("Worker已优雅停止")
	}
}