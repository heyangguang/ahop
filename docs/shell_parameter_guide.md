# Shell脚本参数定义指南

## 概述

AHOP平台支持通过在Shell脚本中添加特定格式的注释来定义参数。这些参数会在执行任务时通过Web界面收集用户输入，并以命令行参数的形式传递给脚本。

## 参数定义格式

### 基本语法

在Shell脚本的开头部分（前100行内），使用 `@param` 注释定义参数：

```bash
# @param 参数名 参数描述
```

### 必填参数

在描述末尾添加 `(required)` 标记：

```bash
# @param 参数名 参数描述 (required)
```

### 示例

```bash
#!/bin/bash
# 系统备份脚本
# 用于备份系统文件和数据库

# @param backup_type 备份类型（full/incremental） (required)
# @param target_path 备份文件保存路径 (required)
# @param retention_days 备份保留天数
# @param compress 是否压缩备份文件
```

## 参数类型推断

扫描器会根据参数描述自动推断参数类型：

### 1. 密码类型 (password)
描述中包含"密码"或"password"：
```bash
# @param db_password 数据库密码 (required)
# @param ssh_password SSH登录密码
# @param api_secret API密钥
```

### 2. 整数类型 (integer)
描述中包含"端口"、"天数"、"数字"、"number"：
```bash
# @param port 服务端口
# @param retention_days 日志保留天数
# @param max_connections 最大连接数
# @param timeout 超时时间（秒）
```

### 3. 选择类型 (multiplechoice)
描述中包含"类型"且有"/"分隔的选项（用括号包围）：
```bash
# @param log_level 日志级别（debug/info/warn/error） (required)
# @param environment 运行环境（dev/test/prod）
# @param backup_type 备份类型（full/incremental/differential）
```

### 4. 文本类型 (text) - 默认
其他所有情况：
```bash
# @param hostname 主机名 (required)
# @param config_file 配置文件路径
# @param description 任务描述
```

## 脚本参数接收

### 命令行参数格式

参数会以 `--参数名 值` 的形式传递：
```bash
./script.sh --backup_type full --target_path /backup --retention_days 7
```

### 参数解析示例

```bash
#!/bin/bash
# 数据库备份脚本

# @param db_host 数据库主机地址 (required)
# @param db_port 数据库端口
# @param db_name 数据库名称 (required)
# @param db_user 数据库用户名 (required)
# @param db_password 数据库密码 (required)
# @param backup_path 备份保存路径 (required)
# @param compress 是否压缩（yes/no）

# 初始化变量
DB_HOST=""
DB_PORT="3306"  # 默认值
DB_NAME=""
DB_USER=""
DB_PASSWORD=""
BACKUP_PATH=""
COMPRESS="yes"  # 默认值

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
        --compress)
            COMPRESS="$2"
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

# ... 其他必填参数验证 ...

# 执行备份逻辑
echo "开始备份数据库..."
echo "主机: $DB_HOST:$DB_PORT"
echo "数据库: $DB_NAME"
echo "备份路径: $BACKUP_PATH"
```

## 完整示例

### 示例1：系统健康检查脚本

