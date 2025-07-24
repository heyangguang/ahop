#!/bin/bash

# 数据库重置脚本
# 用于清空并重新创建数据库

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 从 .env 文件读取数据库配置
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# 数据库配置
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-Admin@123}"
DB_NAME="${DB_NAME:-auto_healing_platform}"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}🔄 数据库重置脚本${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

echo -e "${YELLOW}数据库配置：${NC}"
echo "Host: $DB_HOST"
echo "Port: $DB_PORT"
echo "Database: $DB_NAME"
echo "User: $DB_USER"
echo ""

echo -e "${RED}⚠️  警告：这将删除所有数据！${NC}"
echo -n "确定要继续吗？(y/N): "
read -r confirm

if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
    echo -e "${YELLOW}操作已取消${NC}"
    exit 0
fi

echo ""
echo -e "${YELLOW}▶ 步骤 1: 删除现有数据库${NC}"

# 使用 PGPASSWORD 环境变量传递密码
export PGPASSWORD="$DB_PASSWORD"

# 终止所有连接到该数据库的会话
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "SELECT pg_terminate_backend(pg_stat_activity.pid) FROM pg_stat_activity WHERE pg_stat_activity.datname = '$DB_NAME' AND pid <> pg_backend_pid();" 2>/dev/null

# 删除数据库
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "DROP DATABASE IF EXISTS $DB_NAME;"

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ 数据库已删除${NC}"
else
    echo -e "${RED}✗ 删除数据库失败${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}▶ 步骤 2: 创建新数据库${NC}"

# 创建新数据库
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "CREATE DATABASE $DB_NAME;"

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ 数据库已创建${NC}"
else
    echo -e "${RED}✗ 创建数据库失败${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}▶ 步骤 3: 启动应用程序（自动执行迁移）${NC}"
echo ""

# 提示用户启动应用
echo -e "${GREEN}数据库已重置！${NC}"
echo ""
echo "请按以下步骤操作："
echo ""
echo "1. 启动应用程序（会自动创建表和初始数据）："
echo -e "${BLUE}   go run cmd/server/main.go${NC}"
echo ""
echo "2. 应用启动后会自动创建："
echo "   - 默认租户"
echo "   - 所有权限"
echo "   - 平台管理员角色"
echo "   - 默认管理员账号 (admin/Admin@123)"
echo ""
echo "3. （可选）创建额外的测试数据："
echo -e "${BLUE}   bash scripts/init_data.sh${NC}"

# 清理环境变量
unset PGPASSWORD