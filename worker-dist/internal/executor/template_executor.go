package executor

import (
	"ahop-worker/internal/models"
	"ahop-worker/internal/types"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// TemplateExecutor 模板任务执行器
type TemplateExecutor struct {
	*BaseExecutor
	db              *gorm.DB
	repoBaseDir     string
	templateBaseDir string  // 独立模板目录
	log             *logrus.Logger
	gitSyncExecutor *GitSyncExecutor // 用于Git仓库同步
	authClient      interface{} // AuthClient接口，避免循环依赖
}

// NewTemplateExecutor 创建模板执行器
func NewTemplateExecutor(db *gorm.DB, repoBaseDir string, templateBaseDir string, log *logrus.Logger) *TemplateExecutor {
	return &TemplateExecutor{
		BaseExecutor: NewBaseExecutor([]string{
			"template", // 模板任务
		}),
		db:              db,
		repoBaseDir:     repoBaseDir,
		templateBaseDir: templateBaseDir,
		log:             log,
	}
}

// SetGitSyncExecutor 设置Git同步执行器
func (e *TemplateExecutor) SetGitSyncExecutor(gitSyncExecutor *GitSyncExecutor) {
	e.gitSyncExecutor = gitSyncExecutor
}

// SetAuthClient 设置认证客户端
func (e *TemplateExecutor) SetAuthClient(authClient interface{}) {
	e.authClient = authClient
}

// Execute 执行模板任务
func (e *TemplateExecutor) Execute(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
	result := &TaskResult{
		Success: true,
		Details: make(map[string]interface{}),
		Logs:    make([]string, 0),
		Result:  make(map[string]interface{}),
	}

	// 记录任务开始
	e.LogMessage(onLog, "info", "system",
		fmt.Sprintf("用户 %s（租户：%s）通过 %s 发起的模板任务开始执行",
			taskCtx.Username, taskCtx.TenantName, taskCtx.Source), "", "")
	e.LogProgress(onProgress, 10, "开始执行模板任务")

	// 获取模板ID
	templateIDFloat, ok := taskCtx.Params["template_id"].(float64)
	if !ok {
		result.Success = false
		result.Error = "缺少 template_id 参数"
		return result
	}
	templateID := uint(templateIDFloat)

	// 获取变量
	variables, _ := taskCtx.Params["variables"].(map[string]interface{})

	// 获取主机信息
	hostInfoMap, ok := taskCtx.Params["_host_info_map"].(map[uint]*types.HostInfo)
	if !ok {
		result.Success = false
		result.Error = "缺少主机信息映射"
		return result
	}

	// 从数据库查询模板信息
	var taskTemplate models.TaskTemplate
	if err := e.db.First(&taskTemplate, templateID).Error; err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("查询任务模板失败: %v", err)
		e.LogMessage(onLog, "error", "template", result.Error, "", "")
		return result
	}

	e.LogMessage(onLog, "info", "template",
		fmt.Sprintf("执行模板: %s (%s), 类型: %s, 主机数: %d",
			taskTemplate.Name, taskTemplate.Code, taskTemplate.ScriptType, len(hostInfoMap)), "", "")
	e.LogProgress(onProgress, 20, "已加载模板信息")

	// 构建脚本文件路径（从独立模板目录）
	templatePath := filepath.Join(e.templateBaseDir, fmt.Sprintf("%d/%s", taskCtx.TenantID, taskTemplate.Code))
	scriptPath := filepath.Join(templatePath, taskTemplate.EntryFile)

	// 检查脚本文件是否存在
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		// 尝试按需同步模板
		e.log.WithFields(logrus.Fields{
			"template_id":   taskTemplate.ID,
			"template_code": taskTemplate.Code,
			"script_path":   scriptPath,
		}).Info("模板文件不存在，尝试按需同步")
		
		if err := e.syncTemplateOnDemand(&taskTemplate); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("模板同步失败: %v", err)
			e.LogMessage(onLog, "error", "template", result.Error, "", "")
			return result
		}
		
		// 再次检查文件是否存在
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			result.Success = false
			result.Error = fmt.Sprintf("模板同步后脚本文件仍不存在: %s", scriptPath)
			e.LogMessage(onLog, "error", "template", result.Error, "", "")
			return result
		}
		
		e.LogMessage(onLog, "info", "template", "模板按需同步成功", "", "")
	}

	// 根据脚本类型执行
	switch taskTemplate.ScriptType {
	case "shell":
		return e.executeShellTemplate(ctx, taskCtx, &taskTemplate, scriptPath, variables, hostInfoMap, onProgress, onLog)
	case "ansible":
		return e.executeAnsibleTemplate(ctx, taskCtx, &taskTemplate, scriptPath, variables, hostInfoMap, onProgress, onLog)
	default:
		result.Success = false
		result.Error = fmt.Sprintf("不支持的脚本类型: %s", taskTemplate.ScriptType)
		e.LogMessage(onLog, "error", "template", result.Error, "", "")
		return result
	}
}

