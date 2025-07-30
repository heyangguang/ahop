#!/bin/bash
# Name: Enhanced Disk Cleanup
# Description: 自动清理指定目录的日志文件和临时文件，释放磁盘空间
#
# @param LOG_RETENTION_DAYS 日志文件保留天数 (required)
# @param CLEAN_LOG_DIRS 要清理的日志目录路径 (required)
# @param CLEAN_TMP_DIRS 要清理的临时文件目录路径
# @param TMP_FILE_AGE 临时文件保留天数
# @param CLEAN_PACKAGE_CACHE 是否清理包管理器缓存 (yes/no)
# @param DRY_RUN 测试模式 (yes/no)
# @param CLEAN_TYPE 清理类型 (logs/temp/cache/all)
# @param MAX_DEPTH 清理目录最大深度
# @param NOTIFICATION_EMAIL 完成后通知邮箱地址
# @param LOG_LEVEL 日志级别 (debug/info/warn/error)

# 初始化默认值
LOG_RETENTION_DAYS="${LOG_RETENTION_DAYS:-7}"
CLEAN_LOG_DIRS="${CLEAN_LOG_DIRS:-/var/log}"
CLEAN_TMP_DIRS="${CLEAN_TMP_DIRS:-/tmp}"
TMP_FILE_AGE="${TMP_FILE_AGE:-3}"
CLEAN_PACKAGE_CACHE="${CLEAN_PACKAGE_CACHE:-no}"
DRY_RUN="${DRY_RUN:-no}"
CLEAN_TYPE="${CLEAN_TYPE:-all}"
MAX_DEPTH="${MAX_DEPTH:-3}"
NOTIFICATION_EMAIL="${NOTIFICATION_EMAIL:-}"
LOG_LEVEL="${LOG_LEVEL:-info}"

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --LOG_RETENTION_DAYS)
            LOG_RETENTION_DAYS="$2"
            shift 2
            ;;
        --CLEAN_LOG_DIRS)
            CLEAN_LOG_DIRS="$2"
            shift 2
            ;;
        --CLEAN_TMP_DIRS)
            CLEAN_TMP_DIRS="$2"
            shift 2
            ;;
        --TMP_FILE_AGE)
            TMP_FILE_AGE="$2"
            shift 2
            ;;
        --CLEAN_PACKAGE_CACHE)
            CLEAN_PACKAGE_CACHE="$2"
            shift 2
            ;;
        --DRY_RUN)
            DRY_RUN="$2"
            shift 2
            ;;
        --CLEAN_TYPE)
            CLEAN_TYPE="$2"
            shift 2
            ;;
        --MAX_DEPTH)
            MAX_DEPTH="$2"
            shift 2
            ;;
        --NOTIFICATION_EMAIL)
            NOTIFICATION_EMAIL="$2"
            shift 2
            ;;
        --LOG_LEVEL)
            LOG_LEVEL="$2"
            shift 2
            ;;
        *)
            echo "Unknown parameter: $1"
            exit 1
            ;;
    esac
done

# 日志函数
log() {
    local level=$1
    shift
    local message="$@"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    case $level in
        DEBUG|debug)
            [[ "$LOG_LEVEL" == "debug" ]] && echo "[$timestamp] [DEBUG] $message"
            ;;
        INFO|info)
            [[ "$LOG_LEVEL" =~ ^(debug|info)$ ]] && echo "[$timestamp] [INFO] $message"
            ;;
        WARN|warn)
            [[ "$LOG_LEVEL" =~ ^(debug|info|warn)$ ]] && echo "[$timestamp] [WARN] $message"
            ;;
        ERROR|error)
            echo "[$timestamp] [ERROR] $message" >&2
            ;;
    esac
}

# 参数验证
if [[ -z "$LOG_RETENTION_DAYS" ]] || ! [[ "$LOG_RETENTION_DAYS" =~ ^[0-9]+$ ]]; then
    log ERROR "LOG_RETENTION_DAYS must be a positive number"
    exit 1
fi

if [[ -z "$CLEAN_LOG_DIRS" ]]; then
    log ERROR "CLEAN_LOG_DIRS is required"
    exit 1
fi

# 显示配置
log INFO "=== Disk Cleanup Configuration ==="
log INFO "Log retention days: $LOG_RETENTION_DAYS"
log INFO "Log directories: $CLEAN_LOG_DIRS"
log INFO "Temp directories: $CLEAN_TMP_DIRS"
log INFO "Temp file age: $TMP_FILE_AGE days"
log INFO "Clean package cache: $CLEAN_PACKAGE_CACHE"
log INFO "Dry run mode: $DRY_RUN"
log INFO "Clean type: $CLEAN_TYPE"
log INFO "Max depth: $MAX_DEPTH"
log INFO "Log level: $LOG_LEVEL"
[[ -n "$NOTIFICATION_EMAIL" ]] && log INFO "Notification email: $NOTIFICATION_EMAIL"

# 统计变量
TOTAL_FILES_REMOVED=0
TOTAL_SPACE_FREED=0