```bash
#!/bin/bash
# System Health Check Script
# 系统健康状态检查脚本

# @param check_type 检查类型（disk/memory/cpu/network/all） (required)
# @param threshold 告警阈值（百分比）
# @param output_format 输出格式（text/json/html）
# @param email_alert 告警邮箱地址

# 参数默认值
CHECK_TYPE=""
THRESHOLD="80"
OUTPUT_FORMAT="text"
EMAIL_ALERT=""

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --check_type)
            CHECK_TYPE="$2"
            shift 2
            ;;
        --threshold)
            THRESHOLD="$2"
            shift 2
            ;;
        --output_format)
            OUTPUT_FORMAT="$2"
            shift 2
            ;;
        --email_alert)
            EMAIL_ALERT="$2"
            shift 2
            ;;
        *)
            echo "未知参数: $1"
            echo "使用方法: $0 --check_type <type> [options]"
            exit 1
            ;;
    esac
done

# 参数验证
if [ -z "$CHECK_TYPE" ]; then
    echo "错误: 必须指定检查类型 (--check_type)"
    echo "可选值: disk, memory, cpu, network, all"
    exit 1
fi

# 执行检查
case $CHECK_TYPE in
    disk)
        check_disk_usage
        ;;
    memory)
        check_memory_usage
        ;;
    cpu)
        check_cpu_usage
        ;;
    network)
        check_network_status
        ;;
    all)
        check_disk_usage
        check_memory_usage
        check_cpu_usage
        check_network_status
        ;;
    *)
        echo "错误: 无效的检查类型 '$CHECK_TYPE'"
        exit 1
        ;;
esac
```

### 示例2：应用部署脚本

```bash
#!/bin/bash
# Application Deployment Script
# 应用自动化部署脚本

# @param app_name 应用名称 (required)
# @param app_version 应用版本 (required)
# @param deploy_env 部署环境（dev/test/staging/prod） (required)
# @param target_host 目标服务器地址 (required)
# @param target_port SSH端口
# @param target_user SSH用户名
# @param deploy_path 部署路径
# @param backup_old 是否备份旧版本（yes/no）
# @param health_check_url 健康检查URL
# @param rollback_on_failure 失败时是否回滚（yes/no）

# 变量初始化
APP_NAME=""
APP_VERSION=""
DEPLOY_ENV=""
TARGET_HOST=""
TARGET_PORT="22"
TARGET_USER="deploy"
DEPLOY_PATH="/opt/apps"
BACKUP_OLD="yes"
HEALTH_CHECK_URL=""
ROLLBACK_ON_FAILURE="yes"

# 解析参数（省略）...

# 参数验证
if [ -z "$APP_NAME" ] || [ -z "$APP_VERSION" ] || [ -z "$DEPLOY_ENV" ] || [ -z "$TARGET_HOST" ]; then
    echo "错误: 缺少必填参数"
    echo "必填参数: --app_name, --app_version, --deploy_env, --target_host"
    exit 1
fi

# 根据环境设置不同配置
case $DEPLOY_ENV in
    dev)
        CONFIG_FILE="config.dev.yml"
        ;;
    test)
        CONFIG_FILE="config.test.yml"
        ;;
    staging)
        CONFIG_FILE="config.staging.yml"
        ;;
    prod)
        CONFIG_FILE="config.prod.yml"
        BACKUP_OLD="yes"  # 生产环境强制备份
        ;;
esac

echo "=== 开始部署应用 ==="
echo "应用: $APP_NAME v$APP_VERSION"
echo "环境: $DEPLOY_ENV"
echo "目标: $TARGET_USER@$TARGET_HOST:$TARGET_PORT"
echo "路径: $DEPLOY_PATH/$APP_NAME"
```

### 示例3：日志清理脚本

```bash
#!/bin/bash
# Log Cleanup Script  
# 日志文件清理脚本

# @param log_dir 日志目录路径 (required)
# @param retention_days 保留天数 (required)
# @param file_pattern 文件匹配模式
# @param archive 是否归档（yes/no）
# @param archive_path 归档保存路径
# @param dry_run 测试模式（yes/no）

LOG_DIR=""
RETENTION_DAYS=""
FILE_PATTERN="*.log"
ARCHIVE="no"
ARCHIVE_PATH="/archive/logs"
DRY_RUN="no"

# 参数解析...

# 执行清理
if [ "$DRY_RUN" == "yes" ]; then
    echo "=== 测试模式，不会实际删除文件 ==="
fi

echo "清理目录: $LOG_DIR"
echo "文件模式: $FILE_PATTERN"
echo "保留天数: $RETENTION_DAYS"

# 查找需要清理的文件
find "$LOG_DIR" -name "$FILE_PATTERN" -mtime +$RETENTION_DAYS -type f | while read file; do
    if [ "$ARCHIVE" == "yes" ]; then
        # 归档文件
        archive_file="$ARCHIVE_PATH/$(basename $file).$(date +%Y%m%d)"
        if [ "$DRY_RUN" == "yes" ]; then
            echo "[测试] 归档: $file -> $archive_file"
        else
            cp "$file" "$archive_file" && gzip "$archive_file"
            echo "已归档: $file"
        fi
    fi
    
    # 删除文件
    if [ "$DRY_RUN" == "yes" ]; then
        echo "[测试] 删除: $file"
    else
        rm -f "$file"
        echo "已删除: $file"
    fi
done
```