// executeShellTemplate 执行Shell模板
func (e *TemplateExecutor) executeShellTemplate(ctx context.Context, taskCtx *TaskContext, template *models.TaskTemplate,
	scriptPath string, variables map[string]interface{}, hostInfoMap map[uint]*types.HostInfo,
	onProgress ProgressCallback, onLog LogCallback) *TaskResult {

	result := &TaskResult{
		Success: true,
		Details: make(map[string]interface{}),
		Logs:    make([]string, 0),
		Result:  make(map[string]interface{}),
	}

	// 构建命令行参数
	var cmdArgs []string
	if len(variables) > 0 {
		for key, value := range variables {
			// 对值进行安全处理
			safeValue := fmt.Sprintf("%v", value)
			// 添加命令行参数 --key value
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key))
			cmdArgs = append(cmdArgs, safeValue)
		}
	}

	// 记录执行信息
	if len(cmdArgs) > 0 {
		e.LogMessage(onLog, "info", "template",
			fmt.Sprintf("脚本参数: %s", strings.Join(cmdArgs, " ")), "", "")
	}

	e.LogProgress(onProgress, 30, "脚本准备完成，开始在主机上执行")

	// 在每台主机上执行
	hostResults := make(map[string]interface{})
	successCount := 0
	failedCount := 0

	// 将hostInfoMap转为slice以便遍历
	hostList := make([]*types.HostInfo, 0, len(hostInfoMap))
	for _, hostInfo := range hostInfoMap {
		hostList = append(hostList, hostInfo)
	}

	// 执行脚本
	for i, hostInfo := range hostList {
		progress := 30 + (i * 60 / len(hostList))
		e.LogProgress(onProgress, progress, fmt.Sprintf("正在执行主机 %s (%s)", hostInfo.Hostname, hostInfo.IP))

		e.LogMessage(onLog, "info", "template",
			fmt.Sprintf("开始在主机 %s (%s) 执行Shell脚本...", hostInfo.Hostname, hostInfo.IP),
			hostInfo.Hostname, "")

		// 创建实时日志记录器
		var realtimeLogger *RealtimeLogger
		if e.redisClient != nil && taskCtx.TaskID != "" {
			realtimeLogger = NewRealtimeLogger(e.redisClient, taskCtx.TaskID)
		}

		// 通过SSH执行脚本
		stdout, stderr, exitCode, err := e.executeSSHScriptWithArgs(hostInfo, scriptPath, cmdArgs, template.RequireSudo, realtimeLogger)

		hostKey := fmt.Sprintf("%s:%d", hostInfo.IP, hostInfo.Port)

		if err != nil {
			failedCount++
			hostResults[hostKey] = map[string]interface{}{
				"host_id":   hostInfo.ID,
				"hostname":  hostInfo.Hostname,
				"ip":        hostInfo.IP,
				"port":      hostInfo.Port,
				"success":   false,
				"error":     err.Error(),
				"message":   fmt.Sprintf("脚本执行失败: %v", err),
				"output":    stdout,
				"stderr":    stderr,
				"exit_code": exitCode,
			}
			e.LogMessage(onLog, "error", "template",
				fmt.Sprintf("主机 %s (%s) 脚本执行失败: %v", hostInfo.Hostname, hostInfo.IP, err),
				hostInfo.Hostname, "")
		} else {
			successCount++
			hostResults[hostKey] = map[string]interface{}{
				"host_id":   hostInfo.ID,
				"hostname":  hostInfo.Hostname,
				"ip":        hostInfo.IP,
				"port":      hostInfo.Port,
				"success":   true,
				"message":   "脚本执行成功",
				"output":    stdout,
				"stderr":    stderr,
				"exit_code": exitCode,
			}
			e.LogMessage(onLog, "info", "template",
				fmt.Sprintf("主机 %s (%s) 脚本执行成功", hostInfo.Hostname, hostInfo.IP),
				hostInfo.Hostname, "")
		}
	}

	// 汇总结果
	result.Result = map[string]interface{}{
		"hosts": hostResults,
		"summary": map[string]interface{}{
			"total":   len(hostInfoMap),
			"success": successCount,
			"failed":  failedCount,
		},
		"template": map[string]interface{}{
			"id":          template.ID,
			"name":        template.Name,
			"code":        template.Code,
			"script_type": template.ScriptType,
		},
	}

	if successCount == 0 && len(hostInfoMap) > 0 {
		result.Success = false
		result.Error = "所有主机执行失败"
	}

	e.LogProgress(onProgress, 100, "模板任务执行完成")
	e.LogMessage(onLog, "info", "template",
		fmt.Sprintf("模板任务完成: 成功%d个，失败%d个", successCount, failedCount), "", "")

	return result
}

