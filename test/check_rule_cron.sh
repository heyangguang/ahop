#!/bin/bash

# 检查规则的cron表达式

TOKEN=$(curl -s -X POST 'http://localhost:8080/api/v1/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"username": "admin", "password": "Admin@123"}' | jq -r '.data.token')

echo "=== 规则的cron表达式 ==="
curl -s -X GET "http://localhost:8080/api/v1/healing/rules/12" \
  -H "Authorization: Bearer $TOKEN" | jq '.data | {id, name, cron_expr, is_active, next_run_at}'

echo -e "\n=== 调度器状态 ==="
curl -s -X GET "http://localhost:8080/api/v1/healing/rules/scheduler-status" \
  -H "Authorization: Bearer $TOKEN" | jq '.data.rules[] | select(.rule_id == 12)'