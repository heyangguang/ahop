# Ansible 参数定义指南

## 概述

AHOP平台支持通过 `survey.yml`、`survey.yaml` 或 `survey.json` 文件来定义Ansible项目的参数。这些参数会在执行任务时通过Web界面收集用户输入。

## 文件位置

Survey文件应放置在Ansible项目的根目录下：
```
ansible-project/
├── survey.yml         # 参数定义文件（推荐）
├── survey.yaml        # 或使用.yaml扩展名
├── survey.json        # 或使用JSON格式
├── playbooks/
├── roles/
└── ...
```

## Survey文件格式

### YAML格式（推荐）

```yaml
name: 项目名称
description: 项目描述信息
spec:
  - type: text                      # 参数类型
    variable: hostname              # 变量名（在playbook中使用）
    question_name: 目标主机         # 显示给用户的问题标题
    question_description: 请输入目标主机的IP地址或主机名  # 问题描述
    required: true                  # 是否必填
    default: "192.168.1.100"       # 默认值
    
  - type: password
    variable: ssh_password
    question_name: SSH密码
    question_description: 输入目标主机的SSH密码
    required: true
    
  - type: integer
    variable: port_number
    question_name: 服务端口
    question_description: 应用服务监听端口
    required: false
    default: 8080
    min: 1024                      # 最小值
    max: 65535                     # 最大值
```

### JSON格式

```json
{
  "name": "项目名称",
  "description": "项目描述信息",
  "spec": [
    {
      "type": "text",
      "variable": "hostname",
      "question_name": "目标主机",
      "question_description": "请输入目标主机的IP地址或主机名",
      "required": true,
      "default": "192.168.1.100"
    }
  ]
}
```

## 参数类型详解

### 1. text - 文本输入
```yaml
- type: text
  variable: app_name
  question_name: 应用名称
  question_description: 请输入应用程序的名称
  required: true
  default: "myapp"
  min_length: 1          # 最小长度（可选）
  max_length: 100        # 最大长度（可选）
```

### 2. textarea - 多行文本
```yaml
- type: textarea
  variable: config_content
  question_name: 配置内容
  question_description: 粘贴配置文件内容
  required: false
  default: |
    # 默认配置
    server:
      port: 8080
```

### 3. password - 密码输入
```yaml
- type: password
  variable: db_password
  question_name: 数据库密码
  question_description: 输入数据库连接密码
  required: true
  min_length: 8          # 密码最小长度
```

### 4. integer - 整数
```yaml
- type: integer
  variable: replica_count
  question_name: 副本数量
  question_description: 设置应用副本数
  required: true
  default: 3
  min: 1                 # 最小值
  max: 10                # 最大值
```

### 5. float - 浮点数
```yaml
- type: float
  variable: cpu_limit
  question_name: CPU限制
  question_description: CPU使用限制（核心数）
  required: false
  default: 1.5
  min: 0.1
  max: 8.0
```

### 6. multiplechoice - 单选
```yaml
- type: multiplechoice
  variable: environment
  question_name: 部署环境
  question_description: 选择部署的目标环境
  required: true
  default: "production"
  choices:
    - "development"
    - "testing"
    - "staging"
    - "production"
```

### 7. multiselect - 多选
```yaml
- type: multiselect
  variable: features
  question_name: 启用功能
  question_description: 选择要启用的功能模块
  required: false
  default:
    - "monitoring"
    - "logging"
  choices:
    - "monitoring"
    - "logging"
    - "backup"
    - "security"
    - "cache"
```

## 完整示例

### 示例1：Web应用部署

