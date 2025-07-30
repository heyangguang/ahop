#!/usr/bin/env python3
"""
真实感模拟工单插件 - 生成随机且真实的工单数据
"""

import json
import random
import hashlib
from datetime import datetime, timedelta
from flask import Flask, request, jsonify
from faker import Faker

app = Flask(__name__)
fake = Faker('zh_CN')  # 使用中文数据

# 真实的问题模板
ISSUE_TEMPLATES = {
    "database": {
        "titles": [
            "MySQL主库连接池耗尽",
            "数据库响应时间超过阈值",
            "主从同步延迟严重",
            "数据库死锁频繁发生",
            "查询性能严重下降",
            "数据库连接数异常增长",
            "备份任务执行失败",
            "索引缺失导致全表扫描",
            "数据库内存使用率过高",
            "慢查询日志激增"
        ],
        "descriptions": [
            "监控显示{service}的数据库连接池在{time}达到上限，导致新的请求无法获取连接",
            "数据库查询响应时间从平均{normal_time}ms上升到{current_time}ms，影响{affected_users}用户",
            "主从同步延迟达到{delay}秒，可能导致数据不一致问题",
            "过去{period}内发生了{count}次死锁，主要集中在{table}表",
            "SQL查询 '{query}' 的执行时间从{old_time}秒增加到{new_time}秒",
            "当前连接数{current}，接近最大限制{max}，需要立即处理",
            "{backup_type}备份任务在{time}执行失败，错误信息：{error}",
            "发现{table}表的查询未使用索引，导致{rows}行数据全表扫描",
            "数据库服务器内存使用率达到{percent}%，可能触发OOM",
            "慢查询数量在过去{time}内增长了{percent}%，影响整体性能"
        ]
    },
    "application": {
        "titles": [
            "API接口响应超时",
            "服务启动失败",
            "内存泄漏导致OOM",
            "CPU使用率异常",
            "接口错误率上升",
            "定时任务执行异常",
            "消息队列堆积",
            "缓存命中率下降",
            "文件上传功能故障",
            "第三方服务调用失败"
        ],
        "descriptions": [
            "{api_endpoint}接口的平均响应时间达到{response_time}秒，超过SLA要求",
            "{service}服务在{time}启动失败，错误日志显示：{error_message}",
            "{service}服务内存使用持续增长，已达到{memory}GB，疑似内存泄漏",
            "{service}的CPU使用率持续在{cpu_percent}%以上，影响其他服务",
            "{api_endpoint}接口错误率从{old_rate}%上升到{new_rate}%",
            "定时任务{job_name}在{time}执行失败，已连续失败{fail_count}次",
            "{queue_name}队列当前积压{message_count}条消息，消费速度严重滞后",
            "Redis缓存命中率从{old_hit_rate}%下降到{new_hit_rate}%，数据库压力增大",
            "用户反馈文件上传功能异常，错误信息：{error_detail}",
            "调用{third_party}服务失败率达到{fail_rate}%，影响{feature}功能"
        ]
    },
    "network": {
        "titles": [
            "网络延迟异常",
            "防火墙规则配置错误",
            "负载均衡器故障",
            "DNS解析异常",
            "网络带宽占用过高",
            "SSL证书即将过期",
            "CDN节点异常",
            "网络丢包率上升",
            "VPN连接不稳定",
            "DDoS攻击预警"
        ],
        "descriptions": [
            "{region}区域到{service}的网络延迟从{normal_latency}ms增加到{current_latency}ms",
            "防火墙规则变更后，{service}无法访问{destination}，影响{feature}功能",
            "{lb_name}负载均衡器的{backend_count}个后端节点中有{unhealthy_count}个不健康",
            "域名{domain}的DNS解析出现异常，部分地区用户无法访问",
            "出口带宽使用率达到{bandwidth_percent}%，可能影响服务质量",
            "{domain}的SSL证书将在{days}天后过期，需要及时更新",
            "CDN节点{node}响应异常，影响{region}地区用户访问",
            "网络丢包率达到{packet_loss}%，正常值应低于{threshold}%",
            "VPN连接每{interval}分钟断开一次，影响远程办公",
            "检测到异常流量，疑似DDoS攻击，当前QPS达到{qps}"
        ]
    },
    "security": {
        "titles": [
            "异常登录行为检测",
            "敏感数据泄露风险",
            "安全漏洞扫描告警",
            "权限配置异常",
            "恶意请求拦截",
            "密码策略违规",
            "审计日志异常",
            "加密服务故障",
            "访问控制失效",
            "安全补丁待更新"
        ],
        "descriptions": [
            "检测到账号{account}在{time}内从{location_count}个不同地区登录",
            "日志文件{log_file}中发现{count}条包含敏感信息的记录",
            "安全扫描发现{vulnerability}漏洞，CVSS评分{score}，需要立即修复",
            "{user}账号被赋予了{permission}权限，违反最小权限原则",
            "WAF在过去{time}内拦截了{count}次恶意请求，主要来自{source}",
            "发现{count}个账号使用弱密码，不符合安全策略要求",
            "审计日志服务在{time}停止记录，可能存在安全风险",
            "加密服务{service}响应异常，无法完成{operation}操作",
            "{resource}的访问控制列表配置错误，导致未授权访问",
            "{component}组件存在{patch_count}个待安装的安全补丁"
        ]
    },
    "infrastructure": {
        "titles": [
            "磁盘空间不足",
            "服务器硬件故障",
            "备份系统异常",
            "监控告警风暴",
            "容器编排异常",
            "存储性能下降",
            "集群节点故障",
            "电力供应告警",
            "温度监控异常",
            "虚拟化平台故障"
        ],
        "descriptions": [
            "{server}服务器的{partition}分区使用率达到{usage}%，预计{days}天后耗尽",
            "{server}服务器的{component}组件故障，需要更换硬件",
            "备份任务{backup_job}已连续{days}天失败，最后成功备份时间：{last_success}",
            "监控系统在{time}内产生了{alert_count}条告警，可能存在告警规则问题",
            "Kubernetes集群中{pod_count}个Pod处于{state}状态超过{duration}",
            "存储系统IOPS从{normal_iops}下降到{current_iops}，影响{services}服务",
            "集群节点{node}失去响应，上面运行的{service_count}个服务需要迁移",
            "机房{datacenter}的UPS电量剩余{battery}%，市电供应不稳定",
            "机柜{cabinet}温度达到{temperature}°C，超过安全阈值",
            "vSphere集群{cluster}的{host}主机出现紫屏，影响{vm_count}个虚拟机"
        ]
    }
}

