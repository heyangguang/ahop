#!/bin/bash

# 测试自愈定时触发功能

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

# 创建测试工作流
create_workflow() {
    print_info "创建测试工作流..."
    
    CREATE_WORKFLOW_RESPONSE=$(curl -s -X POST "${API_URL}/healing/workflows" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "定时测试工作流",
            "code": "scheduled_test_workflow",
            "description": "用于测试定时触发的工作流",
            "definition": {
                "nodes": [
                    {
                        "id": "start",
                        "name": "开始",
                        "type": "start",
                        "config": {},
                        "next_nodes": ["fetch_tickets"]
                    },
                    {
                        "id": "fetch_tickets",
                        "name": "获取工单",
                        "type": "data_fetch",
                        "config": {
                            "source": "ticket",
                            "output": "tickets",
                            "filters": {
                                "status": "open",
                                "title_contains": "测试"
                            }
                        },
                        "next_nodes": ["condition"]
                    },
                    {
                        "id": "condition",
                        "name": "判断是否有工单",
                        "type": "condition",
                        "config": {
                            "expression": "len(tickets) > 0"
                        },
                        "next_nodes": ["process", "end"]
                    },
                    {
                        "id": "process",
                        "name": "处理工单",
                        "type": "data_process",
                        "config": {
                            "extract": {
                                "first_ticket_id": "tickets[0].id"
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
                    {"from": "start", "to": "fetch_tickets"},
                    {"from": "fetch_tickets", "to": "condition"},
                    {"from": "condition", "to": "process", "condition": "true"},
                    {"from": "condition", "to": "end", "condition": "false"},
                    {"from": "process", "to": "end"}
                ],
                "variables": {}
            },
            "timeout_minutes": 10,
            "max_retries": 0,
            "allow_parallel": false
        }')
    
    WORKFLOW_ID=$(echo $CREATE_WORKFLOW_RESPONSE | jq -r '.data.id')
    if [ "$WORKFLOW_ID" == "null" ] || [ -z "$WORKFLOW_ID" ]; then
        print_error "创建工作流失败"
        echo $CREATE_WORKFLOW_RESPONSE | jq
        exit 1
    fi
    print_info "工作流创建成功，ID: $WORKFLOW_ID"
}

# 创建定时规则
create_scheduled_rule() {
    print_info "创建定时规则（每分钟执行）..."
    
    # 使用当前时间的下一分钟
    CURRENT_MINUTE=$(date +%-M)
    NEXT_MINUTE=$(( (CURRENT_MINUTE + 1) % 60 ))
    CRON_EXPR="$NEXT_MINUTE * * * *"
    
    print_info "Cron表达式: $CRON_EXPR"
    
    CREATE_RULE_RESPONSE=$(curl -s -X POST "${API_URL}/healing/rules" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d @- <<EOF
{
    "name": "定时测试规则",
    "code": "scheduled_test_rule",
    "description": "每分钟执行的测试规则",
    "trigger_type": "scheduled",
    "cron_expr": "$CRON_EXPR",
    "match_rules": {
        "source": "ticket",
        "field": "status",
        "operator": "equals",
        "value": "open"
    },
    "priority": 1,
    "workflow_id": $WORKFLOW_ID,
    "max_executions": 10,
    "cooldown_minutes": 0
}
EOF
    )
    
    RULE_ID=$(echo $CREATE_RULE_RESPONSE | jq -r '.data.id')
    if [ "$RULE_ID" == "null" ] || [ -z "$RULE_ID" ]; then
        print_error "创建规则失败"
        echo $CREATE_RULE_RESPONSE | jq
        exit 1
    fi
    print_info "规则创建成功，ID: $RULE_ID"
}

# 创建测试工单
create_test_ticket() {
    print_info "创建测试工单..."
    
    # 先获取一个插件
    PLUGINS_RESPONSE=$(curl -s -X GET "${API_URL}/ticket-plugins" \
        -H "Authorization: Bearer ${TOKEN}")
    
    PLUGIN_ID=$(echo $PLUGINS_RESPONSE | jq -r '.data.data[0].id')
    if [ "$PLUGIN_ID" == "null" ] || [ -z "$PLUGIN_ID" ]; then
        print_warn "没有找到工单插件，跳过创建工单"
        return
    fi
    
    # 创建工单
    CREATE_TICKET_RESPONSE=$(curl -s -X POST "${API_URL}/tickets" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "plugin_id": '$PLUGIN_ID',
            "external_id": "TEST-'$(date +%s)'",
            "title": "测试工单 - 定时触发",
            "status": "open",
            "priority": "high",
            "custom_data": {
                "description": "用于测试定时触发功能"
            }
        }')
    
    TICKET_ID=$(echo $CREATE_TICKET_RESPONSE | jq -r '.data.id')
    if [ "$TICKET_ID" == "null" ] || [ -z "$TICKET_ID" ]; then
        print_warn "创建工单失败"
        echo $CREATE_TICKET_RESPONSE | jq
    else
        print_info "工单创建成功，ID: $TICKET_ID"
    fi
}

# 手动执行规则
execute_rule_manually() {
    print_info "手动执行规则..."
    
    EXECUTE_RESPONSE=$(curl -s -X POST "${API_URL}/healing/rules/${RULE_ID}/execute" \
        -H "Authorization: Bearer ${TOKEN}")
    
    EXECUTION_ID=$(echo $EXECUTE_RESPONSE | jq -r '.data.execution_id')
    if [ "$EXECUTION_ID" == "null" ] || [ -z "$EXECUTION_ID" ]; then
        print_error "手动执行失败"
        echo $EXECUTE_RESPONSE | jq
        return 1
    fi
    
    print_info "手动执行成功，执行ID: $EXECUTION_ID"
    
    # 等待执行完成
    sleep 3
    
    # 获取执行日志（暂时注释，因为需要数据库ID而不是execution_id）
    # get_execution_logs "$EXECUTION_ID"
}

# 获取执行日志
get_execution_logs() {
    local execution_id=$1
    print_info "获取执行日志..."
    
    LOGS_RESPONSE=$(curl -s -X GET "${API_URL}/healing/executions/${execution_id}/logs" \
        -H "Authorization: Bearer ${TOKEN}")
    
    echo $LOGS_RESPONSE | jq '.data[]'
}

# 等待定时触发
wait_for_scheduled_execution() {
    print_info "等待定时触发（最多等待90秒）..."
    
    local start_time=$(date +%s)
    local timeout=90
    local found=false
    
    while [ $(($(date +%s) - start_time)) -lt $timeout ]; do
        # 获取规则详情
        RULE_RESPONSE=$(curl -s -X GET "${API_URL}/healing/rules/${RULE_ID}" \
            -H "Authorization: Bearer ${TOKEN}")
        
        EXECUTE_COUNT=$(echo $RULE_RESPONSE | jq -r '.data.execute_count')
        
        if [ "$EXECUTE_COUNT" -gt 0 ]; then
            print_info "定时触发成功！执行次数: $EXECUTE_COUNT"
            found=true
            break
        fi
        
        echo -n "."
        sleep 5
    done
    
    echo ""
    
    if [ "$found" = false ]; then
        print_warn "未检测到定时触发"
    fi
}

# 清理测试数据
cleanup() {
    print_info "清理测试数据..."
    
    if [ ! -z "$RULE_ID" ]; then
        curl -s -X DELETE "${API_URL}/healing/rules/${RULE_ID}" \
            -H "Authorization: Bearer ${TOKEN}" > /dev/null
        print_info "删除规则 $RULE_ID"
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
    
    print_info "开始测试自愈定时触发功能"
    echo "================================"
    
    login
    create_workflow
    create_scheduled_rule
    create_test_ticket
    
    # 测试手动执行
    print_info "测试手动执行功能"
    execute_rule_manually
    
    # 等待定时触发
    wait_for_scheduled_execution
    
    echo "================================"
    print_info "测试完成"
}

# 捕获退出信号，确保清理
trap cleanup EXIT

main