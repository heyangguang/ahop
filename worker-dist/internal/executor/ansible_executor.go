package executor

import (
	"ahop-worker/internal/types"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"gorm.io/gorm"
)

// AnsibleExecutor Ansible执行器
type AnsibleExecutor struct {
	*BaseExecutor
	db *gorm.DB
}

// NewAnsibleExecutor 创建Ansible执行器
func NewAnsibleExecutor(db *gorm.DB) *AnsibleExecutor {
	return &AnsibleExecutor{
		BaseExecutor: NewBaseExecutor([]string{
			"collect", // 信息采集
		}),
		db: db,
	}
}

// Execute 执行Ansible任务
func (e *AnsibleExecutor) Execute(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
	result := &TaskResult{
		Details: make(map[string]interface{}),
		Logs:    make([]string, 0),
		Result:  make(map[string]interface{}),
	}

	// 记录开始日志
	e.LogMessage(onLog, "info", "system",
		fmt.Sprintf("开始执行Ansible任务: %s", taskCtx.TaskType), "", "")

	e.LogProgress(onProgress, 0, "任务开始")

	if taskCtx.TaskType == "collect" {
		return e.executeCollect(ctx, taskCtx, onProgress, onLog)
	}

	result.Success = false
	result.Error = fmt.Sprintf("不支持的任务类型: %s", taskCtx.TaskType)

	return result
}

// executeCollect 执行信息采集任务
func (e *AnsibleExecutor) executeCollect(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
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

	// 记录任务发起人和租户信息
	e.LogMessage(onLog, "info", "system",
		fmt.Sprintf("用户 %s（租户：%s）通过 %s 发起的采集任务开始执行",
			taskCtx.Username, taskCtx.TenantName, taskCtx.Source), "", "")

	e.LogMessage(onLog, "info", "system", fmt.Sprintf("准备采集 %d 台主机信息", len(hostInfoMap)), "", "")

	e.LogProgress(onProgress, 10, "解析任务参数完成")

	// 为每个主机生成inventory和执行setup
	allFacts := make(map[string]interface{})
	successCount := 0
	failedCount := 0

	hostList := make([]*types.HostInfo, 0, len(hostInfoMap))
	for _, hostInfo := range hostInfoMap {
		hostList = append(hostList, hostInfo)
	}

	for i, hostInfo := range hostList {
		progress := 10 + (i * 80 / len(hostList))
		e.LogProgress(onProgress, progress, fmt.Sprintf("正在采集主机 %s (%s) 的信息", hostInfo.Hostname, hostInfo.IP))

		// 记录开始采集日志
		e.LogMessage(onLog, "info", "ansible",
			fmt.Sprintf("开始采集主机 %s (%s) 的信息...", hostInfo.Hostname, hostInfo.IP),
			hostInfo.Hostname, "")

		// 生成单个主机的inventory
		inventoryPath, err := e.generateInventoryForHost(hostInfo)
		if err != nil {
			failedCount++
			hostKey := fmt.Sprintf("%s:%d", hostInfo.IP, hostInfo.Port)
			allFacts[hostKey] = map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}
			e.LogMessage(onLog, "error", "ansible",
				fmt.Sprintf("生成主机 %s (%s) 的inventory失败: %v", hostInfo.Hostname, hostInfo.IP, err),
				hostInfo.Hostname, "")
			continue
		}
		defer os.Remove(inventoryPath)

		// 创建带有task_id的context
		ctxWithTaskID := context.WithValue(ctx, "task_id", taskCtx.TaskID)

		// 执行setup模块
		facts, err := e.runAnsibleSetup(ctxWithTaskID, inventoryPath, hostInfo, onLog)
		hostKey := fmt.Sprintf("%s:%d", hostInfo.IP, hostInfo.Port)
		if err != nil {
			failedCount++
			// 创建详细的失败信息，与ping任务保持一致
			failureResult := map[string]interface{}{
				"host_id":   hostInfo.ID,
				"hostname":  hostInfo.Hostname,
				"ip":        hostInfo.IP,
				"port":      hostInfo.Port,
				"success":   false,
				"error":     err.Error(),
				"message":   fmt.Sprintf("ansible执行失败: %v", err),
				"output":    "", // ansible失败时通常没有正常输出
				"stderr":    "", // 简化处理，error已经包含错误信息
				"exit_code": -1, // 未知退出码
			}
			
			// 如果facts不为nil，说明虽然命令失败但有部分输出，保存这些信息
			if facts != nil {
				// 保存原始的facts信息，包括命令错误信息
				failureResult["ansible_facts"] = facts
				if commandError, exists := facts["command_error"]; exists {
					failureResult["command_error"] = commandError
				}
				if rawOutput, exists := facts["raw_output"]; exists {
					failureResult["output"] = rawOutput
				}
			}
			
			allFacts[hostKey] = failureResult
			// 记录失败日志
			e.LogMessage(onLog, "error", "ansible",
				fmt.Sprintf("主机 %s (%s) 信息采集失败: %v", hostInfo.Hostname, hostInfo.IP, err),
				hostInfo.Hostname, "")
		} else {
			successCount++
			e.LogMessage(onLog, "info", "ansible",
				fmt.Sprintf("主机 %s (%s) 信息采集成功", hostInfo.Hostname, hostInfo.IP),
				hostInfo.Hostname, "")

			// 创建详细的成功信息，包含ansible_facts
			successResult := map[string]interface{}{
				"host_id":   hostInfo.ID,
				"hostname":  hostInfo.Hostname,
				"ip":        hostInfo.IP,
				"port":      hostInfo.Port,
				"success":   true,
				"message":   "ansible执行成功",
				"output":    "", // 简化处理，主要数据在ansible_facts中
				"stderr":    "", // 成功时一般没有stderr
				"exit_code": 0,  // 成功退出码
			}
			
			// 检查 facts 是否包含 ansible_facts
			if factsValue, exists := facts["ansible_facts"]; exists {
				if ansibleFacts, ok := factsValue.(map[string]interface{}); ok && len(ansibleFacts) > 0 {
					successResult["ansible_facts"] = ansibleFacts
				} else {
					successResult["ansible_facts"] = facts
				}
			} else {
				// 否则 facts 就是 ansible_facts
				successResult["ansible_facts"] = facts
			}
			
			allFacts[hostKey] = successResult
		}
	}

	// 记录任务汇总信息
	e.LogMessage(onLog, "info", "ansible",
		fmt.Sprintf("信息采集任务完成: 成功%d个，失败%d个", successCount, failedCount),
		"", "")

	// 汇总结果 - 只在 Result 中存储，避免重复
	result.Result = map[string]interface{}{
		"hosts": allFacts,
		"summary": map[string]interface{}{
			"total":   len(hostInfoMap),
			"success": successCount,
			"failed":  failedCount,
		},
	}

	if successCount == 0 && len(hostInfoMap) > 0 {
		result.Success = false
		result.Error = "所有主机信息采集失败"
	}

	e.LogProgress(onProgress, 100, "信息采集完成")

	return result
}