# 真实的服务名称
SERVICES = [
    "order-service", "payment-service", "user-service", "auth-service",
    "inventory-service", "notification-service", "report-service", "analytics-service",
    "search-service", "recommendation-service", "cart-service", "product-service",
    "shipping-service", "customer-service", "marketing-service", "billing-service"
]

# 真实的报告者
REPORTERS = {
    "system": ["监控系统", "告警系统", "Prometheus", "Zabbix", "自动巡检", "健康检查"],
    "team": ["运维团队", "DBA团队", "安全团队", "网络团队", "开发团队", "DevOps团队"],
    "user": ["客服反馈", "用户投诉", "内部用户", "测试团队", "产品经理"],
    "person": []  # 将使用faker生成真实姓名
}

# 工单分配规则
ASSIGNMENT_RULES = {
    "database": ["DBA团队", "数据库管理员", "运维团队"],
    "application": ["开发团队", "应用运维", "DevOps团队", "后端开发组"],
    "network": ["网络团队", "网络工程师", "运维团队"],
    "security": ["安全团队", "安全工程师", "SOC团队"],
    "infrastructure": ["基础设施团队", "系统管理员", "运维团队", "硬件工程师"]
}

def generate_disk_space_ticket(ticket_id):
    """生成磁盘空间不足的专门工单"""
    # 时间生成
    created_delta = timedelta(
        minutes=random.randint(0, 120)  # 最近2小时内的工单
    )
    created_at = datetime.now() - created_delta
    updated_at = created_at + timedelta(minutes=random.randint(0, 30))
    
    # 选择主机
    hosts = [
        "192.168.31.66", "192.168.31.100", "192.168.31.150", "192.168.31.200",
        "192.168.32.10", "192.168.32.20", "192.168.32.30", "192.168.32.40"
    ]
    affected_hosts = random.sample(hosts, k=random.randint(1, 2))
    
    # 磁盘使用率
    disk_usage = random.randint(85, 99)
    partition = random.choice(["/", "/var", "/home", "/data", "/opt", "/tmp"])
    
    # 状态和优先级
    if disk_usage >= 95:
        priority = "critical"
        status = random.choice(["open", "open", "in_progress"])  # 更多open状态
    elif disk_usage >= 90:
        priority = random.choice(["critical", "high"])
        status = random.choice(["open", "in_progress", "in_progress"])
    else:
        priority = "high"
        status = random.choice(["open", "in_progress", "resolved"])
    
    # 标题变体
    title_variants = [
        f"磁盘空间不足 - {affected_hosts[0]} {partition}分区 {disk_usage}%",
        f"紧急：{affected_hosts[0]}服务器磁盘空间不足",
        f"[告警] 磁盘空间不足：{partition} ({disk_usage}%)",
        f"生产环境磁盘告警 - {affected_hosts[0]}",
        f"磁盘空间不足影响服务运行 - {partition}分区"
    ]
    
    title = random.choice(title_variants)
    
    # 描述
    descriptions = [
        f"监控系统发现{affected_hosts[0]}服务器的{partition}分区使用率达到{disk_usage}%，"
        f"剩余空间仅{random.randint(1, 10)}GB，可能影响服务正常运行。请立即处理。",
        
        f"告警详情：\n主机：{affected_hosts[0]}\n分区：{partition}\n"
        f"当前使用率：{disk_usage}%\n剩余空间：{random.randint(100, 5000)}MB\n"
        f"增长速度：每小时{random.randint(100, 500)}MB",
        
        f"紧急告警！{partition}分区空间即将耗尽。\n"
        f"影响主机：{', '.join(affected_hosts)}\n"
        f"当前使用：{disk_usage}%\n"
        f"预计{random.randint(1, 24)}小时内将完全占满。"
    ]
    
    description = random.choice(descriptions)
    
    # 生成唯一ID
    ticket_hash = hashlib.md5(f"DISK-{ticket_id}{created_at}".encode()).hexdigest()[:6].upper()
    
    return {
        "id": f"DISK-{ticket_id:04d}-{ticket_hash}",
        "title": title,
        "description": description,
        "status": status,
        "priority": priority,
        "type": "incident",
        "reporter": random.choice(["监控系统", "Zabbix", "Prometheus", "自动巡检"]),
        "assignee": random.choice(["运维团队", "系统管理员", "基础设施团队"]),
        "category": "infrastructure",
        "service": random.choice(SERVICES),
        "tags": ["磁盘空间", "紧急", "基础设施", "生产环境"],
        "created_at": created_at.strftime("%Y-%m-%d %H:%M:%S"),
        "updated_at": updated_at.strftime("%Y-%m-%d %H:%M:%S"),
        "custom_fields": {
            "environment": "生产环境",
            "affected_components": [partition],
            "affected_hosts": affected_hosts,
            "disk_usage": disk_usage,
            "partition": partition,
            "remaining_space_gb": random.randint(1, 20),
            "growth_rate_mb_per_hour": random.randint(50, 1000),
            "root_cause": "待分析" if status in ["open", "in_progress"] else "日志文件累积",
            "resolution": "无" if status in ["open", "in_progress"] else "已清理日志文件，释放空间",
            "impact_level": "高",
            "sla_deadline": (created_at + timedelta(hours=random.choice([2, 4, 8]))).strftime("%Y-%m-%d %H:%M:%S")
        }
    }