// executeAnsibleTemplate 执行Ansible模板
func (e *TemplateExecutor) executeAnsibleTemplate(ctx context.Context, taskCtx *TaskContext, template *models.TaskTemplate,
	playbookPath string, variables map[string]interface{}, hostInfoMap map[uint]*types.HostInfo,
	onProgress ProgressCallback, onLog LogCallback) *TaskResult {

	result := &TaskResult{
		Success: true,
		Details: make(map[string]interface{}),
		Logs:    make([]string, 0),
		Result:  make(map[string]interface{}),
	}

	// 创建临时inventory文件
	inventoryFile, err := e.createAnsibleInventory(hostInfoMap)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("创建inventory文件失败: %v", err)
		return result
	}
	defer os.Remove(inventoryFile)

	e.LogProgress(onProgress, 30, "准备执行Ansible playbook")

	// 构建ansible-playbook命令
	args := []string{
		"-i", inventoryFile,
		playbookPath,
	}

	// 添加变量 - 需要正确处理不同类型
	for k, v := range variables {
		var varStr string
		switch val := v.(type) {
		case string:
			// 字符串类型，需要引号
			varStr = fmt.Sprintf("%s=%q", k, val)
		case []interface{}:
			// 数组类型，转换为 JSON
			jsonBytes, _ := json.Marshal(val)
			varStr = fmt.Sprintf("%s=%s", k, string(jsonBytes))
		case map[string]interface{}:
			// 对象类型，转换为 JSON
			jsonBytes, _ := json.Marshal(val)
			varStr = fmt.Sprintf("%s=%s", k, string(jsonBytes))
		default:
			// 其他类型（数字、布尔等）
			varStr = fmt.Sprintf("%s=%v", k, val)
		}
		args = append(args, "-e", varStr)
	}

	// 如果需要sudo
	if template.RequireSudo {
		args = append(args, "--become")
	}

	// 创建实时日志记录器
	var realtimeLogger *RealtimeLogger
	if e.redisClient != nil && taskCtx.TaskID != "" {
		realtimeLogger = NewRealtimeLogger(e.redisClient, taskCtx.TaskID)
	}

	e.LogMessage(onLog, "info", "template",
		fmt.Sprintf("执行命令: ansible-playbook %s", strings.Join(args, " ")), "", "")

	// 执行ansible-playbook
	cmd := exec.CommandContext(ctx, "ansible-playbook", args...)
	
	// 设置工作目录为playbook所在目录
	cmd.Dir = filepath.Dir(playbookPath)
	
	// 设置环境变量
	cmd.Env = append(os.Environ(),
		"ANSIBLE_HOST_KEY_CHECKING=False",
		// 移除 JSON callback 以获得实时输出
		// "ANSIBLE_STDOUT_CALLBACK=json",
		// "ANSIBLE_LOAD_CALLBACK_PLUGINS=True",
		"ANSIBLE_GATHERING=explicit",  // 默认禁用facts收集，提高性能
	)

	// 创建管道以实时读取输出
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("创建stdout管道失败: %v", err)
		return result
	}
	
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("创建stderr管道失败: %v", err)
		return result
	}

	// 用于保存完整输出（数据库需要）
	var stdout, stderr bytes.Buffer
	
	// 启动命令（非阻塞）
	if err = cmd.Start(); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("启动ansible-playbook失败: %v", err)
		return result
	}
	
	// 并发读取stdout和stderr
	var wg sync.WaitGroup
	wg.Add(2)
	
	// 读取stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stdout.WriteString(line + "\n")  // 保存到buffer
			
			// 实时发送到Redis
			if realtimeLogger != nil && strings.TrimSpace(line) != "" {
				realtimeLogger.LogOutput("ansible", line, "")
			}
		}
	}()
	
	// 读取stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stderr.WriteString(line + "\n")  // 保存到buffer
			
			// 实时发送错误到Redis
			if realtimeLogger != nil && strings.TrimSpace(line) != "" {
				realtimeLogger.LogError("ansible", line, "")
			}
		}
	}()
	
	// 等待命令完成
	err = cmd.Wait()
	
	// 等待所有输出读取完成
	wg.Wait()
	
	// 获取退出码
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// 解析结果
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Ansible执行失败: %v", err)
		e.LogMessage(onLog, "error", "template", result.Error, "", "")
	} else {
		e.LogMessage(onLog, "info", "template", "Ansible playbook执行成功", "", "")
	}

	// 设置结果 - ansible是批量执行，不需要按主机分组
	result.Result = map[string]interface{}{
		"output":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": exitCode,
		"success":   err == nil,
		"message":   func() string { if err == nil { return "Ansible playbook执行成功" } else { return fmt.Sprintf("Ansible执行失败: %v", err) } }(),
		"template": map[string]interface{}{
			"id":          template.ID,
			"name":        template.Name,
			"code":        template.Code,
			"script_type": template.ScriptType,
		},
	}

	e.LogProgress(onProgress, 100, "Ansible任务执行完成")

	return result
}

