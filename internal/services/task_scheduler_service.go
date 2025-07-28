package services

import (
	"ahop/internal/models"
	"ahop/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// TaskSchedulerService 任务调度服务
type TaskSchedulerService struct {
	db              *gorm.DB
	taskService     *TaskService         // 复用现有任务服务
	templateService *TaskTemplateService // 复用模板服务
	cron            *cron.Cron
	jobs            map[uint]cron.EntryID // scheduledTaskID -> cronEntryID
	mu              sync.RWMutex
	running         bool
}

// NewTaskSchedulerService 创建任务调度服务
func NewTaskSchedulerService(db *gorm.DB, taskService *TaskService, templateService *TaskTemplateService) *TaskSchedulerService {
	return &TaskSchedulerService{
		db:              db,
		taskService:     taskService,
		templateService: templateService,
		cron:            cron.New(),
		jobs:            make(map[uint]cron.EntryID),
	}
}

// Start 启动调度器
func (s *TaskSchedulerService) Start() error {
	if s.running {
		return fmt.Errorf("调度器已经在运行")
	}

	logger.GetLogger().Info("启动定时任务调度器")

	// 加载所有启用的定时任务
	var scheduledTasks []models.ScheduledTask
	err := s.db.Where("is_active = ?", true).Find(&scheduledTasks).Error
	if err != nil {
		return fmt.Errorf("加载定时任务失败: %v", err)
	}

	// 添加到调度器
	for _, task := range scheduledTasks {
		if err := s.addJob(&task); err != nil {
			logger.GetLogger().Errorf("添加定时任务失败 [%s]: %v", task.Name, err)
			continue
		}
	}

	// 启动cron调度器
	s.cron.Start()
	s.running = true

	logger.GetLogger().Infof("定时任务调度器启动成功，已加载 %d 个任务", len(scheduledTasks))
	return nil
}

// Stop 停止调度器
func (s *TaskSchedulerService) Stop() {
	if !s.running {
		return
	}

	logger.GetLogger().Info("停止定时任务调度器")
	s.cron.Stop()
	s.running = false
}

// CreateScheduledTask 创建定时任务
func (s *TaskSchedulerService) CreateScheduledTask(tenantID, userID uint, req *CreateScheduledTaskRequest) (*models.ScheduledTask, error) {
	// 1. 验证任务模板
	template, err := s.templateService.GetByID(req.TemplateID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("任务模板不存在")
	}

	if template.Status != "active" {
		return nil, fmt.Errorf("任务模板未激活")
	}

	// 2. 验证主机
	if len(req.HostIDs) == 0 {
		return nil, fmt.Errorf("请选择执行主机")
	}

	// 验证主机是否存在且属于当前租户
	var hostCount int64
	s.db.Model(&models.Host{}).Where("id IN ? AND tenant_id = ?", req.HostIDs, tenantID).Count(&hostCount)
	if int(hostCount) != len(req.HostIDs) {
		return nil, fmt.Errorf("部分主机不存在或不属于当前租户")
	}

	// 3. 验证变量（复用模板验证逻辑）
	if err := s.templateService.ValidateTemplateVariables(template, req.Variables); err != nil {
		return nil, fmt.Errorf("参数验证失败: %v", err)
	}

	// 4. 验证cron表达式
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(req.CronExpr)
	if err != nil {
		return nil, fmt.Errorf("无效的cron表达式: %v", err)
	}

	// 5. 创建定时任务
	hostIDsJSON, _ := json.Marshal(req.HostIDs)
	variablesJSON, _ := json.Marshal(req.Variables)

	scheduledTask := &models.ScheduledTask{
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		CronExpr:    req.CronExpr,
		TemplateID:  req.TemplateID,
		HostIDs:     models.JSON(hostIDsJSON),
		Variables:   models.JSON(variablesJSON),
		TimeoutMins: req.TimeoutMins,
		CreatedBy:   userID,
		NextRunAt:   s.calculateNextRun(schedule),
		LastStatus:  models.ScheduledTaskStatusIdle,
	}

	// 6. 保存到数据库
	if err := s.db.Create(scheduledTask).Error; err != nil {
		return nil, fmt.Errorf("创建定时任务失败: %v", err)
	}

	// 7. 添加到调度器
	if req.IsActive {
		if err := s.addJob(scheduledTask); err != nil {
			// 回滚
			s.db.Delete(scheduledTask)
			return nil, fmt.Errorf("添加调度任务失败: %v", err)
		}
	}

	// 重新加载关联数据
	s.db.Preload("Template").First(scheduledTask, scheduledTask.ID)

	return scheduledTask, nil
}

// UpdateScheduledTask 更新定时任务
func (s *TaskSchedulerService) UpdateScheduledTask(id, tenantID, userID uint, req *UpdateScheduledTaskRequest) (*models.ScheduledTask, error) {
	// 查询现有任务
	var scheduledTask models.ScheduledTask
	if err := s.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&scheduledTask).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("定时任务不存在")
		}
		return nil, err
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.TimeoutMins > 0 {
		updates["timeout_mins"] = req.TimeoutMins
	}

	// 如果更新了执行配置
	needReschedule := false
	
	if req.CronExpr != "" && req.CronExpr != scheduledTask.CronExpr {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(req.CronExpr)
		if err != nil {
			return nil, fmt.Errorf("无效的cron表达式: %v", err)
		}
		updates["cron_expr"] = req.CronExpr
		updates["next_run_at"] = s.calculateNextRun(schedule)
		needReschedule = true
	}

	if len(req.HostIDs) > 0 {
		// 验证主机
		var hostCount int64
		s.db.Model(&models.Host{}).Where("id IN ? AND tenant_id = ?", req.HostIDs, tenantID).Count(&hostCount)
		if int(hostCount) != len(req.HostIDs) {
			return nil, fmt.Errorf("部分主机不存在或不属于当前租户")
		}
		hostIDsJSON, _ := json.Marshal(req.HostIDs)
		updates["host_ids"] = models.JSON(hostIDsJSON)
	}

	if req.Variables != nil {
		// 验证变量
		template, err := s.templateService.GetByID(scheduledTask.TemplateID, tenantID)
		if err != nil {
			return nil, fmt.Errorf("任务模板不存在")
		}
		if err := s.templateService.ValidateTemplateVariables(template, req.Variables); err != nil {
			return nil, fmt.Errorf("参数验证失败: %v", err)
		}
		variablesJSON, _ := json.Marshal(req.Variables)
		updates["variables"] = models.JSON(variablesJSON)
	}

	updates["updated_by"] = userID
	updates["updated_at"] = time.Now()

	// 更新数据库
	if err := s.db.Model(&scheduledTask).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新定时任务失败: %v", err)
	}

	// 如果需要重新调度
	if needReschedule && scheduledTask.IsActive {
		s.removeJob(scheduledTask.ID)
		s.db.First(&scheduledTask, scheduledTask.ID) // 重新加载
		if err := s.addJob(&scheduledTask); err != nil {
			logger.GetLogger().Errorf("重新调度任务失败: %v", err)
		}
	}

	// 重新加载完整数据
	s.db.Preload("Template").First(&scheduledTask, scheduledTask.ID)

	return &scheduledTask, nil
}

