#!/bin/bash

# 测试自愈模块功能

set -f  # 禁用通配符展开，避免cron表达式中的*被展开

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# API地址
API_URL="http://localhost:8080/api/v1"

# 打印带颜色的信息
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# 检查依赖
check_dependencies() {
    if ! command -v jq &> /dev/null; then
        print_error "jq 未安装。请先安装 jq"
        exit 1
    fi
}

# 登录获取token
login() {
    print_info "登录系统..."
    
    LOGIN_RESPONSE=$(curl -s -X POST "${API_URL}/auth/login" \
        -H "Content-Type: application/json" \
        -d '{
            "username": "admin",
            "password": "Admin@123"
        }')
    
    TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.token')
    
    if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
        print_error "登录失败"
        echo $LOGIN_RESPONSE | jq
        exit 1
    fi
    
    print_info "登录成功"
}

# 测试工作流CRUD
test_workflow_crud() {
    print_info "测试工作流 CRUD..."
    
    # 创建工作流
    print_info "创建测试工作流..."
    CREATE_WORKFLOW_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "服务重启工作流",
            "code": "service_restart_workflow",
            "description": "自动重启异常服务的工作流",
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
                            "template_id": 1,
                            "timeout": 60
                        },
                        "next_nodes": ["condition_check"]
                    },
                    {
                        "id": "condition_check",
                        "name": "判断服务状态",
                        "type": "condition",
                        "config": {
                            "expression": "task_result.service_status != \"running\""
                        },
                        "next_nodes": ["restart_service", "end"]
                    },
                    {
                        "id": "restart_service",
                        "name": "重启服务",
                        "type": "task_execute",
                        "config": {
                            "template_id": 2,
                            "timeout": 120
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
                    {"from": "check_service", "to": "condition_check"},
                    {"from": "condition_check", "to": "restart_service", "condition": "true"},
                    {"from": "condition_check", "to": "end", "condition": "false"},
                    {"from": "restart_service", "to": "end"}
                ],
                "variables": {
                    "service_name": "nginx",
                    "max_retries": 3
                }
            },
            "timeout_minutes": 30,
            "max_retries": 2,
            "allow_parallel": false
        }')
    
    WORKFLOW_ID=$(echo $CREATE_WORKFLOW_RESPONSE | jq -r '.data.id')
    if [ "$WORKFLOW_ID" == "null" ] || [ -z "$WORKFLOW_ID" ]; then
        print_error "创建工作流失败"
        echo $CREATE_WORKFLOW_RESPONSE | jq
        exit 1
    fi
    print_info "工作流创建成功，ID: $WORKFLOW_ID"
    
    # 获取工作流列表
    print_info "获取工作流列表..."
    LIST_WORKFLOW_RESPONSE=$(curl -s -X GET "${API_URL}/healing/workflows" \
        -H "Authorization: Bearer ${TOKEN}")
    
    echo $LIST_WORKFLOW_RESPONSE | jq '.data'
    
    # 获取单个工作流
    print_info "获取工作流详情..."
    GET_WORKFLOW_RESPONSE=$(curl -s -X GET "${API_URL}/healing/workflows/${WORKFLOW_ID}" \
        -H "Authorization: Bearer ${TOKEN}")
    
    echo $GET_WORKFLOW_RESPONSE | jq '.data'
    
    # 更新工作流
    print_info "更新工作流..."
    UPDATE_WORKFLOW_RESPONSE=$(curl -s -X PUT "${API_URL}/healing/workflows/${WORKFLOW_ID}" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "description": "自动重启异常服务的工作流（已更新）",
            "timeout_minutes": 60
        }')
    
    echo $UPDATE_WORKFLOW_RESPONSE | jq '.data.description'
    
    # 启用/禁用工作流
    print_info "禁用工作流..."
    DISABLE_WORKFLOW_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows/${WORKFLOW_ID}/disable" \
        -H "Authorization: Bearer ${TOKEN}")
    
    echo $DISABLE_WORKFLOW_RESPONSE | jq
    
    print_info "启用工作流..."
    ENABLE_WORKFLOW_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows/${WORKFLOW_ID}/enable" \
        -H "Authorization: Bearer ${TOKEN}")
    
    echo $ENABLE_WORKFLOW_RESPONSE | jq
    
    # 克隆工作流
    print_info "克隆工作流..."
    CLONE_WORKFLOW_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows/${WORKFLOW_ID}/clone" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "code": "service_restart_workflow_v2",
            "name": "服务重启工作流 v2"
        }')
    
    CLONED_WORKFLOW_ID=$(echo $CLONE_WORKFLOW_RESPONSE | jq -r '.data.id')
    print_info "工作流克隆成功，新ID: $CLONED_WORKFLOW_ID"
}