## 最佳实践

### 1. 参数命名规范
- 使用小写字母和下划线
- 名称要有描述性
- 避免使用Shell保留字

```bash
# 好的命名
# @param db_host 数据库主机地址
# @param backup_path 备份文件路径
# @param max_retries 最大重试次数

# 避免的命名
# @param host 主机  # 太简短
# @param PATH 路径  # 使用了环境变量名
# @param 1st_server 第一台服务器  # 以数字开头
```

### 2. 参数描述规范
- 描述要清晰明了
- 包含可选值时用括号说明
- 必填参数添加 (required) 标记

```bash
# @param log_level 日志级别（debug/info/warn/error）
# @param backup_type 备份类型（full/incremental） (required)
# @param retention_days 备份保留天数（默认7天）
```

### 3. 参数验证
- 验证必填参数是否提供
- 检查参数值的有效性
- 提供有用的错误信息

```bash
# 必填参数验证
if [ -z "$DB_HOST" ]; then
    echo "错误: 必须指定数据库主机地址 (--db_host)"
    exit 1
fi

# 数值范围验证
if [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
    echo "错误: 端口号必须在 1-65535 之间"
    exit 1
fi

# 选项验证
case $LOG_LEVEL in
    debug|info|warn|error)
        ;;
    *)
        echo "错误: 无效的日志级别 '$LOG_LEVEL'"
        echo "可选值: debug, info, warn, error"
        exit 1
        ;;
esac
```

### 4. 默认值处理
- 为可选参数提供合理默认值
- 在变量初始化时设置默认值
- 在参数描述中说明默认值

```bash
# @param timeout 超时时间（秒，默认30）
# @param retry_count 重试次数（默认3）

# 变量初始化时设置默认值
TIMEOUT="30"
RETRY_COUNT="3"
```

### 5. 帮助信息
提供使用说明函数：

```bash
show_help() {
    cat << EOF
使用方法: $0 [选项]

必填参数:
    --db_host HOST        数据库主机地址
    --db_name NAME        数据库名称
    
可选参数:
    --db_port PORT        数据库端口 (默认: 3306)
    --backup_path PATH    备份路径 (默认: /backup)
    --compress yes/no     是否压缩 (默认: yes)
    --help               显示此帮助信息

示例:
    $0 --db_host localhost --db_name mydb
    $0 --db_host 192.168.1.100 --db_name mydb --db_port 3307
EOF
}

# 在参数解析中添加
case $1 in
    --help|-h)
        show_help
        exit 0
        ;;
esac
```

## 注意事项

1. **参数定义位置**
   - 必须在脚本前100行内
   - 建议放在脚本头部注释区域

2. **参数格式**
   - @param 后必须有空格
   - 参数名只能包含字母、数字和下划线
   - 描述可以包含中文

3. **参数传递**
   - 所有参数都以字符串形式传递
   - 脚本需要自行转换类型
   - 空值参数不会传递

4. **安全考虑**
   - 密码类参数不要打印到日志
   - 验证输入防止注入攻击
   - 使用引号包围变量值

5. **错误处理**
   - 设置 `set -e` 使脚本在错误时退出
   - 提供清晰的错误信息
   - 使用合适的退出码