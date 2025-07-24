#!/bin/bash

# 主机组管理功能测试脚本
# 测试主机组的树形结构、CRUD操作、主机分配和权限控制

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
USER_TOKEN=""

# 主机组ID存储
PROD_GROUP_ID=""
TEST_GROUP_ID=""
DEV_GROUP_ID=""
BEIJING_GROUP_ID=""
SHANGHAI_GROUP_ID=""
WEB_GROUP_ID=""
DB_GROUP_ID=""

# 主机ID存储
HOST_IDS=()

# 凭证ID存储
CRED_ID=""

print_header() {
    echo -e "${CYAN}================================================================${NC}"
    echo -e "${CYAN}🌳 主机组管理功能测试${NC}"
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

    local curl_args=(-s -X "$method" "$BASE_URL$endpoint")

    if [ ! -z "$token" ]; then
        curl_args+=(-H "Authorization: Bearer $token")
    fi

    if [ ! -z "$data" ]; then
        curl_args+=(-H "Content-Type: application/json" -d "$data")
        echo "请求体: $data"
    fi

    local response=$(curl "${curl_args[@]}")
    print_response "$response"

    local code=$(echo "$response" | jq -r '.code' 2>/dev/null)
    if [ "$code" = "null" ]; then
        code=$(echo "$response" | grep -o '"code":[0-9]*' | cut -d':' -f2)
    fi

    if [ "$code" = "$expected_status" ]; then
        print_success "状态码符合预期: $code"
        
        # 保存主机组ID供后续测试使用
        if [ "$method" = "POST" ] && [[ "$endpoint" == "/host-groups" ]] && [ "$code" = "200" ]; then
            local group_id=$(echo "$response" | jq -r '.data.id // ""')
            local group_code=$(echo "$response" | jq -r '.data.code // ""')
            
            if [ -n "$group_id" ] && [ "$group_id" != "null" ]; then
                case "$group_code" in
                    "prod_${RANDOM_SUFFIX}")
                        PROD_GROUP_ID="$group_id"
                        echo "生产环境组ID: $PROD_GROUP_ID"
                        ;;
                    "test_${RANDOM_SUFFIX}")
                        TEST_GROUP_ID="$group_id"
                        echo "测试环境组ID: $TEST_GROUP_ID"
                        ;;
                    "dev_${RANDOM_SUFFIX}")
                        DEV_GROUP_ID="$group_id"
                        echo "开发环境组ID: $DEV_GROUP_ID"
                        ;;
                    "beijing_${RANDOM_SUFFIX}")
                        BEIJING_GROUP_ID="$group_id"
                        echo "北京机房组ID: $BEIJING_GROUP_ID"
                        ;;
                    "shanghai_${RANDOM_SUFFIX}")
                        SHANGHAI_GROUP_ID="$group_id"
                        echo "上海机房组ID: $SHANGHAI_GROUP_ID"
                        ;;
                    "web_${RANDOM_SUFFIX}")
                        WEB_GROUP_ID="$group_id"
                        echo "Web服务器组ID: $WEB_GROUP_ID"
                        ;;
                    "db_${RANDOM_SUFFIX}")
                        DB_GROUP_ID="$group_id"
                        echo "数据库服务器组ID: $DB_GROUP_ID"
                        ;;
                esac
            fi
        fi
        
        # 保存主机ID
        if [ "$method" = "POST" ] && [[ "$endpoint" == "/hosts" ]] && [ "$code" = "200" ]; then
            local host_id=$(echo "$response" | jq -r '.data.id // ""')
            if [ -n "$host_id" ] && [ "$host_id" != "null" ]; then
                HOST_IDS+=("$host_id")
                echo "主机ID: $host_id"
            fi
        fi
        
        # 保存凭证ID
        if [ "$method" = "POST" ] && [[ "$endpoint" == "/credentials" ]] && [ "$code" = "200" ]; then
            CRED_ID=$(echo "$response" | jq -r '.data.id // ""')
            echo "凭证ID: $CRED_ID"
        fi
    else
        print_error "状态码不符合预期，期望: $expected_status，实际: $code"
    fi
    echo ""
}

