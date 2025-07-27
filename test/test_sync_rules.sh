#!/bin/bash

# 测试同步规则（包括 regex 操作符）

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

# 创建工单插件
create_plugin() {
    print_info "创建工单插件..."
    
    PLUGIN_RESPONSE=$(curl -s -X POST "${API_URL}/ticket-plugins" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "正则测试插件",
            "code": "regex_test_plugin",
            "description": "用于测试regex操作符",
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
        
        PLUGIN_ID=$(echo $LIST_RESPONSE | jq -r '.data[] | select(.code == "regex_test_plugin") | .id')
        
        if [ -z "$PLUGIN_ID" ]; then
            print_error "创建插件失败"
            echo $PLUGIN_RESPONSE | jq
            exit 1
        fi
    fi
    
    print_info "插件ID: $PLUGIN_ID"
}

# 测试 regex 操作符
test_regex_rules() {
    print_info "配置正则表达式同步规则..."
    
    # 配置规则：
    # 1. 只包含生产环境的工单（标题包含 PROD-）
    # 2. 排除测试工单（描述包含 test 或 demo）
    # 3. 只包含紧急工单（标题包含 [紧急] 或 [URGENT]）
    
    RULES_RESPONSE=$(curl -s -X POST "${API_URL}/ticket-plugins/${PLUGIN_ID}/sync-rules" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{
            "rules": [
                {
                    "name": "只包含生产环境工单",
                    "field": "external_id",
                    "operator": "regex",
                    "value": "^PROD-.*",
                    "action": "include",
                    "enabled": true
                },
                {
                    "name": "排除测试工单",
                    "field": "description",
                    "operator": "regex",
                    "value": "(?i)(test|demo|sample)",
                    "action": "exclude",
                    "enabled": true
                },
                {
                    "name": "包含紧急标记",
                    "field": "title",
                    "operator": "regex",
                    "value": "\\[(紧急|URGENT|Critical)\\]",
                    "action": "include",
                    "enabled": true
                },
                {
                    "name": "排除已解决的旧工单",
                    "field": "external_id",
                    "operator": "regex",
                    "value": "^TICKET-00[0-4][0-9]",
                    "action": "exclude",
                    "enabled": true
                }
            ]
        }')
    
    echo "配置的同步规则："
    echo $RULES_RESPONSE | jq '.data'
}

# 测试同步
test_sync() {
    print_info "执行测试同步..."
    
    SYNC_RESPONSE=$(curl -s -X POST "${API_URL}/ticket-plugins/${PLUGIN_ID}/test-sync" \
        -H "Authorization: Bearer ${TOKEN}")
    
    print_info "同步结果："
    echo $SYNC_RESPONSE | jq '.data'
    
    # 显示被过滤的情况
    TOTAL_FETCHED=$(echo $SYNC_RESPONSE | jq -r '.data.total_fetched')
    TOTAL_FILTERED=$(echo $SYNC_RESPONSE | jq -r '.data.total_filtered_out')
    TOTAL_PROCESSED=$(echo $SYNC_RESPONSE | jq -r '.data.total_processed')
    
    print_info "统计信息："
    echo "- 获取的工单总数: $TOTAL_FETCHED"
    echo "- 被过滤掉的工单: $TOTAL_FILTERED"
    echo "- 实际处理的工单: $TOTAL_PROCESSED"
}

# 获取实际同步的工单
get_synced_tickets() {
    print_info "获取同步后的工单列表..."
    
    TICKETS_RESPONSE=$(curl -s -X GET "${API_URL}/tickets?plugin_id=${PLUGIN_ID}" \
        -H "Authorization: Bearer ${TOKEN}")
    
    print_info "同步的工单："
    echo $TICKETS_RESPONSE | jq '.data[] | {id: .id, external_id: .external_id, title: .title}'
}

# 主流程
main() {
    print_info "开始测试同步规则 regex 操作符"
    echo "================================"
    
    login
    create_plugin
    test_regex_rules
    test_sync
    get_synced_tickets
    
    echo "================================"
    print_info "测试完成！"
    print_info "提示：regex 操作符支持标准的 Go 正则表达式语法"
    print_info "常用模式："
    print_info "  ^PROD-.*        - 以 PROD- 开头"
    print_info "  (?i)test        - 不区分大小写匹配 test"
    print_info "  \\[紧急\\]       - 匹配中括号内的文字"
    print_info "  (A|B|C)         - 匹配 A 或 B 或 C"
}

# 运行主流程
main