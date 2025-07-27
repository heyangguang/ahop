package worker

import (
	"ahop-worker/internal/executor"
	"ahop-worker/internal/models"
	"ahop-worker/internal/types"
	"ahop-worker/pkg/auth"
	"ahop-worker/pkg/config"
	"ahop-worker/pkg/database"
	"ahop-worker/pkg/queue"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Worker 分布式Worker
type Worker struct {
	// 配置
	config *config.Config
	log    *logrus.Logger

	// 组件
	queue               *queue.RedisQueue
	db                  *gorm.DB
	executors           map[string]executor.Executor
	authClient          *auth.AuthClient
	gitSyncExecutor     *executor.GitSyncExecutor
	templateCopyExecutor *executor.TemplateCopyExecutor

	// 状态管理
	id           string
	status       string
	taskCount    int
	totalTasks   int64
	successTasks int64
	failedTasks  int64
	mu           sync.RWMutex

	// 控制器
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	// 心跳和注册
	heartbeatTicker *time.Ticker
}

// NewWorker 创建新Worker
func NewWorker(cfg *config.Config, log *logrus.Logger) (*Worker, error) {
	// 1. 获取Worker ID（必须配置）
	workerID := cfg.Worker.WorkerID
	if workerID == "" {
		return nil, fmt.Errorf("worker_id未配置，请在配置文件中设置worker.worker_id")
	}
	log.WithField("worker_id", workerID).Info("使用配置的worker_id")
	
	// 2. 向Master认证获取配置
	authClient := auth.NewAuthClient(cfg.Master.URL, cfg.Worker.AccessKey, cfg.Worker.SecretKey)
	authResult, err := authClient.Authenticate(workerID)
	if err != nil {
		return nil, fmt.Errorf("向Master认证失败: %v", err)
	}

	log.WithFields(logrus.Fields{
		"worker_id": workerID,
		"master_url": cfg.Master.URL,
	}).Info("Worker认证成功，获得数据库和Redis凭据")

	// 3. 使用从Master获得的Redis配置连接Redis
	redisQueue := queue.NewRedisQueue(&queue.Config{
		Host:     authResult.RedisConfig.Host,
		Port:     authResult.RedisConfig.Port,
		Password: authResult.RedisConfig.Password,
		DB:       authResult.RedisConfig.DB,
		Prefix:   authResult.RedisConfig.Prefix,
	})

	// 测试Redis连接
	if err := redisQueue.Ping(); err != nil {
		return nil, fmt.Errorf("Redis连接失败: %v", err)
	}

	// 4. 使用从Master获得的凭据连接数据库
	dbConnConfig := config.DatabaseConfig{
		Host:     authResult.DatabaseConfig.Host,
		Port:     authResult.DatabaseConfig.Port,
		User:     authResult.DatabaseConfig.User,
		Password: authResult.DatabaseConfig.Password,
		DBName:   authResult.DatabaseConfig.DBName,
		SSLMode:  authResult.DatabaseConfig.SSLMode,
	}
	
	db, err := database.Connect(dbConnConfig)
	if err != nil {
		return nil, fmt.Errorf("数据库连接失败: %v", err)
	}

	w := &Worker{
		config:     cfg,
		log:        log,
		queue:      redisQueue,
		db:         db,
		executors:  make(map[string]executor.Executor),
		id:         workerID,  // 使用生成的唯一Worker ID
		status:     "offline",
		authClient: authClient,
	}

	// 注册执行器
	w.registerExecutors()

	return w, nil
}

// Start 启动Worker
func (w *Worker) Start(ctx context.Context) error {
	w.ctx, w.cancelFunc = context.WithCancel(ctx)

	w.mu.Lock()
	w.status = "online"
	w.mu.Unlock()

	// 注册到数据库
	if err := w.registerWorker(); err != nil {
		return fmt.Errorf("注册Worker失败: %v", err)
	}

	// 启动心跳
	w.startHeartbeat()

	// 启动任务恢复服务
	w.startTaskRecovery()

	// 启动Git同步订阅服务
	w.startGitSyncSubscriber()

	// 启动Git扫描队列消费者
	w.startGitScanConsumer()

	// 启动Git仓库清理服务
	w.startGitRepoCleanup()

	// 启动任务消费者
	for i := 0; i < w.config.Worker.Concurrency; i++ {
		w.wg.Add(1)
		go w.taskConsumer(i)
	}
	
	// 启动模板复制消息消费者
	w.wg.Add(1)
	go w.templateCopyConsumer()

	w.log.WithFields(logrus.Fields{
		"worker_id":   w.id,
		"concurrency": w.config.Worker.Concurrency,
	}).Info("Worker启动成功")

	return nil
}

// Stop 停止Worker
func (w *Worker) Stop(ctx context.Context) error {
	w.log.Info("开始停止Worker")

	w.mu.Lock()
	w.status = "stopping"
	w.mu.Unlock()

	// 停止心跳
	if w.heartbeatTicker != nil {
		w.heartbeatTicker.Stop()
	}

	// 取消上下文
	if w.cancelFunc != nil {
		w.cancelFunc()
	}

	// 等待所有任务完成或超时
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("所有任务已完成")
	case <-ctx.Done():
		w.log.Warn("停止超时，强制退出")
	}

	// 通知Master断开连接
	if w.authClient != nil {
		if err := w.authClient.DisconnectWorker(); err != nil {
			w.log.WithError(err).Warn("通知Master断开连接失败")
		} else {
			w.log.Info("已通知Master断开连接")
		}
	}

	w.mu.Lock()
	w.status = "offline"
	w.mu.Unlock()

	// 更新数据库状态
	w.updateWorkerStatus()

	// 关闭连接
	if w.queue != nil {
		w.queue.Close()
	}

	w.log.Info("Worker已停止")
	return nil
}

