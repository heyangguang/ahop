#!/bin/bash

# 标签系统测试脚本
# 测试标签的 CRUD 操作以及凭证标签管理

BASE_URL="http://localhost:8080/api/v1"

# 生成随机后缀避免冲突
SUFFIX=$(date +%s)

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 测试计数器
TOTAL_TESTS=0
PASSED_TESTS=0

# 存储创建的资源ID（用于清理）
CREATED_TAG_IDS=()
CREATED_CREDENTIAL_ID=""

print_test() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -e "${YELLOW}📋 测试 $TOTAL_TESTS: $1${NC}"
}

print_success() {
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

# 清理函数
cleanup() {
    echo -e "\n${YELLOW}🧹 清理测试数据...${NC}"
    
    # 删除创建的凭证
    if [ -n "$CREATED_CREDENTIAL_ID" ]; then
        curl -s -X DELETE "$BASE_URL/credentials/$CREATED_CREDENTIAL_ID" \
            -H "Authorization: Bearer $TOKEN" > /dev/null
        echo -e "${GREEN}删除凭证 ID: $CREATED_CREDENTIAL_ID${NC}"
    fi
    
    # 删除创建的标签
    for tag_id in "${CREATED_TAG_IDS[@]}"; do
        if [ -n "$tag_id" ]; then
            curl -s -X DELETE "$BASE_URL/tags/$tag_id" \
                -H "Authorization: Bearer $TOKEN" > /dev/null
            echo -e "${GREEN}删除标签 ID: $tag_id${NC}"
        fi
    done
}

# 注册清理函数
trap cleanup EXIT

# 检查 jq 是否安装
if ! command -v jq &> /dev/null; then
    echo -e "${RED}错误: 需要安装 jq 工具${NC}"
    echo "请运行: sudo apt-get install jq 或 brew install jq"
    exit 1
fi

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}🏷️  标签系统测试${NC}"
echo -e "${BLUE}========================================${NC}"

# 1. 登录获取 token
print_test "管理员登录"
LOGIN_RESP=$(curl -s -X POST "$BASE_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d '{
        "username": "admin",
        "password": "Admin@123"
    }')

TOKEN=$(echo "$LOGIN_RESP" | jq -r '.data.token // ""')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    print_error "登录失败"
    echo "$LOGIN_RESP" | jq '.'
    exit 1
fi
print_success "登录成功"

# 获取用户信息
ME_RESP=$(curl -s -X GET "$BASE_URL/auth/me" \
    -H "Authorization: Bearer $TOKEN")
TENANT_ID=$(echo "$ME_RESP" | jq -r '.data.current_tenant.id // 1')

echo -e "\n${BLUE}=== 标签 CRUD 测试 ===${NC}"

