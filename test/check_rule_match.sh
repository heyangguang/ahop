#!/bin/bash

# 检查规则的匹配规则

TOKEN=$(curl -s -X POST 'http://localhost:8080/api/v1/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"username": "admin", "password": "Admin@123"}' | jq -r '.data.token')

echo "=== 规则详情 ==="
curl -s -X GET "http://localhost:8080/api/v1/healing/rules/12" \
  -H "Authorization: Bearer $TOKEN" | jq '.data | {id, name, trigger_type, match_rules}'