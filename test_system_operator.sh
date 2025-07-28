#\!/bin/bash

# 测试系统操作的operator记录

# 1. 登录获取token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "Admin@123"
  }' | jq -r '.data.token')

# 2. 创建凭证用于Git仓库
CRED_ID=$(curl -s -X POST http://localhost:8080/api/v1/credentials \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Git仓库凭证",
    "type": "token",
    "token": "github_pat_test123",
    "description": "用于测试系统操作记录"
  }' | jq -r '.data.id')

echo "创建的凭证ID: $CRED_ID"

# 3. 创建使用该凭证的Git仓库
REPO_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/git-repositories \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "测试系统操作",
    "code": "test-system-'$(date +%s)'",
    "url": "https://github.com/test/repo.git",
    "branch": "main",
    "is_public": false,
    "credential_id": '$CRED_ID'
  }')

echo -e "\n仓库创建响应:"
echo "$REPO_RESPONSE" | jq -r '.message // .error'

# 等待异步操作
sleep 2

# 4. 查看所有操作日志
echo -e "\n查看所有操作日志:"
docker exec postgres-server psql -U postgres -d auto_healing_platform -c \
  "SELECT id, operator_type, operator_info, user_id, purpose 
   FROM credential_usage_logs 
   WHERE credential_id = $CRED_ID 
   ORDER BY created_at DESC;" | cat

echo -e "\n完成"
