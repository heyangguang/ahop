# Shell脚本参数定义指南

## 概述

AHOP平台支持通过在Shell脚本中添加特定格式的注释来定义参数。这些参数会在执行任务时通过Web界面收集用户输入，并以命令行参数的形式传递给脚本。

## 核心语法

### 完整格式

```bash
# @param 参数名 [类型] 参数描述 (选项列表) (required)
```

各部分说明：
- `@param` - 固定的参数声明标记
- `参数名` - 参数变量名，只能包含字母、数字和下划线
- `[类型]` - 可选，参数类型，用方括号包裹
- `参数描述` - 参数的描述文字
- `(选项列表)` - 可选，用于选择类型的参数，选项用 `/` 分隔
- `(required)` - 可选，标记参数为必填

### 格式示例

```bash
# 基本格式 - 系统自动推断类型
# @param db_host 数据库主机地址

# 指定类型 - 明确声明参数类型
# @param db_port [integer] 数据库端口号

# 带选项的参数 - 括号内是可选值
# @param log_level 日志级别 (debug/info/warn/error)

# 必填参数 - 末尾添加 (required)
# @param db_name 数据库名称 (required)

# 完整示例 - 包含所有元素
# @param env [select] 部署环境 (dev/test/prod) (required)
```

## 支持的参数类型

### 1. 文本类型

#### text / string - 单行文本
```bash
# @param host_name [text] 主机名称
# @param api_key [string] API密钥
```
- 用途：普通文本输入
- UI：单行输入框

#### textarea / 多行文本 - 多行文本
```bash
# @param config_content [textarea] 配置文件内容
# @param sql_script [多行文本] SQL脚本
```
- 用途：长文本、配置、脚本等
- UI：多行文本框

#### password / 密码 - 密码输入
```bash
# @param db_password [password] 数据库密码
# @param ssh_key_passphrase [密码] SSH密钥密码
```
- 用途：敏感信息输入
- UI：密码输入框（隐藏显示）

### 2. 数值类型

#### integer / int / 整数 - 整数
```bash
# @param port [integer] 服务端口号
# @param retry_count [int] 重试次数
# @param timeout_seconds [整数] 超时时间（秒）
```
- 用途：整数值输入
- UI：数字输入框（只允许整数）

#### float / 浮点数 - 浮点数
```bash
# @param cpu_threshold [float] CPU使用率阈值
# @param memory_limit_gb [浮点数] 内存限制（GB）
```
- 用途：小数值输入
- UI：数字输入框（允许小数）

### 3. 选择类型

#### select / choice / 单选 - 单选
```bash
# @param environment [select] 部署环境 (dev/test/staging/prod)
# @param backup_type [choice] 备份类型 (full/incremental)
# @param enable_ssl [单选] 启用SSL (yes/no)
```
- 用途：从预定义选项中选择一个
- UI：下拉选择框或单选按钮

#### multiselect / 多选 - 多选
```bash
# @param features [multiselect] 启用的功能 (auth/cache/log/monitor)
# @param backup_targets [多选] 备份目标 (database/files/config/logs)
```
- 用途：从预定义选项中选择多个
- UI：复选框组

## 智能类型推断

当不指定类型时，系统会根据参数描述自动推断类型：

### 推断为 password
描述包含以下关键词：
- password / 密码
- secret / 密钥
- token / 令牌

```bash
# @param db_password 数据库密码              # → password
# @param api_secret API访问密钥              # → password
# @param auth_token 认证令牌                 # → password
```

### 推断为 integer
描述包含以下关键词：
- port / 端口
- number / 数字
- 天数 / days
- count / 数量
- timeout / 超时

```bash
# @param server_port 服务器端口              # → integer
# @param retention_days 日志保留天数         # → integer
# @param max_connections 最大连接数          # → integer
```

### 推断为 select（单选）
满足以下条件之一：
1. 描述包含"类型"/"type"且有斜杠分隔的选项
2. 描述包含括号（中英文均可）且内有斜杠分隔的选项

```bash
# @param log_level 日志级别 (debug/info/warn/error)     # → select
# @param env_type 环境类型（开发/测试/生产）              # → select
# @param enable_feature 是否启用功能 (yes/no)            # → select
```

