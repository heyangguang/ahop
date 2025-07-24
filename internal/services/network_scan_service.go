package services

import (
	"ahop/internal/database"
	"ahop/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// NetworkScanService 网络扫描服务
type NetworkScanService struct {
	// 核心组件
	parser *NetworkParser
	
	// 扫描器
	scanners map[string]ScannerInterface
	
	// Redis客户端用于任务存储和WebSocket推送
	redisClient *redis.Client
	
	// 默认配置
	defaultTimeout     time.Duration
	defaultConcurrency int
	
	// Redis键前缀
	taskKeyPrefix string
	// 任务TTL（30分钟）
	taskTTL time.Duration
}

// NewNetworkScanService 创建网络扫描服务
func NewNetworkScanService() *NetworkScanService {
	service := &NetworkScanService{
		parser:             NewNetworkParser(),
		scanners:           make(map[string]ScannerInterface),
		redisClient:        database.GetRedisQueue().GetClient(),
		defaultTimeout:     5 * time.Second,
		defaultConcurrency: 50,
		taskKeyPrefix:      "network_scan:task:",
		taskTTL:            30 * time.Minute, // 30分钟TTL
	}
	
	// 注册扫描器
	service.registerScanners()
	
	return service
}

// registerScanners 注册所有扫描器
func (s *NetworkScanService) registerScanners() {
	// 注册PING扫描器
	pingScanner := NewPingScanner(s.defaultTimeout)
	s.scanners[pingScanner.GetProtocol()] = pingScanner
	
	// 注册TCP扫描器
	tcpScanner := NewTCPScanner(s.defaultTimeout, nil) // 使用默认端口
	s.scanners[tcpScanner.GetProtocol()] = tcpScanner
	
	// 注册UDP扫描器
	udpScanner := NewUDPScanner(s.defaultTimeout, nil) // 使用默认端口
	s.scanners[udpScanner.GetProtocol()] = udpScanner
	
	// 注册ARP扫描器
	arpScanner := NewARPScanner(s.defaultTimeout)
	s.scanners[arpScanner.GetProtocol()] = arpScanner
}

// StartScan 启动网络扫描
func (s *NetworkScanService) StartScan(tenantID, userID uint, username string, config *models.ScanConfig) (*models.ScanTask, error) {
	// 验证配置
	if err := s.validateConfig(config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %v", err)
	}
	
	// 解析目标网络
	allIPs, err := s.parser.ParseNetworks(config.Networks)
	if err != nil {
		return nil, fmt.Errorf("解析目标网络失败: %v", err)
	}
	
	// 处理排除列表
	exclusions, err := s.parser.ParseExclusions(config.ExcludeIPs)
	if err != nil {
		return nil, fmt.Errorf("解析排除列表失败: %v", err)
	}
	
	// 过滤排除的IP
	targetIPs := s.parser.FilterExclusions(allIPs, exclusions)
	if len(targetIPs) == 0 {
		return nil, fmt.Errorf("过滤后没有有效的扫描目标")
	}
	
	// 创建扫描任务
	scanID := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())
	
	task := &models.ScanTask{
		ScanID:     scanID,
		TenantID:   tenantID,
		UserID:     userID,
		Username:   username,
		Config:     config,
		Status:     models.ScanStatusRunning,
		Results:    make([]*models.ScanResult, 0),
		Progress:   0,
		StartTime:  time.Now(),
		Context:    ctx,
		CancelFunc: cancel,
	}
	
	// 保存任务到Redis
	if err := s.saveTaskToRedis(task); err != nil {
		return nil, fmt.Errorf("保存任务到Redis失败: %v", err)
	}
	
	// 异步启动扫描（不阻塞）
	go s.executeScan(task, targetIPs)
	
	return task, nil
}

