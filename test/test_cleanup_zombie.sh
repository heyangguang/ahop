#!/bin/bash

# 测试僵尸任务清理功能

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

# 查看当前的僵尸任务
print_info "\n查找僵尸任务..."
print_info "检查数据库中 queued 状态的任务..."

# 执行清理
print_info "\n执行清理..."
CLEANUP_RESPONSE=$(curl -s -X POST "${API_URL}/tasks/cleanup" \
    -H "Authorization: Bearer $TOKEN")

if [ $(echo $CLEANUP_RESPONSE | jq -r '.code') -eq 200 ]; then
    print_info "✓ 清理成功"
    echo $CLEANUP_RESPONSE | jq
else
    print_error "✗ 清理失败"
    echo $CLEANUP_RESPONSE | jq
fi

# 手动清理特定任务（如果需要）
if [ ! -z "$1" ]; then
    TASK_ID=$1
    print_info "\n手动标记任务 $TASK_ID 为失败..."
    
    # 使用 psql 直接更新
    PGPASSWORD=Admin@123 psql -h localhost -U postgres -d auto_healing_platform -c \
        "UPDATE tasks SET status = 'failed', error = '手动清理：任务在队列中丢失', finished_at = NOW() WHERE task_id = '$TASK_ID' AND status = 'queued';"
    
    print_info "已更新任务状态"
fi

print_info "\n测试完成！"