// DeleteScheduledTask 删除定时任务
func (s *TaskSchedulerService) DeleteScheduledTask(id, tenantID uint) error {
	var scheduledTask models.ScheduledTask
	if err := s.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&scheduledTask).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("定时任务不存在")
		}
		return err
	}

	// 检查是否正在执行
	if scheduledTask.LastStatus == models.ScheduledTaskStatusRunning {
		return fmt.Errorf("任务正在执行中，无法删除")
	}

	// 从调度器移除
	s.removeJob(id)

	// 使用事务级联删除所有相关数据
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 获取所有相关的task_id
		var taskIDs []string
		if err := tx.Model(&models.ScheduledTaskExecution{}).
			Where("scheduled_task_id = ?", id).
			Pluck("task_id", &taskIDs).Error; err != nil {
			return fmt.Errorf("获取任务ID列表失败: %v", err)
		}

		// 2. 删除所有任务日志
		if len(taskIDs) > 0 {
			if err := tx.Where("task_id IN ?", taskIDs).Delete(&models.TaskLog{}).Error; err != nil {
				return fmt.Errorf("删除任务日志失败: %v", err)
			}
			
			// 3. 删除所有任务
			if err := tx.Where("task_id IN ?", taskIDs).Delete(&models.Task{}).Error; err != nil {
				return fmt.Errorf("删除任务失败: %v", err)
			}
		}

		// 4. 删除执行历史
		if err := tx.Where("scheduled_task_id = ?", id).Delete(&models.ScheduledTaskExecution{}).Error; err != nil {
			return fmt.Errorf("删除执行历史失败: %v", err)
		}

		// 5. 删除定时任务
		if err := tx.Delete(&scheduledTask).Error; err != nil {
			return fmt.Errorf("删除定时任务失败: %v", err)
		}

		logger.GetLogger().Infof("成功删除定时任务 [%s] 及其 %d 个相关任务", scheduledTask.Name, len(taskIDs))
		return nil
	})
}

