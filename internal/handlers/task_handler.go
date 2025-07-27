package handlers

import (
	"ahop/internal/database"
	"ahop/internal/models"
	"ahop/internal/services"
	"ahop/pkg/jwt"
	"ahop/pkg/pagination"
	"ahop/pkg/response"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// TaskHandler 任务处理器
type TaskHandler struct {
	taskService *services.TaskService
}

// NewTaskHandler 创建任务处理器实例
func NewTaskHandler(taskService *services.TaskService) *TaskHandler {
	return &TaskHandler{
		taskService: taskService,
	}
}

// Create 创建任务
func (h *TaskHandler) Create(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	var req struct {
		TaskType    string                 `json:"task_type" binding:"required,oneof=ping collect template"`
		Name        string                 `json:"name" binding:"required,min=1,max=200"`
		Priority    int                    `json:"priority" binding:"omitempty,min=1,max=10"`
		Timeout     int                    `json:"timeout" binding:"omitempty,min=1,max=86400"`
		Description string                 `json:"description" binding:"max=500"`
		
		// 所有任务类型都使用 hosts 字段
		// ping/collect: 主机ID数组
		// template: 主机ID数组
		Hosts       []uint                 `json:"hosts,omitempty"`
		
		// template任务专用字段
		TemplateID  uint                   `json:"template_id,omitempty"`
		Variables   map[string]interface{} `json:"variables,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// 解析验证错误，提供更友好的提示
		if validationErr, ok := err.(validator.ValidationErrors); ok {
			errorMsg := "参数验证失败："
			for _, fieldErr := range validationErr {
				switch fieldErr.Field() {
				case "TaskType":
					errorMsg = "任务类型必须是 ping、collect 或 template"
				case "Name":
					errorMsg = "任务名称不能为空，且长度在1-200个字符之间"
				case "Priority":
					errorMsg = "任务优先级必须在1-10之间"
				case "Timeout":
					errorMsg = "超时时间必须在1-86400秒之间"
				default:
					errorMsg = fmt.Sprintf("字段 %s 验证失败", fieldErr.Field())
				}
				break // 只返回第一个错误
			}
			response.BadRequest(c, errorMsg)
			return
		}
		response.BadRequest(c, "请求参数格式错误")
		return
	}

	// 设置默认值
	if req.Priority == 0 {
		req.Priority = 5
	}
	if req.Timeout == 0 {
		req.Timeout = 3600 // 默认1小时
	}

	// 根据任务类型进行不同的处理
	switch req.TaskType {
	case models.TaskTypeTemplate:
		// 模板任务
		if req.TemplateID == 0 {
			response.BadRequest(c, "模板任务必须指定template_id")
			return
		}
		if len(req.Hosts) == 0 {
			response.BadRequest(c, "模板任务必须指定目标主机")
			return
		}

		// 创建基础任务对象
		task := &models.Task{
			TenantID:    claims.CurrentTenantID,
			Name:        req.Name,
			Priority:    req.Priority,
			Timeout:     req.Timeout,
			Description: req.Description,
			CreatedBy:   claims.UserID,
			Username:    claims.Username,
			Source:      "api",
		}

		// 调用模板任务创建方法
		if err := h.taskService.CreateTemplateTask(task, req.TemplateID, req.Variables, req.Hosts); err != nil {
			// 如果是参数验证失败，返回 BadRequest
			if strings.Contains(err.Error(), "参数验证失败") || 
			   strings.Contains(err.Error(), "任务模板不存在") ||
			   strings.Contains(err.Error(), "主机不存在") {
				response.BadRequest(c, err.Error())
			} else {
				response.ServerError(c, err.Error())
			}
			return
		}
		response.Success(c, task)

	case models.TaskTypePing, models.TaskTypeCollect:
		// ping/collect任务
		if len(req.Hosts) == 0 {
			response.BadRequest(c, "ping/collect任务必须指定hosts")
			return
		}

		// 验证主机是否存在且属于当前租户
		if err := h.validateHosts(claims.CurrentTenantID, req.Hosts); err != nil {
			response.BadRequest(c, err.Error())
			return
		}

		// 验证 variables 中的 ansible 参数
		if err := h.validateVariables(req.TaskType, req.Variables); err != nil {
			response.BadRequest(c, err.Error())
			return
		}

		// 构建任务参数
		params := h.buildTaskParams(req.Hosts, req.Variables)
		paramsData, err := json.Marshal(params)
		if err != nil {
			response.BadRequest(c, "参数序列化失败")
			return
		}

		// 创建任务
		task := &models.Task{
			TenantID:    claims.CurrentTenantID,
			TaskType:    req.TaskType,
			Name:        req.Name,
			Priority:    req.Priority,
			Params:      paramsData,
			Timeout:     req.Timeout,
			Description: req.Description,
			CreatedBy:   claims.UserID,
			Username:    claims.Username,
			Source:      "api",
		}

		if err := h.taskService.CreateTask(task); err != nil {
			response.ServerError(c, err.Error())
			return
		}
		response.Success(c, task)

	default:
		response.BadRequest(c, "不支持的任务类型")
		return
	}
}

// GetByID 获取任务详情
func (h *TaskHandler) GetByID(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)
	taskID := c.Param("id")

	task, err := h.taskService.GetTask(taskID, claims.CurrentTenantID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, task)
}

// List 获取任务列表
func (h *TaskHandler) List(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)

	// 解析分页参数
	params := pagination.ParsePageParams(c)

	// 构建过滤条件
	filters := make(map[string]interface{})
	if taskType := c.Query("task_type"); taskType != "" {
		filters["task_type"] = taskType
	}
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if priority := c.Query("priority"); priority != "" {
		if p, err := strconv.Atoi(priority); err == nil {
			filters["priority"] = p
		}
	}
	if createdBy := c.Query("created_by"); createdBy != "" {
		if uid, err := strconv.ParseUint(createdBy, 10, 64); err == nil {
			filters["created_by"] = uint(uid)
		}
	}

	// 获取数据
	tasks, total, err := h.taskService.ListTasks(
		claims.CurrentTenantID,
		params.Page,
		params.PageSize,
		filters,
	)

	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	// 构建分页信息
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, tasks, pageInfo)
}

// Cancel 取消任务
func (h *TaskHandler) Cancel(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)
	taskID := c.Param("id")

	if err := h.taskService.CancelTask(taskID, claims.CurrentTenantID, claims.UserID); err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "任务已取消", nil)
}

// GetLogs 获取任务日志
func (h *TaskHandler) GetLogs(c *gin.Context) {
	claims := c.MustGet("claims").(*jwt.JWTClaims)
	taskID := c.Param("id")

	// 解析分页参数
	params := pagination.ParsePageParams(c)

	// 获取日志
	logs, total, err := h.taskService.GetTaskLogs(
		taskID,
		claims.CurrentTenantID,
		params.Page,
		params.PageSize,
	)

	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	// 构建分页信息
	pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
	response.SuccessWithPage(c, logs, pageInfo)
}

// GetStats 获取队列统计信息
func (h *TaskHandler) GetStats(c *gin.Context) {
	stats, err := h.taskService.GetQueueStats()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, stats)
}

// validateHosts 验证主机是否存在且属于当前租户
func (h *TaskHandler) validateHosts(tenantID uint, hostIDs []uint) error {
	var count int64
	if err := database.GetDB().Model(&models.Host{}).
		Where("id IN ? AND tenant_id = ?", hostIDs, tenantID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("查询主机失败: %v", err)
	}

	if int(count) != len(hostIDs) {
		return fmt.Errorf("包含无效的主机ID或主机不属于当前租户")
	}

	// 验证所有主机都有凭证
	var hostsWithoutCred int64
	if err := database.GetDB().Model(&models.Host{}).
		Where("id IN ? AND tenant_id = ? AND (credential_id IS NULL OR credential_id = 0)", 
			hostIDs, tenantID).
		Count(&hostsWithoutCred).Error; err != nil {
		return fmt.Errorf("查询主机凭证失败: %v", err)
	}

	if hostsWithoutCred > 0 {
		return fmt.Errorf("存在 %d 个主机没有绑定凭证", hostsWithoutCred)
	}

	return nil
}

// validateVariables 验证任务变量
func (h *TaskHandler) validateVariables(taskType string, variables map[string]interface{}) error {
	if variables == nil {
		return nil
	}

	// 验证 verbosity
	if v, ok := variables["verbosity"]; ok {
		switch val := v.(type) {
		case float64:
			if val < 0 || val > 5 {
				return fmt.Errorf("verbosity 必须在 0-5 之间")
			}
		case int:
			if val < 0 || val > 5 {
				return fmt.Errorf("verbosity 必须在 0-5 之间")
			}
		default:
			return fmt.Errorf("verbosity 必须是数字")
		}
	}

	// 特定任务类型的验证
	if taskType == "collect" {
		// collect 任务不支持 check_mode
		if checkMode, ok := variables["check_mode"]; ok {
			if enabled, ok := checkMode.(bool); ok && enabled {
				return fmt.Errorf("collect 任务不支持 check_mode")
			}
		}
	}

	return nil
}

// buildTaskParams 构建任务参数
func (h *TaskHandler) buildTaskParams(hosts []uint, variables map[string]interface{}) map[string]interface{} {
	params := map[string]interface{}{
		"hosts": hosts,
	}

	if variables != nil && len(variables) > 0 {
		params["variables"] = variables
	}

	return params
}