# 清理可能存在的测试标签
cleanup_existing_tags() {
    echo -e "${YELLOW}清理已存在的测试标签...${NC}"
    
    # 获取所有标签
    TAGS_RESP=$(curl -s -X GET "$BASE_URL/tags" \
        -H "Authorization: Bearer $TOKEN")
    
    # 要清理的标签列表
    local test_tags=(
        "环境:生产"
        "地域:北京" 
        "app:web-server"
        "服务器:上海"
        "db:master"
        "cluster:prod"
    )
    
    # 先获取所有凭证，删除使用测试标签的凭证
    echo -e "${YELLOW}清理使用测试标签的凭证...${NC}"
    CREDS_RESP=$(curl -s -X GET "$BASE_URL/credentials" \
        -H "Authorization: Bearer $TOKEN")
    
    # 提取所有凭证ID
    local cred_ids=$(echo "$CREDS_RESP" | jq -r '.data[]?.id // empty')
    
    # 删除所有名称包含test的凭证（测试凭证）
    for cred_id in $cred_ids; do
        cred_name=$(echo "$CREDS_RESP" | jq -r --arg id "$cred_id" '.data[] | select(.id == ($id | tonumber)) | .name // ""')
        if [[ "$cred_name" == *"test"* ]] || [[ "$cred_name" == *"标签"* ]]; then
            curl -s -X DELETE "$BASE_URL/credentials/$cred_id" \
                -H "Authorization: Bearer $TOKEN" > /dev/null
            echo -e "${GREEN}删除测试凭证: $cred_name (ID: $cred_id)${NC}"
        fi
    done
    
    # 遍历每个测试标签
    for tag_pair in "${test_tags[@]}"; do
        IFS=':' read -r key value <<< "$tag_pair"
        
        # 查找匹配的标签
        tag_id=$(echo "$TAGS_RESP" | jq -r --arg k "$key" --arg v "$value" '.data[]? | select(.key == $k and .value == $v) | .id // empty')
        
        if [ -n "$tag_id" ]; then
            # 尝试删除标签
            DELETE_RESP=$(curl -s -X DELETE "$BASE_URL/tags/$tag_id" \
                -H "Authorization: Bearer $TOKEN")
            
            # 检查删除结果
            DELETE_CODE=$(echo "$DELETE_RESP" | jq -r '.code // ""')
            if [ "$DELETE_CODE" = "200" ]; then
                echo -e "${GREEN}删除已存在的标签: $key:$value (ID: $tag_id)${NC}"
            else
                # 如果删除失败，可能是因为有其他凭证在使用，忽略错误
                echo -e "${YELLOW}标签 $key:$value 可能被其他资源使用，跳过删除${NC}"
            fi
        fi
    done
}

# 执行清理
cleanup_existing_tags

# 2. 创建标签 - 环境标签
print_test "创建标签 - 环境:生产"

# 先尝试获取已存在的标签
EXISTING_TAG=$(curl -s -X GET "$BASE_URL/tags" \
    -H "Authorization: Bearer $TOKEN" | \
    jq -r '.data[] | select(.key == "环境" and .value == "生产") | .id // empty')

if [ -n "$EXISTING_TAG" ]; then
    TAG_ID_1=$EXISTING_TAG
    print_success "使用已存在的标签，ID: $TAG_ID_1"
