// Package security 提供认证与安全工具。
// 本文件：会话（session）模块——JWT 签发/校验/吊销与登录态 Cookie 的唯一实现。
// 中间件、AuthService 与各登录路径都消费本模块的 interface，黑名单 key 与
// Bearer/Cookie 解析逻辑不再散落各处。
//
// 双令牌会话（ADR-0012）：access（2h，鉴权中间件专用，不入黑名单）+
// refresh（7 天，刷新端点专用，轮换时旧值立即入黑名单防重放）；登出吊销 refresh。
package security

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"forklift-training/internal/cache"
	"forklift-training/internal/config"
)

// TokenType 令牌类型（双令牌会话，ADR-0012）：access 供鉴权中间件用（短生命周期、不入黑名单），
// refresh 仅刷新端点用（长周期、可轮换吊销）。
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// defaultRefreshExpiry 默认 refresh token 有效期（JWT_REFRESH_EXPIRES_DAYS=7，可配置覆盖）。
const defaultRefreshExpiry = 7 * 24 * time.Hour

// Claims JWT 声明。
type Claims struct {
	UserID    int    `json:"user_id"`
	Account   string `json:"account"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// CookieConfig 登录态 Cookie 配置（父域名共享登录）。
type CookieConfig struct {
	Name   string
	Domain string
	Secure bool
}

// BlacklistStore 黑名单存储接口（生产 Redis，测试内存实现）。
// PutIfAbsent 是会话轮换原子性的落点（SETNX 语义）：以「写入即抢占」同时完成
// 吊销与并发互斥，并发双刷同一 refresh 恰有一路成功。
type BlacklistStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	PutIfAbsent(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
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

// PutIfAbsent 原子写入：key 不存在时写入并返回 true，已存在返回 false（SETNX 语义）。
func (RedisBlacklistStore) PutIfAbsent(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return cache.SetNX(ctx, key, value, ttl)
}

// Session 会话模块：签发（issue）/ 校验（verify）/ 吊销（revoke）JWT，
// 并负责 Bearer/Cookie 令牌提取与登录态 Cookie 写清除。
type Session struct {
	jwtSecret     string
	jwtExpiry     time.Duration
	refreshExpiry time.Duration
	cookie        CookieConfig
	blacklist     BlacklistStore
}

// NewSession 构造会话模块（默认 Redis 黑名单存储；refresh 默认 7 天）。
func NewSession(jwtSecret string, jwtExpiry time.Duration, cookie CookieConfig) *Session {
	return NewSessionWithBlacklistAndRefresh(jwtSecret, jwtExpiry, defaultRefreshExpiry, cookie, RedisBlacklistStore{})
}

// NewSessionWithBlacklistAndRefresh 构造会话模块：黑名单存储与 refresh 有效期均可注入（测试用）。
func NewSessionWithBlacklistAndRefresh(jwtSecret string, jwtExpiry, refreshExpiry time.Duration, cookie CookieConfig, blacklist BlacklistStore) *Session {
	return &Session{
		jwtSecret:     jwtSecret,
		jwtExpiry:     jwtExpiry,
		refreshExpiry: refreshExpiry,
		cookie:        cookie,
		blacklist:     blacklist,
	}
}

// SessionFromConfig 从应用配置构造会话模块（黑名单固定为 Redis 存储，refresh 用配置值）。
func SessionFromConfig(cfg *config.Config) *Session {
	return NewSessionWithBlacklistAndRefresh(cfg.JWTSecretKey, cfg.JWTExpiry(), cfg.JWTRefreshExpiry(), CookieConfig{
		Name:   cfg.AuthCookie.Name,
		Domain: cfg.AuthCookie.Domain,
		Secure: cfg.AuthCookie.Secure,
	}, RedisBlacklistStore{})
}

// Issue 签发 access token（双令牌会话：短生命周期、只供鉴权中间件，ADR-0012）。
// claims：user_id/account/role/token_type=access，过期时长由配置决定（默认 2h）。
func (s *Session) Issue(userID int, account, role string) (string, error) {
	return s.issue(userID, account, role, TokenTypeAccess, s.jwtExpiry)
}

// IssueRefresh 签发 refresh token（仅刷新端点专用，长周期、轮换吊销）。
func (s *Session) IssueRefresh(userID int, account, role string) (string, error) {
	return s.issue(userID, account, role, TokenTypeRefresh, s.refreshExpiry)
}

// IssuePair 一次签发双令牌（登录/刷新共用），返回 (access, refresh, error)。
func (s *Session) IssuePair(userID int, account, role string) (string, string, error) {
	access, err := s.Issue(userID, account, role)
	if err != nil {
		return "", "", err
	}
	refresh, err := s.IssueRefresh(userID, account, role)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// issue 签发单个 JWT（按 token_type 设置 claims 与有效期）。
func (s *Session) issue(userID int, account, role, tokenType string, expiry time.Duration) (string, error) {
	claims := &Claims{
		UserID:    userID,
		Account:   account,
		Role:      role,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   account,
			ID:        randomJWTID(), // 每次签发唯一，保证轮换后的新 token 与旧 token 一定不同
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

// VerifyAccess 校验 access token（鉴权中间件专用）：verify 之上额外要求 token_type=access，
// refresh token 传入鉴权端点直接拒绝。
func (s *Session) VerifyAccess(tokenStr string) (*Claims, error) {
	claims, err := s.verify(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, errors.New("token type must be access")
	}
	return claims, nil
}

// ValidateRefresh 校验 refresh token（刷新端点专用）：要求 token_type=refresh。
func (s *Session) ValidateRefresh(tokenStr string) (*Claims, error) {
	claims, err := s.verify(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeRefresh {
		return nil, errors.New("token type must be refresh")
	}
	return claims, nil
}

// ErrInvalidRefresh 刷新失败的可判定错误：refresh 无效/过期/类型不符/已吊销/已被并发轮换消费。
// 刷新端点据此统一按未认证处理（401 防枚举）；其余错误按服务器内部错误处理。
var ErrInvalidRefresh = errors.New("invalid refresh token")

// RotateRefresh 原子刷新轮换（ADR-0016）：校验 → 黑名单原子抢占 → 签发新对。
// 抢占即吊销：在旧 refresh 的黑名单 key 上 PutIfAbsent（SETNX），成功的一路同时完成旧值吊销，
// 并发双刷同一 refresh 恰有一路成功——修复原「查黑名单 → 签发 → 写黑名单」的 check-then-act 竞态。
// 黑名单存储故障时失败关闭（返回非 ErrInvalidRefresh 错误，端点按 500 处理）；与登录链路
// 读黑名单失败放行的取舍不同：轮换失败只影响续期，不放大故障面。
func (s *Session) RotateRefresh(ctx context.Context, refreshToken string) (string, string, error) {
	claims, err := s.ValidateRefresh(refreshToken)
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", ErrInvalidRefresh, err)
	}
	if claims.ExpiresAt == nil || !claims.ExpiresAt.After(time.Now()) {
		return "", "", fmt.Errorf("%w: expired", ErrInvalidRefresh)
	}
	won, err := s.blacklist.PutIfAbsent(ctx, s.blacklistKey(refreshToken), "1", time.Until(claims.ExpiresAt.Time))
	if err != nil {
		return "", "", err
	}
	if !won {
		return "", "", fmt.Errorf("%w: already rotated or revoked", ErrInvalidRefresh)
	}
	return s.IssuePair(claims.UserID, claims.Account, claims.Role)
}

// RevokeRefresh 吊销 refresh token：写入黑名单，TTL = token 剩余有效期。
// 供登出使用（轮换路径的吊销由 RotateRefresh 的原子抢占承担）。
// 无效或类型不是 refresh 的令牌静默忽略（access 短生命周期，不入黑名单）。
func (s *Session) RevokeRefresh(ctx context.Context, tokenStr string) error {
	if _, err := s.ValidateRefresh(tokenStr); err != nil {
		return nil
	}
	return s.revoke(ctx, tokenStr)
}

// verify 解析并校验 JWT（显式校验签名算法，拒绝非 HMAC 算法，防止 alg=none 攻击）。
// 外部统一走 VerifyAccess / ValidateRefresh 类型分流入口，不直接暴露通用解析。
func (s *Session) verify(tokenStr string) (*Claims, error) {
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

// revoke 将 token 写入黑名单，TTL = token 剩余有效期。
// 无效或已过期的 token 无需吊销，静默返回。
func (s *Session) revoke(ctx context.Context, tokenStr string) error {
	claims, err := s.verify(tokenStr)
	if err != nil || claims.ExpiresAt == nil {
		return nil
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil
	}
	return s.blacklist.Set(ctx, s.blacklistKey(tokenStr), "1", ttl)
}

// ExtractToken 提取登录令牌：优先 Bearer 头，其次父域名 Cookie（子域名共享登录）。
// 纯函数，便于测试；authHeader 为空或非 Bearer 时回退到 cookieValue。
func (s *Session) ExtractToken(authHeader, cookieValue string) string {
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	return cookieValue
}

// CookieName 返回登录态 Cookie 名称（中间件读取 Cookie 用，避免重复持有配置）。
func (s *Session) CookieName() string {
	return s.cookie.Name
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

// randomJWTID 生成随机 jti（防重放/保证每次签发唯一；crypto/rand 失败时退化为时间戳）。
func randomJWTID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}

// blacklistKey 黑名单缓存 key（唯一实现，不再散落字面量）。
func (s *Session) blacklistKey(tokenStr string) string {
	tokenHash := sha256.Sum256([]byte(tokenStr))
	return "jwt:blacklist:" + hex.EncodeToString(tokenHash[:])
}
