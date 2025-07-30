#!/bin/bash

# 测试 Worker 改进功能
# 1. 测试状态更新时机
# 2. 测试业务错误不重试
# 3. 测试系统错误重试

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

# 测试1: 创建一个没有 hosts 的任务，验证不会重试
print_info "\n测试1: 创建没有 hosts 的任务（验证业务错误不重试）..."
NO_HOSTS_TASK=$(curl -s -X POST "${API_URL}/tasks" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "task_type": "template",
        "name": "测试业务错误处理",
        "description": "验证缺少 hosts 参数直接失败",
        "priority": 5,
        "params": {
            "template_id": 1,
            "variables": {}
        }
    }')

# 如果创建失败（服务端已经验证），直接跳过
if [ $(echo $NO_HOSTS_TASK | jq -r '.code') -ne 200 ]; then
    print_info "服务端已拒绝创建（预期行为）"
    echo "错误: $(echo $NO_HOSTS_TASK | jq -r '.message')"
else
    TASK_ID=$(echo $NO_HOSTS_TASK | jq -r '.data.task_id')
    print_info "任务创建成功，ID: $TASK_ID"
    
    # 等待 Worker 处理
    print_info "等待 Worker 处理..."
    sleep 3
    
    # 检查任务状态
    TASK_STATUS=$(curl -s -X GET "${API_URL}/tasks/${TASK_ID}" \
        -H "Authorization: Bearer $TOKEN")
    
    STATUS=$(echo $TASK_STATUS | jq -r '.data.status')
    ERROR=$(echo $TASK_STATUS | jq -r '.data.error')
    
    print_info "任务状态: $STATUS"
    if [ "$STATUS" = "failed" ]; then
        print_info "✓ 业务错误正确处理，任务直接失败"
        print_info "错误信息: $ERROR"
    else
        print_error "✗ 任务状态不正确，应该是 failed"
    fi
fi

# 测试2: 创建一个正常的任务，验证状态流转
print_info "\n测试2: 创建正常任务（验证状态流转）..."
NORMAL_TASK=$(curl -s -X POST "${API_URL}/tasks" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "task_type": "template",
        "name": "测试状态流转",
        "description": "验证状态从 queued → locked → running → success",
        "priority": 5,
        "params": {
            "template_id": 1,
            "hosts": [1, 2],
            "variables": {}
        }
    }')

if [ $(echo $NORMAL_TASK | jq -r '.code') -eq 200 ]; then
    TASK_ID=$(echo $NORMAL_TASK | jq -r '.data.task_id')
    print_info "任务创建成功，ID: $TASK_ID"
    
    # 立即检查状态（应该是 queued）
    STATUS1=$(curl -s -X GET "${API_URL}/tasks/${TASK_ID}" \
        -H "Authorization: Bearer $TOKEN" | jq -r '.data.status')
    print_info "初始状态: $STATUS1"
    
    # 等待一下再检查（应该变成 locked 或 running）
    sleep 1
    STATUS2=$(curl -s -X GET "${API_URL}/tasks/${TASK_ID}" \
        -H "Authorization: Bearer $TOKEN" | jq -r '.data.status')
    print_info "1秒后状态: $STATUS2"
    
    # 等待任务完成
    sleep 5
    FINAL_STATUS=$(curl -s -X GET "${API_URL}/tasks/${TASK_ID}" \
        -H "Authorization: Bearer $TOKEN")
    
    STATUS=$(echo $FINAL_STATUS | jq -r '.data.status')
    WORKER_ID=$(echo $FINAL_STATUS | jq -r '.data.worker_id')
    
    print_info "最终状态: $STATUS"
    print_info "Worker ID: $WORKER_ID"
    
    if [ "$WORKER_ID" != "null" ] && [ ! -z "$WORKER_ID" ]; then
        print_info "✓ Worker ID 正确记录"
    else
        print_error "✗ Worker ID 未记录"
    fi
else
    print_error "创建任务失败"
    echo $NORMAL_TASK | jq
fi

# 测试3: 查看 Worker 日志
print_info "\n测试3: 查看 Worker 日志..."
print_info "最近的 Worker 日志："
tail -n 20 /opt/ahop/worker-dist/logs/worker.log | grep -E "(开始处理任务|更新任务状态|业务错误|系统错误|重新入队)" || true

print_info "\n测试完成！"

# 提示
print_info "\n改进总结："
echo "1. Worker 获取任务后立即更新状态为 'locked'"
echo "2. 开始执行时更新状态为 'running'"
echo "3. 业务错误（如缺少参数）直接标记为 'failed'，不重试"
echo "4. 只有系统错误才重新入队"
echo "5. 避免产生新的僵尸任务"