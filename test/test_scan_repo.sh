#!/bin/bash

# 测试扫描指定仓库

set -e

# 基本配置
BASE_URL="http://localhost:8080"
ADMIN_USER="admin"
ADMIN_PASS="Admin@123"
REPO_ID=$1

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

if [ -z "$REPO_ID" ]; then
    echo -e "${RED}错误: 请提供仓库ID${NC}"
    echo "用法: $0 <repo_id>"
    exit 1
fi

echo -e "${YELLOW}======== 测试扫描仓库 $REPO_ID ========${NC}"

# 1. 登录获取token
echo -e "\n${YELLOW}1. 登录系统${NC}"
LOGIN_RESP=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}")

TOKEN=$(echo $LOGIN_RESP | grep -o '"token":"[^"]*' | cut -d'"' -f4)
if [ -z "$TOKEN" ]; then
    echo -e "${RED}✗ 登录失败${NC}"
    exit 1
fi
echo -e "${GREEN}✓ 登录成功${NC}"

# 2. 扫描仓库
echo -e "\n${YELLOW}2. 扫描仓库 $REPO_ID${NC}"
SCAN_RESP=$(curl -s -X POST "$BASE_URL/api/v1/git-repositories/$REPO_ID/scan-templates" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json")

echo -e "${BLUE}扫描响应:${NC}"
echo $SCAN_RESP | jq . || echo $SCAN_RESP

# 3. 检查扫描结果
TEMPLATE_COUNT=$(echo $SCAN_RESP | jq '.data | length' 2>/dev/null || echo "0")
echo -e "\n${YELLOW}3. 扫描结果分析${NC}"
echo -e "发现模板数量: ${BLUE}$TEMPLATE_COUNT${NC}"

if [ "$TEMPLATE_COUNT" != "0" ]; then
    echo -e "\n${YELLOW}识别到的模板:${NC}"
    echo $SCAN_RESP | jq -r '.data[] | "\(.script_type) - \(.name) [\(.path)]"' 2>/dev/null || echo "解析失败"
    
    echo -e "\n${YELLOW}模板详情:${NC}"
    echo $SCAN_RESP | jq '.data' 2>/dev/null || echo "解析失败"
fi

# 4. 检查仓库目录结构
echo -e "\n${YELLOW}4. 仓库目录结构${NC}"
REPO_PATH="/data/ahop/repos/1/$REPO_ID"
if [ -d "$REPO_PATH" ]; then
    echo "仓库路径: $REPO_PATH"
    echo -e "\n${BLUE}文件列表:${NC}"
    find $REPO_PATH -type f \( -name "*.yml" -o -name "*.yaml" -o -name "*.sh" \) | grep -v ".git" | sort
else
    echo -e "${RED}仓库目录不存在: $REPO_PATH${NC}"
fi

echo -e "\n${GREEN}======== 扫描测试完成 ========${NC}"