#!/bin/bash

# 测试增强扫描功能

# 设置基础URL
BASE_URL="http://localhost:8080/api/v1"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}→ $1${NC}"
}

# 1. 登录获取令牌
print_info "登录系统..."
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d '{
        "username": "admin",
        "password": "Admin@123"
    }')

TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.token')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    print_error "登录失败"
    echo "$LOGIN_RESPONSE" | jq .
    exit 1
fi
print_success "登录成功"

# 2. 获取Git仓库列表
print_info "获取Git仓库列表..."
REPOS=$(curl -s -X GET "$BASE_URL/git-repositories" \
    -H "Authorization: Bearer $TOKEN")

REPO_ID=$(echo "$REPOS" | jq -r '.data[0].id')
REPO_NAME=$(echo "$REPOS" | jq -r '.data[0].name')

if [ -z "$REPO_ID" ] || [ "$REPO_ID" = "null" ]; then
    print_error "没有找到Git仓库"
    exit 1
fi

print_success "找到仓库: $REPO_NAME (ID: $REPO_ID)"

# 3. 扫描仓库模板（增强扫描）
print_info "执行增强扫描..."
SCAN_RESULT=$(curl -s -X POST "$BASE_URL/git-repositories/$REPO_ID/scan-templates" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json")

# 检查扫描结果
CODE=$(echo "$SCAN_RESULT" | jq -r '.code')
if [ "$CODE" != "200" ]; then
    print_error "扫描失败"
    echo "$SCAN_RESULT" | jq .
    exit 1
fi

# 4. 分析扫描结果
print_success "扫描成功，分析结果..."

# 获取survey数量
SURVEY_COUNT=$(echo "$SCAN_RESULT" | jq -r '.data.surveys | length')
print_info "找到 $SURVEY_COUNT 个Survey文件"

# 显示每个survey
if [ "$SURVEY_COUNT" -gt 0 ]; then
    echo "$SCAN_RESULT" | jq -r '.data.surveys[] | "  - \(.path) (\(.name))"'
fi

# 获取文件树统计
TOTAL_FILES=$(echo "$SCAN_RESULT" | jq -r '.data.stats.total_files')
TOTAL_DIRS=$(echo "$SCAN_RESULT" | jq -r '.data.stats.total_dirs')
PLAYBOOK_FILES=$(echo "$SCAN_RESULT" | jq -r '.data.stats.playbook_files')

print_info "文件统计:"
echo "  - 总文件数: $TOTAL_FILES"
echo "  - 总目录数: $TOTAL_DIRS"
echo "  - Playbook文件数: $PLAYBOOK_FILES"

# 5. 显示文件树结构（前10个文件）
print_info "文件树结构（前10个文件）:"
echo "$SCAN_RESULT" | jq -r '
    def flatten_tree(node):
        if node.type == "file" then
            node.path
        else
            (node.children[]? | flatten_tree(.))
        end;
    .data.file_tree | flatten_tree(.) | limit(10; .)
'

# 6. 如果有survey，显示第一个survey的参数
if [ "$SURVEY_COUNT" -gt 0 ]; then
    print_info "第一个Survey的参数:"
    echo "$SCAN_RESULT" | jq '.data.surveys[0].parameters[] | "  - \(.name) (\(.type)): \(.description)"'
fi

print_success "增强扫描测试完成"