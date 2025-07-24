# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

AHOP（Auto Healing Platform，自动化故障自愈平台）是一个面向运维团队的多租户自动化平台，支持通过API集成外部工单系统，执行Ansible剧本和Shell脚本进行故障自愈。

**核心功能：**
- 多租户架构，支持不同客户隔离
- 凭证管理（SSH密钥、密码、API密钥等）
- 主机管理和批量操作
- Git仓库集成，同步自愈脚本
- 任务执行引擎（SSH、Ansible）
- 定时任务和工作流编排
- 外部工单系统集成
- 审计日志和执行历史

**技术栈：**
- Go 1.24.5
- Gin Web 框架
- PostgreSQL + GORM
- JWT 认证
- 清洁架构模式（贫血模型）
- AES-256 加密（凭证管理）

## 快速开始

### 首次运行
```bash
# 1. 配置数据库（修改 .env 文件）
# 2. 重置数据库
bash scripts/reset_database.sh

# 3. 启动应用（自动创建表和初始数据）
go run cmd/server/*.go

# 4. 使用默认管理员登录
# 用户名: admin
# 密码: Admin@123
```

## 常用命令

### 运行应用
```bash
# 确保 PostgreSQL 正在运行且 .env 已配置
go run cmd/server/*.go

# 或者
cd cmd/server && go run .
```

### 构建
```bash
go build -o ahop cmd/server/*.go
```

### 依赖管理
```bash
go mod download  # 下载依赖
go mod tidy      # 清理依赖
```

### 数据库管理

#### 重置数据库
```bash
# 清空并重新创建数据库
bash scripts/reset_database.sh

# 启动应用（自动执行迁移和种子数据）
go run cmd/server/main.go

# （可选）创建额外测试数据
bash scripts/init_data.sh
```

#### 自动初始化数据
应用启动时会自动创建：
- 默认租户（code: default）
- 所有系统权限
- 平台管理员角色
- 默认管理员账号：
  - 用户名：`admin`
  - 密码：`Admin@123`

#### 种子数据流程
1. `internal/database/migrate.go` - 执行表结构迁移
2. `cmd/server/seed.go` - 创建初始数据（避免循环依赖）
   - `createDefaultTenant()` - 创建默认租户
   - `initializePermissions()` - 初始化所有权限
   - `createPlatformAdminRole()` - 创建平台管理员角色
   - `createDefaultAdmin()` - 创建默认管理员用户

### 测试
```bash
# 运行权限测试套件
bash test/test_jwt_perm.sh

# 检查特定权限
bash test/check_perm.sh

# 测试租户切换功能
bash test/test_tenant_switch.sh
```

## 架构说明

### 目录结构
- `cmd/server/` - 应用入口点
- `internal/` - 内部包（业务逻辑）
  - `database/` - 数据库连接、迁移
  - `handlers/` - HTTP 请求处理器
  - `middleware/` - 认证、错误处理、CORS
  - `models/` - 领域实体
  - `router/` - 路由定义
  - `services/` - 业务逻辑层
- `pkg/` - 可复用包
  - `config/` - 配置管理
  - `errors/` - 自定义错误类型
  - `jwt/` - JWT 工具
  - `logger/` - 结构化日志
  - `pagination/` - 分页工具
  - `response/` - API 响应格式化

### 核心架构模式

#### 1. 贫血模型（Anemic Domain Model）
- **Model 层**：只包含数据结构和基础方法（如 SetPassword、CheckPassword）
- **Service 层**：承载所有业务逻辑、验证、事务处理
- **原则**：模型不包含业务逻辑，只是数据载体

#### 2. 多租户架构
- **数据隔离**：所有业务表包含 `TenantID` 字段
- **查询隔离**：服务层自动添加租户过滤条件
- **API 隔离**：中间件层验证租户访问权限

#### 3. 三级权限体系

**平台超级管理员**
- 标识：`IsPlatformAdmin = true`
- 权限：可以跨租户操作，绕过所有权限检查
- 用途：系统维护、租户管理

**租户管理员**
- 标识：`IsTenantAdmin = true`
- 权限：管理本租户内所有资源
- 限制：不能访问其他租户数据

**租户普通用户**
- 标识：两个管理员字段都为 false
- 权限：通过角色权限系统控制
- 特点：细粒度权限控制

#### 4. 租户切换功能（平台管理员专用）

**JWT 扩展**：
- `TenantID` - 用户所属租户（固定不变）
- `CurrentTenantID` - 当前操作的租户（可动态切换）

**相关 API 端点**：
```
GET  /api/v1/auth/me             # 获取完整用户信息（包含可切换租户）
POST /api/v1/auth/switch-tenant  # 切换到指定租户
```

**使用流程**：
1. 调用 `/auth/me` 获取当前用户完整信息
2. 平台管理员会在响应中看到 `switchable_tenants` 字段
3. 调用 `/auth/switch-tenant` 切换到目标租户
4. 获得新 token，后续操作都在目标租户范围内

**中间件支持**：
- `RequireSameTenant()` - 使用 CurrentTenantID 进行租户隔离
- `RequireOwnerOrAdmin()` - 基于 CurrentTenantID 判断权限

