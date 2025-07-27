# 工单插件字段映射与过滤规则指南

## 一、字段映射配置

### 1.1 可映射的目标字段

AHOP系统中的工单（Ticket）模型包含以下可映射字段：

| 目标字段 | 类型 | 说明 | 是否必需 |
|---------|------|------|---------|
| `external_id` | string | 外部系统的工单ID | **必需** |
| `title` | string | 工单标题 | **必需** |
| `description` | string | 工单描述 | 可选 |
| `status` | string | 工单状态 | 可选 |
| `priority` | string | 优先级 | 可选 |
| `type` | string | 工单类型 | 可选 |
| `reporter` | string | 报告人 | 可选 |
| `assignee` | string | 处理人 | 可选 |
| `category` | string | 分类 | 可选 |
| `service` | string | 相关服务 | 可选 |
| `tags` | string | 标签（逗号分隔） | 可选 |

### 1.2 源字段格式

源字段支持多种访问方式：

#### 1.2.1 简单字段
```json
{
  "source_field": "title",
  "target_field": "title"
}
```

#### 1.2.2 嵌套对象访问
使用点号（.）访问嵌套对象的属性：
```json
{
  "source_field": "fields.priority.name",  // 访问 fields 对象中 priority 对象的 name 属性
  "target_field": "priority"
}
```

#### 1.2.3 数组索引访问
使用数字索引访问数组元素：
```json
{
  "source_field": "tags.0",  // 获取 tags 数组的第一个元素
  "target_field": "tags"
}
```

#### 1.2.4 嵌套数组访问
组合使用点号和索引：
```json
{
  "source_field": "custom_fields.affected_components.0",  // 获取嵌套数组的第一个元素
  "target_field": "service"
}
```

#### 1.2.5 深层嵌套
支持任意深度的嵌套：
```json
{
  "source_field": "data.issues.0.fields.labels.2",  // 深层嵌套访问
  "target_field": "tags"
}
```

### 1.3 默认映射规则

如果不配置字段映射，系统会自动尝试以下字段名：

| 目标字段 | 自动尝试的源字段名 |
|---------|------------------|
| external_id | `id`, `ticket_id`, `issue_id` |
| title | `title`, `summary`, `subject` |
| description | `description`, `details`, `body` |
| status | `status`, `state` |
| priority | `priority`, `urgency` |
| type | `type`, `issue_type`, `ticket_type` |
| reporter | `reporter`, `created_by`, `requester` |
| assignee | `assignee`, `assigned_to` |
| category | `category`, `classification` |
| service | `service`, `application` |

### 1.4 字段映射示例

#### 示例1：JIRA格式映射
```json
{
  "mappings": [
    {
      "source_field": "key",
      "target_field": "external_id",
      "required": true
    },
    {
      "source_field": "fields.summary",
      "target_field": "title",
      "required": true
    },
    {
      "source_field": "fields.description",
      "target_field": "description"
    },
    {
      "source_field": "fields.status.name",
      "target_field": "status"
    },
    {
      "source_field": "fields.priority.name",
      "target_field": "priority",
      "default_value": "medium"
    },
    {
      "source_field": "fields.issuetype.name",
      "target_field": "type"
    },
    {
      "source_field": "fields.reporter.displayName",
      "target_field": "reporter"
    },
    {
      "source_field": "fields.assignee.displayName",
      "target_field": "assignee"
    },
    {
      "source_field": "fields.labels",
      "target_field": "tags"
    }
  ]
}
```

#### 示例2：ServiceNow格式映射
```json
{
  "mappings": [
    {
      "source_field": "number",
      "target_field": "external_id",
      "required": true
    },
    {
      "source_field": "short_description",
      "target_field": "title",
      "required": true
    },
    {
      "source_field": "description",
      "target_field": "description"
    },
    {
      "source_field": "state",
      "target_field": "status"
    },
    {
      "source_field": "priority",
      "target_field": "priority"
    },
    {
      "source_field": "category",
      "target_field": "category"
    },
    {
      "source_field": "assigned_to.display_value",
      "target_field": "assignee"
    },
    {
      "source_field": "opened_by.display_value",
      "target_field": "reporter"
    }
  ]
}
```

