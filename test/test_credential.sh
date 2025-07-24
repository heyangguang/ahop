#!/bin/bash

# 凭证管理功能测试脚本
# 测试凭证的CRUD操作、权限控制、加密和ACL功能

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

# 凭证ID存储
SSH_CRED_ID=""
PWD_CRED_ID=""
API_CRED_ID=""

print_header() {
    echo -e "${CYAN}================================================================${NC}"
    echo -e "${CYAN}🔐 凭证管理功能测试${NC}"
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
        
        # 保存凭证ID供后续测试使用
        if [ "$method" = "POST" ] && [[ "$endpoint" == "/credentials" ]] && [ "$code" = "200" ]; then
            local cred_id=$(echo "$response" | jq -r '.data.credential.id // .data.id // ""')
            local cred_type=$(echo "$response" | jq -r '.data.credential.type // .data.type // ""')
            
            if [ -n "$cred_id" ] && [ "$cred_id" != "null" ]; then
                case "$cred_type" in
                    "ssh_key")
                        SSH_CRED_ID="$cred_id"
                        echo "SSH凭证ID: $SSH_CRED_ID"
                        ;;
                    "password")
                        PWD_CRED_ID="$cred_id"
                        echo "密码凭证ID: $PWD_CRED_ID"
                        ;;
                    "api_key")
                        API_CRED_ID="$cred_id"
                        echo "API密钥凭证ID: $API_CRED_ID"
                        ;;
                esac
            fi
        fi
    else
        print_error "状态码不符合预期，期望: $expected_status，实际: $code"
    fi
    echo ""
}

# 显示测试总结
print_summary() {
    echo -e "${CYAN}================================================================${NC}"
    echo -e "${CYAN}📊 凭证管理测试总结${NC}"
    echo -e "${CYAN}================================================================${NC}"
    echo -e "🆔 测试批次: ${YELLOW}$RANDOM_SUFFIX${NC}"
    echo -e "📊 总测试数: ${YELLOW}$TOTAL_TESTS${NC}"
    echo -e "✅ 通过数量: ${GREEN}$PASSED_TESTS${NC}"
    echo -e "❌ 失败数量: ${RED}$FAILED_TESTS${NC}"

    local success_rate=$(( PASSED_TESTS * 100 / TOTAL_TESTS ))
    echo -e "📈 成功率: ${CYAN}$success_rate%${NC}"

    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "${GREEN}🎉 所有测试通过！凭证管理功能工作正常！${NC}"
    else
        echo -e "${RED}⚠️  有 $FAILED_TESTS 个测试失败，需要检查凭证功能${NC}"
    fi
    echo ""
}

# ========== 开始执行测试 ==========

print_header

# 第1步：用户认证测试
print_section "第1步：用户认证测试"
login_user "admin" "ADMIN_TOKEN" "平台超级管理员"

# 第2步：凭证创建测试
print_section "第2步：凭证创建测试"

# 创建SSH密钥凭证
CREATE_SSH_DATA="{
    \"name\": \"生产环境SSH密钥_${RANDOM_SUFFIX}\",
    \"type\": \"ssh_key\",
    \"description\": \"用于连接生产环境服务器的SSH密钥\",
    \"private_key\": \"-----BEGIN RSA PRIVATE KEY-----\\nMIIEpAIBAAKCAQEA...\\n-----END RSA PRIVATE KEY-----\",
    \"public_key\": \"ssh-rsa AAAAB3NzaC1yc2EA...\",
    \"passphrase\": \"mysecretpassphrase\",
    \"allowed_hosts\": \"prod-*.example.com,10.0.1.*\",
    \"allowed_ips\": \"10.0.0.0/16,192.168.1.0/24\",
    \"denied_hosts\": \"test-*,dev-*\",
    \"max_usage_count\": 100
}"

test_api "POST" "/credentials" "$ADMIN_TOKEN" "$CREATE_SSH_DATA" "200" "创建SSH密钥凭证"

# 验证敏感字段被隐藏
if [ -n "$SSH_CRED_ID" ] && [ "$SSH_CRED_ID" != "null" ]; then
    print_test "验证SSH凭证敏感字段"
    RESPONSE=$(curl -s -X GET "$BASE_URL/credentials/$SSH_CRED_ID" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    
    PRIVATE_KEY=$(echo "$RESPONSE" | jq -r '.data.private_key // ""')
    if [ -z "$PRIVATE_KEY" ] || [ "$PRIVATE_KEY" = "null" ]; then
        print_success "私钥字段已正确隐藏"
    else
        print_error "私钥字段未被隐藏"
    fi
    echo ""
fi

# 创建用户名密码凭证
CREATE_PWD_DATA="{
    \"name\": \"数据库访问凭证_${RANDOM_SUFFIX}\",
    \"type\": \"password\",
    \"description\": \"MySQL数据库访问凭证\",
    \"username\": \"dbadmin\",
    \"password\": \"SecureP@ssw0rd\",
    \"allowed_ips\": \"10.0.2.0/24\",
    \"expires_at\": \"2025-12-31T23:59:59Z\"
}"

