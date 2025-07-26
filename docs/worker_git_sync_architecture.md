# Worker端Git仓库同步架构设计

## 1. 整体架构

Worker端Git仓库同步采用发布/订阅模式，通过Redis Pub/Sub接收同步任务，在本地管理Git仓库副本。

### 组件关系
```
Server端                          Worker端
  │                                │
  ├─ GitRepositoryService          ├─ GitSyncWorker
  │    └─ NotifyWorkerSync()       │    ├─ 监听 Redis Pub/Sub
  │                                │    ├─ 克隆/拉取仓库
  └─ GitSyncScheduler              │    └─ 记录同步日志
       └─ 定时触发同步               │
                                   └─ 定期清理旧副本
```

## 2. 存储路径设计

### 路径结构
```
/data/ahop/repos/
└── {tenant_id}/                    # 租户隔离
    └── {repo_id}/                  # 仓库ID
        └── {worker_id}_{timestamp}_{random}/  # 唯一实例
            ├── .git/               # Git元数据
            ├── scripts/            # 脚本目录
            └── ...                 # 其他文件
```

### 路径组成说明
- **tenant_id**: 租户ID，实现多租户隔离
- **repo_id**: 仓库ID，对应数据库中的仓库记录
- **worker_id**: Worker标识符，格式：`worker-{hostname}-{uuid前8位}`
- **timestamp**: Unix时间戳，便于排序和清理
- **random**: 6位随机字符串，避免并发冲突

### 示例
```
/data/ahop/repos/1/15/worker-server01-a1b2c3d4_1703123456_x9y8z7/
```

## 3. 同步流程

### 3.1 手动同步流程
```
1. 用户通过API触发同步
2. Server发布同步消息到Redis
3. Worker接收消息并执行同步
4. Worker记录同步日志到数据库
```

### 3.2 定时同步流程
```
1. GitSyncScheduler定时触发
2. Server发布同步消息到Redis
3. Worker接收消息并执行同步
4. Worker记录同步日志到数据库
```

### 3.3 同步消息格式
```json
{
    "action": "sync",           // sync|delete
    "tenant_id": 1,
    "repository_id": 15,
    "repository": {
        "id": 15,
        "name": "ansible-playbooks",
        "url": "https://github.com/example/ansible-playbooks.git",
        "branch": "main",
        "is_public": false,
        "credential_id": 5
    },
    "operator_id": 100          // 手动触发时的用户ID
}
```

## 4. 认证机制

### 4.1 公开仓库
- 直接使用HTTPS URL克隆
- 无需认证信息

### 4.2 私有仓库
支持多种认证方式：

#### 用户名密码（HTTPS）
```bash
git clone https://username:password@github.com/example/repo.git
```

#### SSH密钥
```bash
# 使用临时SSH配置
GIT_SSH_COMMAND="ssh -i /tmp/key_xxx -o StrictHostKeyChecking=no" git clone git@github.com:example/repo.git
```

#### Personal Access Token
```bash
git clone https://token@github.com/example/repo.git
```

## 5. 错误处理

### 5.1 重试机制
- 网络错误：自动重试3次，间隔10秒
- 认证失败：不重试，记录错误
- 仓库不存在：不重试，记录错误

### 5.2 错误记录
所有错误都记录到GitSyncLog表：
- error_message: 错误描述
- command_output: Git命令输出
- status: failed

## 6. 清理策略

### 6.1 自动清理
- 默认保留7天内的仓库副本
- 每天执行一次清理任务
- 基于目录名中的时间戳判断

### 6.2 手动清理
- 删除仓库时清理所有本地副本
- 通过delete消息触发

## 7. 并发控制

### 7.1 多Worker支持
- 每个Worker独立运行
- 通过路径中的worker_id区分
- 互不干扰

### 7.2 同一Worker并发
- 使用随机字符串避免路径冲突
- 每次同步创建新目录
- 不复用已有目录

## 8. 监控指标

### 8.1 同步日志字段
- task_type: scheduled/manual
- status: running/success/failed
- duration: 执行时长（秒）
- local_path: 本地仓库路径
- command_output: Git命令输出

### 8.2 可监控指标
- 同步成功率
- 平均同步时长
- 失败原因分布
- 存储空间占用

## 9. 安全考虑

### 9.1 凭证安全
- 凭证在数据库中加密存储
- Worker端即用即销毁
- 不在日志中记录敏感信息

### 9.2 文件系统安全
- 严格的目录权限（755）
- 租户间完全隔离
- 定期清理避免空间耗尽

### 9.3 网络安全
- 支持HTTPS/SSH协议
- 可配置代理服务器
- 超时控制防止hang住

## 10. 配置项

### 环境变量
```bash
# Git仓库基础目录
GIT_REPO_BASE_DIR=/data/ahop/repos

# 仓库保留天数
GIT_REPO_KEEP_DAYS=7

# Git命令超时（秒）
GIT_COMMAND_TIMEOUT=300

# 代理配置（可选）
HTTP_PROXY=http://proxy.example.com:8080
HTTPS_PROXY=http://proxy.example.com:8080
```

## 11. 后续优化

### 11.1 性能优化
- 使用Git shallow clone减少数据传输
- 实现增量同步，只拉取变更
- 并行处理多个仓库同步

### 11.2 功能增强
- 支持Git LFS大文件
- 支持Submodule子模块
- 支持多分支管理
- 实现仓库缓存共享

### 11.3 可靠性提升
- 实现同步任务队列持久化
- 添加健康检查接口
- 实现主从Worker模式