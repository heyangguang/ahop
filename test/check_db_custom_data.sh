#!/bin/bash

echo "检查数据库中 custom_data 的原始值..."

# 使用环境变量
source /opt/ahop/.env

# 构建psql命令
PSQL_CMD="psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME"

# 设置密码
export PGPASSWORD=$DB_PASSWORD

# 查询最新的几条工单记录
echo "=== 最新的工单记录 ==="
$PSQL_CMD -c "
SELECT 
    id,
    external_id,
    title,
    length(custom_data::text) as custom_data_length,
    substring(custom_data::text, 1, 100) as custom_data_preview
FROM tickets 
ORDER BY created_at DESC 
LIMIT 5;
"

# 查询特定工单的 custom_data
echo -e "\n=== 查看特定工单的完整 custom_data ==="
$PSQL_CMD -c "
SELECT 
    external_id,
    custom_data::text as custom_data
FROM tickets 
WHERE external_id = 'DISK-0033-D24374'
LIMIT 1;
"

# 检查是否有空的 custom_data
echo -e "\n=== 检查 custom_data 为空的记录 ==="
$PSQL_CMD -c "
SELECT count(*) as empty_custom_data_count
FROM tickets 
WHERE custom_data IS NULL OR custom_data::text = 'null';
"