#### 示例3：自定义系统映射（包含数组访问）
```json
{
  "mappings": [
    {
      "source_field": "ticket_number",
      "target_field": "external_id",
      "required": true
    },
    {
      "source_field": "subject",
      "target_field": "title",
      "required": true
    },
    {
      "source_field": "content",
      "target_field": "description"
    },
    {
      "source_field": "custom_fields.environment",
      "target_field": "tags",
      "default_value": "未分类"
    },
    {
      "source_field": "urgency_level",
      "target_field": "priority",
      "default_value": "low"
    },
    {
      "source_field": "custom_fields.affected_services.0",
      "target_field": "service"
    },
    {
      "source_field": "assigned_teams.0.name",
      "target_field": "assignee"
    }
  ]
}
```

## 二、过滤规则配置

### 2.1 规则结构

每个过滤规则包含以下字段：

| 字段 | 类型 | 说明 | 必需 |
|------|------|------|------|
| name | string | 规则名称 | 是 |
| field | string | 要检查的字段路径 | 是 |
| operator | string | 比较操作符 | 是 |
| value | string | 比较值 | 是 |
| action | string | 动作：include/exclude | 是 |
| enabled | bool | 是否启用 | 否 |
| priority | int | 优先级（自动分配） | 否 |

### 2.2 支持的操作符

| 操作符 | 说明 | 示例 |
|--------|------|------|
| `equals` | 等于 | status equals "open" |
| `not_equals` | 不等于 | status not_equals "closed" |
| `contains` | 包含 | title contains "紧急" |
| `not_contains` | 不包含 | description not_contains "测试" |
| `in` | 在列表中 | priority in "critical,high" |
| `not_in` | 不在列表中 | type not_in "test,demo" |
| `regex` | 正则匹配 | title regex "^PROD-.*" |
| `greater` | 大于 | created_days greater "7" |
| `less` | 小于 | updated_hours less "24" |

### 2.3 规则执行逻辑

1. **规则按数组顺序执行**（优先级自动按顺序分配）
2. **exclude规则优先**：如果匹配exclude规则，立即排除
3. **include规则必须全部满足**：所有include规则都必须匹配才会包含
4. **没有规则时默认包含所有**

### 2.4 过滤规则示例

#### 示例1：只同步生产环境的高优先级工单
```json
{
  "rules": [
    {
      "name": "只包含生产环境",
      "field": "tags",
      "operator": "contains",
      "value": "生产环境",
      "action": "include",
      "enabled": true
    },
    {
      "name": "只包含高优先级",
      "field": "priority",
      "operator": "in",
      "value": "critical,high",
      "action": "include",
      "enabled": true
    },
    {
      "name": "排除已解决的",
      "field": "status",
      "operator": "in",
      "value": "resolved,closed",
      "action": "exclude",
      "enabled": true
    }
  ]
}
```

#### 示例2：按工单类型过滤
```json
{
  "rules": [
    {
      "name": "只同步事件类工单",
      "field": "type",
      "operator": "in",
      "value": "incident,problem",
      "action": "include",
      "enabled": true
    },
    {
      "name": "排除测试工单",
      "field": "title",
      "operator": "contains",
      "value": "测试",
      "action": "exclude",
      "enabled": true
    }
  ]
}
```

#### 示例3：按时间过滤（需要外部系统支持）
```json
{
  "rules": [
    {
      "name": "只同步最近的工单",
      "field": "created_at",
      "operator": "greater",
      "value": "2024-01-01",
      "action": "include",
      "enabled": true
    }
  ]
}
```