// EnableScheduledTask 启用定时任务
func (s *TaskSchedulerService) EnableScheduledTask(id, tenantID uint) error {
	var scheduledTask models.ScheduledTask
	if err := s.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&scheduledTask).Error; err != nil {
		return fmt.Errorf("定时任务不存在")
	}

	if scheduledTask.IsActive {
		return nil // 已经启用
	}

	// 更新状态
	updates := map[string]interface{}{
		"is_active":  true,
		"updated_at": time.Now(),
	}

	// 计算下次运行时间
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(scheduledTask.CronExpr)
	if err == nil {
		updates["next_run_at"] = s.calculateNextRun(schedule)
	}

	if err := s.db.Model(&scheduledTask).Updates(updates).Error; err != nil {
		return fmt.Errorf("启用定时任务失败: %v", err)
	}

	// 添加到调度器
	s.db.First(&scheduledTask, scheduledTask.ID) // 重新加载
	if err := s.addJob(&scheduledTask); err != nil {
		return fmt.Errorf("添加调度任务失败: %v", err)
	}

	return nil
}

// DisableScheduledTask 禁用定时任务
func (s *TaskSchedulerService) DisableScheduledTask(id, tenantID uint) error {
	var scheduledTask models.ScheduledTask
	if err := s.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&scheduledTask).Error; err != nil {
		return fmt.Errorf("定时任务不存在")
	}

	if !scheduledTask.IsActive {
		return nil // 已经禁用
	}

	// 检查是否正在执行
	if scheduledTask.LastStatus == models.ScheduledTaskStatusRunning {
		return fmt.Errorf("任务正在执行中，无法禁用")
	}

	// 从调度器移除
	s.removeJob(id)

	// 更新状态
	updates := map[string]interface{}{
		"is_active":   false,
		"next_run_at": nil,
		"updated_at":  time.Now(),
	}

	if err := s.db.Model(&scheduledTask).Updates(updates).Error; err != nil {
		return fmt.Errorf("禁用定时任务失败: %v", err)
	}

	return nil
}

// RunScheduledTaskNow 立即执行定时任务
func (s *TaskSchedulerService) RunScheduledTaskNow(id, tenantID, userID uint) (*models.Task, error) {
	var scheduledTask models.ScheduledTask
	if err := s.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&scheduledTask).Error; err != nil {
		return nil, fmt.Errorf("定时任务不存在")
	}

	// 检查是否正在执行
	if scheduledTask.LastStatus == models.ScheduledTaskStatusRunning {
		return nil, fmt.Errorf("任务正在执行中")
	}

	// 手动触发执行
	task, err := s.executeScheduledTask(&scheduledTask, userID)
	if err != nil {
		return nil, fmt.Errorf("执行任务失败: %v", err)
	}

	return task, nil
}

// GetByID 获取定时任务详情
func (s *TaskSchedulerService) GetByID(id, tenantID uint) (*models.ScheduledTask, error) {
	var scheduledTask models.ScheduledTask
	err := s.db.Preload("Template").Where("id = ? AND tenant_id = ?", id, tenantID).First(&scheduledTask).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("定时任务不存在")
		}
		return nil, err
	}
	return &scheduledTask, nil
}

// List 获取定时任务列表
func (s *TaskSchedulerService) List(tenantID uint, page, pageSize int, filters map[string]interface{}) ([]models.ScheduledTask, int64, error) {
	query := s.db.Model(&models.ScheduledTask{}).Where("tenant_id = ?", tenantID)

	// 应用过滤条件
	if isActive, ok := filters["is_active"]; ok {
		query = query.Where("is_active = ?", isActive)
	}
	if templateID, ok := filters["template_id"]; ok {
		query = query.Where("template_id = ?", templateID)
	}
	if status, ok := filters["status"]; ok {
		query = query.Where("last_status = ?", status)
	}
	if name, ok := filters["name"]; ok {
		query = query.Where("name LIKE ?", "%"+name.(string)+"%")
	}

	var total int64
	query.Count(&total)

	// 计算分页
	offset := (page - 1) * pageSize

	var tasks []models.ScheduledTask
	err := query.Preload("Template").
		Offset(offset).Limit(pageSize).
		Order("created_at DESC").
		Find(&tasks).Error

	return tasks, total, err
}

