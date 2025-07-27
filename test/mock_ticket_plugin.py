#!/usr/bin/env python3
"""
模拟工单插件 - 用于测试AHOP工单集成功能

使用方法:
1. 安装依赖: pip install flask
2. 运行服务: python mock_ticket_plugin.py
3. 服务将在 http://localhost:5000 启动
"""

import json
import random
from datetime import datetime, timedelta
from flask import Flask, request, jsonify

app = Flask(__name__)

# 模拟工单数据
MOCK_TICKETS = [
    {
        "id": "MOCK-001",
        "title": "生产环境数据库连接异常",
        "description": "MySQL主库连接池耗尽，导致应用无法正常访问数据库",
        "status": "open",
        "priority": "critical",
        "type": "incident",
        "reporter": "监控系统",
        "assignee": "DBA团队",
        "category": "database",
        "service": "order-service",
        "tags": ["生产环境", "紧急", "数据库"],
        "created_at": (datetime.now() - timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
        "updated_at": (datetime.now() - timedelta(minutes=30)).strftime("%Y-%m-%d %H:%M:%S")
    },
    {
        "id": "MOCK-002",
        "title": "用户登录接口响应缓慢",
        "description": "用户反馈登录接口响应时间超过5秒",
        "status": "in_progress",
        "priority": "high",
        "type": "incident",
        "reporter": "客服团队",
        "assignee": "后端开发",
        "category": "application",
        "service": "auth-service",
        "tags": ["性能问题", "用户体验"],
        "created_at": (datetime.now() - timedelta(hours=1)).strftime("%Y-%m-%d %H:%M:%S"),
        "updated_at": (datetime.now() - timedelta(minutes=10)).strftime("%Y-%m-%d %H:%M:%S")
    },
    {
        "id": "MOCK-003",
        "title": "磁盘空间告警",
        "description": "/data分区使用率达到85%",
        "status": "open",
        "priority": "medium",
        "type": "problem",
        "reporter": "监控系统",
        "assignee": "运维团队",
        "category": "infrastructure",
        "service": "file-storage",
        "tags": ["预警", "存储"],
        "created_at": (datetime.now() - timedelta(minutes=45)).strftime("%Y-%m-%d %H:%M:%S"),
        "updated_at": (datetime.now() - timedelta(minutes=45)).strftime("%Y-%m-%d %H:%M:%S")
    },
    {
        "id": "MOCK-004",
        "title": "定时任务执行失败",
        "description": "每日报表生成任务连续3天执行失败",
        "status": "resolved",
        "priority": "low",
        "type": "problem",
        "reporter": "张三",
        "assignee": "李四",
        "category": "application",
        "service": "report-service",
        "tags": ["定时任务"],
        "created_at": (datetime.now() - timedelta(days=3)).strftime("%Y-%m-%d %H:%M:%S"),
        "updated_at": (datetime.now() - timedelta(hours=12)).strftime("%Y-%m-%d %H:%M:%S")
    }
]

@app.route('/tickets', methods=['GET'])
def get_tickets():
    """获取工单列表"""
    # 获取查询参数
    minutes = request.args.get('minutes', type=int, default=60)
    
    # 验证认证（可选）
    auth_header = request.headers.get('Authorization')
    if auth_header and not auth_header.startswith('Bearer '):
        return jsonify({
            "success": False,
            "message": "Invalid authorization header"
        }), 401
    
    # 过滤最近更新的工单
    cutoff_time = datetime.now() - timedelta(minutes=minutes)
    filtered_tickets = []
    
    for ticket in MOCK_TICKETS:
        updated_at = datetime.strptime(ticket['updated_at'], "%Y-%m-%d %H:%M:%S")
        if updated_at >= cutoff_time:
            filtered_tickets.append(ticket)
    
    # 随机生成一些新工单（模拟实时数据）
    if random.random() > 0.7 and minutes <= 10:
        new_ticket = {
            "id": f"MOCK-{random.randint(100, 999)}",
            "title": f"自动发现的问题 #{random.randint(1, 100)}",
            "description": "这是一个自动生成的测试工单",
            "status": "open",
            "priority": random.choice(["high", "medium", "low"]),
            "type": "incident",
            "reporter": "自动监控",
            "assignee": "待分配",
            "category": random.choice(["database", "application", "network"]),
            "service": f"service-{random.randint(1, 5)}",
            "tags": ["自动生成", "测试"],
            "created_at": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "updated_at": datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        }
        filtered_tickets.append(new_ticket)
    
    return jsonify({
        "success": True,
        "data": filtered_tickets
    })

@app.route('/tickets/<ticket_id>/comment', methods=['POST'])
def update_ticket(ticket_id):
    """更新工单（添加评论）"""
    data = request.get_json()
    
    if not data or 'content' not in data:
        return jsonify({
            "success": False,
            "message": "Missing comment content"
        }), 400
    
    # 模拟更新操作
    print(f"收到工单 {ticket_id} 的评论: {data['content']}")
    
    # 如果请求包含状态更新
    if 'status' in data:
        print(f"更新工单状态为: {data['status']}")
    
    return jsonify({
        "success": True,
        "message": "Comment added successfully"
    })

@app.route('/health', methods=['GET'])
def health_check():
    """健康检查"""
    return jsonify({
        "status": "healthy",
        "service": "Mock Ticket Plugin",
        "timestamp": datetime.now().isoformat()
    })

if __name__ == '__main__':
    print("模拟工单插件启动中...")
    print("访问地址: http://localhost:5000")
    print("测试接口:")
    print("  - GET  /tickets?minutes=10")
    print("  - POST /tickets/{id}/comment")
    print("  - GET  /health")
    app.run(host='0.0.0.0', port=5000, debug=True)