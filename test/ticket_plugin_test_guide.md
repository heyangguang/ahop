# 工单插件测试指南

## 可用的模拟工单系统

### 1. 基础版 (`mock_ticket_plugin.py`)
- **端口**: 5000
- **特点**: 4个固定工单 + 随机生成
- **适合**: 基础功能测试

### 2. 高级版 (`advanced_mock_ticket_plugin.py`)
- **端口**: 5001
- **特点**: 
  - 50个初始工单
  - 支持多种工单格式（JIRA、ServiceNow）
  - 分页和高级过滤
  - 错误注入测试
- **适合**: 复杂场景测试

### 3. 真实感版 (`realistic_mock_ticket_plugin.py`)
- **端口**: 5002
- **特点**:
  - 100-200个动态生成的真实工单
  - 5大类别：数据库、应用、网络、安全、基础设施
  - 真实的问题场景和描述
  - 自动更新和状态变化
  - 包含自定义字段
- **适合**: 生产环境模拟测试

## 快速测试步骤

### 1. 启动模拟系统

```bash
# 选择一个版本运行
cd /opt/ahop

# 基础版
python test/mock_ticket_plugin.py

# 高级版（需要安装 flask）
pip install flask
python test/advanced_mock_ticket_plugin.py

# 真实感版（建议安装 faker，但不是必需）
pip install flask faker
python test/realistic_mock_ticket_plugin.py
```

### 2. 运行自动化测试

```bash
# 确保AHOP服务正在运行
# 然后执行测试脚本
./test/test_ticket_plugin.sh
```

### 3. 手动测试特定功能

```bash
# 设置环境变量
export AHOP_URL="http://localhost:8080/api/v1"
export PLUGIN_URL="http://localhost:5002"  # 真实感版

# 登录获取Token
TOKEN=$(curl -s -X POST $AHOP_URL/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Admin@123"}' \
  | jq -r '.data.token')

# 注册插件
curl -X POST $AHOP_URL/ticket-plugins \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "真实工单系统",
    "code": "realistic-tickets",
    "description": "模拟真实生产环境的工单",
    "base_url": "'$PLUGIN_URL'",
    "auth_type": "none",
    "sync_enabled": true,
    "sync_interval": 5
  }'
```

## 测试场景示例

### 场景1: 测试不同优先级的工单同步

```bash
# 只同步高优先级工单
curl -X POST $AHOP_URL/ticket-plugins/1/test-sync \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_params": {
      "priority": "critical",
      "minutes": 1440
    },
    "test_options": {
      "sample_size": 5,
      "show_mapping_details": true
    }
  }'
```

### 场景2: 测试不同类别的工单

```bash
# 只同步数据库相关工单
curl -X POST $AHOP_URL/ticket-plugins/1/test-sync \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_params": {
      "category": "database",
      "status": "open"
    },
    "test_options": {
      "sample_size": 10
    }
  }'
```

### 场景3: 测试实时数据同步

```bash
# 获取最近10分钟的工单
curl -X POST $AHOP_URL/ticket-plugins/1/test-sync \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_params": {
      "minutes": 10
    },
    "test_options": {
      "sample_size": 20,
      "show_filtered": true
    }
  }'
```

## 验证同步结果

### 1. 查看同步日志

```bash
curl -X GET "$AHOP_URL/ticket-plugins/1/sync-logs?page=1&page_size=10" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 2. 查看同步的工单

```bash
# 所有工单
curl -X GET $AHOP_URL/tickets \
  -H "Authorization: Bearer $TOKEN" | jq

# 按状态过滤
curl -X GET "$AHOP_URL/tickets?status=open" \
  -H "Authorization: Bearer $TOKEN" | jq

# 查看统计信息
curl -X GET $AHOP_URL/tickets/stats \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 3. 查看特定工单详情

```bash
# 获取工单ID后查看详情
TICKET_ID=1  # 替换为实际ID
curl -X GET $AHOP_URL/tickets/$TICKET_ID \
  -H "Authorization: Bearer $TOKEN" | jq
```

## 真实感版本的特色功能

### 查看工单统计

```bash
# 直接从模拟系统获取统计
curl http://localhost:5002/stats | jq
```

输出示例：
```json
{
  "success": true,
  "data": {
    "total": 150,
    "by_status": {
      "open": 25,
      "in_progress": 45,
      "resolved": 50,
      "closed": 30
    },
    "by_priority": {
      "critical": 20,
      "high": 40,
      "medium": 60,
      "low": 30
    },
    "by_category": {
      "database": 30,
      "application": 35,
      "network": 25,
      "security": 30,
      "infrastructure": 30
    },
    "recent_24h": 18
  }
}
```

## 测试技巧

1. **测试前检查模拟系统**
   ```bash
   curl http://localhost:5002/health | jq
   ```

2. **使用不同参数组合**
   - 时间范围: `minutes=5, 30, 60, 1440`
   - 状态过滤: `status=open, in_progress, resolved, closed`
   - 优先级: `priority=critical, high, medium, low`
   - 类别: `category=database, application, network, security, infrastructure`

3. **观察数据变化**
   - 真实感版本会自动更新工单
   - 多次执行同步可以看到不同结果

4. **错误场景测试**
   - 停止模拟系统测试连接失败
   - 使用错误的认证信息
   - 测试网络超时（可以修改模拟系统添加延迟）

## 常见问题

1. **Q: 为什么看不到新工单？**
   A: 检查时间过滤参数，增大 `minutes` 值

2. **Q: 同步失败怎么办？**
   A: 查看AHOP日志和模拟系统是否正常运行

3. **Q: 如何模拟大量数据？**
   A: 修改真实感版本的 `target_count` 参数

4. **Q: 如何测试字段映射？**
   A: 使用高级版本的不同格式（JIRA、ServiceNow）