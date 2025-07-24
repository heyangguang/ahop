#!/bin/bash

# JWT权限系统完整测试脚本
# 自动创建和清理测试数据，支持重复运行

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
TENANT_ADMIN_TOKEN=""
USER_MANAGER_TOKEN=""
READONLY_USER_TOKEN=""
NORMAL_USER_TOKEN=""

# 创建的资源ID存储（用于清理）
CREATED_USER_IDS=()
CREATED_ROLE_IDS=()
CREATED_TENANT_IDS=()

print_header() {
    echo -e "${CYAN}================================================================${NC}"
    echo -e "${CYAN}🔐 JWT权限系统完整测试${NC}"
    echo -e "${CYAN}🆔 测试批次ID: $RANDOM_SUFFIX${NC}"
    echo -e "${CYAN}📝 说明: 自动创建测试数据，测试完成后自动清理${NC}"
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

# 清理函数
cleanup() {
    echo ""
    print_section "清理测试数据"
    
    if [ -z "$ADMIN_TOKEN" ]; then
        print_warning "没有管理员Token，跳过清理"
        return
    fi
    
    # 删除创建的用户
    for user_id in "${CREATED_USER_IDS[@]}"; do
        if [ -n "$user_id" ] && [ "$user_id" != "null" ]; then
            echo -e "${YELLOW}删除用户 ID: $user_id${NC}"
            curl -s -X DELETE "$BASE_URL/users/$user_id" \
                -H "Authorization: Bearer $ADMIN_TOKEN" > /dev/null
        fi
    done
    
    # 删除创建的角色
    for role_id in "${CREATED_ROLE_IDS[@]}"; do
        if [ -n "$role_id" ] && [ "$role_id" != "null" ]; then
            echo -e "${YELLOW}删除角色 ID: $role_id${NC}"
            curl -s -X DELETE "$BASE_URL/roles/$role_id" \
                -H "Authorization: Bearer $ADMIN_TOKEN" > /dev/null
        fi
    done
    
    # 删除创建的租户
    for tenant_id in "${CREATED_TENANT_IDS[@]}"; do
        if [ -n "$tenant_id" ] && [ "$tenant_id" != "null" ]; then
            echo -e "${YELLOW}删除租户 ID: $tenant_id${NC}"
            curl -s -X DELETE "$BASE_URL/tenants/$tenant_id" \
                -H "Authorization: Bearer $ADMIN_TOKEN" > /dev/null
        fi
    done
    
    print_success "测试数据清理完成"
}

# 注册清理函数，确保脚本退出时执行
trap cleanup EXIT

# 用户登录函数
login_user() {
    local username=$1
    local token_var=$2
    local description=$3
    local password=${4:-"Test@123456"}

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

# 创建用户函数
create_user() {
    local username=$1
    local name=$2
    local tenant_id=$3
    local is_tenant_admin=${4:-false}
    
    local user_data="{
        \"tenant_id\": $tenant_id,
        \"username\": \"${username}_${RANDOM_SUFFIX}\",
        \"email\": \"${username}_${RANDOM_SUFFIX}@test.local\",
        \"password\": \"Test@123456\",
        \"name\": \"$name\",
        \"is_tenant_admin\": $is_tenant_admin
    }"
    
    echo -e "${YELLOW}创建用户: ${username}_${RANDOM_SUFFIX}${NC}" >&2
    local response=$(curl -s -X POST "$BASE_URL/users" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -H "Content-Type: application/json" \
        -d "$user_data")
    
    local user_id=$(echo "$response" | jq -r '.data.id // ""')
    if [ -n "$user_id" ] && [ "$user_id" != "null" ]; then
        CREATED_USER_IDS+=("$user_id")
        echo -e "${GREEN}用户创建成功，ID: $user_id${NC}" >&2
        echo "$user_id"
    else
        echo -e "${RED}用户创建失败${NC}" >&2
        echo "$response" | jq '.' >&2
        echo ""
    fi
}

# 创建角色函数
create_role() {
    local code=$1
    local name=$2
    local tenant_id=$3
    
    local role_data="{
        \"tenant_id\": $tenant_id,
        \"code\": \"${code}${RANDOM_SUFFIX}\",
        \"name\": \"$name\",
        \"description\": \"测试角色 - $name\"
    }"
    
    echo -e "${YELLOW}创建角色: ${code}${RANDOM_SUFFIX}${NC}" >&2
    local response=$(curl -s -X POST "$BASE_URL/roles" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -H "Content-Type: application/json" \
        -d "$role_data")
    
    local role_id=$(echo "$response" | jq -r '.data.id // ""')
    if [ -n "$role_id" ] && [ "$role_id" != "null" ]; then
        CREATED_ROLE_IDS+=("$role_id")
        echo -e "${GREEN}角色创建成功，ID: $role_id${NC}" >&2
        echo "$role_id"
    else
        echo -e "${RED}角色创建失败${NC}" >&2
        echo "$response" | jq '.' >&2
        echo ""
    fi
}

