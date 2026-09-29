// Package security 提供认证与安全工具。
// 本文件：会话（session）模块——JWT 签发/校验/吊销与登录态 Cookie 的唯一实现。
// 中间件、AuthService 与各登录路径都消费本模块的 interface，黑名单 key 与
// Bearer/Cookie 解析逻辑不再散落各处。
//
// 双令牌会话（ADR-0012）：access（2h，鉴权中间件专用，不入黑名单）+
// refresh（7 天，刷新端点专用，轮换时旧值立即入黑名单防重放）；登出吊销 refresh。
//
// ADR-0067（修订 ADR-0016）：refresh 在浏览器侧改由本模块下发的 httpOnly Cookie 承载
// （Path 收在认证族前缀 /api/auth，见 RefreshCookiePath），请求体通道保留给移动端与非浏览器客户端；
// 签发/回写/吊销三条动作仍收敛在本模块单点。
//
// 两族 Cookie（主站 / 招聘者）可以在同一 host 上并存，所以「读哪一族」必须由 access 决定
// ——口径与判据见本文件「族判定」段与移动端 ADR-0030 ② 第 2 条。
package security

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

// refresh 令牌的浏览器通道（ADR-0067 决策 1、2）：httpOnly Cookie 优先、请求体通道保留。
//
// RefreshCookiePath = `/api/auth`：这是对 ADR-0067 决策 2「Cookie 只用于该一个端点」的**修订后落点**，
// 因为原设计挡掉的那件事恰恰是产品语义本身 ——
// Path 收在 `/api/auth/refresh` 时，浏览器不会把这枚 Cookie 发到 `/api/auth/logout`，
// 于是「登出 = 单会话终止（手上这一枚 refresh 失效）」（`CONTEXT.md`「会话」词条，自 ADR-0016 就在）
// 在浏览器侧**静默退化**成「只清本地、长效凭证继续可用」。放宽一档到认证族前缀 ⇒
// 两个消费点（refresh / logout）都被覆盖，登出重新拿得到吊销所需的那枚凭证。
// 它仍然不是 `/`：`/` 等于把 7 天凭证挂到全站每一个请求上；现在只有 `/api/auth/**` 这一族收到，
// 其中不消费它的端点（登录/注册/验证码）本就要覆写或清除这枚 Cookie，不存在新的读取方。
// CSRF 面不因此放宽：`SameSite=Lax` 下跨站 POST 不带 Cookie，刷新与登出都只接受同站 POST。
// 名字族与 access cookie 同名族但不同名，Domain / Secure **一律继承**各自的 access cookie 配置
// （见 refreshCookiesFor）：本仓是子域名多工作区，作用域只能有一处事实源，另开一个配置项
// 就是留一处「两个 cookie 域口径不一致」的漂移点，故这里不新增任何环境变量。
const (
	RefreshCookiePath                 = "/api/auth"
	DefaultRefreshCookieName          = "hrwai_refresh"
	DefaultRecruiterRefreshCookieName = "recruiter_refresh"

	// roleRecruiter 与 service.RecruiterRole 同值：分流 refresh cookie 归属用。
	// 不直接引用 service 是因为依赖方向是 service → security，反向引用成环。
	roleRecruiter = "recruiter"
)

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
// 招牌隔离（#370）：学员侧 hrwai_token 父域共享，招聘者 recruiter_token host-only。
type Session struct {
	jwtSecret       string
	jwtExpiry       time.Duration
	refreshExpiry   time.Duration
	cookie          CookieConfig // hrwai / admin / tutor 共享的父域 cookie
	recruiterCookie CookieConfig // recruiter 独立 host-only cookie
	// refreshCookie / recruiterRefreshCookie（ADR-0067）：浏览器侧 refresh 的落点。
	// 两者都由对应的 access cookie 经 refreshCookiesFor 推导，Path 一律收在 RefreshCookiePath。
	refreshCookie          CookieConfig
	recruiterRefreshCookie CookieConfig
	blacklist              BlacklistStore
}

