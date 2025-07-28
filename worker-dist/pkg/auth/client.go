package auth

import (
	"ahop-worker/internal/types"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// AuthClient Worker认证客户端
type AuthClient struct {
	masterURL string
	accessKey string
	secretKey string
	workerID  string
	client    *http.Client
}

// NewAuthClient 创建认证客户端
func NewAuthClient(masterURL, accessKey, secretKey string) *AuthClient {
	return &AuthClient{
		masterURL: masterURL,
		accessKey: accessKey,
		secretKey: secretKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AuthRequest 认证请求
type AuthRequest struct {
	AccessKey string `json:"access_key"`
	WorkerID  string `json:"worker_id"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"`
}

// AuthResponse 认证响应（AHOP标准格式）
type AuthResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
	SSLMode  string `json:"sslmode"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	Prefix   string `json:"prefix"`
}

// AuthResult 认证结果
type AuthResult struct {
	DatabaseConfig *DatabaseConfig
	RedisConfig    *RedisConfig
}

// Authenticate 向Master认证获取配置
func (c *AuthClient) Authenticate(workerID string) (*AuthResult, error) {
	// 保存workerID
	c.workerID = workerID
	
	// 1. 构造认证请求
	timestamp := time.Now().Unix()
	req := AuthRequest{
		AccessKey: c.accessKey,
		WorkerID:  workerID,
		Timestamp: timestamp,
	}

	// 2. 计算签名
	req.Signature = c.calculateSignature(req.AccessKey, req.WorkerID, req.Timestamp)

	// 3. 发送HTTP请求
	authURL := c.masterURL + "/api/v1/worker/auth"
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	httpReq, err := http.NewRequest("POST", authURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "AHOP-Worker/1.0")

	// 4. 执行请求
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送认证请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 5. 解析响应
	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, fmt.Errorf("解析认证响应失败: %v", err)
	}


	// 6. 检查认证结果
	if resp.StatusCode != http.StatusOK || authResp.Code != 200 {
		return nil, fmt.Errorf("认证失败: %s (HTTP %d, Code %d)", authResp.Message, resp.StatusCode, authResp.Code)
	}

	// 7. 转换配置
	result := &AuthResult{}
	
	// 提取数据库配置
	if databaseConfig, ok := authResp.Data["database_config"].(map[string]interface{}); ok {
		dbConfig, err := c.convertDatabaseConfig(databaseConfig)
		if err != nil {
			return nil, fmt.Errorf("解析数据库配置失败: %v", err)
		}
		result.DatabaseConfig = dbConfig
	} else {
		return nil, fmt.Errorf("响应中缺少database_config字段")
	}
	
	// 提取Redis配置
	if redisConfig, ok := authResp.Data["redis_config"].(map[string]interface{}); ok {
		redisConf, err := c.convertRedisConfig(redisConfig)
		if err != nil {
			return nil, fmt.Errorf("解析Redis配置失败: %v", err)
		}
		result.RedisConfig = redisConf
	} else {
		return nil, fmt.Errorf("响应中缺少redis_config字段")
	}

	return result, nil
}

// calculateSignature 计算请求签名
func (c *AuthClient) calculateSignature(accessKey, workerID string, timestamp int64) string {
	// 构造待签名字符串
	stringToSign := fmt.Sprintf("%s|%s|%d", accessKey, workerID, timestamp)

	// HMAC-SHA256签名
	h := hmac.New(sha256.New, []byte(c.secretKey))
	h.Write([]byte(stringToSign))

	return hex.EncodeToString(h.Sum(nil))
}

// convertDatabaseConfig 转换数据库配置
func (c *AuthClient) convertDatabaseConfig(configMap map[string]interface{}) (*DatabaseConfig, error) {
	config := &DatabaseConfig{}

	// 提取各字段
	if host, ok := configMap["host"].(string); ok {
		config.Host = host
	} else {
		return nil, fmt.Errorf("缺少host字段")
	}

	if port, ok := configMap["port"].(float64); ok {
		config.Port = int(port)
	} else {
		config.Port = 5432 // 默认PostgreSQL端口
	}

	if user, ok := configMap["user"].(string); ok {
		config.User = user
	} else {
		return nil, fmt.Errorf("缺少user字段")
	}

	if password, ok := configMap["password"].(string); ok {
		config.Password = password
	} else {
		return nil, fmt.Errorf("缺少password字段")
	}

	if dbname, ok := configMap["dbname"].(string); ok {
		config.DBName = dbname
	} else {
		return nil, fmt.Errorf("缺少dbname字段")
	}

	if sslmode, ok := configMap["sslmode"].(string); ok {
		config.SSLMode = sslmode
	} else {
		config.SSLMode = "disable" // 默认值
	}

	return config, nil
}

// convertRedisConfig 转换Redis配置
func (c *AuthClient) convertRedisConfig(configMap map[string]interface{}) (*RedisConfig, error) {
	config := &RedisConfig{}

	// 提取各字段
	if host, ok := configMap["host"].(string); ok {
		config.Host = host
	} else {
		return nil, fmt.Errorf("缺少host字段")
	}

	if port, ok := configMap["port"].(float64); ok {
		config.Port = int(port)
	} else {
		config.Port = 6379 // 默认Redis端口
	}

	if password, ok := configMap["password"].(string); ok {
		config.Password = password
	}

	if db, ok := configMap["db"].(float64); ok {
		config.DB = int(db)
	} else {
		config.DB = 0 // 默认数据库
	}

	if prefix, ok := configMap["prefix"].(string); ok {
		config.Prefix = prefix
	} else {
		config.Prefix = "ahop:queue" // 默认前缀
	}

	return config, nil
}

// GenerateWorkerID 生成唯一的Worker ID
func GenerateWorkerID() string {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	
	// 使用主机名+时间戳纳秒确保唯一性
	return fmt.Sprintf("worker-%s-%d", hostname, time.Now().UnixNano())
}

// DisconnectWorker 通知Master断开Worker连接
func (c *AuthClient) DisconnectWorker() error {
	// 构造请求
	url := fmt.Sprintf("%s/api/v1/worker/disconnect", c.masterURL)
	requestData := map[string]string{
		"worker_id": c.workerID,
	}
	
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("序列化请求数据失败: %v", err)
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	
	// 添加认证头
	timestamp := time.Now().Unix()
	signature := c.calculateSignature(c.accessKey, c.workerID, timestamp)
	req.Header.Set("X-Access-Key", c.accessKey)
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-Signature", signature)
	req.Header.Set("Content-Type", "application/json")
	
	// 执行请求
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	// 检查响应
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	return nil
}

// GetDecryptedCredential 获取解密后的凭证信息
func (c *AuthClient) GetDecryptedCredential(credentialID, tenantID uint) (*types.CredentialInfo, error) {
	// 构造请求URL
	url := fmt.Sprintf("%s/api/v1/worker/credentials/%d/decrypt?tenant_id=%d", c.masterURL, credentialID, tenantID)
	
	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}
	
	// 添加认证头
	timestamp := time.Now().Unix()
	signature := c.calculateSignature(c.accessKey, fmt.Sprintf("%d", credentialID), timestamp)
	req.Header.Set("X-Access-Key", c.accessKey)
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-Signature", signature)
	req.Header.Set("Content-Type", "application/json")
	
	// 执行请求
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	// 解析响应
	// 首先检查HTTP状态码
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("凭证不存在 (ID: %d)", credentialID)
	}
	
	var response struct {
		Code    int                   `json:"code"`
		Message string                `json:"message"`
		Data    types.CredentialInfo  `json:"data"`
	}
	
	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}
	
	// 如果状态码不是200，可能返回的是错误信息而不是标准格式
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取凭证失败: HTTP %d, 响应: %s", resp.StatusCode, string(body))
	}
	
	// 解析响应
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v, 原始响应: %s", err, string(body))
	}
	
	if response.Code != 200 {
		return nil, fmt.Errorf("获取凭证失败: %s (Code %d)", response.Message, response.Code)
	}
	
	return &response.Data, nil
}


// Request 发送通用API请求到Master
func (c *AuthClient) Request(method, endpoint string, body []byte) (*http.Response, error) {
	// 构造完整URL
	url := c.masterURL + endpoint
	
	// 创建请求
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}
	
	// 添加认证头
	timestamp := time.Now().Unix()
	signature := c.calculateSignature(c.accessKey, endpoint, timestamp)
	req.Header.Set("X-Access-Key", c.accessKey)
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-Signature", signature)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AHOP-Worker/1.0")
	
	// 执行请求
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	
	return resp, nil
}

// SendHeartbeat 发送心跳到Master
func (c *AuthClient) SendHeartbeat(workerID string) error {
	data := map[string]string{
		"worker_id": workerID,
	}
	
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	
	resp, err := c.Request("PUT", "/api/v1/worker/heartbeat", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed: HTTP %d", resp.StatusCode)
	}
	
	return nil
}

// InitializationData Worker初始化数据
type InitializationData struct {
	Repositories []map[string]interface{} `json:"repositories"`
	Templates    []map[string]interface{} `json:"templates"`
	Timestamp    int64                    `json:"timestamp"`
}

// GetInitializationData 获取Worker初始化数据
func (c *AuthClient) GetInitializationData() (*InitializationData, error) {
	resp, err := c.Request("GET", "/api/v1/worker/initialization", nil)
	if err != nil {
		return nil, fmt.Errorf("请求初始化数据失败: %v", err)
	}
	defer resp.Body.Close()
	
	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取初始化数据失败: HTTP %d, 响应: %s", resp.StatusCode, string(body))
	}
	
	// 解析标准响应格式
	var response struct {
		Code    int                `json:"code"`
		Message string             `json:"message"`
		Data    InitializationData `json:"data"`
	}
	
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}
	
	if response.Code != 200 {
		return nil, fmt.Errorf("获取初始化数据失败: %s (Code %d)", response.Message, response.Code)
	}
	
	return &response.Data, nil
}