def generate_realistic_ticket_no_disk(ticket_id):
    """生成一个真实感的工单（不包含磁盘空间）"""
    # 排除infrastructure类别中的磁盘相关问题
    categories = [c for c in ISSUE_TEMPLATES.keys() if c != 'infrastructure']
    category = random.choice(categories)
    
    template = ISSUE_TEMPLATES[category]
    
    # 选择标题和描述模板
    title_template = random.choice(template["titles"])
    desc_template = random.choice(template["descriptions"])
    
    # 生成时间
    created_delta = timedelta(
        minutes=random.randint(0, 30)  # 最近30分钟内的工单
    )
    created_at = datetime.now() - created_delta
    
    # 更新时间应该在创建之后
    update_delta = timedelta(
        minutes=random.randint(0, int(created_delta.total_seconds() / 60))
    )
    updated_at = datetime.now() - update_delta
    
    # 根据问题年龄决定状态
    age_minutes = created_delta.total_seconds() / 60
    if age_minutes < 10:
        status = random.choice(['open', 'open', 'in_progress'])
    else:
        status = random.choice(['in_progress', 'resolved'])
    
    # 根据类别决定优先级
    priority_weights = {
        "database": {"critical": 30, "high": 40, "medium": 25, "low": 5},
        "application": {"critical": 20, "high": 35, "medium": 35, "low": 10},
        "network": {"critical": 25, "high": 35, "medium": 30, "low": 10},
        "security": {"critical": 40, "high": 40, "medium": 15, "low": 5}
    }
    
    priority = random.choices(
        list(priority_weights[category].keys()),
        weights=list(priority_weights[category].values())
    )[0]
    
    # 生成描述的参数（简化版）
    desc_params = {
        "service": random.choice(SERVICES),
        "time": fake.time(),
        "affected_users": random.randint(10, 1000),
        "normal_time": random.randint(50, 200),
        "current_time": random.randint(500, 2000),
        "api_endpoint": f"/api/v1/{fake.word()}",
        "error_message": "Connection timeout",
        "region": random.choice(["华北", "华东", "华南"]),
        "domain": fake.domain_name(),
        "account": fake.user_name()
    }
    
    # 生成描述
    description = desc_template
    for key in desc_params:
        if f"{{{key}}}" in description:
            description = description.replace(f"{{{key}}}", str(desc_params[key]))
    
    # 填充其他可能的占位符
    remaining_placeholders = set()
    import re
    for match in re.finditer(r'\{(\w+)\}', description):
        remaining_placeholders.add(match.group(1))
    
    for placeholder in remaining_placeholders:
        description = description.replace(f"{{{placeholder}}}", "N/A")
    
    # 选择报告者
    reporter = random.choice(["监控系统", "告警系统", "用户反馈"])
    
    # 选择处理人
    assignee = random.choice(ASSIGNMENT_RULES.get(category, ["运维团队"]))
    
    # 生成标签
    base_tags = ["生产环境"]
    if priority in ["critical", "high"]:
        base_tags.append("紧急")
    
    # 生成唯一ID
    ticket_hash = hashlib.md5(f"{ticket_id}{created_at}".encode()).hexdigest()[:6].upper()
    
    return {
        "id": f"TICKET-{ticket_id:04d}-{ticket_hash}",
        "title": title_template,
        "description": description,
        "status": status,
        "priority": priority,
        "type": "incident",
        "reporter": reporter,
        "assignee": assignee,
        "category": category,
        "service": random.choice(SERVICES),
        "tags": base_tags,
        "created_at": created_at.strftime("%Y-%m-%d %H:%M:%S"),
        "updated_at": updated_at.strftime("%Y-%m-%d %H:%M:%S"),
        "custom_fields": {
            "environment": "生产环境",
            "affected_components": [random.choice(SERVICES)],
            "affected_hosts": [random.choice(["192.168.31.66", "192.168.31.100"])],
            "impact_level": "中"
        }
    }