// generateInventoryForHost 为单个主机生成inventory
func (e *AnsibleExecutor) generateInventoryForHost(hostInfo *types.HostInfo) (string, error) {
	// 构建基础inventory
	hostVars := map[string]interface{}{
		"ansible_host":            hostInfo.IP,
		"ansible_port":            hostInfo.Port,
		"ansible_ssh_common_args": "-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null",
		"ansible_connection":      "ssh",
	}

	// 根据凭证类型设置认证信息
	switch hostInfo.Credential.Type {
	case "password":
		hostVars["ansible_user"] = hostInfo.Credential.Username
		hostVars["ansible_password"] = hostInfo.Credential.Password

	case "ssh_key":
		// 保存私钥到临时文件
		keyFile, err := e.savePrivateKey(hostInfo.Credential.PrivateKey)
		if err != nil {
			return "", fmt.Errorf("保存私钥失败: %v", err)
		}

		hostVars["ansible_user"] = hostInfo.Credential.Username
		hostVars["ansible_ssh_private_key_file"] = keyFile

		// 如果有passphrase，也需要设置
		if hostInfo.Credential.Passphrase != "" {
			hostVars["ansible_ssh_pass"] = hostInfo.Credential.Passphrase
		}

	default:
		return "", fmt.Errorf("不支持的凭证类型: %s", hostInfo.Credential.Type)
	}

	inventory := map[string]interface{}{
		"all": map[string]interface{}{
			"hosts": map[string]interface{}{
				hostInfo.IP: hostVars,
			},
		},
	}

	// 写入临时文件
	data, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		return "", err
	}

	tmpFile, err := ioutil.TempFile("", "ansible-inventory-*.json")
	if err != nil {
		return "", err
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", err
	}

	tmpFile.Close()
	return tmpFile.Name(), nil
}