// GetExecutionHistory 获取执行历史
func (s *TaskSchedulerService) GetExecutionHistory(scheduledTaskID, tenantID uint, page, pageSize int) ([]models.ScheduledTaskExecution, int64, error) {
	// 验证任务归属
	var count int64
	s.db.Model(&models.ScheduledTask{}).Where("id = ? AND tenant_id = ?", scheduledTaskID, tenantID).Count(&count)
	if count == 0 {
		return nil, 0, fmt.Errorf("定时任务不存在")
	}

	query := s.db.Model(&models.ScheduledTaskExecution{}).Where("scheduled_task_id = ?", scheduledTaskID)

	var total int64
	query.Count(&total)

	offset := (page - 1) * pageSize

	var executions []models.ScheduledTaskExecution
	err := query.Preload("Task").
		Order("triggered_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&executions).Error

	return executions, total, err
}

// GetTaskLogs 获取定时任务的执行日志
func (s *TaskSchedulerService) GetTaskLogs(scheduledTaskID, tenantID uint, page, pageSize int, filters map[string]interface{}) ([]models.TaskLog, int64, error) {
	// 1. 验证定时任务归属
	var scheduledTask models.ScheduledTask
	if err := s.db.Where("id = ? AND tenant_id = ?", scheduledTaskID, tenantID).First(&scheduledTask).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, 0, fmt.Errorf("定时任务不存在")
		}
		return nil, 0, err
	}

	// 2. 获取该定时任务的所有执行历史
	var executions []models.ScheduledTaskExecution
	executionQuery := s.db.Model(&models.ScheduledTaskExecution{}).Where("scheduled_task_id = ?", scheduledTaskID)
	
	// 如果指定了特定的执行ID
	if executionID, ok := filters["execution_id"].(uint); ok && executionID > 0 {
		executionQuery = executionQuery.Where("id = ?", executionID)
	}
	
	// 获取所有相关的TaskID
	var taskIDs []string
	if err := executionQuery.Pluck("task_id", &taskIDs).Error; err != nil {
		return nil, 0, fmt.Errorf("获取任务ID列表失败: %v", err)
	}
	
	if len(taskIDs) == 0 {
		return []models.TaskLog{}, 0, nil
	}

	// 3. 构建日志查询
	query := s.db.Model(&models.TaskLog{}).Where("task_id IN ?", taskIDs)
	
	// 应用过滤条件
	if level, ok := filters["level"].(string); ok && level != "" {
		query = query.Where("level = ?", level)
	}
	
	if host, ok := filters["host"].(string); ok && host != "" {
		query = query.Where("host_name = ?", host)
	}
	
	if keyword, ok := filters["keyword"].(string); ok && keyword != "" {
		query = query.Where("message LIKE ?", "%"+keyword+"%")
	}
	
	// 时间范围过滤
	if startTime, ok := filters["start_time"].(string); ok && startTime != "" {
		query = query.Where("timestamp >= ?", startTime)
	}
	
	if endTime, ok := filters["end_time"].(string); ok && endTime != "" {
		query = query.Where("timestamp <= ?", endTime)
	}

	// 4. 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计日志数量失败: %v", err)
	}

	// 5. 分页查询
	offset := (page - 1) * pageSize
	var logs []models.TaskLog
	err := query.Order("timestamp DESC").
		Offset(offset).Limit(pageSize).
		Find(&logs).Error

	if err != nil {
		return nil, 0, fmt.Errorf("查询日志失败: %v", err)
	}

	return logs, total, nil
}

// 内部方法

// addJob 添加任务到调度器
func (s *TaskSchedulerService) addJob(scheduledTask *models.ScheduledTask) error {
	if !scheduledTask.IsActive {
		return nil
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(scheduledTask.CronExpr)
	if err != nil {
		return fmt.Errorf("无效的cron表达式: %v", err)
	}

	// 创建任务函数
	jobFunc := func() {
		s.executeScheduledTaskAsync(scheduledTask.ID)
	}

	// 添加到cron调度器
	entryID, err := s.cron.AddFunc(scheduledTask.CronExpr, jobFunc)
	if err != nil {
		return fmt.Errorf("添加定时任务失败: %v", err)
	}

	// 记录任务ID
	s.mu.Lock()
	s.jobs[scheduledTask.ID] = entryID
	s.mu.Unlock()

	logger.GetLogger().Infof("已添加定时任务 [%s] (ID: %d)，cron: %s", scheduledTask.Name, scheduledTask.ID, scheduledTask.CronExpr)
	return nil
}

// removeJob 从调度器移除任务
func (s *TaskSchedulerService) removeJob(scheduledTaskID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, exists := s.jobs[scheduledTaskID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobs, scheduledTaskID)
		logger.GetLogger().Infof("已移除定时任务 ID: %d", scheduledTaskID)
	}
}

