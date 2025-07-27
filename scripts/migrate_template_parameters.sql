-- 任务模板参数结构迁移脚本
-- 将旧的参数格式转换为新的前端友好格式

-- 备份原始数据
CREATE TABLE IF NOT EXISTS task_templates_backup_params AS 
SELECT id, parameters, updated_at 
FROM task_templates 
WHERE parameters IS NOT NULL;

-- 更新参数格式
UPDATE task_templates 
SET parameters = (
    SELECT jsonb_agg(
        jsonb_build_object(
            'name', COALESCE(
                param->>'name',           -- 优先使用已有的 name
                param->>'variable'        -- 否则使用 variable
            ),
            'type', CASE 
                -- 类型标准化
                WHEN param->>'type' IN ('string', 'text') THEN 'text'
                WHEN param->>'type' = 'textarea' THEN 'textarea'
                WHEN param->>'type' IN ('integer', 'int', 'float') THEN 'number'
                WHEN param->>'type' IN ('password', 'secret') THEN 'password'
                WHEN param->>'type' IN ('select', 'multiplechoice') THEN 'select'
                WHEN param->>'type' = 'multiselect' THEN 'multiselect'
                ELSE COALESCE(param->>'type', 'text')
            END,
            'label', COALESCE(
                param->>'label',          -- 优先使用已有的 label
                param->>'question_name',  -- 其次使用 question_name
                param->>'name',           -- 再次使用 name
                param->>'variable'        -- 最后使用 variable
            ),
            'description', COALESCE(
                param->>'description',
                param->>'question_description',
                ''
            ),
            'required', COALESCE(
                (param->>'required')::boolean,
                false
            ),
            'default', COALESCE(
                param->'default',
                param->'default_value',
                null
            ),
            'options', COALESCE(
                param->'options',
                param->'choices',
                null
            ),
            'validation', CASE 
                WHEN param->>'min' IS NOT NULL OR param->>'max' IS NOT NULL 
                THEN jsonb_build_object(
                    'min', CASE 
                        WHEN param->>'min' ~ '^\d+$' THEN (param->>'min')::int
                        ELSE NULL
                    END,
                    'max', CASE 
                        WHEN param->>'max' ~ '^\d+$' THEN (param->>'max')::int
                        ELSE NULL
                    END
                )
                ELSE NULL
            END,
            'source', COALESCE(
                param->>'source',
                'migrated'  -- 标记为迁移的数据
            )
        )
    )
    FROM jsonb_array_elements(parameters) AS param
)
WHERE parameters IS NOT NULL 
  AND jsonb_typeof(parameters) = 'array';

-- 记录迁移信息
DO $$
DECLARE
    migrated_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO migrated_count 
    FROM task_templates 
    WHERE parameters IS NOT NULL;
    
    RAISE NOTICE '成功迁移了 % 个任务模板的参数格式', migrated_count;
END $$;

-- 验证迁移结果（可选）
-- SELECT id, name, 
--        jsonb_pretty(parameters) as formatted_params 
-- FROM task_templates 
-- WHERE parameters IS NOT NULL 
-- LIMIT 5;