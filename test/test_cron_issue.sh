#!/bin/bash

# 测试cron表达式问题

API_URL="http://localhost:8080/api/v1"

# 登录
LOGIN_RESPONSE=$(curl -s -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{
        "username": "admin",
        "password": "Admin@123"
    }')

TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.token')

# 创建工作流
CREATE_WORKFLOW_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "测试工作流",
        "code": "test_workflow",
        "description": "测试cron表达式问题",
        "definition": {
            "nodes": [
                {"id": "start", "name": "开始", "type": "start", "config": {}, "next_nodes": ["end"]},
                {"id": "end", "name": "结束", "type": "end", "config": {}, "next_nodes": []}
            ],
            "connections": [{"from": "start", "to": "end"}],
            "variables": {}
        }
    }')

WORKFLOW_ID=$(echo $CREATE_WORKFLOW_RESPONSE | jq -r '.data.id')
echo "工作流ID: $WORKFLOW_ID"

# 使用单引号避免通配符展开
CREATE_RULE_RESPONSE=$(curl -s -X POST "${API_URL}/healing/rules" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{
        \"name\": \"测试规则\",
        \"code\": \"test_rule\",
        \"description\": \"测试cron表达式\",
        \"trigger_type\": \"scheduled\",
        \"cron_expr\": \"*/5 * * * *\",
        \"match_rules\": {
            \"source\": \"ticket\",
            \"field\": \"title\",
            \"operator\": \"contains\",
            \"value\": \"test\"
        },
        \"priority\": 10,
        \"workflow_id\": $WORKFLOW_ID
    }")

echo "创建规则响应："
echo $CREATE_RULE_RESPONSE | jq '.'

RULE_ID=$(echo $CREATE_RULE_RESPONSE | jq -r '.data.id')

# 获取规则详情
GET_RULE_RESPONSE=$(curl -s -X GET "${API_URL}/healing/rules/${RULE_ID}" \
    -H "Authorization: Bearer ${TOKEN}")

echo -e "\n规则详情："
echo $GET_RULE_RESPONSE | jq '.data | {id, code, cron_expr}'

# 清理
curl -s -X DELETE "${API_URL}/healing/rules/${RULE_ID}" -H "Authorization: Bearer ${TOKEN}" > /dev/null
curl -s -X DELETE "${API_URL}/healing/workflows/${WORKFLOW_ID}" -H "Authorization: Bearer ${TOKEN}" > /dev/null

echo -e "\n测试完成"