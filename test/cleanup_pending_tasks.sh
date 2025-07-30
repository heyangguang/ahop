#!/bin/bash
# 清理 pending 状态的 healing_workflow 任务

echo "清理 pending 状态的 healing_workflow 任务..."

# 需要数据库连接信息
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_USER=${DB_USER:-postgres}
DB_NAME=${DB_NAME:-auto_healing_platform}

# 使用 PGPASSWORD 环境变量
export PGPASSWORD="${DB_PASSWORD}"

# 先统计
echo "统计 pending 任务..."
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -t -c "
SELECT COUNT(*) FROM tasks 
WHERE status = 'pending' 
AND task_type = 'healing_workflow';"

# 询问是否继续
read -p "是否将这些任务标记为失败？(y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]
then
    # 执行更新
    psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "
    UPDATE tasks 
    SET status = 'failed',
        error_msg = '任务类型已废弃，未能入队',
        finished_at = NOW(),
        updated_at = NOW()
    WHERE status = 'pending' 
    AND task_type = 'healing_workflow';"
    
    echo "清理完成！"
fi