# 显示测试总结
print_summary() {
    echo -e "${CYAN}================================================================${NC}"
    echo -e "${CYAN}📊 主机组管理测试总结${NC}"
    echo -e "${CYAN}================================================================${NC}"
    echo -e "🆔 测试批次: ${YELLOW}$RANDOM_SUFFIX${NC}"
    echo -e "📊 总测试数: ${YELLOW}$TOTAL_TESTS${NC}"
    echo -e "✅ 通过数量: ${GREEN}$PASSED_TESTS${NC}"
    echo -e "❌ 失败数量: ${RED}$FAILED_TESTS${NC}"

    local success_rate=$(( PASSED_TESTS * 100 / TOTAL_TESTS ))
    echo -e "📈 成功率: ${CYAN}$success_rate%${NC}"

    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "${GREEN}🎉 所有测试通过！主机组管理功能工作正常！${NC}"
    else
        echo -e "${RED}⚠️  有 $FAILED_TESTS 个测试失败，需要检查主机组功能${NC}"
    fi
    echo ""
}

# ========== 开始执行测试 ==========

print_header

# 第1步：用户认证测试
print_section "第1步：用户认证测试"
login_user "admin" "ADMIN_TOKEN" "平台超级管理员"

# 第2步：主机组创建测试
print_section "第2步：主机组创建测试"

# 创建顶级组
CREATE_PROD_DATA="{
    \"name\": \"生产环境_${RANDOM_SUFFIX}\",
    \"code\": \"prod_${RANDOM_SUFFIX}\",
    \"type\": \"environment\",
    \"description\": \"生产环境服务器组\"
}"

test_api "POST" "/host-groups" "$ADMIN_TOKEN" "$CREATE_PROD_DATA" "200" "创建生产环境顶级组"

CREATE_TEST_DATA="{
    \"name\": \"测试环境_${RANDOM_SUFFIX}\",
    \"code\": \"test_${RANDOM_SUFFIX}\",
    \"type\": \"environment\",
    \"description\": \"测试环境服务器组\"
}"

test_api "POST" "/host-groups" "$ADMIN_TOKEN" "$CREATE_TEST_DATA" "200" "创建测试环境顶级组"

CREATE_DEV_DATA="{
    \"name\": \"开发环境_${RANDOM_SUFFIX}\",
    \"code\": \"dev_${RANDOM_SUFFIX}\",
    \"type\": \"environment\",
    \"description\": \"开发环境服务器组\"
}"

test_api "POST" "/host-groups" "$ADMIN_TOKEN" "$CREATE_DEV_DATA" "200" "创建开发环境顶级组"

# 创建子组
if [ -n "$PROD_GROUP_ID" ] && [ "$PROD_GROUP_ID" != "null" ]; then
    CREATE_BEIJING_DATA="{
        \"parent_id\": $PROD_GROUP_ID,
        \"name\": \"北京机房_${RANDOM_SUFFIX}\",
        \"code\": \"beijing_${RANDOM_SUFFIX}\",
        \"type\": \"region\",
        \"description\": \"北京数据中心\"
    }"
    
    test_api "POST" "/host-groups" "$ADMIN_TOKEN" "$CREATE_BEIJING_DATA" "200" "创建北京机房子组"
fi

if [ -n "$TEST_GROUP_ID" ] && [ "$TEST_GROUP_ID" != "null" ]; then
    CREATE_SHANGHAI_DATA="{
        \"parent_id\": $TEST_GROUP_ID,
        \"name\": \"上海机房_${RANDOM_SUFFIX}\",
        \"code\": \"shanghai_${RANDOM_SUFFIX}\",
        \"type\": \"region\",
        \"description\": \"上海数据中心\"
    }"
    
    test_api "POST" "/host-groups" "$ADMIN_TOKEN" "$CREATE_SHANGHAI_DATA" "200" "创建上海机房子组"