// executeSSHScript 通过SSH执行脚本
func (e *TemplateExecutor) executeSSHScript(hostInfo *types.HostInfo, script string, requireSudo bool, 
	realtimeLogger *RealtimeLogger) (stdout, stderr string, exitCode int, err error) {
	
	// 创建SSH配置
	config := &ssh.ClientConfig{
		User:            hostInfo.Credential.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// 根据凭证类型设置认证方式
	switch hostInfo.Credential.Type {
	case "password":
		config.Auth = append(config.Auth, ssh.Password(hostInfo.Credential.Password))
	case "ssh_key":
		signer, parseErr := ssh.ParsePrivateKey([]byte(hostInfo.Credential.PrivateKey))
		if parseErr != nil {
			err = fmt.Errorf("解析私钥失败: %v", parseErr)
			return
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	default:
		err = fmt.Errorf("不支持的凭证类型: %s", hostInfo.Credential.Type)
		return
	}

	// 连接SSH
	addr := fmt.Sprintf("%s:%d", hostInfo.IP, hostInfo.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		err = fmt.Errorf("SSH连接失败: %v", err)
		if realtimeLogger != nil {
			realtimeLogger.LogError("template", err.Error(), hostInfo.Hostname)
		}
		return
	}
	defer client.Close()

	// 创建会话
	session, err := client.NewSession()
	if err != nil {
		err = fmt.Errorf("创建SSH会话失败: %v", err)
		if realtimeLogger != nil {
			realtimeLogger.LogError("template", err.Error(), hostInfo.Hostname)
		}
		return
	}
	defer session.Close()

	// 准备执行命令
	command := script
	if requireSudo {
		command = fmt.Sprintf("sudo bash -c %q", script)
	}

	// 创建管道以实时读取输出
	stdoutPipe, pipeErr := session.StdoutPipe()
	if pipeErr != nil {
		err = fmt.Errorf("创建stdout管道失败: %v", pipeErr)
		return
	}
	
	stderrPipe, pipeErr := session.StderrPipe()
	if pipeErr != nil {
		err = fmt.Errorf("创建stderr管道失败: %v", pipeErr)
		return
	}

	// 用于保存完整输出
	var stdoutBuf, stderrBuf bytes.Buffer
	
	// 发送开始执行信息
	if realtimeLogger != nil {
		realtimeLogger.LogOutput("template", "开始执行脚本...", hostInfo.Hostname)
	}

	// 启动命令（非阻塞）
	if err = session.Start(command); err != nil {
		err = fmt.Errorf("启动命令失败: %v", err)
		return
	}
	
	// 并发读取stdout和stderr
	var wg sync.WaitGroup
	wg.Add(2)
	
	// 读取stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stdoutBuf.WriteString(line + "\n")
			
			// 实时发送到Redis
			if realtimeLogger != nil && strings.TrimSpace(line) != "" {
				realtimeLogger.LogOutput("template", line, hostInfo.Hostname)
			}
		}
	}()
	
	// 读取stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line + "\n")
			
			// 实时发送错误到Redis
			if realtimeLogger != nil && strings.TrimSpace(line) != "" {
				realtimeLogger.LogError("template", line, hostInfo.Hostname)
			}
		}
	}()
	
	// 等待命令完成
	runErr := session.Wait()
	
	// 等待所有输出读取完成
	wg.Wait()
	
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
		err = runErr
	}

	// 发送执行结果（保留结果通知）
	if realtimeLogger != nil {
		if err != nil {
			realtimeLogger.LogError("template", fmt.Sprintf("脚本执行失败 (退出码: %d)", exitCode), hostInfo.Hostname)
		} else {
			realtimeLogger.LogOutput("template", fmt.Sprintf("脚本执行成功 (退出码: %d)", exitCode), hostInfo.Hostname)
		}
	}

	return
}

