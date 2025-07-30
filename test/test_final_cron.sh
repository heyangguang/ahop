#!/bin/bash

set -f  # 禁用通配符展开

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
        "name": "最终测试工作流",
        "code": "final_test_workflow",
        "description": "最终测试",
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

# 创建规则
RULE_DATA=$(cat <<'EOF'
{
    "name": "最终测试规则",
    "code": "final_test_rule",
    "description": "测试cron表达式最终修复",
    "trigger_type": "scheduled",
    "cron_expr": "*/5 * * * *",
    "match_rules": {
        "source": "ticket",
        "field": "title",
        "operator": "contains",
        "value": "test"
    },
    "priority": 10,
    "workflow_id": WORKFLOW_ID_PLACEHOLDER
}
EOF
)

# 替换占位符
RULE_DATA=${RULE_DATA/WORKFLOW_ID_PLACEHOLDER/$WORKFLOW_ID}

RULE_ID=$(curl -s -X POST "${API_URL}/healing/rules" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$RULE_DATA" | jq -r '.data.id')

echo "规则ID: $RULE_ID"

# 获取并显示cron表达式
CRON_EXPR=$(curl -s -X GET "${API_URL}/healing/rules/${RULE_ID}" \
    -H "Authorization: Bearer ${TOKEN}" | jq -r '.data.cron_expr')

echo "保存的cron表达式: $CRON_EXPR"

# 验证
if [ "$CRON_EXPR" = "*/5 * * * *" ]; then
    echo "✅ cron表达式保存正确!"
else
    echo "❌ cron表达式保存错误!"
fi

# 清理
curl -s -X DELETE "${API_URL}/healing/rules/${RULE_ID}" -H "Authorization: Bearer ${TOKEN}" > /dev/null
curl -s -X DELETE "${API_URL}/healing/workflows/${WORKFLOW_ID}" -H "Authorization: Bearer ${TOKEN}" > /dev/null

set +f  # 恢复通配符展开