def generate_realistic_ticket(ticket_id):
    """生成一个真实感的工单"""
    # 30%概率生成磁盘空间相关工单
    if random.random() < 0.3:
        return generate_disk_space_ticket(ticket_id)
    
    category = random.choice(list(ISSUE_TEMPLATES.keys()))
    template = ISSUE_TEMPLATES[category]
    
    # 选择标题和描述模板
    title_template = random.choice(template["titles"])
    desc_template = random.choice(template["descriptions"])
    
    # 生成时间
    created_delta = timedelta(
        days=random.randint(0, 30),
        hours=random.randint(0, 23),
        minutes=random.randint(0, 59)
    )
    created_at = datetime.now() - created_delta
    
    # 更新时间应该在创建之后
    update_delta = timedelta(
        hours=random.randint(0, int(created_delta.total_seconds() / 3600)),
        minutes=random.randint(0, 59)
    )
    updated_at = datetime.now() - update_delta
    
    # 根据问题年龄决定状态
    age_hours = created_delta.total_seconds() / 3600
    if age_hours < 2:
        status_weights = {"open": 70, "in_progress": 30, "resolved": 0, "closed": 0}
    elif age_hours < 24:
        status_weights = {"open": 30, "in_progress": 50, "resolved": 15, "closed": 5}
    elif age_hours < 72:
        status_weights = {"open": 10, "in_progress": 30, "resolved": 40, "closed": 20}
    else:
        status_weights = {"open": 5, "in_progress": 10, "resolved": 35, "closed": 50}
    
    status = random.choices(
        list(status_weights.keys()),
        weights=list(status_weights.values())
    )[0]
    
    # 根据类别决定优先级
    priority_weights = {
        "database": {"critical": 30, "high": 40, "medium": 25, "low": 5},
        "application": {"critical": 20, "high": 35, "medium": 35, "low": 10},
        "network": {"critical": 25, "high": 35, "medium": 30, "low": 10},
        "security": {"critical": 40, "high": 40, "medium": 15, "low": 5},
        "infrastructure": {"critical": 35, "high": 30, "medium": 25, "low": 10}
    }
    
    priority = random.choices(
        list(priority_weights[category].keys()),
        weights=list(priority_weights[category].values())
    )[0]
    
    # 生成描述的参数
    desc_params = {
        "service": random.choice(SERVICES),
        "time": fake.time(),
        "affected_users": random.randint(10, 10000),
        "normal_time": random.randint(50, 200),
        "current_time": random.randint(500, 5000),
        "delay": random.randint(1, 300),
        "period": f"{random.randint(1, 24)}小时",
        "count": random.randint(5, 100),
        "table": fake.word() + "_table",
        "query": f"SELECT * FROM {fake.word()} WHERE {fake.word()} = ?",
        "old_time": round(random.uniform(0.1, 1.0), 2),
        "new_time": round(random.uniform(2.0, 10.0), 2),
        "current": random.randint(100, 900),
        "max": 1000,
        "backup_type": random.choice(["全量", "增量", "差异"]),
        "error": fake.sentence(),
        "rows": random.randint(10000, 1000000),
        "percent": random.randint(70, 99),
        "api_endpoint": f"/api/v1/{fake.word()}/{fake.word()}",
        "response_time": round(random.uniform(3.0, 30.0), 1),
        "error_message": fake.sentence(),
        "memory": round(random.uniform(4.0, 32.0), 1),
        "cpu_percent": random.randint(80, 100),
        "old_rate": round(random.uniform(0.1, 1.0), 2),
        "new_rate": round(random.uniform(5.0, 20.0), 2),
        "job_name": fake.word() + "_job",
        "fail_count": random.randint(2, 10),
        "queue_name": fake.word() + "_queue",
        "message_count": random.randint(1000, 100000),
        "old_hit_rate": random.randint(85, 95),
        "new_hit_rate": random.randint(30, 60),
        "error_detail": fake.sentence(),
        "third_party": random.choice(["支付网关", "短信服务", "邮件服务", "地图API"]),
        "fail_rate": random.randint(10, 80),
        "feature": random.choice(["支付", "登录", "下单", "查询"]),
        "region": random.choice(["华北", "华东", "华南", "西南", "西北"]),
        "normal_latency": random.randint(10, 50),
        "current_latency": random.randint(100, 1000),
        "destination": fake.ipv4(),
        "lb_name": f"lb-{fake.word()}",
        "backend_count": random.randint(3, 10),
        "unhealthy_count": random.randint(1, 5),
        "domain": fake.domain_name(),
        "bandwidth_percent": random.randint(80, 98),
        "days": random.randint(1, 30),
        "node": f"cdn-{fake.city()}-{random.randint(1, 10)}",
        "packet_loss": round(random.uniform(1.0, 10.0), 2),
        "threshold": 0.5,
        "interval": random.randint(5, 30),
        "qps": random.randint(10000, 1000000),
        "account": fake.user_name(),
        "location_count": random.randint(3, 20),
        "log_file": f"/var/log/{fake.word()}.log",
        "vulnerability": random.choice(["SQL注入", "XSS", "CSRF", "XXE", "反序列化"]),
        "score": round(random.uniform(4.0, 9.9), 1),
        "user": fake.user_name(),
        "permission": random.choice(["root", "admin", "sudo", "write-all"]),
        "source": fake.ipv4(),
        "operation": random.choice(["加密", "解密", "签名", "验证"]),
        "resource": f"/{fake.word()}/{fake.word()}",
        "component": random.choice(["nginx", "apache", "mysql", "redis", "docker"]),
        "patch_count": random.randint(1, 20),
        "server": f"server-{random.randint(1, 100)}",
        "partition": random.choice(["/", "/var", "/home", "/data", "/opt"]),
        "usage": random.randint(85, 99),
        "backup_job": f"backup_{fake.word()}",
        "last_success": (datetime.now() - timedelta(days=random.randint(1, 30))).strftime("%Y-%m-%d"),
        "alert_count": random.randint(100, 10000),
        "pod_count": random.randint(1, 50),
        "state": random.choice(["Pending", "CrashLoopBackOff", "ImagePullBackOff", "Evicted"]),
        "duration": f"{random.randint(10, 120)}分钟",
        "normal_iops": random.randint(5000, 20000),
        "current_iops": random.randint(100, 2000),
        "services": f"{random.randint(3, 10)}个",
        "service_count": random.randint(5, 20),
        "datacenter": f"DC{random.randint(1, 3)}",
        "battery": random.randint(10, 50),
        "cabinet": f"A{random.randint(1, 10)}-{random.randint(1, 20)}",
        "temperature": random.randint(35, 50),
        "cluster": f"cluster-{fake.word()}",
        "host": f"esxi-{random.randint(1, 20)}",
        "vm_count": random.randint(10, 50)
    }
    
    # 生成描述
    description = desc_template
    for key, value in desc_params.items():
        description = description.replace(f"{{{key}}}", str(value))
    
    # 选择报告者
    reporter_type = random.choice(list(REPORTERS.keys()))
    if reporter_type == "person":
        reporter = fake.name()
    else:
        reporter = random.choice(REPORTERS[reporter_type])
    
    # 选择处理人
    assignee = random.choice(ASSIGNMENT_RULES[category])
    # 有时候加上具体的人名
    if random.random() > 0.5:
        assignee = f"{assignee} - {fake.name()}"
    
    # 生成标签
    base_tags = []
    
    # 环境标签
    env_tag = random.choice(["生产环境", "预发布环境", "测试环境", "开发环境"])
    base_tags.append(env_tag)
    
    # 紧急程度标签
    if priority in ["critical", "high"]:
        base_tags.append(random.choice(["紧急", "严重", "立即处理"]))
    
    # 类别相关标签
    category_tags = {
        "database": ["数据库", "MySQL", "PostgreSQL", "Redis", "MongoDB"],
        "application": ["应用", "API", "微服务", "性能", "BUG"],
        "network": ["网络", "延迟", "连接", "带宽", "路由"],
        "security": ["安全", "漏洞", "入侵", "合规", "审计"],
        "infrastructure": ["基础设施", "硬件", "存储", "计算", "容器"]
    }
    base_tags.extend(random.sample(category_tags[category], k=random.randint(1, 2)))
    
    # 影响范围标签
    if random.random() > 0.5:
        base_tags.append(random.choice(["影响用户", "影响业务", "数据风险", "服务降级"]))
    
    # 工单类型
    types = {
        "database": ["incident", "problem"],
        "application": ["incident", "problem", "change"],
        "network": ["incident", "problem", "change"],
        "security": ["incident", "security_incident"],
        "infrastructure": ["incident", "problem", "change", "maintenance"]
    }
    ticket_type = random.choice(types.get(category, ["incident"]))
    
    # 生成唯一ID
    ticket_hash = hashlib.md5(f"{ticket_id}{created_at}".encode()).hexdigest()[:6].upper()
    
    return {
        "id": f"TICKET-{ticket_id:04d}-{ticket_hash}",
        "title": title_template,
        "description": description,
        "status": status,
        "priority": priority,
        "type": ticket_type,
        "reporter": reporter,
        "assignee": assignee,
        "category": category,
        "service": random.choice(SERVICES),
        "tags": base_tags,
        "created_at": created_at.strftime("%Y-%m-%d %H:%M:%S"),
        "updated_at": updated_at.strftime("%Y-%m-%d %H:%M:%S"),
        "custom_fields": {
            "environment": env_tag,
            "affected_components": random.sample(SERVICES, k=random.randint(1, 3)),
            "affected_hosts": random.sample([
                "192.168.31.66", "192.168.31.100", "192.168.31.150", "192.168.31.200",
                "192.168.32.10", "192.168.32.20", "192.168.32.30", "192.168.32.40",
                "10.0.1.10", "10.0.1.20", "10.0.1.30", "10.0.1.40",
                "172.16.1.10", "172.16.1.20", "172.16.1.30", "172.16.1.40"
            ], k=random.randint(1, 3)),
            "root_cause": "待分析" if status in ["open", "in_progress"] else fake.sentence(),
            "resolution": "无" if status in ["open", "in_progress"] else fake.paragraph(nb_sentences=3),
            "impact_level": random.choice(["高", "中", "低"]),
            "sla_deadline": (created_at + timedelta(hours=random.choice([4, 8, 24, 72]))).strftime("%Y-%m-%d %H:%M:%S")
        }
    }