// createAnsibleInventory 创建Ansible inventory文件（使用JSON格式，更安全）
func (e *TemplateExecutor) createAnsibleInventory(hostInfoMap map[uint]*types.HostInfo) (string, error) {
	// 构建inventory结构
	inventory := map[string]interface{}{
		"all": map[string]interface{}{
			"hosts": make(map[string]interface{}),
		},
	}
	allHosts := inventory["all"].(map[string]interface{})["hosts"].(map[string]interface{})

	// 收集临时文件，用于清理
	var tempFiles []string
	
	for _, hostInfo := range hostInfoMap {
		// 如果没有hostname，使用IP作为主机标识
		hostIdentifier := hostInfo.Hostname
		if hostIdentifier == "" {
			hostIdentifier = hostInfo.IP
		}
		
		hostVars := map[string]interface{}{
			"ansible_host": hostInfo.IP,
			"ansible_port": hostInfo.Port,
			"ansible_ssh_common_args": "-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null",
			"ansible_connection": "ssh",
		}
		
		// 添加认证信息
		if hostInfo.Credential != nil {
			hostVars["ansible_user"] = hostInfo.Credential.Username
			
			switch hostInfo.Credential.Type {
			case "password":
				hostVars["ansible_password"] = hostInfo.Credential.Password
			case "ssh_key":
				// 创建临时私钥文件
				keyFile, err := ioutil.TempFile("", "ansible-key-*.pem")
				if err != nil {
					// 清理已创建的临时文件
					for _, f := range tempFiles {
						os.Remove(f)
					}
					return "", fmt.Errorf("创建私钥文件失败: %v", err)
				}
				keyFile.WriteString(hostInfo.Credential.PrivateKey)
				keyFile.Close()
				os.Chmod(keyFile.Name(), 0600)
				tempFiles = append(tempFiles, keyFile.Name())
				
				hostVars["ansible_ssh_private_key_file"] = keyFile.Name()
				
				// 如果有passphrase
				if hostInfo.Credential.Passphrase != "" {
					hostVars["ansible_ssh_pass"] = hostInfo.Credential.Passphrase
				}
			}
		}
		
		allHosts[hostIdentifier] = hostVars
	}

	// 写入JSON格式的inventory文件
	data, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		// 清理临时文件
		for _, f := range tempFiles {
			os.Remove(f)
		}
		return "", fmt.Errorf("序列化inventory失败: %v", err)
	}
	
	tmpFile, err := ioutil.TempFile("", "ansible-inventory-*.json")
	if err != nil {
		// 清理临时文件
		for _, f := range tempFiles {
			os.Remove(f)
		}
		return "", fmt.Errorf("创建inventory文件失败: %v", err)
	}
	
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		// 清理临时文件
		for _, f := range tempFiles {
			os.Remove(f)
		}
		return "", fmt.Errorf("写入inventory文件失败: %v", err)
	}

	tmpFile.Close()
	return tmpFile.Name(), nil
}

