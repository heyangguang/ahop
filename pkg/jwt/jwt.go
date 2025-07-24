package jwt

import (
	"ahop/pkg/config"
	"errors"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims JWT声明
type JWTClaims struct {
	UserID          uint   `json:"user_id"`
	TenantID        uint   `json:"tenant_id"`         // 用户所属租户
	CurrentTenantID uint   `json:"current_tenant_id"` // 当前操作的租户（用于平台管理员切换）
	Username        string `json:"username"`
	IsPlatformAdmin bool   `json:"is_platform_admin"`
	IsTenantAdmin   bool   `json:"is_tenant_admin"`
	jwt.RegisteredClaims
}

// JWTManager JWT管理器
type JWTManager struct {
	secretKey     string
	tokenDuration time.Duration
}

// NewJWTManager 创建JWT管理器
func NewJWTManager(secretKey string, tokenDuration time.Duration) *JWTManager {
	return &JWTManager{
		secretKey:     secretKey,
		tokenDuration: tokenDuration,
	}
}

// GenerateToken 生成JWT令牌
func (manager *JWTManager) GenerateToken(userID, tenantID uint, username string, isPlatformAdmin, isTenantAdmin bool) (string, error) {
	// 默认情况下，当前操作租户等于用户所属租户
	currentTenantID := tenantID

	claims := JWTClaims{
		UserID:          userID,
		TenantID:        tenantID,
		CurrentTenantID: currentTenantID,
		Username:        username,
		IsPlatformAdmin: isPlatformAdmin,
		IsTenantAdmin:   isTenantAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(manager.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "AHOP",
			Subject:   username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(manager.secretKey))
}

// GenerateTokenWithTenant 生成指定当前租户的JWT令牌（用于平台管理员切换租户）
func (manager *JWTManager) GenerateTokenWithTenant(userID, tenantID, currentTenantID uint, username string, isPlatformAdmin, isTenantAdmin bool) (string, error) {
	claims := JWTClaims{
		UserID:          userID,
		TenantID:        tenantID,
		CurrentTenantID: currentTenantID,
		Username:        username,
		IsPlatformAdmin: isPlatformAdmin,
		IsTenantAdmin:   isTenantAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(manager.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "AHOP",
			Subject:   username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(manager.secretKey))
}

// VerifyToken 验证JWT令牌
func (manager *JWTManager) VerifyToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&JWTClaims{},
		func(token *jwt.Token) (interface{}, error) {
			// 验证签名方法
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("意外的签名方法")
			}
			return []byte(manager.secretKey), nil
		},
	)

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, errors.New("无法解析token声明")
	}

	return claims, nil
}

// RefreshToken 刷新令牌
func (manager *JWTManager) RefreshToken(tokenString string) (string, error) {
	claims, err := manager.VerifyToken(tokenString)
	if err != nil {
		return "", err
	}

	// 生成新的令牌，保持当前租户ID
	return manager.GenerateTokenWithTenant(
		claims.UserID,
		claims.TenantID,
		claims.CurrentTenantID,
		claims.Username,
		claims.IsPlatformAdmin,
		claims.IsTenantAdmin,
	)
}

// GetTokenDuration 获取令牌有效期
func (manager *JWTManager) GetTokenDuration() time.Duration {
	return manager.tokenDuration
}

// 单例实现
var (
	defaultManager *JWTManager
	once           sync.Once
)

// GetJWTManager 获取全局JWT管理器实例
func GetJWTManager() *JWTManager {
	once.Do(func() {
		cfg := config.GetConfig()
		tokenDuration, err := time.ParseDuration(cfg.JWT.TokenDuration)
		if err != nil {
			tokenDuration = 24 * time.Hour
		}
		defaultManager = NewJWTManager(cfg.JWT.SecretKey, tokenDuration)
	})
	return defaultManager
}
