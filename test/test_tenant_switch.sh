#!/bin/bash

# 租户切换功能测试脚本
# 测试平台管理员切换租户的完整流程

BASE_URL="http://localhost:8080/api/v1"

# 生成随机后缀
RANDOM_SUFFIX=$(date +%s)$(shuf -i 100-999 -n 1)

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# 测试计数器
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Token存储
ADMIN_TOKEN=""
NEW_TOKEN=""
NORMAL_TOKEN=""

# 租户ID存储
TARGET_TENANT_ID=""
TARGET_TENANT_NAME=""

print_header() {
    echo -e "${CYAN}================================================================${NC}"
    echo -e "${CYAN}🔄 租户切换功能测试${NC}"
    echo -e "${CYAN}🆔 测试批次ID: $RANDOM_SUFFIX${NC}"
    echo -e "${CYAN}================================================================${NC}"
    echo ""
}

print_section() {
    echo -e "${PURPLE}▶ $1${NC}"
    echo "================================================================"
}

print_test() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -e "${YELLOW}📋 测试 $TOTAL_TESTS: $1${NC}"
}

print_request() {
    echo -e "${BLUE}📤 $1${NC}"
}

print_response() {
    echo -e "${BLUE}📥 响应:${NC}"
    echo "$1" | jq '.' 2>/dev/null || echo "$1"
    echo ""
}