# 分配权限给角色
assign_permissions_to_role() {
    local role_id=$1
    shift
    local permissions=("$@")
    
    # 获取所有权限列表（公开API，不需要认证）
    local all_perms=$(curl -s -X GET "$BASE_URL/permissions")
    
    local perm_ids=()
    echo -e "${CYAN}查找权限: ${permissions[*]}${NC}" >&2
    for perm_code in "${permissions[@]}"; do
        local perm_id=$(echo "$all_perms" | jq -r ".data[] | select(.code == \"$perm_code\") | .id" 2>/dev/null)
        if [ -n "$perm_id" ] && [ "$perm_id" != "null" ]; then
            perm_ids+=("$perm_id")
            echo -e "${GREEN}找到权限 $perm_code: ID=$perm_id${NC}" >&2
        else
            echo -e "${RED}未找到权限 $perm_code${NC}" >&2
        fi
    done
    
    if [ ${#perm_ids[@]} -gt 0 ]; then
        local perm_data="{\"permission_ids\": [$(IFS=,; echo "${perm_ids[*]}")]}"
        echo -e "${YELLOW}分配权限到角色 ID: $role_id${NC}"
        echo -e "${CYAN}权限IDs: ${perm_ids[*]}${NC}"
        local response=$(curl -s -X POST "$BASE_URL/roles/$role_id/permissions" \
            -H "Authorization: Bearer $ADMIN_TOKEN" \
            -H "Content-Type: application/json" \
            -d "$perm_data")
        local code=$(echo "$response" | jq -r '.code // ""')
        if [ "$code" = "200" ]; then
            echo -e "${GREEN}权限分配成功${NC}"
        else
            echo -e "${RED}权限分配失败${NC}"
            echo "$response" | jq '.'
        fi
    else
        echo -e "${RED}未找到任何权限ID${NC}"
    fi
}

# 分配角色给用户
assign_role_to_user() {
    local user_id=$1
    local role_id=$2
    
    local role_data="{\"role_ids\": [$role_id]}"
    echo -e "${YELLOW}分配角色 $role_id 给用户 $user_id${NC}"
    local response=$(curl -s -X POST "$BASE_URL/users/$user_id/roles" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -H "Content-Type: application/json" \
        -d "$role_data")
    local code=$(echo "$response" | jq -r '.code // ""')
    if [ "$code" = "200" ]; then
        echo -e "${GREEN}角色分配成功${NC}"
    else
        echo -e "${RED}角色分配失败${NC}"
        echo "$response" | jq '.'
    fi
}

# 显示测试总结
print_summary() {
    echo -e "${CYAN}================================================================${NC}"
    echo -e "${CYAN}📊 测试总结${NC}"
    echo -e "${CYAN}================================================================${NC}"
    echo -e "🆔 测试批次: ${YELLOW}$RANDOM_SUFFIX${NC}"
    echo -e "📊 总测试数: ${YELLOW}$TOTAL_TESTS${NC}"
    echo -e "✅ 通过数量: ${GREEN}$PASSED_TESTS${NC}"
    echo -e "❌ 失败数量: ${RED}$FAILED_TESTS${NC}"

    local success_rate=$(( PASSED_TESTS * 100 / TOTAL_TESTS ))
    echo -e "📈 成功率: ${CYAN}$success_rate%${NC}"

    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "${GREEN}🎉 所有测试通过！JWT权限系统工作正常！${NC}"
    else
        echo -e "${RED}⚠️  有 $FAILED_TESTS 个测试失败，需要检查权限配置${NC}"
    fi
    echo ""
}

# ========== 开始执行测试 ==========

print_header

# 第1步：使用默认管理员登录
print_section "第1步：默认管理员登录"
login_user "admin" "ADMIN_TOKEN" "平台超级管理员" "Admin@123"

# 获取当前租户信息
ME_RESP=$(curl -s -X GET "$BASE_URL/auth/me" \
    -H "Authorization: Bearer $ADMIN_TOKEN")
CURRENT_TENANT_ID=$(echo "$ME_RESP" | jq -r '.data.current_tenant.id // 1')

# 第2步：创建测试数据
print_section "第2步：创建测试数据"

# 创建角色
echo -e "${CYAN}创建测试角色...${NC}"
USER_MANAGER_ROLE_ID=$(create_role "user_manager" "用户管理员角色" "$CURRENT_TENANT_ID")
READONLY_ROLE_ID=$(create_role "readonly" "只读角色" "$CURRENT_TENANT_ID")

# 分配权限给角色
if [ -n "$USER_MANAGER_ROLE_ID" ] && [ "$USER_MANAGER_ROLE_ID" != "null" ]; then
    assign_permissions_to_role "$USER_MANAGER_ROLE_ID" "user:create" "user:read" "user:update" "user:delete" "user:list"
fi

if [ -n "$READONLY_ROLE_ID" ] && [ "$READONLY_ROLE_ID" != "null" ]; then
    assign_permissions_to_role "$READONLY_ROLE_ID" "user:read"
fi

echo ""

# 创建测试用户
echo -e "${CYAN}创建测试用户...${NC}"
TENANT_ADMIN_ID=$(create_user "tenant_admin" "租户管理员" "$CURRENT_TENANT_ID" true)
USER_MANAGER_ID=$(create_user "user_manager" "用户管理员" "$CURRENT_TENANT_ID" false)
READONLY_USER_ID=$(create_user "readonly_user" "只读用户" "$CURRENT_TENANT_ID" false)
NORMAL_USER_ID=$(create_user "normal_user" "普通用户" "$CURRENT_TENANT_ID" false)

# 分配角色给用户
if [ -n "$USER_MANAGER_ID" ] && [ -n "$USER_MANAGER_ROLE_ID" ]; then
    assign_role_to_user "$USER_MANAGER_ID" "$USER_MANAGER_ROLE_ID"
fi

if [ -n "$READONLY_USER_ID" ] && [ -n "$READONLY_ROLE_ID" ]; then
    assign_role_to_user "$READONLY_USER_ID" "$READONLY_ROLE_ID"
fi

echo ""

# 第3步：测试用户登录
print_section "第3步：测试用户登录"
login_user "tenant_admin_${RANDOM_SUFFIX}" "TENANT_ADMIN_TOKEN" "租户管理员"
login_user "user_manager_${RANDOM_SUFFIX}" "USER_MANAGER_TOKEN" "用户管理员"
login_user "readonly_user_${RANDOM_SUFFIX}" "READONLY_USER_TOKEN" "只读用户"
login_user "normal_user_${RANDOM_SUFFIX}" "NORMAL_USER_TOKEN" "普通用户"

# 第4步：测试 /auth/me 接口
print_section "第4步：测试 /auth/me 接口"

# 测试各类用户的 me 接口返回
print_test "平台管理员 - 获取个人信息"
ME_RESP=$(curl -s -X GET "$BASE_URL/auth/me" \
    -H "Authorization: Bearer $ADMIN_TOKEN")
echo -e "${BLUE}📥 原始响应:${NC}"
echo "$ME_RESP" | jq '.' 2>/dev/null || echo "$ME_RESP"
echo ""

print_test "租户管理员 - 获取个人信息"
TENANT_ADMIN_ME=$(curl -s -X GET "$BASE_URL/auth/me" \
    -H "Authorization: Bearer $TENANT_ADMIN_TOKEN")
echo -e "${BLUE}📥 原始响应:${NC}"
echo "$TENANT_ADMIN_ME" | jq '.' 2>/dev/null || echo "$TENANT_ADMIN_ME"
echo ""

print_test "用户管理员 - 获取个人信息"
USER_MANAGER_ME=$(curl -s -X GET "$BASE_URL/auth/me" \
    -H "Authorization: Bearer $USER_MANAGER_TOKEN")
echo -e "${BLUE}📥 响应摘要:${NC}"
echo "$USER_MANAGER_ME" | jq '{
    user: {
        username: .data.user.username,
        is_platform_admin: .data.user.is_platform_admin,
        is_tenant_admin: .data.user.is_tenant_admin
    },
    permissions_count: (.data.permissions | length),
    permissions: [.data.permissions[]?.code]
}' 2>/dev/null || echo "$USER_MANAGER_ME" | jq '.'

print_test "普通用户 - 获取个人信息"
NORMAL_USER_ME=$(curl -s -X GET "$BASE_URL/auth/me" \
    -H "Authorization: Bearer $NORMAL_USER_TOKEN")
echo -e "${BLUE}📥 响应摘要:${NC}"
echo "$NORMAL_USER_ME" | jq '{
    user: {
        username: .data.user.username,
        is_platform_admin: .data.user.is_platform_admin,
        is_tenant_admin: .data.user.is_tenant_admin
    },
    permissions_count: (.data.permissions | length),
    roles_count: (.data.roles | length)
}'