### 推断为 text（默认）
不满足上述条件时，默认为文本类型：

```bash
# @param project_name 项目名称               # → text
# @param description 任务描述                # → text
```

## 实际示例

### 示例1：数据库备份脚本
```bash
#!/bin/bash
# Name: MySQL数据库备份
# Description: 自动备份MySQL数据库并支持压缩和远程传输

# 基本参数 - 使用智能推断
# @param db_host 数据库主机地址 (required)
# @param db_name 数据库名称 (required)
# @param db_user 数据库用户名 (required)
# @param db_password 数据库密码 (required)

# 指定类型的参数
# @param db_port [integer] 数据库端口（默认3306）
# @param backup_type [select] 备份类型 (full/incremental/schema-only) (required)
# @param compression [select] 压缩方式 (none/gzip/bzip2/xz)
# @param retention_days [integer] 本地备份保留天数
# @param remote_backup [select] 是否远程备份 (yes/no)
# @param remote_host [text] 远程备份服务器地址
# @param notification_emails [textarea] 通知邮箱地址（每行一个）
```

### 示例2：应用部署脚本
```bash
#!/bin/bash
# Name: 应用自动化部署
# Description: 支持多环境的应用部署脚本

# 必填参数
# @param app_name [text] 应用名称 (required)
# @param app_version [text] 应用版本号 (required)
# @param environment [select] 部署环境 (dev/test/staging/prod) (required)

# 服务器配置
# @param target_servers [textarea] 目标服务器列表（每行一个IP） (required)
# @param ssh_port [integer] SSH端口号
# @param deploy_user [text] 部署用户名
# @param deploy_password [password] 部署用户密码

# 部署选项
# @param deploy_strategy [select] 部署策略 (rolling/blue-green/canary)
# @param health_check_url [text] 健康检查URL
# @param rollback_on_failure [select] 失败时自动回滚 (yes/no)
# @param services_to_restart [multiselect] 需要重启的服务 (nginx/tomcat/redis/mysql)
# @param pre_deploy_script [textarea] 部署前执行的脚本
# @param post_deploy_script [textarea] 部署后执行的脚本
```

### 示例3：系统监控脚本
```bash
#!/bin/bash
# Name: 系统资源监控
# Description: 监控系统资源使用情况并发送告警

# 监控目标
# @param monitor_items [multiselect] 监控项目 (cpu/memory/disk/network/process) (required)
# @param check_interval [integer] 检查间隔（秒）
# @param duration [integer] 监控持续时间（分钟）

# 告警阈值
# @param cpu_threshold [float] CPU使用率告警阈值（%）
# @param memory_threshold [float] 内存使用率告警阈值（%）
# @param disk_threshold [float] 磁盘使用率告警阈值（%）
# @param network_threshold [integer] 网络流量告警阈值（MB/s）

# 告警配置
# @param alert_method [multiselect] 告警方式 (email/sms/webhook/log)
# @param email_recipients [textarea] 邮件接收人（每行一个邮箱）
# @param webhook_url [text] Webhook地址
# @param sms_numbers [textarea] 短信接收号码（每行一个）

# 输出选项
# @param output_format [select] 输出格式 (text/json/html/csv)
# @param save_report [select] 是否保存报告 (yes/no)
# @param report_path [text] 报告保存路径
```

## 脚本参数接收

### 参数传递机制

AHOP平台会将用户在Web界面输入的参数转换为命令行参数传递给脚本：

```bash
# 用户输入 → 命令行参数
./your_script.sh --参数名1 值1 --参数名2 值2 --参数名3 值3
```

**重要说明**：
- 参数名前加双横线 `--`
- 参数名和值用空格分隔
- 如果值包含空格，会自动用引号包裹
- 空值参数不会传递
- 多选参数的值用逗号分隔

### 完整的参数解析模板

