#!/bin/bash

# 创建额外测试数据脚本
# 在应用启动后创建额外的测试数据

BASE_URL="http://localhost:8080/api/v1"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}📦 创建额外测试数据${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 等待服务启动
echo -e "${YELLOW}等待服务启动...${NC}"
max_attempts=30
attempt=0

while ! curl -s "$BASE_URL/health" > /dev/null; do
    if [ $attempt -ge $max_attempts ]; then
        echo -e "${RED}✗ 服务未能在 30 秒内启动${NC}"
        exit 1
    fi
    attempt=$((attempt + 1))
    echo -n "."
    sleep 1
done

echo ""
echo -e "${GREEN}✓ 服务已启动${NC}"
echo ""

# 1. 使用默认管理员登录
echo -e "${YELLOW}▶ 步骤 1: 使用默认管理员登录${NC}"
LOGIN_RESP=$(curl -s -X POST "$BASE_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d '{
        "username": "admin",
        "password": "Admin@123"
    }')

TOKEN=$(echo $LOGIN_RESP | jq -r '.data.token // .token')
if [ "$TOKEN" != "null" ] && [ -n "$TOKEN" ]; then
    echo -e "${GREEN}✓ 登录成功${NC}"
else
    echo -e "${RED}✗ 登录失败，请确保数据库已初始化${NC}"
    echo $LOGIN_RESP | jq '.'
    exit 1
fi

# 2. 创建测试租户
echo -e "${YELLOW}▶ 步骤 2: 创建测试租户${NC}"
TEST_TENANT_RESP=$(curl -s -X POST "$BASE_URL/tenants" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "测试租户",
        "code": "test"
    }')

TEST_TENANT_ID=$(echo $TEST_TENANT_RESP | jq -r '.data.id // .id')
if [ "$TEST_TENANT_ID" != "null" ] && [ -n "$TEST_TENANT_ID" ]; then
    echo -e "${GREEN}✓ 测试租户创建成功 (ID: $TEST_TENANT_ID)${NC}"
else
    echo -e "${YELLOW}! 测试租户可能已存在${NC}"
fi

# 3. 创建租户管理员角色
echo -e "${YELLOW}▶ 步骤 3: 创建租户管理员角色${NC}"
TENANT_ADMIN_ROLE_RESP=$(curl -s -X POST "$BASE_URL/roles" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "tenant_id": 1,
        "name": "租户管理员",
        "code": "tenant_admin",
        "description": "租户内最高权限管理员"
    }')

TENANT_ADMIN_ROLE_ID=$(echo $TENANT_ADMIN_ROLE_RESP | jq -r '.data.id // .id')
if [ "$TENANT_ADMIN_ROLE_ID" != "null" ] && [ -n "$TENANT_ADMIN_ROLE_ID" ]; then
    echo -e "${GREEN}✓ 租户管理员角色创建成功${NC}"
    
    # 分配常用权限
    PERMS_RESP=$(curl -s -X GET "$BASE_URL/permissions" -H "Authorization: Bearer $TOKEN")
    TENANT_PERMS=$(echo $PERMS_RESP | jq -r '.data[] | select(.code | contains("user") or contains("role")) | .id' | tr '\n' ',' | sed 's/,$//')
    
    if [ -n "$TENANT_PERMS" ]; then
        curl -s -X POST "$BASE_URL/roles/$TENANT_ADMIN_ROLE_ID/permissions" \
            -H "Authorization: Bearer $TOKEN" \
            -H "Content-Type: application/json" \
            -d "{\"permission_ids\": [$TENANT_PERMS]}" > /dev/null
        echo -e "${GREEN}✓ 已分配权限给租户管理员角色${NC}"
    fi
fi

# 4. 创建普通用户角色
echo -e "${YELLOW}▶ 步骤 4: 创建普通用户角色${NC}"
USER_ROLE_RESP=$(curl -s -X POST "$BASE_URL/roles" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "tenant_id": 1,
        "name": "普通用户",
        "code": "user",
        "description": "普通用户角色"
    }')

USER_ROLE_ID=$(echo $USER_ROLE_RESP | jq -r '.data.id // .id')
if [ "$USER_ROLE_ID" != "null" ] && [ -n "$USER_ROLE_ID" ]; then
    echo -e "${GREEN}✓ 普通用户角色创建成功${NC}"
fi

# 5. 创建测试用户
echo -e "${YELLOW}▶ 步骤 5: 创建测试用户${NC}"

# 创建租户管理员用户
TENANT_ADMIN_RESP=$(curl -s -X POST "$BASE_URL/users" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "tenant_id": 1,
        "username": "tenant_admin",
        "email": "tenant_admin@example.com",
        "password": "Test@123",
        "name": "租户管理员",
        "is_tenant_admin": true
    }')

if [ "$(echo $TENANT_ADMIN_RESP | jq -r '.data.id // .id')" != "null" ]; then
    echo -e "${GREEN}✓ 租户管理员用户创建成功${NC}"
    echo "  用户名: tenant_admin"
    echo "  密码: Test@123"
fi

# 创建普通测试用户
TEST_USER_RESP=$(curl -s -X POST "$BASE_URL/users" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "tenant_id": 1,
        "username": "testuser",
        "email": "testuser@example.com",
        "password": "Test@123",
        "name": "测试用户"
    }')

if [ "$(echo $TEST_USER_RESP | jq -r '.data.id // .id')" != "null" ]; then
    echo -e "${GREEN}✓ 普通测试用户创建成功${NC}"
    echo "  用户名: testuser"
    echo "  密码: Test@123"
fi

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}✓ 初始化完成！${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "默认管理员账号："
echo "  用户名: admin"
echo "  密码: Admin@123"
echo ""
echo "可以使用以下命令测试："
echo -e "${BLUE}bash test/test_jwt_perm.sh${NC}"
echo -e "${BLUE}bash test/test_tenant_switch.sh${NC}"