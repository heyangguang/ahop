package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
)

// Decrypt 解密凭证中的敏感数据（使用GCM模式）
func Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	key := getEncryptionKey()
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertextBytes) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := ciphertextBytes[:nonceSize], ciphertextBytes[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %v", err)
	}

	return string(plaintext), nil
}

// getEncryptionKey 获取加密密钥
func getEncryptionKey() []byte {
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	if key == "" {
		key = "ahop-credential-encryption-key32" // 默认密钥，与主项目保持一致
	}

	// 确保密钥长度为32字节（AES-256）
	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		// 如果密钥不足32字节，用0填充
		tmp := make([]byte, 32)
		copy(tmp, keyBytes)
		return tmp
	}
	
	// 如果密钥超过32字节，截取前32字节
	return keyBytes[:32]
}