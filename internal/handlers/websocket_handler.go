package handlers

import (
	"ahop/internal/database"
	"ahop/internal/services"
	"ahop/pkg/config"
	"ahop/pkg/jwt"
	"ahop/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// WebSocketHandler WebSocket处理器
type WebSocketHandler struct {
	upgrader        websocket.Upgrader
	redisClient     *redis.Client
	log             *logrus.Logger
	jwtManager      *jwt.JWTManager
	userService     *services.UserService
	taskService     *services.TaskService
	networkScanSvc  *services.NetworkScanService
}

// NewWebSocketHandler 创建WebSocket处理器
func NewWebSocketHandler(userService *services.UserService, taskService *services.TaskService) *WebSocketHandler {
	// 获取CORS配置
	cfg := config.GetConfig()
	allowedOrigins := cfg.CORS.AllowOrigins
	
	return &WebSocketHandler{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// 检查Origin是否在允许列表中
				origin := r.Header.Get("Origin")
				
				// 如果允许所有源
				for _, allowed := range allowedOrigins {
					if allowed == "*" {
						return true
					}
				}
				
				// 如果Origin为空（同源请求），允许
				if origin == "" {
					return true
				}
				
				// 检查Origin是否在允许列表中
				for _, allowed := range allowedOrigins {
					if matchOrigin(origin, allowed) {
						return true
					}
				}
				
				// 记录被拒绝的Origin
				logger.GetLogger().Warnf("WebSocket连接被拒绝，非法Origin: %s", origin)
				return false
			},
			ReadBufferSize:  1024 * 32,  // 增加到32KB
			WriteBufferSize: 1024 * 32,  // 增加到32KB
		},
		redisClient:    database.GetRedisQueue().GetClient(),
		log:            logger.GetLogger(),
		jwtManager:     jwt.GetJWTManager(), // 使用全局JWT管理器
		userService:    userService,
		taskService:    taskService,
		networkScanSvc: services.NewNetworkScanService(),
	}
}

// TaskLogs 处理任务日志的WebSocket连接
func (h *WebSocketHandler) TaskLogs(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务ID不能为空"})
		return
	}

	// 从查询参数获取token（WebSocket不支持自定义header）
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少认证令牌"})
		return
	}

	// 验证token
	claims, err := h.jwtManager.VerifyToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的令牌"})
		return
	}

	// 验证用户是否有权限查看该任务的日志
	hasPermission, err := h.userService.HasPermission(claims.UserID, "task:logs")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "权限检查失败"})
		return
	}
	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足：需要 task:logs 权限"})
		return
	}

	// 验证任务是否属于用户的当前租户
	_, err = h.taskService.GetTask(taskID, claims.CurrentTenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在或无权访问"})
		return
	}

	// 升级为WebSocket连接
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.WithError(err).Error("Failed to upgrade WebSocket connection")
		return
	}
	defer conn.Close()

	h.log.WithFields(logrus.Fields{
		"task_id": taskID,
		"user_id": claims.UserID,
	}).Info("WebSocket connection established")

	// 处理连接
	h.handleTaskLogConnection(conn, taskID, claims)
}

// handleTaskLogConnection 处理任务日志连接
func (h *WebSocketHandler) handleTaskLogConnection(conn *websocket.Conn, taskID string, claims *jwt.JWTClaims) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 订阅Redis channel
	channel := fmt.Sprintf("task:logs:%s", taskID)
	pubsub := h.redisClient.Subscribe(ctx, channel)
	defer pubsub.Close()

	// 等待订阅成功
	_, err := pubsub.Receive(ctx)
	if err != nil {
		h.log.WithError(err).Error("Failed to subscribe to Redis channel")
		return
	}

	// 启动goroutine处理客户端消息（主要是ping/pong）
	go h.readPump(conn, cancel)

	// 获取消息channel
	ch := pubsub.Channel()

	// 设置写入超时
	const writeTimeout = 10 * time.Second

	// 创建心跳ticker - 每60秒发送一次ping
	pingTicker := time.NewTicker(60 * time.Second)
	defer pingTicker.Stop()

	// 跟踪最后接收消息的时间
	lastMessageTime := time.Now()
	// 在最后一条消息后等待的时间
	const gracePeriod = 5 * time.Second
	// 检查是否应该关闭连接的ticker
	graceTicker := time.NewTicker(1 * time.Second)
	defer graceTicker.Stop()

	// 循环接收并转发消息
	for {
		select {
		case <-ctx.Done():
			return

		case <-graceTicker.C:
			// 检查是否超过了宽限期
			if time.Since(lastMessageTime) > gracePeriod {
				// 检查任务是否已完成
				task, err := h.taskService.GetTask(taskID, claims.CurrentTenantID)
				if err == nil && (task.Status == "success" || task.Status == "failed") {
					h.log.WithField("task_id", taskID).Info("Task completed and grace period expired, closing WebSocket")
					return
				}
			}

		case <-pingTicker.C:
			// 发送ping消息保持连接
			conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				h.log.WithError(err).Error("Failed to send ping")
				return
			}

		case msg := <-ch:
			if msg == nil {
				continue // 不立即返回，等待宽限期
			}

			// 更新最后消息时间
			lastMessageTime = time.Now()

			// 设置写入超时
			conn.SetWriteDeadline(time.Now().Add(writeTimeout))

			// 解析消息并发送给客户端
			var logData map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &logData); err != nil {
				h.log.WithError(err).Error("Failed to parse log message")
				continue
			}

			// 发送给客户端
			if err := conn.WriteJSON(logData); err != nil {
				h.log.WithError(err).Error("Failed to send message to client")
				return
			}
		}
	}
}

