#!/bin/bash
# MySQL数据库备份脚本
# 用于备份MySQL数据库到指定位置

# @param db_host 数据库主机地址 (required)
# @param db_port 数据库端口
# @param db_name 数据库名称 (required)
# @param db_user 数据库用户名 (required)
# @param db_password 数据库密码 (required)
# @param backup_path 备份文件保存路径 (required)
# @param retention_days 备份保留天数

# 设置错误时退出
set -e

# 初始化变量
DB_HOST=""
DB_PORT="3306"
DB_NAME=""
DB_USER=""
DB_PASSWORD=""
BACKUP_PATH=""
RETENTION_DAYS="7"

# 解析命令行参数
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
            exit 1
            ;;
    esac
done

# 验证必填参数
if [ -z "$DB_HOST" ]; then
    echo "错误: 必须指定数据库主机地址 (--db_host)"
    exit 1
fi

if [ -z "$DB_NAME" ]; then
    echo "错误: 必须指定数据库名称 (--db_name)"
    exit 1
fi

if [ -z "$DB_USER" ]; then
    echo "错误: 必须指定数据库用户名 (--db_user)"
    exit 1
fi

if [ -z "$DB_PASSWORD" ]; then
    echo "错误: 必须指定数据库密码 (--db_password)"
    exit 1
fi

if [ -z "$BACKUP_PATH" ]; then
    echo "错误: 必须指定备份路径 (--backup_path)"
    exit 1
fi

# 创建备份目录
echo "=== 开始数据库备份 ==="
echo "主机: $DB_HOST:$DB_PORT"
echo "数据库: $DB_NAME"
echo "用户: $DB_USER"
echo "备份路径: $BACKUP_PATH"
echo "保留天数: $RETENTION_DAYS"

# 确保备份目录存在
mkdir -p "$BACKUP_PATH"

# 生成备份文件名
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_PATH/${DB_NAME}_${TIMESTAMP}.sql"
BACKUP_GZ="${BACKUP_FILE}.gz"

# 执行备份
echo ""
echo "正在备份数据库..."
export MYSQL_PWD="$DB_PASSWORD"

if mysqldump -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" \
    --single-transaction \
    --routines \
    --triggers \
    --events \
    "$DB_NAME" > "$BACKUP_FILE"; then
    echo "✓ 数据库导出成功"
else
    echo "✗ 数据库导出失败"
    exit 1
fi

# 压缩备份文件
echo "正在压缩备份文件..."
if gzip "$BACKUP_FILE"; then
    echo "✓ 压缩完成: $BACKUP_GZ"
    echo "  文件大小: $(du -h "$BACKUP_GZ" | cut -f1)"
else
    echo "✗ 压缩失败"
    exit 1
fi

# 清理旧备份
echo ""
echo "清理超过 $RETENTION_DAYS 天的旧备份..."
find "$BACKUP_PATH" -name "${DB_NAME}_*.sql.gz" -mtime +$RETENTION_DAYS -type f | while read old_backup; do
    echo "  删除: $(basename "$old_backup")"
    rm -f "$old_backup"
done

# 列出当前所有备份
echo ""
echo "当前备份列表:"
ls -lh "$BACKUP_PATH/${DB_NAME}_"*.sql.gz 2>/dev/null | tail -5 || echo "  (无备份文件)"

echo ""
echo "=== 备份完成 ==="
echo "备份文件: $BACKUP_GZ"

# 可选：验证备份文件
if [ -f "$BACKUP_GZ" ]; then
    echo "✓ 备份验证通过"
    exit 0
else
    echo "✗ 备份验证失败：文件不存在"
    exit 1
fi