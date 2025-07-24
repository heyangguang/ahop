#!/bin/bash

# 修复孤儿任务的脚本
# 用于重新入队状态为"queued"但不在Redis队列中的任务

TASK_ID="1cfadaf6-d44d-4ab2-afcc-8a8d9d246bf9"

echo "正在修复孤儿任务: $TASK_ID"

# 方法1: 重置任务状态为pending，让系统重新处理
curl -X PATCH "http://localhost:8080/api/v1/tasks/$TASK_ID/status" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"status": "pending"}'

echo ""
echo "任务状态已重置为pending，系统会自动重新入队"
echo "请检查Worker日志观察任务是否被重新处理"