#!/bin/bash

# 检查队列中的任务状态

echo "=== 检查Redis队列状态 ==="
echo

# 检查各优先级队列长度
echo "队列长度统计："
for i in {1..10}; do
    len=$(redis-cli -a "Admin@123" LLEN "ahop:queue:priority:$i" 2>/dev/null)
    if [ "$len" -gt 0 ]; then
        echo "  优先级 $i: $len 个任务"
    fi
done
echo

# 获取优先级5队列中的任务
echo "优先级5队列中的任务："
redis-cli -a "Admin@123" LRANGE "ahop:queue:priority:5" 0 2 2>/dev/null | while read -r line; do
    if [ ! -z "$line" ]; then
        # 尝试提取task_id
        task_id=$(echo "$line" | grep -o '"task_id":"[^"]*"' | cut -d'"' -f4)
        task_type=$(echo "$line" | grep -o '"task_type":"[^"]*"' | cut -d'"' -f4)
        created=$(echo "$line" | grep -o '"created":[0-9]*' | cut -d':' -f2)
        
        if [ ! -z "$task_id" ]; then
            echo "  Task ID: $task_id"
            echo "  Type: $task_type"
            if [ ! -z "$created" ]; then
                echo "  Created: $(date -d @$created '+%Y-%m-%d %H:%M:%S')"
            fi
            echo "  ---"
        fi
    fi
done

echo
echo "=== 检查Worker连接状态 ==="
# 检查是否有活跃的Worker
worker_count=$(redis-cli -a "Admin@123" ZCARD "ahop:workers:active" 2>/dev/null)
echo "活跃Worker数量: ${worker_count:-0}"

if [ "${worker_count:-0}" -gt 0 ]; then
    echo "活跃的Worker列表："
    redis-cli -a "Admin@123" ZRANGE "ahop:workers:active" 0 -1 WITHSCORES 2>/dev/null
fi

echo
echo "=== 总结 ==="
echo "如果有任务在队列中但没有Worker处理，这些任务会一直保持'queued'状态。"
echo "解决方案："
echo "1. 启动Worker来处理队列中的任务"
echo "2. 或者手动取消这些任务"
echo "3. 等待清理服务标记它们为失败（如果它们不在Redis队列中）"