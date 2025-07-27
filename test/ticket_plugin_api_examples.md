# 工单插件集成 REST API 测试示例

## 准备工作

### 1. 启动模拟工单插件

```bash
# 简单版本（端口5000）
cd /opt/ahop
python test/mock_ticket_plugin.py

# 或高级版本（端口5001）- 支持更多测试场景
python test/advanced_mock_ticket_plugin.py
```

### 2. 确保AHOP服务正在运行

```bash
go run cmd/server/*.go
```

## REST API 测试步骤

### 1. 登录获取Token

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "Admin@123"
  }'
```

响应示例：
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "user": {
      "id": 1,
      "username": "admin",
      "tenant_id": 1
    }
  }
}
```

### 2. 创建工单插件

```bash
# 设置TOKEN变量
TOKEN="你的token"

# 创建插件
curl -X POST http://localhost:8080/api/v1/ticket-plugins \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "测试工单系统",
    "code": "test-ticket-system",
    "description": "用于测试的模拟工单系统",
    "base_url": "http://localhost:5000",
    "auth_type": "bearer",
    "auth_token": "test-token-12345",
    "sync_enabled": true,
    "sync_interval": 5
  }'
```

### 3. 测试插件连接

```bash
# 假设插件ID为1
curl -X POST http://localhost:8080/api/v1/ticket-plugins/1/test \
  -H "Authorization: Bearer ${TOKEN}"
```

### 4. 测试同步（预览模式）

```bash
curl -X POST http://localhost:8080/api/v1/ticket-plugins/1/test-sync \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_params": {
      "minutes": 60,
      "status": "open"
    },
    "test_options": {
      "sample_size": 5,
      "show_filtered": true,
      "show_mapping_details": true
    }
  }'
```

响应示例：
```json
{
  "code": 200,
  "data": {
    "success": true,
    "summary": {
      "total_fetched": 10,
      "total_filtered": 8,
      "total_mapped": 8,
      "filter_rules_applied": 2
    },
    "samples": {
      "raw_data": [...],
      "filtered_out": [...],
      "mapped_data": [
        {
          "external_id": "MOCK-001",
          "title": "生产环境数据库连接异常",
          "status": "open",
          "_mapping_info": {
            "title": {
              "source_field": "title",
              "source_value": "生产环境数据库连接异常"
            }
          }
        }
      ]
    }
  }
}
```

### 5. 执行实际同步

```bash
curl -X POST http://localhost:8080/api/v1/ticket-plugins/1/sync \
  -H "Authorization: Bearer ${TOKEN}"
```

### 6. 查看同步日志

```bash
curl -X GET http://localhost:8080/api/v1/ticket-plugins/1/sync-logs \
  -H "Authorization: Bearer ${TOKEN}"
```

### 7. 查看同步的工单

```bash
# 查看所有工单
curl -X GET http://localhost:8080/api/v1/tickets \
  -H "Authorization: Bearer ${TOKEN}"

# 按条件过滤
curl -X GET "http://localhost:8080/api/v1/tickets?status=open&priority=high" \
  -H "Authorization: Bearer ${TOKEN}"

# 查看工单详情
curl -X GET http://localhost:8080/api/v1/tickets/1 \
  -H "Authorization: Bearer ${TOKEN}"

# 获取工单统计
curl -X GET http://localhost:8080/api/v1/tickets/stats \
  -H "Authorization: Bearer ${TOKEN}"
```

## 高级测试场景

### 1. 测试不同的工单格式

使用高级模拟插件，测试JIRA格式：

```bash
curl -X POST http://localhost:8080/api/v1/ticket-plugins \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "JIRA集成",
    "code": "jira-integration",
    "description": "JIRA工单系统集成",
    "base_url": "http://localhost:5001",
    "auth_type": "bearer",
    "auth_token": "jira-token",
    "sync_enabled": true,
    "sync_interval": 10
  }'
```

测试同步JIRA格式数据：

```bash
curl -X POST http://localhost:8080/api/v1/ticket-plugins/2/test-sync \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_params": {
      "format": "jira",
      "status": "open",
      "priority": "critical"
    },
    "test_options": {
      "sample_size": 3,
      "show_mapping_details": true
    }
  }'
```

### 2. 测试字段映射

创建自定义字段映射（需要先实现字段映射API）：

```bash
# 映射JIRA字段到AHOP字段
curl -X POST http://localhost:8080/api/v1/ticket-plugins/2/field-mappings \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "mappings": [
      {
        "source_field": "fields.summary",
        "target_field": "title",
        "required": true
      },
      {
        "source_field": "fields.priority.name",
        "target_field": "priority",
        "required": true
      },
      {
        "source_field": "fields.status.name",
        "target_field": "status",
        "required": true
      }
    ]
  }'
```

### 3. 测试同步规则

创建过滤规则（需要先实现同步规则API）：

```bash
# 只同步高优先级的开放工单
curl -X POST http://localhost:8080/api/v1/ticket-plugins/1/sync-rules \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "rules": [
      {
        "name": "只同步高优先级",
        "field": "priority",
        "operator": "in",
        "value": "critical,high",
        "action": "include",
        "priority": 1
      },
      {
        "name": "排除已关闭工单",
        "field": "status",
        "operator": "equals",
        "value": "closed",
        "action": "exclude",
        "priority": 2
      }
    ]
  }'
```

### 4. 错误处理测试

测试插件返回错误：

```bash
# 使用高级模拟插件的错误注入功能
curl -X POST http://localhost:8080/api/v1/ticket-plugins/1/test-sync \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_params": {
      "inject_error": "true"
    },
    "test_options": {
      "sample_size": 1
    }
  }'
```

## 插件管理操作

### 禁用插件

```bash
curl -X POST http://localhost:8080/api/v1/ticket-plugins/1/disable \
  -H "Authorization: Bearer ${TOKEN}"
```

### 启用插件

```bash
curl -X POST http://localhost:8080/api/v1/ticket-plugins/1/enable \
  -H "Authorization: Bearer ${TOKEN}"
```

### 更新插件配置

```bash
curl -X PUT http://localhost:8080/api/v1/ticket-plugins/1 \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "sync_interval": 15,
    "base_url": "http://localhost:5001"
  }'
```

### 删除插件

```bash
curl -X DELETE http://localhost:8080/api/v1/ticket-plugins/1 \
  -H "Authorization: Bearer ${TOKEN}"
```

## 使用Postman测试

如果你更喜欢使用Postman，可以导入以下配置：

1. 创建环境变量：
   - `base_url`: http://localhost:8080/api/v1
   - `token`: 登录后获取的token

2. 创建请求集合，包含所有上述API

3. 在请求头中使用：`Authorization: Bearer {{token}}`

## 故障排除

1. **连接被拒绝**
   - 确保AHOP和模拟插件都在运行
   - 检查端口是否正确

2. **认证失败**
   - 确保使用了正确的token
   - Token可能已过期，重新登录

3. **插件测试失败**
   - 检查插件URL是否可访问
   - 查看AHOP和插件的日志

4. **同步无数据**
   - 检查时间过滤参数
   - 确认插件返回了数据