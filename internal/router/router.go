package router

import (
	"ahop/internal/database"
	"ahop/internal/handlers"
	"ahop/internal/middleware"
	"ahop/internal/services"
	"ahop/pkg/response"
	"time"

	"github.com/gin-gonic/gin"
)

// SetupRouter 设置路由
func SetupRouter() *gin.Engine {
	router := gin.New()

	// 中间件
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.ErrorHandler())
	router.Use(middleware.SetupCORS())

	// 注册路由
	registerRoutes(router)
	return router
}

// 注册所有路由
func registerRoutes(router *gin.Engine) {

	auth := middleware.NewAuthMiddleware()

	// API路由组
	api := router.Group("/api/v1")
	{
		// 健康检查接口
		api.GET("/health", healthCheck)
		api.GET("/ping", ping)

		// 🆕 JWT认证路由（无需认证）
		authHandler := handlers.NewAuthHandler(services.NewUserService(), services.NewTenantService())
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/login", authHandler.Login)          // 用户登录
			authGroup.POST("/logout", authHandler.Logout)        // 用户登出
			authGroup.POST("/refresh", authHandler.RefreshToken) // 刷新Token

			// 🔒 获取当前用户完整信息
			authGroup.GET("/me", auth.RequireLogin(), authHandler.Me)

			// 🔒 租户切换（仅平台管理员）
			authGroup.POST("/switch-tenant", auth.RequireLogin(), authHandler.SwitchTenant)
		}

		// 用户路由（添加权限保护）
		userHandler := handlers.NewUserHandler()
		users := api.Group("/users")
		{
			// 🔒 基础CRUD（添加权限保护）
			users.POST("", auth.RequireLogin(), auth.RequirePermission("user:create"), userHandler.Create)
			users.GET("", auth.RequireLogin(), auth.RequirePermission("user:list"), userHandler.GetAll)
			users.GET("/:id", auth.RequireLogin(), auth.RequireOwnerOrAdmin(), userHandler.GetByID) // 个人信息查看，只需登录
			users.PUT("/:id", auth.RequireLogin(), auth.RequireOwnerOrAdmin(), userHandler.Update)  // 用户可以改自己，管理员可以改任何人
			users.DELETE("/:id", auth.RequireLogin(), auth.RequirePermission("user:delete"), userHandler.Delete)

			// 🔒 快捷操作（需要用户管理权限）
			users.POST("/:id/activate", auth.RequireLogin(), auth.RequirePermission("user:update"), userHandler.Activate)
			users.POST("/:id/deactivate", auth.RequireLogin(), auth.RequirePermission("user:update"), userHandler.Deactivate)
			users.POST("/:id/lock", auth.RequireLogin(), auth.RequirePermission("user:update"), userHandler.Lock)
			users.POST("/:id/reset-password", auth.RequireLogin(), auth.RequireOwnerOrAdmin(), userHandler.ResetPassword)

			// 🔒 查询接口（需要读取权限）
			users.GET("/username/:username", auth.RequireLogin(), auth.RequirePermission("user:read"), userHandler.GetByUsername)
			users.GET("/email", auth.RequireLogin(), auth.RequirePermission("user:read"), userHandler.GetByEmail)

			// 🔒 统计接口（管理员权限）
			users.GET("/stats", auth.RequireLogin(), auth.RequireTenantAdmin(), userHandler.GetStats)
			users.GET("/recent", auth.RequireLogin(), auth.RequireTenantAdmin(), userHandler.GetRecentlyCreated)
			users.GET("/status-distribution", auth.RequireLogin(), auth.RequireTenantAdmin(), userHandler.GetStatusDistribution)

			// 🔒 用户角色管理（用户可以查看自己的，管理员可以管理任何人）
			users.POST("/:id/roles", auth.RequireLogin(), auth.RequireOwnerOrAdmin(), userHandler.AssignRoles)
			users.POST("/:id/roles/add", auth.RequireLogin(), auth.RequireOwnerOrAdmin(), userHandler.AddRole)
			users.DELETE("/:id/roles/:role_id", auth.RequireLogin(), auth.RequireOwnerOrAdmin(), userHandler.RemoveRole)
			users.GET("/:id/roles", auth.RequireLogin(), auth.RequireOwnerOrAdmin(), userHandler.GetUserRoles)
			users.GET("/:id/permissions", auth.RequireLogin(), auth.RequireOwnerOrAdmin(), userHandler.GetUserPermissions)

			// 🔒 权限检查（用户可以检查自己，管理员可以检查任何人）
			users.GET("/:id/check-permission", auth.RequireLogin(), auth.RequireOwnerOrAdmin(), userHandler.CheckPermission)
			users.GET("/:id/check-role", auth.RequireLogin(), auth.RequireOwnerOrAdmin(), userHandler.CheckRole)
		}

		// 🔐 租户路由（添加权限保护）
		tenantHandler := handlers.NewTenantHandler(services.NewTenantService())
		tenants := api.Group("/tenants")
		{
			// 🔒 基础CRUD（需要相应权限）
			tenants.POST("", auth.RequireLogin(), auth.RequirePermission("tenant:create"), tenantHandler.Create)
			tenants.GET("", auth.RequireLogin(), auth.RequirePermission("tenant:list"), tenantHandler.GetAll)
			tenants.GET("/:id", auth.RequireLogin(), auth.RequirePermission("tenant:read"), tenantHandler.GetByID)
			tenants.PUT("/:id", auth.RequireLogin(), auth.RequirePermission("tenant:update"), tenantHandler.Update)
			tenants.DELETE("/:id", auth.RequireLogin(), auth.RequirePermission("tenant:delete"), tenantHandler.Delete)

			// 🔒 租户管理操作（需要租户管理权限）
			tenants.POST("/:id/activate", auth.RequireLogin(), auth.RequirePermission("tenant:update"), tenantHandler.Activate)
			tenants.POST("/:id/deactivate", auth.RequireLogin(), auth.RequirePermission("tenant:update"), tenantHandler.Deactivate)

			// 🔒 统计功能（平台管理员专用）
			tenants.GET("/stats", auth.RequireLogin(), auth.RequirePlatformAdmin(), tenantHandler.GetStats)
			tenants.GET("/recent", auth.RequireLogin(), auth.RequirePlatformAdmin(), tenantHandler.GetRecentlyCreated)
			tenants.GET("/status-distribution", auth.RequireLogin(), auth.RequirePlatformAdmin(), tenantHandler.GetStatusDistribution)
		}

		// 🔐 角色路由（添加权限保护）
		roleHandler := handlers.NewRoleHandler(services.NewRoleService())
		roles := api.Group("/roles")
		{
			// 🔒 基础CRUD（需要角色管理权限）
			roles.POST("", auth.RequireLogin(), auth.RequirePermission("role:create"), roleHandler.Create)
			roles.GET("/:id", auth.RequireLogin(), auth.RequirePermission("role:read"), roleHandler.GetByID)
			roles.PUT("/:id", auth.RequireLogin(), auth.RequirePermission("role:update"), roleHandler.Update)
			roles.DELETE("/:id", auth.RequireLogin(), auth.RequirePermission("role:delete"), roleHandler.Delete)

			// 🔒 租户相关（需要权限 + 租户隔离）
			roles.GET("/tenant/:tenant_id", auth.RequireLogin(), auth.RequirePermission("role:list"), auth.RequireSameTenant(), roleHandler.GetByTenant)

			// 🔒 权限管理（租户管理员及以上）
			roles.POST("/:id/permissions", auth.RequireLogin(), auth.RequireTenantAdmin(), roleHandler.AssignPermissions)
			roles.GET("/:id/permissions", auth.RequireLogin(), auth.RequirePermission("role:read"), roleHandler.GetPermissions)
		}

		// 🔐 权限路由（部分保护）
		permissionHandler := handlers.NewPermissionHandler(services.NewPermissionService())
		permissions := api.Group("/permissions")
		{
			// 🔓 公开查看（任何人都可以查看有哪些权限）
			permissions.GET("", permissionHandler.GetAll)
			permissions.GET("/module/:module", permissionHandler.GetByModule)
		}

		// 🔐 凭证路由（添加权限保护）
		credentialHandler := handlers.NewCredentialHandler(services.NewCredentialService(database.GetDB()), services.NewTagService())
		credentials := api.Group("/credentials")
		{
			// 🔒 基础CRUD（需要凭证管理权限）
			credentials.POST("", auth.RequireLogin(), auth.RequirePermission("credential:create"), credentialHandler.Create)
			credentials.GET("", auth.RequireLogin(), auth.RequirePermission("credential:list"), credentialHandler.List)
			credentials.GET("/:id", auth.RequireLogin(), auth.RequirePermission("credential:read"), credentialHandler.Get)
			credentials.PUT("/:id", auth.RequireLogin(), auth.RequirePermission("credential:update"), credentialHandler.Update)
			credentials.DELETE("/:id", auth.RequireLogin(), auth.RequirePermission("credential:delete"), credentialHandler.Delete)

			// 🔒 获取解密凭证（需要特殊权限）
			credentials.GET("/:id/decrypt", auth.RequireLogin(), auth.RequirePermission("credential:decrypt"), credentialHandler.GetDecrypted)

			// 🔒 查看使用日志（需要读取权限）
			credentials.GET("/:id/logs", auth.RequireLogin(), auth.RequirePermission("credential:read"), credentialHandler.GetUsageLogs)

			// 🔒 标签管理
			credentials.GET("/:id/tags", auth.RequireLogin(), auth.RequireSameTenant(), credentialHandler.GetTags)
			credentials.PUT("/:id/tags", auth.RequireLogin(), auth.RequirePermission("credential:update"), credentialHandler.UpdateTags)
		}

		// 🔐 标签路由（添加权限保护）
		tagHandler := handlers.NewTagHandler(services.NewTagService())
		tags := api.Group("/tags")
		{
			// 🔒 基础CRUD（需要标签管理权限）
			tags.GET("", auth.RequireLogin(), auth.RequirePermission("tag:list"), tagHandler.GetAll)
			tags.GET("/:id", auth.RequireLogin(), auth.RequirePermission("tag:read"), tagHandler.GetByID)
			tags.POST("", auth.RequireLogin(), auth.RequirePermission("tag:create"), tagHandler.Create)
			tags.PUT("/:id", auth.RequireLogin(), auth.RequirePermission("tag:update"), tagHandler.Update)
			tags.DELETE("/:id", auth.RequireLogin(), auth.RequirePermission("tag:delete"), tagHandler.Delete)

			// 🔒 分组查询（需要列表权限）
			tags.GET("/grouped", auth.RequireLogin(), auth.RequirePermission("tag:list"), tagHandler.GetGroupedByKey)
		}

		// 🔐 主机组路由（添加权限保护）
		hostGroupHandler := handlers.NewHostGroupHandler(services.NewHostGroupService(database.GetDB()), services.NewHostService(database.GetDB()))
		hostGroups := api.Group("/host-groups")
		{
			// 🔒 树形结构查询
			hostGroups.GET("/tree", auth.RequireLogin(), auth.RequirePermission("host_group:list"), hostGroupHandler.GetTree)
			hostGroups.GET("/:id/tree", auth.RequireLogin(), auth.RequirePermission("host_group:read"), hostGroupHandler.GetSubTree)
			
			// 🔒 路径查询
			hostGroups.GET("/path", auth.RequireLogin(), auth.RequirePermission("host_group:read"), hostGroupHandler.GetByPath)
			hostGroups.GET("/:id/ancestors", auth.RequireLogin(), auth.RequirePermission("host_group:read"), hostGroupHandler.GetAncestors)
			hostGroups.GET("/:id/descendants", auth.RequireLogin(), auth.RequirePermission("host_group:read"), hostGroupHandler.GetDescendants)
			
			// 🔒 基础CRUD（需要主机组管理权限）
			hostGroups.POST("", auth.RequireLogin(), auth.RequirePermission("host_group:create"), hostGroupHandler.Create)
			hostGroups.GET("", auth.RequireLogin(), auth.RequirePermission("host_group:list"), hostGroupHandler.List)
			hostGroups.GET("/:id", auth.RequireLogin(), auth.RequirePermission("host_group:read"), hostGroupHandler.GetByID)
			hostGroups.PUT("/:id", auth.RequireLogin(), auth.RequirePermission("host_group:update"), hostGroupHandler.Update)
			hostGroups.DELETE("/:id", auth.RequireLogin(), auth.RequirePermission("host_group:delete"), hostGroupHandler.Delete)
			
			// 🔒 组移动（需要移动权限）
			hostGroups.POST("/:id/move", auth.RequireLogin(), auth.RequirePermission("host_group:move"), hostGroupHandler.Move)
			
			// 🔒 主机管理（需要管理主机权限）
			hostGroups.GET("/:id/hosts", auth.RequireLogin(), auth.RequirePermission("host_group:read"), hostGroupHandler.GetHosts)
			hostGroups.POST("/:id/hosts", auth.RequireLogin(), auth.RequirePermission("host_group:manage_hosts"), hostGroupHandler.AssignHosts)
			hostGroups.DELETE("/:id/hosts", auth.RequireLogin(), auth.RequirePermission("host_group:manage_hosts"), hostGroupHandler.RemoveHosts)
			hostGroups.PUT("/:id/hosts", auth.RequireLogin(), auth.RequirePermission("host_group:manage_hosts"), hostGroupHandler.UpdateHostsGroup)
		}

		// 🔐 主机路由（添加权限保护）
		hostHandler := handlers.NewHostHandler(services.NewHostService(database.GetDB()))
		hosts := api.Group("/hosts")
		{
			// 🔒 批量导入/导出
			hosts.GET("/template", auth.RequireLogin(), auth.RequirePermission("host:import"), hostHandler.DownloadTemplate)
			hosts.POST("/import", auth.RequireLogin(), auth.RequirePermission("host:import"), hostHandler.Import)
			hosts.GET("/export", auth.RequireLogin(), auth.RequirePermission("host:export"), hostHandler.Export)

			// 🔒 基础CRUD（需要主机管理权限）
			hosts.POST("", auth.RequireLogin(), auth.RequirePermission("host:create"), hostHandler.Create)
			hosts.GET("", auth.RequireLogin(), auth.RequirePermission("host:list"), hostHandler.List)
			hosts.GET("/:id", auth.RequireLogin(), auth.RequirePermission("host:read"), hostHandler.GetByID)
			hosts.PUT("/:id", auth.RequireLogin(), auth.RequirePermission("host:update"), hostHandler.Update)
			hosts.DELETE("/:id", auth.RequireLogin(), auth.RequirePermission("host:delete"), hostHandler.Delete)

			// 🔒 标签管理
			hosts.GET("/:id/tags", auth.RequireLogin(), auth.RequirePermission("host:read"), hostHandler.GetTags)
			hosts.PUT("/:id/tags", auth.RequireLogin(), auth.RequirePermission("host:update"), hostHandler.UpdateTags)

			// 🔒 未分组主机（放在动态路由之前）
			hosts.GET("/ungrouped", auth.RequireLogin(), auth.RequirePermission("host:list"), hostGroupHandler.GetUngroupedHosts)
			
			// 🔒 连接测试
			hosts.POST("/:id/test-connection", auth.RequireLogin(), auth.RequirePermission("host:read"), hostHandler.TestConnection)
			hosts.POST("/:id/test-ping", auth.RequireLogin(), auth.RequirePermission("host:read"), hostHandler.TestPing)
			
			// 🔒 主机组管理（需要主机更新权限）
			hosts.GET("/:id/groups", auth.RequireLogin(), auth.RequirePermission("host:read"), hostGroupHandler.GetHostGroups)
			hosts.PUT("/:id/group", auth.RequireLogin(), auth.RequirePermission("host:update"), hostGroupHandler.UpdateHostGroup)
		}

		// 🔐 任务路由（添加权限保护）
		taskHandler := handlers.NewTaskHandler(services.NewTaskService(database.GetDB(), database.GetRedisQueue()))
		tasks := api.Group("/tasks")
		{
			// 🔒 基础CRUD（需要任务管理权限）
			tasks.POST("", auth.RequireLogin(), auth.RequirePermission("task:create"), taskHandler.Create)
			tasks.GET("", auth.RequireLogin(), auth.RequirePermission("task:list"), taskHandler.List)
			tasks.GET("/:id", auth.RequireLogin(), auth.RequirePermission("task:read"), taskHandler.GetByID)
			tasks.POST("/:id/cancel", auth.RequireLogin(), auth.RequirePermission("task:cancel"), taskHandler.Cancel)

			// 🔒 日志查看
			tasks.GET("/:id/logs", auth.RequireLogin(), auth.RequirePermission("task:logs"), taskHandler.GetLogs)

			// 🔒 统计信息
			tasks.GET("/stats", auth.RequireLogin(), auth.RequirePermission("task:stats"), taskHandler.GetStats)
		}

		// 🔐 WebSocket路由（任务实时日志）
		wsHandler := handlers.NewWebSocketHandler(services.NewUserService(), services.NewTaskService(database.GetDB(), database.GetRedisQueue()))
		ws := api.Group("/ws")
		{
			// WebSocket连接不能使用常规的中间件，认证通过query参数处理
			ws.GET("/tasks/:id/logs", wsHandler.TaskLogs)
			
			// 网络扫描WebSocket路由
			ws.GET("/network-scan/:scan_id", wsHandler.NetworkScanResults)
		}

		// 🔐 Worker认证路由（无需认证，Worker使用AK/SK认证）
		workerAuthHandler := handlers.NewWorkerAuthHandler(services.NewWorkerAuthService(database.GetDB()))
		worker := api.Group("/worker")
		{
			// Worker认证接口（Worker使用AK/SK认证）
			worker.POST("/auth", workerAuthHandler.Authenticate)
			// Worker心跳接口（Worker使用AK/SK认证）
			worker.PUT("/heartbeat", workerAuthHandler.Heartbeat)
			// Worker主动断开连接接口（Worker使用AK/SK认证）
			worker.POST("/disconnect", workerAuthHandler.Disconnect)
		}

		// 🔐 Worker授权管理路由（管理员权限）
		workerAuths := api.Group("/admin/worker-auths")
		{
			// 🔒 创建Worker授权（需要平台管理员权限）
			workerAuths.POST("", auth.RequireLogin(), auth.RequirePlatformAdmin(), workerAuthHandler.CreateWorkerAuth)

			// 🔒 查看Worker授权列表（需要平台管理员权限）
			workerAuths.GET("", auth.RequireLogin(), auth.RequirePlatformAdmin(), workerAuthHandler.ListWorkerAuths)

			// 🔒 更新Worker授权状态（需要平台管理员权限）
			workerAuths.PUT("/:id/status", auth.RequireLogin(), auth.RequirePlatformAdmin(), workerAuthHandler.UpdateWorkerAuthStatus)

			// 🔒 删除Worker授权（需要平台管理员权限）
			workerAuths.DELETE("/:id", auth.RequireLogin(), auth.RequirePlatformAdmin(), workerAuthHandler.DeleteWorkerAuth)
		}

		// 🔐 分布式Worker管理路由（添加权限保护）
		workerManagerHandler := handlers.NewWorkerManagerHandler(services.NewWorkerManagerService(database.GetDB()))
		workers := api.Group("/workers")
		{
			// 🔒 Worker列表查看（需要Worker列表权限）
			workers.GET("", auth.RequireLogin(), auth.RequirePermission("worker:list"), workerManagerHandler.GetAllWorkers)
			workers.GET("/summary", auth.RequireLogin(), auth.RequirePermission("worker:list"), workerManagerHandler.GetWorkerSummary)
			workers.GET("/active", auth.RequireLogin(), auth.RequirePermission("worker:list"), workerManagerHandler.GetActiveWorkers)
			workers.GET("/status", auth.RequireLogin(), auth.RequirePermission("worker:list"), workerManagerHandler.GetWorkersByStatus)
			workers.GET("/dashboard", auth.RequireLogin(), auth.RequirePermission("worker:list"), workerManagerHandler.GetDashboardData)

			// 🔒 单个Worker详情（需要Worker详情权限）
			workers.GET("/:worker_id", auth.RequireLogin(), auth.RequirePermission("worker:read"), workerManagerHandler.GetWorkerByID)
			workers.GET("/:worker_id/tasks", auth.RequireLogin(), auth.RequirePermission("worker:read"), workerManagerHandler.GetWorkerTasks)

			// 🔒 Worker管理操作（需要具体权限）
			workers.DELETE("/:worker_id", auth.RequireLogin(), auth.RequirePermission("worker:delete"), workerManagerHandler.RemoveOfflineWorker)
			workers.GET("/queue/stats", auth.RequireLogin(), auth.RequirePermission("worker:queue"), workerManagerHandler.GetQueueStats)
		}

		// 🔐 网络扫描路由（添加权限保护）
		networkScanHandler := handlers.NewNetworkScanHandler(
			services.NewNetworkScanService(),
			services.NewHostService(database.GetDB()),
		)
		networkScan := api.Group("/network-scan")
		{
			// 🔒 扫描控制（需要网络扫描权限）
			networkScan.POST("/start", auth.RequireLogin(), auth.RequirePermission("network_scan:start"), networkScanHandler.StartScan)
			networkScan.GET("/:scan_id", auth.RequireLogin(), auth.RequirePermission("network_scan:view"), networkScanHandler.GetScanStatus)
			networkScan.POST("/:scan_id/cancel", auth.RequireLogin(), auth.RequirePermission("network_scan:cancel"), networkScanHandler.CancelScan)

			// 🔒 任务管理
			networkScan.GET("/active", auth.RequireLogin(), auth.RequirePermission("network_scan:view"), networkScanHandler.GetActiveTasks)
			networkScan.POST("/estimate", auth.RequireLogin(), auth.RequirePermission("network_scan:start"), networkScanHandler.EstimateTargets)

			// 🔒 结果查询
			networkScan.GET("/:scan_id/result", auth.RequireLogin(), auth.RequirePermission("network_scan:view"), networkScanHandler.GetScanResult)

			// 🔒 批量导入（需要主机导入权限）
			networkScan.POST("/import", auth.RequireLogin(), auth.RequirePermission("network_scan:import"), networkScanHandler.ImportDiscoveredHosts)
		}

		// 🔐 Git仓库路由（添加权限保护）
		gitRepoService := services.NewGitRepositoryService(database.GetDB())
		gitRepoService.SetQueue(database.GetRedisQueue())
		gitRepoHandler := handlers.NewGitRepositoryHandler(gitRepoService)
		gitRepos := api.Group("/git-repositories")
		{
			// 🔒 基础CRUD（需要Git仓库管理权限）
			gitRepos.POST("", auth.RequireLogin(), auth.RequirePermission("git_repository:create"), gitRepoHandler.Create)
			gitRepos.GET("", auth.RequireLogin(), auth.RequirePermission("git_repository:list"), gitRepoHandler.List)
			gitRepos.GET("/:id", auth.RequireLogin(), auth.RequirePermission("git_repository:read"), gitRepoHandler.GetByID)
			gitRepos.PUT("/:id", auth.RequireLogin(), auth.RequirePermission("git_repository:update"), gitRepoHandler.Update)
			gitRepos.DELETE("/:id", auth.RequireLogin(), auth.RequirePermission("git_repository:delete"), gitRepoHandler.Delete)

			// 🔒 同步相关（需要同步权限）
			gitRepos.GET("/:id/sync-logs", auth.RequireLogin(), auth.RequirePermission("git_repository:sync_logs"), gitRepoHandler.GetSyncLogs)
			gitRepos.POST("/:id/sync", auth.RequireLogin(), auth.RequirePermission("git_repository:sync"), gitRepoHandler.ManualSync)
			gitRepos.POST("/:id/scan-templates", auth.RequireLogin(), auth.RequirePermission("git_repository:sync"), gitRepoHandler.ScanTemplates)
		}


		// 🔐 任务模板路由
		taskTemplateHandler := handlers.NewTaskTemplateHandler(services.NewTaskTemplateService(database.GetDB()))
		taskTemplates := api.Group("/task-templates")
		{
			// 🔒 基础CRUD（需要任务模板管理权限）
			taskTemplates.POST("", auth.RequireLogin(), auth.RequirePermission("task_template:create"), taskTemplateHandler.Create)
			taskTemplates.GET("", auth.RequireLogin(), auth.RequirePermission("task_template:list"), taskTemplateHandler.List)
			taskTemplates.GET("/:id", auth.RequireLogin(), auth.RequirePermission("task_template:read"), taskTemplateHandler.GetByID)
			taskTemplates.PUT("/:id", auth.RequireLogin(), auth.RequirePermission("task_template:update"), taskTemplateHandler.Update)
			taskTemplates.DELETE("/:id", auth.RequireLogin(), auth.RequirePermission("task_template:delete"), taskTemplateHandler.Delete)
			
			// 🔐 Worker同步任务模板（无需认证，Worker使用AK/SK）
			taskTemplates.POST("/sync", taskTemplateHandler.SyncFromWorker)
		}

	}
}

func healthCheck(c *gin.Context) {
	data := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now(),
		"service":   "AHOP",
		"version":   "1.0.0",
	}
	response.Success(c, data)
}

func ping(c *gin.Context) {
	response.SuccessWithMessage(c, "pong", nil)
}
