#!/bin/bash

# 测试规则匹配工单记录功能

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

TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.token')
if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
    print_error "登录失败: $LOGIN_RESPONSE"
    exit 1
fi

# 获取第一个规则
print_info "获取第一个自愈规则..."
RULES_RESPONSE=$(curl -s -X GET "$API_BASE_URL/healing/rules?page=1&page_size=1" \
  -H "Authorization: Bearer $TOKEN")

FIRST_RULE_ID=$(echo $RULES_RESPONSE | jq -r '.data[0].id')
FIRST_RULE_NAME=$(echo $RULES_RESPONSE | jq -r '.data[0].name')

if [ "$FIRST_RULE_ID" == "null" ] || [ -z "$FIRST_RULE_ID" ]; then
    print_error "没有找到自愈规则"
    exit 1
fi

print_info "找到规则: $FIRST_RULE_NAME (ID: $FIRST_RULE_ID)"

# 获取该规则的执行记录
print_info "获取规则 '$FIRST_RULE_NAME' 的执行记录..."
EXECUTIONS_RESPONSE=$(curl -s -X GET "$API_BASE_URL/healing/rules/$FIRST_RULE_ID/executions?page=1&page_size=5" \
  -H "Authorization: Bearer $TOKEN")

echo ""
echo "========== 规则执行记录 =========="
echo "$EXECUTIONS_RESPONSE" | jq -r '
  .data[] | 
  "执行时间: \(.execution_time)
状态: \(.status)
扫描工单数: \(.total_tickets_scanned)
匹配工单数: \(.matched_tickets)
创建执行数: \(.executions_created)
执行耗时: \(.duration)ms
匹配的工单: \(if .matched_ticket_list and (.matched_ticket_list | length) > 0 then 
  (.matched_ticket_list | map("  - [\(.external_id)] \(.title) (优先级: \(.priority), 状态: \(.status))") | join("\n"))
else 
  "  无"
end)
---"'

# 获取执行统计
print_info "获取规则执行统计..."
STATS_RESPONSE=$(curl -s -X GET "$API_BASE_URL/healing/rules/execution-stats" \
  -H "Authorization: Bearer $TOKEN")

echo ""
echo "========== 执行统计 =========="
echo "$STATS_RESPONSE" | jq -r '
  .data[] | 
  "规则: \(.rule_name)
总执行次数: \(.total_executions)
成功次数: \(.success_executions)
失败次数: \(.failed_executions)
成功率: \(.success_rate | tostring | .[0:5])%
总扫描工单: \(.total_tickets_scanned)
总匹配工单: \(.total_matched)
匹配率: \(.match_rate | tostring | .[0:5])%
平均耗时: \(.avg_duration | tostring | .[0:6])ms
---"'

print_info "测试完成！"