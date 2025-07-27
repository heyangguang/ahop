# 任务模板独立存储设计文档

## 背景

原设计中，任务模板与Git仓库通过外键（repository_id）绑定。这种设计存在以下问题：

1. **Gap问题**：Git仓库内容可能更新，但任务模板不会自动同步，导致执行时使用的代码与创建时不一致
2. **依赖问题**：删除Git仓库会影响任务模板的使用
3. **灵活性不足**：无法独立管理任务模板

## 新设计方案

### 1. 数据库结构变更

移除 `repository_id` 外键，改用 `source_git_info` JSONB字段存储Git来源信息：

```sql
-- 原结构
repository_id  uint  -- Git仓库ID（外键）

-- 新结构  
source_git_info  JSONB  -- Git来源信息（快照）
```

`source_git_info` 字段示例：
```json
{
  "repository_id": 1,
  "repository_name": "运维脚本库",
  "repository_url": "https://github.com/example/ops-scripts.git",
  "branch": "main",
  "original_path": "scripts/disk_cleanup.sh",
  "created_at": "2024-01-20 10:30:00"
}
```

### 2. 文件存储结构

任务模板文件独立存储在Worker端：

```
/opt/ahop-worker/templates/
├── {tenant_id}/
│   └── {template_code}/
│       ├── entry.sh              # 入口文件
│       ├── lib/                  # 依赖文件
│       └── config/               # 配置文件
```

### 3. 创建流程

1. 用户通过扫描Git仓库获取可用的脚本模板
2. 选择要导入的脚本，调整参数后创建任务模板
3. 主服务端：
   - 保存模板元数据到数据库
   - 记录Git来源信息（快照）
   - 发送消息到Worker队列
4. Worker端：
   - 接收文件复制消息
   - 从Git仓库复制文件到独立目录
   - 确认复制完成

### 4. 执行流程

1. 用户创建任务，选择任务模板
2. 主服务端将任务推送到队列
3. Worker端：
   - 从独立的模板目录读取文件
   - 不再依赖Git仓库
   - 执行任务

### 5. 实现进度

- [x] 主服务端数据库结构调整
- [x] TaskTemplateService改造
- [x] 数据库迁移脚本
- [ ] Worker端文件复制逻辑
- [ ] Worker端任务执行适配

### 6. API变更

#### 创建任务模板

请求中需要包含 `original_path` 字段：

```json
POST /api/v1/task-templates
{
  "name": "磁盘清理脚本",
  "code": "disk_cleanup",
  "script_type": "shell",
  "entry_file": "disk_cleanup.sh",
  "repository_id": 1,              // 临时使用，用于查找Git仓库信息
  "original_path": "scripts/disk_cleanup.sh",  // Git仓库中的原始路径
  ...
}
```

#### 列表查询

不再支持按 `repository_id` 过滤：

```json
GET /api/v1/task-templates?script_type=shell&search=disk
```

### 7. 数据迁移

对于现有数据，通过迁移脚本将 `repository_id` 关联信息转换为 `source_git_info`：

```sql
UPDATE task_templates t
SET source_git_info = jsonb_build_object(
    'repository_id', t.repository_id,
    'repository_name', r.name,
    'repository_url', r.url,
    'branch', r.branch,
    'original_path', t.entry_file,
    'created_at', to_char(t.created_at, 'YYYY-MM-DD HH24:MI:SS')
)
FROM git_repositories r
WHERE t.repository_id = r.id;
```

### 8. 优势

1. **版本稳定性**：任务模板使用的代码版本固定，不受Git仓库更新影响
2. **独立管理**：可以独立编辑、版本控制任务模板
3. **性能优化**：执行时直接读取本地文件，无需Git操作
4. **可追溯性**：保留Git来源信息，知道模板的出处

### 9. 注意事项

1. Worker端需要足够的磁盘空间存储模板文件
2. 需要定期清理不再使用的模板文件
3. 多Worker环境下需要确保所有Worker都有相同的模板文件