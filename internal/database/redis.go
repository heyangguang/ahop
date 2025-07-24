package database

import (
	"ahop/pkg/config"
	"ahop/pkg/queue"
	"sync"
)

var (
	redisQueueInstance *queue.RedisQueue
	redisQueueOnce     sync.Once
)

// GetRedisQueue 获取Redis队列的单例实例
func GetRedisQueue() *queue.RedisQueue {
	redisQueueOnce.Do(func() {
		cfg := config.GetConfig()
		redisQueueInstance = queue.NewRedisQueue(&queue.Config{
			Host:     cfg.Redis.Host,
			Port:     cfg.Redis.Port,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
			Prefix:   cfg.Redis.Prefix,
		})
	})
	return redisQueueInstance
}

// CloseRedisQueue 关闭Redis连接
func CloseRedisQueue() error {
	if redisQueueInstance != nil {
		return redisQueueInstance.Close()
	}
	return nil
}