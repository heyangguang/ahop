#!/bin/bash

# 查看队列API

# 获取token
TOKEN=$(curl -s -X POST 'http://localhost:8080/api/v1/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"username": "admin", "password": "Admin@123"}' | jq -r '.data.token')

echo "Token获取成功"

# 测试队列API是否存在
echo -e "\n=== 测试队列API ==="
RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X GET "http://localhost:8080/api/v1/queue/tasks" \
  -H "Authorization: Bearer $TOKEN")

HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d: -f2)
BODY=$(echo "$RESPONSE" | grep -v "HTTP_CODE:")

echo "HTTP状态码: $HTTP_CODE"

if [ "$HTTP_CODE" = "404" ]; then
    echo "队列API未找到！看起来 QueueHandler 还没有注册到路由中。"
else
    echo "响应内容:"
    echo "$BODY" | jq '.'
fi