// executeSSHScriptWithArgs 通过SSH执行带参数的脚本
func (e *TemplateExecutor) executeSSHScriptWithArgs(hostInfo *types.HostInfo, scriptPath string, args []string, 
	requireSudo bool, realtimeLogger *RealtimeLogger) (stdout, stderr string, exitCode int, err error) {
	
	// 创建SSH配置
	config := &ssh.ClientConfig{
		User:            hostInfo.Credential.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// 根据凭证类型设置认证方式
	switch hostInfo.Credential.Type {
	case "password":
		config.Auth = append(config.Auth, ssh.Password(hostInfo.Credential.Password))
	case "ssh_key":
		signer, parseErr := ssh.ParsePrivateKey([]byte(hostInfo.Credential.PrivateKey))
		if parseErr != nil {
			err = fmt.Errorf("解析私钥失败: %v", parseErr)
			return
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	default:
		err = fmt.Errorf("不支持的凭证类型: %s", hostInfo.Credential.Type)
		return
	}

	// 连接SSH
	addr := fmt.Sprintf("%s:%d", hostInfo.IP, hostInfo.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		err = fmt.Errorf("SSH连接失败: %v", err)
		if realtimeLogger != nil {
			realtimeLogger.LogError("template", err.Error(), hostInfo.Hostname)
		}
		return
	}
	defer client.Close()

	// 创建会话
	session, err := client.NewSession()
	if err != nil {
		err = fmt.Errorf("创建SSH会话失败: %v", err)
		if realtimeLogger != nil {
			realtimeLogger.LogError("template", err.Error(), hostInfo.Hostname)
		}
		return
	}
	defer session.Close()

	// 创建临时脚本文件
	tmpScript := fmt.Sprintf("/tmp/ahop_script_%d.sh", time.Now().UnixNano())
	
	// 首先上传脚本文件
	uploadSession, err := client.NewSession()
	if err != nil {
		err = fmt.Errorf("创建上传会话失败: %v", err)
		return
	}
	defer uploadSession.Close()

	// 读取脚本内容
	scriptContent, readErr := ioutil.ReadFile(scriptPath)
	if readErr != nil {
		err = fmt.Errorf("读取脚本文件失败: %v", readErr)
		return
	}

	// 上传脚本
	// 使用特殊的分隔符避免与脚本内容冲突
	uploadCmd := fmt.Sprintf("cat > %s << 'AHOP_SCRIPT_EOF'\n%s\nAHOP_SCRIPT_EOF\nchmod +x %s", tmpScript, string(scriptContent), tmpScript)
	if uploadErr := uploadSession.Run(uploadCmd); uploadErr != nil {
		err = fmt.Errorf("上传脚本失败: %v", uploadErr)
		return
	}

	// 构建执行命令
	command := tmpScript
	if len(args) > 0 {
		// 对参数进行引号处理，防止空格等特殊字符
		quotedArgs := make([]string, len(args))
		for i, arg := range args {
			// 如果参数包含空格或特殊字符，添加引号
			if strings.ContainsAny(arg, " \t\n'\"$") {
				quotedArgs[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(arg, "'", "'\"'\"'"))
			} else {
				quotedArgs[i] = arg
			}
		}
		command = fmt.Sprintf("%s %s", tmpScript, strings.Join(quotedArgs, " "))
	}
	
	if requireSudo {
		command = fmt.Sprintf("sudo bash -c %q", command)
	}

	// 创建管道以实时读取输出
	stdoutPipe, pipeErr := session.StdoutPipe()
	if pipeErr != nil {
		err = fmt.Errorf("创建stdout管道失败: %v", pipeErr)
		return
	}
	
	stderrPipe, pipeErr := session.StderrPipe()
	if pipeErr != nil {
		err = fmt.Errorf("创建stderr管道失败: %v", pipeErr)
		return
	}

	// 用于保存完整输出
	var stdoutBuf, stderrBuf bytes.Buffer
	
	// 发送开始执行信息
	if realtimeLogger != nil {
		realtimeLogger.LogOutput("template", fmt.Sprintf("执行脚本: %s", command), hostInfo.Hostname)
	}

	// 启动命令（非阻塞）
	if err = session.Start(command); err != nil {
		err = fmt.Errorf("启动命令失败: %v", err)
		return
	}
	
	// 并发读取stdout和stderr
	var wg sync.WaitGroup
	wg.Add(2)
	
	// 读取stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stdoutBuf.WriteString(line + "\n")
			
			// 实时发送到Redis
			if realtimeLogger != nil && strings.TrimSpace(line) != "" {
				realtimeLogger.LogOutput("template", line, hostInfo.Hostname)
			}
		}
	}()
	
	// 读取stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line + "\n")
			
			// 实时发送错误到Redis
			if realtimeLogger != nil && strings.TrimSpace(line) != "" {
				realtimeLogger.LogError("template", line, hostInfo.Hostname)
			}
		}
	}()
	
	// 等待命令完成
	runErr := session.Wait()
	
	// 等待所有输出读取完成
	wg.Wait()
	
	// 清理临时文件
	cleanupSession, _ := client.NewSession()
	if cleanupSession != nil {
		cleanupSession.Run(fmt.Sprintf("rm -f %s", tmpScript))
		cleanupSession.Close()
	}
	
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
		err = runErr
	}

	// 发送执行结果（保留结果通知）
	if realtimeLogger != nil {
		if err != nil {
			realtimeLogger.LogError("template", fmt.Sprintf("脚本执行失败 (退出码: %d)", exitCode), hostInfo.Hostname)
		} else {
			realtimeLogger.LogOutput("template", fmt.Sprintf("脚本执行成功 (退出码: %d)", exitCode), hostInfo.Hostname)
		}
	}

	return
}