## 统一组件使用指南

### 1. 错误处理（pkg/errors）

定义统一错误码：
```go
const (
    CodeSuccess      = 200
    CodeInvalidParam = 400
    CodeUnauthorized = 401
    CodeForbidden    = 403
    CodeNotFound     = 404
    CodeServerError  = 500
)
```

### 2. API 响应格式（pkg/response）

**成功响应**
```go
response.Success(c, data)
response.SuccessWithMessage(c, "操作成功", data)
```

**分页响应**
```go
pageInfo := pagination.NewPageInfo(page, pageSize, total)
response.SuccessWithPage(c, data, pageInfo)
```

**错误响应**
```go
response.BadRequest(c, "参数错误")
response.Unauthorized(c, "未授权")
response.Forbidden(c, "禁止访问")
response.NotFound(c, "资源不存在")
response.ServerError(c, "服务器错误")
```

### 3. 分页处理（pkg/pagination）

```go
// 解析请求参数
params := pagination.ParsePageParams(c)

// 数据库查询
offset := params.GetOffset()
limit := params.GetLimit()
db.Offset(offset).Limit(limit).Find(&results)

// 创建分页信息
pageInfo := pagination.NewPageInfo(params.Page, params.PageSize, total)
```

**分页限制**：
- 默认页码：1
- 默认每页：10 条
- 最大每页：100 条

### 4. JWT 认证（pkg/jwt）

```go
// 生成令牌
token, err := jwtManager.GenerateToken(
    userID, 
    tenantID, 
    username, 
    isPlatformAdmin, 
    isTenantAdmin
)

// 验证令牌
claims, err := jwtManager.VerifyToken(token)
```

### 5. 日志记录（pkg/logger）

```go
log := logger.GetLogger()

// 基本日志
log.Info("操作成功")
log.Error("操作失败", err)

// 带字段的日志
log.WithFields(logrus.Fields{
    "user_id": userID,
    "tenant_id": tenantID,
    "action": "create_user",
}).Info("创建用户成功")
```

## 开发指南

### 1. 添加新的 API 端点

**步骤 1：定义 Handler**
```go
// internal/handlers/xxx_handler.go
type XxxHandler struct {
    xxxService *services.XxxService
}

func (h *XxxHandler) Create(c *gin.Context) {
    // 1. 解析参数
    // 2. 调用 Service
    // 3. 返回响应
}
```

**步骤 2：实现 Service**
```go
// internal/services/xxx_service.go
type XxxService struct {
    db *gorm.DB
}

func (s *XxxService) Create(tenantID uint, ...) (*models.Xxx, error) {
    // 业务逻辑实现
}
```

**步骤 3：注册路由**
```go
// internal/router/router.go
xxxGroup := v1.Group("/xxx")
{
    xxxGroup.POST("", auth.RequireLogin(), auth.RequirePermission("xxx:create"), xxxHandler.Create)
    xxxGroup.GET("/:id", auth.RequireLogin(), auth.RequireSameTenant(), xxxHandler.GetByID)
}
```

### 2. 实现多租户隔离

**Model 层**
```go
type YourModel struct {
    ID       uint   `gorm:"primarykey"`
    TenantID uint   `gorm:"not null;index" json:"tenant_id"`
    // 其他字段...
}
```

**Service 层查询**
```go
// 始终添加租户过滤
func (s *YourService) GetByTenant(tenantID uint) ([]models.YourModel, error) {
    var results []models.YourModel
    err := s.db.Where("tenant_id = ?", tenantID).Find(&results).Error
    return results, err
}
```

**Handler 层获取租户 ID**
```go
// 从 JWT claims 获取
claims, _ := c.Get("claims")
userClaims := claims.(*jwt.JWTClaims)
tenantID := userClaims.TenantID
```

### 3. 权限控制最佳实践

**细粒度权限**
```go
// 权限格式：模块:操作
"user:create"    // 创建用户
"user:read"      // 查看用户
"user:update"    // 更新用户
"user:delete"    // 删除用户
"user:list"    // 用户列表
```

**中间件组合**
```go
// 只需登录
router.GET("/profile", auth.RequireLogin(), handler.GetProfile)

// 需要特定权限
router.POST("/users", auth.RequireLogin(), auth.RequirePermission("user:create"), handler.Create)

// 需要同租户
router.GET("/users", auth.RequireLogin(), auth.RequireSameTenant(), handler.List)

// 所有者或管理员
router.PUT("/users/:id", auth.RequireLogin(), auth.RequireOwnerOrAdmin(), handler.Update)
```

### 4. 数据库操作规范

**模型变更**
- 修改模型后，应用会在启动时自动迁移
- 种子数据定义在 `cmd/server/seed.go`（避免循环依赖）
- 开发环境可使用 `scripts/reset_database.sh` 完全重置

**使用事务**
```go
err := s.db.Transaction(func(tx *gorm.DB) error {
    // 多个数据库操作
    if err := tx.Create(&model1).Error; err != nil {
        return err
    }
    if err := tx.Create(&model2).Error; err != nil {
        return err
    }
    return nil
})
```

