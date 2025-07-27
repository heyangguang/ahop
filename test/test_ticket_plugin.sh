#!/bin/bash

# 工单插件集成测试脚本
# 使用前请确保：
# 1. AHOP服务正在运行 (默认端口 8080)
# 2. 模拟工单插件正在运行 (python test/mock_ticket_plugin.py)

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 配置
AHOP_URL="http://localhost:8080/api/v1"
PLUGIN_URL="http://localhost:5000"

# 测试用户凭证
USERNAME="admin"
PASSWORD="Admin@123"

echo -e "${YELLOW}=== AHOP 工单插件集成测试 ===${NC}"
echo ""

# 步骤1: 登录获取token
echo -e "${GREEN}步骤1: 登录AHOP系统${NC}"
LOGIN_RESPONSE=$(curl -s -X POST "${AHOP_URL}/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "'${USERNAME}'",
    "password": "'${PASSWORD}'"
  }')

TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.token')
if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
    echo -e "${RED}登录失败！${NC}"
    echo $LOGIN_RESPONSE | jq
    exit 1
fi
echo -e "${GREEN}登录成功！${NC}"
echo ""

# 步骤2: 创建工单插件
echo -e "${GREEN}步骤2: 注册工单插件${NC}"
PLUGIN_RESPONSE=$(curl -s -X POST "${AHOP_URL}/ticket-plugins" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "测试工单系统",
    "code": "test-ticket-system",
    "description": "用于测试的模拟工单系统",
    "base_url": "'${PLUGIN_URL}'",
    "auth_type": "bearer",
    "auth_token": "test-token-12345",
    "sync_enabled": true,
    "sync_interval": 5
  }')

PLUGIN_ID=$(echo $PLUGIN_RESPONSE | jq -r '.data.id')
if [ "$PLUGIN_ID" == "null" ] || [ -z "$PLUGIN_ID" ]; then
    echo -e "${RED}创建插件失败！${NC}"
    echo $PLUGIN_RESPONSE | jq
    # 尝试获取已存在的插件
    echo -e "${YELLOW}尝试获取已存在的插件...${NC}"
    LIST_RESPONSE=$(curl -s -X GET "${AHOP_URL}/ticket-plugins" \
      -H "Authorization: Bearer ${TOKEN}")
    PLUGIN_ID=$(echo $LIST_RESPONSE | jq -r '.data[] | select(.code == "test-ticket-system") | .id')
    if [ -z "$PLUGIN_ID" ]; then
        exit 1
    fi
fi
echo -e "${GREEN}插件创建/获取成功！插件ID: ${PLUGIN_ID}${NC}"
echo ""

# 步骤3: 测试插件连接
echo -e "${GREEN}步骤3: 测试插件连接${NC}"
TEST_CONN_RESPONSE=$(curl -s -X POST "${AHOP_URL}/ticket-plugins/${PLUGIN_ID}/test" \
  -H "Authorization: Bearer ${TOKEN}")

echo $TEST_CONN_RESPONSE | jq
echo ""

# 步骤4: 创建字段映射（可选）
echo -e "${GREEN}步骤4: 配置字段映射（使用默认映射）${NC}"
echo "跳过，将使用系统默认的字段映射规则"
echo ""

# 步骤5: 创建同步规则（可选）
echo -e "${GREEN}步骤5: 配置同步规则（示例：只同步高优先级工单）${NC}"
# 这里可以添加创建同步规则的API调用
echo "跳过，将同步所有工单"
echo ""

# 步骤6: 测试同步（预览）
echo -e "${GREEN}步骤6: 测试同步功能（预览模式）${NC}"
TEST_SYNC_RESPONSE=$(curl -s -X POST "${AHOP_URL}/ticket-plugins/${PLUGIN_ID}/test-sync" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_params": {
      "minutes": 60
    },
    "test_options": {
      "sample_size": 3,
      "show_filtered": true,
      "show_mapping_details": true
    }
  }')

echo -e "${YELLOW}测试同步结果：${NC}"
echo $TEST_SYNC_RESPONSE | jq
echo ""

# 步骤7: 手动触发同步
echo -e "${GREEN}步骤7: 手动触发实际同步${NC}"
read -p "是否执行实际同步？(y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]
then
    SYNC_RESPONSE=$(curl -s -X POST "${AHOP_URL}/ticket-plugins/${PLUGIN_ID}/sync" \
      -H "Authorization: Bearer ${TOKEN}")
    
    echo $SYNC_RESPONSE | jq
    echo ""
    
    # 等待几秒让同步完成
    echo "等待同步完成..."
    sleep 3
    
    # 步骤8: 查看同步日志
    echo -e "${GREEN}步骤8: 查看同步日志${NC}"
    LOGS_RESPONSE=$(curl -s -X GET "${AHOP_URL}/ticket-plugins/${PLUGIN_ID}/sync-logs" \
      -H "Authorization: Bearer ${TOKEN}")
    
    echo $LOGS_RESPONSE | jq '.data[0]'
    echo ""
    
    # 步骤9: 查看同步的工单
    echo -e "${GREEN}步骤9: 查看同步的工单${NC}"
    TICKETS_RESPONSE=$(curl -s -X GET "${AHOP_URL}/tickets" \
      -H "Authorization: Bearer ${TOKEN}")
    
    echo -e "${YELLOW}同步的工单列表：${NC}"
    echo $TICKETS_RESPONSE | jq '.data[] | {id: .external_id, title: .title, status: .status, priority: .priority}'
fi

echo ""
echo -e "${GREEN}=== 测试完成 ===${NC}"
echo ""
echo -e "${YELLOW}其他可用的API：${NC}"
echo "- 获取插件列表: GET ${AHOP_URL}/ticket-plugins"
echo "- 获取插件详情: GET ${AHOP_URL}/ticket-plugins/${PLUGIN_ID}"
echo "- 更新插件配置: PUT ${AHOP_URL}/ticket-plugins/${PLUGIN_ID}"
echo "- 禁用插件: POST ${AHOP_URL}/ticket-plugins/${PLUGIN_ID}/disable"
echo "- 启用插件: POST ${AHOP_URL}/ticket-plugins/${PLUGIN_ID}/enable"
echo "- 删除插件: DELETE ${AHOP_URL}/ticket-plugins/${PLUGIN_ID}"
echo "- 查看工单详情: GET ${AHOP_URL}/tickets/{ticket_id}"
echo "- 工单统计: GET ${AHOP_URL}/tickets/stats"