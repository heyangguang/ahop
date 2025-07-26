#!/bin/bash

# 测试文件树详细信息

# 设置基础URL
BASE_URL="http://localhost:8080/api/v1"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
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

print_detail() {
    echo -e "${BLUE}  $1${NC}"
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
    exit 1
fi
print_success "登录成功"

# 2. 直接使用仓库ID 68
REPO_ID=68
print_info "扫描仓库ID: $REPO_ID"

# 3. 扫描仓库模板
SCAN_RESULT=$(curl -s -X POST "$BASE_URL/git-repositories/$REPO_ID/scan-templates" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json")

# 保存结果到文件以便调试
echo "$SCAN_RESULT" > /tmp/scan_result.json

# 检查扫描结果
CODE=$(echo "$SCAN_RESULT" | jq -r '.code')
if [ "$CODE" != "200" ]; then
    print_error "扫描失败"
    echo "$SCAN_RESULT" | jq .
    exit 1
fi

print_success "扫描成功"

# 4. 显示文件树结构
print_info "文件树结构:"

# 定义递归函数来打印树
print_tree() {
    local node="$1"
    local indent="$2"
    
    # 获取节点信息
    local id=$(echo "$node" | jq -r '.id')
    local name=$(echo "$node" | jq -r '.name')
    local type=$(echo "$node" | jq -r '.type')
    local file_type=$(echo "$node" | jq -r '.file_type // ""')
    local selectable=$(echo "$node" | jq -r '.selectable')
    local path=$(echo "$node" | jq -r '.path')
    
    # 打印节点
    if [ "$type" = "directory" ]; then
        echo -e "${indent}📁 ${YELLOW}${name}${NC} [${id}]"
    else
        # 根据文件类型使用不同的图标和颜色
        local icon="📄"
        local color="${NC}"
        
        case "$file_type" in
            "ansible")
                icon="🎭"
                color="${GREEN}"
                ;;
            "shell")
                icon="🔧"
                color="${BLUE}"
                ;;
            "template")
                icon="📝"
                color="${YELLOW}"
                ;;
            "survey")
                icon="📋"
                color="${RED}"
                ;;
        esac
        
        echo -e "${indent}${icon} ${color}${name}${NC} [${id}] (${file_type}) ${selectable:+✓}"
    fi
    
    # 递归处理子节点
    local children=$(echo "$node" | jq -c '.children[]?' 2>/dev/null)
    if [ -n "$children" ]; then
        while IFS= read -r child; do
            if [ -n "$child" ]; then
                print_tree "$child" "${indent}  "
            fi
        done <<< "$children"
    fi
}

# 获取文件树根节点
FILE_TREE=$(echo "$SCAN_RESULT" | jq -c '.data.file_tree')
if [ -n "$FILE_TREE" ] && [ "$FILE_TREE" != "null" ]; then
    print_tree "$FILE_TREE" ""
else
    print_error "没有文件树数据"
fi

# 5. 显示文件统计
echo
print_info "文件统计:"
echo "$SCAN_RESULT" | jq -r '.data.stats | 
    "  Ansible文件: \(.ansible_files)",
    "  Shell脚本: \(.shell_files)",
    "  模板文件: \(.template_files)",
    "  Survey文件: \(.survey_files)",
    "  总计: \(.total_files)"'

# 6. 显示可选文件列表
echo
print_info "可选文件列表:"
echo "$SCAN_RESULT" | jq -r '
    def collect_files(node):
        if node.type == "file" and node.selectable then
            "  [\(node.id)] \(node.path) (\(node.file_type))"
        else
            (node.children[]? | collect_files(.))
        end;
    .data.file_tree | collect_files(.)
' | sort

print_success "测试完成"