else
    CREATE_TAG_RESP=$(curl -s -X POST "$BASE_URL/tags" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{
            "key": "环境",
            "value": "生产",
            "color": "#FF5722"
        }')

    TAG_CODE=$(echo "$CREATE_TAG_RESP" | jq -r '.code // ""')
    if [ "$TAG_CODE" = "200" ]; then
        TAG_ID_1=$(echo "$CREATE_TAG_RESP" | jq -r '.data.id')
        CREATED_TAG_IDS+=("$TAG_ID_1")
        print_success "创建成功，ID: $TAG_ID_1"
    else
        print_error "创建失败"
        echo "$CREATE_TAG_RESP" | jq '.'
    fi
fi

# 3. 创建标签 - 地域标签
print_test "创建标签 - 地域:北京"
CREATE_TAG_RESP=$(curl -s -X POST "$BASE_URL/tags" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "key": "地域",
        "value": "北京",
        "color": "#2196F3"
    }')

TAG_CODE=$(echo "$CREATE_TAG_RESP" | jq -r '.code // ""')
if [ "$TAG_CODE" = "200" ]; then
    TAG_ID_2=$(echo "$CREATE_TAG_RESP" | jq -r '.data.id')
    CREATED_TAG_IDS+=("$TAG_ID_2")
    print_success "创建成功，ID: $TAG_ID_2"
else
    print_error "创建失败"
    echo "$CREATE_TAG_RESP" | jq '.'
fi

# 4. 创建标签 - 应用标签
print_test "创建标签 - app:web-server"
CREATE_TAG_RESP=$(curl -s -X POST "$BASE_URL/tags" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "key": "app",
        "value": "web-server",
        "color": "#4CAF50"
    }')

TAG_CODE=$(echo "$CREATE_TAG_RESP" | jq -r '.code // ""')
if [ "$TAG_CODE" = "200" ]; then
    TAG_ID_3=$(echo "$CREATE_TAG_RESP" | jq -r '.data.id')
    CREATED_TAG_IDS+=("$TAG_ID_3")
    print_success "创建成功，ID: $TAG_ID_3"
else
    print_error "创建失败"
    echo "$CREATE_TAG_RESP" | jq '.'
fi

# 5. 测试重复创建
print_test "创建重复标签（应该失败）"
CREATE_TAG_RESP=$(curl -s -X POST "$BASE_URL/tags" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "key": "环境",
        "value": "生产",
        "color": "#FF5722"
    }')

TAG_CODE=$(echo "$CREATE_TAG_RESP" | jq -r '.code // ""')
if [ "$TAG_CODE" != "200" ]; then
    print_success "正确拒绝了重复标签"
else
    print_error "错误地允许了重复标签"
fi

# 6. 获取标签列表
print_test "获取标签列表"
LIST_RESP=$(curl -s -X GET "$BASE_URL/tags" \
    -H "Authorization: Bearer $TOKEN")

LIST_CODE=$(echo "$LIST_RESP" | jq -r '.code // ""')
if [ "$LIST_CODE" = "200" ]; then
    TAG_COUNT=$(echo "$LIST_RESP" | jq '.data | length')
    print_success "获取成功，标签数量: $TAG_COUNT"
    echo "$LIST_RESP" | jq '.data[] | {id: .id, key: .key, value: .value, color: .color}'
else
    print_error "获取失败"
    echo "$LIST_RESP" | jq '.'
fi

# 7. 按 key 过滤标签
print_test "按 key 过滤标签（key=环境）"
FILTER_RESP=$(curl -s -X GET "$BASE_URL/tags?key=环境" \
    -H "Authorization: Bearer $TOKEN")

FILTER_CODE=$(echo "$FILTER_RESP" | jq -r '.code // ""')
if [ "$FILTER_CODE" = "200" ]; then
    FILTER_COUNT=$(echo "$FILTER_RESP" | jq '.data | length')
    print_success "过滤成功，找到 $FILTER_COUNT 个标签"
else
    print_error "过滤失败"
fi

# 8. 获取分组标签
print_test "获取分组标签"
GROUPED_RESP=$(curl -s -X GET "$BASE_URL/tags/grouped" \
    -H "Authorization: Bearer $TOKEN")

GROUPED_CODE=$(echo "$GROUPED_RESP" | jq -r '.code // ""')
if [ "$GROUPED_CODE" = "200" ]; then
    print_success "获取分组成功"
    echo "$GROUPED_RESP" | jq '.data'
else
    print_error "获取分组失败"
    echo "$GROUPED_RESP" | jq '.'
fi

# 9. 获取标签详情
if [ -n "$TAG_ID_1" ]; then
    print_test "获取标签详情"
    DETAIL_RESP=$(curl -s -X GET "$BASE_URL/tags/$TAG_ID_1" \
        -H "Authorization: Bearer $TOKEN")
    
    DETAIL_CODE=$(echo "$DETAIL_RESP" | jq -r '.code // ""')
    if [ "$DETAIL_CODE" = "200" ]; then
        print_success "获取详情成功"
        echo "$DETAIL_RESP" | jq '.data'
    else
        print_error "获取详情失败"
    fi
fi

# 10. 更新标签颜色
if [ -n "$TAG_ID_1" ]; then
    print_test "更新标签颜色"
    UPDATE_RESP=$(curl -s -X PUT "$BASE_URL/tags/$TAG_ID_1" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{
            "color": "#9C27B0"
        }')
    
    UPDATE_CODE=$(echo "$UPDATE_RESP" | jq -r '.code // ""')
    if [ "$UPDATE_CODE" = "200" ]; then
        NEW_COLOR=$(echo "$UPDATE_RESP" | jq -r '.data.color')
        print_success "更新成功，新颜色: $NEW_COLOR"
    else
        print_error "更新失败"
    fi
fi

echo -e "\n${BLUE}=== 凭证标签管理测试 ===${NC}"

# 11. 创建测试凭证
print_test "创建测试凭证"
CREATE_CRED_RESP=$(curl -s -X POST "$BASE_URL/credentials" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "测试服务器凭证_'$SUFFIX'",
        "type": "password",
        "description": "用于标签测试的凭证",
        "username": "testuser",
        "password": "Test@123456"
    }')

CRED_CODE=$(echo "$CREATE_CRED_RESP" | jq -r '.code // ""')
if [ "$CRED_CODE" = "200" ]; then
    CREATED_CREDENTIAL_ID=$(echo "$CREATE_CRED_RESP" | jq -r '.data.id')
    print_success "凭证创建成功，ID: $CREATED_CREDENTIAL_ID"
else
    print_error "凭证创建失败"
    echo "$CREATE_CRED_RESP" | jq '.'
fi

# 12. 给凭证打标签
if [ -n "$CREATED_CREDENTIAL_ID" ] && [ -n "$TAG_ID_1" ] && [ -n "$TAG_ID_2" ]; then
    print_test "给凭证打标签"
    UPDATE_TAGS_RESP=$(curl -s -X PUT "$BASE_URL/credentials/$CREATED_CREDENTIAL_ID/tags" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{
            "tag_ids": ['$TAG_ID_1', '$TAG_ID_2', '$TAG_ID_3']
        }')
    
    UPDATE_TAGS_CODE=$(echo "$UPDATE_TAGS_RESP" | jq -r '.code // ""')
    if [ "$UPDATE_TAGS_CODE" = "200" ]; then
        TAG_COUNT=$(echo "$UPDATE_TAGS_RESP" | jq '.data.tags | length')
        print_success "标签更新成功，当前标签数: $TAG_COUNT"
        echo "$UPDATE_TAGS_RESP" | jq '.data.tags'
    else
        print_error "标签更新失败"
        echo "$UPDATE_TAGS_RESP" | jq '.'
    fi
fi

# 13. 获取凭证标签
if [ -n "$CREATED_CREDENTIAL_ID" ]; then
    print_test "获取凭证标签"
    GET_TAGS_RESP=$(curl -s -X GET "$BASE_URL/credentials/$CREATED_CREDENTIAL_ID/tags" \
        -H "Authorization: Bearer $TOKEN")
    
    GET_TAGS_CODE=$(echo "$GET_TAGS_RESP" | jq -r '.code // ""')
    if [ "$GET_TAGS_CODE" = "200" ]; then
        TAG_COUNT=$(echo "$GET_TAGS_RESP" | jq '.data | length')
        print_success "获取成功，标签数: $TAG_COUNT"
    else
        print_error "获取失败"
    fi
fi

# 14. 更新凭证标签（替换）
if [ -n "$CREATED_CREDENTIAL_ID" ] && [ -n "$TAG_ID_3" ]; then
    print_test "更新凭证标签（只保留一个）"
    UPDATE_TAGS_RESP=$(curl -s -X PUT "$BASE_URL/credentials/$CREATED_CREDENTIAL_ID/tags" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{
            "tag_ids": ['$TAG_ID_3']
        }')
    
    UPDATE_TAGS_CODE=$(echo "$UPDATE_TAGS_RESP" | jq -r '.code // ""')
    if [ "$UPDATE_TAGS_CODE" = "200" ]; then
        TAG_COUNT=$(echo "$UPDATE_TAGS_RESP" | jq '.data.tags | length')
        print_success "标签替换成功，当前标签数: $TAG_COUNT"
    else
        print_error "标签替换失败"
    fi
fi

# 15. 清空凭证标签
if [ -n "$CREATED_CREDENTIAL_ID" ]; then
    print_test "清空凭证标签"
    CLEAR_TAGS_RESP=$(curl -s -X PUT "$BASE_URL/credentials/$CREATED_CREDENTIAL_ID/tags" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{
            "tag_ids": []
        }')
    
    CLEAR_TAGS_CODE=$(echo "$CLEAR_TAGS_RESP" | jq -r '.code // ""')
    if [ "$CLEAR_TAGS_CODE" = "200" ]; then
        TAG_COUNT=$(echo "$CLEAR_TAGS_RESP" | jq '.data.tags | length')
        if [ "$TAG_COUNT" = "0" ]; then
            print_success "标签清空成功"
        else
            print_error "标签未能清空"
        fi
    else
        print_error "清空操作失败"
    fi
fi

# 16. 测试删除正在使用的标签
if [ -n "$CREATED_CREDENTIAL_ID" ] && [ -n "$TAG_ID_1" ]; then
    print_test "删除正在使用的标签（应该失败）"
    
    # 先给凭证打上标签
    curl -s -X PUT "$BASE_URL/credentials/$CREATED_CREDENTIAL_ID/tags" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"tag_ids": ['$TAG_ID_1']}' > /dev/null
    
    # 尝试删除标签
    DELETE_RESP=$(curl -s -X DELETE "$BASE_URL/tags/$TAG_ID_1" \
        -H "Authorization: Bearer $TOKEN")
    
    DELETE_CODE=$(echo "$DELETE_RESP" | jq -r '.code // ""')
    if [ "$DELETE_CODE" != "200" ]; then
        print_success "正确拒绝了删除正在使用的标签"
    else
        print_error "错误地允许删除正在使用的标签"
    fi
fi

# 17. 测试标签验证
print_test "创建无效标签 - 特殊字符"
INVALID_TAG_RESP=$(curl -s -X POST "$BASE_URL/tags" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "key": "env@#$",
        "value": "prod!@#",
        "color": "#FF5722"
    }')

INVALID_CODE=$(echo "$INVALID_TAG_RESP" | jq -r '.code // ""')
if [ "$INVALID_CODE" != "200" ]; then
    print_success "正确拒绝了无效字符"
else
    print_error "错误地允许了无效字符"
fi

# 18. 测试标签长度限制
print_test "创建超长标签键"
LONG_KEY=$(printf 'a%.0s' {1..60})
LONG_TAG_RESP=$(curl -s -X POST "$BASE_URL/tags" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "key": "'$LONG_KEY'",
        "value": "test",
        "color": "#FF5722"
    }')

LONG_CODE=$(echo "$LONG_TAG_RESP" | jq -r '.code // ""')
if [ "$LONG_CODE" != "200" ]; then
    print_success "正确拒绝了超长标签键"
else
    print_error "错误地允许了超长标签键"
fi

# 19. 获取凭证列表（验证标签预加载）
print_test "获取凭证列表（包含标签）"
CRED_LIST_RESP=$(curl -s -X GET "$BASE_URL/credentials" \
    -H "Authorization: Bearer $TOKEN")

CRED_LIST_CODE=$(echo "$CRED_LIST_RESP" | jq -r '.code // ""')
if [ "$CRED_LIST_CODE" = "200" ]; then
    # 查找我们创建的凭证
    CRED_WITH_TAGS=$(echo "$CRED_LIST_RESP" | jq '.data[] | select(.id == '$CREATED_CREDENTIAL_ID')')
    if [ -n "$CRED_WITH_TAGS" ]; then
        HAS_TAGS=$(echo "$CRED_WITH_TAGS" | jq 'has("tags")')
        if [ "$HAS_TAGS" = "true" ]; then
            print_success "凭证列表正确包含标签信息"
        else
            print_error "凭证列表缺少标签信息"
        fi
    fi
else
    print_error "获取凭证列表失败"
fi

echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}📊 测试总结${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "总测试数: ${YELLOW}$TOTAL_TESTS${NC}"
echo -e "通过数量: ${GREEN}$PASSED_TESTS${NC}"
echo -e "失败数量: ${RED}$((TOTAL_TESTS - PASSED_TESTS))${NC}"

if [ $PASSED_TESTS -eq $TOTAL_TESTS ]; then
    echo -e "${GREEN}🎉 所有测试通过！${NC}"
else
    echo -e "${RED}⚠️  有 $((TOTAL_TESTS - PASSED_TESTS)) 个测试失败${NC}"
fi