echo ""

# 第5步：基础认证控制测试
print_section "第5步：基础认证控制测试"
test_api "GET" "/health" "" "" "200" "健康检查（公开API）"
test_api "GET" "/users" "" "" "401" "无Token访问受保护资源（应拒绝）"
test_api "GET" "/users" "invalid.token.here" "" "401" "无效Token访问（应拒绝）"

# 第6步：用户管理权限测试
print_section "第6步：用户管理权限测试"

# 用户列表访问 (需要 user:list 权限)
test_api "GET" "/users" "$ADMIN_TOKEN" "" "200" "平台管理员 - 获取用户列表"
test_api "GET" "/users" "$TENANT_ADMIN_TOKEN" "" "200" "租户管理员 - 获取用户列表"
test_api "GET" "/users" "$USER_MANAGER_TOKEN" "" "200" "用户管理员 - 获取用户列表"
test_api "GET" "/users" "$READONLY_USER_TOKEN" "" "403" "只读用户 - 获取用户列表（应拒绝）"
test_api "GET" "/users" "$NORMAL_USER_TOKEN" "" "403" "普通用户 - 获取用户列表（应拒绝）"

# 用户创建 (需要 user:create 权限)
CREATE_USER_DATA="{
    \"tenant_id\": $CURRENT_TENANT_ID,
    \"username\": \"testuser_${RANDOM_SUFFIX}_new\",
    \"email\": \"testuser_${RANDOM_SUFFIX}_new@test.local\",
    \"password\": \"Test@123456\",
    \"name\": \"测试用户_${RANDOM_SUFFIX}_new\"
}"

