# AHOP 分布式Worker

独立的分布式任务执行器，用于AHOP自动化平台。

## 特性

- **独立部署**: 完全独立的Go程序，可部署在任意机器
- **分布式**: 支持多个Worker节点，自动负载均衡
- **任务执行**: 支持Ansible任务、主机连接测试、信息采集等
- **实时监控**: 心跳机制，实时上报状态和资源使用情况
- **容错性**: 任务重试、优雅关闭、错误恢复
- **可扩展**: 插件化执行器架构，易于添加新任务类型

## 架构

```
┌─────────────────┐    Redis队列    ┌──────────────────┐
│   AHOP 主服务   │ ──────────────> │  分布式Worker    │
│                 │                 │                  │
│ 任务创建与管理  │                 │  任务执行引擎    │
└─────────────────┘                 └──────────────────┘
         │                                   │
         │            PostgreSQL             │
         └───────────────────────────────────┘
               数据库（任务状态、日志）
```

## 支持的任务类型

- **host_facts**: 主机信息采集
- **host_ping**: 主机连通性测试
- **ansible_adhoc**: Ansible Ad-hoc命令执行
- **ansible_setup**: Ansible setup模块执行

## 快速开始

### 1. 构建Worker

```bash
# 克隆或复制Worker代码到目标机器
cd /opt/ahop-worker

# 构建
make build
```

### 2. 配置

```bash
# 创建配置文件
make config

# 编辑配置文件
vim config.json
```

关键配置项：

```json
{
  "worker": {
    "id": "worker-01",           // Worker唯一标识
    "concurrency": 3,            // 并发任务数
    "timeout": "30m"             // 任务超时时间
  },
  "redis": {
    "host": "your-redis-host",   // Redis服务器地址
    "password": "your-password"  // Redis密码
  },
  "database": {
    "host": "your-db-host",      // 数据库服务器地址
    "password": "your-password"  // 数据库密码
  }
}
```

### 3. 运行

#### 方式一：直接运行
```bash
make run-with-config
```

#### 方式二：环境变量
```bash
export WORKER_ID=worker-prod-01
export REDIS_HOST=10.0.0.100
export REDIS_PASSWORD=yourpassword
export DB_HOST=10.0.0.101
export DB_PASSWORD=yourpassword

make run-env
```

#### 方式三：systemd服务
```bash
# 安装为系统服务
sudo make install

# 启动服务
sudo systemctl enable ahop-worker
sudo systemctl start ahop-worker

# 查看状态
sudo systemctl status ahop-worker
```

## 部署方案

### 单机部署

适合测试和小规模环境：

```bash
# 在同一台机器上运行多个Worker实例
./ahop-worker -config config-worker1.json &
./ahop-worker -config config-worker2.json &
```

### 分布式部署

适合生产环境：

```bash
# 机器1
export WORKER_ID=worker-node1
./ahop-worker

# 机器2  
export WORKER_ID=worker-node2
./ahop-worker

# 机器3
export WORKER_ID=worker-node3
./ahop-worker
```

### Docker部署

```bash
# 构建镜像
make docker

# 运行容器
docker run -d --name ahop-worker \
  -e WORKER_ID=worker-docker-01 \
  -e REDIS_HOST=your-redis-host \
  -e REDIS_PASSWORD=your-password \
  -e DB_HOST=your-db-host \
  -e DB_PASSWORD=your-password \
  ahop-worker:1.0.0
```

## 环境变量配置

| 变量名 | 描述 | 默认值 |
|--------|------|--------|
| `WORKER_ID` | Worker唯一标识 | worker-{hostname}-{timestamp} |
| `WORKER_CONCURRENCY` | 并发任务数 | 2 |
| `REDIS_HOST` | Redis主机地址 | localhost |
| `REDIS_PORT` | Redis端口 | 6379 |
| `REDIS_PASSWORD` | Redis密码 | |
| `DB_HOST` | 数据库主机地址 | localhost |
| `DB_PORT` | 数据库端口 | 5432 |
| `DB_USER` | 数据库用户名 | postgres |
| `DB_PASSWORD` | 数据库密码 | |
| `DB_NAME` | 数据库名 | auto_healing_platform |
| `LOG_LEVEL` | 日志级别 | info |

## 监控和管理

### 查看Worker状态

在AHOP主服务中查看Worker状态：

```bash
curl -X GET http://ahop-server:8080/api/v1/workers/status \
  -H "Authorization: Bearer your-token"
```

### 查看日志

```bash
# 实时日志
make logs

# 或直接查看
tail -f logs/worker.log
```

### 性能监控

Worker会自动上报以下指标：
- CPU使用率
- 内存使用率  
- 当前任务数
- 总任务数/成功数/失败数

## 故障排除

### 常见问题

1. **Redis连接失败**
   ```bash
   # 测试Redis连接
   redis-cli -h your-redis-host -p 6379 -a your-password ping
   ```

2. **数据库连接失败**
   ```bash
   # 测试数据库连接
   PGPASSWORD=your-password psql -h your-db-host -U postgres -d auto_healing_platform -c "SELECT 1;"
   ```

3. **Ansible命令失败**
   ```bash
   # 确保安装了Ansible
   ansible --version
   
   # 检查PATH
   which ansible
   ```

### 日志分析

Worker日志包含以下信息：
- Worker启动/停止事件
- 任务接收和执行状态
- 系统资源使用情况
- 错误和异常信息

### 性能调优

1. **调整并发数**: 根据机器性能调整`concurrency`
2. **内存优化**: 监控内存使用，避免内存泄漏
3. **网络优化**: 确保与Redis和数据库的网络延迟最小

## 开发指南

### 添加新的执行器

1. 创建执行器文件：
   ```go
   // internal/executor/my_executor.go
   type MyExecutor struct {
       *BaseExecutor
   }
   
   func (e *MyExecutor) Execute(ctx context.Context, taskCtx *TaskContext, onProgress ProgressCallback, onLog LogCallback) *TaskResult {
       // 实现执行逻辑
   }
   ```

2. 注册执行器：
   ```go
   // internal/worker/worker.go
   func (w *Worker) registerExecutors() {
       // 注册新执行器
       myExecutor := executor.NewMyExecutor()
       for _, taskType := range myExecutor.GetSupportedTypes() {
           w.executors[taskType] = myExecutor
       }
   }
   ```

### 构建和测试

```bash
# 运行测试
go test ./...

# 构建
make build

# 清理
make clean
```

## 许可证

[你的许可证信息]