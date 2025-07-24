package executor

import (
	"ahop-worker/internal/types"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// ShellExecutor Shell命令执行器
type ShellExecutor struct {
	*BaseExecutor
}

// NewShellExecutor 创建Shell执行器
func NewShellExecutor() *ShellExecutor {
	return &ShellExecutor{
		BaseExecutor: NewBaseExecutor([]string{
			"shell_command",  // 直接Shell命令执行
			"shell_script",   // Shell脚本执行
		}),
	}
}

// Execute 执行Shell任务
func (e *ShellExecutor) Execute(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
	result := &TaskResult{
		Details: make(map[string]interface{}),
		Logs:    make([]string, 0),
	}

	e.LogMessage(onLog, "info", "shell", fmt.Sprintf("开始执行Shell任务: %s", taskCtx.TaskType), "", "")
	e.LogProgress(onProgress, 0, "任务开始")

	switch taskCtx.TaskType {
	case "shell_command":
		return e.executeShellCommand(ctx, taskCtx, onProgress, onLog)
	case "shell_script":
		return e.executeShellScript(ctx, taskCtx, onProgress, onLog)
	default:
		result.Success = false
		result.Error = fmt.Sprintf("不支持的任务类型: %s", taskCtx.TaskType)
		return result
	}
}

// executeShellCommand 通过SSH执行Shell命令 - 统一数据结构和双数据流处理
func (e *ShellExecutor) executeShellCommand(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
	result := &TaskResult{
		Success: true,
		Details: make(map[string]interface{}),
		Logs:    make([]string, 0),
		Result:  make(map[string]interface{}),
	}

	// 获取主机信息映射
	hostInfoMapInterface, ok := taskCtx.Params["_host_info_map"]
	if !ok {
		result.Success = false
		result.Error = "缺少主机信息映射"
		return result
	}

	hostInfoMap, ok := hostInfoMapInterface.(map[uint]*types.HostInfo)
	if !ok {
		result.Success = false
		result.Error = "主机信息映射格式错误"
		return result
	}

	command, ok := taskCtx.Params["command"].(string)
	if !ok || command == "" {
		result.Success = false
		result.Error = "缺少command参数"
		return result
	}

	// 记录任务发起人和租户信息
	e.LogMessage(onLog, "info", "system",
		fmt.Sprintf("用户 %s（租户：%s）通过 %s 发起的shell任务开始执行",
			taskCtx.Username, taskCtx.TenantName, taskCtx.Source), "", "")

	e.LogMessage(onLog, "info", "system", fmt.Sprintf("准备在 %d 台主机执行命令: %s", len(hostInfoMap), command), "", "")
	e.LogProgress(onProgress, 10, "解析任务参数完成")

	// 执行结果
	allResults := make(map[string]interface{})
	successCount := 0
	failedCount := 0

	// 将hostInfoMap转为slice以便遍历
	hostList := make([]*types.HostInfo, 0, len(hostInfoMap))
	for _, hostInfo := range hostInfoMap {
		hostList = append(hostList, hostInfo)
	}

	// 在每台主机上执行命令
	for i, hostInfo := range hostList {
		progress := 10 + (i * 80 / len(hostList))
		e.LogProgress(onProgress, progress, fmt.Sprintf("正在执行主机 %s (%s)", hostInfo.Hostname, hostInfo.IP))

		// 记录开始执行日志
		e.LogMessage(onLog, "info", "shell",
			fmt.Sprintf("开始在主机 %s (%s) 执行命令...", hostInfo.Hostname, hostInfo.IP),
			hostInfo.Hostname, "")

		// 创建实时日志记录器
		var realtimeLogger *RealtimeLogger
		if e.redisClient != nil && taskCtx.TaskID != "" {
			realtimeLogger = NewRealtimeLogger(e.redisClient, taskCtx.TaskID)
		}

		// 执行SSH命令
		stdout, stderr, exitCode, err := e.executeSSHCommandDetailed(hostInfo.IP, hostInfo.Port, hostInfo.Credential, command, realtimeLogger, hostInfo.Hostname)
		
		hostKey := fmt.Sprintf("%s:%d", hostInfo.IP, hostInfo.Port)
		
		if err != nil {
			failedCount++
			// 创建详细的失败信息，与ping和ansible保持一致
			allResults[hostKey] = map[string]interface{}{
				"host_id":   hostInfo.ID,
				"hostname":  hostInfo.Hostname,
				"ip":        hostInfo.IP,
				"port":      hostInfo.Port,
				"success":   false,
				"error":     err.Error(),
				"message":   fmt.Sprintf("shell命令执行失败: %v", err),
				"output":    stdout,
				"stderr":    stderr,
				"exit_code": exitCode,
			}
			// 记录失败日志
			e.LogMessage(onLog, "error", "shell",
				fmt.Sprintf("主机 %s (%s) 命令执行失败: %v", hostInfo.Hostname, hostInfo.IP, err),
				hostInfo.Hostname, "")
		} else {
			successCount++
			// 创建详细的成功信息
			allResults[hostKey] = map[string]interface{}{
				"host_id":   hostInfo.ID,
				"hostname":  hostInfo.Hostname,
				"ip":        hostInfo.IP,
				"port":      hostInfo.Port,
				"success":   true,
				"message":   "shell命令执行成功",
				"output":    stdout,
				"stderr":    stderr,
				"exit_code": exitCode,
			}
			e.LogMessage(onLog, "info", "shell",
				fmt.Sprintf("主机 %s (%s) 命令执行成功", hostInfo.Hostname, hostInfo.IP),
				hostInfo.Hostname, "")
		}
	}

	// 记录任务汇总信息
	e.LogMessage(onLog, "info", "shell",
		fmt.Sprintf("shell任务完成: 成功%d个，失败%d个", successCount, failedCount),
		"", "")

	// 汇总结果 - 只在 Result 中存储，避免重复
	result.Result = map[string]interface{}{
		"hosts": allResults,
		"summary": map[string]interface{}{
			"total":   len(hostInfoMap),
			"success": successCount,
			"failed":  failedCount,
		},
	}

	if successCount == 0 && len(hostInfoMap) > 0 {
		result.Success = false
		result.Error = "所有主机shell命令执行失败"
	}

	e.LogProgress(onProgress, 100, "shell任务完成")

	return result
}

// executeSSHCommandDetailed 通过SSH执行命令，支持双数据流处理
func (e *ShellExecutor) executeSSHCommandDetailed(host string, port int, credential *types.CredentialInfo, command string, realtimeLogger *RealtimeLogger, hostname string) (stdout, stderr string, exitCode int, err error) {
	// 创建SSH配置
	config := &ssh.ClientConfig{
		User:            credential.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// 根据凭证类型设置认证方式
	switch credential.Type {
	case "password":
		config.Auth = append(config.Auth, ssh.Password(credential.Password))
	case "ssh_key":
		signer, parseErr := ssh.ParsePrivateKey([]byte(credential.PrivateKey))
		if parseErr != nil {
			err = fmt.Errorf("解析私钥失败: %v", parseErr)
			return
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
		// 如果有密码短语
		if credential.Passphrase != "" {
			config.Auth = append(config.Auth, ssh.Password(credential.Passphrase))
		}
	default:
		err = fmt.Errorf("不支持的凭证类型: %s", credential.Type)
		return
	}

	// 连接SSH
	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		err = fmt.Errorf("SSH连接失败: %v", err)
		// 发送连接失败信息到WebSocket
		if realtimeLogger != nil {
			realtimeLogger.LogError("shell", fmt.Sprintf("SSH连接失败: %v", err), hostname)
		}
		return
	}
	defer client.Close()

	// 创建会话
	session, err := client.NewSession()
	if err != nil {
		err = fmt.Errorf("创建SSH会话失败: %v", err)
		// 发送会话创建失败信息到WebSocket
		if realtimeLogger != nil {
			realtimeLogger.LogError("shell", fmt.Sprintf("创建SSH会话失败: %v", err), hostname)
		}
		return
	}
	defer session.Close()

	// 分别捕获stdout和stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	// 发送开始执行信息到WebSocket
	if realtimeLogger != nil {
		realtimeLogger.LogOutput("shell", fmt.Sprintf("执行命令: %s", command), hostname)
	}

	// 执行命令
	runErr := session.Run(command)
	
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	// 获取退出码
	exitCode = 0
	if runErr != nil {
		if exitError, ok := runErr.(*ssh.ExitError); ok {
			exitCode = exitError.ExitStatus()
		} else {
			exitCode = -1
		}
		err = fmt.Errorf("命令执行失败: %v", runErr)
	}

	// 双数据流处理 - 发送输出到WebSocket
	if realtimeLogger != nil {
		// 发送stdout
		if stdout != "" {
			lines := strings.Split(stdout, "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					realtimeLogger.LogOutput("shell", line, hostname)
				}
			}
		}
		
		// 发送stderr
		if stderr != "" {
			lines := strings.Split(stderr, "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					realtimeLogger.LogError("shell", line, hostname)
				}
			}
		}

		// 发送执行结果
		if err != nil {
			realtimeLogger.LogError("shell", fmt.Sprintf("主机 %s (%s) 命令执行失败 (退出码: %d): %v", hostname, host, exitCode, runErr), hostname)
		} else {
			realtimeLogger.LogOutput("shell", fmt.Sprintf("主机 %s (%s) 命令执行成功 (退出码: %d)", hostname, host, exitCode), hostname)
		}
	}

	return
}