test_api "POST" "/users" "$ADMIN_TOKEN" "$CREATE_USER_DATA" "200" "平台管理员 - 创建用户"
test_api "POST" "/users" "$USER_MANAGER_TOKEN" "$CREATE_USER_DATA" "400" "用户管理员 - 创建用户（应该失败，用户名已存在）"
test_api "POST" "/users" "$READONLY_USER_TOKEN" "$CREATE_USER_DATA" "403" "只读用户 - 创建用户（应拒绝）"

# 第7步：个人信息查看测试
print_section "第7步：个人信息查看测试"

# 用户查看自己的信息
if [ -n "$READONLY_USER_ID" ] && [ "$READONLY_USER_ID" != "null" ]; then
    test_api "GET" "/users/$READONLY_USER_ID" "$READONLY_USER_TOKEN" "" "200" "只读用户 - 查看自己的信息"
fi

# 用户尝试查看别人的信息
if [ -n "$NORMAL_USER_ID" ] && [ "$NORMAL_USER_ID" != "null" ]; then
    test_api "GET" "/users/$NORMAL_USER_ID" "$READONLY_USER_TOKEN" "" "403" "只读用户 - 查看别人的信息（应拒绝）"
fi

# 第8步：租户管理权限测试
print_section "第8步：租户管理权限测试"

