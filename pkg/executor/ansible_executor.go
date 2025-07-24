package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

// AnsibleExecutor Ansible执行器
type AnsibleExecutor struct {
	*BaseExecutor
}

// NewAnsibleExecutor 创建Ansible执行器
func NewAnsibleExecutor() *AnsibleExecutor {
	return &AnsibleExecutor{
		BaseExecutor: NewBaseExecutor([]string{
			"host_facts",    // 主机信息采集
			"host_ping",     // 主机连通性测试
			"ansible_adhoc", // Ansible Ad-hoc命令
			"ansible_setup", // Ansible setup模块
		}),
	}
}

// Execute 执行Ansible任务
func (e *AnsibleExecutor) Execute(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
	result := &TaskResult{
		Details: make(map[string]interface{}),
		Logs:    make([]string, 0),
	}

	e.LogMessage(onLog, "info", "ansible", fmt.Sprintf("开始执行Ansible任务: %s", taskCtx.TaskType), "", nil)
	e.LogProgress(onProgress, 0, "任务开始")

	switch taskCtx.TaskType {
	case "host_facts":
		return e.executeHostFacts(ctx, taskCtx, onProgress, onLog)
	case "host_ping":
		return e.executeHostPing(ctx, taskCtx, onProgress, onLog)
	case "ansible_adhoc":
		return e.executeAdhocCommand(ctx, taskCtx, onProgress, onLog)
	case "ansible_setup":
		return e.executeSetup(ctx, taskCtx, onProgress, onLog)
	default:
		result.Success = false
		result.Error = fmt.Sprintf("不支持的任务类型: %s", taskCtx.TaskType)
		return result
	}
}

// executeHostFacts 执行主机信息采集
func (e *AnsibleExecutor) executeHostFacts(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
	result := &TaskResult{
		Details: make(map[string]interface{}),
		Logs:    make([]string, 0),
	}

	// 解析参数
	hosts, ok := taskCtx.Params["hosts"].([]interface{})
	if !ok || len(hosts) == 0 {
		result.Success = false
		result.Error = "缺少主机列表参数"
		return result
	}

	hostInfo, ok := taskCtx.Params["host_info"].(map[string]interface{})
	if !ok {
		result.Success = false
		result.Error = "缺少主机连接信息"
		return result
	}

	e.LogProgress(onProgress, 10, "解析任务参数完成")
	e.LogMessage(onLog, "info", "ansible", fmt.Sprintf("准备采集 %d 台主机信息", len(hosts)), "", nil)

	// 生成inventory文件
	inventoryPath, err := e.generateInventory(hosts, hostInfo)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("生成inventory失败: %v", err)
		return result
	}
	defer os.Remove(inventoryPath)

	e.LogProgress(onProgress, 20, "inventory文件生成完成")

	// 执行setup模块
	facts, err := e.runAnsibleSetup(ctx, inventoryPath, onProgress, onLog)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("执行setup模块失败: %v", err)
		return result
	}

	result.Success = true
	result.Result = facts
	result.Details["hosts_count"] = len(hosts)
	result.Details["facts_collected"] = len(facts)

	e.LogProgress(onProgress, 100, "主机信息采集完成")
	e.LogMessage(onLog, "info", "ansible", "主机信息采集任务执行成功", "", result.Details)

	return result
}

