#\!/bin/bash

# 测试凭证使用日志新功能（详细版）

# 1. 登录获取token
echo "登录获取token..."
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "Admin@123"
  }' | jq -r '.data.token')

if [ -z "$TOKEN" ]; then
  echo "登录失败"
  exit 1
fi

# 2. 创建新凭证
echo -e "\n创建新凭证..."
CRED_ID=$(curl -s -X POST http://localhost:8080/api/v1/credentials \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "测试凭证2",
    "type": "api_key",
    "api_key": "test-api-key-123",
    "description": "测试operator_info字段"
  }' | jq -r '.data.id')

echo "凭证ID: $CRED_ID"

# 3. 获取解密的凭证
echo -e "\n获取解密的凭证..."
curl -s -X GET "http://localhost:8080/api/v1/credentials/$CRED_ID/decrypt?purpose=api_test" \
  -H "Authorization: Bearer $TOKEN" | jq '{id: .data.id, name: .data.name, api_key: .data.api_key}'

# 等待一下让异步日志写入
sleep 1

# 4. 查看数据库中的日志记录
echo -e "\n查看数据库中的操作日志..."
docker exec postgres-server psql -U postgres -d auto_healing_platform -c \
  "SELECT operator_type, operator_info, user_id, purpose, success, created_at 
   FROM credential_usage_logs 
   WHERE credential_id = $CRED_ID 
   ORDER BY created_at DESC;" | cat

# 5. 测试系统操作（Git仓库同步）
echo -e "\n创建Git仓库（使用凭证）..."
REPO_ID=$(curl -s -X POST http://localhost:8080/api/v1/git-repositories \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "测试仓库",
    "code": "test-repo",
    "url": "https://github.com/test/repo.git",
    "branch": "main",
    "is_public": false,
    "credential_id": '$CRED_ID'
  }' | jq -r '.data.id')

echo "仓库ID: $REPO_ID"

# 等待异步操作
sleep 2

# 6. 再次查看日志，应该能看到系统操作的记录
echo -e "\n查看所有操作日志（包括系统操作）..."
docker exec postgres-server psql -U postgres -d auto_healing_platform -c \
  "SELECT operator_type, operator_info, user_id, purpose, host_name, created_at 
   FROM credential_usage_logs 
   WHERE credential_id = $CRED_ID 
   ORDER BY created_at DESC 
   LIMIT 5;" | cat

echo -e "\n测试完成"