# 测试规则CRUD
test_rule_crud() {
    print_info "测试自愈规则 CRUD..."
    
    # 创建规则
    print_info "创建测试规则..."
    CREATE_RULE_RESPONSE=$(curl -s -X POST "${API_URL}/healing/rules" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d @- <<EOF
{
    "name": "服务异常自动重启规则",
    "code": "service_down_auto_restart",
    "description": "检测到服务异常时自动执行重启工作流",
    "trigger_type": "scheduled",
    "cron_expr": "*/5 * * * *",
    "match_rules": {
        "source": "ticket",
        "field": "title",
        "operator": "contains",
        "value": "服务异常",
        "logic_op": "and",
        "conditions": [
            {
                "source": "ticket",
                "field": "status",
                "operator": "equals",
                "value": "open"
            }
        ]
    },
    "priority": 10,
    "workflow_id": $WORKFLOW_ID,
    "max_executions": 100,
    "cooldown_minutes": 10
}
EOF
    )
    
    RULE_ID=$(echo $CREATE_RULE_RESPONSE | jq -r '.data.id')
    if [ "$RULE_ID" == "null" ] || [ -z "$RULE_ID" ]; then
        print_error "创建规则失败"
        echo $CREATE_RULE_RESPONSE | jq
        return 1
    fi
    print_info "规则创建成功，ID: $RULE_ID"
    
    # 获取规则列表
    print_info "获取规则列表..."
    LIST_RULE_RESPONSE=$(curl -s -X GET "${API_URL}/healing/rules" \
        -H "Authorization: Bearer ${TOKEN}")
    
    echo $LIST_RULE_RESPONSE | jq '.data'
    
    # 获取单个规则
    print_info "获取规则详情..."
    GET_RULE_RESPONSE=$(curl -s -X GET "${API_URL}/healing/rules/${RULE_ID}" \
        -H "Authorization: Bearer ${TOKEN}")
    
    echo $GET_RULE_RESPONSE | jq '.data'
    
    # 更新规则
    print_info "更新规则..."
    UPDATE_RULE_RESPONSE=$(curl -s -X PUT "${API_URL}/healing/rules/${RULE_ID}" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "description": "检测到服务异常时自动执行重启工作流（已更新）",
            "cooldown_minutes": 15
        }')
    
    echo $UPDATE_RULE_RESPONSE | jq '.data.description'
    
    # 启用/禁用规则
    print_info "禁用规则..."
    DISABLE_RULE_RESPONSE=$(curl -s -X POST "${API_URL}/healing/rules/${RULE_ID}/disable" \
        -H "Authorization: Bearer ${TOKEN}")
    
    echo $DISABLE_RULE_RESPONSE | jq
    
    print_info "启用规则..."
    ENABLE_RULE_RESPONSE=$(curl -s -X POST "${API_URL}/healing/rules/${RULE_ID}/enable" \
        -H "Authorization: Bearer ${TOKEN}")
    
    echo $ENABLE_RULE_RESPONSE | jq
}

# 清理测试数据
cleanup() {
    print_info "清理测试数据..."
    
    if [ ! -z "$RULE_ID" ]; then
        curl -s -X DELETE "${API_URL}/healing/rules/${RULE_ID}" \
            -H "Authorization: Bearer ${TOKEN}" > /dev/null
        print_info "删除规则 $RULE_ID"
    fi
    
    if [ ! -z "$CLONED_WORKFLOW_ID" ]; then
        curl -s -X DELETE "${API_URL}/healing/workflows/${CLONED_WORKFLOW_ID}" \
            -H "Authorization: Bearer ${TOKEN}" > /dev/null
        print_info "删除克隆的工作流 $CLONED_WORKFLOW_ID"
    fi
    
    if [ ! -z "$WORKFLOW_ID" ]; then
        curl -s -X DELETE "${API_URL}/healing/workflows/${WORKFLOW_ID}" \
            -H "Authorization: Bearer ${TOKEN}" > /dev/null
        print_info "删除工作流 $WORKFLOW_ID"
    fi
}

# 主流程
main() {
    check_dependencies
    
    print_info "开始测试自愈模块"
    echo "================================"
    
    login
    test_workflow_crud
    test_rule_crud
    cleanup
    
    echo "================================"
    print_info "测试完成"
}

# 捕获退出信号，确保清理
trap cleanup EXIT

main