// registerExecutors 注册执行器
func (w *Worker) registerExecutors() {
	// 获取Redis客户端（用于实时日志发布）
	redisClient := w.queue.GetClient()

	// 注册Ansible执行器（用于collect任务）
	ansibleExecutor := executor.NewAnsibleExecutor(w.db)
	ansibleExecutor.SetRedisClient(redisClient)
	for _, taskType := range ansibleExecutor.GetSupportedTypes() {
		w.executors[taskType] = ansibleExecutor
	}

	// 注册Ping执行器（用于ping任务）
	pingExecutor := executor.NewPingExecutor()
	pingExecutor.SetRedisClient(redisClient)
	for _, taskType := range pingExecutor.GetSupportedTypes() {
		w.executors[taskType] = pingExecutor
	}

	// 注册模板执行器（用于template任务）
	// 模板独立存储目录，默认为 repos 同级的 templates 目录
	templateBaseDir := filepath.Join(filepath.Dir(w.config.Git.RepoBaseDir), "templates")
	templateExecutor := executor.NewTemplateExecutor(w.db, w.config.Git.RepoBaseDir, templateBaseDir, w.log)
	templateExecutor.SetRedisClient(redisClient)
	for _, taskType := range templateExecutor.GetSupportedTypes() {
		w.executors[taskType] = templateExecutor
	}

	// 创建Git同步执行器（不作为普通任务执行器注册）
	w.gitSyncExecutor = executor.NewGitSyncExecutor(w.db, w.authClient, w.config.Git.RepoBaseDir, w.log)
	
	// 创建模板复制执行器
	w.templateCopyExecutor = executor.NewTemplateCopyExecutor(w.config.Git.RepoBaseDir, templateBaseDir, w.log)

	w.log.WithField("executors", len(w.executors)).Info("已注册任务执行器")
}

// registerWorker 注册Worker到数据库
func (w *Worker) registerWorker() error {
	// 获取系统信息
	hostname, _ := os.Hostname()
	ipAddress := w.getLocalIP()

	// 支持的任务类型
	taskTypes := make([]string, 0)
	for taskType := range w.executors {
		taskTypes = append(taskTypes, taskType)
	}

	worker := &models.Worker{
		WorkerID:      w.id,
		WorkerType:    "distributed",
		Hostname:      hostname,
		IPAddress:     ipAddress,
		Concurrent:    w.config.Worker.Concurrency,
		TaskTypes:     fmt.Sprintf("%v", taskTypes),
		Version:       "1.0.0",
		Status:        "online",
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
	}

	// 检查是否已存在
	var existingWorker models.Worker
	err := w.db.Where("worker_id = ?", w.id).First(&existingWorker).Error
	if err == nil {
		// 更新现有记录
		return w.db.Model(&existingWorker).Updates(worker).Error
	} else if err == gorm.ErrRecordNotFound {
		// 创建新记录
		return w.db.Create(worker).Error
	}

	return err
}

