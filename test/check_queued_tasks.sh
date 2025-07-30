#!/bin/bash

# 查看queued状态任务的详细信息

BASE_URL="http://localhost:8080/api/v1"

# 登录
echo "登录管理员账号..."
LOGIN_RESPONSE=$(curl -s -X POST "${BASE_URL}/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "Admin@123"
  }')

TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.token' 2>/dev/null)

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "登录失败！"
  exit 1
fi

# 获取所有任务
echo -e "\n获取所有任务（包括queued状态）..."
TASKS_RESPONSE=$(curl -s -X GET "${BASE_URL}/tasks?page_size=100" \
  -H "Authorization: Bearer $TOKEN")

# 筛选出queued状态的任务
echo -e "\n=== Queued状态的任务 ==="
echo $TASKS_RESPONSE | jq -r '.data.data[] | select(.status == "queued") | "\(.task_id) | \(.name) | \(.created_at) | \(.queued_at)"' | while IFS='|' read -r task_id name created_at queued_at; do
    task_id=$(echo $task_id | tr -d ' ')
    name=$(echo $name | tr -d ' ')
    created_at=$(echo $created_at | tr -d ' ')
    queued_at=$(echo $queued_at | tr -d ' ')
    
    echo "任务ID: $task_id"
    echo "任务名称: $name"
    echo "创建时间: $created_at"
    echo "入队时间: $queued_at"
    
    # 检查Redis中是否存在
    echo -n "Redis队列状态: "
    exists=$(redis-cli -a "Admin@123" EXISTS "ahop:queue:task:$task_id" 2>/dev/null)
    if [ "$exists" = "1" ]; then
        echo "存在任务信息"
        # 获取任务状态
        status=$(redis-cli -a "Admin@123" HGET "ahop:queue:task:$task_id" status 2>/dev/null)
        echo "Redis中的状态: $status"
    else
        echo "不存在（僵尸任务）"
    fi
    
    # 检查是否在任何优先级队列中
    echo -n "在队列中: "
    found=0
    for priority in {1..10}; do
        queue_len=$(redis-cli -a "Admin@123" LLEN "ahop:queue:priority:$priority" 2>/dev/null)
        if [ "$queue_len" -gt 0 ]; then
            # 搜索队列中是否有该任务
            in_queue=$(redis-cli -a "Admin@123" LRANGE "ahop:queue:priority:$priority" 0 -1 2>/dev/null | grep -c "$task_id")
            if [ "$in_queue" -gt 0 ]; then
                echo "是（优先级$priority队列）"
                found=1
                break
            fi
        fi
    done
    if [ $found -eq 0 ]; then
        echo "否（确认是僵尸任务）"
    fi
    
    echo "---"
done