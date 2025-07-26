#!/bin/bash
# 简单的测试脚本 - 不需要mysqldump等外部命令

# @param message 要显示的消息
# @param count 重复次数

# 解析参数
MESSAGE="Hello from AHOP!"
COUNT=3

while [[ $# -gt 0 ]]; do
    case $1 in
        --message)
            MESSAGE="$2"
            shift 2
            ;;
        --count)
            COUNT="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

echo "==================================="
echo "AHOP Task Template Test"
echo "==================================="
echo "Message: $MESSAGE"
echo "Count: $COUNT"
echo "Time: $(date)"
echo "==================================="

# 循环输出消息
for i in $(seq 1 $COUNT); do
    echo "Echo $i: $MESSAGE"
    sleep 1
done

echo ""
echo "Test completed successfully!"
exit 0