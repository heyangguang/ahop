package connector

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHConnector SSH连接器
type SSHConnector struct {
	Host     string
	Port     int
	Username string
	Password string
	KeyData  string
	Timeout  time.Duration
}

// TestResult 连接测试结果
type TestResult struct {
	Success  bool                   `json:"success"`
	Message  string                 `json:"message"`
	Duration time.Duration          `json:"duration"`
	Error    string                 `json:"error,omitempty"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

// NewSSHConnector 创建SSH连接器
func NewSSHConnector(host string, port int, username, password, keyData string) *SSHConnector {
	return &SSHConnector{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		KeyData:  keyData,
		Timeout:  30 * time.Second,
	}
}

// TestConnection 测试SSH连接
func (c *SSHConnector) TestConnection() *TestResult {
	start := time.Now()
	result := &TestResult{
		Details: make(map[string]interface{}),
	}

	// 1. 测试网络连通性（TCP连接）
	address := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	conn, err := net.DialTimeout("tcp", address, c.Timeout)
	if err != nil {
		result.Success = false
		result.Message = "网络连接失败"
		result.Error = err.Error()
		result.Duration = time.Since(start)
		result.Details["step"] = "tcp_connection"
		return result
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			// 日志记录关闭错误，但不影响主要逻辑
		}
	}()

	// 2. 测试SSH连接
	config := &ssh.ClientConfig{
		User:            c.Username,
		Timeout:         c.Timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 注意：生产环境应该验证主机密钥
	}

	// 根据凭证类型设置认证方式
	if c.KeyData != "" {
		// 使用私钥认证
		signer, err := ssh.ParsePrivateKey([]byte(c.KeyData))
		if err != nil {
			result.Success = false
			result.Message = "私钥解析失败"
			result.Error = err.Error()
			result.Duration = time.Since(start)
			result.Details["step"] = "key_parsing"
			return result
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	} else if c.Password != "" {
		// 使用密码认证
		config.Auth = []ssh.AuthMethod{ssh.Password(c.Password)}
	} else {
		result.Success = false
		result.Message = "未提供认证信息"
		result.Error = "需要密码或私钥"
		result.Duration = time.Since(start)
		result.Details["step"] = "auth_config"
		return result
	}

	// 建立SSH连接
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		result.Success = false
		result.Message = "SSH认证失败"
		result.Error = err.Error()
		result.Duration = time.Since(start)
		result.Details["step"] = "ssh_auth"
		return result
	}
	defer client.Close()

	// 3. 执行简单测试命令
	session, err := client.NewSession()
	if err != nil {
		result.Success = false
		result.Message = "创建SSH会话失败"
		result.Error = err.Error()
		result.Duration = time.Since(start)
		result.Details["step"] = "ssh_session"
		return result
	}
	defer session.Close()

	// 执行 echo 命令测试
	output, err := session.Output("echo 'SSH connection test successful'")
	if err != nil {
		result.Success = false
		result.Message = "命令执行失败"
		result.Error = err.Error()
		result.Duration = time.Since(start)
		result.Details["step"] = "command_execution"
		return result
	}

	// 连接测试成功
	result.Success = true
	result.Message = "SSH连接测试成功"
	result.Duration = time.Since(start)
	result.Details["step"] = "completed"
	result.Details["output"] = string(output)
	result.Details["server_version"] = string(client.ServerVersion())

	return result
}

// TestPing 简单的网络ping测试
func (c *SSHConnector) TestPing() *TestResult {
	start := time.Now()
	result := &TestResult{
		Details: make(map[string]interface{}),
	}

	address := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("端口 %d 连接失败", c.Port)
		result.Error = err.Error()
		result.Duration = time.Since(start)
		return result
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			// 日志记录关闭错误，但不影响主要逻辑
		}
	}()

	result.Success = true
	result.Message = fmt.Sprintf("端口 %d 连接成功", c.Port)
	result.Duration = time.Since(start)
	result.Details["remote_addr"] = conn.RemoteAddr().String()

	return result
}
