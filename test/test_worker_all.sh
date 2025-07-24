#!/bin/bash

# AHOP Worker 综合测试脚本
# 测试内容：Shell执行、Ansible模块、分布式抢占、错误处理、多参数格式

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

# 测试结果统计
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# 记录测试结果
record_test() {
    local test_name=$1
    local result=$2
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    if [ "$result" = "pass" ]; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
        echo -e "${GREEN}✓ $test_name${NC}"
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo -e "${RED}✗ $test_name${NC}"
    fi
}

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     AHOP Worker 综合测试套件           ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"

# 获取JWT Token
echo -e "\n${YELLOW}准备测试环境...${NC}"
JWT_TOKEN=$(curl -s -X POST "http://localhost:8080/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username": "admin", "password": "Admin@123"}' | jq -r '.data.token')

if [ -z "$JWT_TOKEN" ] || [ "$JWT_TOKEN" = "null" ]; then
    echo -e "${RED}✗ 无法获取JWT Token${NC}"
    exit 1
fi

# 清理环境
pkill -f ahop-worker 2>/dev/null || true
sleep 2

# 创建测试资源
echo -e "${YELLOW}创建测试资源...${NC}"

# 正确的凭证
GOOD_CRED=$(curl -s -X POST "http://localhost:8080/api/v1/credentials" \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "test-good-cred",
        "type": "password",
        "username": "root",
        "password": "heyang2015."
    }' | jq -r '.data.id')

# 错误的凭证
BAD_CRED=$(curl -s -X POST "http://localhost:8080/api/v1/credentials" \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "test-bad-cred",
        "type": "password",
        "username": "wronguser",
        "password": "wrongpass"
    }' | jq -r '.data.id')

# 创建主机
GOOD_HOST=$(curl -s -X POST "http://localhost:8080/api/v1/hosts" \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
        \"name\": \"test-good-host\",
        \"ip_address\": \"127.0.0.1\",
        \"port\": 22,
        \"credential_id\": $GOOD_CRED
    }" | jq -r '.data.id')

BAD_HOST=$(curl -s -X POST "http://localhost:8080/api/v1/hosts" \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
        \"name\": \"test-bad-host\",
        \"ip_address\": \"127.0.0.2\",
        \"port\": 22,
        \"credential_id\": $BAD_CRED
    }" | jq -r '.data.id')

echo -e "${GREEN}✓ 测试资源创建完成${NC}"

# 启动单个Worker测试基础功能
echo -e "\n${BLUE}═══ 第一部分：基础功能测试 ═══${NC}"

cd /opt/ahop/worker-dist
export REDIS_HOST="localhost"
export REDIS_PASSWORD="Admin@123"
export DB_HOST="localhost"
export DB_PASSWORD="Admin@123"
export LOG_LEVEL="info"
export CREDENTIAL_ENCRYPTION_KEY="ahop-credential-encryption-key32"
export WORKER_ID="test-worker"
export WORKER_CONCURRENCY="2"

./build/ahop-worker &
WORKER_PID=$!
sleep 3

# 测试1: 原生Shell命令执行（不通过Ansible）
echo -e "\n${YELLOW}1. 原生Shell命令执行测试${NC}"
TASK1=$(curl -s -X POST "http://localhost:8080/api/v1/tasks" \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "原生Shell测试",
        "task_type": "shell_command",
        "priority": 5,
        "params": {
            "hosts": ["127.0.0.1"],
            "command": "echo \"Hello from Native Shell\" && whoami && date"
        },
        "timeout": 30
    }' | jq -r '.data.task_id')

sleep 5
RESULT1=$(curl -s -H "Authorization: Bearer $JWT_TOKEN" \
    "http://localhost:8080/api/v1/tasks/$TASK1" | jq -r '.data.status')
if [ "$RESULT1" = "success" ]; then
    record_test "原生Shell命令执行" "pass"
else
    record_test "原生Shell命令执行" "fail"
fi

# 测试1.5: Ansible Shell执行（对比）
echo -e "\n${YELLOW}1.5. Ansible Shell执行测试${NC}"
TASK1_5=$(curl -s -X POST "http://localhost:8080/api/v1/tasks" \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "Ansible Shell测试",
        "task_type": "ansible_adhoc",
        "priority": 5,
        "params": {
            "hosts": ["127.0.0.1"],
            "module": "shell",
            "args": "echo \"Hello from Ansible Shell\" && whoami"
        },
        "timeout": 30
    }' | jq -r '.data.task_id')