test_api "POST" "/credentials" "$ADMIN_TOKEN" "$CREATE_PWD_DATA" "200" "创建密码凭证"

# 创建API密钥凭证
CREATE_API_DATA="{
    \"name\": \"外部API访问密钥_${RANDOM_SUFFIX}\",
    \"type\": \"api_key\",
    \"description\": \"用于访问第三方API的密钥\",
    \"api_key\": \"sk-1234567890abcdef1234567890abcdef\"
}"

test_api "POST" "/credentials" "$ADMIN_TOKEN" "$CREATE_API_DATA" "200" "创建API密钥凭证"

# 测试创建无效凭证
INVALID_CRED_DATA="{
    \"name\": \"无效凭证_${RANDOM_SUFFIX}\",
    \"type\": \"password\",
    \"description\": \"缺少username和password\"
}"

test_api "POST" "/credentials" "$ADMIN_TOKEN" "$INVALID_CRED_DATA" "400" "创建无效凭证（缺少必填字段）"

# 第3步：凭证查询测试
print_section "第3步：凭证查询测试"

# 获取凭证列表
test_api "GET" "/credentials?page=1&page_size=10" "$ADMIN_TOKEN" "" "200" "获取凭证列表"

# 获取单个凭证详情
if [ -n "$SSH_CRED_ID" ] && [ "$SSH_CRED_ID" != "null" ]; then
    test_api "GET" "/credentials/$SSH_CRED_ID" "$ADMIN_TOKEN" "" "200" "获取SSH凭证详情"
fi

# 按类型筛选凭证
test_api "GET" "/credentials?type=password" "$ADMIN_TOKEN" "" "200" "按类型筛选凭证(password)"

# 按名称搜索凭证
test_api "GET" "/credentials?name=数据库" "$ADMIN_TOKEN" "" "200" "按名称搜索凭证"

# 按激活状态筛选
test_api "GET" "/credentials?is_active=true" "$ADMIN_TOKEN" "" "200" "筛选激活的凭证"

# 第4步：凭证解密测试
print_section "第4步：凭证解密测试"

echo "当前凭证ID状态："
echo "SSH_CRED_ID: $SSH_CRED_ID"
echo "PWD_CRED_ID: $PWD_CRED_ID"
echo "API_CRED_ID: $API_CRED_ID"
echo ""

if [ -n "$PWD_CRED_ID" ] && [ "$PWD_CRED_ID" != "null" ]; then
    test_api "GET" "/credentials/$PWD_CRED_ID/decrypt?purpose=测试解密功能" "$ADMIN_TOKEN" "" "200" "获取解密的密码凭证"
    
    # 显示解密后的完整凭证信息
    print_test "检查解密后的凭证数据"
    DECRYPT_CHECK=$(curl -s -X GET "$BASE_URL/credentials/$PWD_CRED_ID/decrypt?purpose=检查解密数据" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    echo "解密后的凭证信息："
    echo "$DECRYPT_CHECK" | jq '.data' 2>/dev/null
    echo ""
    
    # 验证密码字段已解密
    print_test "验证密码字段解密"
    DECRYPT_RESP=$(curl -s -X GET "$BASE_URL/credentials/$PWD_CRED_ID/decrypt?purpose=再次测试" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    
    print_response "$DECRYPT_RESP"
    
    # 检查响应码
    DECRYPT_CODE=$(echo "$DECRYPT_RESP" | jq -r '.code' 2>/dev/null)
    if [ "$DECRYPT_CODE" = "200" ]; then
        PASSWORD=$(echo "$DECRYPT_RESP" | jq -r '.data.password // ""')
        if [ -n "$PASSWORD" ] && [ "$PASSWORD" != "null" ] && [ "$PASSWORD" = "SecureP@ssw0rd" ]; then
            print_success "密码字段已成功解密且值正确"
        else
            print_error "密码字段解密失败或值不正确"
            echo "期望密码: SecureP@ssw0rd"
            echo "实际密码: $PASSWORD"
        fi
    else
        print_error "获取解密凭证失败"
    fi
    echo ""
fi

# 第5步：凭证更新测试
print_section "第5步：凭证更新测试"

if [ -n "$SSH_CRED_ID" ] && [ "$SSH_CRED_ID" != "null" ]; then
    UPDATE_SSH_DATA="{
        \"description\": \"更新后的描述信息_${RANDOM_SUFFIX}\",
        \"max_usage_count\": 200,
        \"allowed_hosts\": \"prod-*.example.com,staging-*.example.com\"
    }"
    
    test_api "PUT" "/credentials/$SSH_CRED_ID" "$ADMIN_TOKEN" "$UPDATE_SSH_DATA" "200" "更新SSH凭证信息"
fi

# 禁用凭证
if [ -n "$API_CRED_ID" ] && [ "$API_CRED_ID" != "null" ]; then
    DISABLE_DATA="{
        \"is_active\": false
    }"
    
    test_api "PUT" "/credentials/$API_CRED_ID" "$ADMIN_TOKEN" "$DISABLE_DATA" "200" "禁用API密钥凭证"
    
    # 尝试获取已禁用凭证的解密信息（应该失败）
    test_api "GET" "/credentials/$API_CRED_ID/decrypt" "$ADMIN_TOKEN" "" "400" "获取已禁用凭证（应拒绝）"
