package main

import (
	"ahop/internal/database"
	"ahop/internal/models"
	"ahop/pkg/logger"
	"fmt"

	"gorm.io/gorm"
)

// seedData 初始化种子数据
func seedData() error {
	appLogger := logger.GetLogger()
	appLogger.Info("Starting seed data initialization...")

	db := database.GetDB()

	// 1. 创建默认租户
	if err := createDefaultTenant(db); err != nil {
		return fmt.Errorf("创建默认租户失败: %v", err)
	}

	// 2. 初始化权限
	if err := initializePermissions(db); err != nil {
		return fmt.Errorf("初始化权限失败: %v", err)
	}

	// 3. 创建平台管理员角色
	if err := createPlatformAdminRole(db); err != nil {
		return fmt.Errorf("创建平台管理员角色失败: %v", err)
	}

	// 4. 创建默认管理员用户
	if err := createDefaultAdmin(db); err != nil {
		return fmt.Errorf("创建默认管理员失败: %v", err)
	}

	appLogger.Info("Seed data initialization completed successfully")
	return nil
}

// createDefaultTenant 创建默认租户
func createDefaultTenant(db *gorm.DB) error {
	var count int64
	db.Model(&models.Tenant{}).Where("code = ?", "default").Count(&count)
	if count > 0 {
		logger.GetLogger().Info("默认租户已存在，跳过创建")
		return nil
	}

	tenant := &models.Tenant{
		Name:   "默认租户",
		Code:   "default",
		Status: models.TenantStatusActive,
	}

	if err := db.Create(tenant).Error; err != nil {
		return err
	}

	logger.GetLogger().Info("默认租户创建成功")
	return nil
}

