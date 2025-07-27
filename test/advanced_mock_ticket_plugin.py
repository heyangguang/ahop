#!/usr/bin/env python3
"""
高级模拟工单插件 - 支持更多测试场景

特性：
1. 支持多种认证方式（Bearer Token, API Key）
2. 支持分页和过滤
3. 支持自定义字段结构（模拟JIRA、ServiceNow等）
4. 支持错误注入测试
"""

import json
import random
from datetime import datetime, timedelta
from flask import Flask, request, jsonify

app = Flask(__name__)

# 模拟不同系统的工单结构
TICKET_TEMPLATES = {
    "jira": {
        "id": "JIRA-{id}",
        "fields": {
            "summary": "{title}",
            "description": "{description}",
            "status": {
                "name": "{status}"
            },
            "priority": {
                "name": "{priority}"
            },
            "issuetype": {
                "name": "{type}"
            },
            "reporter": {
                "displayName": "{reporter}"
            },
            "assignee": {
                "displayName": "{assignee}"
            },
            "components": [{
                "name": "{category}"
            }],
            "labels": "{tags}",
            "created": "{created_at}",
            "updated": "{updated_at}"
        }
    },
    "servicenow": {
        "number": "INC{id}",
        "short_description": "{title}",
        "description": "{description}",
        "state": "{status}",
        "priority": "{priority}",
        "category": "{category}",
        "assigned_to": {
            "display_value": "{assignee}"
        },
        "opened_by": {
            "display_value": "{reporter}"
        },
        "sys_created_on": "{created_at}",
        "sys_updated_on": "{updated_at}",
        "sys_tags": "{tags}"
    },
    "default": {
        "id": "{id}",
        "title": "{title}",
        "description": "{description}",
        "status": "{status}",
        "priority": "{priority}",
        "type": "{type}",
        "reporter": "{reporter}",
        "assignee": "{assignee}",
        "category": "{category}",
        "service": "{service}",
        "tags": "{tags}",
        "created_at": "{created_at}",
        "updated_at": "{updated_at}"
    }
}

# 生成基础工单数据
def generate_tickets(count=50):
    tickets = []
    statuses = ["open", "in_progress", "resolved", "closed"]
    priorities = ["critical", "high", "medium", "low"]
    types = ["incident", "problem", "change", "service_request"]
    categories = ["database", "application", "network", "infrastructure", "security"]
    services = ["order-service", "auth-service", "payment-service", "user-service", "notification-service"]
    
    for i in range(1, count + 1):
        # 生成时间，越新的工单ID越大
        created_delta = timedelta(days=random.randint(0, 30), hours=random.randint(0, 23))
        updated_delta = timedelta(hours=random.randint(0, int(created_delta.total_seconds() / 3600)))
        
        ticket = {
            "id": f"{i:04d}",
            "title": f"工单标题 #{i} - {random.choice(['系统故障', '性能问题', '功能异常', '安全告警', '服务中断'])}",
            "description": f"这是工单 #{i} 的详细描述。问题发生在{random.choice(categories)}模块。",
            "status": random.choice(statuses),
            "priority": random.choice(priorities),
            "type": random.choice(types),
            "reporter": random.choice(["张三", "李四", "王五", "监控系统", "客服团队"]),
            "assignee": random.choice(["DBA团队", "后端开发", "运维团队", "安全团队", "待分配"]),
            "category": random.choice(categories),
            "service": random.choice(services),
            "tags": random.sample(["生产环境", "测试环境", "紧急", "计划内", "客户投诉"], k=random.randint(1, 3)),
            "created_at": (datetime.now() - created_delta).strftime("%Y-%m-%d %H:%M:%S"),
            "updated_at": (datetime.now() - updated_delta).strftime("%Y-%m-%d %H:%M:%S")
        }
        tickets.append(ticket)
    
    return tickets

# 全局工单数据
TICKETS = generate_tickets()

def format_ticket(ticket, template_name="default"):
    """根据模板格式化工单数据"""
    template = TICKET_TEMPLATES.get(template_name, TICKET_TEMPLATES["default"])
    
    def replace_placeholders(obj, ticket_data):
        if isinstance(obj, dict):
            return {k: replace_placeholders(v, ticket_data) for k, v in obj.items()}
        elif isinstance(obj, list):
            return [replace_placeholders(item, ticket_data) for item in obj]
        elif isinstance(obj, str):
            for key, value in ticket_data.items():
                placeholder = f"{{{key}}}"
                if placeholder in obj:
                    if isinstance(value, list):
                        obj = obj.replace(placeholder, json.dumps(value))
                    else:
                        obj = obj.replace(placeholder, str(value))
            return obj
        else:
            return obj
    
    return replace_placeholders(template, ticket)

