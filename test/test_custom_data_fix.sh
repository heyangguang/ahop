#!/bin/bash

# 测试 custom_data 字段是否正确处理

echo "========================================="
echo "测试 custom_data 双重编码修复"
echo "========================================="

# 数据库连接信息
DB_USER="postgres"
DB_NAME="auto_healing_platform"
DB_PASSWORD="Admin@123"
export PGPASSWORD=$DB_PASSWORD

echo -e "\n1. 检查数据库中的原始 ticket 数据..."
psql -U $DB_USER -d $DB_NAME -c "
SELECT 
    id,
    external_id,
    title,
    substring(custom_data::text, 1, 100) as custom_data_preview
FROM tickets 
WHERE custom_data IS NOT NULL 
LIMIT 5;"

echo -e "\n2. 检查 healing_executions 表中的 trigger_source 数据..."
psql -U $DB_USER -d $DB_NAME -c "
SELECT 
    id,
    execution_id,
    workflow_id,
    substring(trigger_source::text, 1, 200) as trigger_source_preview
FROM healing_executions 
WHERE trigger_source IS NOT NULL 
  AND trigger_source::text LIKE '%matched_item%'
ORDER BY created_at DESC
LIMIT 5;"

echo -e "\n3. 检查最新的 trigger_source 中的 custom_data 是否还是 base64 编码..."
psql -U $DB_USER -d $DB_NAME -t -c "
SELECT trigger_source::text
FROM healing_executions 
WHERE trigger_source IS NOT NULL 
  AND trigger_source::text LIKE '%custom_data%'
ORDER BY created_at DESC
LIMIT 1;" | python3 -c "
import sys
import json

try:
    data = sys.stdin.read().strip()
    if data:
        trigger = json.loads(data)
        if 'matched_item' in trigger and 'custom_data' in trigger['matched_item']:
            custom_data = trigger['matched_item']['custom_data']
            print(f'\\nCustom data type: {type(custom_data).__name__}')
            if isinstance(custom_data, str):
                print(f'Custom data is string, length: {len(custom_data)}')
                print(f'First 100 chars: {custom_data[:100]}')
                # 检查是否是 base64
                if all(c in 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=' for c in custom_data.replace('\\n', '')):
                    print('⚠️  看起来仍然是 base64 编码的数据！')
                else:
                    print('✅ 不是 base64 编码的数据')
            elif isinstance(custom_data, dict):
                print('✅ Custom data 是字典对象（正确）')
                print(f'Keys: {list(custom_data.keys())}')
            else:
                print(f'Custom data 是 {type(custom_data).__name__} 类型')
except Exception as e:
    print(f'解析错误: {e}')
"

echo -e "\n4. 测试新的工作流执行..."
echo "需要手动触发一个自愈规则来测试新代码是否生效"