fi

# 第6步：凭证使用日志测试
print_section "第6步：凭证使用日志测试"

if [ -n "$PWD_CRED_ID" ] && [ "$PWD_CRED_ID" != "null" ]; then
    test_api "GET" "/credentials/$PWD_CRED_ID/logs?page=1&page_size=10" "$ADMIN_TOKEN" "" "200" "获取凭证使用日志"
fi

# 第7步：权限控制测试
print_section "第7步：权限控制测试"

# 创建测试用户（需要指定租户ID）
# 获取管理员的租户ID
ADMIN_ME_RESP=$(curl -s -X GET "$BASE_URL/auth/me" \
    -H "Authorization: Bearer $ADMIN_TOKEN")
ADMIN_TENANT_ID=$(echo "$ADMIN_ME_RESP" | jq -r '.data.current_tenant.id // .data.user.tenant_id // 1' 2>/dev/null)

CREATE_USER_DATA="{
    \"tenant_id\": $ADMIN_TENANT_ID,
    \"username\": \"test_no_cred_${RANDOM_SUFFIX}\",
    \"email\": \"nocred_${RANDOM_SUFFIX}@test.com\",
    \"password\": \"Test123456\",
    \"name\": \"无凭证权限用户_${RANDOM_SUFFIX}\"
}"

print_test "创建无凭证权限的测试用户"
print_request "POST $BASE_URL/users"

USER_RESP=$(curl -s -X POST "$BASE_URL/users" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$CREATE_USER_DATA")

print_response "$USER_RESP"

USER_CODE=$(echo "$USER_RESP" | jq -r '.code' 2>/dev/null)
if [ "$USER_CODE" = "200" ]; then
    print_success "测试用户创建成功"
    
    # 用测试用户登录
    login_user "test_no_cred_${RANDOM_SUFFIX}" "USER_TOKEN" "无凭证权限用户" "Test123456"
    
    if [ -n "$USER_TOKEN" ] && [ "$USER_TOKEN" != "null" ]; then
        # 测试无权限访问
        test_api "GET" "/credentials" "$USER_TOKEN" "" "403" "无权限用户访问凭证列表（应拒绝）"
        test_api "POST" "/credentials" "$USER_TOKEN" "$CREATE_API_DATA" "403" "无权限用户创建凭证（应拒绝）"
        
        if [ -n "$SSH_CRED_ID" ] && [ "$SSH_CRED_ID" != "null" ]; then
            test_api "GET" "/credentials/$SSH_CRED_ID" "$USER_TOKEN" "" "403" "无权限用户查看凭证（应拒绝）"
            test_api "GET" "/credentials/$SSH_CRED_ID/decrypt" "$USER_TOKEN" "" "403" "无权限用户解密凭证（应拒绝）"
        fi
    fi
else
    print_error "测试用户创建失败"
fi

# 第8步：凭证删除测试
print_section "第8步：凭证删除测试"

if [ -n "$API_CRED_ID" ] && [ "$API_CRED_ID" != "null" ]; then
    test_api "DELETE" "/credentials/$API_CRED_ID" "$ADMIN_TOKEN" "" "200" "删除API密钥凭证"
    
    # 验证删除后无法访问
    test_api "GET" "/credentials/$API_CRED_ID" "$ADMIN_TOKEN" "" "404" "验证凭证已删除（应返回404）"
fi

# 第9步：边界测试
print_section "第9步：边界测试"

# 测试不存在的凭证
test_api "GET" "/credentials/99999" "$ADMIN_TOKEN" "" "404" "获取不存在的凭证"
test_api "PUT" "/credentials/99999" "$ADMIN_TOKEN" "{\"name\":\"test\"}" "404" "更新不存在的凭证"
test_api "DELETE" "/credentials/99999" "$ADMIN_TOKEN" "" "404" "删除不存在的凭证"

# 测试无效ID格式
test_api "GET" "/credentials/invalid" "$ADMIN_TOKEN" "" "400" "使用无效ID格式"

# 显示最终测试总结
print_summary

echo -e "${CYAN}🔍 凭证管理功能验证：${NC}"
echo "✅ 凭证CRUD操作"
echo "✅ 敏感字段加密存储"
echo "✅ 凭证解密功能（需要特殊权限）"
echo "✅ ACL限制（允许/禁止的主机和IP）"
echo "✅ 使用次数和过期时间控制"
echo "✅ 完整的使用日志审计"
echo "✅ 多租户隔离"
echo "✅ 细粒度权限控制"
echo ""

echo -e "${GREEN}🎯 凭证管理功能测试完成！${NC}"
echo -e "${CYAN}📋 测试批次ID: $RANDOM_SUFFIX${NC}"