// refreshCookiesFor 由两族 access cookie 推导出各自的 refresh cookie 配置（ADR-0067 决策 2 的
// 「同一套域名与安全口径」）：Domain/Secure 逐字继承，只换名字与 Path。
// 招聘者侧保持 host-only——轮换时把招聘者那支写进父域等于凭空扩大凭证作用域。
func refreshCookiesFor(cookie, recruiterCookie CookieConfig) (CookieConfig, CookieConfig) {
	return CookieConfig{Name: DefaultRefreshCookieName, Domain: cookie.Domain, Secure: cookie.Secure},
		CookieConfig{Name: DefaultRecruiterRefreshCookieName, Domain: recruiterCookie.Domain, Secure: recruiterCookie.Secure}
}

// NewSession 构造会话模块（默认 Redis 黑名单存储；refresh 默认 7 天）。
func NewSession(jwtSecret string, jwtExpiry time.Duration, cookie CookieConfig) *Session {
	return NewSessionWithBlacklistAndRefresh(jwtSecret, jwtExpiry, defaultRefreshExpiry, cookie, RedisBlacklistStore{})
}

// NewSessionWithBlacklistAndRefresh 构造会话模块：黑名单存储与 refresh 有效期均可注入（测试用）。
func NewSessionWithBlacklistAndRefresh(jwtSecret string, jwtExpiry, refreshExpiry time.Duration, cookie CookieConfig, blacklist BlacklistStore) *Session {
	return NewSessionWithRecruiterCookie(jwtSecret, jwtExpiry, refreshExpiry, cookie,
		CookieConfig{Name: "recruiter_token", Secure: cookie.Secure}, blacklist)
}

// NewSessionWithRecruiterCookie 构造会话模块：显式指定招聘者 cookie（测试可注入 host-only 配置）。
func NewSessionWithRecruiterCookie(jwtSecret string, jwtExpiry, refreshExpiry time.Duration, cookie, recruiterCookie CookieConfig, blacklist BlacklistStore) *Session {
	if recruiterCookie.Name == "" {
		recruiterCookie.Name = "recruiter_token"
	}
	refresh, recruiterRefresh := refreshCookiesFor(cookie, recruiterCookie)
	return &Session{
		jwtSecret:              jwtSecret,
		jwtExpiry:              jwtExpiry,
		refreshExpiry:          refreshExpiry,
		cookie:                 cookie,
		recruiterCookie:        recruiterCookie,
		refreshCookie:          refresh,
		recruiterRefreshCookie: recruiterRefresh,
		blacklist:              blacklist,
	}
}

// SessionFromConfig 从应用配置构造会话模块（黑名单固定为 Redis 存储，refresh 用配置值）。
func SessionFromConfig(cfg *config.Config) *Session {
	return SessionFromConfigWithBlacklist(cfg, RedisBlacklistStore{})
}