// ValidateParams 验证参数
func (e *TemplateExecutor) ValidateParams(params map[string]interface{}) error {
	// 检查template_id
	if _, ok := params["template_id"]; !ok {
		return fmt.Errorf("缺少 template_id 参数")
	}
	
	// 检查主机信息
	if _, ok := params["_host_info_map"]; !ok {
		return fmt.Errorf("缺少主机信息映射")
	}
	
	return nil
}

// syncTemplateOnDemand 按需同步模板
func (e *TemplateExecutor) syncTemplateOnDemand(template *models.TaskTemplate) error {
	// 从source_git_info获取Git仓库信息
	var gitInfo map[string]interface{}
	if template.SourceGitInfo != nil {
		if err := template.SourceGitInfo.Unmarshal(&gitInfo); err != nil {
			return fmt.Errorf("解析Git来源信息失败: %v", err)
		}
	} else {
		return fmt.Errorf("模板缺少Git来源信息")
	}

	// 获取仓库ID
	repoID, ok := gitInfo["repository_id"].(float64)
	if !ok {
		return fmt.Errorf("Git来源信息缺少repository_id")
	}

	// 获取original_path（文件在仓库中的子目录）
	sourcePath := ""
	if op, ok := gitInfo["original_path"].(string); ok {
		sourcePath = op
	}

	// 从数据库查询Git仓库信息
	var gitRepo models.GitRepository
	if err := e.db.Where("id = ?", uint(repoID)).First(&gitRepo).Error; err != nil {
		return fmt.Errorf("查询Git仓库失败: %v", err)
	}

	// 构建源仓库路径
	localPath := gitRepo.LocalPath
	sourceRepoPath := filepath.Join(e.repoBaseDir, localPath)
	if sourcePath != "" {
		sourceRepoPath = filepath.Join(sourceRepoPath, sourcePath)
	}
	
	// 检查Git仓库是否存在
	if _, err := os.Stat(sourceRepoPath); os.IsNotExist(err) {
		// 如果Git仓库也不存在，需要先同步Git仓库
		e.log.WithFields(logrus.Fields{
			"repository_id": uint(repoID),
			"local_path":    localPath,
		}).Info("Git仓库不存在，需要先同步仓库")
		
		// 构建Git同步消息（从gitInfo中获取必要信息）
		syncMsg := &types.GitSyncMessage{
			RepositoryID: uint(repoID),
			TenantID:     template.TenantID,
			Action:       "sync",
			TaskType:     "manual",  // 按需同步
			Repository: types.RepositoryInfo{
				ID:        uint(repoID),
				Name:      gitRepo.Name,
				LocalPath: gitRepo.LocalPath,
			},
		}
		
		// 从gitInfo中补充其他信息
		if url, ok := gitInfo["url"].(string); ok {
			syncMsg.Repository.URL = url
		}
		if branch, ok := gitInfo["branch"].(string); ok {
			syncMsg.Repository.Branch = branch
		}
		if isPublic, ok := gitInfo["is_public"].(bool); ok {
			syncMsg.Repository.IsPublic = isPublic
		}
		if credID, ok := gitInfo["credential_id"].(float64); ok {
			credentialID := uint(credID)
			syncMsg.Repository.CredentialID = &credentialID
		}
		
		// 使用GitSyncExecutor同步仓库
		if e.gitSyncExecutor != nil {
			if err := e.gitSyncExecutor.ProcessGitSyncMessage(syncMsg, ""); err != nil {
				return fmt.Errorf("同步Git仓库失败: %v", err)
			}
		} else {
			return fmt.Errorf("GitSyncExecutor未初始化")
		}
	}

	// 构建目标路径
	templateDir := filepath.Join(e.templateBaseDir, fmt.Sprintf("%d/%s", template.TenantID, template.Code))
	
	// 创建目标目录
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return fmt.Errorf("创建模板目录失败: %v", err)
	}
	
	// 复制入口文件
	sourceFile := filepath.Join(sourceRepoPath, template.EntryFile)
	targetFile := filepath.Join(templateDir, template.EntryFile)
	
	if err := e.copyFile(sourceFile, targetFile); err != nil {
		return fmt.Errorf("复制入口文件失败: %v", err)
	}
	
	// 复制包含的文件
	for _, file := range template.IncludedFiles {
		srcPath := filepath.Join(sourceRepoPath, file.Path)
		dstPath := filepath.Join(templateDir, file.Path)
		
		// 创建子目录
		if dir := filepath.Dir(dstPath); dir != templateDir {
			if err := os.MkdirAll(dir, 0755); err != nil {
				e.log.WithError(err).WithField("dir", dir).Warn("创建子目录失败")
				continue
			}
		}
		
		// 复制文件
		if err := e.copyFile(srcPath, dstPath); err != nil {
			e.log.WithError(err).WithFields(logrus.Fields{
				"src": srcPath,
				"dst": dstPath,
			}).Warn("复制包含文件失败")
		}
	}

	e.log.WithFields(logrus.Fields{
		"template_id":   template.ID,
		"template_code": template.Code,
		"tenant_id":     template.TenantID,
	}).Info("模板按需同步成功")

	return nil
}

// copyFile 复制文件
func (e *TemplateExecutor) copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %v", err)
	}
	defer source.Close()
	
	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %v", err)
	}
	defer destination.Close()
	
	if _, err := io.Copy(destination, source); err != nil {
		return fmt.Errorf("复制文件内容失败: %v", err)
	}
	
	// 复制文件权限
	info, err := os.Stat(src)
	if err == nil {
		os.Chmod(dst, info.Mode())
	}
	
	return nil
}