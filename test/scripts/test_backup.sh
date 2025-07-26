#!/bin/bash

# 测试备份脚本 - 用于调试参数传递问题

echo "=== 测试备份脚本开始 ==="
echo "接收到的参数数量: $#"
echo "所有参数: $@"

# 打印每个参数
i=1
for arg in "$@"; do
    echo "参数 $i: $arg"
    i=$((i + 1))
done

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --db_host)
            DB_HOST="$2"
            shift 2
            ;;
        --db_port)
            DB_PORT="$2"
            shift 2
            ;;
        --db_name)
            DB_NAME="$2"
            shift 2
            ;;
        --db_user)
            DB_USER="$2"
            shift 2
            ;;
        --db_password)
            DB_PASSWORD="$2"
            shift 2
            ;;
        --backup_path)
            BACKUP_PATH="$2"
            shift 2
            ;;
        --retention_days)
            RETENTION_DAYS="$2"
            shift 2
            ;;
        *)
            echo "未知参数: $1"
            shift
            ;;
    esac
done

# 打印解析后的参数
echo ""
echo "=== 解析后的参数 ==="
echo "DB_HOST: ${DB_HOST:-未设置}"
echo "DB_PORT: ${DB_PORT:-未设置}"
echo "DB_NAME: ${DB_NAME:-未设置}"
echo "DB_USER: ${DB_USER:-未设置}"
echo "DB_PASSWORD: ${DB_PASSWORD//?/*}"  # 隐藏密码
echo "BACKUP_PATH: ${BACKUP_PATH:-未设置}"
echo "RETENTION_DAYS: ${RETENTION_DAYS:-未设置}"

# 检查必需参数
if [ -z "$DB_HOST" ] || [ -z "$DB_NAME" ] || [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$BACKUP_PATH" ]; then
    echo ""
    echo "错误: 缺少必需参数"
    echo "用法: $0 --db_host HOST --db_port PORT --db_name NAME --db_user USER --db_password PASS --backup_path PATH [--retention_days DAYS]"
    exit 1
fi

# 设置默认值
DB_PORT=${DB_PORT:-3306}
RETENTION_DAYS=${RETENTION_DAYS:-7}

echo ""
echo "=== 执行备份测试 ==="
echo "创建备份目录: $BACKUP_PATH"
mkdir -p "$BACKUP_PATH" || { echo "创建目录失败"; exit 1; }

# 生成备份文件名
BACKUP_FILE="$BACKUP_PATH/test_backup_$(date +%Y%m%d_%H%M%S).sql"
echo "备份文件: $BACKUP_FILE"

# 模拟备份（实际应该使用mysqldump）
echo "-- 测试备份文件" > "$BACKUP_FILE"
echo "-- 数据库: $DB_NAME" >> "$BACKUP_FILE"
echo "-- 主机: $DB_HOST:$DB_PORT" >> "$BACKUP_FILE"
echo "-- 用户: $DB_USER" >> "$BACKUP_FILE"
echo "-- 时间: $(date)" >> "$BACKUP_FILE"

if [ -f "$BACKUP_FILE" ]; then
    echo "备份成功！"
    ls -la "$BACKUP_FILE"
else
    echo "备份失败！"
    exit 1
fi

echo ""
echo "=== 测试备份脚本完成 ==="
exit 0