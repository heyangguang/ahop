#!/bin/bash

# 测试刷新规则

TOKEN=$(curl -s -X POST 'http://localhost:8080/api/v1/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"username": "admin", "password": "Admin@123"}' | jq -r '.data.token')

echo "=== 当前规则状态 ==="
curl -s -X GET "http://localhost:8080/api/v1/healing/rules/12" \
  -H "Authorization: Bearer $TOKEN" | jq '.data | {id, name, next_run_at, cron_expr}'

echo -e "\n=== 触发一次微小更新来刷新调度器 ==="
# 更新描述字段来触发刷新
curl -s -X PUT "http://localhost:8080/api/v1/healing/rules/12" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "检测到磁盘空间不足相关工单时，自动执行磁盘清理工作流 (updated)"
  }' | jq '.message'

sleep 2

echo -e "\n=== 更新后的规则状态 ==="
curl -s -X GET "http://localhost:8080/api/v1/healing/rules/12" \
  -H "Authorization: Bearer $TOKEN" | jq '.data | {id, name, next_run_at, cron_expr}'

echo -e "\n=== 查看调度器状态 ==="
curl -s -X GET "http://localhost:8080/api/v1/healing/rules/scheduler-status" \
  -H "Authorization: Bearer $TOKEN" | jq '.data.rules[] | select(.rule_id == 12) | {rule_id, next_run_at, next_run_in}'