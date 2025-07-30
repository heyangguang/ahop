#!/bin/bash

# 测试更新自愈规则的下次执行时间

# 获取token
TOKEN=$(curl -s -X POST 'http://localhost:8080/api/v1/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"username": "admin", "password": "Admin@123"}' | jq -r '.data.token')

echo "=== 当前规则状态 ==="
curl -s -X GET "http://localhost:8080/api/v1/healing/rules/12" \
  -H "Authorization: Bearer $TOKEN" | jq '.data | {id, name, is_active, next_run_at}'

echo -e "\n=== 触发规则更新（切换激活状态两次） ==="
# 先禁用
echo "禁用规则..."
curl -s -X PUT "http://localhost:8080/api/v1/healing/rules/12" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "is_active": false
  }' | jq '.message'

sleep 1

# 再启用
echo "启用规则..."
curl -s -X PUT "http://localhost:8080/api/v1/healing/rules/12" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "is_active": true
  }' | jq '.message'

sleep 2

echo -e "\n=== 更新后的规则状态 ==="
curl -s -X GET "http://localhost:8080/api/v1/healing/rules/12" \
  -H "Authorization: Bearer $TOKEN" | jq '.data | {id, name, is_active, next_run_at}'

echo -e "\n=== 调度器中的状态 ==="
curl -s -X GET "http://localhost:8080/api/v1/healing/rules/scheduler-status" \
  -H "Authorization: Bearer $TOKEN" | jq '.data.rules[] | select(.rule_id == 12) | {rule_id, next_run_at, next_run_in}'