# 生成初始工单池
TICKET_POOL = []
# 记录最后一次生成硬盘工单的时间
LAST_DISK_TICKET_TIME = None

def refresh_ticket_pool():
    """刷新工单池，模拟实时数据"""
    global TICKET_POOL, LAST_DISK_TICKET_TIME
    
    # 保留最近的工单
    now = datetime.now()
    TICKET_POOL = [
        t for t in TICKET_POOL 
        if (now - datetime.strptime(t['updated_at'], "%Y-%m-%d %H:%M:%S")).days < 7
    ]
    
    # 控制工单生成频率
    # 检查是否应该生成硬盘空间工单（每分钟最多一条）
    should_generate_disk = False
    if LAST_DISK_TICKET_TIME is None:
        should_generate_disk = True
    else:
        time_since_last_disk = (now - LAST_DISK_TICKET_TIME).total_seconds()
        if time_since_last_disk >= 60:  # 至少间隔60秒
            should_generate_disk = True
    
    # 保持合理的工单数量（20-30个）
    current_count = len(TICKET_POOL)
    if current_count < 20:
        # 生成1-3个新工单
        new_tickets_count = random.randint(1, 3)
        start_id = max([int(t['id'].split('-')[1]) for t in TICKET_POOL] + [0]) + 1
        
        for i in range(new_tickets_count):
            if should_generate_disk and i == 0:
                # 生成硬盘空间工单
                TICKET_POOL.append(generate_disk_space_ticket(start_id + i))
                LAST_DISK_TICKET_TIME = now
                should_generate_disk = False
            else:
                # 生成其他类型工单（禁用随机硬盘工单）
                ticket = generate_realistic_ticket_no_disk(start_id + i)
                TICKET_POOL.append(ticket)
    
    # 随机更新一些现有工单
    if len(TICKET_POOL) > 0:
        update_count = min(3, len(TICKET_POOL))
        for ticket in random.sample(TICKET_POOL, k=update_count):
            ticket['updated_at'] = now.strftime("%Y-%m-%d %H:%M:%S")
            if ticket['status'] == 'open' and random.random() > 0.5:
                ticket['status'] = 'in_progress'
            elif ticket['status'] == 'in_progress' and random.random() > 0.7:
                ticket['status'] = 'resolved'

