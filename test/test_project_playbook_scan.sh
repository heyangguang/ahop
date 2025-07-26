#!/bin/bash

# 测试项目内独立playbook的识别

set -e

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${YELLOW}======== 测试项目内独立Playbook识别 ========${NC}"

# 1. 创建测试目录结构（模拟仓库69的结构）
echo -e "\n${YELLOW}1. 创建测试目录结构${NC}"
TEST_REPO="/tmp/project_playbook_test_$$"
mkdir -p $TEST_REPO/{roles,group_vars,inventory}
mkdir -p $TEST_REPO/roles/{common,app}/{tasks,defaults}

# 创建项目入口文件 site.yml
cat > $TEST_REPO/site.yml << 'EOF'
---
- name: 部署Web应用
  hosts: all
  become: yes
  
  pre_tasks:
    - name: 更新apt缓存
      apt:
        update_cache: yes
      when: ansible_os_family == "Debian"

- name: 配置应用服务器
  hosts: app_servers
  become: yes
  roles:
    - common
    - app
EOF

# 创建独立的playbook init_system.yml
cat > $TEST_REPO/init_system.yml << 'EOF'
---
- name: 系统初始化配置
  hosts: all
  become: yes
  vars:
    timezone: Asia/Shanghai
    common_packages:
      - vim
      - curl
      - wget
    ssh_port: 22022
    allowed_ips:
      - "10.0.0.0/8"
      - "172.16.0.0/12"
  tasks:
    - name: 设置时区
      timezone:
        name: "{{ timezone }}"
    
    - name: 安装常用软件包
      apt:
        name: "{{ common_packages }}"
        state: present
EOF

# 创建另一个独立playbook
cat > $TEST_REPO/backup_database.yml << 'EOF'
---
- name: 备份数据库
  hosts: database
  vars:
    backup_dir: "/backup/mysql"
    retention_days: 7
  tasks:
    - name: 创建备份目录
      file:
        path: "{{ backup_dir }}"
        state: directory
EOF

# 创建group_vars
cat > $TEST_REPO/group_vars/all.yml << 'EOF'
---
app_name: myapp
app_port: 8080
app_env: production
enable_monitoring: true
EOF

# 创建role
cat > $TEST_REPO/roles/common/tasks/main.yml << 'EOF'
---
- name: Install basic packages
  apt:
    name: "{{ item }}"
  loop:
    - git
    - python3
EOF

cat > $TEST_REPO/roles/common/defaults/main.yml << 'EOF'
---
common_user: deploy
common_group: deploy
EOF

# 创建一个shell脚本
cat > $TEST_REPO/deploy.sh << 'EOF'
#!/bin/bash
# @description 部署脚本
# @param version string "版本号" required
# @param env string "环境" options=dev,test,prod

echo "Deploying version $1 to $2"
EOF
chmod +x $TEST_REPO/deploy.sh

# 初始化Git仓库
cd $TEST_REPO
git init
git add .
git commit -m "Test repository"

echo -e "${GREEN}✓ 测试仓库创建完成${NC}"

# 2. 显示目录结构
echo -e "\n${YELLOW}2. 仓库结构：${NC}"
find $TEST_REPO -type f -name "*.yml" -o -name "*.sh" | grep -v ".git" | sort

# 3. 预期结果
echo -e "\n${YELLOW}3. 预期扫描结果：${NC}"
echo -e "${BLUE}应该识别出：${NC}"
echo "1. [Ansible项目] 整个项目"
echo "   - 入口: site.yml"
echo "   - 包含roles: common, app"
echo "   - 参数: app_name, app_port, app_env等（从group_vars提取）"
echo ""
echo "2. [独立Playbook] init_system.yml"
echo "   - Context: part_of_project（标记为项目的一部分）"
echo "   - 参数: timezone, common_packages, ssh_port, allowed_ips"
echo ""
echo "3. [独立Playbook] backup_database.yml"
echo "   - Context: part_of_project"
echo "   - 参数: backup_dir, retention_days"
echo ""
echo "4. [Shell脚本] deploy.sh"
echo "   - Context: part_of_project"
echo "   - 参数: version, env"

echo -e "\n${GREEN}======== 测试准备完成 ========${NC}"
echo -e "${YELLOW}测试仓库路径: $TEST_REPO${NC}"