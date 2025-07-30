#!/bin/bash

# 测试数据处理节点功能

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

# 创建测试数据处理的工作流
print_info "创建数据处理测试工作流..."
CREATE_WORKFLOW_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "数据处理测试工作流",
        "code": "data_process_test",
        "description": "测试数据提取和转换功能",
        "definition": {
            "nodes": [
                {
                    "id": "start",
                    "name": "开始",
                    "type": "start",
                    "config": {},
                    "next_nodes": ["process_data"]
                },
                {
                    "id": "process_data",
                    "name": "处理数据",
                    "type": "data_process",
                    "config": {
                        "extract": {
                            "ticket_id": "ticket.id",
                            "ticket_title": "ticket.title",
                            "affected_hosts": "ticket.custom_fields.affected_hosts",
                            "first_host": "ticket.custom_fields.affected_hosts[0]",
                            "service_name": "ticket.custom_fields.service_name"
                        },
                        "transform": {
                            "host_count": "len(affected_hosts)",
                            "host_list_str": "join(affected_hosts, \",\")",
                            "first_host_display": "default(first_host, \"无主机\")",
                            "service_display": "default(service_name, \"未知服务\")",
                            "alert_message": {
                                "function": "format",
                                "args": ["服务 {} 在 {} 台主机上出现异常", "service_display", "host_count"]
                            },
                            "is_critical": "contains(ticket_title, \"紧急\")",
                            "unique_hosts": "unique(affected_hosts)"
                        }
                    },
                    "next_nodes": ["show_results"]
                },
                {
                    "id": "show_results",
                    "name": "显示结果",
                    "type": "ticket_update",
                    "config": {
                        "ticket_var": "ticket",
                        "updates": {
                            "comment": "{{alert_message}}"
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
                {"from": "start", "to": "process_data"},
                {"from": "process_data", "to": "show_results"},
                {"from": "show_results", "to": "end"}
            ],
            "variables": {
                "ticket": {
                    "id": 123,
                    "title": "紧急：服务异常",
                    "custom_fields": {
                        "affected_hosts": ["192.168.1.10", "192.168.1.20", "192.168.1.10"],
                        "service_name": "nginx"
                    }
                }
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

# 显示数据处理功能说明
print_info "数据处理功能说明："
echo -e "\n${YELLOW}数据提取（extract）支持的路径格式：${NC}"
echo "  - 简单字段：ticket.id"
echo "  - 嵌套字段：ticket.custom_fields.service_name"
echo "  - 数组索引：affected_hosts[0]"
echo "  - 数组所有元素：affected_hosts[*]"

echo -e "\n${YELLOW}数据转换（transform）支持的函数：${NC}"
echo "  - len(array) - 获取长度"
echo "  - join(array, separator) - 数组转字符串"
echo "  - first(array) / last(array) - 首尾元素"
echo "  - toString(value) - 转字符串"
echo "  - default(value, defaultValue) - 默认值"
echo "  - format(template, arg1, arg2...) - 格式化"
echo "  - contains(array/string, item) - 包含判断"
echo "  - unique(array) - 数组去重"

# 手动执行工作流测试
print_info "执行工作流..."
EXECUTE_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows/${WORKFLOW_ID}/execute" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
        "trigger_type": "manual"
    }')

EXECUTION_ID=$(echo $EXECUTE_RESPONSE | jq -r '.data.execution_id')
if [ -z "$EXECUTION_ID" ] || [ "$EXECUTION_ID" = "null" ]; then
    print_error "执行工作流失败"
    echo $EXECUTE_RESPONSE | jq
else
    print_info "工作流执行成功，执行ID: $EXECUTION_ID"
    
    # 等待一下让工作流执行完成
    sleep 2
    
    # 获取执行日志
    print_info "获取执行日志..."
    LOGS_RESPONSE=$(curl -s -X GET "${API_URL}/healing/executions/${EXECUTION_ID}/logs" \
        -H "Authorization: Bearer ${TOKEN}")
    
    echo "执行日志："
    echo $LOGS_RESPONSE | jq '.data[] | select(.node_id == "process_data") | .output'
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