// executeSSHCommand 通过SSH执行命令（向后兼容的旧方法）
func (e *ShellExecutor) executeSSHCommand(host string, port int, info map[string]interface{}, command string, onLog LogCallback) (string, error) {
	// 获取认证信息
	username, _ := info["username"].(string)
	password, _ := info["password"].(string)
	privateKey, _ := info["private_key"].(string)

	// 创建SSH配置
	config := &ssh.ClientConfig{
		User:            username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// 设置认证方式
	if password != "" {
		config.Auth = append(config.Auth, ssh.Password(password))
	}
	if privateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(privateKey))
		if err == nil {
			config.Auth = append(config.Auth, ssh.PublicKeys(signer))
		}
	}

	// 连接SSH
	addr := fmt.Sprintf("%s:%d", host, port)
	e.LogMessage(onLog, "debug", "shell", fmt.Sprintf("正在连接 %s@%s", username, addr), host, "")
	
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("SSH连接失败: %v", err)
	}
	defer client.Close()

	// 创建会话
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建SSH会话失败: %v", err)
	}
	defer session.Close()

	// 执行命令
	e.LogMessage(onLog, "debug", "shell", fmt.Sprintf("执行命令: %s", command), host, "")
	
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(command)
	
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\nSTDERR:\n" + stderr.String()
	}

	if err != nil {
		return output, fmt.Errorf("命令执行失败: %v", err)
	}

	return output, nil
}