// startHeartbeat 启动心跳
func (w *Worker) startHeartbeat() {
	w.heartbeatTicker = time.NewTicker(30 * time.Second) // 每30秒发送心跳

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer w.heartbeatTicker.Stop()

		for {
			select {
			case <-w.ctx.Done():
				return
			case <-w.heartbeatTicker.C:
				w.sendHeartbeat()
			}
		}
	}()
}

// startTaskRecovery 启动任务恢复服务
func (w *Worker) startTaskRecovery() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(2 * time.Minute) // 每2分钟扫描一次
		defer ticker.Stop()
		
		for {
			select {
			case <-w.ctx.Done():
				return
			case <-ticker.C:
				if err := w.queue.RecoverOrphanedTasks(); err != nil {
					w.log.WithError(err).Warn("恢复孤儿任务失败")
				} else {
					w.log.Debug("完成孤儿任务扫描")
				}
			}
		}
	}()
}

// sendHeartbeat 发送心跳
func (w *Worker) sendHeartbeat() {
	w.mu.RLock()
	taskCount := w.taskCount
	totalTasks := w.totalTasks
	successTasks := w.successTasks
	failedTasks := w.failedTasks
	w.mu.RUnlock()

	// 获取系统资源使用情况
	cpuPercent, _ := cpu.Percent(time.Second, false)
	memInfo, _ := mem.VirtualMemory()

	var cpuUsage float64
	if len(cpuPercent) > 0 {
		cpuUsage = cpuPercent[0]
	}

	var memUsage float64
	if memInfo != nil {
		memUsage = memInfo.UsedPercent
	}

	// 更新数据库
	updates := map[string]interface{}{
		"last_heartbeat": time.Now(),
		"status":         w.status,
		"task_count":     taskCount,
		"cpu_usage":      cpuUsage,
		"memory_usage":   memUsage,
		"total_tasks":    totalTasks,
		"success_tasks":  successTasks,
		"failed_tasks":   failedTasks,
	}

	if err := w.db.Model(&models.Worker{}).Where("worker_id = ?", w.id).Updates(updates).Error; err != nil {
		w.log.WithError(err).Error("发送心跳失败")
	}
	
	// 通知Master更新心跳
	if w.authClient != nil {
		if err := w.authClient.SendHeartbeat(w.id); err != nil {
			w.log.WithError(err).Warn("更新Master心跳失败")
		}
	}
}

// updateWorkerStatus 更新Worker状态
func (w *Worker) updateWorkerStatus() {
	w.mu.RLock()
	status := w.status
	w.mu.RUnlock()

	w.db.Model(&models.Worker{}).Where("worker_id = ?", w.id).Updates(map[string]interface{}{
		"status":         status,
		"last_heartbeat": time.Now(),
	})
}

// templateCopyConsumer 模板复制消息消费者
func (w *Worker) templateCopyConsumer() {
	defer w.wg.Done()
	
	// 订阅模板复制通道（使用模式订阅）
	pubsub := w.queue.GetClient().PSubscribe(w.ctx, "template:copy:*")
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	
	log := w.log.WithFields(logrus.Fields{
		"worker_id": w.id,
		"consumer":  "template_copy",
	})
	
	log.Info("模板复制消费者启动")
	
	for {
		select {
		case <-w.ctx.Done():
			log.Info("模板复制消费者收到退出信号")
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}
			
			// 处理消息
			log.WithFields(logrus.Fields{
				"channel": msg.Channel,
				"pattern": msg.Pattern,
			}).Debug("收到模板复制消息")
			
			if err := w.templateCopyExecutor.HandleTemplateCopy(w.ctx, []byte(msg.Payload)); err != nil {
				log.WithError(err).Errorf("处理模板复制消息失败: channel=%s", msg.Channel)
			} else {
				log.Infof("成功处理模板复制消息: channel=%s", msg.Channel)
			}
		}
	}
}

