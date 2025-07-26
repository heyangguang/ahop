#!/bin/bash

# 测试有survey文件的仓库扫描

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

# 2. 直接使用仓库ID 68
REPO_ID=68
print_info "使用仓库ID: $REPO_ID"

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
    echo
    print_info "Survey文件列表:"
    echo "$SCAN_RESULT" | jq -r '.data.surveys[] | 
        "  [\(.path)]",
        "    名称: \(.name)",
        "    描述: \(.description)",
        "    参数数量: \(.parameters | length)",
        ""'
fi

# 显示第一个survey的详细参数
if [ "$SURVEY_COUNT" -gt 0 ]; then
    print_info "第一个Survey的参数详情:"
    echo "$SCAN_RESULT" | jq -r '.data.surveys[0].parameters[] | 
        "  - \(.name) [\(.type)]",
        "    描述: \(.description)",
        "    必填: \(.required)",
        "    默认值: \(.default // "无")",
        (if .options and (.options | length > 0) then "    选项: \(.options | join(", "))" else empty end),
        ""'
fi

# 获取文件树统计
TOTAL_FILES=$(echo "$SCAN_RESULT" | jq -r '.data.stats.total_files')
TOTAL_DIRS=$(echo "$SCAN_RESULT" | jq -r '.data.stats.total_dirs')
PLAYBOOK_FILES=$(echo "$SCAN_RESULT" | jq -r '.data.stats.playbook_files')

echo
print_info "文件统计:"
echo "  - 总文件数: $TOTAL_FILES"
echo "  - 总目录数: $TOTAL_DIRS"
echo "  - Playbook文件数: $PLAYBOOK_FILES"
echo "  - Survey文件数: $SURVEY_COUNT"

print_success "测试完成"