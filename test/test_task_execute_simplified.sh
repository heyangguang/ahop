#!/bin/bash

# 测试简化后的任务执行节点

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

# 创建测试工作流
print_info "创建任务执行测试工作流..."
CREATE_WORKFLOW_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "任务执行节点测试工作流",
        "code": "task_execute_test",
        "description": "测试简化后的任务执行节点",
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
                            "ticket_id": "global_context.id",
                            "affected_hosts": "global_context.custom_fields.affected_hosts",
                            "service_name": "global_context.custom_fields.service"
                        }
                    },
                    "next_nodes": ["restart_service"]
                },
                {
                    "id": "restart_service",
                    "name": "重启服务",
                    "type": "task_execute",
                    "config": {
                        "template_id": 1,
                        "hosts": "{{affected_hosts}}",
                        "host_match_by": "ip",
                        "task_params": {
                            "service_name": "{{service_name}}",
                            "restart_mode": "graceful",
                            "timeout": 30
                        },
                        "timeout": 300
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
                {"from": "extract_data", "to": "restart_service"},
                {"from": "restart_service", "to": "end"}
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

# 显示任务执行节点新格式说明
print_info "任务执行节点新格式说明："
echo -e "\n${YELLOW}配置格式：${NC}"
echo '{
  "type": "task_execute",
  "config": {
    "template_id": 123,                    // 任务模板ID（必需）
    "hosts": "{{affected_hosts}}",         // 主机标识列表（支持变量引用）
    "host_match_by": "ip",                 // ip | hostname（必需）
    "task_params": {                       // 任务参数
      "service_name": "{{service_name}}",  // 支持变量引用
      "restart_mode": "graceful"           // 支持硬编码
    },
    "timeout": 300                         // 执行超时（秒）
  }
}'

echo -e "\n${YELLOW}改进点：${NC}"
echo "1. 移除了 task_template_code（只用 template_id）"
echo "2. 移除了 host_var（统一用 hosts + 变量引用）"
echo "3. 移除了 variables（只用 task_params）"
echo "4. 新增 host_match_by 指定主机匹配方式"
echo "5. 统一的变量解析器支持 {{var|default:value}}"

# 手动执行工作流测试
print_info "执行工作流..."
EXECUTE_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows/${WORKFLOW_ID}/execute" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
        "trigger_type": "manual",
        "trigger_source": {
            "global_context": {
                "id": "TICK-123",
                "title": "服务异常",
                "custom_fields": {
                    "affected_hosts": ["192.168.1.10", "192.168.1.20"],
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