// executeScan 执行扫描任务
func (s *NetworkScanService) executeScan(task *models.ScanTask, targetIPs []net.IP) {
	defer func() {
		// 设置结束时间
		now := time.Now()
		task.EndTime = &now
		
		// 最终状态更新
		if task.GetStatus() == models.ScanStatusRunning {
			task.SetStatus(models.ScanStatusCompleted)
		}
		
		// 推送完成消息
		s.pushComplete(task)
		
		// 更新任务状态到Redis（任务完成状态会自动过期）
		if err := s.saveTaskToRedis(task); err != nil {
			// 记录错误但不影响主流程
		}
	}()
	
	// 设置默认并发数
	concurrency := s.defaultConcurrency
	if task.Config.Concurrency > 0 {
		concurrency = task.Config.Concurrency
	}
	
	// 创建信号量控制并发
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	
	totalTargets := len(targetIPs) * len(task.Config.Methods)
	completed := 0
	var completedMu sync.Mutex
	
	// 推送初始进度
	s.pushProgress(task, 0, "开始扫描...", totalTargets, 0, 0)
	
	// 遍历所有IP和扫描方法
	for _, ip := range targetIPs {
		for _, method := range task.Config.Methods {
			// 检查是否被取消
			select {
			case <-task.Context.Done():
				task.SetStatus(models.ScanStatusCancelled)
				wg.Wait() // 等待已启动的goroutine完成
				return
			default:
			}
			
			// 获取扫描器
			scanner, exists := s.scanners[method]
			if !exists {
				completedMu.Lock()
				completed++
				progress := (completed * 100) / totalTargets
				completedMu.Unlock()
				
				task.UpdateProgress(progress)
				continue
			}
			
			// 获取信号量
			semaphore <- struct{}{}
			wg.Add(1)
			
			go func(targetIP net.IP, scanMethod string, scanner ScannerInterface) {
				defer func() {
					<-semaphore // 释放信号量
					wg.Done()
				}()
				
				// 执行扫描
				results := scanner.Scan(task.Context, targetIP)
				
				// 添加结果到任务
				for _, result := range results {
					task.AddResult(result)
					
					// 如果发现存活主机，立即推送结果
					if result.Status == models.ScanResultStatusAlive {
						s.pushResult(task, targetIP.String(), results)
					}
				}
				
				// 定期保存任务状态到Redis（每10个目标保存一次）
				if completed%10 == 0 || completed == totalTargets {
					if err := s.saveTaskToRedis(task); err != nil {
						// 记录错误但不影响扫描流程
					}
				}
				
				// 更新进度
				completedMu.Lock()
				completed++
				progress := (completed * 100) / totalTargets
				found := task.GetAliveCount()
				completedMu.Unlock()
				
				task.UpdateProgress(progress)
				s.pushProgress(task, progress, fmt.Sprintf("已扫描 %s", targetIP.String()), totalTargets, completed, found)
				
			}(ip, method, scanner)
		}
	}
	
	// 等待所有扫描完成
	wg.Wait()
	
	// 最终进度更新
	found := task.GetAliveCount()
	task.UpdateProgress(100)
	s.pushProgress(task, 100, "扫描完成", totalTargets, totalTargets, found)
}

// validateConfig 验证扫描配置
func (s *NetworkScanService) validateConfig(config *models.ScanConfig) error {
	if len(config.Networks) == 0 {
		return fmt.Errorf("必须指定至少一个目标网络")
	}
	
	if len(config.Methods) == 0 {
		return fmt.Errorf("必须指定至少一种扫描方法")
	}
	
	// 验证扫描方法
	if err := s.parser.ValidateMethods(config.Methods); err != nil {
		return err
	}
	
	// 验证端口
	if err := s.parser.ValidatePorts(config.Ports); err != nil {
		return err
	}
	
	// 验证超时
	if config.Timeout <= 0 {
		config.Timeout = 5 // 默认5秒
	}
	if config.Timeout > 60 {
		return fmt.Errorf("超时时间不能超过60秒")
	}
	
	// 验证并发数
	if config.Concurrency < 0 {
		config.Concurrency = s.defaultConcurrency
	}
	if config.Concurrency > 1000 {
		return fmt.Errorf("并发数不能超过1000")
	}
	
	return nil
}

// GetScanTask 获取扫描任务
func (s *NetworkScanService) GetScanTask(scanID string) (*models.ScanTask, bool) {
	task, err := s.getTaskFromRedis(scanID)
	if err != nil {
		return nil, false
	}
	return task, true
}