# 初始化工单池（生成少量初始工单）
for i in range(10):
    TICKET_POOL.append(generate_realistic_ticket_no_disk(i + 1))

@app.route('/tickets', methods=['GET'])
def get_tickets():
    """获取工单列表"""
    # 每次请求都检查是否需要刷新工单池
    refresh_ticket_pool()
    
    # 获取查询参数
    minutes = request.args.get('minutes', type=int)
    status = request.args.get('status')
    priority = request.args.get('priority')
    category = request.args.get('category')
    
    # 过滤工单
    filtered_tickets = TICKET_POOL.copy()
    
    if minutes:
        cutoff_time = datetime.now() - timedelta(minutes=minutes)
        filtered_tickets = [
            t for t in filtered_tickets
            if datetime.strptime(t['updated_at'], "%Y-%m-%d %H:%M:%S") >= cutoff_time
        ]
    
    if status:
        filtered_tickets = [t for t in filtered_tickets if t['status'] == status]
    
    if priority:
        filtered_tickets = [t for t in filtered_tickets if t['priority'] == priority]
    
    if category:
        filtered_tickets = [t for t in filtered_tickets if t['category'] == category]
    
    # 按更新时间排序
    filtered_tickets.sort(key=lambda x: x['updated_at'], reverse=True)
    
    return jsonify({
        "success": True,
        "data": filtered_tickets[:50],  # 最多返回50条
        "total": len(filtered_tickets),
        "timestamp": datetime.now().isoformat()
    })

