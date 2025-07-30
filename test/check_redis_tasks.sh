#!/bin/bash

# 检查Redis中的任务状态

echo "=== 检查Redis中的任务 ==="

# 任务ID列表
TASK_IDS=(
    "859bbb0f-7a49-44bd-a366-641ede8b0f9c"
    "b612b3d9-4da5-4eea-922f-f520ad6c615e"
    "ee2f4435-4790-4ad4-9b48-241a5e1ab15a"
    "fb1d411a-fe04-40a5-82c4-2f5fc0dc324d"
    "f43fcbd6-c427-4922-9d62-9d27140e0339"
)

for task_id in "${TASK_IDS[@]}"; do
    echo -e "\n任务ID: $task_id"
    
    # 检查任务信息是否存在
    task_key="ahop:queue:task:$task_id"
    exists=$(redis-cli -a "Admin@123" EXISTS "$task_key" 2>/dev/null)
    
    if [ "$exists" = "1" ]; then
        echo "  Redis任务信息: 存在"
        # 获取任务信息
        echo "  任务详情:"
        redis-cli -a "Admin@123" HGETALL "$task_key" 2>/dev/null | while read -r key && read -r value; do
            echo "    $key: $value"
        done
    else
        echo "  Redis任务信息: 不存在"
    fi
    
    # 检查是否在任何优先级队列中
    echo -n "  在队列中: "
    found=0
    for priority in {1..10}; do
        queue_key="ahop:queue:priority:$priority"
        # 检查队列中是否包含该任务
        in_queue=$(redis-cli -a "Admin@123" LRANGE "$queue_key" 0 -1 2>/dev/null | grep -c "$task_id")
        if [ "$in_queue" -gt 0 ]; then
            echo "是（优先级$priority队列）"
            found=1
            break
        fi
    done
    if [ $found -eq 0 ]; then
        echo "否"
    fi
done

echo -e "\n=== 检查各优先级队列长度 ==="
for i in {1..10}; do
    len=$(redis-cli -a "Admin@123" LLEN "ahop:queue:priority:$i" 2>/dev/null)
    if [ "$len" -gt 0 ]; then
        echo "优先级 $i: $len 个任务"
    fi
done