@app.route('/tickets', methods=['GET'])
def get_tickets():
    """获取工单列表 - 支持多种查询参数"""
    # 认证检查
    auth_header = request.headers.get('Authorization')
    api_key = request.headers.get('X-API-Key')
    
    # 如果启用了认证检查
    if request.args.get('require_auth') == 'true':
        if not auth_header and not api_key:
            return jsonify({
                "success": False,
                "message": "Authentication required"
            }), 401
    
    # 获取查询参数
    minutes = request.args.get('minutes', type=int)
    status = request.args.get('status')
    priority = request.args.get('priority')
    page = request.args.get('page', type=int, default=1)
    page_size = request.args.get('page_size', type=int, default=20)
    format_type = request.args.get('format', default='default')
    
    # 错误注入（用于测试错误处理）
    if request.args.get('inject_error') == 'true':
        return jsonify({
            "success": False,
            "message": "模拟的服务器错误"
        }), 500
    
    # 过滤工单
    filtered_tickets = TICKETS.copy()
    
    # 按时间过滤
    if minutes:
        cutoff_time = datetime.now() - timedelta(minutes=minutes)
        filtered_tickets = [
            t for t in filtered_tickets
            if datetime.strptime(t['updated_at'], "%Y-%m-%d %H:%M:%S") >= cutoff_time
        ]
    
    # 按状态过滤
    if status:
        filtered_tickets = [t for t in filtered_tickets if t['status'] == status]
    
    # 按优先级过滤
    if priority:
        filtered_tickets = [t for t in filtered_tickets if t['priority'] == priority]
    
    # 分页
    total = len(filtered_tickets)
    start = (page - 1) * page_size
    end = start + page_size
    paginated_tickets = filtered_tickets[start:end]
    
    # 格式化输出
    formatted_tickets = [format_ticket(t, format_type) for t in paginated_tickets]
    
    return jsonify({
        "success": True,
        "data": formatted_tickets,
        "pagination": {
            "page": page,
            "page_size": page_size,
            "total": total,
            "total_pages": (total + page_size - 1) // page_size
        }
    })

@app.route('/tickets/<ticket_id>', methods=['GET'])
def get_ticket_detail(ticket_id):
    """获取单个工单详情"""
    format_type = request.args.get('format', default='default')
    
    # 查找工单
    ticket = None
    for t in TICKETS:
        if t['id'] == ticket_id or f"JIRA-{t['id']}" == ticket_id or f"INC{t['id']}" == ticket_id:
            ticket = t
            break
    
    if not ticket:
        return jsonify({
            "success": False,
            "message": f"Ticket {ticket_id} not found"
        }), 404
    
    return jsonify({
        "success": True,
        "data": format_ticket(ticket, format_type)
    })

@app.route('/tickets/<ticket_id>/comment', methods=['POST'])
def add_comment(ticket_id):
    """添加工单评论"""
    data = request.get_json()
    
    if not data or 'content' not in data:
        return jsonify({
            "success": False,
            "message": "Missing comment content"
        }), 400
    
    print(f"[{datetime.now()}] 工单 {ticket_id} 新增评论:")
    print(f"  内容: {data['content']}")
    if 'status' in data:
        print(f"  状态更新: {data['status']}")
    
    return jsonify({
        "success": True,
        "message": "Comment added successfully",
        "data": {
            "id": f"comment-{random.randint(1000, 9999)}",
            "ticket_id": ticket_id,
            "content": data['content'],
            "created_at": datetime.now().isoformat()
        }
    })

@app.route('/health', methods=['GET'])
def health_check():
    """健康检查"""
    return jsonify({
        "status": "healthy",
        "service": "Advanced Mock Ticket Plugin",
        "version": "2.0",
        "timestamp": datetime.now().isoformat(),
        "tickets_count": len(TICKETS)
    })

@app.route('/info', methods=['GET'])
def plugin_info():
    """插件信息 - 用于测试插件发现"""
    return jsonify({
        "name": "高级模拟工单系统",
        "version": "2.0",
        "capabilities": {
            "formats": list(TICKET_TEMPLATES.keys()),
            "filters": ["minutes", "status", "priority"],
            "pagination": True,
            "authentication": ["bearer", "apikey"]
        },
        "endpoints": {
            "tickets": "/tickets",
            "ticket_detail": "/tickets/{id}",
            "add_comment": "/tickets/{id}/comment",
            "health": "/health"
        }
    })

if __name__ == '__main__':
    print("高级模拟工单插件启动中...")
    print(f"总共生成了 {len(TICKETS)} 个测试工单")
    print("访问地址: http://localhost:5001")
    print("\n可用的查询参数:")
    print("  - format: default, jira, servicenow")
    print("  - minutes: 过滤最近N分钟更新的工单")
    print("  - status: open, in_progress, resolved, closed")
    print("  - priority: critical, high, medium, low")
    print("  - page, page_size: 分页参数")
    print("  - require_auth: true/false (是否需要认证)")
    print("  - inject_error: true/false (注入错误用于测试)")
    print("\n测试示例:")
    print("  - GET /tickets?format=jira&minutes=60&status=open")
    print("  - GET /tickets?format=servicenow&priority=high&page=1&page_size=10")
    print("  - GET /info")
    app.run(host='0.0.0.0', port=5001, debug=True)