sleep 5
RESULT1_5=$(curl -s -H "Authorization: Bearer $JWT_TOKEN" \
    "http://localhost:8080/api/v1/tasks/$TASK1_5" | jq -r '.data.status')
if [ "$RESULT1_5" = "success" ]; then
    record_test "Ansible Shell执行" "pass"
else
    record_test "Ansible Shell执行" "fail"
fi

# 测试2: Ansible ping模块
echo -e "\n${YELLOW}2. Ansible ping模块测试${NC}"
TASK2=$(curl -s -X POST "http://localhost:8080/api/v1/tasks" \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
        \"name\": \"Ping测试\",
        \"task_type\": \"host_ping\",
        \"priority\": 5,
        \"params\": {
            \"host_ids\": [$GOOD_HOST]
        },
        \"timeout\": 30
    }" | jq -r '.data.task_id')

sleep 5
RESULT2=$(curl -s -H "Authorization: Bearer $JWT_TOKEN" \
    "http://localhost:8080/api/v1/tasks/$TASK2" | jq -r '.data.status')
if [ "$RESULT2" = "success" ]; then
    record_test "Ansible ping模块" "pass"
else
    record_test "Ansible ping模块" "fail"
fi

# 测试3: 主机信息采集
echo -e "\n${YELLOW}3. 主机信息采集测试${NC}"
TASK3=$(curl -s -X POST "http://localhost:8080/api/v1/tasks" \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
        \"name\": \"信息采集测试\",
        \"task_type\": \"host_facts\",
        \"priority\": 5,
        \"params\": {
            \"host_ids\": [$GOOD_HOST]
        },
        \"timeout\": 60
    }" | jq -r '.data.task_id')

# host_facts (setup模块) 需要更长时间
sleep 10
RESULT3=$(curl -s -H "Authorization: Bearer $JWT_TOKEN" \
    "http://localhost:8080/api/v1/tasks/$TASK3" | jq -r '.data.status')
if [ "$RESULT3" = "success" ]; then
    record_test "主机信息采集" "pass"
else
    record_test "主机信息采集" "fail"
    # 显示错误信息帮助调试
    ERROR_MSG=$(curl -s -H "Authorization: Bearer $JWT_TOKEN" \
        "http://localhost:8080/api/v1/tasks/$TASK3" | jq -r '.data.error')
    echo "  错误: $ERROR_MSG"
fi

# 测试4: 错误处理（错误凭证）
echo -e "\n${YELLOW}4. 错误处理测试${NC}"
TASK4=$(curl -s -X POST "http://localhost:8080/api/v1/tasks" \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
        \"name\": \"错误凭证测试\",
        \"task_type\": \"ansible_adhoc\",
        \"priority\": 5,
        \"params\": {
            \"hosts\": [\"127.0.0.2\"],
            \"module\": \"shell\",
            \"args\": \"whoami\"
        },
        \"timeout\": 30
    }" | jq -r '.data.task_id')

sleep 5
RESULT4=$(curl -s -H "Authorization: Bearer $JWT_TOKEN" \
    "http://localhost:8080/api/v1/tasks/$TASK4" | jq -r '.data.status')
if [ "$RESULT4" = "failed" ]; then
    record_test "错误处理（立即失败）" "pass"
else
    record_test "错误处理（立即失败）" "fail"
fi

# 停止单Worker
kill $WORKER_PID 2>/dev/null || true
sleep 2

# 第二部分：分布式测试
echo -e "\n${BLUE}═══ 第二部分：分布式抢占测试 ═══${NC}"

# 启动3个Worker
export WORKER_ID="dist-worker-1"
export WORKER_CONCURRENCY="3"
./build/ahop-worker &
PID1=$!

export WORKER_ID="dist-worker-2"
export WORKER_CONCURRENCY="2"
./build/ahop-worker &
PID2=$!

export WORKER_ID="dist-worker-3"
export WORKER_CONCURRENCY="1"
./build/ahop-worker &
PID3=$!

echo -e "${GREEN}✓ 启动3个Worker (并发度: 3:2:1)${NC}"
sleep 3