// executeHostPing 执行主机连通性测试
func (e *AnsibleExecutor) executeHostPing(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
	result := &TaskResult{
		Details: make(map[string]interface{}),
		Logs:    make([]string, 0),
	}

	// 解析参数（与host_facts类似）
	hosts, ok := taskCtx.Params["hosts"].([]interface{})
	if !ok || len(hosts) == 0 {
		result.Success = false
		result.Error = "缺少主机列表参数"
		return result
	}

	hostInfo, ok := taskCtx.Params["host_info"].(map[string]interface{})
	if !ok {
		result.Success = false
		result.Error = "缺少主机连接信息"
		return result
	}

	e.LogProgress(onProgress, 10, "开始ping测试")

	// 生成inventory
	inventoryPath, err := e.generateInventory(hosts, hostInfo)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("生成inventory失败: %v", err)
		return result
	}
	defer os.Remove(inventoryPath)

	// 执行ping模块
	pingResults, err := e.runAnsiblePing(ctx, inventoryPath, onProgress, onLog)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("执行ping模块失败: %v", err)
		return result
	}

	result.Success = true
	result.Result = pingResults
	result.Details["hosts_count"] = len(hosts)

	e.LogProgress(onProgress, 100, "ping测试完成")
	return result
}

// executeAdhocCommand 执行Ad-hoc命令
func (e *AnsibleExecutor) executeAdhocCommand(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
	result := &TaskResult{
		Details: make(map[string]interface{}),
		Logs:    make([]string, 0),
	}

	// 解析参数
	module, ok := taskCtx.Params["module"].(string)
	if !ok {
		result.Success = false
		result.Error = "缺少module参数"
		return result
	}

	args, _ := taskCtx.Params["args"].(string)
	hosts, ok := taskCtx.Params["hosts"].([]interface{})
	if !ok || len(hosts) == 0 {
		result.Success = false
		result.Error = "缺少主机列表参数"
		return result
	}

	hostInfo, ok := taskCtx.Params["host_info"].(map[string]interface{})
	if !ok {
		result.Success = false
		result.Error = "缺少主机连接信息"
		return result
	}

	e.LogProgress(onProgress, 10, fmt.Sprintf("准备执行模块: %s", module))

	// 生成inventory
	inventoryPath, err := e.generateInventory(hosts, hostInfo)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("生成inventory失败: %v", err)
		return result
	}
	defer os.Remove(inventoryPath)

	// 执行adhoc命令
	output, err := e.runAnsibleAdhoc(ctx, inventoryPath, module, args, onProgress, onLog)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("执行adhoc命令失败: %v", err)
		return result
	}

	result.Success = true
	result.Result = output
	result.Details["module"] = module
	result.Details["args"] = args

	e.LogProgress(onProgress, 100, "adhoc命令执行完成")
	return result
}

// executeSetup 执行setup模块
func (e *AnsibleExecutor) executeSetup(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
	// setup模块与host_facts基本相同
	return e.executeHostFacts(ctx, taskCtx, onProgress, onLog)
}

