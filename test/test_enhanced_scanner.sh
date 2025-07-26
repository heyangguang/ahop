#!/bin/bash

# 测试增强的智能扫描功能

set -e

# 基本配置
BASE_URL="http://localhost:8080"
ADMIN_USER="admin"
ADMIN_PASS="Admin@123"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${YELLOW}======== 测试增强的智能扫描功能 ========${NC}"

# 1. 创建测试目录结构
echo -e "\n${YELLOW}1. 创建测试目录结构${NC}"
TEST_REPO="/tmp/enhanced_scan_test_$$"
mkdir -p $TEST_REPO

# 创建一个Ansible项目结构
mkdir -p $TEST_REPO/ansible-project/{roles,group_vars,inventory}
mkdir -p $TEST_REPO/ansible-project/roles/{nginx,mysql}/{tasks,defaults,handlers,templates}

# 创建项目入口文件
cat > $TEST_REPO/ansible-project/site.yml << 'EOF'
---
- name: Deploy Complete Application Stack
  hosts: all
  vars:
    app_version: "{{ app_version | default('1.0.0') }}"
    deploy_env: "{{ deploy_env }}"
  roles:
    - nginx
    - mysql
EOF

# 创建group_vars
cat > $TEST_REPO/ansible-project/group_vars/all.yml << 'EOF'
---
app_domain: example.com
enable_ssl: false
EOF

# 创建Nginx Role
cat > $TEST_REPO/ansible-project/roles/nginx/tasks/main.yml << 'EOF'
---
- name: Install Nginx
  apt:
    name: nginx
    state: present

- name: Configure Nginx
  template:
    src: nginx.conf.j2
    dest: /etc/nginx/nginx.conf
  notify: restart nginx
EOF

cat > $TEST_REPO/ansible-project/roles/nginx/defaults/main.yml << 'EOF'
---
nginx_port: 80
nginx_worker_processes: auto
nginx_worker_connections: 1024
EOF

cat > $TEST_REPO/ansible-project/roles/nginx/handlers/main.yml << 'EOF'
---
- name: restart nginx
  service:
    name: nginx
    state: restarted
EOF

# 创建MySQL Role
cat > $TEST_REPO/ansible-project/roles/mysql/tasks/main.yml << 'EOF'
---
- name: Install MySQL
  apt:
    name: mysql-server
    state: present

- name: Configure MySQL
  template:
    src: my.cnf.j2
    dest: /etc/mysql/my.cnf
EOF

cat > $TEST_REPO/ansible-project/roles/mysql/defaults/main.yml << 'EOF'
---
mysql_port: 3306
mysql_bind_address: 127.0.0.1
mysql_max_connections: 200
EOF

# 创建独立的Ansible Roles
mkdir -p $TEST_REPO/shared-roles/{redis,mongodb}/{tasks,defaults}

cat > $TEST_REPO/shared-roles/redis/tasks/main.yml << 'EOF'
---
- name: Install Redis
  apt:
    name: redis-server
    state: present
EOF

cat > $TEST_REPO/shared-roles/redis/defaults/main.yml << 'EOF'
---
redis_port: 6379
redis_bind: 127.0.0.1
EOF

# 创建独立的Playbook文件
mkdir -p $TEST_REPO/playbooks

cat > $TEST_REPO/playbooks/backup.yml << 'EOF'
---
- name: Backup Database
  hosts: database
  vars:
    backup_path: "{{ backup_path }}"
    retention_days: "{{ retention_days | default(7) }}"
  tasks:
    - name: Create backup directory
      file:
        path: "{{ backup_path }}"
        state: directory
    
    - name: Backup MySQL
      shell: mysqldump --all-databases > {{ backup_path }}/backup.sql
EOF

# 创建Shell脚本
mkdir -p $TEST_REPO/scripts

cat > $TEST_REPO/scripts/health_check.sh << 'EOF'
#!/bin/bash
# @description Health check script for services
# @param service string "Service name to check" required
# @param timeout integer "Timeout in seconds" default=30
# @param notify_email string "Email for notifications" options=admin@example.com,ops@example.com

SERVICE=$1
TIMEOUT=${2:-30}

echo "Checking health of $SERVICE..."
EOF

chmod +x $TEST_REPO/scripts/health_check.sh

# 初始化Git仓库
cd $TEST_REPO
git init
git add .
git commit -m "Initial test repository"

echo -e "${GREEN}✓ 测试仓库创建完成${NC}"
echo "仓库结构："
tree -L 3 $TEST_REPO 2>/dev/null || find $TEST_REPO -type f | head -20

# 2. 预期的扫描结果
echo -e "\n${YELLOW}2. 预期扫描结果：${NC}"
echo -e "${BLUE}应该识别出以下模板：${NC}"
echo "1. [Ansible项目] Ansible project 项目"
echo "   - 路径: /ansible-project"
echo "   - 入口: site.yml"
echo "   - 包含: nginx, mysql roles"
echo ""
echo "2. [独立Role] Redis Role"
echo "   - 路径: /shared-roles/redis"
echo ""
echo "3. [独立Role] Mongodb Role"
echo "   - 路径: /shared-roles/mongodb"
echo ""
echo "4. [独立Playbook] Backup Database"
echo "   - 路径: /playbooks/backup.yml"
echo ""
echo "5. [Shell脚本] Health check"
echo "   - 路径: /scripts/health_check.sh"
echo ""
echo -e "${RED}不应该识别的：${NC}"
echo "✗ ansible-project/roles/nginx (属于项目的一部分)"
echo "✗ ansible-project/roles/mysql (属于项目的一部分)"

# 清理
echo -e "\n${YELLOW}测试仓库路径: $TEST_REPO${NC}"
echo "运行完成后，可以手动删除: rm -rf $TEST_REPO"

echo -e "\n${GREEN}======== 测试准备完成 ========${NC}"
echo -e "${YELLOW}现在可以：${NC}"
echo "1. 在AHOP中创建Git仓库，URL使用: file://$TEST_REPO"
echo "2. 执行同步操作"
echo "3. 执行扫描操作，查看识别结果"