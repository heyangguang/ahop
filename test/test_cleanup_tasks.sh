#!/bin/bash

# 测试任务清理接口

# 基础配置
BASE_URL="http://localhost:8080/api/v1"
TOKEN=""

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 登录获取Token
echo -e "${YELLOW}1. 登录管理员账号...${NC}"
LOGIN_RESPONSE=$(curl -s -X POST "${BASE_URL}/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "Admin@123"
  }')

TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.token' 2>/dev/null)

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo -e "${RED}登录失败！${NC}"
  echo $LOGIN_RESPONSE | jq .
  exit 1
fi

echo -e "${GREEN}登录成功！${NC}"

# 查看当前队列状态
echo -e "\n${YELLOW}2. 查看当前队列状态...${NC}"
QUEUE_STATUS=$(curl -s -X GET "${BASE_URL}/tasks/stats" \
  -H "Authorization: Bearer $TOKEN")

echo "队列状态："
echo $QUEUE_STATUS | jq .

# 获取当前队列中的任务
echo -e "\n${YELLOW}3. 获取队列中的任务...${NC}"
TASKS_RESPONSE=$(curl -s -X GET "${BASE_URL}/tasks?status=queued&page_size=20" \
  -H "Authorization: Bearer $TOKEN")

echo "队列中的任务："
echo $TASKS_RESPONSE | jq '.data.data[] | {task_id, name, status, created_at}'

# 执行清理
echo -e "\n${YELLOW}4. 执行僵尸任务清理...${NC}"
CLEANUP_RESPONSE=$(curl -s -X POST "${BASE_URL}/tasks/cleanup" \
  -H "Authorization: Bearer $TOKEN")

echo "清理结果："
echo $CLEANUP_RESPONSE | jq .

# 再次查看队列状态
echo -e "\n${YELLOW}5. 清理后的队列状态...${NC}"
QUEUE_STATUS_AFTER=$(curl -s -X GET "${BASE_URL}/tasks/stats" \
  -H "Authorization: Bearer $TOKEN")

echo "清理后的队列状态："
echo $QUEUE_STATUS_AFTER | jq .

# 再次获取队列中的任务
echo -e "\n${YELLOW}6. 清理后队列中的任务...${NC}"
TASKS_AFTER=$(curl -s -X GET "${BASE_URL}/tasks?status=queued&page_size=20" \
  -H "Authorization: Bearer $TOKEN")

echo "清理后队列中的任务："
echo $TASKS_AFTER | jq '.data.data[] | {task_id, name, status, created_at}'