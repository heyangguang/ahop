package services

import (
	"ahop/pkg/config"
	"ahop/pkg/logger"
	"ahop/pkg/queue"
	"context"
	"sync"

	"github.com/sirupsen/logrus"
)

// TaskRecoveryService 任务恢复服务
type TaskRecoveryService struct {
	queue  *queue.RedisQueue
	log    *logrus.Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewTaskRecoveryService 创建任务恢复服务
func NewTaskRecoveryService() *TaskRecoveryService {
	cfg := config.GetConfig()
	log := logger.GetLogger()

	// 创建Redis队列实例
	queueConfig := &queue.Config{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		Prefix:   cfg.Redis.Prefix,
	}
	redisQueue := queue.NewRedisQueue(queueConfig)

	return &TaskRecoveryService{
		queue: redisQueue,
		log:   log,
	}
}

// Start 启动任务恢复服务
func (s *TaskRecoveryService) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.log.Info("启动任务恢复服务")

	// 任务恢复功能现在由Worker端实现
	// 不再需要Master端的恢复服务

	return nil
}

// Stop 停止任务恢复服务
func (s *TaskRecoveryService) Stop() error {
	s.log.Info("停止任务恢复服务")

	if s.cancel != nil {
		s.cancel()
	}

	// 等待所有goroutine完成
	s.wg.Wait()

	// 关闭Redis连接
	if s.queue != nil {
		s.queue.Close()
	}

	return nil
}