# 创建10个任务测试分布式抢占
echo -e "\n${YELLOW}5. 分布式任务抢占测试${NC}"
TASK_IDS=()
for i in {1..10}; do
    TASK_ID=$(curl -s -X POST "http://localhost:8080/api/v1/tasks" \
        -H "Authorization: Bearer $JWT_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
            \"name\": \"分布式任务-$i\",
            \"task_type\": \"ansible_adhoc\",
            \"priority\": 5,
            \"params\": {
                \"hosts\": [\"127.0.0.1\"],
                \"module\": \"shell\",
                \"args\": \"echo 'Task $i executed'\"
            },
            \"timeout\": 30
        }" | jq -r '.data.task_id')
    TASK_IDS+=($TASK_ID)
done

# 等待任务完成
sleep 15

# 统计任务分配
W1=$(grep '"worker_id":"dist-worker-1"' /opt/ahop/worker-dist/logs/worker.log | grep '"msg":"开始处理任务"' | wc -l)
W2=$(grep '"worker_id":"dist-worker-2"' /opt/ahop/worker-dist/logs/worker.log | grep '"msg":"开始处理任务"' | wc -l)
W3=$(grep '"worker_id":"dist-worker-3"' /opt/ahop/worker-dist/logs/worker.log | grep '"msg":"开始处理任务"' | wc -l)

echo "Worker-1: $W1 个任务"
echo "Worker-2: $W2 个任务"
echo "Worker-3: $W3 个任务"

# 检查分配是否合理（Worker1应该处理最多）
if [ $W1 -ge $W2 ] && [ $W2 -ge $W3 ]; then
    record_test "分布式任务分配" "pass"
else
    record_test "分布式任务分配" "fail"
fi

# 检查所有任务是否成功
ALL_SUCCESS=true
for TASK_ID in "${TASK_IDS[@]}"; do
    STATUS=$(curl -s -H "Authorization: Bearer $JWT_TOKEN" \
        "http://localhost:8080/api/v1/tasks/$TASK_ID" | jq -r '.data.status')
    if [ "$STATUS" != "success" ]; then
        ALL_SUCCESS=false
        break
    fi
done

if [ "$ALL_SUCCESS" = "true" ]; then
    record_test "所有分布式任务成功" "pass"
else
    record_test "所有分布式任务成功" "fail"
fi

# 测试6: 不同参数格式
echo -e "\n${YELLOW}6. 多参数格式测试${NC}"

# 测试详细格式
TASK6=$(curl -s -X POST "http://localhost:8080/api/v1/tasks" \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
        \"name\": \"详细格式测试\",
        \"task_type\": \"ansible_adhoc\",
        \"priority\": 5,
        \"params\": {
            \"hosts\": [{
                \"ip\": \"127.0.0.1\",
                \"port\": 22,
                \"credential_id\": $GOOD_CRED
            }],
            \"module\": \"shell\",
            \"args\": \"hostname\"
        },
        \"timeout\": 30
    }" | jq -r '.data.task_id')

sleep 5
RESULT6=$(curl -s -H "Authorization: Bearer $JWT_TOKEN" \
    "http://localhost:8080/api/v1/tasks/$TASK6" | jq -r '.data.status')
if [ "$RESULT6" = "success" ]; then
    record_test "详细参数格式" "pass"
else
    record_test "详细参数格式" "fail"
fi

# 清理
echo -e "\n${YELLOW}清理测试环境...${NC}"
kill $PID1 $PID2 $PID3 2>/dev/null || true

# 删除测试资源
curl -s -X DELETE "http://localhost:8080/api/v1/hosts/$GOOD_HOST" \
    -H "Authorization: Bearer $JWT_TOKEN" > /dev/null
curl -s -X DELETE "http://localhost:8080/api/v1/hosts/$BAD_HOST" \
    -H "Authorization: Bearer $JWT_TOKEN" > /dev/null
curl -s -X DELETE "http://localhost:8080/api/v1/credentials/$GOOD_CRED" \
    -H "Authorization: Bearer $JWT_TOKEN" > /dev/null
curl -s -X DELETE "http://localhost:8080/api/v1/credentials/$BAD_CRED" \
    -H "Authorization: Bearer $JWT_TOKEN" > /dev/null

# 测试总结
echo -e "\n${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║            测试结果总结                ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"

echo -e "\n总测试数: $TOTAL_TESTS"
echo -e "${GREEN}通过: $PASSED_TESTS${NC}"
echo -e "${RED}失败: $FAILED_TESTS${NC}"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}✅ 所有测试通过！${NC}"
    exit 0
else
    echo -e "\n${RED}❌ 有测试失败，请检查日志${NC}"
    exit 1
fi