// CancelScan 取消扫描任务
func (s *NetworkScanService) CancelScan(scanID string) error {
	task, exists := s.GetScanTask(scanID)
	if !exists {
		return fmt.Errorf("scan task not found: %s", scanID)
	}
	
	// 取消上下文
	task.CancelFunc()
	task.SetStatus(models.ScanStatusCancelled)
	
	return nil
}

// GetActiveTasks 获取所有活跃任务
func (s *NetworkScanService) GetActiveTasks() []*models.ScanTask {
	var tasks []*models.ScanTask
	
	// 从Redis获取所有任务键
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	pattern := s.taskKeyPrefix + "*"
	keys, err := s.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return tasks // 返回空列表
	}
	
	// 获取每个任务的详情
	for _, key := range keys {
		scanID := strings.TrimPrefix(key, s.taskKeyPrefix)
		if task, err := s.getTaskFromRedis(scanID); err == nil {
			tasks = append(tasks, task)
		}
	}
	
	return tasks
}

// pushProgress 推送进度信息（WebSocket）
func (s *NetworkScanService) pushProgress(task *models.ScanTask, progress int, message string, total, scanned, found int) {
	progressData := &models.ProgressData{
		Progress: progress,
		Message:  message,
		Total:    total,
		Scanned:  scanned,
		Found:    found,
	}
	
	wsMessage := &models.WSMessage{
		Type:   models.WSMessageTypeProgress,
		Data:   progressData,
		ScanID: task.ScanID,
	}
	
	s.publishMessage(task.ScanID, wsMessage)
}

// pushResult 推送扫描结果（WebSocket）
func (s *NetworkScanService) pushResult(task *models.ScanTask, ip string, results []*models.ScanResult) {
	resultData := &models.ResultData{
		IP:      ip,
		Results: results,
	}
	
	wsMessage := &models.WSMessage{
		Type:   models.WSMessageTypeResult,
		Data:   resultData,
		ScanID: task.ScanID,
	}
	
	s.publishMessage(task.ScanID, wsMessage)
}

// pushComplete 推送完成信息（WebSocket）
func (s *NetworkScanService) pushComplete(task *models.ScanTask) {
	duration := "未知"
	if task.EndTime != nil {
		duration = task.EndTime.Sub(task.StartTime).String()
	}
	
	completeData := &models.CompleteData{
		ScanID:     task.ScanID,
		TotalFound: task.GetAliveCount(),
		Duration:   duration,
		Results:    task.GetResults(),
	}
	
	wsMessage := &models.WSMessage{
		Type:   models.WSMessageTypeComplete,
		Data:   completeData,
		ScanID: task.ScanID,
	}
	
	s.publishMessage(task.ScanID, wsMessage)
}

// pushError 推送错误信息（WebSocket）
func (s *NetworkScanService) pushError(task *models.ScanTask, message, errorStr string) {
	errorData := &models.ErrorData{
		ScanID:  task.ScanID,
		Message: message,
		Error:   errorStr,
	}
	
	wsMessage := &models.WSMessage{
		Type:   models.WSMessageTypeError,
		Data:   errorData,
		ScanID: task.ScanID,
	}
	
	s.publishMessage(task.ScanID, wsMessage)
}

// EstimateTargetCount 估算目标数量
func (s *NetworkScanService) EstimateTargetCount(config *models.ScanConfig) (int, error) {
	return s.parser.GetTargetCount(config.Networks, config.Methods, config.Ports)
}

// GetTaskStatus 获取任务状态快照
func (s *NetworkScanService) GetTaskStatus(scanID string) (*models.ScanTask, error) {
	task, exists := s.GetScanTask(scanID)
	if !exists {
		return nil, fmt.Errorf("scan task not found: %s", scanID)
	}
	
	// 返回任务状态的快照（避免并发问题）
	snapshot := &models.ScanTask{
		ScanID:    task.ScanID,
		TenantID:  task.TenantID,
		UserID:    task.UserID,
		Username:  task.Username,
		Config:    task.Config,
		Status:    task.GetStatus(),
		Results:   task.GetResults(),
		Progress:  task.GetProgress(),
		StartTime: task.StartTime,
		EndTime:   task.EndTime,
		Error:     task.GetError(),
	}
	
	return snapshot, nil
}