`survey.yml`:
```yaml
name: Web应用自动化部署
description: 部署Java Web应用到Tomcat服务器
spec:
  # 基础配置
  - type: text
    variable: target_host
    question_name: 目标服务器
    question_description: 输入要部署的服务器IP地址
    required: true
    
  - type: multiplechoice
    variable: deploy_env
    question_name: 部署环境
    question_description: 选择部署环境
    required: true
    default: "production"
    choices:
      - "development"
      - "testing"
      - "production"
      
  # 应用配置
  - type: text
    variable: app_name
    question_name: 应用名称
    question_description: 应用的标识名称
    required: true
    default: "webapp"
    
  - type: text
    variable: app_version
    question_name: 应用版本
    question_description: 要部署的版本号（如：1.0.0）
    required: true
    
  - type: integer
    variable: app_port
    question_name: 应用端口
    question_description: 应用监听端口
    required: true
    default: 8080
    min: 1024
    max: 65535
    
  # 数据库配置
  - type: text
    variable: db_host
    question_name: 数据库地址
    question_description: MySQL数据库服务器地址
    required: true
    default: "localhost"
    
  - type: integer
    variable: db_port
    question_name: 数据库端口
    required: false
    default: 3306
    
  - type: text
    variable: db_name
    question_name: 数据库名称
    required: true
    
  - type: text
    variable: db_user
    question_name: 数据库用户
    required: true
    default: "root"
    
  - type: password
    variable: db_password
    question_name: 数据库密码
    required: true
    
  # 高级选项
  - type: multiselect
    variable: deploy_modules
    question_name: 部署模块
    question_description: 选择要部署的模块
    required: false
    default:
      - "web"
      - "api"
    choices:
      - "web"
      - "api"
      - "admin"
      - "scheduler"
      
  - type: textarea
    variable: jvm_options
    question_name: JVM参数
    question_description: 自定义JVM启动参数
    required: false
    default: "-Xms512m -Xmx2048m"
```

### 示例2：系统初始化

`survey.yml`:
```yaml
name: Linux系统初始化
description: 初始化新安装的Linux服务器
spec:
  - type: text
    variable: hostname
    question_name: 主机名
    question_description: 设置服务器主机名
    required: true
    
  - type: multiplechoice
    variable: timezone
    question_name: 时区设置
    required: true
    default: "Asia/Shanghai"
    choices:
      - "Asia/Shanghai"
      - "Asia/Hong_Kong"
      - "UTC"
      
  - type: multiselect
    variable: system_packages
    question_name: 系统软件包
    question_description: 选择要安装的基础软件包
    required: false
    default:
      - "vim"
      - "htop"
      - "git"
    choices:
      - "vim"
      - "htop"
      - "git"
      - "wget"
      - "curl"
      - "tmux"
      - "tree"
      
  - type: text
    variable: admin_user
    question_name: 管理员用户名
    required: true
    default: "admin"
    
  - type: password
    variable: admin_password
    question_name: 管理员密码
    required: true
    min_length: 8
    
  - type: textarea
    variable: ssh_public_keys
    question_name: SSH公钥
    question_description: 添加SSH公钥（每行一个）
    required: false
```

## 在Playbook中使用参数

定义的参数可以直接在Ansible playbook中作为变量使用：

```yaml
---
- name: 部署Web应用
  hosts: "{{ target_host }}"
  vars:
    environment: "{{ deploy_env }}"
    
  tasks:
    - name: 创建应用目录
      file:
        path: "/opt/{{ app_name }}"
        state: directory
        
    - name: 配置数据库连接
      template:
        src: db.conf.j2
        dest: "/opt/{{ app_name }}/db.conf"
      vars:
        database_host: "{{ db_host }}"
        database_port: "{{ db_port }}"
        database_name: "{{ db_name }}"
        database_user: "{{ db_user }}"
        database_pass: "{{ db_password }}"
```

## 最佳实践

1. **参数命名规范**
   - 使用小写字母和下划线
   - 避免使用Ansible保留字
   - 名称要有描述性

2. **默认值设置**
   - 尽可能提供合理的默认值
   - 敏感信息不设置默认值

3. **参数验证**
   - 使用min/max限制数值范围
   - 使用min_length/max_length限制字符串长度
   - 使用choices限制可选值

4. **参数分组**
   - 相关参数放在一起
   - 使用注释分隔不同组别
   - 按重要性排序（必填在前）

5. **描述信息**
   - question_name要简洁明了
   - question_description提供详细说明
   - 包含格式要求或示例

## 注意事项

1. Survey文件必须是有效的YAML或JSON格式
2. 变量名不能包含特殊字符（除了下划线）
3. 必填参数（required: true）在执行时必须提供值
4. 参数类型要与实际使用匹配（如端口号用integer）
5. 密码类型参数在界面上会隐藏输入内容