@app.route('/tickets/<ticket_id>', methods=['PUT'])
def update_ticket(ticket_id):
    """更新工单（状态、评论等）- 测试模式，接受任何工单ID"""
    data = request.get_json()
    
    print(f"\n{'='*60}")
    print(f"[{datetime.now().strftime('%Y-%m-%d %H:%M:%S')}] 收到工单更新请求")
    print(f"工单ID: {ticket_id}")
    print(f"请求数据: {json.dumps(data, indent=2, ensure_ascii=False)}")
    print(f"{'='*60}\n")
    
    if not data:
        return jsonify({
            "success": False,
            "message": "Missing request body"
        }), 400
    
    # 查找工单（如果存在则更新，不存在也不报错）
    ticket = None
    for t in TICKET_POOL:
        if t['id'] == ticket_id:
            ticket = t
            break
    
    updated_fields = []
    
    # 更新状态
    if 'status' in data:
        if ticket:
            ticket['status'] = data['status']
        updated_fields.append('status')
        print(f"✓ 更新状态: {data['status']}")
    
    # 添加评论
    if 'comment' in data:
        comment_text = ''
        if isinstance(data['comment'], dict):
            # 处理结构化评论
            comment_text = data['comment'].get('template', '')
            comment_type = data['comment'].get('type', 'general')
            include_logs = data['comment'].get('include_logs', False)
            print(f"收到 {comment_type} 类型的评论")
        else:
            # 处理简单字符串评论
            comment_text = str(data['comment'])
        
        if ticket and comment_text:
            if 'comments' not in ticket:
                ticket['comments'] = []
            ticket['comments'].append({
                'content': comment_text,
                'created_at': datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
                'author': 'AHOP System'
            })
        updated_fields.append('comment')
        preview = comment_text[:50] if len(comment_text) > 50 else comment_text
        print(f"✓ 添加评论: {preview}...")
    
    # 更新自定义字段
    if 'custom_fields' in data:
        if ticket:
            if 'custom_fields' not in ticket:
                ticket['custom_fields'] = {}
            ticket['custom_fields'].update(data['custom_fields'])
        updated_fields.extend(data['custom_fields'].keys())
        print(f"✓ 更新自定义字段: {list(data['custom_fields'].keys())}")
    
    # 更新修改时间
    updated_at = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    if ticket:
        ticket['updated_at'] = updated_at
    
    # 总是返回成功（测试模式）
    response_data = {
        "success": True,
        "message": "Ticket updated successfully (test mode)",
        "data": {
            "external_id": ticket_id,
            "updated_fields": updated_fields,
            "updated_at": updated_at,
            "ticket_exists": ticket is not None
        }
    }
    
    print(f"\n响应数据: {json.dumps(response_data, indent=2, ensure_ascii=False)}")
    print(f"{'='*60}\n")
    
    return jsonify(response_data)

@app.route('/tickets/<ticket_id>/comment', methods=['POST'])
def add_comment(ticket_id):
    """添加工单评论"""
    data = request.get_json()
    
    # 查找并更新工单
    for ticket in TICKET_POOL:
        if ticket['id'] == ticket_id:
            ticket['updated_at'] = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
            if 'status' in data and data['status']:
                ticket['status'] = data['status']
            break
    
    return jsonify({
        "success": True,
        "message": "Comment added successfully"
    })