```bash
#!/bin/bash
# Name: 数据库备份脚本
# Description: 备份MySQL数据库并支持多种选项

# 参数定义
# @param db_host [text] 数据库主机地址 (required)
# @param db_port [integer] 数据库端口号
# @param db_name [text] 数据库名称 (required)
# @param db_user [text] 数据库用户名 (required)
# @param db_password [password] 数据库密码 (required)
# @param backup_type [select] 备份类型 (full/incremental/schema-only) (required)
# @param compression [select] 压缩方式 (none/gzip/bzip2)
# @param backup_path [text] 备份保存路径 (required)
# @param excluded_tables [multiselect] 排除的表 (logs/sessions/cache/temp)
# @param notification_emails [textarea] 通知邮箱（每行一个）
# @param dry_run [select] 测试运行 (yes/no)

# ========== 参数初始化 ==========
# 设置默认值
DB_HOST=""
DB_PORT="3306"
DB_NAME=""
DB_USER=""
DB_PASSWORD=""
BACKUP_TYPE=""
COMPRESSION="gzip"
BACKUP_PATH=""
EXCLUDED_TABLES=""
NOTIFICATION_EMAILS=""
DRY_RUN="no"

# ========== 参数解析 ==========
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
        --backup_type)
            BACKUP_TYPE="$2"
            shift 2
            ;;
        --compression)
            COMPRESSION="$2"
            shift 2
            ;;
        --backup_path)
            BACKUP_PATH="$2"
            shift 2
            ;;
        --excluded_tables)
            EXCLUDED_TABLES="$2"
            shift 2
            ;;
        --notification_emails)
            NOTIFICATION_EMAILS="$2"
            shift 2
            ;;
        --dry_run)
            DRY_RUN="$2"
            shift 2
            ;;
        *)
            echo "错误: 未知参数 '$1'"
            echo "使用 --help 查看帮助"
            exit 1
            ;;
    esac
done

# ========== 参数验证 ==========
# 检查必填参数
MISSING_PARAMS=""
[ -z "$DB_HOST" ] && MISSING_PARAMS="$MISSING_PARAMS --db_host"
[ -z "$DB_NAME" ] && MISSING_PARAMS="$MISSING_PARAMS --db_name"
[ -z "$DB_USER" ] && MISSING_PARAMS="$MISSING_PARAMS --db_user"
[ -z "$DB_PASSWORD" ] && MISSING_PARAMS="$MISSING_PARAMS --db_password"
[ -z "$BACKUP_TYPE" ] && MISSING_PARAMS="$MISSING_PARAMS --backup_type"
[ -z "$BACKUP_PATH" ] && MISSING_PARAMS="$MISSING_PARAMS --backup_path"

if [ -n "$MISSING_PARAMS" ]; then
    echo "错误: 缺少必填参数:$MISSING_PARAMS"
    exit 1
fi

# 验证参数值的有效性
# 验证端口号
if ! [[ "$DB_PORT" =~ ^[0-9]+$ ]] || [ "$DB_PORT" -lt 1 ] || [ "$DB_PORT" -gt 65535 ]; then
    echo "错误: 无效的端口号 '$DB_PORT' (必须是1-65535之间的数字)"
    exit 1
fi

# 验证备份类型
case $BACKUP_TYPE in
    full|incremental|schema-only)
        ;;
    *)
        echo "错误: 无效的备份类型 '$BACKUP_TYPE'"
        echo "有效选项: full, incremental, schema-only"
        exit 1
        ;;
esac

# 验证压缩方式
case $COMPRESSION in
    none|gzip|bzip2)
        ;;
    *)
        echo "错误: 无效的压缩方式 '$COMPRESSION'"
        echo "有效选项: none, gzip, bzip2"
        exit 1
        ;;
esac

# ========== 处理特殊参数 ==========
# 处理多选参数（逗号分隔转数组）
if [ -n "$EXCLUDED_TABLES" ]; then
    IFS=',' read -ra EXCLUDED_TABLES_ARRAY <<< "$EXCLUDED_TABLES"
fi

# 处理多行文本参数（换行符分隔）
if [ -n "$NOTIFICATION_EMAILS" ]; then
    # 将换行符转换为数组
    mapfile -t EMAIL_ARRAY <<< "$NOTIFICATION_EMAILS"
fi

# ========== 显示配置（用于调试） ==========
if [ "$DRY_RUN" = "yes" ]; then
    echo "===== 测试模式 - 配置信息 ====="
    echo "数据库主机: $DB_HOST:$DB_PORT"
    echo "数据库名称: $DB_NAME"
    echo "数据库用户: $DB_USER"
    echo "数据库密码: ****"
    echo "备份类型: $BACKUP_TYPE"
    echo "压缩方式: $COMPRESSION"
    echo "备份路径: $BACKUP_PATH"
    
    if [ -n "$EXCLUDED_TABLES" ]; then
        echo "排除的表:"
        for table in "${EXCLUDED_TABLES_ARRAY[@]}"; do
            echo "  - $table"
        done
    fi
    
    if [ ${#EMAIL_ARRAY[@]} -gt 0 ]; then
        echo "通知邮箱:"
        for email in "${EMAIL_ARRAY[@]}"; do
            [ -n "$email" ] && echo "  - $email"
        done
    fi
    echo "================================"
fi

# ========== 执行主逻辑 ==========
# ... 实际的备份逻辑 ...
```

