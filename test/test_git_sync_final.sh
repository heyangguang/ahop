#!/bin/bash

# Git仓库同步完整测试脚本

# 配置
BASE_URL="http://localhost:8080/api/v1"
DB_NAME="auto_healing_platform"
DB_USER="postgres"
DB_PASSWORD="mysecurepassword"
REDIS_HOST="localhost"
REDIS_PORT="6379"
REDIS_PASSWORD="Admin@123"

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 辅助函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# 数据库查询函数
query_db() {
    PGPASSWORD=$DB_PASSWORD psql -h localhost -U $DB_USER -d $DB_NAME -t -c "$1"
}

# 检查Redis连接
check_redis() {
    log_info "检查Redis连接..."
    redis-cli -h $REDIS_HOST -p $REDIS_PORT -a $REDIS_PASSWORD ping > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        log_success "Redis连接正常"
    else
        log_error "Redis连接失败"
        exit 1
    fi
}

# 清理环境
cleanup() {
    log_info "清理测试环境..."
    
    # 删除测试创建的仓库
    query_db "DELETE FROM git_repositories WHERE name LIKE 'Test%Git%Repo%';" > /dev/null 2>&1
    
    # 删除测试创建的凭证
    query_db "DELETE FROM credentials WHERE name LIKE 'Test%Git%Credential%';" > /dev/null 2>&1
    
    # 删除同步日志
    query_db "DELETE FROM git_sync_logs WHERE repository_id NOT IN (SELECT id FROM git_repositories);" > /dev/null 2>&1
    
    # 删除任务模板
    query_db "DELETE FROM task_templates WHERE repository_id NOT IN (SELECT id FROM git_repositories);" > /dev/null 2>&1
    
    # 清理文件系统
    rm -rf /data/ahop/repos/1/* 2>/dev/null
    
    log_success "环境清理完成"
}

# 登录获取token
login() {
    log_info "登录获取token..."
    
    local response=$(curl -s -X POST "$BASE_URL/auth/login" \
        -H "Content-Type: application/json" \
        -d '{
            "username": "admin",
            "password": "Admin@123"
        }')
    
    TOKEN=$(echo $response | jq -r '.data.token')
    
    if [ "$TOKEN" != "null" ] && [ -n "$TOKEN" ]; then
        log_success "登录成功"
    else
        log_error "登录失败: $response"
        exit 1
    fi
}

# 创建SSH凭证
create_ssh_credential() {
    log_info "创建SSH凭证..."
    
    local response=$(curl -s -X POST "$BASE_URL/credentials" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "Test Git SSH Credential",
            "code": "test_git_ssh_'$(date +%s)'",
            "type": "ssh_key",
            "username": "git",
            "private_key": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
            "description": "测试用SSH凭证"
        }')
    
    SSH_CRED_ID=$(echo $response | jq -r '.data.id')
    
    if [ "$SSH_CRED_ID" != "null" ] && [ -n "$SSH_CRED_ID" ]; then
        log_success "SSH凭证创建成功，ID: $SSH_CRED_ID"
    else
        log_error "创建SSH凭证失败: $response"
        return 1
    fi
}

# 创建密码凭证
create_password_credential() {
    log_info "创建密码凭证..."
    
    local response=$(curl -s -X POST "$BASE_URL/credentials" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "Test Git Password Credential",
            "code": "test_git_pwd_'$(date +%s)'",
            "type": "password",
            "username": "testuser",
            "password": "testpass123",
            "description": "测试用密码凭证"
        }')
    
    PWD_CRED_ID=$(echo $response | jq -r '.data.id')
    
    if [ "$PWD_CRED_ID" != "null" ] && [ -n "$PWD_CRED_ID" ]; then
        log_success "密码凭证创建成功，ID: $PWD_CRED_ID"
    else
        log_error "创建密码凭证失败: $response"
        return 1
    fi
}

# 创建公开仓库
create_public_repo() {
    log_info "创建公开仓库..."
    
    local response=$(curl -s -X POST "$BASE_URL/git-repositories" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "Test Public Git Repo",
            "code": "test_public_'$(date +%s)'",
            "url": "https://github.com/gin-gonic/gin.git",
            "branch": "master",
            "is_public": true,
            "sync_enabled": false,
            "description": "测试公开仓库"
        }')
    
    PUBLIC_REPO_ID=$(echo $response | jq -r '.data.id')
    
    if [ "$PUBLIC_REPO_ID" != "null" ] && [ -n "$PUBLIC_REPO_ID" ]; then
        log_success "公开仓库创建成功，ID: $PUBLIC_REPO_ID"
        
        # 查询LocalPath
        local local_path=$(query_db "SELECT local_path FROM git_repositories WHERE id = $PUBLIC_REPO_ID;" | xargs)
        log_info "仓库LocalPath: $local_path"
    else
        log_error "创建公开仓库失败: $response"
        return 1
    fi
}

# 创建私有仓库
create_private_repo() {
    log_info "创建私有仓库..."
    
    # 使用一个需要认证的私有仓库URL（示例）
    local response=$(curl -s -X POST "$BASE_URL/git-repositories" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "Test Private Git Repo",
            "code": "test_private_'$(date +%s)'",
            "url": "https://github.com/private/test-repo.git",
            "branch": "main",
            "is_public": false,
            "credential_id": '$PWD_CRED_ID',
            "sync_enabled": false,
            "description": "测试私有仓库"
        }')
    
    PRIVATE_REPO_ID=$(echo $response | jq -r '.data.id')
    
    if [ "$PRIVATE_REPO_ID" != "null" ] && [ -n "$PRIVATE_REPO_ID" ]; then
        log_success "私有仓库创建成功，ID: $PRIVATE_REPO_ID"
        
        # 查询LocalPath
        local local_path=$(query_db "SELECT local_path FROM git_repositories WHERE id = $PRIVATE_REPO_ID;" | xargs)
        log_info "仓库LocalPath: $local_path"
    else
        log_error "创建私有仓库失败: $response"
        return 1
    fi
}

# 等待同步完成
wait_for_sync() {
    local repo_id=$1
    local max_wait=60
    local waited=0
    
    log_info "等待仓库 $repo_id 同步完成..."
    
    while [ $waited -lt $max_wait ]; do
        local status=$(query_db "SELECT status FROM git_sync_logs WHERE repository_id = $repo_id ORDER BY created_at DESC LIMIT 1;" | xargs)
        
        if [ "$status" = "success" ]; then
            log_success "仓库同步成功"
            return 0
        elif [ "$status" = "failed" ]; then
            log_error "仓库同步失败"
            return 1
        fi
        
        sleep 2
        waited=$((waited + 2))
    done
    
    log_warning "等待超时"
    return 1
}

# 验证同步结果
verify_sync() {
    local repo_id=$1
    local repo_name=$2
    
    log_info "验证仓库 $repo_name (ID: $repo_id) 的同步结果..."
    
    # 查询同步日志
    log_info "查询同步日志..."
    query_db "SELECT id, status, worker_id, started_at, finished_at, duration, from_commit, to_commit, error_message FROM git_sync_logs WHERE repository_id = $repo_id ORDER BY created_at DESC LIMIT 5;"
    
    # 查询最新的成功同步
    local sync_log=$(query_db "SELECT local_path, to_commit FROM git_sync_logs WHERE repository_id = $repo_id AND status = 'success' ORDER BY created_at DESC LIMIT 1;")
    
    if [ -n "$sync_log" ]; then
        log_success "找到成功的同步记录"
        
        # 检查文件系统
        local local_path=$(query_db "SELECT local_path FROM git_repositories WHERE id = $repo_id;" | xargs)
        local full_path="/data/ahop/repos/$local_path"
        
        if [ -d "$full_path/.git" ]; then
            log_success "仓库已克隆到: $full_path"
            
            # 显示仓库信息
            cd "$full_path"
            log_info "当前分支: $(git branch --show-current)"
            log_info "最新提交: $(git log -1 --oneline)"
            log_info "文件数量: $(find . -type f -not -path "./.git/*" | wc -l)"
        else
            log_error "仓库目录不存在: $full_path"
        fi
    else
        log_error "没有找到成功的同步记录"
    fi
    
    # 查询凭证使用日志
    log_info "查询凭证使用日志..."
    query_db "SELECT credential_id, purpose, success, error_message, created_at FROM credential_usage_logs WHERE purpose LIKE '%Git仓库同步%' AND credential_id IN (SELECT credential_id FROM git_repositories WHERE id = $repo_id) ORDER BY created_at DESC LIMIT 5;"
}

# 测试手动同步
test_manual_sync() {
    local repo_id=$1
    local repo_name=$2
    
    log_info "测试手动同步仓库 $repo_name (ID: $repo_id)..."
    
    local response=$(curl -s -X POST "$BASE_URL/git-repositories/$repo_id/sync" \
        -H "Authorization: Bearer $TOKEN")
    
    if [ "$(echo $response | jq -r '.code')" = "200" ]; then
        log_success "手动同步触发成功"
        wait_for_sync $repo_id
        verify_sync $repo_id "$repo_name"
    else
        log_error "手动同步触发失败: $response"
    fi
}

# 测试同步并扫描
test_sync_with_scan() {
    local repo_id=$1
    local repo_name=$2
    
    log_info "测试同步并扫描仓库 $repo_name (ID: $repo_id)..."
    
    local response=$(curl -s -X POST "$BASE_URL/git-repositories/$repo_id/sync-scan" \
        -H "Authorization: Bearer $TOKEN")
    
    if [ "$(echo $response | jq -r '.code')" = "200" ]; then
        log_success "同步并扫描触发成功"
        
        # 等待同步完成
        wait_for_sync $repo_id
        
        # 等待扫描完成
        sleep 5
        
        # 查询任务模板
        log_info "查询扫描生成的任务模板..."
        query_db "SELECT id, name, script_path, script_type, created_at FROM task_templates WHERE repository_id = $repo_id ORDER BY created_at DESC;"
    else
        log_error "同步并扫描触发失败: $response"
    fi
}

# 测试删除仓库
test_delete_repo() {
    local repo_id=$1
    local repo_name=$2
    
    log_info "测试删除仓库 $repo_name (ID: $repo_id)..."
    
    # 获取LocalPath
    local local_path=$(query_db "SELECT local_path FROM git_repositories WHERE id = $repo_id;" | xargs)
    local full_path="/data/ahop/repos/$local_path"
    
    local response=$(curl -s -X DELETE "$BASE_URL/git-repositories/$repo_id" \
        -H "Authorization: Bearer $TOKEN")
    
    if [ "$(echo $response | jq -r '.code')" = "200" ]; then
        log_success "仓库删除成功"
        
        # 等待Worker处理删除
        sleep 3
        
        # 检查文件系统
        if [ -d "$full_path" ]; then
            log_error "仓库目录仍然存在: $full_path"
        else
            log_success "仓库目录已删除"
        fi
        
        # 检查数据库
        local count=$(query_db "SELECT COUNT(*) FROM git_repositories WHERE id = $repo_id;" | xargs)
        if [ "$count" = "0" ]; then
            log_success "数据库记录已删除"
        else
            log_error "数据库记录仍然存在"
        fi
    else
        log_error "仓库删除失败: $response"
    fi
}

# 主测试流程
main() {
    log_info "开始Git仓库同步完整测试..."
    echo "================================================"
    
    # 前置检查
    check_redis
    
    # 清理环境
    cleanup
    
    # 登录
    login
    
    # 创建凭证
    create_password_credential
    create_ssh_credential
    
    echo "================================================"
    log_info "测试公开仓库同步..."
    echo "================================================"
    
    # 创建并测试公开仓库
    if create_public_repo; then
        sleep 2
        test_manual_sync $PUBLIC_REPO_ID "Test Public Git Repo"
        
        # 测试再次同步（应该是pull而不是clone）
        log_info "测试再次同步（pull操作）..."
        test_manual_sync $PUBLIC_REPO_ID "Test Public Git Repo"
        
        # 测试同步并扫描
        test_sync_with_scan $PUBLIC_REPO_ID "Test Public Git Repo"
    fi
    
    echo "================================================"
    log_info "测试私有仓库同步..."
    echo "================================================"
    
    # 创建并测试私有仓库
    if create_private_repo; then
        sleep 2
        test_manual_sync $PRIVATE_REPO_ID "Test Private Git Repo"
    fi
    
    echo "================================================"
    log_info "测试仓库删除..."
    echo "================================================"
    
    # 测试删除仓库
    if [ -n "$PUBLIC_REPO_ID" ]; then
        test_delete_repo $PUBLIC_REPO_ID "Test Public Git Repo"
    fi
    
    echo "================================================"
    log_info "最终验证..."
    echo "================================================"
    
    # 显示所有同步日志
    log_info "所有同步日志："
    query_db "SELECT gr.name, gsl.status, gsl.worker_id, gsl.duration, gsl.from_commit, gsl.to_commit, gsl.created_at 
              FROM git_sync_logs gsl 
              JOIN git_repositories gr ON gsl.repository_id = gr.id 
              WHERE gr.name LIKE 'Test%Git%Repo%' 
              ORDER BY gsl.created_at DESC;"
    
    # 显示凭证使用日志
    log_info "凭证使用日志："
    query_db "SELECT c.name, cul.purpose, cul.success, cul.created_at 
              FROM credential_usage_logs cul 
              JOIN credentials c ON cul.credential_id = c.id 
              WHERE cul.purpose LIKE '%Git仓库同步%' 
              ORDER BY cul.created_at DESC 
              LIMIT 10;"
    
    echo "================================================"
    log_success "Git仓库同步完整测试完成！"
    log_warning "注意：测试数据已保留在数据库中供查看"
    echo "================================================"
}

# 执行主测试
main