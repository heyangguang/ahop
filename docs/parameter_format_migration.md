# 任务模板参数格式迁移指南

## 概述

为了提供更好的前端开发体验，我们统一了任务模板参数的数据结构，使字段名更加直观和语义化。

## 旧格式 vs 新格式

### 旧格式（AWX 风格）
```json
{
  "variable": "message",
  "type": "text",
  "question_name": "测试消息",
  "question_description": "要显示的消息内容",
  "required": false,
  "default": "Hello",
  "choices": ["option1", "option2"],
  "min": 1,
  "max": 10
}
```

### 新格式（前端友好）
```json
{
  "name": "message",              // variable → name
  "type": "text",                 // 类型标准化
  "label": "测试消息",             // question_name → label
  "description": "要显示的消息内容", // question_description → description
  "required": false,
  "default": "Hello",             // default_value → default
  "options": ["option1", "option2"], // choices → options
  "validation": {                 // 验证规则嵌套
    "min": 1,
    "max": 10
  },
  "source": "scanner"             // 新增：参数来源
}
```

## 字段映射说明

| 旧字段名 | 新字段名 | 说明 |
|---------|---------|------|
| `variable` | `name` | 参数名（执行时使用的变量名） |
| `question_name` | `label` | 显示标签（UI显示） |
| `question_description` | `description` | 详细描述 |
| `default` 或 `default_value` | `default` | 默认值 |
| `choices` | `options` | 选项列表 |
| `min`, `max` | `validation.min`, `validation.max` | 验证规则嵌套在 validation 对象中 |

## 类型标准化

| Scanner 类型 | 新标准类型 | 说明 |
|-------------|-----------|------|
| `text`, `string` | `text` | 单行文本 |
| `textarea` | `textarea` | 多行文本 |
| `integer`, `int`, `float` | `number` | 数字类型 |
| `password`, `secret` | `password` | 密码类型 |
| `multiplechoice` | `select` | 单选 |
| `multiselect` | `multiselect` | 多选 |

## 执行时的使用

### Shell 脚本执行
```go
// 使用 name 字段作为命令行参数
for name, value := range variables {
    cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", name))
    cmdArgs = append(cmdArgs, value)
}
// 结果: script.sh --message "Hello" --count 5
```

### Ansible 执行
```go
// 使用 name 字段作为 Ansible 变量
for name, value := range variables {
    args = append(args, "-e", fmt.Sprintf("%s=%v", name, value))
}
// 结果: ansible-playbook -e message="Hello" -e count=5
```

## 前端使用示例

### 参数展示
```javascript
function renderParameter(param) {
    return `
        <div class="form-group">
            <label>${param.label}</label>
            ${param.description ? `<small>${param.description}</small>` : ''}
            <input 
                type="${getInputType(param.type)}"
                name="${param.name}"
                placeholder="${param.default || ''}"
                ${param.required ? 'required' : ''}
            />
        </div>
    `;
}
```

### 提交任务
```javascript
// 构建 variables 对象，使用 name 作为 key
const variables = {};
parameters.forEach(param => {
    variables[param.name] = formData[param.name];
});

const task = {
    template_id: templateId,
    variables: variables,
    hosts: selectedHosts
};
```

## 数据迁移

运行迁移脚本：
```bash
psql -U postgres -d auto_healing_platform -f scripts/migrate_template_parameters.sql
```

## 注意事项

1. **关键字段**：`name` 字段是最重要的，它必须与脚本中的参数名完全一致
2. **向后兼容**：迁移脚本会自动处理旧数据
3. **验证规则**：新格式将验证规则嵌套在 `validation` 对象中，结构更清晰