@app.route('/health', methods=['GET'])
def health_check():
    """健康检查"""
    return jsonify({
        "status": "healthy",
        "service": "Realistic Mock Ticket System",
        "version": "3.0",
        "timestamp": datetime.now().isoformat(),
        "tickets_in_pool": len(TICKET_POOL)
    })

@app.route('/generate-test-tickets', methods=['POST'])
def generate_test_tickets():
    """生成测试工单 - 用于自愈规则测试"""
    global LAST_DISK_TICKET_TIME
    
    data = request.get_json() or {}
    count = data.get('count', 10)
    ticket_type = data.get('type', 'mixed')  # disk, mixed
    
    new_tickets = []
    start_id = max([int(t['id'].split('-')[1]) for t in TICKET_POOL] + [1000]) + 1
    
    now = datetime.now()
    disk_tickets_generated = 0
    
    for i in range(count):
        if ticket_type == 'disk':
            # 检查是否可以生成硬盘工单
            if LAST_DISK_TICKET_TIME is None or (now - LAST_DISK_TICKET_TIME).total_seconds() >= 60:
                ticket = generate_disk_space_ticket(start_id + i)
                LAST_DISK_TICKET_TIME = now
                disk_tickets_generated += 1
            else:
                # 如果不能生成硬盘工单，生成其他类型
                ticket = generate_realistic_ticket_no_disk(start_id + i)
        elif ticket_type == 'mixed':
            # 混合模式：优先生成一个硬盘工单（如果可以），其余为其他类型
            if disk_tickets_generated == 0 and (LAST_DISK_TICKET_TIME is None or (now - LAST_DISK_TICKET_TIME).total_seconds() >= 60):
                ticket = generate_disk_space_ticket(start_id + i)
                LAST_DISK_TICKET_TIME = now
                disk_tickets_generated += 1
            else:
                ticket = generate_realistic_ticket_no_disk(start_id + i)
        else:
            ticket = generate_realistic_ticket_no_disk(start_id + i)
        
        # 确保最近的工单
        ticket['created_at'] = (datetime.now() - timedelta(minutes=random.randint(0, 30))).strftime("%Y-%m-%d %H:%M:%S")
        ticket['updated_at'] = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        
        new_tickets.append(ticket)
        TICKET_POOL.append(ticket)
    
    # 限制工单池大小
    if len(TICKET_POOL) > 50:
        TICKET_POOL[:] = TICKET_POOL[-50:]
    
    return jsonify({
        "success": True,
        "message": f"Generated {count} test tickets (disk tickets: {disk_tickets_generated})",
        "tickets": new_tickets
    })

@app.route('/stats', methods=['GET'])
def get_stats():
    """获取工单统计信息"""
    stats = {
        "total": len(TICKET_POOL),
        "by_status": {},
        "by_priority": {},
        "by_category": {},
        "recent_24h": 0
    }
    
    cutoff_24h = datetime.now() - timedelta(hours=24)
    
    for ticket in TICKET_POOL:
        # 状态统计
        status = ticket['status']
        stats['by_status'][status] = stats['by_status'].get(status, 0) + 1
        
        # 优先级统计
        priority = ticket['priority']
        stats['by_priority'][priority] = stats['by_priority'].get(priority, 0) + 1
        
        # 类别统计
        category = ticket['category']
        stats['by_category'][category] = stats['by_category'].get(category, 0) + 1
        
        # 24小时内的工单
        if datetime.strptime(ticket['created_at'], "%Y-%m-%d %H:%M:%S") >= cutoff_24h:
            stats['recent_24h'] += 1
    
    return jsonify({
        "success": True,
        "data": stats,
        "timestamp": datetime.now().isoformat()
    })

if __name__ == '__main__':
    print("真实感模拟工单系统启动中...")
    print(f"初始工单池包含 {len(TICKET_POOL)} 个工单")
    print("\n访问地址: http://localhost:5002")
    print("\n特性:")
    print("- 真实的问题场景和描述")
    print("- 动态工单生成和更新")
    print("- 支持多维度过滤")
    print("- 包含自定义字段")
    print("\n可用端点:")
    print("  GET  /tickets - 获取工单列表")
    print("  GET  /tickets?minutes=60&status=open&priority=high")
    print("  GET  /stats - 查看统计信息")
    print("  PUT  /tickets/{id} - 更新工单（状态、评论、自定义字段）")
    print("  POST /tickets/{id}/comment - 添加评论")
    print("  POST /generate-test-tickets - 生成测试工单")
    print("       参数: {\"count\": 20, \"type\": \"disk\"} # disk/mixed")
    print("  GET  /health - 健康检查")
    
    app.run(host='0.0.0.0', port=5002, debug=True)