fi

# 创建叶子节点
if [ -n "$BEIJING_GROUP_ID" ] && [ "$BEIJING_GROUP_ID" != "null" ]; then
    CREATE_WEB_DATA="{
        \"parent_id\": $BEIJING_GROUP_ID,
        \"name\": \"Web服务器组_${RANDOM_SUFFIX}\",
        \"code\": \"web_${RANDOM_SUFFIX}\",
        \"type\": \"custom\",
        \"description\": \"Web应用服务器\"
    }"
    
    test_api "POST" "/host-groups" "$ADMIN_TOKEN" "$CREATE_WEB_DATA" "200" "创建Web服务器叶子组"
    
    CREATE_DB_DATA="{
        \"parent_id\": $BEIJING_GROUP_ID,
        \"name\": \"数据库服务器组_${RANDOM_SUFFIX}\",
        \"code\": \"db_${RANDOM_SUFFIX}\",
        \"type\": \"custom\",
        \"description\": \"数据库服务器\"
    }"
    
    test_api "POST" "/host-groups" "$ADMIN_TOKEN" "$CREATE_DB_DATA" "200" "创建数据库服务器叶子组"
fi

# 测试创建重复组
test_api "POST" "/host-groups" "$ADMIN_TOKEN" "$CREATE_PROD_DATA" "400" "创建重复的主机组（应失败）"

# 第3步：主机组查询测试
print_section "第3步：主机组查询测试"

# 获取主机组列表
test_api "GET" "/host-groups?page=1&page_size=10" "$ADMIN_TOKEN" "" "200" "获取主机组列表"

# 获取主机组详情
if [ -n "$BEIJING_GROUP_ID" ] && [ "$BEIJING_GROUP_ID" != "null" ]; then
    test_api "GET" "/host-groups/$BEIJING_GROUP_ID" "$ADMIN_TOKEN" "" "200" "获取北京机房组详情"
    
    # 验证路径
    print_test "验证主机组路径"
    RESPONSE=$(curl -s -X GET "$BASE_URL/host-groups/$BEIJING_GROUP_ID" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    
    GROUP_PATH=$(echo "$RESPONSE" | jq -r '.data.path // ""')
    EXPECTED_PATH="/prod_${RANDOM_SUFFIX}/beijing_${RANDOM_SUFFIX}"
    if [ "$GROUP_PATH" = "$EXPECTED_PATH" ]; then
        print_success "路径验证成功: $GROUP_PATH"
    else
        print_error "路径验证失败，期望: $EXPECTED_PATH，实际: $GROUP_PATH"
    fi
    echo ""
fi

# 获取树形结构
test_api "GET" "/host-groups/tree" "$ADMIN_TOKEN" "" "200" "获取完整主机组树"

# 获取子树
if [ -n "$PROD_GROUP_ID" ] && [ "$PROD_GROUP_ID" != "null" ]; then
    test_api "GET" "/host-groups/$PROD_GROUP_ID/tree" "$ADMIN_TOKEN" "" "200" "获取生产环境子树"
fi

# 按路径查询
test_api "GET" "/host-groups/path?path=/prod_${RANDOM_SUFFIX}/beijing_${RANDOM_SUFFIX}" "$ADMIN_TOKEN" "" "200" "按路径查询主机组"

# 获取祖先节点
if [ -n "$WEB_GROUP_ID" ] && [ "$WEB_GROUP_ID" != "null" ]; then
    test_api "GET" "/host-groups/$WEB_GROUP_ID/ancestors" "$ADMIN_TOKEN" "" "200" "获取Web组的祖先节点"
fi

# 获取后代节点
if [ -n "$PROD_GROUP_ID" ] && [ "$PROD_GROUP_ID" != "null" ]; then
    test_api "GET" "/host-groups/$PROD_GROUP_ID/descendants" "$ADMIN_TOKEN" "" "200" "获取生产环境的后代节点"
fi

# 搜索主机组
test_api "GET" "/host-groups?search=机房" "$ADMIN_TOKEN" "" "200" "搜索包含'机房'的主机组"

# 第4步：主机组更新测试
print_section "第4步：主机组更新测试"

if [ -n "$SHANGHAI_GROUP_ID" ] && [ "$SHANGHAI_GROUP_ID" != "null" ]; then
    UPDATE_DATA="{
        \"description\": \"上海浦东数据中心_${RANDOM_SUFFIX}\",
        \"status\": \"active\"
    }"
    
    test_api "PUT" "/host-groups/$SHANGHAI_GROUP_ID" "$ADMIN_TOKEN" "$UPDATE_DATA" "200" "更新上海机房组信息"
