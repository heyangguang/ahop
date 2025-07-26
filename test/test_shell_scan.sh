#!/bin/bash

echo "1. 登录系统..."
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username": "admin", "password": "Admin@123"}' | jq -r '.data.token')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo "登录失败"
    exit 1
fi

echo "Token: ${TOKEN:0:20}..."

echo -e "\n2. 扫描仓库69..."
RESULT=$(curl -s -X POST http://localhost:8080/api/v1/git-repositories/69/scan-templates \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json")

echo -e "\n3. 检查响应..."
echo "$RESULT" | jq '.code'

echo -e "\n4. Shell脚本参数信息..."
echo "$RESULT" | jq '.data.surveys'

echo -e "\n5. 文件统计..."
echo "$RESULT" | jq '.data.stats'

echo -e "\n6. Worker日志最后10行..."
tail -10 /opt/ahop/worker-dist/worker_shell_test.log