// taskConsumer 任务消费者
func (w *Worker) taskConsumer(consumerID int) {
	defer w.wg.Done()

	log := w.log.WithFields(logrus.Fields{
		"worker_id":   w.id,
		"consumer_id": consumerID,
	})

	log.Info("任务消费者启动")

	for {
		select {
		case <-w.ctx.Done():
			log.Info("任务消费者收到退出信号")
			return
		default:
			// 从队列获取任务
			if err := w.processTask(log); err != nil {
				log.WithError(err).Error("处理任务失败")
				time.Sleep(1 * time.Second) // 避免错误循环
			}
		}
	}
}

// processTask 处理单个任务
func (w *Worker) processTask(log *logrus.Entry) error {
	// 从队列获取任务（1秒超时）
	taskMessage, err := w.queue.Dequeue(1 * time.Second)
	if err != nil {
		return fmt.Errorf("获取任务失败: %v", err)
	}

	if taskMessage == nil {
		return nil // 没有任务
	}

	// 增加任务计数
	w.mu.Lock()
	w.taskCount++
	w.totalTasks++
	w.mu.Unlock()

	// 确保任务完成后减少计数
	defer func() {
		w.mu.Lock()
		w.taskCount--
		w.mu.Unlock()
	}()

	taskLog := log.WithFields(logrus.Fields{
		"task_id":     taskMessage.TaskID,
		"task_type":   taskMessage.TaskType,
		"tenant_id":   taskMessage.TenantID,
		"tenant_name": taskMessage.TenantName,
		"user_id":     taskMessage.UserID,
		"username":    taskMessage.Username,
		"source":      taskMessage.Source,
	})

	taskLog.Info("开始处理任务")

	// 更新任务状态为运行中
	if err := w.queue.UpdateTaskStatus(taskMessage.TaskID, "running", 0, w.id); err != nil {
		taskLog.WithError(err).Error("更新任务状态失败")
	}

	// 从数据库获取任务详情
	task, err := w.getTaskFromDB(taskMessage.TaskID)
	if err != nil {
		taskLog.WithError(err).Error("获取任务详情失败")
		// 判断是否是临时错误，决定是否重新入队
		if w.shouldRetryTask(err) {
			taskLog.Warn("任务将重新入队")
			if requeueErr := w.requeueTask(taskMessage); requeueErr != nil {
				taskLog.WithError(requeueErr).Error("重新入队失败")
			}
		} else {
			// 不可恢复的错误，标记为失败
			w.queue.SetTaskResult(taskMessage.TaskID, nil, err.Error())
		}
		return nil // 返回nil避免消费者循环中断
	}

	// 准备主机信息（使用新的V2版本）
	if err := w.prepareHostInfoV2(task.TenantID, taskMessage.Params); err != nil {
		taskLog.WithError(err).Error("准备主机信息失败")
		// 判断是否是临时错误，决定是否重新入队
		if w.shouldRetryTask(err) {
			taskLog.Warn("主机信息准备失败，任务将重新入队")
			if requeueErr := w.requeueTask(taskMessage); requeueErr != nil {
				taskLog.WithError(requeueErr).Error("重新入队失败")
			}
		} else {
			// 不可恢复的错误，标记为失败
			w.queue.SetTaskResult(taskMessage.TaskID, nil, err.Error())
			w.updateTaskInDB(task, &executor.TaskResult{
				Success: false,
				Error:   err.Error(),
				Details: make(map[string]interface{}),
			})
			w.mu.Lock()
			w.failedTasks++
			w.mu.Unlock()
		}
		return nil // 返回nil表示任务已处理完成
	}

	// 执行任务
	result := w.executeTask(task, taskMessage, taskLog)

	// 更新统计
	w.mu.Lock()
	if result.Success {
		w.successTasks++
	} else {
		w.failedTasks++
	}
	w.mu.Unlock()

	// 保存结果到队列
	if result.Success {
		w.queue.SetTaskResult(taskMessage.TaskID, result.Result, "")
	} else {
		w.queue.SetTaskResult(taskMessage.TaskID, result.Result, result.Error)
	}

	// 更新数据库任务状态
	w.updateTaskInDB(task, result)

	// 如果任务成功且需要更新主机信息
	if result.Success && w.shouldUpdateHostInfo(taskMessage.TaskType) {
		if err := w.updateHostInfo(taskMessage.TaskType, taskMessage.Params, result); err != nil {
			taskLog.WithError(err).Error("更新主机信息失败")
			// 注意：主机信息更新失败不影响任务本身的状态
		}
	}

	if result.Success {
		taskLog.Info("任务执行成功")
	} else {
		taskLog.WithField("error", result.Error).Error("任务执行失败")
	}

	return nil
}