// generateInventory 生成Ansible inventory文件
func (e *AnsibleExecutor) generateInventory(hosts []interface{}, hostInfo map[string]interface{}) (string, error) {
	inventory := make(map[string]interface{})
	allHosts := make(map[string]interface{})

	for _, hostInterface := range hosts {
		host, ok := hostInterface.(string)
		if !ok {
			continue
		}

		info, ok := hostInfo[host].(map[string]interface{})
		if !ok {
			continue
		}

		hostVars := map[string]interface{}{
			"ansible_host": host,
		}

		if port, ok := info["port"].(float64); ok {
			hostVars["ansible_port"] = int(port)
		}

		if username, ok := info["username"].(string); ok {
			hostVars["ansible_user"] = username
		}

		if password, ok := info["password"].(string); ok && password != "" {
			hostVars["ansible_password"] = password
		}

		if privateKey, ok := info["private_key"].(string); ok && privateKey != "" {
			// 保存私钥到临时文件
			keyFile, err := e.savePrivateKey(privateKey)
			if err == nil {
				hostVars["ansible_ssh_private_key_file"] = keyFile
			}
		}

		// 设置其他连接参数
		hostVars["ansible_ssh_common_args"] = "-o StrictHostKeyChecking=no"

		allHosts[host] = hostVars
	}

	inventory["all"] = map[string]interface{}{
		"hosts": allHosts,
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

// savePrivateKey 保存私钥到临时文件
func (e *AnsibleExecutor) savePrivateKey(privateKey string) (string, error) {
	tmpFile, err := ioutil.TempFile("", "ansible-key-*.pem")
	if err != nil {
		return "", err
	}

	if _, err := tmpFile.WriteString(privateKey); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", err
	}

	tmpFile.Close()

	// 设置文件权限为600
	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

// runAnsibleSetup 运行ansible setup模块
func (e *AnsibleExecutor) runAnsibleSetup(ctx context.Context, inventoryPath string, onProgress ProgressCallback, onLog LogCallback) (map[string]interface{}, error) {
	e.LogProgress(onProgress, 30, "执行setup模块")

	cmd := exec.CommandContext(ctx, "ansible", "all", "-i", inventoryPath, "-m", "setup")
	return e.runAnsibleCommand(cmd, onProgress, onLog)
}

// runAnsiblePing 运行ansible ping模块
func (e *AnsibleExecutor) runAnsiblePing(ctx context.Context, inventoryPath string, onProgress ProgressCallback, onLog LogCallback) (map[string]interface{}, error) {
	e.LogProgress(onProgress, 30, "执行ping模块")

	cmd := exec.CommandContext(ctx, "ansible", "all", "-i", inventoryPath, "-m", "ping")
	return e.runAnsibleCommand(cmd, onProgress, onLog)
}

// runAnsibleAdhoc 运行ansible adhoc命令
func (e *AnsibleExecutor) runAnsibleAdhoc(ctx context.Context, inventoryPath, module, args string, onProgress ProgressCallback, onLog LogCallback) (map[string]interface{}, error) {
	e.LogProgress(onProgress, 30, fmt.Sprintf("执行%s模块", module))

	cmdArgs := []string{"all", "-i", inventoryPath, "-m", module}
	if args != "" {
		cmdArgs = append(cmdArgs, "-a", args)
	}

	cmd := exec.CommandContext(ctx, "ansible", cmdArgs...)
	return e.runAnsibleCommand(cmd, onProgress, onLog)
}

// runAnsibleCommand 运行ansible命令并解析输出
func (e *AnsibleExecutor) runAnsibleCommand(cmd *exec.Cmd, onProgress ProgressCallback, onLog LogCallback) (map[string]interface{}, error) {
	// 设置环境变量
	cmd.Env = append(os.Environ(),
		"ANSIBLE_HOST_KEY_CHECKING=False",
		"ANSIBLE_STDOUT_CALLBACK=json",
	)

	e.LogProgress(onProgress, 40, "启动ansible命令")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// 读取输出
	var outputLines []string
	var errorLines []string

	// 读取stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			outputLines = append(outputLines, line)
			e.LogMessage(onLog, "info", "ansible", line, "", nil)
		}
	}()

	// 读取stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			errorLines = append(errorLines, line)
			e.LogMessage(onLog, "error", "ansible", line, "", nil)
		}
	}()

	e.LogProgress(onProgress, 70, "等待命令执行完成")

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		errorMsg := strings.Join(errorLines, "\n")
		return nil, fmt.Errorf("ansible命令执行失败: %v, 错误信息: %s", err, errorMsg)
	}

	e.LogProgress(onProgress, 90, "解析执行结果")

	// 解析输出
	output := strings.Join(outputLines, "\n")
	result := make(map[string]interface{})

	if output != "" {
		// 尝试解析JSON输出
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			// 如果不是JSON格式，保存原始输出
			result["raw_output"] = output
		}
	}

	return result, nil
}

// ValidateParams 验证Ansible任务参数
func (e *AnsibleExecutor) ValidateParams(params map[string]interface{}) error {
	// 检查必需的参数
	if _, ok := params["hosts"]; !ok {
		return fmt.Errorf("缺少hosts参数")
	}

	if _, ok := params["host_info"]; !ok {
		return fmt.Errorf("缺少host_info参数")
	}

	return nil
}