// SessionFromConfigWithBlacklist 同 SessionFromConfig，但黑名单存储可注入
// （契约测试链路没有 Redis，而注销/登出/轮换都要真实写黑名单）。
func SessionFromConfigWithBlacklist(cfg *config.Config, blacklist BlacklistStore) *Session {
	hrwaiCookie := CookieConfig{
		Name:   cfg.AuthCookie.Name,
		Domain: cfg.AuthCookie.Domain,
		Secure: cfg.AuthCookie.Secure,
	}
	recruiterCookie := CookieConfig{
		Name:   cfg.RecruiterCookie.Name,
		Domain: cfg.RecruiterCookie.Domain,
		Secure: cfg.RecruiterCookie.Secure,
	}
	if recruiterCookie.Name == "" {
		recruiterCookie.Name = "recruiter_token"
	}
	return NewSessionWithRecruiterCookie(cfg.JWTSecretKey, cfg.JWTExpiry(), cfg.JWTRefreshExpiry(), hrwaiCookie, recruiterCookie, blacklist)
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

// refreshRevocationKey 用户级 refresh 吊销标记键（#622，移动端 ADR-0006 修复方向 2）。
// 键带角色命名空间：hrwai_users 与 recruiter_users 两套 ID 空间共用本 Session 单例，
// 裸 userID 会撞键。
func (s *Session) refreshRevocationKey(role string, userID int) string {
	return fmt.Sprintf("jwt:pwd_revoked:%s:%d", role, userID)
}

// RevokeIdentity 全会话吊销（会话终止两族之二，ADR-0060 票2）：写入用户级吊销标记（时间戳），
// 该身份名下所有 refresh 链一次性失效。改密、禁用招聘者、注销三处共用本动作。
// TTL = refresh 有效期——标记之前签发的任何 refresh（含轮换滑动续期）最长存活不超过它，
// 标记过期即自然失效，无需清理任务。
//
// 失败策略由调用方决定，本动作只如实返回：改密/禁用是「已生效动作之后的补救」，调用方记日志
// 不阻断；注销没有已生效动作，调用方据此让整体不生效（见 AuthService.DeleteAccount）。
func (s *Session) RevokeIdentity(ctx context.Context, role string, userID int) error {
	return s.blacklist.Set(ctx, s.refreshRevocationKey(role, userID),
		strconv.FormatInt(time.Now().Unix(), 10), s.refreshExpiry)
}

// refreshRevoked 检查 refresh 是否不晚于改密吊销标记签发（iat ≤ 标记即拒绝）。
// 同秒语义：JWT iat 为秒精度（jwt.TimePrecision=Second），标记同一秒内签发的 token
// 一并拒绝——宁错杀（改密前 token 同秒逃逸后会轮换洗白整条链），合法重登被拒属
// 自我修复（下次刷新被拒即强制重登，新秒的 token 正常）。
// 读故障放行（fail-open）：与登录链路「读黑名单失败放行」的取舍一致（ADR-0016），
// 后续 RotateRefresh 的 PutIfAbsent 抢占仍 fail-closed 兜底，Redis 整体故障时不会真签发。
func (s *Session) refreshRevoked(ctx context.Context, role string, userID int, issuedAt time.Time) bool {
	v, err := s.blacklist.Get(ctx, s.refreshRevocationKey(role, userID))
	if err != nil {
		return false
	}
	ts, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return false
	}
	return !issuedAt.After(time.Unix(ts, 0))
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
	// 改密吊销标记检查（#622）：iat 早于标记的 refresh 一律拒绝——改密后旧会话链
	// （含快捷登录的静默续登）立即失效；并入 ErrInvalidRefresh 保持 401 防枚举口径。
	if claims.IssuedAt != nil && s.refreshRevoked(ctx, claims.Role, claims.UserID, claims.IssuedAt.Time) {
		return "", "", fmt.Errorf("%w: password changed", ErrInvalidRefresh)
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

// RotateAndSetCookie 轮换并把新 refresh 回写成 httpOnly Cookie（ADR-0067 决策 1、2 的落点）。
//
// 语义与 RotateRefresh 完全一致（轮换/吊销不因此改变，票 #1363 判据 2），只是多一步下发：
// 浏览器侧下一轮只认这枚 Cookie；响应体里 refresh_token 照旧返回，请求体通道客户端
// （移动端 / 非浏览器客户端）继续按 ADR-0016 的形态持有。
// Cookie 归属按新令牌的 claims.Role 分流，招聘者那支不会写进父域（作用域只收窄不外扩）。
func (s *Session) RotateAndSetCookie(ctx context.Context, w http.ResponseWriter, refreshToken string) (string, string, error) {
	access, refresh, err := s.RotateRefresh(ctx, refreshToken)
	if err != nil {
		return "", "", err
	}
	// 刚签出的这枚必然自校验通过；真失败时只回退到「响应体通道」（不阻断续期），
	// 不把它升级成 500——此刻旧 refresh 已被抢占吊销，报错误只会把用户挡在门外。
	if claims, verr := s.ValidateRefresh(refresh); verr == nil {
		s.setRefreshCookieForRole(w, claims.Role, refresh)
	}
	return access, refresh, nil
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

// SignOut 单会话终止（会话终止两族之一，ADR-0060 票2）：撤销手上这枚 refresh
// （为空或无效即静默跳过）并清除主站登录态 Cookie。
//
// token 取自请求体还是 Bearer 头是**入口差异**，不是第三种语义——两条入口都收敛到本动作。
// 没有角色形参：本仓只有主站这一条登出端点（招聘者面没有），留着那个分支就是一个实现
// 撑起的假想 seam（ADR-0060 自己的判据）。
// 吊销失败仍清 Cookie：本地登录态已不可用，凭证缺口由日志暴露（与既有登出口径一致）。
// 终止该身份全部会话不在此处：那属 RevokeIdentity。
//
// ADR-0067 之后 refresh 的本地清除也收敛到本动作（ClearRefreshCookies）：登出至少要把凭证
// 从浏览器里抹掉。服务端吊销同样收敛到本动作——refresh cookie 的 Path = /api/auth（决策 2 的
// 最小暴露面在此让一步的理由见 RefreshCookiePath 注释），浏览器登出取得到手上那一支，
// `CONTEXT.md`「会话」的单会话终止在浏览器侧照旧成立（#1385 缺口 1 的处置）。
// 取不到的那一种要说明白：本次请求没有族线索（既没带 Bearer 头也没带 access cookie）时
// RefreshCookieForRequest 返回空串 ⇒ 登出退化为「只清本地」。这是刻意的：此时任选一族去吊销
// 就是拿名序决定「谁的会话被终止」，而两族并存时那个答案一定是错的其中一边。
// 终止该身份全部会话不在此处：那属 RevokeIdentity。
func (s *Session) SignOut(ctx context.Context, w http.ResponseWriter, refreshToken string) error {
	var err error
	if refreshToken != "" {
		err = s.RevokeRefresh(ctx, refreshToken)
	}
	s.ClearCookie(w)
	s.ClearRefreshCookies(w)
	return err
}

// ClearLoginCookies 抹掉本地全部登录态 Cookie（access + 两族 refresh）：登出与注销共用一处。
func (s *Session) ClearLoginCookies(w http.ResponseWriter) {
	s.ClearCookie(w)
	s.ClearRefreshCookies(w)
}

// SetLoginCookies 登录路径一次性下发 access + refresh（ADR-0067：refresh 从此有 Cookie 通道，
// 响应体里的双令牌照旧返回，非浏览器客户端继续按 ADR-0016 的形态持有）。
func (s *Session) SetLoginCookies(w http.ResponseWriter, access, refresh string) {
	s.SetCookie(w, access)
	s.SetRefreshCookie(w, refresh)
}

// SetRecruiterLoginCookies 招聘者登录：两枚 Cookie 都保持 host-only（招牌隔离 #370，作用域不外扩）。
func (s *Session) SetRecruiterLoginCookies(w http.ResponseWriter, access, refresh string) {
	s.SetRecruiterCookie(w, access)
	s.SetRecruiterRefreshCookie(w, refresh)
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

// AccessCookieValue 读取本次请求携带的 access cookie 值（按 CookieNames() 的优先级取第一枚非空）。
// 鉴权中间件与 refresh 的族判定**共用本函数**：两处对「当前活跃身份」的回答必须是同一个，
// 否则续期会轮换另一族凭证（见 RefreshCookieForRequest）。
func (s *Session) AccessCookieValue(r *http.Request) string {
	for _, name := range s.CookieNames() {
		if ck, err := r.Cookie(name); err == nil && ck.Value != "" {
			return ck.Value
		}
	}
	return ""
}

// CookieName 返回登录态 Cookie 名称（中间件读取 Cookie 用，避免重复持有配置）。
func (s *Session) CookieName() string {
	return s.cookie.Name
}

// writeLoginCookie 登录态 Cookie 的唯一写形：httpOnly + SameSite=Lax + 由配置决定的 Domain/Secure
// 三处口径集中一处（ADR-0016 的 access 与 ADR-0067 的 refresh 共用，差别只有 Name/Path/MaxAge）。
// maxAge 传秒：-1 表示清除。
func writeLoginCookie(w http.ResponseWriter, cfg CookieConfig, value, path string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.Name,
		Value:    value,
		Path:     path,
		Domain:   cfg.Domain,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// SetCookie 将 JWT 写入父域名 httpOnly Cookie，实现子域名间登录态共享。
func (s *Session) SetCookie(w http.ResponseWriter, token string) {
	writeLoginCookie(w, s.cookie, token, "/", int(s.jwtExpiry.Seconds()))
}

// ClearCookie 清除登录 Cookie（登出时调用）。
func (s *Session) ClearCookie(w http.ResponseWriter) {
	writeLoginCookie(w, s.cookie, "", "/", -1)
}

// RecruiterCookieName 返回招聘者登录态 Cookie 名称（host-only 隔离）。
func (s *Session) RecruiterCookieName() string {
	if s.recruiterCookie.Name != "" {
		return s.recruiterCookie.Name
	}
	return "recruiter_token"
}

// CookieNames 返回所有登录态 Cookie 名称（按优先级：hrwai 优先，recruiter 次之），
// 中间件读取时依次尝试以兼容双 cookie 场景。
func (s *Session) CookieNames() []string {
	names := []string{s.cookie.Name}
	if rc := s.RecruiterCookieName(); rc != "" && rc != s.cookie.Name {
		names = append(names, rc)
	}
	return names
}

// SetRecruiterCookie 将招聘者 JWT 写入 host-only httpOnly Cookie（不设 Domain，浏览器仅对当前 host 发送）。
func (s *Session) SetRecruiterCookie(w http.ResponseWriter, token string) {
	cfg := s.recruiterCookie
	cfg.Name = s.RecruiterCookieName()
	writeLoginCookie(w, cfg, token, "/", int(s.jwtExpiry.Seconds()))
}

// ===== ADR-0067：refresh 令牌的浏览器通道（httpOnly Cookie 优先，请求体通道保留）=====

// RefreshCookieNames 返回两族 refresh cookie 名（主站先、招聘者次之）。
// ⚠️ 只用于**枚举**（登出/注销要把两族都清掉）与测试，不参与选族：两族可以在同一个 host 上
// 并存，「按名序取第一枚非空」正是把招聘者面的续期轮换到学员那一族上的那条路（见本段末
// RefreshCookieForRequest 的口径）。
func (s *Session) RefreshCookieNames() []string {
	names := make([]string, 0, 2)
	if s.refreshCookie.Name != "" {
		names = append(names, s.refreshCookie.Name)
	}
	if rn := s.RecruiterRefreshCookieName(); rn != "" && rn != s.refreshCookie.Name {
		names = append(names, rn)
	}
	return names
}

// RecruiterRefreshCookieName 返回招聘者 refresh cookie 名（host-only）。
func (s *Session) RecruiterRefreshCookieName() string {
	if s.recruiterRefreshCookie.Name != "" {
		return s.recruiterRefreshCookie.Name
	}
	return DefaultRecruiterRefreshCookieName
}

// RefreshCookieName 返回主站 refresh cookie 名。
func (s *Session) RefreshCookieName() string {
	if s.refreshCookie.Name != "" {
		return s.refreshCookie.Name
	}
	return DefaultRefreshCookieName
}

// ===== 族判定（#1376 跨端评审 · 移动端 ADR-0030 ② 第 2 条、④ 第 1 项）=====
//
// 两族 refresh cookie **可以并存**：ADR-0022 把招聘者 access 收紧为 host-only 之后，父域那枚
// hrwai_refresh 在招聘者子域上依然会被投递 —— 这是**浏览器**侧的形状，也是本段口径的射程。
// （#1389 真机读数：uni-app x 的 App 容器**不维持 cookie jar**，App 与小程序都走请求体通道；
// 「App/H5 自动带 cookie」那句出自 uni-app（vue 版）参数表，uni-app x 的表里没有它。⇒ 串族在
// App 上当前不可达，H5 面未测。⇒ 族判定不是为 App 落的，是为 Web 这条并存路径落的。）
// 于是「按名序取第一枚非空」会让招聘者面的续期**轮换学员那一族**，而客户端把续出的令牌连同
// 当前内存角色一起落盘 ⇒ 登录态被静默换成另一个身份、UI 还停在原身份。
//
// 口径：**族由 access 定**。本次续期归属哪一族，判据与鉴权中间件对「你是谁」的回答**同源**
// （ExtractToken 的头优先 + AccessCookieValue 的遍历顺序），这里只取其中的 role 一个字段。
// 两条线索都没有 ⇒ Cookie 通道视为**不可用**（不是「任选一族」）：端点据此回退请求体，
// 而请求体那支的归属由令牌自身的 claims.Role 决定、不经 Cookie 选族，不存在串族面。

// accessRoleOf 只从 JWT 载荷里读出 role，**不做任何认证判定**：不验签、不看 exp/iat、
// 不校验 token_type、不查黑名单。它回答的是「客户端自认为属于哪一族」，答案只用来选 Cookie；
// 认证面仍只能走 VerifyAccess / ValidateRefresh。
// 用例 TestRefreshFamily_Role解析不是认证面 把这句话钉成代码（错签名的令牌在这里能读出 role、
// 在 VerifyAccess 那里必须被拒）。
func accessRoleOf(tokenStr string) string {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.Role
}

// RefreshFamilyRole 返回本次请求所声明的令牌族（与同一请求里鉴权中间件会认的那个身份同源）。
// 空串 = 没有任何族线索（既没带 Bearer 头，也没带 access cookie）。
func (s *Session) RefreshFamilyRole(r *http.Request) string {
	return accessRoleOf(s.ExtractToken(r.Header.Get("Authorization"), s.AccessCookieValue(r)))
}

// RefreshCookieForRequest 取出本次请求 Cookie 通道里**属于该族**的 refresh 凭证。
// 返回空串 = Cookie 通道不可用（没有族线索，或该族没带 Cookie）⇒ 端点回退请求体。
// 分流口径与回写侧的 setRefreshCookieForRole 一致：recruiter 落 host-only 那一族，其余落主站那一族。
func (s *Session) RefreshCookieForRequest(r *http.Request) string {
	role := s.RefreshFamilyRole(r)
	if role == "" {
		return ""
	}
	name := s.RefreshCookieName()
	if role == roleRecruiter {
		name = s.RecruiterRefreshCookieName()
	}
	ck, err := r.Cookie(name)
	if err != nil || ck.Value == "" {
		return ""
	}
	return ck.Value
}

// SetRefreshCookie 下发主站 refresh 的 httpOnly Cookie（Path = RefreshCookiePath，认证族前缀）。
// ⚠️ 取配置必须走**局部拷贝**（与同文件 SetRecruiterCookie 同形）：`*Session` 是跨请求共享的，
// 直接写 `s.refreshCookie.Name` 就是并发请求在同一个字段上 race —— 本仓刚为同一个形状修过
// 限流器（`middleware/ratelimit.go` 的「普通字段与 cleanup  goroutine 形成数据竞争」那条），
// 这里不许复发。名字缺省值由 RefreshCookieName() 读时兜，不需要写回去缓存。
func (s *Session) SetRefreshCookie(w http.ResponseWriter, token string) {
	cfg := s.refreshCookie
	cfg.Name = s.RefreshCookieName()
	writeLoginCookie(w, cfg, token, RefreshCookiePath, int(s.refreshExpiry.Seconds()))
}

// SetRecruiterRefreshCookie 下发招聘者 refresh 的 httpOnly Cookie（host-only，作用域不外扩）。
// 取配置的形态与 SetRefreshCookie 一致：局部拷贝，不写共享 Session 字段。
func (s *Session) SetRecruiterRefreshCookie(w http.ResponseWriter, token string) {
	cfg := s.recruiterRefreshCookie
	cfg.Name = s.RecruiterRefreshCookieName()
	writeLoginCookie(w, cfg, token, RefreshCookiePath, int(s.refreshExpiry.Seconds()))
}

// ClearRefreshCookies 清掉两族 refresh cookie（登出/注销的本地凭证清除）。
// 注意两处细节：清除必须与写入同 Path/Domain（否则浏览器不认这条 Set-Cookie 是删除），
// 且 Max-Age 必须是 -1（只把值置空、Max-Age 仍是 7 天等于又发了一枚空值 Cookie）。
func (s *Session) ClearRefreshCookies(w http.ResponseWriter) {
	writeLoginCookie(w, CookieConfig{
		Name:   s.RefreshCookieName(),
		Domain: s.refreshCookie.Domain,
		Secure: s.refreshCookie.Secure,
	}, "", RefreshCookiePath, -1)
	writeLoginCookie(w, CookieConfig{
		Name:   s.RecruiterRefreshCookieName(),
		Domain: s.recruiterRefreshCookie.Domain,
		Secure: s.recruiterRefreshCookie.Secure,
	}, "", RefreshCookiePath, -1)
}

// setRefreshCookieForRole 按令牌归属角色回写 refresh cookie（招聘者只落 host-only 那族）。
func (s *Session) setRefreshCookieForRole(w http.ResponseWriter, role, token string) {
	if role == roleRecruiter {
		s.SetRecruiterRefreshCookie(w, token)
		return
	}
	s.SetRefreshCookie(w, token)
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
