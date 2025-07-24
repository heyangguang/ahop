#!/bin/bash

# 快速测试脚本
# 用于快速验证特定功能

BASE_URL="http://localhost:8080/api/v1"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 默认使用admin账号
USERNAME="admin"
PASSWORD="Admin@123"

print_header() {
    echo -e "${BLUE}=== AHOP 快速测试工具 ===${NC}"
    echo ""
}

print_usage() {
    echo "用法: $0 <功能> [选项]"
    echo ""
    echo "功能:"
    echo "  health      - 健康检查"
    echo "  login       - 登录测试"
    echo "  users       - 用户列表"
    echo "  tenants     - 租户列表"
    echo "  roles       - 角色列表"
    echo "  permissions - 权限列表"
    echo "  credentials - 凭证列表"
    echo "  me          - 当前用户信息"
    echo ""
    echo "选项:"
    echo "  -u <用户名>  - 指定登录用户名（默认: admin）"
    echo "  -p <密码>    - 指定登录密码（默认: Admin@123）"
    echo ""
    echo "示例:"
    echo "  $0 health                    # 检查服务健康状态"
    echo "  $0 login                     # 使用默认账号登录"
    echo "  $0 users                     # 获取用户列表"
    echo "  $0 credentials -u testuser   # 使用testuser账号查看凭证"
}

# 解析命令行参数
if [ $# -eq 0 ] || [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    print_header
    print_usage
    exit 0
fi

FUNCTION=$1
shift

# 解析选项
while getopts "u:p:" opt; do
    case $opt in
        u) USERNAME="$OPTARG";;
        p) PASSWORD="$OPTARG";;
        *) print_usage; exit 1;;
    esac
done

# 健康检查
test_health() {
    echo -e "${YELLOW}检查服务健康状态...${NC}"
    curl -s -X GET "$BASE_URL/health" | jq '.'
}

# 登录测试
test_login() {
    echo -e "${YELLOW}登录用户: $USERNAME${NC}"
    local response=$(curl -s -X POST "$BASE_URL/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"username\": \"$USERNAME\", \"password\": \"$PASSWORD\"}")
    
    echo "$response" | jq '.'
    
    local token=$(echo "$response" | jq -r '.data.token // .token' 2>/dev/null)
    if [ -n "$token" ] && [ "$token" != "null" ]; then
        echo -e "${GREEN}✅ 登录成功${NC}"
        echo -e "${BLUE}Token: ${token:0:50}...${NC}"
        export AUTH_TOKEN="$token"
    else
        echo -e "${RED}❌ 登录失败${NC}"
        exit 1
    fi
}

# 获取Token
get_token() {
    if [ -z "$AUTH_TOKEN" ]; then
        echo -e "${YELLOW}正在登录...${NC}"
        local response=$(curl -s -X POST "$BASE_URL/auth/login" \
            -H "Content-Type: application/json" \
            -d "{\"username\": \"$USERNAME\", \"password\": \"$PASSWORD\"}")
        
        AUTH_TOKEN=$(echo "$response" | jq -r '.data.token // .token' 2>/dev/null)
        if [ -z "$AUTH_TOKEN" ] || [ "$AUTH_TOKEN" = "null" ]; then
            echo -e "${RED}❌ 登录失败${NC}"
            echo "$response" | jq '.'
            exit 1
        fi
        echo -e "${GREEN}✅ 登录成功${NC}"
    fi
}

# 通用API调用
call_api() {
    local endpoint=$1
    get_token
    echo -e "${YELLOW}调用: GET $BASE_URL$endpoint${NC}"
    curl -s -X GET "$BASE_URL$endpoint" \
        -H "Authorization: Bearer $AUTH_TOKEN" | jq '.'
}

# 执行测试
print_header

case $FUNCTION in
    health)
        test_health
        ;;
    login)
        test_login
        ;;
    users)
        call_api "/users?page=1&page_size=10"
        ;;
    tenants)
        call_api "/tenants?page=1&page_size=10"
        ;;
    roles)
        call_api "/roles/tenant/1"
        ;;
    permissions)
        call_api "/permissions"
        ;;
    credentials)
        call_api "/credentials?page=1&page_size=10"
        ;;
    me)
        call_api "/auth/me"
        ;;
    *)
        echo -e "${RED}❌ 未知功能: $FUNCTION${NC}"
        echo ""
        print_usage
        exit 1
        ;;
esac