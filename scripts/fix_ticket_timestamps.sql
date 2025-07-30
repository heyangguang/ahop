-- 修复工单的外部时间字段
-- 将异常的时间值（0001-01-01）设置为 NULL

-- 先查看有多少条记录需要修复
SELECT COUNT(*) as total_tickets,
       COUNT(CASE WHEN external_created_at < '1900-01-01' THEN 1 END) as invalid_created_at,
       COUNT(CASE WHEN external_updated_at < '1900-01-01' THEN 1 END) as invalid_updated_at
FROM tickets;

-- 修复异常的时间值
UPDATE tickets 
SET external_created_at = NULL 
WHERE external_created_at < '1900-01-01';

UPDATE tickets 
SET external_updated_at = NULL 
WHERE external_updated_at < '1900-01-01';

-- 验证修复结果
SELECT COUNT(*) as fixed_tickets
FROM tickets 
WHERE external_created_at IS NULL OR external_updated_at IS NULL;