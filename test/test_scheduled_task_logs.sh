#!/bin/bash

# 测试定时任务日志查询功能

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# API基础URL
BASE_URL="http://localhost:8080/api/v1"

# 登录函数
login() {
    local response=$(curl -s -X POST "$BASE_URL/auth/login" \
        -H "Content-Type: application/json" \
        -d '{
            "username": "admin",
            "password": "Admin@123"
        }')
    
    TOKEN=$(echo $response | jq -r '.data.access_token')
    if [ "$TOKEN" = "null" ]; then
        echo -e "${RED}登录失败${NC}"
        exit 1
    fi
    echo -e "${GREEN}登录成功${NC}"
}

# 测试创建定时任务
test_create_scheduled_task() {
    echo -e "\n${YELLOW}=== 测试创建定时任务 ===${NC}"
    
    # 首先需要一个任务模板
    # 这里假设已有模板ID为1
    local response=$(curl -s -X POST "$BASE_URL/scheduled-tasks" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "测试定时任务-日志查询",
            "description": "用于测试日志查询功能",
            "cron_expr": "*/5 * * * *",
            "template_id": 1,
            "host_ids": [1],
            "variables": {
                "test_var": "test_value"
            },
            "timeout_mins": 10,
            "is_active": true
        }')
    
    SCHEDULED_TASK_ID=$(echo $response | jq -r '.data.id')
    if [ "$SCHEDULED_TASK_ID" != "null" ]; then
        echo -e "${GREEN}创建定时任务成功，ID: $SCHEDULED_TASK_ID${NC}"
    else
        echo -e "${RED}创建定时任务失败: $response${NC}"
        exit 1
    fi
}

# 测试立即执行
test_run_now() {
    echo -e "\n${YELLOW}=== 测试立即执行定时任务 ===${NC}"
    
    local response=$(curl -s -X POST "$BASE_URL/scheduled-tasks/$SCHEDULED_TASK_ID/run" \
        -H "Authorization: Bearer $TOKEN")
    
    TASK_ID=$(echo $response | jq -r '.data.task_id')
    if [ "$TASK_ID" != "null" ]; then
        echo -e "${GREEN}触发任务成功，任务ID: $TASK_ID${NC}"
    else
        echo -e "${RED}触发任务失败: $response${NC}"
    fi
}

# 等待任务执行
wait_for_task() {
    echo -e "\n${YELLOW}=== 等待任务执行 ===${NC}"
    echo "等待10秒让任务执行并生成日志..."
    sleep 10
}

# 测试获取执行历史
test_get_executions() {
    echo -e "\n${YELLOW}=== 测试获取执行历史 ===${NC}"
    
    local response=$(curl -s -X GET "$BASE_URL/scheduled-tasks/$SCHEDULED_TASK_ID/executions" \
        -H "Authorization: Bearer $TOKEN")
    
    local count=$(echo $response | jq '.data | length')
    if [ "$count" -gt 0 ]; then
        echo -e "${GREEN}获取执行历史成功，共 $count 条记录${NC}"
        echo $response | jq '.data[0]'
        
        # 保存第一个执行ID
        EXECUTION_ID=$(echo $response | jq -r '.data[0].id')
    else
        echo -e "${RED}获取执行历史失败或无记录${NC}"
    fi
}

# 测试获取所有日志
test_get_all_logs() {
    echo -e "\n${YELLOW}=== 测试获取定时任务所有日志 ===${NC}"
    
    local response=$(curl -s -X GET "$BASE_URL/scheduled-tasks/$SCHEDULED_TASK_ID/logs" \
        -H "Authorization: Bearer $TOKEN")
    
    local count=$(echo $response | jq '.data | length')
    if [ "$count" -gt 0 ]; then
        echo -e "${GREEN}获取日志成功，共 $count 条${NC}"
        echo "前3条日志："
        echo $response | jq '.data[0:3]'
    else
        echo -e "${YELLOW}暂无日志记录${NC}"
    fi
}

# 测试按执行ID过滤
test_filter_by_execution() {
    echo -e "\n${YELLOW}=== 测试按执行ID过滤日志 ===${NC}"
    
    if [ -z "$EXECUTION_ID" ]; then
        echo -e "${YELLOW}跳过：无执行ID${NC}"
        return
    fi
    
    local response=$(curl -s -X GET "$BASE_URL/scheduled-tasks/$SCHEDULED_TASK_ID/logs?execution_id=$EXECUTION_ID" \
        -H "Authorization: Bearer $TOKEN")
    
    local count=$(echo $response | jq '.data | length')
    echo -e "${GREEN}执行ID $EXECUTION_ID 的日志数: $count${NC}"
}

# 测试按日志级别过滤
test_filter_by_level() {
    echo -e "\n${YELLOW}=== 测试按日志级别过滤 ===${NC}"
    
    for level in info warning error; do
        local response=$(curl -s -X GET "$BASE_URL/scheduled-tasks/$SCHEDULED_TASK_ID/logs?level=$level" \
            -H "Authorization: Bearer $TOKEN")
        
        local count=$(echo $response | jq '.data | length')
        echo -e "级别 $level 的日志数: $count"
    done
}

# 测试关键词搜索
test_keyword_search() {
    echo -e "\n${YELLOW}=== 测试关键词搜索 ===${NC}"
    
    local response=$(curl -s -X GET "$BASE_URL/scheduled-tasks/$SCHEDULED_TASK_ID/logs?keyword=开始" \
        -H "Authorization: Bearer $TOKEN")
    
    local count=$(echo $response | jq '.data | length')
    echo -e "包含'开始'的日志数: $count"
}

# 测试分页
test_pagination() {
    echo -e "\n${YELLOW}=== 测试分页功能 ===${NC}"
    
    local response=$(curl -s -X GET "$BASE_URL/scheduled-tasks/$SCHEDULED_TASK_ID/logs?page=1&page_size=5" \
        -H "Authorization: Bearer $TOKEN")
    
    local page_info=$(echo $response | jq '.pagination')
    echo "分页信息："
    echo $page_info | jq '.'
}

# 清理测试数据
cleanup() {
    echo -e "\n${YELLOW}=== 清理测试数据 ===${NC}"
    
    # 禁用定时任务
    curl -s -X POST "$BASE_URL/scheduled-tasks/$SCHEDULED_TASK_ID/disable" \
        -H "Authorization: Bearer $TOKEN" > /dev/null
    
    # 删除定时任务
    curl -s -X DELETE "$BASE_URL/scheduled-tasks/$SCHEDULED_TASK_ID" \
        -H "Authorization: Bearer $TOKEN" > /dev/null
    
    echo -e "${GREEN}清理完成${NC}"
}

# 主流程
main() {
    echo -e "${YELLOW}开始测试定时任务日志查询功能${NC}"
    
    # 登录
    login
    
    # 执行测试
    test_create_scheduled_task
    test_run_now
    wait_for_task
    test_get_executions
    test_get_all_logs
    test_filter_by_execution
    test_filter_by_level
    test_keyword_search
    test_pagination
    
    # 清理
    cleanup
    
    echo -e "\n${GREEN}测试完成！${NC}"
}

# 执行主流程
main