**预加载关联**
```go
// 预加载单个关联
s.db.Preload("Tenant").First(&user, id)

// 预加载多个关联
s.db.Preload("Roles").Preload("Roles.Permissions").First(&user, id)
```

### 5. 测试建议

**集成测试**
- 使用 `test/test_jwt_perm.sh` 测试完整的认证授权流程
- 测试脚本会创建测试数据并验证权限

**权限测试**
- 使用 `test/check_perm.sh` 验证特定权限
- 确保每个角色只能访问授权的资源

**租户切换测试**
- 使用 `test/test_tenant_switch.sh` 测试租户切换功能
- 验证平台管理员可以切换租户

**凭证管理测试**
- 使用 `test/test_credential.sh` 测试凭证CRUD操作
- 验证凭证加密和ACL限制功能

## 环境配置

### 必需的环境变量

```bash
# 数据库配置
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=你的密码
DB_NAME=auto_healing_platform
DB_SSLMODE=disable

# 服务器配置
SERVER_PORT=8080
SERVER_MODE=debug  # debug/release

# JWT 配置
JWT_SECRET_KEY=你的密钥
JWT_TOKEN_DURATION=24h
JWT_REFRESH_DURATION=7d

# 日志配置
LOG_LEVEL=debug  # debug/info/warn/error
LOG_FILE_PATH=logs/app.log
LOG_MAX_SIZE=100      # MB
LOG_MAX_BACKUPS=7     # 保留文件数
LOG_MAX_AGE=30        # 天
LOG_COMPRESS=true     # 压缩旧日志
LOG_FORMAT=json       # json/text

# 凭证加密配置
CREDENTIAL_ENCRYPTION_KEY=你的32字节加密密钥  # 用于AES-256加密

# Redis配置（任务队列）
REDIS_HOST=localhost          # Redis主机地址
REDIS_PORT=6379              # Redis端口
REDIS_PASSWORD=Admin@123     # Redis密码
REDIS_DB=0                   # Redis数据库编号
REDIS_PREFIX=ahop:queue      # 队列键前缀

# CORS配置
CORS_ALLOW_ORIGINS=*                                           # 允许的源（逗号分隔，*表示所有）
CORS_ALLOW_METHODS=GET,POST,PUT,DELETE,OPTIONS,PATCH          # 允许的HTTP方法
CORS_ALLOW_HEADERS=Origin,Content-Type,Authorization,Accept    # 允许的请求头
CORS_EXPOSE_HEADERS=Content-Length,Content-Type                # 暴露的响应头
CORS_ALLOW_CREDENTIALS=false                                   # 是否允许携带凭证
CORS_MAX_AGE=12                                               # 预检请求缓存时间（小时）
```

## 常见问题

### 1. 如何添加新的权限？

在 `cmd/server/seed.go` 的 `initializePermissions` 方法中添加：
```go
{Code: "your_module:create", Name: "创建XXX", Module: "your_module", Description: "创建XXX的权限"},
```

### 2. 如何实现自定义的中间件？

```go
func YourMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 中间件逻辑
        c.Next()
    }
}
```

### 3. 如何处理文件上传？

使用 Gin 的文件上传功能，注意添加文件大小限制和类型验证。

### 4. 如何优化数据库查询？

- 使用索引：在 Model 中添加 `gorm:"index"`
- 避免 N+1 查询：使用 `Preload`
- 分页查询：使用 pagination 包
- 只查询需要的字段：使用 `Select`

### 5. 凭证管理设计要点

**凭证类型**
- password: 用户名密码
- ssh_key: SSH密钥对
- api_key: API密钥
- token: 认证令牌
- certificate: 证书

**安全特性**
- AES-256加密存储敏感字段
- ACL限制（允许/禁止的主机和IP）
- 使用次数限制
- 过期时间控制
- 完整的使用日志审计

**权限控制**
- credential:create - 创建凭证
- credential:read - 查看凭证（不含明文）
- credential:update - 更新凭证
- credential:delete - 删除凭证
- credential:list - 查看凭证列表
- credential:decrypt - 获取凭证明文（特殊权限）

### 6. 标签系统设计要点

**标签模型**
- 支持 key-value 形式的标签（如：env:prod, region:beijing）
- 每个标签可以设置颜色
- 租户内 key+value 组合唯一
- 使用多对多关联支持多种资源类型

**功能特性**
- 标签 CRUD 操作
- 按 key 分组查询
- 资源标签管理（批量更新）
- 删除前检查使用情况
- 支持中文、英文、数字、下划线和连字符

**权限控制**
- tag:list - 查看标签列表
- tag:read - 查看标签详情
- tag:create - 创建标签
- tag:update - 更新标签
- tag:delete - 删除标签

**API 端点**
- GET /api/v1/tags - 获取标签列表
- GET /api/v1/tags/grouped - 按 key 分组获取
- POST /api/v1/tags - 创建标签
- PUT /api/v1/tags/:id - 更新标签
- DELETE /api/v1/tags/:id - 删除标签
- GET /api/v1/credentials/:id/tags - 获取凭证标签
- PUT /api/v1/credentials/:id/tags - 更新凭证标签（全量替换）