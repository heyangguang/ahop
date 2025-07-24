package executor

import (
	"ahop-worker/internal/types"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// PingExecutor ping执行器
type PingExecutor struct {
	*BaseExecutor
}

// NewPingExecutor 创建ping执行器
func NewPingExecutor() *PingExecutor {
	return &PingExecutor{
		BaseExecutor: NewBaseExecutor([]string{
			"ping", // 网络ping任务
		}),
	}
}

// Execute 执行ping任务
func (e *PingExecutor) Execute(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
	result := &TaskResult{
		Success: true,
		Details: make(map[string]interface{}),
		Logs:    []string{},
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

	e.LogProgress(onProgress, 10, "开始执行ping任务")
	
	// 记录任务发起人和租户信息
	e.LogMessage(onLog, "info", "system", 
		fmt.Sprintf("用户 %s（租户：%s）通过 %s 发起的ping任务开始执行", 
			taskCtx.Username, taskCtx.TenantName, taskCtx.Source), "", "")
	
	e.LogMessage(onLog, "info", "system", fmt.Sprintf("准备ping %d 台主机", len(hostInfoMap)), "", "")

	// ping每个主机
	hostResults := make(map[string]interface{})
	successCount := 0
	failedCount := 0

	for hostID, hostInfo := range hostInfoMap {
		// 记录开始ping主机
		e.LogMessage(onLog, "info", "ping", 
			fmt.Sprintf("开始ping主机 %s (%s)...", hostInfo.Hostname, hostInfo.IP), 
			hostInfo.Hostname, "")
		
		// 执行ping命令（发送3个包）
		cmd := exec.Command("ping", "-c", "3", "-W", "3", hostInfo.IP)
		
		// 分别捕获stdout和stderr
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		
		// 创建实时日志记录器（如果有Redis客户端）
		var realtimeLogger *RealtimeLogger
		if e.redisClient != nil && taskCtx.TaskID != "" {
			realtimeLogger = NewRealtimeLogger(e.redisClient, taskCtx.TaskID)
		}
		
		err := cmd.Run()
		output := stdout.Bytes()
		stderrStr := stderr.String()
		
		// 如果有实时日志记录器，发送完整输出
		if realtimeLogger != nil && len(output) > 0 {
			// 发送ping输出的关键行
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && (strings.Contains(line, "bytes from") || strings.Contains(line, "min/avg/max")) {
					realtimeLogger.LogOutput("ping", line, hostInfo.Hostname)
				}
			}
		}
		
		// 获取退出码
		exitCode := 0
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}
		
		hostResult := map[string]interface{}{
			"host_id":   hostID,
			"ip":        hostInfo.IP,
			"port":      hostInfo.Port,
			"hostname":  hostInfo.Hostname,
			"success":   err == nil,
			"output":    string(output),
			"stderr":    stderrStr,
			"exit_code": exitCode,
		}

		if err == nil {
			// 解析ping输出，提取关键信息
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "min/avg/max") {
					hostResult["stats"] = line
					break
				}
			}
			hostResult["message"] = "ping成功"
			successCount++
			// 记录成功信息，将stats信息放在message中
			var statsMsg string
			if stats, ok := hostResult["stats"]; ok {
				statsMsg = fmt.Sprintf(", %s", stats)
			}
			e.LogMessage(onLog, "info", "ping", 
				fmt.Sprintf("主机 %s (%s) ping成功%s", hostInfo.Hostname, hostInfo.IP, statsMsg), 
				hostInfo.Hostname, "")
		} else {
			hostResult["message"] = fmt.Sprintf("ping失败: %v", err)
			hostResult["error"] = err.Error()
			failedCount++
			
			// 发送错误信息到WebSocket
			if realtimeLogger != nil {
				// 清理stderr内容并发送
				cleanedStderr := strings.TrimSpace(stderrStr)
				if cleanedStderr != "" {
					realtimeLogger.LogError("ping", cleanedStderr, hostInfo.Hostname)
				}
				// 发送失败总结
				realtimeLogger.LogError("ping", fmt.Sprintf("主机 %s (%s) ping失败: %v", hostInfo.Hostname, hostInfo.IP, err), hostInfo.Hostname)
			}
		}

		// 使用 "IP:端口" 作为结果映射的key，与主机信息更新保持一致
		hostKey := fmt.Sprintf("%s:%d", hostInfo.IP, hostInfo.Port)
		hostResults[hostKey] = hostResult
	}

	// 汇总结果 - 只在 Result 中存储，避免重复
	result.Result = map[string]interface{}{
		"hosts": hostResults,
		"summary": map[string]interface{}{
			"total":   len(hostInfoMap),
			"success": successCount,
			"failed":  failedCount,
		},
	}

	// 如果所有主机都失败，则任务失败
	if successCount == 0 && len(hostInfoMap) > 0 {
		result.Success = false
		result.Error = "所有主机ping失败"
	}

	e.LogProgress(onProgress, 100, "ping任务完成")
	
	// 记录汇总信息
	e.LogMessage(onLog, "info", "ping", 
		fmt.Sprintf("ping任务完成: 成功%d个，失败%d个", successCount, failedCount), 
		"", "")
	
	return result
}