// initializePermissions 初始化权限
func initializePermissions(db *gorm.DB) error {
	// 定义默认权限
	defaultPermissions := []models.Permission{
		// 租户管理权限
		{Code: "tenant:create", Name: "创建租户", Module: "tenant", Action: "create", Description: "创建新租户"},
		{Code: "tenant:read", Name: "查看租户", Module: "tenant", Action: "read", Description: "查看租户信息"},
		{Code: "tenant:update", Name: "更新租户", Module: "tenant", Action: "update", Description: "更新租户信息"},
		{Code: "tenant:delete", Name: "删除租户", Module: "tenant", Action: "delete", Description: "删除租户"},
		{Code: "tenant:list", Name: "租户列表", Module: "tenant", Action: "list", Description: "查看租户列表"},

		// 用户管理权限
		{Code: "user:create", Name: "创建用户", Module: "user", Action: "create", Description: "创建新用户"},
		{Code: "user:read", Name: "查看用户", Module: "user", Action: "read", Description: "查看用户信息"},
		{Code: "user:update", Name: "更新用户", Module: "user", Action: "update", Description: "更新用户信息"},
		{Code: "user:delete", Name: "删除用户", Module: "user", Action: "delete", Description: "删除用户"},
		{Code: "user:list", Name: "用户列表", Module: "user", Action: "list", Description: "查看用户列表"},

		// 角色管理权限
		{Code: "role:create", Name: "创建角色", Module: "role", Action: "create", Description: "创建新角色"},
		{Code: "role:read", Name: "查看角色", Module: "role", Action: "read", Description: "查看角色信息"},
		{Code: "role:update", Name: "更新角色", Module: "role", Action: "update", Description: "更新角色信息"},
		{Code: "role:delete", Name: "删除角色", Module: "role", Action: "delete", Description: "删除角色"},
		{Code: "role:list", Name: "角色列表", Module: "role", Action: "list", Description: "查看角色列表"},
		{Code: "role:assign", Name: "分配角色", Module: "role", Action: "assign", Description: "给用户分配角色"},

		// 权限管理权限
		{Code: "permission:read", Name: "查看权限", Module: "permission", Action: "read", Description: "查看权限信息"},
		{Code: "permission:list", Name: "权限列表", Module: "permission", Action: "list", Description: "查看权限列表"},
		{Code: "permission:assign", Name: "分配权限", Module: "permission", Action: "assign", Description: "给角色分配权限"},

		// 凭证管理权限
		{Code: "credential:create", Name: "创建凭证", Module: "credential", Action: "create", Description: "创建新凭证"},
		{Code: "credential:read", Name: "查看凭证", Module: "credential", Action: "read", Description: "查看凭证信息"},
		{Code: "credential:update", Name: "更新凭证", Module: "credential", Action: "update", Description: "更新凭证信息"},
		{Code: "credential:delete", Name: "删除凭证", Module: "credential", Action: "delete", Description: "删除凭证"},
		{Code: "credential:list", Name: "凭证列表", Module: "credential", Action: "list", Description: "查看凭证列表"},
		{Code: "credential:decrypt", Name: "解密凭证", Module: "credential", Action: "decrypt", Description: "获取凭证明文"},

		// 标签管理权限
		{Code: "tag:list", Name: "查看标签列表", Module: "tag", Action: "list", Description: "查看标签列表"},
		{Code: "tag:read", Name: "查看标签详情", Module: "tag", Action: "read", Description: "查看标签详情"},
		{Code: "tag:create", Name: "创建标签", Module: "tag", Action: "create", Description: "创建新标签"},
		{Code: "tag:update", Name: "更新标签", Module: "tag", Action: "update", Description: "更新标签信息"},
		{Code: "tag:delete", Name: "删除标签", Module: "tag", Action: "delete", Description: "删除标签"},

		// 主机管理权限
		{Code: "host:list", Name: "查看主机列表", Module: "host", Action: "list", Description: "查看主机列表"},
		{Code: "host:read", Name: "查看主机详情", Module: "host", Action: "read", Description: "查看主机详情"},
		{Code: "host:create", Name: "创建主机", Module: "host", Action: "create", Description: "创建新主机"},
		{Code: "host:update", Name: "更新主机", Module: "host", Action: "update", Description: "更新主机信息"},
		{Code: "host:delete", Name: "删除主机", Module: "host", Action: "delete", Description: "删除主机"},
		{Code: "host:execute", Name: "主机执行", Module: "host", Action: "execute", Description: "在主机上执行任务"},
		{Code: "host:import", Name: "批量导入主机", Module: "host", Action: "import", Description: "批量导入主机"},
		{Code: "host:export", Name: "批量导出主机", Module: "host", Action: "export", Description: "批量导出主机"},

		// 主机组管理权限
		{Code: "host_group:list", Name: "查看主机组列表", Module: "host_group", Action: "list", Description: "查看主机组列表"},
		{Code: "host_group:read", Name: "查看主机组详情", Module: "host_group", Action: "read", Description: "查看主机组详情"},
		{Code: "host_group:create", Name: "创建主机组", Module: "host_group", Action: "create", Description: "创建新主机组"},
		{Code: "host_group:update", Name: "更新主机组", Module: "host_group", Action: "update", Description: "更新主机组信息"},
		{Code: "host_group:delete", Name: "删除主机组", Module: "host_group", Action: "delete", Description: "删除主机组"},
		{Code: "host_group:move", Name: "移动主机组", Module: "host_group", Action: "move", Description: "移动主机组到其他位置"},
		{Code: "host_group:manage_hosts", Name: "管理组内主机", Module: "host_group", Action: "manage_hosts", Description: "在主机组中添加或移除主机"},

		// 任务管理权限
		{Code: "task:list", Name: "查看任务列表", Module: "task", Action: "list", Description: "查看任务列表"},
		{Code: "task:read", Name: "查看任务详情", Module: "task", Action: "read", Description: "查看任务详情"},
		{Code: "task:create", Name: "创建任务", Module: "task", Action: "create", Description: "创建新任务"},
		{Code: "task:cancel", Name: "取消任务", Module: "task", Action: "cancel", Description: "取消正在执行的任务"},
		{Code: "task:retry", Name: "重试任务", Module: "task", Action: "retry", Description: "重新执行失败的任务"},
		{Code: "task:logs", Name: "查看任务日志", Module: "task", Action: "logs", Description: "查看任务执行日志"},

		// 任务统计权限
		{Code: "task:stats", Name: "查看任务统计", Module: "task", Action: "stats", Description: "查看任务队列统计信息"},

		// Worker管理权限
		{Code: "worker:list", Name: "查看Worker列表", Module: "worker", Action: "list", Description: "查看工作节点列表和统计信息"},
		{Code: "worker:read", Name: "查看Worker详情", Module: "worker", Action: "read", Description: "查看工作节点详情信息"},
		{Code: "worker:delete", Name: "删除Worker", Module: "worker", Action: "delete", Description: "删除离线的工作节点"},
		{Code: "worker:queue", Name: "查看任务队列", Module: "worker", Action: "queue", Description: "查看任务队列统计信息"},

		// 网络扫描权限
		{Code: "network_scan:start", Name: "开始网络扫描", Module: "network_scan", Action: "start", Description: "启动网络扫描任务"},
		{Code: "network_scan:view", Name: "查看扫描结果", Module: "network_scan", Action: "view", Description: "查看网络扫描状态和结果"},
		{Code: "network_scan:cancel", Name: "取消网络扫描", Module: "network_scan", Action: "cancel", Description: "取消正在执行的扫描任务"},
		{Code: "network_scan:import", Name: "导入扫描主机", Module: "network_scan", Action: "import", Description: "将扫描发现的主机导入系统"},
	}

	// 批量创建权限
	for _, perm := range defaultPermissions {
		var count int64
		db.Model(&models.Permission{}).Where("code = ?", perm.Code).Count(&count)
		if count == 0 {
			if err := db.Create(&perm).Error; err != nil {
				return fmt.Errorf("创建权限 %s 失败: %v", perm.Code, err)
			}
		}
	}

	logger.GetLogger().Info("权限初始化完成")
	return nil
}

