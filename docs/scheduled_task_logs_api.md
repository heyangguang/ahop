# 定时任务日志查询API

## 概述

定时任务的日志查询通过以下关联链路实现：
```
ScheduledTask (定时任务) 
    → ScheduledTaskExecution (执行历史) 
    → Task (任务实例) 
    → TaskLog (执行日志)
```

## API接口

### 获取定时任务执行日志

**端点**: `GET /api/v1/scheduled-tasks/:id/logs`

**权限**: `scheduled_task:read`

**路径参数**:
- `id` - 定时任务ID

**查询参数**:
- `execution_id` (可选) - 特定执行历史ID的日志
- `level` (可选) - 日志级别过滤 (debug/info/warning/error)
- `host` (可选) - 主机名过滤
- `keyword` (可选) - 关键词搜索（在message中搜索）
- `start_time` (可选) - 开始时间 (格式: 2006-01-02 15:04:05)
- `end_time` (可选) - 结束时间 (格式: 2006-01-02 15:04:05)
- `page` (可选) - 页码，默认1
- `page_size` (可选) - 每页条数，默认10，最大100

**响应示例**:
```json
{
    "code": 200,
    "message": "success",
    "data": [
        {
            "id": 12345,
            "task_id": "task_uuid_123",
            "timestamp": "2024-01-20T10:30:00Z",
            "level": "info",
            "source": "worker",
            "host_name": "host-001",
            "message": "开始执行Ansible playbook",
            "data": {}
        },
        {
            "id": 12346,
            "task_id": "task_uuid_123",
            "timestamp": "2024-01-20T10:30:05Z",
            "level": "info",
            "source": "ansible",
            "host_name": "host-001",
            "message": "TASK [Gathering Facts]",
            "data": {}
        }
    ],
    "pagination": {
        "page": 1,
        "page_size": 10,
        "total": 150,
        "total_pages": 15
    }
}
```

### 获取执行历史列表

**端点**: `GET /api/v1/scheduled-tasks/:id/executions`

用于获取定时任务的执行历史记录，每条记录包含对应的task_id，可用于进一步查询详细日志。

### 使用示例

#### 1. 查询特定定时任务的所有日志
```bash
curl -X GET "http://localhost:8080/api/v1/scheduled-tasks/1/logs" \
  -H "Authorization: Bearer $TOKEN"
```

#### 2. 查询特定执行的日志
```bash
# 先获取执行历史
curl -X GET "http://localhost:8080/api/v1/scheduled-tasks/1/executions" \
  -H "Authorization: Bearer $TOKEN"

# 使用返回的execution_id查询该次执行的日志
curl -X GET "http://localhost:8080/api/v1/scheduled-tasks/1/logs?execution_id=123" \
  -H "Authorization: Bearer $TOKEN"
```

#### 3. 按日志级别过滤
```bash
# 只查看错误日志
curl -X GET "http://localhost:8080/api/v1/scheduled-tasks/1/logs?level=error" \
  -H "Authorization: Bearer $TOKEN"
```

#### 4. 按主机过滤
```bash
# 查看特定主机的日志
curl -X GET "http://localhost:8080/api/v1/scheduled-tasks/1/logs?host=host-001" \
  -H "Authorization: Bearer $TOKEN"
```

#### 5. 关键词搜索
```bash
# 搜索包含"failed"的日志
curl -X GET "http://localhost:8080/api/v1/scheduled-tasks/1/logs?keyword=failed" \
  -H "Authorization: Bearer $TOKEN"
```

#### 6. 时间范围查询
```bash
# 查询特定时间范围的日志
curl -X GET "http://localhost:8080/api/v1/scheduled-tasks/1/logs?start_time=2024-01-20 00:00:00&end_time=2024-01-20 23:59:59" \
  -H "Authorization: Bearer $TOKEN"
```

#### 7. 组合查询
```bash
# 查询特定执行中，host-001上的错误日志
curl -X GET "http://localhost:8080/api/v1/scheduled-tasks/1/logs?execution_id=123&host=host-001&level=error" \
  -H "Authorization: Bearer $TOKEN"
```

## 前端集成建议

1. **执行历史视图**：
   - 先展示执行历史列表
   - 点击某次执行可查看该次执行的详细日志

2. **日志实时查看**：
   - 对于正在执行的任务，可通过WebSocket订阅 `task:logs:{task_id}` 频道获取实时日志

3. **日志过滤器**：
   - 提供日志级别、主机、时间范围等过滤器
   - 支持关键词搜索功能

4. **日志导出**：
   - 可以基于查询结果提供日志导出功能