// publishMessage 发布消息到Redis频道用于WebSocket推送
func (s *NetworkScanService) publishMessage(scanID string, message *models.WSMessage) {
	if s.redisClient == nil {
		return // Redis不可用时静默失败
	}
	
	// 序列化消息
	data, err := json.Marshal(message)
	if err != nil {
		// 序列化失败，记录错误但不影响主流程
		return
	}
	
	// 发布到Redis频道
	channel := fmt.Sprintf("network_scan:%s", scanID)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	
	s.redisClient.Publish(ctx, channel, data)
	// 忽略发布错误，不影响扫描主流程
}

// saveTaskToRedis 保存任务到Redis
func (s *NetworkScanService) saveTaskToRedis(task *models.ScanTask) error {
	if s.redisClient == nil {
		return fmt.Errorf("Redis客户端不可用")
	}
	
	// 创建任务的可序列化副本（排除不可序列化的字段）
	taskData := struct {
		ScanID    string              `json:"scan_id"`
		TenantID  uint               `json:"tenant_id"`
		UserID    uint               `json:"user_id"`
		Username  string             `json:"username"`
		Config    *models.ScanConfig `json:"config"`
		Status    string             `json:"status"`
		Results   []*models.ScanResult `json:"results"`
		Progress  int                `json:"progress"`
		StartTime time.Time          `json:"start_time"`
		EndTime   *time.Time         `json:"end_time,omitempty"`
		Error     string             `json:"error,omitempty"`
	}{
		ScanID:    task.ScanID,
		TenantID:  task.TenantID,
		UserID:    task.UserID,
		Username:  task.Username,
		Config:    task.Config,
		Status:    task.GetStatus(),
		Results:   task.GetResults(),
		Progress:  task.GetProgress(),
		StartTime: task.StartTime,
		EndTime:   task.EndTime,
		Error:     task.GetError(),
	}
	
	// 序列化任务数据
	data, err := json.Marshal(taskData)
	if err != nil {
		return fmt.Errorf("序列化任务失败: %v", err)
	}
	
	// 保存到Redis并设置TTL
	key := s.taskKeyPrefix + task.ScanID
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = s.redisClient.Set(ctx, key, data, s.taskTTL).Err()
	if err != nil {
		return fmt.Errorf("保存任务到Redis失败: %v", err)
	}
	
	return nil
}

// getTaskFromRedis 从Redis获取任务
func (s *NetworkScanService) getTaskFromRedis(scanID string) (*models.ScanTask, error) {
	if s.redisClient == nil {
		return nil, fmt.Errorf("Redis客户端不可用")
	}
	
	// 从Redis获取任务数据
	key := s.taskKeyPrefix + scanID
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	
	data, err := s.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("从Redis获取任务失败: %v", err)
	}
	
	// 反序列化任务数据
	var taskData struct {
		ScanID    string              `json:"scan_id"`
		TenantID  uint               `json:"tenant_id"`
		UserID    uint               `json:"user_id"`
		Username  string             `json:"username"`
		Config    *models.ScanConfig `json:"config"`
		Status    string             `json:"status"`
		Results   []*models.ScanResult `json:"results"`
		Progress  int                `json:"progress"`
		StartTime time.Time          `json:"start_time"`
		EndTime   *time.Time         `json:"end_time,omitempty"`
		Error     string             `json:"error,omitempty"`
	}
	
	if err := json.Unmarshal([]byte(data), &taskData); err != nil {
		return nil, fmt.Errorf("反序列化任务失败: %v", err)
	}
	
	// 重建ScanTask对象
	task := &models.ScanTask{
		ScanID:    taskData.ScanID,
		TenantID:  taskData.TenantID,
		UserID:    taskData.UserID,
		Username:  taskData.Username,
		Config:    taskData.Config,
		Status:    taskData.Status,
		Results:   taskData.Results,
		Progress:  taskData.Progress,
		StartTime: taskData.StartTime,
		EndTime:   taskData.EndTime,
		Error:     taskData.Error,
		// Context 和 CancelFunc 不会被恢复，因为它们对已完成的任务不重要
		Context:    context.Background(),
		CancelFunc: func() {}, // 空函数，避免panic
	}
	
	return task, nil
}