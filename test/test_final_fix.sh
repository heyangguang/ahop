#!/bin/bash

# 测试最终修复

# 获取token
TOKEN=$(curl -s -X POST 'http://localhost:8080/api/v1/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"username": "admin", "password": "Admin@123"}' | jq -r '.data.token')

echo "=== 测试自愈规则 ==="

# 触发一次规则更新来刷新 next_run_at
echo "1. 禁用再启用规则..."
curl -s -X PUT "http://localhost:8080/api/v1/healing/rules/12" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"is_active": false}' > /dev/null

sleep 1

curl -s -X PUT "http://localhost:8080/api/v1/healing/rules/12" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"is_active": true}' > /dev/null

sleep 1

echo -e "\n2. 查看规则详情..."
curl -s -X GET "http://localhost:8080/api/v1/healing/rules/12" \
  -H "Authorization: Bearer $TOKEN" | jq '{
    data: {
      id: .data.id,
      name: .data.name,
      next_run_at: .data.next_run_at,
      tenant: .data.tenant | {id, name, code},
      workflow_tenant: .data.workflow.tenant | {id, name, code}
    }
  }'

echo -e "\n=== 测试工单插件 ==="

# 触发一次插件更新来刷新 next_run_at
echo "1. 禁用再启用插件..."
curl -s -X POST "http://localhost:8080/api/v1/ticket-plugins/1/disable" \
  -H "Authorization: Bearer $TOKEN" > /dev/null

sleep 1

curl -s -X POST "http://localhost:8080/api/v1/ticket-plugins/1/enable" \
  -H "Authorization: Bearer $TOKEN" > /dev/null

sleep 1

echo -e "\n2. 查看插件详情..."
curl -s -X GET "http://localhost:8080/api/v1/ticket-plugins/1" \
  -H "Authorization: Bearer $TOKEN" | jq '{
    data: {
      id: .data.id,
      name: .data.name,
      sync_interval: .data.sync_interval,
      last_sync_at: .data.last_sync_at,
      next_run_at: .data.next_run_at
    }
  }'