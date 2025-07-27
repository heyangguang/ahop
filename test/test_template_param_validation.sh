#!/bin/bash

# 测试任务模板参数验证功能

API_URL="http://localhost:8080"
TOKEN=""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印函数
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# 1. 登录获取token
login() {
    print_info "正在登录..."
    response=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{
            "username": "admin",
            "password": "Admin@123"
        }')
    
    TOKEN=$(echo $response | jq -r '.data.token')
    if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
        print_error "登录失败"
        echo $response | jq
        exit 1
    fi
    print_info "登录成功"
}

# 2. 创建测试模板（带参数验证）
create_test_template() {
    print_info "创建测试模板..."
    
    # 首先获取仓库ID
    repos_response=$(curl -s -X GET "$API_URL/api/v1/git-repositories" \
        -H "Authorization: Bearer $TOKEN")
    
    repo_id=$(echo $repos_response | jq -r '.data[0].id')
    if [ "$repo_id" == "null" ] || [ -z "$repo_id" ]; then
        print_warning "没有找到Git仓库，请先创建一个Git仓库"
        return 1
    fi
    
    response=$(curl -s -X POST "$API_URL/api/v1/task-templates" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
            \"name\": \"参数验证测试模板\",
            \"code\": \"param_validation_test\",
            \"script_type\": \"shell\",
            \"entry_file\": \"test.sh\",
            \"repository_id\": $repo_id,
            \"original_path\": \"test\",
            \"included_files\": [{\"path\": \"test.sh\"}],
            \"execution_type\": \"ssh\",
            \"require_sudo\": false,
            \"timeout\": 60,
            \"parameters\": [
                {
                    \"variable\": \"required_param\",
                    \"type\": \"text\",
                    \"question_name\": \"必填参数\",
                    \"question_description\": \"这是一个必填参数\",
                    \"required\": true
                },
                {
                    \"variable\": \"number_param\",
                    \"type\": \"integer\",
                    \"question_name\": \"数字参数\",
                    \"question_description\": \"必须在1-10之间\",
                    \"required\": false,
                    \"default\": \"5\",
                    \"min\": 1,
                    \"max\": 10
                },
                {
                    \"variable\": \"select_param\",
                    \"type\": \"multiplechoice\",
                    \"question_name\": \"选择参数\",
                    \"question_description\": \"从选项中选择\",
                    \"required\": false,
                    \"default\": \"option1\",
                    \"choices\": [\"option1\", \"option2\", \"option3\"]
                },
                {
                    \"variable\": \"text_length_param\",
                    \"type\": \"text\",
                    \"question_name\": \"文本长度参数\",
                    \"question_description\": \"长度必须在3-10之间\",
                    \"required\": false,
                    \"min\": 3,
                    \"max\": 10
                }
            ]
        }")
    
    template_id=$(echo $response | jq -r '.data.id')
    if [ "$template_id" == "null" ] || [ -z "$template_id" ]; then
        print_error "创建模板失败"
        echo $response | jq
        return 1
    fi
    
    print_info "模板创建成功，ID: $template_id"
    echo $template_id
}

# 3. 获取测试主机
get_test_host() {
    hosts_response=$(curl -s -X GET "$API_URL/api/v1/hosts" \
        -H "Authorization: Bearer $TOKEN")
    
    host_id=$(echo $hosts_response | jq -r '.data[0].id')
    if [ "$host_id" == "null" ] || [ -z "$host_id" ]; then
        print_warning "没有找到主机，请先创建一个主机"
        return 1
    fi
    echo $host_id
}