// executeShellScript 执行Shell脚本 - 统一数据结构处理
func (e *ShellExecutor) executeShellScript(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
	result := &TaskResult{
		Success: true,
		Details: make(map[string]interface{}),
		Logs:    make([]string, 0),
		Result:  make(map[string]interface{}),
	}

	// 获取脚本内容
	script, ok := taskCtx.Params["script"].(string)
	if !ok || script == "" {
		result.Success = false
		result.Error = "缺少script参数"
		return result
	}

	// 本地执行脚本（用于测试）
	if localExec, ok := taskCtx.Params["local"].(bool); ok && localExec {
		e.LogMessage(onLog, "info", "system", "开始本地执行shell脚本", "", "")
		e.LogProgress(onProgress, 50, "本地执行脚本")
		
		cmd := exec.CommandContext(ctx, "bash", "-c", script)
		
		// 分别捕获stdout和stderr
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		
		err := cmd.Run()
		
		// 获取退出码
		exitCode := 0
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			} else {
				exitCode = -1
			}
		}
		
		e.LogProgress(onProgress, 100, "执行完成")
		
		// 统一结果格式
		localResult := map[string]interface{}{
			"host_id":   0,
			"hostname":  "localhost",
			"ip":        "127.0.0.1",
			"port":      0,
			"success":   err == nil,
			"output":    stdout.String(),
			"stderr":    stderr.String(),
			"exit_code": exitCode,
		}
		
		if err == nil {
			localResult["message"] = "本地脚本执行成功"
			e.LogMessage(onLog, "info", "shell", "本地脚本执行成功", "", "")
		} else {
			localResult["error"] = err.Error()
			localResult["message"] = fmt.Sprintf("本地脚本执行失败: %v", err)
			result.Success = false
			result.Error = fmt.Sprintf("本地脚本执行失败: %v", err)
			e.LogMessage(onLog, "error", "shell", fmt.Sprintf("本地脚本执行失败: %v", err), "", "")
		}
		
		result.Result = map[string]interface{}{
			"hosts": map[string]interface{}{
				"127.0.0.1:0": localResult,
			},
			"summary": map[string]interface{}{
				"total":   1,
				"success": func() int { if err == nil { return 1 } else { return 0 } }(),
				"failed":  func() int { if err != nil { return 1 } else { return 0 } }(),
			},
		}
		
		return result
	}

	// 远程执行（将脚本作为命令执行）
	taskCtx.Params["command"] = script
	return e.executeShellCommand(ctx, taskCtx, onProgress, onLog)
}

// Validate 验证任务参数 - 支持新的参数格式
func (e *ShellExecutor) Validate(params map[string]interface{}) error {
	taskType, ok := params["task_type"].(string)
	if !ok {
		return fmt.Errorf("缺少task_type参数")
	}

	switch taskType {
	case "shell_command":
		if _, ok := params["command"].(string); !ok {
			return fmt.Errorf("shell_command任务缺少command参数")
		}
	case "shell_script":
		if _, ok := params["script"].(string); !ok {
			return fmt.Errorf("shell_script任务缺少script参数")
		}
	}

	// 本地执行不需要主机
	if local, ok := params["local"].(bool); ok && local {
		return nil
	}

	// 检查主机参数 - 优先使用新格式
	if _, ok := params["_host_info_map"]; ok {
		return nil // 新格式，包含完整主机信息
	}

	// 向后兼容检查旧格式
	hasHosts := false
	if _, ok := params["hosts"]; ok {
		hasHosts = true
	}
	if _, ok := params["host_ids"]; ok {
		hasHosts = true
	}

	if !hasHosts {
		return fmt.Errorf("缺少主机列表参数（_host_info_map、hosts或host_ids）")
	}

	return nil
}