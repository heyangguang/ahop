#!/bin/bash

# 预览功能测试脚本

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

# 1. 测试规则预览
print_info "测试规则预览功能..."

# 获取第一个规则
RULES_RESPONSE=$(curl -s -X GET "$API_BASE_URL/healing/rules?page=1&page_size=1" \
  -H "Authorization: Bearer $TOKEN")

RULE_ID=$(echo $RULES_RESPONSE | jq -r '.data[0].id')
RULE_NAME=$(echo $RULES_RESPONSE | jq -r '.data[0].name')

if [ "$RULE_ID" != "null" ] && [ -n "$RULE_ID" ]; then
    print_info "预览规则: $RULE_NAME (ID: $RULE_ID)"
    
    # 预览规则匹配
    PREVIEW_RESPONSE=$(curl -s -X POST "$API_BASE_URL/healing/rules/$RULE_ID/preview" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d '{
        "test_tickets": [
          {
            "external_id": "TEST-001",
            "title": "测试磁盘空间不足告警",
            "description": "服务器 192.168.1.100 磁盘使用率达到 95%",
            "priority": "high",
            "status": "open",
            "custom_data": {
              "disk_usage": 95,
              "partition": "/home",
              "server_ip": "192.168.1.100"
            }
          },
          {
            "external_id": "TEST-002",
            "title": "测试内存使用率告警",
            "description": "服务器 192.168.1.101 内存使用率达到 80%",
            "priority": "medium",
            "status": "open",
            "custom_data": {
              "memory_usage": 80,
              "server_ip": "192.168.1.101"
            }
          }
        ]
      }')
    
    echo ""
    echo "========== 规则预览结果 =========="
    echo "$PREVIEW_RESPONSE" | jq '.'
else
    print_warning "没有找到规则，跳过规则预览测试"
fi

# 2. 测试工作流预览
print_info "测试工作流预览功能..."

# 获取第一个工作流
WORKFLOWS_RESPONSE=$(curl -s -X GET "$API_BASE_URL/healing/workflows?page=1&page_size=1" \
  -H "Authorization: Bearer $TOKEN")

WORKFLOW_ID=$(echo $WORKFLOWS_RESPONSE | jq -r '.data[0].id')
WORKFLOW_NAME=$(echo $WORKFLOWS_RESPONSE | jq -r '.data[0].name')

if [ "$WORKFLOW_ID" != "null" ] && [ -n "$WORKFLOW_ID" ]; then
    print_info "预览工作流: $WORKFLOW_NAME (ID: $WORKFLOW_ID)"
    
    # 预览工作流执行
    WORKFLOW_PREVIEW_RESPONSE=$(curl -s -X POST "$API_BASE_URL/healing/workflows/$WORKFLOW_ID/preview" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d '{
        "trigger_data": {
          "ticket": {
            "id": "TEST-001",
            "title": "磁盘空间不足",
            "custom_data": {
              "disk_usage": 95,
              "partition": "/home",
              "server_ip": "192.168.1.100"
            }
          }
        }
      }')
    
    echo ""
    echo "========== 工作流预览结果 =========="
    echo "$WORKFLOW_PREVIEW_RESPONSE" | jq '.'
else
    print_warning "没有找到工作流，跳过工作流预览测试"
fi

print_info "预览功能测试完成！"