// readPump 处理客户端消息
func (h *WebSocketHandler) readPump(conn *websocket.Conn, cancel context.CancelFunc) {
	defer cancel()

	// 设置读取超时 - 增加到5分钟
	pongWait := 300 * time.Second
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		// 读取消息（主要是处理ping/pong）
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.log.WithError(err).Error("WebSocket unexpected close")
			}
			break
		}
	}
}

// NetworkScanResults 处理网络扫描结果的WebSocket连接
func (h *WebSocketHandler) NetworkScanResults(c *gin.Context) {
	h.log.Info("NetworkScanResults handler called")
	
	scanID := c.Param("scan_id")
	h.log.WithField("scan_id", scanID).Info("Scan ID from params")
	
	if scanID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少扫描任务ID"})
		return
	}

	// 从查询参数获取token进行认证
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少认证令牌"})
		return
	}

	// 验证JWT token
	claims, err := h.jwtManager.VerifyToken(token)
	if err != nil {
		h.log.WithError(err).Error("JWT token validation failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证失败"})
		return
	}

	// 检查扫描任务是否存在且属于当前租户
	task, err := h.networkScanSvc.GetTaskStatus(scanID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "扫描任务不存在"})
		return
	}

	if task.TenantID != claims.CurrentTenantID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限访问此扫描任务"})
		return
	}

	// 升级为WebSocket连接
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.WithError(err).Error("WebSocket upgrade failed")
		return
	}
	defer conn.Close()

	// 设置连接信息
	h.log.WithFields(logrus.Fields{
		"user_id":    claims.UserID,
		"tenant_id":  claims.CurrentTenantID,
		"scan_id":    scanID,
		"remote_addr": c.ClientIP(),
	}).Info("Network scan WebSocket connection established")

	// 创建上下文用于取消操作
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动读取pump（处理客户端消息）
	go h.readPump(conn, cancel)

	// 创建Redis订阅频道
	channel := fmt.Sprintf("network_scan:%s", scanID)
	pubsub := h.redisClient.Subscribe(ctx, channel)
	defer pubsub.Close()

	// 设置写入超时
	writeTimeout := 10 * time.Second
	
	// 发送当前任务状态
	currentTask, err := h.networkScanSvc.GetTaskStatus(scanID)
	if err == nil {
		initialMessage := map[string]interface{}{
			"type": "status",
			"data": map[string]interface{}{
				"scan_id":   currentTask.ScanID,
				"status":    currentTask.Status,
				"progress":  currentTask.Progress,
				"start_time": currentTask.StartTime,
				"results":   len(currentTask.Results),
			},
		}
		conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if err := conn.WriteJSON(initialMessage); err != nil {
			h.log.WithError(err).Error("Failed to send initial status")
			return
		}
	}

	// 创建心跳ticker - 每60秒发送一次ping
	pingTicker := time.NewTicker(60 * time.Second)
	defer pingTicker.Stop()

	// 创建消息接收channel，避免阻塞
	msgChan := make(chan *redis.Message, 100)
	go func() {
		for {
			msg, err := pubsub.ReceiveMessage(ctx)
			if err != nil {
				if err == context.Canceled {
					return
				}
				h.log.WithError(err).Error("Failed to receive Redis message")
				continue
			}
			select {
			case msgChan <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	// 主循环
	for {
		select {
		case <-ctx.Done():
			return
			
		case <-pingTicker.C:
			// 发送ping消息保持连接
			conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				h.log.WithError(err).Error("Failed to send ping")
				return
			}
			
		case msg := <-msgChan:
			// 解析消息
			var scanMessage map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &scanMessage); err != nil {
				h.log.WithError(err).Error("Failed to parse scan message")
				continue
			}

			// 发送给客户端
			conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := conn.WriteJSON(scanMessage); err != nil {
				h.log.WithError(err).Error("Failed to send scan message to client")
				return
			}
		}
	}
}

// matchOrigin 检查origin是否匹配allowed模式
// 支持精确匹配和通配符匹配（如 *.example.com）
func matchOrigin(origin, allowed string) bool {
	// 精确匹配
	if origin == allowed {
		return true
	}
	
	// 检查是否是通配符模式
	if strings.HasPrefix(allowed, "*.") {
		// 获取域名部分（去掉 *.）
		domain := allowed[2:]
		
		// 处理origin中的协议部分
		// 例如：http://sub.example.com -> sub.example.com
		originHost := origin
		if idx := strings.Index(origin, "://"); idx != -1 {
			originHost = origin[idx+3:]
		}
		
		// 去掉端口号（如果有）
		if idx := strings.Index(originHost, ":"); idx != -1 {
			originHost = originHost[:idx]
		}
		
		// 检查是否匹配
		if originHost == domain {
			return true
		}
		
		// 检查是否是子域名
		if strings.HasSuffix(originHost, "."+domain) {
			return true
		}
	}
	
	return false
}