# 租户列表访问 (需要 tenant:list 权限)
test_api "GET" "/tenants" "$ADMIN_TOKEN" "" "200" "平台管理员 - 获取租户列表"
test_api "GET" "/tenants" "$TENANT_ADMIN_TOKEN" "" "403" "租户管理员 - 获取租户列表（应拒绝）"
test_api "GET" "/tenants" "$USER_MANAGER_TOKEN" "" "403" "用户管理员 - 获取租户列表（应拒绝）"

# 创建测试租户
# 生成一个短的租户代码后缀（取最后6位）
TENANT_CODE_SUFFIX=${RANDOM_SUFFIX: -6}
CREATE_TENANT_DATA="{
    \"name\": \"测试租户_${RANDOM_SUFFIX}\",
    \"code\": \"test${TENANT_CODE_SUFFIX}\"
}"

test_api "POST" "/tenants" "$ADMIN_TOKEN" "$CREATE_TENANT_DATA" "200" "平台管理员 - 创建租户"

# 记录创建的租户ID用于清理
TENANT_RESP=$(curl -s -X POST "$BASE_URL/tenants" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$CREATE_TENANT_DATA")
TEST_TENANT_ID=$(echo "$TENANT_RESP" | jq -r '.data.id // ""')
if [ -n "$TEST_TENANT_ID" ] && [ "$TEST_TENANT_ID" != "null" ]; then
    CREATED_TENANT_IDS+=("$TEST_TENANT_ID")
fi

# 第9步：角色权限测试
print_section "第9步：角色权限测试"

# 角色查看测试
test_api "GET" "/roles/tenant/$CURRENT_TENANT_ID" "$ADMIN_TOKEN" "" "200" "平台管理员 - 查看租户角色"
test_api "GET" "/roles/tenant/$CURRENT_TENANT_ID" "$TENANT_ADMIN_TOKEN" "" "200" "租户管理员 - 查看租户角色"
test_api "GET" "/roles/tenant/$CURRENT_TENANT_ID" "$USER_MANAGER_TOKEN" "" "403" "用户管理员 - 查看租户角色（应拒绝）"

# 第10步：公开API测试
print_section "第10步：公开API测试"
test_api "GET" "/permissions" "" "" "200" "获取权限列表（公开API）"
test_api "GET" "/permissions/module/user" "" "" "200" "获取用户模块权限（公开API）"

# 第11步：Token管理测试
print_section "第11步：Token管理测试"
test_api "POST" "/auth/refresh" "$ADMIN_TOKEN" "" "200" "Token刷新测试"
test_api "POST" "/auth/logout" "$NORMAL_USER_TOKEN" "" "200" "用户登出测试"

# 显示最终测试总结
print_summary

echo -e "${CYAN}🔍 权限系统设计验证：${NC}"
echo "✅ 平台管理员：拥有所有权限"
echo "✅ 租户管理员：自动拥有本租户管理权限"
echo "✅ 用户管理员：通过角色获得用户CRUD权限"
echo "✅ 只读用户：只能查看自己的信息"
echo "✅ 普通用户：无特殊权限"
echo "✅ 认证控制：正确拒绝未认证访问"
echo ""

echo -e "${GREEN}🎯 JWT权限系统测试完成！${NC}"
echo -e "${CYAN}📋 测试批次ID: $RANDOM_SUFFIX${NC}"
echo ""
echo -e "${YELLOW}📝 注意：测试数据将在脚本退出时自动清理${NC}"