#!/bin/bash

echo "1. 生成测试工单..."
curl -X POST http://localhost:5002/generate-test-tickets \
  -H "Content-Type: application/json" \
  -d '{"count": 1, "type": "disk"}' | jq '.'

echo -e "\n2. 查看插件返回的原始数据..."
curl -s http://localhost:5002/tickets?minutes=5 | jq '.data[0]' | tee /tmp/original_ticket.json

echo -e "\n3. 触发工单同步..."
curl -X POST "http://localhost:8080/api/v1/ticket-plugins/1/sync" \
  -H "Authorization: Bearer $JWT_TOKEN" | jq '.'

echo -e "\n4. 等待同步完成..."
sleep 3

echo -e "\n5. 查看同步后的工单..."
curl -s "http://localhost:8080/api/v1/tickets?page=1&page_size=1" \
  -H "Authorization: Bearer $JWT_TOKEN" | jq '.data[0]' | tee /tmp/synced_ticket.json

echo -e "\n6. 比较 custom_data..."
echo "原始数据中的 custom_fields:"
jq '.custom_fields' /tmp/original_ticket.json

echo -e "\n同步后的 custom_data:"
jq '.custom_data' /tmp/synced_ticket.json

# 检查是否是 base64
echo -e "\n7. 尝试 base64 解码..."
jq -r '.custom_data' /tmp/synced_ticket.json | base64 -d 2>/dev/null && echo "(成功解码)" || echo "(不是 base64)"