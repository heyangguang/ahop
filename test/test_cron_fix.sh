#!/bin/bash

API_URL="http://localhost:8080/api/v1"

# 登录
TOKEN=$(curl -s -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username": "admin", "password": "Admin@123"}' | jq -r '.data.token')

# 创建工作流
WORKFLOW_ID=$(curl -s -X POST "${API_URL}/healing/workflows" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "测试工作流",
        "code": "test_workflow_fix",
        "description": "测试cron修复",
        "definition": {
            "nodes": [
                {"id": "start", "name": "开始", "type": "start", "config": {}, "next_nodes": ["end"]},
                {"id": "end", "name": "结束", "type": "end", "config": {}, "next_nodes": []}
            ],
            "connections": [{"from": "start", "to": "end"}],
            "variables": {}
        }
    }' | jq -r '.data.id')

echo "工作流ID: $WORKFLOW_ID"

# 使用Here Document避免通配符展开
RULE_ID=$(curl -s -X POST "${API_URL}/healing/rules" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d @- <<EOF | jq -r '.data.id'
{
    "name": "测试规则",
    "code": "test_rule_fix",
    "description": "测试cron表达式",
    "trigger_type": "scheduled",
    "cron_expr": "*/5 * * * *",
    "match_rules": {
        "source": "ticket",
        "field": "title",
        "operator": "contains",
        "value": "test"
    },
    "priority": 10,
    "workflow_id": $WORKFLOW_ID
}
EOF
)

echo "规则ID: $RULE_ID"

# 获取并显示cron表达式
curl -s -X GET "${API_URL}/healing/rules/${RULE_ID}" \
    -H "Authorization: Bearer ${TOKEN}" | jq '.data.cron_expr'

# 清理
curl -s -X DELETE "${API_URL}/healing/rules/${RULE_ID}" -H "Authorization: Bearer ${TOKEN}"
curl -s -X DELETE "${API_URL}/healing/workflows/${WORKFLOW_ID}" -H "Authorization: Bearer ${TOKEN}"