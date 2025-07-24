# 凭证管理安全性分析

## 当前设计

### 优点
1. 模型层使用 `json:"-"` 防止敏感信息意外泄露
2. 凭证存储使用 AES-256-GCM 加密
3. 需要特殊权限 `credential:decrypt` 才能获取明文
4. 所有解密操作都有审计日志

### 风险点
1. 解密后的明文通过 HTTP 响应返回
2. 可能被日志、缓存、代理等中间件捕获
3. 调试环境可能泄露敏感信息

## 更安全的替代方案

### 方案一：临时访问令牌
```go
// 不直接返回凭证，而是返回一个临时令牌
type DecryptResponse struct {
    AccessToken string    `json:"access_token"`
    ExpiresIn   int       `json:"expires_in"`  // 秒
    TokenType   string    `json:"token_type"`
}

// 客户端使用令牌通过专用加密通道获取凭证
```

### 方案二：端到端加密
```go
// 客户端提供公钥，服务端用公钥加密凭证
type DecryptRequest struct {
    CredentialID uint   `json:"credential_id"`
    PublicKey    string `json:"public_key"`
    Purpose      string `json:"purpose"`
}

type DecryptResponse struct {
    EncryptedData string `json:"encrypted_data"` // 用客户端公钥加密
    Algorithm     string `json:"algorithm"`
}
```

### 方案三：一次性密码（OTP）
```go
// 返回一次性使用的凭证副本
type OTPCredential struct {
    ID          string    `json:"id"`          // 一次性ID
    ExpiresAt   time.Time `json:"expires_at"`  // 5分钟后过期
    UseEndpoint string    `json:"use_endpoint"` // 使用此凭证的专用端点
}
```

### 方案四：分段传输
```go
// 将敏感信息分成多个部分，通过不同通道传输
type PartialCredential struct {
    PartID    string `json:"part_id"`
    PartCount int    `json:"part_count"`
    Data      string `json:"data"`
}
```

## 建议的改进

### 1. 短期改进（保持现有架构）
- 添加响应缓存控制头：`Cache-Control: no-store`
- 添加安全响应头：`X-Content-Type-Options: nosniff`
- 对敏感字段进行二次加密，客户端解密
- 限制解密 API 的调用频率

### 2. 中期改进
- 实现临时访问令牌机制
- 添加 IP 白名单限制
- 实现细粒度的字段级权限控制
- 添加多因素认证（MFA）

### 3. 长期改进
- 实现硬件安全模块（HSM）集成
- 使用密钥管理服务（KMS）
- 实现零知识证明协议
- 部署专用的密钥分发服务

## 最佳实践建议

1. **最小权限原则**：
   - 细分解密权限（如 `credential:decrypt:password`）
   - 限制单个用户可解密的凭证数量

2. **审计和监控**：
   - 实时监控异常解密行为
   - 设置解密频率告警
   - 定期审查解密日志

3. **数据生命周期**：
   - 自动轮换凭证
   - 定期清理过期凭证
   - 实现凭证版本管理

4. **网络安全**：
   - 强制使用 TLS 1.3+
   - 实现证书固定（Certificate Pinning）
   - 使用 VPN 或专线访问

## 测试环境的特殊考虑

对于测试环境，可以：
1. 使用模拟凭证而非真实凭证
2. 实现凭证脱敏功能
3. 添加测试环境标识，限制功能
4. 自动清理测试数据

## 结论

当前的实现对于 MVP 或内部系统是可以接受的，但对于生产环境，特别是处理高度敏感信息的场景，建议采用更安全的方案。安全性和易用性需要根据具体业务场景进行权衡。