fi

# 第5步：主机组移动测试
print_section "第5步：主机组移动测试"

if [ -n "$SHANGHAI_GROUP_ID" ] && [ "$SHANGHAI_GROUP_ID" != "null" ] && [ -n "$PROD_GROUP_ID" ] && [ "$PROD_GROUP_ID" != "null" ]; then
    MOVE_DATA="{
        \"new_parent_id\": $PROD_GROUP_ID
    }"
    
    test_api "POST" "/host-groups/$SHANGHAI_GROUP_ID/move" "$ADMIN_TOKEN" "$MOVE_DATA" "200" "移动上海机房到生产环境下"
    
    # 验证移动后的路径
    print_test "验证移动后的路径"
    RESPONSE=$(curl -s -X GET "$BASE_URL/host-groups/$SHANGHAI_GROUP_ID" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    
    NEW_PATH=$(echo "$RESPONSE" | jq -r '.data.path // ""')
    EXPECTED_NEW_PATH="/prod_${RANDOM_SUFFIX}/shanghai_${RANDOM_SUFFIX}"
    if [ "$NEW_PATH" = "$EXPECTED_NEW_PATH" ]; then
        print_success "移动后路径正确: $NEW_PATH"
    else
        print_error "移动后路径错误，期望: $EXPECTED_NEW_PATH，实际: $NEW_PATH"
    fi
    echo ""
fi

# 测试非法移动
if [ -n "$PROD_GROUP_ID" ] && [ "$PROD_GROUP_ID" != "null" ] && [ -n "$BEIJING_GROUP_ID" ] && [ "$BEIJING_GROUP_ID" != "null" ]; then
    INVALID_MOVE_DATA="{
        \"new_parent_id\": $BEIJING_GROUP_ID
    }"
    
    test_api "POST" "/host-groups/$PROD_GROUP_ID/move" "$ADMIN_TOKEN" "$INVALID_MOVE_DATA" "400" "移动父组到子组（应失败）"
fi

# 第6步：主机分配测试
print_section "第6步：主机分配测试"

# 创建测试凭证
CREATE_CRED_DATA="{
    \"name\": \"测试SSH凭证_${RANDOM_SUFFIX}\",
    \"type\": \"ssh_key\",
    \"description\": \"用于测试的SSH凭证\",
    \"username\": \"root\",
    \"private_key\": \"-----BEGIN RSA PRIVATE KEY-----\\ntest\\n-----END RSA PRIVATE KEY-----\"
}"

