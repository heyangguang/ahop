#\!/bin/bash

# 测试operator_info字段是否正确保存

# 1. 登录获取token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "Admin@123"
  }' | jq -r '.data.token')

# 2. 创建一个新凭证
CRED_ID=$(curl -s -X POST http://localhost:8080/api/v1/credentials \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "测试operator_info",
    "type": "token",
    "token": "test-token-value",
    "description": "测试新的日志字段"
  }' | jq -r '.data.id')

echo "创建的凭证ID: $CRED_ID"

# 3. 获取解密的凭证（应该记录operator_info='api-decrypt'）
curl -s -X GET "http://localhost:8080/api/v1/credentials/$CRED_ID/decrypt?purpose=test_operator_info" \
  -H "Authorization: Bearer $TOKEN" > /dev/null

# 等待异步日志写入
sleep 1

# 4. 查看日志记录
echo -e "\n查看日志记录:"
docker exec postgres-server psql -U postgres -d auto_healing_platform -c \
  "SELECT id, operator_type, operator_info, user_id, purpose, created_at 
   FROM credential_usage_logs 
   WHERE credential_id = $CRED_ID 
   ORDER BY created_at DESC;" | cat

echo -e "\n完成"
