# 任务系统重构设计文档

## 概述

本文档记录了任务系统的重构设计，主要目标是简化任务类型、统一参数结构，为未来的任务模板系统做准备。

## 1. 执行器类型

系统支持两种执行器：

- **ansible** - 通过Ansible执行任务
- **shell** - 直接通过SSH执行任务

## 2. 任务类型

### 2.1 当前支持的任务类型

- **ping** - 主机探活
  - 使用网络ICMP ping，不需要SSH连接
  - 不需要凭证
  - 用于快速检查主机网络连通性

- **collect** - 信息采集
  - 使用Ansible的setup模块
  - 需要有效的SSH凭证
  - 采集主机的系统信息（OS、CPU、内存、磁盘等）

### 2.2 未来的任务类型

- **template** - 任务模板
  - 用户创建的可复用任务
  - 支持参数化

## 3. 任务参数结构

### 3.1 基础任务参数

```json
{
  "task_type": "ping|collect|template",
  "priority": 5,  // 1-10，数字越大优先级越高
  "hosts": [
    {
      "ip": "192.168.1.10",
      "port": 22,
      "credential_id": 1  // ping任务可以不需要
    }
  ]
}
```

### 3.2 任务模板参数（未来）

```json
{
  "task_type": "template",
  "template_id": 123,
  "priority": 5,
  "hosts": [...],
  "variables": {
    "version": "1.2.3",
    "env": "prod"
  }
}
```

### 3.3 Ansible执行选项

当使用Ansible执行器时，可以传递以下选项：

```json
{
  "ansible_options": {
    "verbosity": 3,           // 详细级别 0-4 (对应 -v 到 -vvvv)
    "check": true,           // 检查模式 (--check)
    "diff": true,            // 显示变更差异 (--diff)
    "forks": 10,             // 并发执行数 (-f)
    "timeout": 30,           // 连接超时秒数 (-T)
    "become": true,          // 提权执行 (-b)
    "become_user": "root",   // 提权用户 (-u)
    "extra_vars": {          // 额外变量 (-e)
      "key": "value"
    }
  }
}
```

## 4. API变更

### 4.1 创建任务 API

**端点**: `POST /api/v1/tasks`

**请求示例**:

```json
// Ping任务
{
  "task_type": "ping",
  "priority": 5,
  "hosts": [
    {"ip": "192.168.1.10", "port": 22}
  ]
}

// Collect任务
{
  "task_type": "collect", 
  "priority": 5,
  "hosts": [
    {"ip": "192.168.1.10", "port": 22, "credential_id": 1}
  ]
}
```

## 5. Worker端改造

### 5.1 执行器映射

- ping任务 → Shell执行器（执行系统ping命令）
- collect任务 → Ansible执行器（执行setup模块）
- template任务 → 根据模板定义选择执行器

### 5.2 参数处理

Worker需要能够：
1. 解析新的参数结构
2. 根据任务类型选择合适的执行器
3. 处理Ansible选项并转换为命令行参数

## 6. 数据库变更

### 6.1 tasks表

需要调整的字段：
- `task_type` - 改为支持 ping/collect/template
- `params` - JSON结构调整为新格式

### 6.2 未来的task_templates表

```sql
CREATE TABLE task_templates (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    executor_type VARCHAR(20) NOT NULL, -- ansible/shell
    content TEXT NOT NULL,              -- 执行内容
    variables JSONB,                    -- 变量定义
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

## 7. 实施计划

### Phase 1 - 基础改造（当前）
- [x] 整理设计文档
- [ ] 简化任务类型为ping和collect
- [ ] 重构任务参数结构
- [ ] 实现ping任务（网络ICMP）
- [ ] 优化collect任务实现

### Phase 2 - 任务模板（未来）
- [ ] 设计任务模板表结构
- [ ] 实现任务模板CRUD
- [ ] 实现模板变量替换
- [ ] 支持模板执行

### Phase 3 - 高级功能（未来）
- [ ] 支持任务编排
- [ ] 支持条件执行
- [ ] 支持任务依赖

## 8. 注意事项

1. **向后兼容**：需要支持旧的任务参数格式一段时间
2. **错误处理**：新的参数结构需要严格的验证
3. **性能考虑**：ping任务应该尽可能轻量快速
4. **安全性**：Ansible选项需要验证，防止注入攻击