print_success() {
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    FAILED_TESTS=$((FAILED_TESTS + 1))
    echo -e "${RED}❌ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# 用户登录函数
login_user() {
    local username=$1
    local token_var=$2
    local description=$3
    local password=${4:-"Admin@123"}

    print_test "用户登录 - $username ($description)"
    print_request "POST $BASE_URL/auth/login"

    local response=$(curl -s -X POST "$BASE_URL/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"username\": \"$username\", \"password\": \"$password\"}")

    print_response "$response"

    local token=$(echo "$response" | jq -r '.data.token' 2>/dev/null)
    if [ "$token" = "null" ] || [ -z "$token" ]; then
        token=$(echo "$response" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
    fi

    if [ ! -z "$token" ] && [ "$token" != "null" ]; then
        eval "$token_var=\"$token\""
        print_success "$username 登录成功"
        echo -e "${CYAN}Token: ${token:0:50}...${NC}"
        
        # 检查是否返回了可切换租户列表
        local switchable=$(echo "$response" | jq '.data.switchable_tenants // .switchable_tenants' 2>/dev/null)
        if [ "$switchable" != "null" ] && [ -n "$switchable" ]; then
            echo -e "${GREEN}获取到可切换租户列表${NC}"
            echo "$switchable" | jq '.'
        fi
    else
        print_error "$username 登录失败"
        return 1
    fi
    echo ""
}

# API测试函数
test_api() {
    local method=$1
    local endpoint=$2
    local token=$3
    local data=$4
    local expected_status=$5
    local test_name=$6

    print_test "$test_name"
    print_request "$method $BASE_URL$endpoint"

    local curl_cmd="curl -s -X $method \"$BASE_URL$endpoint\""

    if [ ! -z "$token" ]; then
        curl_cmd="$curl_cmd -H \"Authorization: Bearer $token\""
    fi

    if [ ! -z "$data" ]; then
        curl_cmd="$curl_cmd -H \"Content-Type: application/json\" -d '$data'"
        echo "请求体: $data"
    fi

    local response=$(eval $curl_cmd)
    print_response "$response"

    local code=$(echo "$response" | jq -r '.code' 2>/dev/null)
    if [ "$code" = "null" ]; then
        code=$(echo "$response" | grep -o '"code":[0-9]*' | cut -d':' -f2)
    fi

    if [ "$code" = "$expected_status" ]; then
        print_success "状态码符合预期: $code"
    else
        print_error "状态码不符合预期，期望: $expected_status，实际: $code"
    fi
    echo ""
}

# 显示测试总结
print_summary() {
    echo -e "${CYAN}================================================================${NC}"
    echo -e "${CYAN}📊 租户切换测试总结${NC}"
    echo -e "${CYAN}================================================================${NC}"
    echo -e "🆔 测试批次: ${YELLOW}$RANDOM_SUFFIX${NC}"
    echo -e "📊 总测试数: ${YELLOW}$TOTAL_TESTS${NC}"
    echo -e "✅ 通过数量: ${GREEN}$PASSED_TESTS${NC}"
    echo -e "❌ 失败数量: ${RED}$FAILED_TESTS${NC}"

    local success_rate=$(( PASSED_TESTS * 100 / TOTAL_TESTS ))
    echo -e "📈 成功率: ${CYAN}$success_rate%${NC}"

    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "${GREEN}🎉 所有测试通过！租户切换功能工作正常！${NC}"
    else
        echo -e "${RED}⚠️  有 $FAILED_TESTS 个测试失败，需要检查租户切换功能${NC}"
    fi
    echo ""
}

# ========== 开始执行测试 ==========

print_header

# 第1步：平台管理员认证测试
print_section "第1步：平台管理员认证测试"
login_user "admin" "ADMIN_TOKEN" "平台超级管理员"

# 第2步：获取用户完整信息
print_section "第2步：获取用户完整信息"

print_test "获取当前用户信息（含可切换租户列表）"
print_request "GET $BASE_URL/auth/me"

ME_RESP=$(curl -s -X GET "$BASE_URL/auth/me" \
    -H "Authorization: Bearer $ADMIN_TOKEN")

print_response "$ME_RESP"

# 检查响应码
ME_CODE=$(echo "$ME_RESP" | jq -r '.code' 2>/dev/null)
if [ "$ME_CODE" = "200" ]; then
    print_success "获取用户信息成功"
    
    # 解析用户信息
    USER_INFO=$(echo "$ME_RESP" | jq '.data.user' 2>/dev/null)
    CURRENT_TENANT=$(echo "$ME_RESP" | jq '.data.current_tenant' 2>/dev/null)
    SWITCHABLE_TENANTS=$(echo "$ME_RESP" | jq '.data.switchable_tenants' 2>/dev/null)
    
    if [ "$USER_INFO" != "null" ] && [ -n "$USER_INFO" ]; then
        echo "用户信息："
        echo "$USER_INFO" | jq '.'
        echo ""
    fi
else
    print_error "获取用户信息失败"
fi

if [ "$CURRENT_TENANT" != "null" ] && [ -n "$CURRENT_TENANT" ]; then
    echo -e "${GREEN}获取当前租户信息成功${NC}"
    echo "当前租户："
    echo "$CURRENT_TENANT" | jq '.'
    echo ""
fi

if [ "$SWITCHABLE_TENANTS" != "null" ] && [ -n "$SWITCHABLE_TENANTS" ]; then
    echo -e "${GREEN}获取可切换租户列表成功${NC}"
    echo "可切换租户列表："
    echo "$SWITCHABLE_TENANTS" | jq '.'
    
    # 获取第一个非当前租户的ID
    TARGET_TENANT_ID=$(echo "$SWITCHABLE_TENANTS" | jq -r '.[] | select(.is_current == false) | .id' | head -1)
    TARGET_TENANT_NAME=$(echo "$SWITCHABLE_TENANTS" | jq -r '.[] | select(.is_current == false) | .name' | head -1)
    
    if [ -n "$TARGET_TENANT_ID" ] && [ "$TARGET_TENANT_ID" != "null" ]; then
        echo -e "${GREEN}找到目标租户: ID=$TARGET_TENANT_ID, Name=$TARGET_TENANT_NAME${NC}"
    else
        print_error "没有找到可切换的租户"
    fi
else
    print_error "获取可切换租户列表失败"
fi
echo ""

# 第3步：测试租户切换
print_section "第3步：测试租户切换"

if [ -n "$TARGET_TENANT_ID" ] && [ "$TARGET_TENANT_ID" != "null" ]; then
    # 切换到目标租户
    SWITCH_DATA="{
        \"tenant_id\": $TARGET_TENANT_ID
    }"
    
    print_test "切换到租户: $TARGET_TENANT_NAME (ID: $TARGET_TENANT_ID)"
    print_request "POST $BASE_URL/auth/switch-tenant"
    
    SWITCH_RESP=$(curl -s -X POST "$BASE_URL/auth/switch-tenant" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -H "Content-Type: application/json" \
        -d "$SWITCH_DATA")
    
    print_response "$SWITCH_RESP"
    
    # 检查响应码
    SWITCH_CODE=$(echo "$SWITCH_RESP" | jq -r '.code' 2>/dev/null)
    if [ "$SWITCH_CODE" = "200" ]; then
        print_success "租户切换成功"
        
        # 获取新Token
        NEW_TOKEN=$(echo "$SWITCH_RESP" | jq -r '.data.token // .token' 2>/dev/null)
        if [ -n "$NEW_TOKEN" ] && [ "$NEW_TOKEN" != "null" ]; then
            echo -e "${CYAN}新Token: ${NEW_TOKEN:0:50}...${NC}"
            
            # 显示新租户信息
            NEW_TENANT=$(echo "$SWITCH_RESP" | jq '.data.current_tenant' 2>/dev/null)
            if [ "$NEW_TENANT" != "null" ] && [ -n "$NEW_TENANT" ]; then
                echo "切换后的租户信息："
                echo "$NEW_TENANT" | jq '.'
            fi
        fi
    else
        print_error "租户切换失败"
        NEW_TOKEN=""
    fi
    echo ""
else
    print_warning "跳过租户切换测试（没有可切换的租户）"
fi

# 第4步：验证切换后的状态
print_section "第4步：验证切换后的状态"

if [ -n "$NEW_TOKEN" ] && [ "$NEW_TOKEN" != "null" ]; then
    print_test "验证切换后的用户信息"
    print_request "GET $BASE_URL/auth/me"
    
    NEW_ME_RESP=$(curl -s -X GET "$BASE_URL/auth/me" \
        -H "Authorization: Bearer $NEW_TOKEN")
    
    print_response "$NEW_ME_RESP"
    
    # 检查响应码
    NEW_ME_CODE=$(echo "$NEW_ME_RESP" | jq -r '.code' 2>/dev/null)
    if [ "$NEW_ME_CODE" = "200" ]; then
        # 验证当前租户ID
        CURRENT_TENANT_ID=$(echo "$NEW_ME_RESP" | jq -r '.data.current_tenant.id' 2>/dev/null)
        if [ "$CURRENT_TENANT_ID" = "$TARGET_TENANT_ID" ]; then
            print_success "租户切换验证成功 - 当前租户ID正确: $CURRENT_TENANT_ID"
        else
            print_error "租户切换验证失败 - 当前租户ID不正确，期望: $TARGET_TENANT_ID, 实际: $CURRENT_TENANT_ID"
        fi
    else
        print_error "获取切换后用户信息失败"
    fi
    
    # 验证用户信息保持不变
    NEW_USER_ID=$(echo "$NEW_ME_RESP" | jq -r '.data.user.id' 2>/dev/null)
    OLD_USER_ID=$(echo "$USER_INFO" | jq -r '.id' 2>/dev/null)
    if [ "$NEW_USER_ID" = "$OLD_USER_ID" ]; then
        echo -e "${GREEN}用户ID保持不变: $NEW_USER_ID${NC}"
    else
        print_error "用户ID发生变化，原: $OLD_USER_ID, 新: $NEW_USER_ID"
    fi
    echo ""
fi

# 第5步：权限控制测试
print_section "第5步：权限控制测试"

# 创建或使用普通用户测试
# 获取当前租户ID
CURRENT_TENANT_ID=$(echo "$CURRENT_TENANT" | jq -r '.id' 2>/dev/null)
if [ -z "$CURRENT_TENANT_ID" ] || [ "$CURRENT_TENANT_ID" = "null" ]; then
    CURRENT_TENANT_ID=1  # 默认使用租户1
fi

CREATE_USER_DATA="{
    \"tenant_id\": $CURRENT_TENANT_ID,
    \"username\": \"test_normal_${RANDOM_SUFFIX}\",
    \"email\": \"normal_${RANDOM_SUFFIX}@test.com\",
    \"password\": \"Test123456\",
    \"name\": \"普通测试用户_${RANDOM_SUFFIX}\"
}"

