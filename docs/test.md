1. 登录获取Token

POST /api/v1/auth/login
Content-Type: application/json

{
"username": "admin",
"password": "Admin@123"
}

2. 创建凭证（用于私有仓库）

SSH密钥凭证：

POST /api/v1/credentials
Authorization: Bearer {token}
Content-Type: application/json

{
"name": "Git SSH Key",
"code": "git_ssh_key_001",
"type": "ssh_key",
"username": "git",
"private_key": "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----",
"description": "用于Git私有仓库的SSH密钥"
}

密码凭证：

POST /api/v1/credentials
Authorization: Bearer {token}
Content-Type: application/json

{
"name": "Git Password",
"code": "git_password_001",
"type": "password",
"username": "your_username",
"password": "your_password",
"description": "用于Git私有仓库的用户名密码"
}

3. 创建Git仓库

公开仓库：

POST /api/v1/git-repositories
Authorization: Bearer {token}
Content-Type: application/json

{
"name": "Gin Framework",
"code": "gin_framework",
"url": "https://github.com/gin-gonic/gin.git",
"branch": "master",
"is_public": true,
"sync_enabled": false,
"description": "Gin Web框架源码"
}

私有仓库（需要凭证）：

POST /api/v1/git-repositories
Authorization: Bearer {token}
Content-Type: application/json

{
"name": "Private Repo",
"code": "private_repo_001",
"url": "https://github.com/your-org/private-repo.git",
"branch": "main",
"is_public": false,
"credential_id": {credential_id},
"sync_enabled": false,
"description": "私有仓库测试"
}

4. 手动同步仓库（不扫描）

POST /api/v1/git-repositories/{repository_id}/sync
Authorization: Bearer {token}

5. 查看同步日志

GET /api/v1/git-repositories/{repository_id}/sync-logs
Authorization: Bearer {token}

6. 查看所有仓库

GET /api/v1/git-repositories?page=1&page_size=10
Authorization: Bearer {token}

7. 查看单个仓库详情

GET /api/v1/git-repositories/{repository_id}
Authorization: Bearer {token}

8. 更新仓库

PUT /api/v1/git-repositories/{repository_id}
Authorization: Bearer {token}
Content-Type: application/json

{
"name": "Updated Name",
"branch": "develop",
"sync_enabled": true,
"sync_interval": 60,
"description": "更新后的描述"
}

9. 删除仓库

DELETE /api/v1/git-repositories/{repository_id}
Authorization: Bearer {token}

10. 扫描仓库中的任务模板

POST /api/v1/git-repositories/{repository_id}/scan-templates
Authorization: Bearer {token}

11. 查看任务模板（扫描后生成的）

GET /api/v1/task-templates?repository_id={repository_id}
Authorization: Bearer {token}

测试流程建议：

1. 先创建一个公开仓库测试基本同步功能
2. 执行手动同步，检查：
   - /data/ahop/repos/1/{repo_id}/ 目录是否创建
   - Git仓库是否成功克隆
   - 同步日志状态是否为success
   - duration、from_commit、to_commit等字段是否正确
3. 再次执行同步，验证：
   - 是否执行pull而不是重新clone
   - from_commit是否有值（上次的to_commit）
4. 执行扫描模板（使用POST /api/v1/git-repositories/{id}/scan-templates），检查：
   - 是否生成了任务模板
   - 扫描结果是否正确
5. 删除仓库，验证：
   - 文件系统上的目录是否被删除
   - 数据库记录是否被清理