// createPlatformAdminRole 创建平台管理员角色
func createPlatformAdminRole(db *gorm.DB) error {
	var count int64
	db.Model(&models.Role{}).Where("code = ?", "platform_admin").Count(&count)
	if count > 0 {
		logger.GetLogger().Info("平台管理员角色已存在，跳过创建")
		return nil
	}

	// 获取默认租户
	var tenant models.Tenant
	if err := db.Where("code = ?", "default").First(&tenant).Error; err != nil {
		return fmt.Errorf("获取默认租户失败: %v", err)
	}

	// 创建角色
	role := &models.Role{
		TenantID:    tenant.ID, // 平台管理员角色也需要属于一个租户
		Name:        "平台管理员",
		Code:        "platform_admin",
		Description: "系统最高权限管理员",
		IsSystem:    true, // 标记为系统角色
	}

	if err := db.Create(role).Error; err != nil {
		return err
	}

	// 分配所有权限
	var permissions []models.Permission
	db.Find(&permissions)

	var rolePermissions []models.RolePermission
	for _, perm := range permissions {
		rolePermissions = append(rolePermissions, models.RolePermission{
			RoleID:       role.ID,
			PermissionID: perm.ID,
		})
	}

	if len(rolePermissions) > 0 {
		if err := db.Create(&rolePermissions).Error; err != nil {
			return err
		}
	}

	logger.GetLogger().Info("平台管理员角色创建成功")
	return nil
}

// createDefaultAdmin 创建默认管理员用户
func createDefaultAdmin(db *gorm.DB) error {
	var count int64
	db.Model(&models.User{}).Where("username = ?", "admin").Count(&count)
	if count > 0 {
		logger.GetLogger().Info("管理员用户已存在，跳过创建")
		return nil
	}

	// 获取默认租户
	var tenant models.Tenant
	if err := db.Where("code = ?", "default").First(&tenant).Error; err != nil {
		return fmt.Errorf("获取默认租户失败: %v", err)
	}

	// 创建用户
	user := &models.User{
		TenantID:        tenant.ID,
		Username:        "admin",
		Email:           "admin@example.com",
		Name:            "系统管理员",
		Status:          models.UserStatusActive,
		IsPlatformAdmin: true,
		IsTenantAdmin:   true,
	}

	// 设置密码
	if err := user.SetPassword("Admin@123"); err != nil {
		return fmt.Errorf("设置密码失败: %v", err)
	}

	if err := db.Create(user).Error; err != nil {
		return err
	}

	// 分配平台管理员角色
	var role models.Role
	if err := db.Where("code = ?", "platform_admin").First(&role).Error; err == nil {
		userRole := &models.UserRole{
			UserID:    user.ID,
			RoleID:    role.ID,
			CreatedBy: user.ID,
		}
		db.Create(userRole)
	}

	logger.GetLogger().Infof("默认管理员创建成功 - 用户名: admin, 密码: Admin@123")
	return nil
}