### 参数处理技巧

#### 1. 处理带空格的参数值
```bash
# AHOP会自动处理，但脚本中要正确使用引号
FILE_PATH="$2"  # 正确：使用引号
FILE_PATH=$2    # 错误：空格会导致问题
```

#### 2. 处理多选参数
```bash
# 多选参数值用逗号分隔，需要转换为数组
# @param services [multiselect] 要重启的服务 (nginx/mysql/redis)

# 转换为数组
IFS=',' read -ra SERVICES_ARRAY <<< "$SERVICES"

# 遍历处理
for service in "${SERVICES_ARRAY[@]}"; do
    echo "处理服务: $service"
done
```

#### 3. 处理多行文本
```bash
# @param config_content [textarea] 配置内容

# 方法1：保持原样使用
echo "$CONFIG_CONTENT" > config.txt

# 方法2：按行处理
while IFS= read -r line; do
    echo "处理行: $line"
done <<< "$CONFIG_CONTENT"

# 方法3：转换为数组
mapfile -t LINES <<< "$CONFIG_CONTENT"
```

#### 4. 参数默认值最佳实践
```bash
# 在初始化时设置默认值
TIMEOUT="${TIMEOUT:-30}"        # 如果为空，使用30
PORT="${PORT:-3306}"           # 如果为空，使用3306

# 或在参数解析后设置
[ -z "$TIMEOUT" ] && TIMEOUT=30
[ -z "$PORT" ] && PORT=3306
```


## 最佳实践

### 1. 参数命名规范

#### 命名原则
- 使用小写字母和下划线（snake_case）
- 名称要有描述性，避免过于简短
- 避免使用Shell保留字和环境变量名
- 相关参数使用一致的前缀

#### 示例
```bash
# ✅ 推荐的命名
# @param db_host [text] 数据库主机地址
# @param db_port [integer] 数据库端口
# @param db_name [text] 数据库名称
# @param db_user [text] 数据库用户名
# @param db_password [password] 数据库密码

# ❌ 避免的命名
# @param host           # 太简短，不明确
# @param PATH           # 与环境变量冲突
# @param 1st_server     # 以数字开头
# @param user-name      # 使用连字符而非下划线
```

### 2. 参数类型选择指南

#### 类型选择决策树
```bash
# 密码或敏感信息 → [password]
# @param api_key [password] API访问密钥

# 数字类型
#   - 整数 → [integer]
#   - 小数 → [float]
# @param port [integer] 服务端口号
# @param threshold [float] CPU使用率阈值

# 选择类型
#   - 2-5个固定选项 → [select]
#   - 多个选项可多选 → [multiselect]
# @param environment [select] 部署环境 (dev/test/prod)
# @param features [multiselect] 启用的功能 (ssl/cache/log)

# 文本类型
#   - 单行短文本 → [text]
#   - 多行或长文本 → [textarea]
# @param username [text] 用户名
# @param config_content [textarea] 配置文件内容
```

### 3. 参数描述编写规范

#### 描述格式
```bash
# 基本格式
# @param 参数名 [类型] 描述说明

# 带选项的格式
# @param 参数名 [类型] 描述说明 (选项1/选项2/选项3)

# 必填参数
# @param 参数名 [类型] 描述说明 (required)

# 带默认值说明
# @param 参数名 [类型] 描述说明（默认值：xxx）
```

