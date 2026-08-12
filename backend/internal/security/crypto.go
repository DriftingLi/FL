// Package security 提供敏感字段的静态加密工具。
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// EncryptedPrefix 加密值的版本前缀，用于识别与兼容。
const EncryptedPrefix = "enc:v1:"

// EncryptSecret 使用 masterKey 派生 AES-256-GCM 密钥加密明文。
// 返回格式：enc:v1:<base64(nonce + ciphertext)>。
// masterKey 或明文为空时原样返回（保持向后兼容）。
func EncryptSecret(plaintext, masterKey string) (string, error) {
	if masterKey == "" || plaintext == "" {
		return plaintext, nil
	}
	key := sha256.Sum256([]byte(masterKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("创建 AES 加密器失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 失败: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成随机 nonce 失败: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return EncryptedPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret 解密带 EncryptedPrefix 前缀的密文；
// 非前缀值视为历史明文原样返回（兼容加密改造前已入库的数据）。
func DecryptSecret(stored, masterKey string) (string, error) {
	if masterKey == "" || stored == "" || !strings.HasPrefix(stored, EncryptedPrefix) {
		return stored, nil
	}
	raw := strings.TrimPrefix(stored, EncryptedPrefix)
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("密文 base64 解码失败: %w", err)
	}
	key := sha256.Sum256([]byte(masterKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("创建 AES 解密器失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 失败: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("密文长度不足")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败（请检查 SECRET_KEY 是否与加密时一致）: %w", err)
	}
	return string(plain), nil
}
