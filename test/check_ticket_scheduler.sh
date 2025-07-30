#!/bin/bash

# 检查工单同步调度器状态

# 获取token
TOKEN=$(curl -s -X POST 'http://localhost:8080/api/v1/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"username": "admin", "password": "Admin@123"}' | jq -r '.data.token')

echo "=== 工单插件调度器状态 ==="
curl -s -X GET "http://localhost:8080/api/v1/ticket-plugins/scheduler-status" \
  -H "Authorization: Bearer $TOKEN" | jq '.'

echo -e "\n=== 查看所有工单插件 ==="
curl -s -X GET "http://localhost:8080/api/v1/ticket-plugins" \
  -H "Authorization: Bearer $TOKEN" | jq '.data[] | {id, name, sync_enabled, sync_interval, last_sync_at, next_run_at}'