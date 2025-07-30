#!/bin/bash

# 简化的任务检查脚本

# 获取token
TOKEN=$(curl -s -X POST 'http://localhost:8080/api/v1/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"username": "admin", "password": "Admin@123"}' | jq -r '.data.token')

echo "Token获取成功"

# 查看任务列表
echo -e "\n=== 任务列表 ==="
curl -s -X GET "http://localhost:8080/api/v1/tasks?page_size=10" \
  -H "Authorization: Bearer $TOKEN" | jq '.data' | head -100