#\!/bin/bash

# 测试凭证使用日志新功能

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

echo "Token: ${TOKEN:0:50}..."

# 2. 创建测试凭证
echo -e "\n创建测试凭证..."
CRED_ID=$(curl -s -X POST http://localhost:8080/api/v1/credentials \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "测试凭证",
    "type": "password",
    "username": "testuser",
    "password": "testpass",
    "description": "用于测试新的日志功能"
  }' | jq -r '.data.id')

echo "凭证ID: $CRED_ID"

# 3. 获取解密的凭证（用户操作）
echo -e "\n获取解密的凭证..."
curl -s -X GET "http://localhost:8080/api/v1/credentials/$CRED_ID/decrypt?purpose=testing" \
  -H "Authorization: Bearer $TOKEN" | jq

# 4. 查看使用日志
echo -e "\n查看使用日志..."
curl -s -X GET "http://localhost:8080/api/v1/credentials/$CRED_ID/logs" \
  -H "Authorization: Bearer $TOKEN" | jq '.data.data[] | {operator_type, operator_info, user_id, purpose, success}'

# 5. 通过数据库直接查看日志
echo -e "\n直接查看数据库记录..."
docker exec postgres-server psql -U postgres -d auto_healing_platform -c \
  "SELECT operator_type, operator_info, user_id, purpose, success, created_at 
   FROM credential_usage_logs 
   WHERE credential_id = $CRED_ID 
   ORDER BY created_at DESC;"

echo -e "\n测试完成"