#### 描述内容要点
- 说明参数的用途和影响
- 包含单位信息（秒、MB、百分比等）
- 说明格式要求（如IP地址、URL等）
- 列出所有可选值
- 标注默认值

```bash
# @param timeout [integer] 连接超时时间（秒，默认30）
# @param memory_limit [integer] 内存使用限制（MB，最小128）
# @param backup_servers [textarea] 备份服务器列表（每行一个IP地址）
# @param log_format [select] 日志输出格式 (json/text/xml)
```

### 4. 参数验证最佳实践

#### 完整的验证流程
```bash
#!/bin/bash
# 参数验证函数
validate_parameters() {
    local errors=0
    
    # 1. 检查必填参数
    if [ -z "$DB_HOST" ]; then
        echo "错误: 缺少必填参数 --db_host"
        ((errors++))
    fi
    
    # 2. 验证数值范围
    if [[ "$PORT" =~ ^[0-9]+$ ]]; then
        if [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
            echo "错误: 端口号必须在 1-65535 之间，当前值: $PORT"
            ((errors++))
        fi
    else
        echo "错误: 端口号必须是数字，当前值: $PORT"
        ((errors++))
    fi
    
    # 3. 验证选项值
    case $LOG_LEVEL in
        debug|info|warn|error) ;;
        *)
            echo "错误: 无效的日志级别 '$LOG_LEVEL'"
            echo "      有效选项: debug, info, warn, error"
            ((errors++))
            ;;
    esac
    
    # 4. 验证文件/目录
    if [ -n "$CONFIG_FILE" ] && [ ! -f "$CONFIG_FILE" ]; then
        echo "错误: 配置文件不存在: $CONFIG_FILE"
        ((errors++))
    fi
    
    if [ -n "$BACKUP_PATH" ] && [ ! -d "$BACKUP_PATH" ]; then
        echo "警告: 备份目录不存在，将尝试创建: $BACKUP_PATH"
    fi
    
    # 5. 验证格式
    if [ -n "$EMAIL" ] && ! [[ "$EMAIL" =~ ^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$ ]]; then
        echo "错误: 无效的邮箱格式: $EMAIL"
        ((errors++))
    fi
    
    # 返回错误数
    return $errors
}

# 调用验证
if ! validate_parameters; then
    echo "参数验证失败，请检查后重试"
    exit 1
fi
```

### 5. 错误处理和日志

#### 错误处理模式
```bash
#!/bin/bash
set -euo pipefail  # 严格模式

# 定义日志函数
log() {
    local level=$1
    shift
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] [$level] $*" >&2
}

log_info() { log "INFO" "$@"; }
log_warn() { log "WARN" "$@"; }
log_error() { log "ERROR" "$@"; }

# 错误处理函数
handle_error() {
    local exit_code=$?
    local line_number=$1
    log_error "脚本在第 $line_number 行发生错误，退出码: $exit_code"
    
    # 清理操作
    cleanup
    
    exit $exit_code
}

# 设置错误陷阱
trap 'handle_error $LINENO' ERR

# 清理函数
cleanup() {
    log_info "执行清理操作..."
    # 删除临时文件
    # 释放锁
    # 其他清理操作
}

# 确保脚本退出时执行清理
trap cleanup EXIT
```

### 6. 性能优化建议

#### 参数解析优化
```bash
# 使用关联数组存储参数（Bash 4+）
declare -A params=(
    [db_host]=""
    [db_port]="3306"
    [db_name]=""
    [db_user]=""
)

# 批量参数解析
while [[ $# -gt 0 ]]; do
    key="${1#--}"
    if [[ -v params[$key] ]]; then
        params[$key]="$2"
        shift 2
    else
        echo "未知参数: $1"
        exit 1
    fi
done

# 使用参数
DB_HOST="${params[db_host]}"
DB_PORT="${params[db_port]}"
```

### 7. 安全建议