// runAnsibleSetup 运行ansible setup模块 - 重新实现，双数据流处理
func (e *AnsibleExecutor) runAnsibleSetup(ctx context.Context, inventoryPath string, hostInfo *types.HostInfo, onLog LogCallback) (map[string]interface{}, error) {
	// 创建实时日志记录器
	taskID, _ := ctx.Value("task_id").(string)
	realtimeLogger := NewRealtimeLogger(e.BaseExecutor.redisClient, taskID)

	cmd := exec.CommandContext(ctx, "ansible", "all", "-i", inventoryPath, "-m", "setup")
	cmd.Env = append(cmd.Env,
		"ANSIBLE_HOST_KEY_CHECKING=False",
		"ANSIBLE_SSH_RETRIES=2",
		"ANSIBLE_TIMEOUT=30",
	)

	// 执行命令并获取完整输出
	output, err := cmd.CombinedOutput()

	// 处理输出的两个数据流
	stdoutStr := string(output)

	// 1. 实时日志流 - 发送到WebSocket
	if realtimeLogger != nil && stdoutStr != "" {
		// 按行发送实时日志
		lines := strings.Split(stdoutStr, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				realtimeLogger.LogOutput("ansible", line, hostInfo.Hostname)
			}
		}
	}

	// 2. 数据流 - 解析JSON用于数据库更新
	result := make(map[string]interface{})
	
	// 记录命令执行状态和错误信息
	if err != nil {
		result["command_error"] = err.Error()
		result["command_success"] = false
		// 发送错误信息到WebSocket
		if realtimeLogger != nil {
			realtimeLogger.LogError("ansible", fmt.Sprintf("ansible命令执行失败: %v", err), hostInfo.Hostname)
		}
	} else {
		result["command_success"] = true
	}

	if stdoutStr != "" {
		// 保存原始输出供调试
		filename := fmt.Sprintf("/tmp/ansible_output_%s.json", hostInfo.Hostname)
		ioutil.WriteFile(filename, output, 0644)

		// 解析Ansible输出JSON
		startIdx := strings.Index(stdoutStr, "=> {")
		if startIdx != -1 {
			jsonStart := strings.Index(stdoutStr[startIdx:], "{")
			if jsonStart != -1 {
				fullJsonStart := startIdx + jsonStart
				jsonStr := stdoutStr[fullJsonStart:]

				// 直接解析JSON，不做过多清理
				if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
					// 如果直接解析失败，再试试清理
					cleanedJSON := strings.ReplaceAll(jsonStr, "\n", " ")
					cleanedJSON = strings.ReplaceAll(cleanedJSON, "\r", " ")
					cleanedJSON = strings.ReplaceAll(cleanedJSON, "\t", " ")
					cleanedJSON = strings.Join(strings.Fields(cleanedJSON), " ")

					if err := json.Unmarshal([]byte(cleanedJSON), &result); err != nil {
						result["raw_output"] = stdoutStr
						result["parse_error"] = err.Error()
					}
				}
			}
		} else {
			// 直接解析整个输出
			if err := json.Unmarshal(output, &result); err != nil {
				result["raw_output"] = stdoutStr
				result["parse_error"] = err.Error()
			}
		}
	}

	// 即使命令执行失败，也返回解析的结果（如果有的话）
	// 这样可以保存UNREACHABLE等有用信息到数据库
	if err != nil && len(result) <= 2 { // 只有command_error和command_success字段
		return nil, fmt.Errorf("ansible命令执行失败且无输出: %v", err)
	}
	
	return result, nil
}

// extractCompleteJSON 从给定位置提取完整的 JSON 对象
func (e *AnsibleExecutor) extractCompleteJSON(text string) string {
	if len(text) == 0 || text[0] != '{' {
		return text
	}

	braceCount := 0
	inString := false
	escaped := false

	for i, char := range text {
		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if !inString {
			if char == '{' {
				braceCount++
			} else if char == '}' {
				braceCount--
				if braceCount == 0 {
					// 找到匹配的 }，返回完整的 JSON
					return text[:i+1]
				}
			}
		}
	}

	// 如果没有找到匹配的 }，返回原始文本
	return text
}

// ValidateParams 验证参数
func (e *AnsibleExecutor) ValidateParams(params map[string]interface{}) error {
	if _, ok := params["hosts"]; !ok {
		return fmt.Errorf("缺少hosts参数")
	}
	return nil
}

// savePrivateKey 保存私钥到临时文件
func (e *AnsibleExecutor) savePrivateKey(privateKey string) (string, error) {
	// 创建临时文件
	tmpFile, err := ioutil.TempFile("", "ansible-ssh-key-*.pem")
	if err != nil {
		return "", fmt.Errorf("创建临时密钥文件失败: %v", err)
	}

	// 写入私钥内容
	if _, err := tmpFile.WriteString(privateKey); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("写入私钥失败: %v", err)
	}

	// 关闭文件
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("关闭密钥文件失败: %v", err)
	}

	// 设置文件权限为600（仅所有者可读写）
	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("设置密钥文件权限失败: %v", err)
	}

	return tmpFile.Name(), nil
}
