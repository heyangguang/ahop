#!/bin/bash

# 测试新的条件节点格式

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# API 基础 URL
API_URL="http://localhost:8080/api/v1"

# 打印信息
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 登录
print_info "登录系统..."
LOGIN_RESPONSE=$(curl -s -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{
        "username": "admin",
        "password": "Admin@123"
    }')

TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.token')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    print_error "登录失败"
    echo $LOGIN_RESPONSE | jq
    exit 1
fi
print_info "登录成功"

# 创建使用新格式条件节点的工作流
print_info "创建使用新格式条件节点的工作流..."
CREATE_WORKFLOW_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "新格式条件节点测试工作流",
        "code": "new_format_condition_test",
        "description": "测试统一使用 next_nodes 的条件节点",
        "definition": {
            "nodes": [
                {
                    "id": "start",
                    "name": "开始",
                    "type": "start",
                    "config": {},
                    "next_nodes": ["check_service"]
                },
                {
                    "id": "check_service",
                    "name": "检查服务状态",
                    "type": "task_execute",
                    "config": {
                        "task_template_code": "check_service_health",
                        "hosts": "{{affected_hosts}}",
                        "task_params": {
                            "service_name": "{{service_name}}"
                        }
                    },
                    "next_nodes": ["condition_service_health"]
                },
                {
                    "id": "condition_service_health",
                    "name": "服务是否健康",
                    "type": "condition",
                    "config": {
                        "expression": "task_result.status == \"success\""
                    },
                    "next_nodes": ["service_healthy", "restart_service"]
                },
                {
                    "id": "service_healthy",
                    "name": "服务健康",
                    "type": "ticket_update",
                    "config": {
                        "ticket_var": "ticket",
                        "updates": {
                            "comment": "服务状态正常，无需处理"
                        }
                    },
                    "next_nodes": ["end"]
                },
                {
                    "id": "restart_service",
                    "name": "重启服务",
                    "type": "task_execute",
                    "config": {
                        "task_template_code": "restart_service",
                        "hosts": "{{affected_hosts}}",
                        "task_params": {
                            "service_name": "{{service_name}}"
                        }
                    },
                    "next_nodes": ["validate_restart"]
                },
                {
                    "id": "validate_restart",
                    "name": "验证重启结果",
                    "type": "condition",
                    "config": {
                        "expression": "task_result.status == \"success\""
                    },
                    "next_nodes": ["update_success", "update_failed"]
                },
                {
                    "id": "update_success",
                    "name": "更新成功状态",
                    "type": "ticket_update",
                    "config": {
                        "ticket_var": "ticket",
                        "updates": {
                            "comment": "服务已成功重启并恢复正常"
                        }
                    },
                    "next_nodes": ["end"]
                },
                {
                    "id": "update_failed",
                    "name": "更新失败状态",
                    "type": "ticket_update",
                    "config": {
                        "ticket_var": "ticket",
                        "updates": {
                            "comment": "服务重启失败，需要人工介入"
                        }
                    },
                    "next_nodes": ["end"]
                },
                {
                    "id": "end",
                    "name": "结束",
                    "type": "end",
                    "config": {},
                    "next_nodes": []
                }
            ],
            "connections": [
                {"from": "start", "to": "check_service"},
                {"from": "check_service", "to": "condition_service_health"},
                {"from": "condition_service_health", "to": "service_healthy", "condition": "true"},
                {"from": "condition_service_health", "to": "restart_service", "condition": "false"},
                {"from": "service_healthy", "to": "end"},
                {"from": "restart_service", "to": "validate_restart"},
                {"from": "validate_restart", "to": "update_success", "condition": "true"},
                {"from": "validate_restart", "to": "update_failed", "condition": "false"},
                {"from": "update_success", "to": "end"},
                {"from": "update_failed", "to": "end"}
            ],
            "variables": {
                "service_name": "nginx"
            }
        }
    }')

WORKFLOW_ID=$(echo $CREATE_WORKFLOW_RESPONSE | jq -r '.data.id')
if [ -z "$WORKFLOW_ID" ] || [ "$WORKFLOW_ID" = "null" ]; then
    print_error "创建工作流失败"
    echo $CREATE_WORKFLOW_RESPONSE | jq
    exit 1
fi
print_info "工作流创建成功，ID: $WORKFLOW_ID"

# 显示条件节点格式说明
print_info "条件节点格式说明："
echo -e "${YELLOW}条件节点结构：${NC}"
echo '  {
    "id": "condition_1",
    "type": "condition",
    "config": {
      "expression": "评估表达式"
    },
    "next_nodes": ["true分支节点", "false分支节点"]
  }'
echo ""
echo "说明："
echo "- next_nodes[0] = 条件为 true 时执行的节点"
echo "- next_nodes[1] = 条件为 false 时执行的节点"
echo "- 必须有且仅有 2 个 next_nodes"

print_info "测试完成！"

# 清理：删除测试工作流
print_info "清理测试数据..."
DELETE_RESPONSE=$(curl -s -X DELETE "${API_URL}/healing/workflows/${WORKFLOW_ID}" \
    -H "Authorization: Bearer ${TOKEN}")

if [ $(echo $DELETE_RESPONSE | jq -r '.code') -eq 200 ]; then
    print_info "测试工作流已删除"
else
    print_error "删除工作流失败"
fi