# 清理日志文件函数
clean_logs() {
    local dir=$1
    log INFO "Cleaning log files in: $dir"
    
    if [[ ! -d "$dir" ]]; then
        log WARN "Directory not found: $dir"
        return
    fi
    
    local count=0
    local size=0
    
    while IFS= read -r -d '' file; do
        local file_size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || echo 0)
        
        if [[ "$DRY_RUN" == "yes" ]]; then
            log DEBUG "[DRY RUN] Would remove: $file ($(numfmt --to=iec $file_size 2>/dev/null || echo "${file_size} bytes"))"
        else
            if rm -f "$file"; then
                log DEBUG "Removed: $file"
                ((count++))
                ((size+=file_size))
            else
                log WARN "Failed to remove: $file"
            fi
        fi
    done < <(find "$dir" -maxdepth "$MAX_DEPTH" -type f -name "*.log" -mtime +$LOG_RETENTION_DAYS -print0 2>/dev/null)
    
    log INFO "Log cleanup in $dir: $count files, $(numfmt --to=iec $size 2>/dev/null || echo "${size} bytes")"
    ((TOTAL_FILES_REMOVED+=count))
    ((TOTAL_SPACE_FREED+=size))
}

# 清理临时文件函数
clean_temp() {
    local dir=$1
    log INFO "Cleaning temp files in: $dir"
    
    if [[ ! -d "$dir" ]]; then
        log WARN "Directory not found: $dir"
        return
    fi
    
    local count=0
    local size=0
    
    while IFS= read -r -d '' file; do
        local file_size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || echo 0)
        
        if [[ "$DRY_RUN" == "yes" ]]; then
            log DEBUG "[DRY RUN] Would remove: $file ($(numfmt --to=iec $file_size 2>/dev/null || echo "${file_size} bytes"))"
        else
            if rm -f "$file"; then
                log DEBUG "Removed: $file"
                ((count++))
                ((size+=file_size))
            else
                log WARN "Failed to remove: $file"
            fi
        fi
    done < <(find "$dir" -maxdepth "$MAX_DEPTH" -type f -mtime +$TMP_FILE_AGE -print0 2>/dev/null)
    
    log INFO "Temp cleanup in $dir: $count files, $(numfmt --to=iec $size 2>/dev/null || echo "${size} bytes")"
    ((TOTAL_FILES_REMOVED+=count))
    ((TOTAL_SPACE_FREED+=size))
}

# 清理包管理器缓存
clean_package_cache() {
    if [[ "$CLEAN_PACKAGE_CACHE" != "yes" ]]; then
        return
    fi
    
    log INFO "Cleaning package manager cache"
    
    # APT cache (Debian/Ubuntu)
    if command -v apt-get &> /dev/null; then
        if [[ "$DRY_RUN" == "yes" ]]; then
            log INFO "[DRY RUN] Would run: apt-get clean"
        else
            apt-get clean && log INFO "APT cache cleaned"
        fi
    fi
    
    # YUM cache (RHEL/CentOS)
    if command -v yum &> /dev/null; then
        if [[ "$DRY_RUN" == "yes" ]]; then
            log INFO "[DRY RUN] Would run: yum clean all"
        else
            yum clean all && log INFO "YUM cache cleaned"
        fi
    fi
}

# 主清理逻辑
log INFO "=== Starting disk cleanup ==="

case $CLEAN_TYPE in
    logs)
        IFS=',' read -ra LOG_DIRS <<< "$CLEAN_LOG_DIRS"
        for dir in "${LOG_DIRS[@]}"; do
            clean_logs "$(echo $dir | xargs)"  # trim whitespace
        done
        ;;
    temp)
        IFS=',' read -ra TMP_DIRS <<< "$CLEAN_TMP_DIRS"
        for dir in "${TMP_DIRS[@]}"; do
            clean_temp "$(echo $dir | xargs)"  # trim whitespace
        done
        ;;
    cache)
        clean_package_cache
        ;;
    all)
        # Clean everything
        IFS=',' read -ra LOG_DIRS <<< "$CLEAN_LOG_DIRS"
        for dir in "${LOG_DIRS[@]}"; do
            clean_logs "$(echo $dir | xargs)"
        done
        
        IFS=',' read -ra TMP_DIRS <<< "$CLEAN_TMP_DIRS"
        for dir in "${TMP_DIRS[@]}"; do
            clean_temp "$(echo $dir | xargs)"
        done
        
        clean_package_cache
        ;;
    *)
        log ERROR "Invalid clean type: $CLEAN_TYPE"
        exit 1
        ;;
esac

# 总结
log INFO "=== Cleanup Summary ==="
log INFO "Total files removed: $TOTAL_FILES_REMOVED"
log INFO "Total space freed: $(numfmt --to=iec $TOTAL_SPACE_FREED 2>/dev/null || echo "${TOTAL_SPACE_FREED} bytes")"

# 发送通知邮件
if [[ -n "$NOTIFICATION_EMAIL" ]] && [[ "$DRY_RUN" != "yes" ]]; then
    if command -v mail &> /dev/null; then
        echo -e "Disk cleanup completed on $(hostname)\n\nFiles removed: $TOTAL_FILES_REMOVED\nSpace freed: $(numfmt --to=iec $TOTAL_SPACE_FREED 2>/dev/null || echo "${TOTAL_SPACE_FREED} bytes")" | \
        mail -s "Disk Cleanup Report - $(hostname)" "$NOTIFICATION_EMAIL"
        log INFO "Notification sent to: $NOTIFICATION_EMAIL"
    else
        log WARN "Mail command not found, cannot send notification"
    fi
fi

log INFO "Disk cleanup completed"