#!/bin/bash

# 运行所有测试脚本
# 包括JWT权限、租户切换、凭证管理等功能测试

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# 测试脚本列表
TEST_SCRIPTS=(
    "test_jwt_perm.sh:JWT权限系统测试"
    "test_tenant_switch.sh:租户切换功能测试"
    "test_credential.sh:凭证管理功能测试"
)

# 测试结果统计
TOTAL_SCRIPTS=0
PASSED_SCRIPTS=0
FAILED_SCRIPTS=0

print_header() {
    echo -e "${CYAN}================================================================${NC}"
    echo -e "${CYAN}🚀 AHOP 自动化测试套件${NC}"
    echo -e "${CYAN}📅 $(date '+%Y-%m-%d %H:%M:%S')${NC}"
    echo -e "${CYAN}================================================================${NC}"
    echo ""
}

print_section() {
    echo ""
    echo -e "${PURPLE}▶ $1${NC}"
    echo "================================================================"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# 检查服务是否运行
check_service() {
    print_section "服务检查"
    
    # 检查服务是否在监听8080端口
    if lsof -i:8080 >/dev/null 2>&1 || netstat -tln | grep -q ":8080"; then
        print_success "AHOP服务正在运行（端口8080）"
    else
        print_error "AHOP服务未运行，请先启动服务"
        echo "运行命令: go run cmd/server/*.go"
        exit 1
    fi
    
    # 检查健康检查接口
    HEALTH_RESP=$(curl -s -X GET "http://localhost:8080/api/v1/health")
    HEALTH_STATUS=$(echo "$HEALTH_RESP" | jq -r '.data.status // .status' 2>/dev/null)
    
    if [ "$HEALTH_STATUS" = "ok" ]; then
        print_success "健康检查通过"
    else
        print_warning "健康检查异常"
    fi
}

# 运行单个测试脚本
run_test_script() {
    local script_info=$1
    local script_name=$(echo "$script_info" | cut -d':' -f1)
    local script_desc=$(echo "$script_info" | cut -d':' -f2)
    local script_path="$(dirname $0)/$script_name"
    
    TOTAL_SCRIPTS=$((TOTAL_SCRIPTS + 1))
    
    print_section "运行测试: $script_desc"
    echo "脚本: $script_name"
    echo ""
    
    if [ ! -f "$script_path" ]; then
        print_error "测试脚本不存在: $script_path"
        FAILED_SCRIPTS=$((FAILED_SCRIPTS + 1))
        return
    fi
    
    if [ ! -x "$script_path" ]; then
        chmod +x "$script_path"
    fi
    
    # 运行测试脚本
    "$script_path"
    local exit_code=$?
    
    if [ $exit_code -eq 0 ]; then
        print_success "$script_desc 测试通过"
        PASSED_SCRIPTS=$((PASSED_SCRIPTS + 1))
    else
        print_error "$script_desc 测试失败（退出码: $exit_code）"
        FAILED_SCRIPTS=$((FAILED_SCRIPTS + 1))
    fi
    
    echo ""
    echo "================================================================"
}

# 显示测试总结
print_summary() {
    print_section "测试总结"
    
    echo -e "📊 测试脚本总数: ${YELLOW}$TOTAL_SCRIPTS${NC}"
    echo -e "✅ 通过的脚本数: ${GREEN}$PASSED_SCRIPTS${NC}"
    echo -e "❌ 失败的脚本数: ${RED}$FAILED_SCRIPTS${NC}"
    
    local success_rate=0
    if [ $TOTAL_SCRIPTS -gt 0 ]; then
        success_rate=$(( PASSED_SCRIPTS * 100 / TOTAL_SCRIPTS ))
    fi
    echo -e "📈 总体成功率: ${CYAN}$success_rate%${NC}"
    
    echo ""
    if [ $FAILED_SCRIPTS -eq 0 ]; then
        echo -e "${GREEN}🎉 所有测试通过！${NC}"
        return 0
    else
        echo -e "${RED}⚠️  有 $FAILED_SCRIPTS 个测试脚本失败${NC}"
        return 1
    fi
}

# 主函数
main() {
    print_header
    
    # 检查服务状态
    check_service
    
    # 运行所有测试
    for test_script in "${TEST_SCRIPTS[@]}"; do
        run_test_script "$test_script"
    done
    
    # 显示总结
    print_summary
    exit_code=$?
    
    echo ""
    echo -e "${CYAN}📝 测试完成时间: $(date '+%Y-%m-%d %H:%M:%S')${NC}"
    echo -e "${CYAN}================================================================${NC}"
    
    exit $exit_code
}

# 处理命令行参数
if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  --help, -h    显示帮助信息"
    echo "  --list, -l    列出所有测试脚本"
    echo ""
    echo "示例:"
    echo "  $0            运行所有测试"
    echo "  $0 --list     查看可用的测试脚本"
    exit 0
fi

if [ "$1" = "--list" ] || [ "$1" = "-l" ]; then
    echo "可用的测试脚本:"
    for test_script in "${TEST_SCRIPTS[@]}"; do
        local script_name=$(echo "$test_script" | cut -d':' -f1)
        local script_desc=$(echo "$test_script" | cut -d':' -f2)
        echo "  - $script_name: $script_desc"
    done
    exit 0
fi

# 运行主函数
main