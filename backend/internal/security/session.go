// Package security 提供认证与安全工具。
// 本文件：会话（session）模块——JWT 签发/校验/吊销与登录态 Cookie 的唯一实现。
// 中间件、AuthService 与各登录路径都消费本模块的 interface，黑名单 key 与
// Bearer/Cookie 解析逻辑不再散落各处。
package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"forklift-training/internal/cache"
	"forklift-training/internal/config"
)

// Claims JWT 声明。
type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// CookieConfig 登录态 Cookie 配置（父域名共享登录）。
type CookieConfig struct {
	Name   string
	Domain string
	Secure bool
}

// BlacklistStore 黑名单存储接口（生产 Redis，测试内存实现）。
type BlacklistStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

// RedisBlacklistStore 基于全局 Redis 缓存的黑名单存储。
type RedisBlacklistStore struct{}

// Get 读取黑名单条目。
func (RedisBlacklistStore) Get(ctx context.Context, key string) (string, error) {
	return cache.Get(ctx, key)
}

// Set 写入黑名单条目。
func (RedisBlacklistStore) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return cache.Set(ctx, key, value, ttl)
}

// Session 会话模块：签发（issue）/ 校验（verify）/ 吊销（revoke）JWT，
// 并负责 Bearer/Cookie 令牌提取与登录态 Cookie 写清除。
type Session struct {
	jwtSecret string
	jwtExpiry time.Duration
	cookie    CookieConfig
	blacklist BlacklistStore
}

// NewSession 构造会话模块（默认 Redis 黑名单存储）。
func NewSession(jwtSecret string, jwtExpiry time.Duration, cookie CookieConfig) *Session {
	return NewSessionWithBlacklist(jwtSecret, jwtExpiry, cookie, RedisBlacklistStore{})
}

// NewSessionWithBlacklist 构造会话模块，黑名单存储可注入（ADR-0002：
// 生产 Redis，测试内存实现；默认存储见 NewSession）。
func NewSessionWithBlacklist(jwtSecret string, jwtExpiry time.Duration, cookie CookieConfig, blacklist BlacklistStore) *Session {
	return &Session{
		jwtSecret: jwtSecret,
		jwtExpiry: jwtExpiry,
		cookie:    cookie,
		blacklist: blacklist,
	}
}

// SessionFromConfig 从应用配置构造会话模块（黑名单固定为 Redis 存储）。
func SessionFromConfig(cfg *config.Config) *Session {
	return NewSessionWithBlacklist(cfg.JWTSecretKey, cfg.JWTExpiry(), CookieConfig{
		Name:   cfg.AuthCookie.Name,
		Domain: cfg.AuthCookie.Domain,
		Secure: cfg.AuthCookie.Secure,
	}, RedisBlacklistStore{})
}

// Issue 签发 JWT，claims 结构：user_id/username/role，过期时长由配置决定（默认 24 小时）。
func (s *Session) Issue(userID int, username, role string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

// Verify 解析并校验 JWT（显式校验签名算法，拒绝非 HMAC 算法，防止 alg=none 攻击）。
func (s *Session) Verify(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// Revoke 将 token 写入黑名单，TTL = token 剩余有效期。
// 无效或已过期的 token 无需吊销，静默返回。
func (s *Session) Revoke(ctx context.Context, tokenStr string) error {
	claims, err := s.Verify(tokenStr)
	if err != nil || claims.ExpiresAt == nil {
		return nil
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil
	}
	return s.blacklist.Set(ctx, s.blacklistKey(tokenStr), "1", ttl)
}

// IsRevoked 判断 token 是否已被吊销。
// 存储读异常（非 key 不存在）时放行，避免 Redis 宕机导致全员无法登录。
func (s *Session) IsRevoked(ctx context.Context, tokenStr string) (bool, error) {
	_, err := s.blacklist.Get(ctx, s.blacklistKey(tokenStr))
	if err != nil {
		return false, nil
	}
	return true, nil
}

// ExtractToken 提取登录令牌：优先 Bearer 头，其次父域名 Cookie（子域名共享登录）。
// 纯函数，便于测试；authHeader 为空或非 Bearer 时回退到 cookieValue。
func (s *Session) ExtractToken(authHeader, cookieValue string) string {
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	return cookieValue
}

// SetCookie 将 JWT 写入父域名 httpOnly Cookie，实现子域名间登录态共享。
func (s *Session) SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookie.Name,
		Value:    token,
		Path:     "/",
		Domain:   s.cookie.Domain,
		MaxAge:   int(s.jwtExpiry.Seconds()),
		HttpOnly: true,
		Secure:   s.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearCookie 清除登录 Cookie（登出时调用）。
func (s *Session) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookie.Name,
		Value:    "",
		Path:     "/",
		Domain:   s.cookie.Domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// blacklistKey 黑名单缓存 key（唯一实现，不再散落字面量）。
func (s *Session) blacklistKey(tokenStr string) string {
	tokenHash := sha256.Sum256([]byte(tokenStr))
	return "jwt:blacklist:" + hex.EncodeToString(tokenHash[:])
}