// getTaskFromDB 从数据库获取任务详情
func (w *Worker) getTaskFromDB(taskID string) (*models.Task, error) {
	var task models.Task
	if err := w.db.Where("task_id = ?", taskID).First(&task).Error; err != nil {
		return nil, fmt.Errorf("任务不存在: %v", err)
	}
	return &task, nil
}


// executeTask 执行任务
func (w *Worker) executeTask(task *models.Task, taskMessage *queue.TaskMessage, log *logrus.Entry) *executor.TaskResult {
	// 查找执行器
	exec, exists := w.executors[taskMessage.TaskType]
	if !exists {
		return &executor.TaskResult{
			Success: false,
			Error:   fmt.Sprintf("不支持的任务类型: %s", taskMessage.TaskType),
			Details: make(map[string]interface{}),
			Logs:    make([]string, 0),
		}
	}

	// 验证参数
	if err := exec.ValidateParams(taskMessage.Params); err != nil {
		return &executor.TaskResult{
			Success: false,
			Error:   fmt.Sprintf("参数验证失败: %v", err),
			Details: make(map[string]interface{}),
			Logs:    make([]string, 0),
		}
	}

	// 创建任务上下文
	taskCtx := &executor.TaskContext{
		TaskID:     taskMessage.TaskID,
		TaskType:   taskMessage.TaskType,
		TenantID:   taskMessage.TenantID,
		TenantName: taskMessage.TenantName,
		UserID:     taskMessage.UserID,
		Username:   taskMessage.Username,
		Source:     taskMessage.Source,
		Params:     taskMessage.Params,
		Timeout:    time.Duration(task.Timeout) * time.Second,
		CreatedAt:  time.Unix(taskMessage.Created, 0),
	}

	// 设置回调函数
	onProgress := func(progress int, message string) {
		w.logTaskProgress(taskMessage.TaskID, progress, message)
	}

	onLog := func(level, source, message, hostName, stderr string) {
		w.logTaskMessage(taskMessage.TaskID, level, source, message, hostName, stderr)
	}

	// 执行任务
	ctx, cancel := context.WithTimeout(w.ctx, taskCtx.Timeout)
	defer cancel()

	return exec.Execute(ctx, taskCtx, onProgress, onLog)
}

// updateTaskInDB 更新数据库中的任务状态
func (w *Worker) updateTaskInDB(task *models.Task, result *executor.TaskResult) {
	now := time.Now()
	updates := map[string]interface{}{
		"worker_id":   w.id,
		"progress":    100,
		"updated_at":  now,
		"finished_at": &now,
	}

	if result.Success {
		updates["status"] = "success"
		if result.Result != nil {
			resultData, _ := json.Marshal(result.Result)
			updates["result"] = models.JSON(resultData)
		}
	} else {
		updates["status"] = "failed"
		updates["error"] = result.Error
		// 失败时也保存result
		if result.Result != nil {
			resultData, _ := json.Marshal(result.Result)
			updates["result"] = models.JSON(resultData)
		}
	}

	w.db.Model(task).Updates(updates)
}

// logTaskProgress 更新任务进度（不再写入日志）
func (w *Worker) logTaskProgress(taskID string, progress int, message string) {
	// 只更新数据库中的进度值
	w.db.Model(&models.Task{}).Where("task_id = ?", taskID).Update("progress", progress)
	
	// 更新 Redis 中的进度信息（前端可以实时查询）
	// message 可以作为状态描述存储在 Redis 中，但不写入日志
	w.queue.UpdateTaskStatus(taskID, "running", progress, w.id)
}

