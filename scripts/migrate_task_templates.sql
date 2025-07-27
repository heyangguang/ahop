-- 任务模板独立存储改造迁移脚本

-- 1. 添加 source_git_info 字段
ALTER TABLE task_templates 
ADD COLUMN IF NOT EXISTS source_git_info JSONB;

-- 2. 将现有的 repository_id 信息迁移到 source_git_info
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
WHERE t.repository_id = r.id
  AND t.repository_id IS NOT NULL
  AND t.source_git_info IS NULL;

-- 3. 删除 repository_id 上的索引
DROP INDEX IF EXISTS idx_task_templates_repository_id;

-- 4. 删除 repository_id 外键约束
ALTER TABLE task_templates 
DROP CONSTRAINT IF EXISTS fk_task_templates_repository;

-- 5. 删除 repository_id 字段
-- 注意：这一步建议在确认数据迁移成功后再执行
-- ALTER TABLE task_templates DROP COLUMN repository_id;

-- 6. 添加索引优化查询
CREATE INDEX IF NOT EXISTS idx_task_templates_source_git_info ON task_templates USING GIN (source_git_info);