#!/bin/bash
# Name: 参数类型测试脚本
# Description: 演示所有支持的参数类型定义方式
#
# 明确指定类型的参数
# @param HOST_NAME [text] 主机名称 (required)
# @param HOST_PORT [integer] 服务端口号 (required)
# @param ADMIN_PASSWORD [password] 管理员密码 (required)
# @param ENABLE_SSL [select] 是否启用SSL (yes/no)
# @param LOG_LEVEL [select] 日志级别 (debug/info/warn/error)
# @param ENABLED_FEATURES [multiselect] 启用的功能模块 (auth/cache/logging/monitoring/backup)
# @param CONFIG_CONTENT [textarea] 自定义配置内容
# @param TIMEOUT_SECONDS [integer] 超时时间（秒）
# @param MEMORY_LIMIT [float] 内存限制（GB）
#
# 使用智能推断的参数（作为对比）
# @param BACKUP_PASSWORD 备份密码
# @param RETENTION_DAYS 日志保留天数
# @param ENVIRONMENT 运行环境（dev/test/prod）

# 参数解析
echo "=== 参数类型测试脚本 ==="
echo "接收到的参数："

while [[ $# -gt 0 ]]; do
    case $1 in
        --HOST_NAME)
            echo "主机名称: $2 [text]"
            shift 2
            ;;
        --HOST_PORT)
            echo "服务端口: $2 [integer]"
            shift 2
            ;;
        --ADMIN_PASSWORD)
            echo "管理员密码: *** [password - 已隐藏]"
            shift 2
            ;;
        --ENABLE_SSL)
            echo "启用SSL: $2 [select]"
            shift 2
            ;;
        --LOG_LEVEL)
            echo "日志级别: $2 [select]"
            shift 2
            ;;
        --ENABLED_FEATURES)
            echo "启用的功能: $2 [multiselect]"
            shift 2
            ;;
        --CONFIG_CONTENT)
            echo "配置内容: [textarea]"
            echo "$2"
            shift 2
            ;;
        --TIMEOUT_SECONDS)
            echo "超时时间: $2 秒 [integer]"
            shift 2
            ;;
        --MEMORY_LIMIT)
            echo "内存限制: $2 GB [float]"
            shift 2
            ;;
        --BACKUP_PASSWORD)
            echo "备份密码: *** [password - 智能推断]"
            shift 2
            ;;
        --RETENTION_DAYS)
            echo "保留天数: $2 [integer - 智能推断]"
            shift 2
            ;;
        --ENVIRONMENT)
            echo "运行环境: $2 [select - 智能推断]"
            shift 2
            ;;
        *)
            echo "未知参数: $1"
            shift
            ;;
    esac
done

echo ""
echo "脚本执行完成！"