// executeScheduledTaskAsync 异步执行定时任务（cron触发）
func (s *TaskSchedulerService) executeScheduledTaskAsync(scheduledTaskID uint) {
	task, err := s.executeScheduledTask(&models.ScheduledTask{ID: scheduledTaskID}, 0)
	if err != nil {
		logger.GetLogger().Errorf("执行定时任务失败 [ID: %d]: %v", scheduledTaskID, err)
		return
	}
	logger.GetLogger().Infof("定时任务 [ID: %d] 触发成功，创建任务: %s", scheduledTaskID, task.TaskID)
}

// executeScheduledTask 执行定时任务
func (s *TaskSchedulerService) executeScheduledTask(scheduledTask *models.ScheduledTask, manualUserID uint) (*models.Task, error) {
	logger := logger.GetLogger().WithField("scheduled_task_id", scheduledTask.ID)

	// 1. 重新加载最新状态
	var st models.ScheduledTask
	if err := s.db.First(&st, scheduledTask.ID).Error; err != nil {
		return nil, fmt.Errorf("加载定时任务失败: %v", err)
	}

	// 2. 检查是否启用（手动执行时跳过）
	if manualUserID == 0 && !st.IsActive {
		logger.Info("定时任务已禁用，跳过执行")
		return nil, fmt.Errorf("定时任务已禁用")
	}

	// 3. 检查是否正在执行
	if st.LastStatus == models.ScheduledTaskStatusRunning {
		logger.Warn("定时任务正在执行中，跳过本次触发")
		return nil, fmt.Errorf("任务正在执行中")
	}

	// 4. 更新状态为运行中
	s.db.Model(&st).Updates(map[string]interface{}{
		"last_status": models.ScheduledTaskStatusRunning,
		"last_run_at": time.Now(),
	})

	// 5. 验证任务模板
	var template models.TaskTemplate
	if err := s.db.First(&template, st.TemplateID).Error; err != nil {
		s.updateTaskStatus(st.ID, models.ScheduledTaskStatusFailed, "")
		return nil, fmt.Errorf("任务模板不存在: %v", err)
	}

	if template.Status != "active" {
		s.updateTaskStatus(st.ID, models.ScheduledTaskStatusFailed, "")
		return nil, fmt.Errorf("任务模板未激活")
	}

	// 6. 解析配置
	var hostIDs []uint
	if err := json.Unmarshal(st.HostIDs, &hostIDs); err != nil {
		s.updateTaskStatus(st.ID, models.ScheduledTaskStatusFailed, "")
		return nil, fmt.Errorf("解析主机列表失败: %v", err)
	}

	var variables map[string]interface{}
	if len(st.Variables) > 0 {
		if err := json.Unmarshal(st.Variables, &variables); err != nil {
			s.updateTaskStatus(st.ID, models.ScheduledTaskStatusFailed, "")
			return nil, fmt.Errorf("解析变量失败: %v", err)
		}
	}

	// 7. 创建任务实例
	taskName := fmt.Sprintf("[定时] %s", st.Name)
	if manualUserID > 0 {
		taskName = fmt.Sprintf("[手动] %s", st.Name)
	}

	task := &models.Task{
		TenantID:    st.TenantID,
		TaskType:    models.TaskTypeTemplate,
		Name:        taskName,
		Description: fmt.Sprintf("定时任务 #%d 触发执行", st.ID),
		Priority:    5,
		Timeout:     st.TimeoutMins * 60,
		Source:      "schedule",
		CreatedBy:   st.CreatedBy,
		Username:    "定时调度",
	}

	if manualUserID > 0 {
		task.Source = "manual"
		task.CreatedBy = manualUserID
		// 获取用户名
		var user models.User
		if s.db.Select("username").First(&user, manualUserID).Error == nil {
			task.Username = user.Username
		}
	}

	// 8. 调用现有的模板任务创建方法
	err := s.taskService.CreateTemplateTask(task, st.TemplateID, variables, hostIDs)
	if err != nil {
		s.updateTaskStatus(st.ID, models.ScheduledTaskStatusFailed, "")
		return nil, fmt.Errorf("创建任务失败: %v", err)
	}

	// 9. 记录执行历史
	execution := &models.ScheduledTaskExecution{
		ScheduledTaskID: st.ID,
		TaskID:          task.TaskID,
		TriggeredAt:     time.Now(),
	}
	s.db.Create(execution)

	// 10. 更新任务信息
	s.db.Model(&st).Updates(map[string]interface{}{
		"last_task_id": task.TaskID,
		"run_count":    gorm.Expr("run_count + 1"),
	})

	// 11. 异步监控任务完成状态
	go s.monitorTaskCompletion(st.ID, task.TaskID)

	logger.Infof("定时任务触发成功，创建任务: %s", task.TaskID)
	return task, nil
}

