#!/bin/bash

# 测试工单回写功能

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

# 创建工单插件
create_plugin() {
    print_info "创建工单插件..."
    
    PLUGIN_RESPONSE=$(curl -s -X POST "${API_URL}/ticket-plugins" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "测试工单系统",
            "code": "test_ticket_system",
            "description": "用于测试工单回写功能",
            "base_url": "http://localhost:5002",
            "auth_type": "none",
            "sync_enabled": true,
            "sync_interval": 30,
            "sync_window": 60
        }')
    
    PLUGIN_ID=$(echo $PLUGIN_RESPONSE | jq -r '.data.id')
    
    if [ "$PLUGIN_ID" == "null" ] || [ -z "$PLUGIN_ID" ]; then
        print_warn "插件可能已存在，获取插件列表..."
        
        LIST_RESPONSE=$(curl -s -X GET "${API_URL}/ticket-plugins" \
            -H "Authorization: Bearer ${TOKEN}")
        
        PLUGIN_ID=$(echo $LIST_RESPONSE | jq -r '.data[] | select(.code == "test_ticket_system") | .id')
        
        if [ -z "$PLUGIN_ID" ]; then
            print_error "创建插件失败"
            echo $PLUGIN_RESPONSE | jq
            exit 1
        fi
    fi
    
    print_info "插件ID: $PLUGIN_ID"
}

# 手动同步工单
sync_tickets() {
    print_info "手动同步工单..."
    
    SYNC_RESPONSE=$(curl -s -X POST "${API_URL}/ticket-plugins/${PLUGIN_ID}/sync" \
        -H "Authorization: Bearer ${TOKEN}")
    
    echo $SYNC_RESPONSE | jq
    
    # 等待同步完成
    sleep 2
}

# 获取第一个工单
get_first_ticket() {
    print_info "获取工单列表..."
    
    TICKETS_RESPONSE=$(curl -s -X GET "${API_URL}/tickets?plugin_id=${PLUGIN_ID}" \
        -H "Authorization: Bearer ${TOKEN}")
    
    TICKET_ID=$(echo $TICKETS_RESPONSE | jq -r '.data[0].id')
    EXTERNAL_ID=$(echo $TICKETS_RESPONSE | jq -r '.data[0].external_id')
    
    if [ "$TICKET_ID" == "null" ] || [ -z "$TICKET_ID" ]; then
        print_error "没有找到工单"
        echo $TICKETS_RESPONSE | jq
        exit 1
    fi
    
    print_info "工单ID: $TICKET_ID, 外部ID: $EXTERNAL_ID"
}

# 测试工单回写
test_writeback() {
    print_info "测试工单回写..."
    
    # 测试1: 只更新状态
    print_info "测试1: 更新工单状态为 in_progress"
    WRITEBACK_RESPONSE=$(curl -s -X POST "${API_URL}/tickets/${TICKET_ID}/test-writeback" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "status": "in_progress"
        }')
    
    echo $WRITEBACK_RESPONSE | jq
    
    # 测试2: 只添加评论
    print_info "测试2: 添加评论"
    WRITEBACK_RESPONSE=$(curl -s -X POST "${API_URL}/tickets/${TICKET_ID}/test-writeback" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "comment": "正在执行自动修复任务..."
        }')
    
    echo $WRITEBACK_RESPONSE | jq
    
    # 测试3: 同时更新状态和评论
    print_info "测试3: 同时更新状态和评论"
    WRITEBACK_RESPONSE=$(curl -s -X POST "${API_URL}/tickets/${TICKET_ID}/test-writeback" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "status": "resolved",
            "comment": "问题已自动修复完成，请验证。"
        }')
    
    echo $WRITEBACK_RESPONSE | jq
    
    # 测试4: 更新自定义字段
    print_info "测试4: 更新自定义字段"
    WRITEBACK_RESPONSE=$(curl -s -X POST "${API_URL}/tickets/${TICKET_ID}/test-writeback" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "status": "closed",
            "comment": "任务执行成功，问题已解决。",
            "custom_fields": {
                "resolution": "已自动修复",
                "auto_fixed": true,
                "fixed_at": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'"
            }
        }')
    
    echo $WRITEBACK_RESPONSE | jq
}

# 验证更新结果
verify_update() {
    print_info "验证更新结果（直接调用模拟插件）..."
    
    # 直接调用模拟插件API查看工单详情
    PLUGIN_RESPONSE=$(curl -s -X GET "http://localhost:5002/tickets?id=${EXTERNAL_ID}")
    
    print_info "插件返回的工单信息："
    echo $PLUGIN_RESPONSE | jq '.data[0] | {id, status, updated_at, comments, custom_fields}'
}

# 主流程
main() {
    check_dependencies
    
    print_info "开始测试工单回写功能"
    echo "================================"
    
    # 确保模拟插件正在运行
    if ! curl -s -f http://localhost:5002/health > /dev/null; then
        print_error "模拟工单插件未运行，请先启动：python test/realistic_mock_ticket_plugin.py"
        exit 1
    fi
    
    login
    create_plugin
    sync_tickets
    get_first_ticket
    test_writeback
    verify_update
    
    echo "================================"
    print_info "测试完成！"
}

# 运行主流程
main