#### 示例4：复杂的嵌套字段过滤
```json
{
  "rules": [
    {
      "name": "只同步特定项目",
      "field": "fields.project.key",
      "operator": "in",
      "value": "PROD,CORE,API",
      "action": "include",
      "enabled": true
    },
    {
      "name": "排除特定标签",
      "field": "fields.labels",
      "operator": "not_contains",
      "value": "ignore-sync",
      "action": "exclude",
      "enabled": true
    }
  ]
}
```

#### 示例5：使用正则表达式
```json
{
  "rules": [
    {
      "name": "只同步生产工单号",
      "field": "external_id",
      "operator": "regex",
      "value": "^PROD-\\d{4,}$",
      "action": "include",
      "enabled": true
    }
  ]
}
```

## 三、最佳实践

### 3.1 字段映射最佳实践

1. **始终映射必需字段**：`external_id` 和 `title` 是必需的
2. **使用默认值**：为可能缺失的字段设置合理的默认值
3. **注意数据类型**：确保源字段的数据类型与目标字段兼容
4. **测试嵌套字段**：使用测试同步功能验证嵌套字段路径是否正确

### 3.2 过滤规则最佳实践

1. **先宽后窄**：先设置包含规则，再设置排除规则
2. **使用明确的规则名称**：便于理解和维护
3. **定期检查规则效果**：使用测试同步查看哪些数据被过滤
4. **避免冲突规则**：确保include和exclude规则不会相互矛盾
5. **合理排序规则**：规则按数组顺序执行，重要的规则放在前面

### 3.3 性能优化建议

1. **减少规则数量**：过多的规则会影响同步性能
2. **优先使用简单操作符**：`equals`比`regex`性能更好
3. **合并相似规则**：使用`in`操作符代替多个`equals`规则
4. **定期清理无效规则**：删除不再使用的规则

## 四、故障排查

### 4.1 字段映射问题

**问题**：字段没有正确映射
- 检查源字段路径是否正确
- 使用测试同步查看原始数据结构
- 确认字段名大小写是否匹配

**问题**：嵌套字段无法访问
- 确认中间节点都存在
- 检查是否有拼写错误
- 对于数组字段，使用索引访问（如 `items.0` 获取第一个元素）
- 确保索引不超出数组范围

**问题**：数组访问返回空值
- 检查数组是否为空
- 确认索引是否正确（数组索引从0开始）
- 验证数据结构（某些系统可能返回对象而非数组）

### 4.2 过滤规则问题

**问题**：所有数据都被过滤了
- 检查include规则是否过于严格
- 确认字段值的格式是否正确
- 使用测试同步的`show_filtered`选项查看被过滤的原因

**问题**：不想要的数据没有被过滤
- 检查exclude规则是否正确
- 确认规则的优先级设置
- 验证操作符使用是否恰当

## 五、API快速参考

### 配置字段映射
```bash
POST /api/v1/ticket-plugins/{id}/field-mappings
{
  "mappings": [
    {
      "source_field": "源字段路径",
      "target_field": "目标字段名",
      "default_value": "默认值（可选）",
      "required": true/false
    }
  ]
}
```

**支持的源字段路径格式：**
- 简单字段：`status`
- 嵌套对象：`fields.priority.name`
- 数组元素：`tags.0`
- 嵌套数组：`custom_fields.components.0.name`
- 深层路径：`data.issues.0.fields.labels.1`

### 配置过滤规则
```bash
POST /api/v1/ticket-plugins/{id}/sync-rules
{
  "rules": [
    {
      "name": "规则名称",
      "field": "字段路径",
      "operator": "操作符",
      "value": "比较值",
      "action": "include或exclude",
      "enabled": true
    }
  ]
}
```

**注意**：
- 使用数组格式，可以一次配置多个规则
- 规则优先级按数组顺序自动设置
- 每次调用会替换所有现有规则
- `enabled` 字段可选，默认为 true

### 测试配置效果
```bash
POST /api/v1/ticket-plugins/{id}/test-sync
{
  "plugin_params": {},
  "test_options": {
    "sample_size": 5,
    "show_filtered": true,
    "show_mapping_details": true
  }
}
```