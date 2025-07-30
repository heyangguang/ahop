#!/bin/bash

# 测试任务改进功能
# 1. 测试 hosts 参数类型处理
# 2. 测试参数验证
# 3. 测试状态更新

API_URL="http://localhost:8080/api/v1"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印函数
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# 登录获取 token
print_info "登录系统..."
LOGIN_RESPONSE=$(curl -s -X POST "${API_URL}/auth/login" \
    -H 'Content-Type: application/json' \
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

# 测试1: 创建没有 hosts 的任务（应该失败）
print_info "\n测试1: 创建没有 hosts 的任务..."
NO_HOSTS_RESPONSE=$(curl -s -X POST "${API_URL}/tasks" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "task_type": "template",
        "name": "测试无主机任务",
        "description": "测试参数验证",
        "priority": 5,
        "params": {
            "template_id": 1,
            "variables": {}
        }
    }')

if [ $(echo $NO_HOSTS_RESPONSE | jq -r '.code') -eq 400 ]; then
    print_info "✓ 正确拒绝了没有 hosts 的任务"
    echo "错误信息: $(echo $NO_HOSTS_RESPONSE | jq -r '.message')"
else
    print_error "✗ 应该拒绝没有 hosts 的任务"
    echo $NO_HOSTS_RESPONSE | jq
fi

# 测试2: 使用数字数组作为 hosts（测试类型转换）
print_info "\n测试2: 使用数字数组作为 hosts..."
NUMBER_HOSTS_RESPONSE=$(curl -s -X POST "${API_URL}/tasks" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "task_type": "template",
        "name": "测试数字数组主机",
        "description": "测试 hosts 类型转换",
        "priority": 5,
        "params": {
            "template_id": 1,
            "hosts": [1, 2],
            "variables": {}
        }
    }')

if [ $(echo $NUMBER_HOSTS_RESPONSE | jq -r '.code') -eq 200 ]; then
    TASK_ID=$(echo $NUMBER_HOSTS_RESPONSE | jq -r '.data.task_id')
    print_info "✓ 成功创建任务，ID: $TASK_ID"
    
    # 检查任务参数
    TASK_DETAIL=$(curl -s -X GET "${API_URL}/tasks/${TASK_ID}" \
        -H "Authorization: Bearer $TOKEN")
    
    echo "任务参数:"
    echo $TASK_DETAIL | jq '.data.params'
    
    # 检查 hosts 是否存在
    if [ $(echo $TASK_DETAIL | jq '.data.params.hosts | length') -gt 0 ]; then
        print_info "✓ hosts 参数正确保存"
    else
        print_error "✗ hosts 参数未保存"
    fi
else
    print_error "✗ 创建任务失败"
    echo $NUMBER_HOSTS_RESPONSE | jq
fi

# 测试3: 手动触发定时任务（测试主机参数转换）
print_info "\n测试3: 手动触发定时任务..."
# 获取一个定时任务
SCHEDULED_TASKS=$(curl -s -X GET "${API_URL}/scheduled-tasks?page_size=1" \
    -H "Authorization: Bearer $TOKEN")

if [ $(echo $SCHEDULED_TASKS | jq '.data | length') -gt 0 ]; then
    SCHEDULED_ID=$(echo $SCHEDULED_TASKS | jq -r '.data[0].id')
    print_info "触发定时任务 ID: $SCHEDULED_ID"
    
    RUN_RESPONSE=$(curl -s -X POST "${API_URL}/scheduled-tasks/${SCHEDULED_ID}/run" \
        -H "Authorization: Bearer $TOKEN")
    
    if [ $(echo $RUN_RESPONSE | jq -r '.code') -eq 200 ]; then
        TASK_ID=$(echo $RUN_RESPONSE | jq -r '.data.task_id')
        print_info "✓ 定时任务触发成功，任务ID: $TASK_ID"
        
        # 等待一下让任务处理
        sleep 2
        
        # 检查任务状态
        TASK_STATUS=$(curl -s -X GET "${API_URL}/tasks/${TASK_ID}" \
            -H "Authorization: Bearer $TOKEN" | jq -r '.data.status')
        
        print_info "任务状态: $TASK_STATUS"
        
        # 检查定时任务状态
        ST_STATUS=$(curl -s -X GET "${API_URL}/scheduled-tasks/${SCHEDULED_ID}" \
            -H "Authorization: Bearer $TOKEN" | jq -r '.data.last_status')
        
        print_info "定时任务状态: $ST_STATUS"
    else
        print_error "触发失败"
        echo $RUN_RESPONSE | jq
    fi
else
    print_warning "没有定时任务可测试"
fi

# 测试4: 创建空 hosts 数组的任务（应该失败）
print_info "\n测试4: 创建空 hosts 数组的任务..."
EMPTY_HOSTS_RESPONSE=$(curl -s -X POST "${API_URL}/tasks" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "task_type": "template",
        "name": "测试空主机数组",
        "description": "测试空数组验证",
        "priority": 5,
        "params": {
            "template_id": 1,
            "hosts": [],
            "variables": {}
        }
    }')

if [ $(echo $EMPTY_HOSTS_RESPONSE | jq -r '.code') -eq 400 ]; then
    print_info "✓ 正确拒绝了空 hosts 数组"
    echo "错误信息: $(echo $EMPTY_HOSTS_RESPONSE | jq -r '.message')"
else
    print_error "✗ 应该拒绝空 hosts 数组"
    echo $EMPTY_HOSTS_RESPONSE | jq
fi

print_info "\n测试完成！"