// monitorTaskCompletion 监控任务完成状态
func (s *TaskSchedulerService) monitorTaskCompletion(scheduledTaskID uint, taskID string) {
	// 每30秒检查一次，最多检查4小时
	maxAttempts := 480
	for i := 0; i < maxAttempts; i++ {
		time.Sleep(30 * time.Second)

		var task models.Task
		if err := s.db.Where("task_id = ?", taskID).First(&task).Error; err != nil {
			continue
		}

		// 任务已完成
		if task.Status == "success" || task.Status == "failed" || task.Status == "timeout" || task.Status == "cancelled" {
			status := models.ScheduledTaskStatusSuccess
			if task.Status != "success" {
				status = models.ScheduledTaskStatusFailed
			}

			s.updateTaskStatus(scheduledTaskID, status, taskID)

			// 计算下次执行时间
			s.updateNextRunTime(scheduledTaskID)
			return
		}
	}

	// 超时，标记为失败
	s.updateTaskStatus(scheduledTaskID, models.ScheduledTaskStatusFailed, taskID)
	s.updateNextRunTime(scheduledTaskID)
}

// updateTaskStatus 更新任务状态
func (s *TaskSchedulerService) updateTaskStatus(scheduledTaskID uint, status string, taskID string) {
	updates := map[string]interface{}{
		"last_status": status,
		"updated_at":  time.Now(),
	}
	if taskID != "" {
		updates["last_task_id"] = taskID
	}
	s.db.Model(&models.ScheduledTask{}).Where("id = ?", scheduledTaskID).Updates(updates)
}

// updateNextRunTime 更新下次运行时间
func (s *TaskSchedulerService) updateNextRunTime(scheduledTaskID uint) {
	var scheduledTask models.ScheduledTask
	if err := s.db.First(&scheduledTask, scheduledTaskID).Error; err != nil {
		return
	}

	if !scheduledTask.IsActive {
		return
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(scheduledTask.CronExpr)
	if err != nil {
		return
	}

	nextRun := s.calculateNextRun(schedule)
	s.db.Model(&scheduledTask).Update("next_run_at", nextRun)
}

// calculateNextRun 计算下次运行时间
func (s *TaskSchedulerService) calculateNextRun(schedule cron.Schedule) *time.Time {
	next := schedule.Next(time.Now())
	return &next
}

// 请求结构体

// CreateScheduledTaskRequest 创建定时任务请求
type CreateScheduledTaskRequest struct {
	Name        string                 `json:"name" binding:"required,min=1,max=200"`
	Description string                 `json:"description" binding:"max=500"`
	CronExpr    string                 `json:"cron_expr" binding:"required"`
	TemplateID  uint                   `json:"template_id" binding:"required"`
	HostIDs     []uint                 `json:"host_ids" binding:"required,min=1"`
	Variables   map[string]interface{} `json:"variables"`
	TimeoutMins int                    `json:"timeout_mins" binding:"min=1,max=1440"`
	IsActive    bool                   `json:"is_active"`
}

// UpdateScheduledTaskRequest 更新定时任务请求
type UpdateScheduledTaskRequest struct {
	Name        string                 `json:"name" binding:"omitempty,min=1,max=200"`
	Description *string                `json:"description" binding:"omitempty,max=500"`
	CronExpr    string                 `json:"cron_expr" binding:"omitempty"`
	HostIDs     []uint                 `json:"host_ids" binding:"omitempty,min=1"`
	Variables   map[string]interface{} `json:"variables"`
	TimeoutMins int                    `json:"timeout_mins" binding:"omitempty,min=1,max=1440"`
}