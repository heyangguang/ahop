#!/bin/bash

# 测试任务模板使用检查功能

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

# 创建测试任务模板
create_template() {
    print_info "创建测试任务模板..."
    
    TEMPLATE_RESPONSE=$(curl -s -X POST "${API_URL}/task-templates" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "测试模板-检查删除",
            "code": "test_template_delete",
            "description": "用于测试模板删除检查",
            "type": "shell",
            "content": "echo \"Test template\"",
            "host_pattern": "all",
            "variables": {}
        }')
    
    TEMPLATE_ID=$(echo $TEMPLATE_RESPONSE | jq -r '.data.id')
    
    if [ "$TEMPLATE_ID" == "null" ] || [ -z "$TEMPLATE_ID" ]; then
        print_error "创建模板失败"
        echo $TEMPLATE_RESPONSE | jq
        exit 1
    fi
    
    print_info "模板ID: $TEMPLATE_ID"
}

# 使用模板创建任务
create_task_with_template() {
    print_info "使用模板创建任务..."
    
    TASK_RESPONSE=$(curl -s -X POST "${API_URL}/tasks" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d "{
            \"name\": \"测试任务-使用模板\",
            \"task_type\": \"template\",
            \"params\": {
                \"template_id\": $TEMPLATE_ID,
                \"variables\": {}
            }
        }")
    
    TASK_ID=$(echo $TASK_RESPONSE | jq -r '.data.task_id')
    
    if [ "$TASK_ID" == "null" ] || [ -z "$TASK_ID" ]; then
        print_error "创建任务失败"
        echo $TASK_RESPONSE | jq
        exit 1
    fi
    
    print_info "任务ID: $TASK_ID"
}

# 尝试删除正在使用的模板
try_delete_template_in_use() {
    print_info "尝试删除正在使用的模板..."
    
    DELETE_RESPONSE=$(curl -s -X DELETE "${API_URL}/task-templates/${TEMPLATE_ID}" \
        -H "Authorization: Bearer ${TOKEN}")
    
    DELETE_CODE=$(echo $DELETE_RESPONSE | jq -r '.code')
    DELETE_MSG=$(echo $DELETE_RESPONSE | jq -r '.message')
    
    if [ "$DELETE_CODE" != "200" ]; then
        print_info "预期的错误: $DELETE_MSG"
    else
        print_error "错误：不应该允许删除正在使用的模板！"
    fi
}

# 等待任务完成
wait_task_complete() {
    print_info "等待任务完成..."
    
    for i in {1..30}; do
        STATUS_RESPONSE=$(curl -s -X GET "${API_URL}/tasks/${TASK_ID}" \
            -H "Authorization: Bearer ${TOKEN}")
        
        STATUS=$(echo $STATUS_RESPONSE | jq -r '.data.status')
        
        if [ "$STATUS" == "completed" ] || [ "$STATUS" == "failed" ]; then
            print_info "任务状态: $STATUS"
            break
        fi
        
        echo -n "."
        sleep 2
    done
    echo ""
}

# 再次尝试删除模板
try_delete_template_after_complete() {
    print_info "任务完成后尝试删除模板..."
    
    DELETE_RESPONSE=$(curl -s -X DELETE "${API_URL}/task-templates/${TEMPLATE_ID}" \
        -H "Authorization: Bearer ${TOKEN}")
    
    DELETE_CODE=$(echo $DELETE_RESPONSE | jq -r '.code')
    DELETE_MSG=$(echo $DELETE_RESPONSE | jq -r '.message')
    
    if [ "$DELETE_CODE" == "200" ]; then
        print_info "成功删除模板（有历史任务记录但不在运行）"
    else
        print_error "删除失败: $DELETE_MSG"
    fi
}

# 主流程
main() {
    print_info "开始测试任务模板使用检查功能"
    echo "================================"
    
    login
    create_template
    create_task_with_template
    
    print_info "测试场景1：任务运行中，尝试删除模板"
    try_delete_template_in_use
    
    print_info "测试场景2：等待任务完成后删除模板"
    wait_task_complete
    try_delete_template_after_complete
    
    echo "================================"
    print_info "测试完成！"
    print_info "功能说明："
    print_info "1. 如果有正在运行的任务使用模板，禁止删除"
    print_info "2. 如果只有历史任务记录，允许删除但会记录警告日志"
}

# 运行主流程
main