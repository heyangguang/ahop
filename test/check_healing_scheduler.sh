#!/bin/bash

# 检查自愈规则调度器状态

# 获取token
TOKEN=$(curl -s -X POST 'http://localhost:8080/api/v1/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"username": "admin", "password": "Admin@123"}' | jq -r '.data.token')

echo "=== 自愈规则调度器状态 ==="
curl -s -X GET "http://localhost:8080/api/v1/healing/rules/scheduler-status" \
  -H "Authorization: Bearer $TOKEN" | jq '.'

echo -e "\n=== 获取规则ID 12 的详情 ==="
curl -s -X GET "http://localhost:8080/api/v1/healing/rules/12" \
  -H "Authorization: Bearer $TOKEN" | jq '.data | {id, name, cron_expr, is_active, last_execute_at, next_run_at}'

echo -e "\n=== 查看系统调度器总览 ==="
curl -s -X GET "http://localhost:8080/api/v1/system/schedulers" \
  -H "Authorization: Bearer $TOKEN" | jq '.data[] | select(.name == "自愈规则调度器")'