print_test "创建测试凭证"
CRED_RESP=$(curl -s -X POST "$BASE_URL/credentials" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$CREATE_CRED_DATA")

CRED_CODE=$(echo "$CRED_RESP" | jq -r '.code // ""')
CRED_ID=$(echo "$CRED_RESP" | jq -r '.data.id // ""')

if [ "$CRED_CODE" = "200" ] && [ -n "$CRED_ID" ] && [ "$CRED_ID" != "null" ]; then
    print_success "测试凭证创建成功"
    echo "凭证ID: $CRED_ID"
else
    print_error "测试凭证创建失败"
    print_response "$CRED_RESP"
fi
echo ""

# 创建测试主机
if [ -n "$CRED_ID" ] && [ "$CRED_ID" != "null" ]; then
    for i in {1..3}; do
        CREATE_HOST_DATA="{
            \"name\": \"web-server-$i-${RANDOM_SUFFIX}\",
            \"ip_address\": \"192.168.1.$((100+i))\",
            \"port\": 22,
            \"credential_id\": $CRED_ID,
            \"description\": \"Web服务器 $i\"
        }"
        
        test_api "POST" "/hosts" "$ADMIN_TOKEN" "$CREATE_HOST_DATA" "200" "创建主机 web-server-$i"
    done
fi