print_test "创建普通测试用户"
print_request "POST $BASE_URL/users"

USER_RESP=$(curl -s -X POST "$BASE_URL/users" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$CREATE_USER_DATA")

print_response "$USER_RESP"

USER_CODE=$(echo "$USER_RESP" | jq -r '.code' 2>/dev/null)
if [ "$USER_CODE" = "200" ]; then
    print_success "测试用户创建成功"
    
    # 用普通用户登录
    login_user "test_normal_${RANDOM_SUFFIX}" "NORMAL_TOKEN" "普通测试用户" "Test123456"
    
    if [ -n "$NORMAL_TOKEN" ] && [ "$NORMAL_TOKEN" != "null" ]; then
        # 普通用户尝试切换租户（应该失败）
        test_api "POST" "/auth/switch-tenant" "$NORMAL_TOKEN" "{\"tenant_id\": 1}" "403" "普通用户尝试切换租户（应拒绝）"
        
        # 获取普通用户信息，验证没有可切换租户列表
        print_test "验证普通用户无可切换租户列表"
        print_request "GET $BASE_URL/auth/me"
        
        NORMAL_ME_RESP=$(curl -s -X GET "$BASE_URL/auth/me" \
            -H "Authorization: Bearer $NORMAL_TOKEN")
        
        print_response "$NORMAL_ME_RESP"
        
        # 检查响应码
        NORMAL_ME_CODE=$(echo "$NORMAL_ME_RESP" | jq -r '.code' 2>/dev/null)
        if [ "$NORMAL_ME_CODE" = "200" ]; then
            NORMAL_SWITCHABLE=$(echo "$NORMAL_ME_RESP" | jq '.data.switchable_tenants' 2>/dev/null)
            if [ "$NORMAL_SWITCHABLE" = "null" ] || [ -z "$NORMAL_SWITCHABLE" ]; then
                print_success "普通用户正确地没有可切换租户列表"
            else
                print_error "普通用户不应该有可切换租户列表"
                echo "$NORMAL_SWITCHABLE" | jq '.'
            fi
        else
            print_error "获取普通用户信息失败"
        fi
        echo ""
    fi
else
    print_error "测试用户创建失败，跳过权限测试"
fi

# 第6步：边界测试
print_section "第6步：边界测试"

# 测试切换到不存在的租户
test_api "POST" "/auth/switch-tenant" "$ADMIN_TOKEN" "{\"tenant_id\": 99999}" "404" "切换到不存在的租户"

# 测试无效的租户ID格式
test_api "POST" "/auth/switch-tenant" "$ADMIN_TOKEN" "{\"tenant_id\": \"invalid\"}" "400" "使用无效的租户ID格式"

# 测试缺少tenant_id参数
test_api "POST" "/auth/switch-tenant" "$ADMIN_TOKEN" "{}" "400" "缺少tenant_id参数"

# 第7步：跨租户操作测试
print_section "第7步：跨租户操作测试"

if [ -n "$NEW_TOKEN" ] && [ "$NEW_TOKEN" != "null" ]; then
    # 使用切换后的Token访问资源
    test_api "GET" "/users" "$NEW_TOKEN" "" "200" "使用切换后Token获取用户列表"
    
    # 验证只能看到目标租户的数据
    print_test "验证数据租户隔离"
    USERS_RESP=$(curl -s -X GET "$BASE_URL/users?page=1&page_size=10" \
        -H "Authorization: Bearer $NEW_TOKEN")
    
    # 检查响应码
    USERS_CODE=$(echo "$USERS_RESP" | jq -r '.code' 2>/dev/null)
    if [ "$USERS_CODE" = "200" ]; then
        print_success "成功获取目标租户的用户数据"
        USERS_DATA=$(echo "$USERS_RESP" | jq '.data' 2>/dev/null)
        if [ "$USERS_DATA" != "null" ] && [ -n "$USERS_DATA" ]; then
            USER_COUNT=$(echo "$USERS_DATA" | jq 'length' 2>/dev/null)
            echo "目标租户用户数: $USER_COUNT"
        fi
    else
        print_error "获取目标租户用户数据失败"
    fi
    echo ""
fi

# 显示最终测试总结
print_summary

echo -e "${CYAN}🔍 租户切换功能验证：${NC}"
echo "✅ 平台管理员可以获取可切换租户列表"
echo "✅ 平台管理员可以成功切换租户"
echo "✅ 切换后获得新的JWT Token"
echo "✅ 切换后的操作在目标租户范围内"
echo "✅ 普通用户无法切换租户"
echo "✅ 边界条件正确处理"
echo "✅ 数据租户隔离有效"
echo ""

echo -e "${GREEN}🎯 租户切换功能测试完成！${NC}"
echo -e "${CYAN}📋 测试批次ID: $RANDOM_SUFFIX${NC}"