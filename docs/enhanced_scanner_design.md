# 增强的任务模板扫描器设计

## 1. 扫描策略

### 1.1 Shell脚本扫描（现有）
- 扫描所有`.sh`文件
- 每个文件作为独立模板
- 从注释中提取参数和描述

### 1.2 Ansible扫描（增强）

#### A. 项目结构识别
```
扫描优先级：
1. Ansible Collection (galaxy.yml)
2. Ansible Role (meta/main.yml)
3. Playbook项目 (site.yml/main.yml)
4. 独立Playbook文件
```

#### B. 模板粒度定义

**1. Role作为模板**
```
roles/webserver/
├── meta/main.yml      # 包含依赖和描述
├── defaults/main.yml  # 默认变量（作为参数）
├── tasks/main.yml     # 任务入口
└── vars/main.yml      # 变量定义
```
- 模板名称：从meta/main.yml提取或使用role名称
- 参数：合并defaults和vars中的变量
- 入口：role名称（执行时使用-r参数）

**2. 主Playbook作为模板**
```
project/
├── site.yml           # 主入口
├── webservers.yml     # 被include的playbook
├── group_vars/
│   └── all.yml       # 全局变量
└── roles/
```
- 模板名称：基于文件名或playbook中的name
- 参数：从group_vars和playbook中的vars提取
- 入口：site.yml

**3. 独立Playbook作为模板**
```
playbooks/
├── deploy_app.yml
├── backup_db.yml
└── update_config.yml
```
- 每个包含`hosts:`的文件作为独立模板

## 2. 参数提取策略

### 2.1 Shell参数提取
```bash
# @param name string "用户名" required
# @param age integer "年龄" default=18
# @param env select "环境" options=dev,test,prod
```

### 2.2 Ansible参数提取

**从多个来源收集：**
1. `defaults/main.yml` - Role默认变量
2. `vars/main.yml` - Role变量
3. `group_vars/*.yml` - 组变量
4. Playbook中的vars部分
5. `{{ variable }}` - 模板中使用的变量

**参数智能识别：**
```yaml
# defaults/main.yml
nginx_port: 80  # 识别为integer类型
enable_ssl: false  # 识别为boolean类型
app_env: production  # 识别为string类型
allowed_hosts:  # 识别为list类型
  - localhost
  - example.com
```

## 3. 实现伪代码

```go
type TemplateScanner interface {
    Scan(repoPath string) ([]*Template, error)
}

type AnsibleScanner struct {
    // 实现细节
}

func (s *AnsibleScanner) Scan(repoPath string) ([]*Template, error) {
    // 1. 识别项目结构
    structure := s.identifyStructure(repoPath)
    
    switch structure.Type {
    case "role":
        return s.scanRole(repoPath)
    case "collection":
        return s.scanCollection(repoPath)
    case "playbook_project":
        return s.scanPlaybookProject(repoPath)
    default:
        return s.scanStandalonePlaybooks(repoPath)
    }
}

func (s *AnsibleScanner) scanRole(rolePath string) ([]*Template, error) {
    // 1. 读取meta/main.yml获取元数据
    meta := s.readMeta(rolePath)
    
    // 2. 收集所有变量
    vars := s.collectVariables(rolePath)
    
    // 3. 生成模板
    return []*Template{{
        Name: meta.Name,
        Type: "ansible_role",
        EntryPoint: filepath.Base(rolePath),
        Parameters: s.variablesToParameters(vars),
    }}, nil
}
```

## 4. 配置选项

允许用户配置扫描行为：

```yaml
scanner:
  ansible:
    # 扫描策略
    scan_strategy: "smart"  # smart|simple|custom
    
    # Role识别
    treat_role_as_template: true
    
    # 变量提取
    extract_vars_from:
      - defaults
      - vars
      - group_vars
      - playbook_vars
    
    # 忽略模式
    ignore_patterns:
      - "test/*"
      - "examples/*"
    
    # 自定义入口文件
    entry_points:
      - site.yml
      - main.yml
      - playbook.yml
```

## 5. 用户界面增强

扫描结果预览：
```json
{
  "scan_results": [
    {
      "type": "ansible_role",
      "name": "nginx",
      "path": "roles/nginx",
      "description": "Install and configure Nginx",
      "entry_point": "nginx",
      "parameters": [
        {
          "name": "nginx_port",
          "type": "integer",
          "default": "80",
          "source": "defaults/main.yml"
        }
      ],
      "files_included": [
        "meta/main.yml",
        "tasks/main.yml",
        "defaults/main.yml"
      ]
    },
    {
      "type": "playbook",
      "name": "Deploy Application",
      "path": "deploy.yml",
      "description": "Deploy the application",
      "entry_point": "deploy.yml",
      "includes": ["roles/app", "roles/db"]
    }
  ]
}
```

## 6. 执行时的处理

根据模板类型，生成不同的执行命令：

**Shell模板：**
```bash
bash /path/to/script.sh param1 param2
```

**Ansible Role模板：**
```bash
ansible-playbook -i inventory site.yml -e "role_name=nginx" -e "nginx_port=8080"
```

**Ansible Playbook模板：**
```bash
ansible-playbook -i inventory deploy.yml -e "app_version=1.2.3"
```