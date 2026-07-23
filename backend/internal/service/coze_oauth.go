// Package service Coze OAuth JWT 签发。
package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	cozeTokenURL = "https://api.coze.cn/api/permission/oauth2/token"
	cozeAud      = "api.coze.cn"
)

// CozeOAuthService Coze OAuth 服务。
type CozeOAuthService struct {
	appID          string
	kid            string
	privateKey     string
	privateKeyPath string

	mu              sync.Mutex
	cachedKey       string
	cachedKeySource string
}

// NewCozeOAuthService 创建 Coze OAuth 服务实例。
func NewCozeOAuthService(appID, kid, privateKey, privateKeyPath string) *CozeOAuthService {
	return &CozeOAuthService{
		appID:          appID,
		kid:            kid,
		privateKey:     privateKey,
		privateKeyPath: privateKeyPath,
	}
}

// GetAccessToken 获取 Coze 访问令牌。
func (s *CozeOAuthService) GetAccessToken() (map[string]any, error) {
	if s.appID == "" || s.kid == "" {
		return nil, errors.New("Coze OAuth 配置不完整")
	}
	encodedJWT, err := s.generateJWT()
	if err != nil {
		return nil, fmt.Errorf("生成 JWT 失败: %w", err)
	}
	body := map[string]any{
		"grant_type":       "urn:ietf:params:oauth:grant-type:jwt-bearer",
		"duration_seconds": 86399,
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", cozeTokenURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+encodedJWT)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Coze OAuth 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Coze OAuth token 请求失败，状态码 %d", resp.StatusCode)
	}

	var data map[string]any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, fmt.Errorf("解析 Coze OAuth 响应失败: %w", err)
	}

	accessToken, _ := data["access_token"].(string)
	if accessToken == "" {
		errMsg, _ := data["error_message"].(string)
		if errMsg == "" {
			errMsg, _ = data["error_code"].(string)
		}
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return nil, fmt.Errorf("Coze OAuth token 错误: %s", errMsg)
	}

	expiresIn := int64(0)
	if v, ok := data["expires_in"].(float64); ok {
		expiresIn = int64(v)
	}
	return map[string]any{
		"access_token": accessToken,
		"expires_in":   expiresIn,
	}, nil
}

// generateJWT 生成 RS256 JWT。
func (s *CozeOAuthService) generateJWT() (string, error) {
	privateKeyContent, err := s.readPrivateKey()
	if err != nil {
		return "", err
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyContent))
	if err != nil {
		return "", fmt.Errorf("解析 RSA 私钥失败: %w", err)
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    s.appID,
		Audience:  jwt.ClaimStrings{cozeAud},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(600 * time.Second)),
		ID:        uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.kid
	return token.SignedString(key)
}

// readPrivateKey 读取私钥（带缓存）。
func (s *CozeOAuthService) readPrivateKey() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.privateKey != "" {
		source := "env:COZE_OAUTH_PRIVATE_KEY"
		if s.cachedKey != "" && s.cachedKeySource == source {
			return s.cachedKey, nil
		}
		s.cachedKey = s.privateKey
		s.cachedKeySource = source
		return s.cachedKey, nil
	}

	if s.privateKeyPath != "" {
		source := "file:" + s.privateKeyPath
		if s.cachedKey != "" && s.cachedKeySource == source {
			return s.cachedKey, nil
		}
		content, err := os.ReadFile(s.privateKeyPath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("私钥文件不存在: %s", s.privateKeyPath)
			}
			return "", fmt.Errorf("读取私钥文件失败: %w", err)
		}
		s.cachedKey = string(content)
		s.cachedKeySource = source
		return s.cachedKey, nil
	}

	return "", errors.New("无私钥可用：请设置 COZE_OAUTH_PRIVATE_KEY 或 COZE_OAUTH_PRIVATE_KEY_PATH")
}