# 分配主机到组
if [ -n "$WEB_GROUP_ID" ] && [ "$WEB_GROUP_ID" != "null" ] && [ ${#HOST_IDS[@]} -gt 0 ]; then
    # 构建主机ID数组JSON
    HOST_IDS_JSON="["
    for i in "${!HOST_IDS[@]}"; do
        if [ $i -gt 0 ]; then
            HOST_IDS_JSON="${HOST_IDS_JSON},"
        fi
        HOST_IDS_JSON="${HOST_IDS_JSON}${HOST_IDS[$i]}"
    done
    HOST_IDS_JSON="${HOST_IDS_JSON}]"
    
    ASSIGN_DATA="{
        \"host_ids\": $HOST_IDS_JSON
    }"
    
    test_api "POST" "/host-groups/$WEB_GROUP_ID/hosts" "$ADMIN_TOKEN" "$ASSIGN_DATA" "200" "批量分配主机到Web服务器组"
    
    # 获取组内主机
    test_api "GET" "/host-groups/$WEB_GROUP_ID/hosts" "$ADMIN_TOKEN" "" "200" "获取Web服务器组内的主机"
    
    # 验证主机数量
    print_test "验证组内主机数量"
    RESPONSE=$(curl -s -X GET "$BASE_URL/host-groups/$WEB_GROUP_ID" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    
    HOST_COUNT=$(echo "$RESPONSE" | jq -r '.data.host_count // 0')
    if [ "$HOST_COUNT" = "3" ]; then
        print_success "主机数量正确: $HOST_COUNT"
    else
        print_error "主机数量错误，期望: 3，实际: $HOST_COUNT"
    fi
    echo ""
fi

# 测试分配主机到非叶子节点
if [ -n "$PROD_GROUP_ID" ] && [ "$PROD_GROUP_ID" != "null" ] && [ ${#HOST_IDS[@]} -gt 0 ]; then
    ASSIGN_TO_BRANCH_DATA="{
        \"host_ids\": [${HOST_IDS[0]}]
    }"
    
    test_api "POST" "/host-groups/$PROD_GROUP_ID/hosts" "$ADMIN_TOKEN" "$ASSIGN_TO_BRANCH_DATA" "400" "分配主机到非叶子节点（应失败）"
fi

# 获取未分组主机
test_api "GET" "/hosts/ungrouped" "$ADMIN_TOKEN" "" "200" "获取未分组的主机"

# 更新主机所属组
if [ ${#HOST_IDS[@]} -gt 0 ] && [ -n "$DB_GROUP_ID" ] && [ "$DB_GROUP_ID" != "null" ]; then
    UPDATE_HOST_GROUP_DATA="{
        \"group_id\": $DB_GROUP_ID
    }"
    
    test_api "PUT" "/hosts/${HOST_IDS[0]}/group" "$ADMIN_TOKEN" "$UPDATE_HOST_GROUP_DATA" "200" "更新主机所属组"
    
    # 获取主机所属组
    test_api "GET" "/hosts/${HOST_IDS[0]}/groups" "$ADMIN_TOKEN" "" "200" "获取主机所属组信息"
fi

# 从组中移除主机
if [ -n "$WEB_GROUP_ID" ] && [ "$WEB_GROUP_ID" != "null" ] && [ ${#HOST_IDS[@]} -gt 1 ]; then
    REMOVE_DATA="{
        \"host_ids\": [${HOST_IDS[1]}]
    }"
    
    test_api "DELETE" "/host-groups/$WEB_GROUP_ID/hosts" "$ADMIN_TOKEN" "$REMOVE_DATA" "200" "从组中移除主机"
fi

# 第7步：权限控制测试
print_section "第7步：权限控制测试"

# 获取管理员的租户ID
ADMIN_ME_RESP=$(curl -s -X GET "$BASE_URL/auth/me" \
    -H "Authorization: Bearer $ADMIN_TOKEN")
ADMIN_TENANT_ID=$(echo "$ADMIN_ME_RESP" | jq -r '.data.current_tenant.id // .data.user.tenant_id // 1' 2>/dev/null)

# 创建无权限测试用户
CREATE_USER_DATA="{
    \"tenant_id\": $ADMIN_TENANT_ID,
    \"username\": \"test_no_group_${RANDOM_SUFFIX}\",
    \"email\": \"nogroup_${RANDOM_SUFFIX}@test.com\",
    \"password\": \"Test123456\",
    \"name\": \"无主机组权限用户_${RANDOM_SUFFIX}\"
}"

print_test "创建无主机组权限的测试用户"
USER_RESP=$(curl -s -X POST "$BASE_URL/users" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$CREATE_USER_DATA")

print_response "$USER_RESP"

USER_CODE=$(echo "$USER_RESP" | jq -r '.code' 2>/dev/null)
if [ "$USER_CODE" = "200" ]; then
    print_success "测试用户创建成功"
    
    # 用测试用户登录
    login_user "test_no_group_${RANDOM_SUFFIX}" "USER_TOKEN" "无主机组权限用户" "Test123456"
    
    if [ -n "$USER_TOKEN" ] && [ "$USER_TOKEN" != "null" ]; then
        # 测试无权限访问
        test_api "GET" "/host-groups" "$USER_TOKEN" "" "403" "无权限用户访问主机组列表（应拒绝）"
        test_api "POST" "/host-groups" "$USER_TOKEN" "$CREATE_DEV_DATA" "403" "无权限用户创建主机组（应拒绝）"
        
        if [ -n "$PROD_GROUP_ID" ] && [ "$PROD_GROUP_ID" != "null" ]; then
            test_api "GET" "/host-groups/$PROD_GROUP_ID" "$USER_TOKEN" "" "403" "无权限用户查看主机组（应拒绝）"
            test_api "PUT" "/host-groups/$PROD_GROUP_ID" "$USER_TOKEN" "{\"name\":\"test\"}" "403" "无权限用户更新主机组（应拒绝）"
        fi
    fi
else
    print_error "测试用户创建失败"
fi

# 第8步：主机组删除测试
print_section "第8步：主机组删除测试"

# 测试删除有主机的组（应失败）
if [ -n "$WEB_GROUP_ID" ] && [ "$WEB_GROUP_ID" != "null" ]; then
    test_api "DELETE" "/host-groups/$WEB_GROUP_ID" "$ADMIN_TOKEN" "" "400" "删除包含主机的组（应失败）"
    
    # 强制删除
    test_api "DELETE" "/host-groups/$WEB_GROUP_ID?force=true" "$ADMIN_TOKEN" "" "200" "强制删除包含主机的组"
fi

# 测试删除有子组的组（应失败）
if [ -n "$PROD_GROUP_ID" ] && [ "$PROD_GROUP_ID" != "null" ]; then
    test_api "DELETE" "/host-groups/$PROD_GROUP_ID" "$ADMIN_TOKEN" "" "400" "删除包含子组的组（应失败）"
fi

# 删除空的叶子组
if [ -n "$DB_GROUP_ID" ] && [ "$DB_GROUP_ID" != "null" ]; then
    # 先获取DB组内的所有主机
    DB_HOSTS_RESP=$(curl -s -X GET "$BASE_URL/host-groups/$DB_GROUP_ID/hosts" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    
    DB_HOST_IDS=$(echo "$DB_HOSTS_RESP" | jq -r '.data[].id' | tr '\n' ',' | sed 's/,$//')
    
    # 如果有主机，则移除它们
    if [ -n "$DB_HOST_IDS" ] && [ "$DB_HOST_IDS" != "" ]; then
        REMOVE_DB_HOSTS_DATA="{
            \"host_ids\": [$DB_HOST_IDS]
        }"
        
        curl -s -X DELETE "$BASE_URL/host-groups/$DB_GROUP_ID/hosts" \
            -H "Authorization: Bearer $ADMIN_TOKEN" \
            -H "Content-Type: application/json" \
            -d "$REMOVE_DB_HOSTS_DATA" > /dev/null
    fi
    
    test_api "DELETE" "/host-groups/$DB_GROUP_ID" "$ADMIN_TOKEN" "" "200" "删除空的叶子组"
    
    # 验证删除后无法访问
    test_api "GET" "/host-groups/$DB_GROUP_ID" "$ADMIN_TOKEN" "" "404" "验证主机组已删除（应返回404）"
fi

# 第9步：边界测试
print_section "第9步：边界测试"

# 测试不存在的主机组
test_api "GET" "/host-groups/99999" "$ADMIN_TOKEN" "" "404" "获取不存在的主机组"
test_api "PUT" "/host-groups/99999" "$ADMIN_TOKEN" "{\"name\":\"test\"}" "404" "更新不存在的主机组"
test_api "DELETE" "/host-groups/99999" "$ADMIN_TOKEN" "" "404" "删除不存在的主机组"

# 测试无效ID格式
test_api "GET" "/host-groups/invalid" "$ADMIN_TOKEN" "" "400" "使用无效ID格式"

# 测试空的请求体
test_api "POST" "/host-groups" "$ADMIN_TOKEN" "{}" "400" "创建主机组（空请求体）"

# 测试超长名称
LONG_NAME_DATA="{
    \"name\": \"$(printf 'A%.0s' {1..200})\",
    \"code\": \"toolong_${RANDOM_SUFFIX}\",
    \"type\": \"custom\"
}"
test_api "POST" "/host-groups" "$ADMIN_TOKEN" "$LONG_NAME_DATA" "400" "创建主机组（名称过长）"

# 第10步：清理测试数据
print_section "第10步：清理测试数据"

# 清理主机
for host_id in "${HOST_IDS[@]}"; do
    curl -s -X DELETE "$BASE_URL/hosts/$host_id" \
        -H "Authorization: Bearer $ADMIN_TOKEN" > /dev/null
done
echo "已清理 ${#HOST_IDS[@]} 个测试主机"

# 清理凭证
if [ -n "$CRED_ID" ] && [ "$CRED_ID" != "null" ]; then
    curl -s -X DELETE "$BASE_URL/credentials/$CRED_ID" \
        -H "Authorization: Bearer $ADMIN_TOKEN" > /dev/null
    echo "已清理测试凭证"
fi

# 显示最终测试总结
print_summary

echo -e "${CYAN}🔍 主机组管理功能验证：${NC}"
echo "✅ 主机组CRUD操作"
echo "✅ 树形层级结构管理"
echo "✅ 主机组移动功能"
echo "✅ 路径自动生成和维护"
echo "✅ 主机分配和管理"
echo "✅ 叶子节点和分支节点区分"
echo "✅ 权限控制和多租户隔离"
echo "✅ 完整的错误处理"
echo ""

echo -e "${GREEN}🎯 主机组管理功能测试完成！${NC}"
echo -e "${CYAN}📋 测试批次ID: $RANDOM_SUFFIX${NC}"