#!/bin/bash

# 自愈规则执行记录测试脚本
# 测试规则执行记录的创建和查询功能

# API基础URL
API_BASE_URL="http://localhost:8080/api/v1"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印带颜色的信息
print_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }

# 登录获取token
print_info "登录获取认证token..."
LOGIN_RESPONSE=$(curl -s -X POST "$API_BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "Admin@123"
  }')

# 提取token
TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.token')
if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
    print_error "登录失败: $LOGIN_RESPONSE"
    exit 1
fi
print_info "登录成功，获取到token"

# 获取规则执行记录列表
print_info "获取规则执行记录列表..."
EXECUTIONS_RESPONSE=$(curl -s -X GET "$API_BASE_URL/healing/rule-executions?page=1&page_size=10" \
  -H "Authorization: Bearer $TOKEN")

echo "规则执行记录列表："
echo $EXECUTIONS_RESPONSE | jq .

# 获取执行统计
print_info "获取规则执行统计..."
STATS_RESPONSE=$(curl -s -X GET "$API_BASE_URL/healing/rule-executions/stats" \
  -H "Authorization: Bearer $TOKEN")

echo "规则执行统计："
echo $STATS_RESPONSE | jq .

# 获取最近的执行记录
print_info "获取最近的执行记录..."
RECENT_RESPONSE=$(curl -s -X GET "$API_BASE_URL/healing/rule-executions/recent?limit=5" \
  -H "Authorization: Bearer $TOKEN")

echo "最近的执行记录："
echo $RECENT_RESPONSE | jq .

# 如果有规则，获取指定规则的执行记录
print_info "获取第一个规则的执行记录..."
RULES_RESPONSE=$(curl -s -X GET "$API_BASE_URL/healing/rules?page=1&page_size=1" \
  -H "Authorization: Bearer $TOKEN")

FIRST_RULE_ID=$(echo $RULES_RESPONSE | jq -r '.data[0].id')
if [ "$FIRST_RULE_ID" != "null" ] && [ -n "$FIRST_RULE_ID" ]; then
    print_info "获取规则 $FIRST_RULE_ID 的执行记录..."
    RULE_EXECUTIONS_RESPONSE=$(curl -s -X GET "$API_BASE_URL/healing/rules/$FIRST_RULE_ID/executions?page=1&page_size=10" \
      -H "Authorization: Bearer $TOKEN")
    
    echo "规则 $FIRST_RULE_ID 的执行记录："
    echo $RULE_EXECUTIONS_RESPONSE | jq .
else
    print_warning "没有找到规则，跳过规则执行记录测试"
fi

# 手动执行一个规则并检查执行记录
if [ "$FIRST_RULE_ID" != "null" ] && [ -n "$FIRST_RULE_ID" ]; then
    print_info "手动执行规则 $FIRST_RULE_ID..."
    EXECUTE_RESPONSE=$(curl -s -X POST "$API_BASE_URL/healing/rules/$FIRST_RULE_ID/execute" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d '{}')
    
    echo "执行响应："
    echo $EXECUTE_RESPONSE | jq .
    
    # 等待几秒让执行完成
    sleep 3
    
    # 再次获取该规则的执行记录
    print_info "获取更新后的执行记录..."
    UPDATED_EXECUTIONS_RESPONSE=$(curl -s -X GET "$API_BASE_URL/healing/rules/$FIRST_RULE_ID/executions?page=1&page_size=1" \
      -H "Authorization: Bearer $TOKEN")
    
    echo "最新的执行记录："
    echo $UPDATED_EXECUTIONS_RESPONSE | jq .
    
    # 获取执行记录详情
    LATEST_EXEC_ID=$(echo $UPDATED_EXECUTIONS_RESPONSE | jq -r '.data[0].id')
    if [ "$LATEST_EXEC_ID" != "null" ] && [ -n "$LATEST_EXEC_ID" ]; then
        print_info "获取执行记录 $LATEST_EXEC_ID 的详情..."
        DETAIL_RESPONSE=$(curl -s -X GET "$API_BASE_URL/healing/rule-executions/$LATEST_EXEC_ID/detail" \
          -H "Authorization: Bearer $TOKEN")
        
        echo "执行记录详情："
        echo $DETAIL_RESPONSE | jq .
    fi
fi

print_info "测试完成！"