// logTaskMessage 记录任务日志
func (w *Worker) logTaskMessage(taskID, level, source, message, hostName, stderr string) {
	timestamp := time.Now()
	logEntry := &models.TaskLog{
		TaskID:    taskID,
		Timestamp: timestamp,
		Level:     level,
		Source:    source,
		Message:   message,
		HostName:  hostName,
	}

	// 不再存储stderr到Data字段，保持Data为空

	w.db.Create(logEntry)
	
	// 实时发布日志到Redis（不等待数据库写入）
	go func() {
		channel := fmt.Sprintf("task:logs:%s", taskID)
		logMsg := map[string]interface{}{
			"task_id":   taskID,
			"timestamp": timestamp.Unix(),
			"level":     level,
			"source":    source,
			"message":   message,
			"host_name": hostName,
		}
		
		// 始终添加stderr字段（即使为空）
		logMsg["stderr"] = stderr
		
		if msgData, err := json.Marshal(logMsg); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			w.queue.GetClient().Publish(ctx, channel, msgData)
		}
	}()
}

// getLocalIP 获取本地IP地址
func (w *Worker) getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return ""
}

// shouldRetryTask 判断错误是否应该重试任务
func (w *Worker) shouldRetryTask(err error) bool {
	if err == nil {
		return false
	}
	
	errMsg := err.Error()
	
	// 临时性错误，应该重试
	temporaryErrors := []string{
		"cached plan must not change result type", // PostgreSQL计划缓存错误
		"connection refused",                       // 连接被拒绝
		"connection reset",                         // 连接重置  
		"timeout",                                  // 超时
		"temporary failure",                        // 临时失败
		"service unavailable",                      // 服务不可用
		"too many connections",                     // 连接数过多
	}
	
	for _, tempErr := range temporaryErrors {
		if strings.Contains(strings.ToLower(errMsg), tempErr) {
			return true
		}
	}
	
	// 永久性错误，不应该重试
	permanentErrors := []string{
		"任务不存在",         // 任务已被删除
		"主机不存在",         // 主机不存在
		"凭证不存在",         // 凭证不存在
		"权限不足",          // 权限问题
		"参数格式错误",       // 参数错误
		"不支持的任务类型",    // 任务类型错误
	}
	
	for _, permErr := range permanentErrors {
		if strings.Contains(errMsg, permErr) {
			return false
		}
	}
	
	// 默认情况下，对于未知错误也进行重试（最多重试几次）
	return true
}

// requeueTask 重新入队任务（现在由恢复服务处理，这里只是占位）
func (w *Worker) requeueTask(taskMessage *queue.TaskMessage) error {
	// Worker不再直接重新入队任务，任务恢复由恢复服务处理
	// 这里直接返回错误，让任务最终失败
	return fmt.Errorf("Worker无法重新入队任务，请等待恢复服务处理")
}

// GetStatus 获取Worker状态
func (w *Worker) GetStatus() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return map[string]interface{}{
		"worker_id":     w.id,
		"status":        w.status,
		"task_count":    w.taskCount,
		"total_tasks":   w.totalTasks,
		"success_tasks": w.successTasks,
		"failed_tasks":  w.failedTasks,
		"concurrency":   w.config.Worker.Concurrency,
		"goroutines":    runtime.NumGoroutine(),
	}
}

// startGitSyncSubscriber 启动Git同步订阅服务
func (w *Worker) startGitSyncSubscriber() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		// 订阅Git同步通道（使用模式订阅）
		pubsub := w.queue.GetClient().PSubscribe(w.ctx, "git:sync:*")
		defer pubsub.Close()

		ch := pubsub.Channel()
		
		w.log.Info("Git同步订阅服务已启动")

		for {
			select {
			case <-w.ctx.Done():
				w.log.Info("Git同步订阅服务收到退出信号")
				return
			case msg := <-ch:
				if msg == nil {
					continue
				}

				// 解析消息
				var syncMsg types.GitSyncMessage
				if err := json.Unmarshal([]byte(msg.Payload), &syncMsg); err != nil {
					w.log.WithError(err).Error("解析Git同步消息失败")
					continue
				}

				// 异步处理同步任务
				go func(msg types.GitSyncMessage) {
					if err := w.gitSyncExecutor.ProcessGitSyncMessage(&msg, w.id); err != nil {
						w.log.WithError(err).WithFields(logrus.Fields{
							"repository_id": msg.RepositoryID,
							"tenant_id":     msg.TenantID,
							"action":        msg.Action,
						}).Error("处理Git同步任务失败")
					}
				}(syncMsg)
			}
		}
	}()
}


