#!/bin/bash

# 测试改进后的工单更新节点

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

# 删除已存在的测试工作流
print_info "清理已存在的测试工作流..."
WORKFLOWS_RESPONSE=$(curl -s -X GET "${API_URL}/healing/workflows" \
    -H "Authorization: Bearer ${TOKEN}")

# 删除所有匹配的工作流
echo $WORKFLOWS_RESPONSE | jq -r '.data[]? | select(.code=="ticket_update_test") | .id' | while read EXISTING_ID; do
    if [ ! -z "$EXISTING_ID" ] && [ "$EXISTING_ID" != "null" ]; then
        DELETE_RESPONSE=$(curl -s -X DELETE "${API_URL}/healing/workflows/${EXISTING_ID}" \
            -H "Authorization: Bearer ${TOKEN}")
        print_info "已删除旧的测试工作流 (ID: $EXISTING_ID)"
    fi
done

# 创建测试工作流
print_info "创建工单更新测试工作流..."
CREATE_WORKFLOW_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "工单更新节点测试工作流",
        "code": "ticket_update_test_$(date +%s)",
        "description": "测试改进后的工单更新节点",
        "definition": {
            "nodes": [
                {
                    "id": "start",
                    "name": "开始",
                    "type": "start",
                    "config": {},
                    "next_nodes": ["extract_data"]
                },
                {
                    "id": "extract_data",
                    "name": "提取数据",
                    "type": "data_process",
                    "config": {
                        "extract": {
                            "ticket": "global_context",
                            "ticket_id": "global_context.id",
                            "affected_hosts": "global_context.custom_fields.affected_hosts",
                            "service_name": "global_context.custom_fields.service"
                        }
                    },
                    "next_nodes": ["simulate_task"]
                },
                {
                    "id": "simulate_task",
                    "name": "模拟任务执行",
                    "type": "data_process",
                    "config": {
                        "transform": {
                            "task_result": {
                                "function": "default",
                                "args": [{"status": "success"}]
                            },
                            "task_message": {
                                "function": "default",
                                "args": ["服务重启成功"]
                            }
                        }
                    },
                    "next_nodes": ["check_result"]
                },
                {
                    "id": "check_result",
                    "name": "检查执行结果",
                    "type": "condition",
                    "config": {
                        "expression": "task_result.status == \"success\""
                    },
                    "next_nodes": ["update_success", "update_failed"]
                },
                {
                    "id": "update_success",
                    "name": "更新工单-成功",
                    "type": "ticket_update",
                    "config": {
                        "ticket_var": "ticket",
                        "updates": {
                            "status": "resolved",
                            "comment": "{{execution_summary}}",
                            "custom_fields": {
                                "resolution": "自动修复成功",
                                "execution_id": "{{execution.id}}",
                                "auto_fixed": true
                            }
                        }
                    },
                    "next_nodes": ["end"]
                },
                {
                    "id": "update_failed",
                    "name": "更新工单-失败",
                    "type": "ticket_update",
                    "config": {
                        "ticket_var": "ticket",
                        "updates": {
                            "comment": "{{execution_summary}}",
                            "custom_fields": {
                                "auto_fix_attempted": true,
                                "execution_id": "{{execution.id}}"
                            }
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
                {"from": "start", "to": "extract_data"},
                {"from": "extract_data", "to": "simulate_task"},
                {"from": "simulate_task", "to": "check_result"},
                {"from": "check_result", "to": "update_success", "condition": "true"},
                {"from": "check_result", "to": "update_failed", "condition": "false"},
                {"from": "update_success", "to": "end"},
                {"from": "update_failed", "to": "end"}
            ],
            "variables": {}
        }
    }')

WORKFLOW_ID=$(echo $CREATE_WORKFLOW_RESPONSE | jq -r '.data.id')
if [ -z "$WORKFLOW_ID" ] || [ "$WORKFLOW_ID" = "null" ]; then
    print_error "创建工作流失败"
    echo $CREATE_WORKFLOW_RESPONSE | jq
    exit 1
fi
print_info "工作流创建成功，ID: $WORKFLOW_ID"

# 启用工作流
print_info "启用工作流..."
ENABLE_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows/${WORKFLOW_ID}/enable" \
    -H "Authorization: Bearer ${TOKEN}")

if [ $(echo $ENABLE_RESPONSE | jq -r '.code') -eq 200 ]; then
    print_info "工作流已启用"
else
    print_error "启用工作流失败"
    echo $ENABLE_RESPONSE | jq
fi

# 显示工单更新节点改进说明
print_info "工单更新节点改进说明："
echo -e "\n${YELLOW}新特性：${NC}"
echo "1. 执行摘要自动生成并包含在工单评论中"
echo "2. 支持完整的工单更新（状态、评论、自定义字段）"
echo "3. 所有更新字段都支持变量引用"
echo "4. 更新失败不阻塞工作流"

echo -e "\n${YELLOW}可用变量：${NC}"
echo "- {{execution_summary}} - 完整的执行日志摘要"
echo "- {{execution.id}} - 执行ID"
echo "- {{execution.status}} - 执行状态"
echo "- {{execution.duration}} - 执行耗时"
echo "- {{workflow.name}} - 工作流名称"

echo -e "\n${YELLOW}执行摘要格式：${NC}"
echo "✅ 自动修复成功"
echo ""
echo "【执行信息】"
echo "- 工作流：xxx"
echo "- 执行ID：xxx"
echo "- 执行时间：xxx"
echo ""
echo "【执行步骤】"
echo "[时间] ✓ 步骤详情..."
echo ""
echo "【执行结果】"
echo "所有任务执行成功..."

# 手动执行工作流测试
print_info "执行工作流..."
EXECUTE_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows/${WORKFLOW_ID}/execute" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
        "trigger_type": "manual",
        "trigger_source": {
            "global_context": {
                "id": 123,
                "external_id": "TICK-123",
                "title": "服务异常",
                "status": "open",
                "custom_fields": {
                    "affected_hosts": ["192.168.1.10"],
                    "service": "nginx"
                }
            }
        }
    }')

EXECUTION_ID=$(echo $EXECUTE_RESPONSE | jq -r '.data.execution_id')
if [ -z "$EXECUTION_ID" ] || [ "$EXECUTION_ID" = "null" ]; then
    print_error "执行工作流失败"
    echo $EXECUTE_RESPONSE | jq
else
    print_info "工作流执行成功，执行ID: $EXECUTION_ID"
    
    # 等待执行完成
    sleep 3
    
    # 获取执行日志
    print_info "获取执行日志..."
    LOGS_RESPONSE=$(curl -s -X GET "${API_URL}/healing/executions/${EXECUTION_ID}/logs" \
        -H "Authorization: Bearer ${TOKEN}")
    
    echo -e "\n${YELLOW}执行日志：${NC}"
    echo $LOGS_RESPONSE | jq '.data'
fi

# 清理：删除测试工作流
print_info "清理测试数据..."
DELETE_RESPONSE=$(curl -s -X DELETE "${API_URL}/healing/workflows/${WORKFLOW_ID}" \
    -H "Authorization: Bearer ${TOKEN}")

if [ $(echo $DELETE_RESPONSE | jq -r '.code') -eq 200 ]; then
    print_info "测试工作流已删除"
else
    print_error "删除工作流失败"
fi

print_info "测试完成！"