#### 敏感信息处理
```bash
# 1. 不要在日志中打印密码
log_info "连接数据库: $DB_USER@$DB_HOST:$DB_PORT"  # 不打印密码

# 2. 使用环境变量传递敏感信息
export PGPASSWORD="$DB_PASSWORD"  # PostgreSQL
mysql -h "$DB_HOST" -u "$DB_USER" -p"$DB_PASSWORD"  # 注意：会在进程列表中显示

# 3. 清理命令历史
history -d $(history 1)  # 删除包含密码的命令

# 4. 设置严格的文件权限
umask 077  # 新建文件只有所有者可读写
```

## 快速参考卡

### 参数定义语法速查
```bash
# 基本格式
# @param 参数名 参数描述

# 指定类型
# @param 参数名 [类型] 参数描述

# 带选项
# @param 参数名 [类型] 参数描述 (选项1/选项2/选项3)

# 必填参数
# @param 参数名 [类型] 参数描述 (required)

# 完整示例
# @param env [select] 部署环境 (dev/test/prod) (required)
```

### 支持的类型
| 类型关键词 | UI组件 | 用途 |
|-----------|--------|------|
| text, string | 单行输入框 | 普通文本 |
| password, 密码 | 密码输入框 | 敏感信息 |
| integer, int, 整数 | 数字输入框 | 整数值 |
| float, 浮点数 | 数字输入框 | 小数值 |
| select, choice, 单选 | 下拉框 | 单选项 |
| multiselect, 多选 | 复选框组 | 多选项 |
| textarea, 多行文本 | 多行文本框 | 长文本 |

### 常用参数模板
```bash
# 数据库连接
# @param db_host [text] 数据库主机地址 (required)
# @param db_port [integer] 数据库端口（默认3306）
# @param db_name [text] 数据库名称 (required)
# @param db_user [text] 数据库用户名 (required)
# @param db_password [password] 数据库密码 (required)

# 服务器配置
# @param server_host [text] 服务器地址 (required)
# @param ssh_port [integer] SSH端口（默认22）
# @param ssh_user [text] SSH用户名
# @param ssh_password [password] SSH密码

# 通用选项
# @param environment [select] 运行环境 (dev/test/staging/prod)
# @param log_level [select] 日志级别 (debug/info/warn/error)
# @param dry_run [select] 测试模式 (yes/no)
# @param enable_backup [select] 启用备份 (yes/no)
```

## 注意事项

### 定义规则
1. **位置要求**：参数定义必须在脚本前100行内，建议放在脚本头部
2. **格式要求**：`@param` 后必须有空格，参数名只能包含字母、数字和下划线
3. **连续性要求**：参数定义必须连续，遇到非注释行会停止扫描

### 传递机制
1. **参数格式**：`--参数名 值` 形式传递
2. **类型转换**：所有参数以字符串传递，脚本需自行转换
3. **空值处理**：空值参数不会传递给脚本
4. **特殊字符**：包含空格的值会自动加引号

### 安全建议
1. **敏感信息**：使用 `[password]` 类型，避免在日志中打印
2. **输入验证**：始终验证参数值，防止注入攻击
3. **错误处理**：使用 `set -euo pipefail` 启用严格模式
4. **权限控制**：使用 `umask 077` 限制文件权限

## 故障排查

### 参数未被扫描
1. 检查参数定义是否在前100行内
2. 确保使用 `# @param` 格式（`#` 和 `@param` 之间有空格）
3. 确认参数定义是连续的注释块
4. 参数名不能包含特殊字符（只允许字母、数字、下划线）

### 类型推断错误
1. 明确指定类型：`# @param name [type] description`
2. 检查括号格式：支持中英文括号 `()` 和 `（）`
3. 选项用 `/` 分隔：`(option1/option2/option3)`

### 参数接收问题
1. 使用 `"$2"` 而非 `$2` 接收参数值
2. 检查参数名拼写是否一致
3. 使用 `shift 2` 正确移动参数位置
4. 添加未知参数处理分支

## 更多资源

- 完整示例脚本：`/opt/ahop/scripts/shell/`
- Worker扫描器源码：`/opt/ahop/worker-dist/internal/scanner/shell_scanner.go`
- 测试脚本：`/opt/ahop/scripts/shell/test_param_types.sh`