// startGitScanConsumer 启动Git扫描队列消费者
func (w *Worker) startGitScanConsumer() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		
		w.log.Info("Git扫描队列消费者已启动")
		
		for {
			select {
			case <-w.ctx.Done():
				w.log.Info("Git扫描队列消费者收到退出信号")
				return
			default:
				// 使用BRPOP阻塞等待扫描任务（抢占式）
				result, err := w.queue.GetClient().BRPop(w.ctx, 5*time.Second, "git:scan:queue").Result()
				if err != nil {
					if err == context.Canceled || err == context.DeadlineExceeded {
						return
					}
					// 超时是正常的，继续等待
					continue
				}
				
				if len(result) < 2 {
					continue
				}
				
				// result[0] 是队列名，result[1] 是消息内容
				msgData := result[1]
				
				// 解析消息
				var msg struct {
					TaskID       string `json:"task_id"`
					Action       string `json:"action"`
					RepositoryID uint   `json:"repository_id"`
					TenantID     uint   `json:"tenant_id"`
					Repository   struct {
						ID        uint   `json:"id"`
						Name      string `json:"name"`
						LocalPath string `json:"local_path"`
					} `json:"repository"`
				}
				
				if err := json.Unmarshal([]byte(msgData), &msg); err != nil {
					w.log.WithError(err).Error("解析扫描消息失败")
					continue
				}
				
				// 处理扫描任务
				w.log.WithFields(logrus.Fields{
					"task_id":       msg.TaskID,
					"repository_id": msg.RepositoryID,
					"tenant_id":     msg.TenantID,
					"action":        msg.Action,
				}).Info("收到Git扫描任务")
				
				if msg.Action == "scan" && msg.TaskID != "" {
					// 构建仓库路径
					repoPath := fmt.Sprintf("%s/%s", w.config.Git.RepoBaseDir, msg.Repository.LocalPath)
					
					// 准备结果key
					resultKey := fmt.Sprintf("scan:result:%s", msg.TaskID)
					
					// 执行增强扫描
					w.log.Info("执行增强扫描（包含文件树）")
					enhancedResult, err := w.gitSyncExecutor.ScanRepositoryEnhanced(
						repoPath, 
						msg.Repository.ID, 
						msg.Repository.Name,
					)
					
					if err != nil {
						w.log.WithError(err).Error("增强扫描失败")
						// 返回空结果
						emptyResult, _ := json.Marshal(&executor.EnhancedScanResult{
							Surveys: []executor.SurveyFile{},
						})
						w.queue.GetClient().Set(w.ctx, resultKey, string(emptyResult), 60*time.Second)
					} else {
						// 序列化结果
						resultData, err := json.Marshal(enhancedResult)
						if err != nil {
							w.log.WithError(err).Error("序列化增强扫描结果失败")
							emptyResult, _ := json.Marshal(&executor.EnhancedScanResult{
								Surveys: []executor.SurveyFile{},
							})
							w.queue.GetClient().Set(w.ctx, resultKey, string(emptyResult), 60*time.Second)
						} else {
							w.queue.GetClient().Set(w.ctx, resultKey, string(resultData), 60*time.Second)
							w.log.WithFields(logrus.Fields{
								"task_id":      msg.TaskID,
								"survey_count": len(enhancedResult.Surveys),
								"total_files":  enhancedResult.Stats.TotalFiles,
							}).Info("增强扫描完成，结果已存入Redis")
						}
					}
				}
			}
		}
	}()
}

// startGitRepoCleanup 启动Git仓库清理服务
// 现在使用固定路径，不再需要清理旧副本
func (w *Worker) startGitRepoCleanup() {
	// 不再需要清理功能
}