# 4. 测试参数验证
test_param_validation() {
    local template_id=$1
    local host_id=$2
    
    print_info "\n========== 测试参数验证 =========="
    
    # 测试1：缺少必填参数
    print_info "\n测试1：缺少必填参数"
    response=$(curl -s -X POST "$API_URL/api/v1/tasks" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
            \"task_type\": \"template\",
            \"template_id\": $template_id,
            \"name\": \"测试缺少必填参数\",
            \"hosts\": [$host_id],
            \"variables\": {
                \"number_param\": 5
            }
        }")
    
    if echo $response | grep -q "缺少必填参数"; then
        print_info "✓ 正确检测到缺少必填参数"
        echo "错误信息: $(echo $response | jq -r '.message')"
    else
        print_error "✗ 未能检测到缺少必填参数"
        echo $response | jq
    fi
    
    # 测试2：数字超出范围
    print_info "\n测试2：数字超出范围"
    response=$(curl -s -X POST "$API_URL/api/v1/tasks" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
            \"task_type\": \"template\",
            \"template_id\": $template_id,
            \"name\": \"测试数字超出范围\",
            \"hosts\": [$host_id],
            \"variables\": {
                \"required_param\": \"test\",
                \"number_param\": 20
            }
        }")
    
    if echo $response | grep -q "值必须小于等于"; then
        print_info "✓ 正确检测到数字超出范围"
        echo "错误信息: $(echo $response | jq -r '.message')"
    else
        print_error "✗ 未能检测到数字超出范围"
        echo $response | jq
    fi
    
    # 测试3：选项值不合法
    print_info "\n测试3：选项值不合法"
    response=$(curl -s -X POST "$API_URL/api/v1/tasks" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
            \"task_type\": \"template\",
            \"template_id\": $template_id,
            \"name\": \"测试选项值不合法\",
            \"hosts\": [$host_id],
            \"variables\": {
                \"required_param\": \"test\",
                \"select_param\": \"invalid_option\"
            }
        }")
    
    if echo $response | grep -q "值必须是以下选项之一"; then
        print_info "✓ 正确检测到选项值不合法"
        echo "错误信息: $(echo $response | jq -r '.message')"
    else
        print_error "✗ 未能检测到选项值不合法"
        echo $response | jq
    fi
    
    # 测试4：文本长度不符合要求
    print_info "\n测试4：文本长度太短"
    response=$(curl -s -X POST "$API_URL/api/v1/tasks" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
            \"task_type\": \"template\",
            \"template_id\": $template_id,
            \"name\": \"测试文本长度\",
            \"hosts\": [$host_id],
            \"variables\": {
                \"required_param\": \"test\",
                \"text_length_param\": \"ab\"
            }
        }")
    
    if echo $response | grep -q "长度必须大于等于"; then
        print_info "✓ 正确检测到文本长度不符合要求"
        echo "错误信息: $(echo $response | jq -r '.message')"
    else
        print_error "✗ 未能检测到文本长度问题"
        echo $response | jq
    fi
    
    # 测试5：所有参数正确
    print_info "\n测试5：所有参数正确"
    response=$(curl -s -X POST "$API_URL/api/v1/tasks" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
            \"task_type\": \"template\",
            \"template_id\": $template_id,
            \"name\": \"测试参数正确\",
            \"hosts\": [$host_id],
            \"variables\": {
                \"required_param\": \"test value\",
                \"number_param\": 5,
                \"select_param\": \"option2\",
                \"text_length_param\": \"valid\"
            }
        }")
    
    if echo $response | jq -r '.code' | grep -q "200"; then
        print_info "✓ 参数验证通过，任务创建成功"
        echo "任务ID: $(echo $response | jq -r '.data.id')"
    else
        print_error "✗ 参数正确但任务创建失败"
        echo $response | jq
    fi
}

# 主流程
main() {
    login
    
    template_id=$(create_test_template)
    if [ -z "$template_id" ] || [ "$template_id" == "1" ]; then
        print_error "无法创建测试模板"
        exit 1
    fi
    
    host_id=$(get_test_host)
    if [ -z "$host_id" ] || [ "$host_id" == "1" ]; then
        print_error "无法获取测试主机"
        exit 1
    fi
    
    test_param_validation $template_id $host_id
    
    print_info "\n测试完成！"
}

# 运行测试
main