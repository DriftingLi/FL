package security

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var errKeyNotFound = errors.New("key not found")

// inmemoryBlacklistStore 内存版黑名单存储（测试用注入双）。
type inmemoryBlacklistStore struct {
	mu sync.Mutex
	m  map[string]string
}

func newInmemoryBlacklistStore() *inmemoryBlacklistStore {
	return &inmemoryBlacklistStore{m: make(map[string]string)}
}

func (s *inmemoryBlacklistStore) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	if !ok {
		return "", errKeyNotFound
	}
	return v, nil
}

func (s *inmemoryBlacklistStore) Set(_ context.Context, key, value string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
	return nil
}

// PutIfAbsent 原子抢占（SETNX 语义）：加锁保证并发双刷恰一成功。
func (s *inmemoryBlacklistStore) PutIfAbsent(_ context.Context, key, value string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key]; ok {
		return false, nil
	}
	s.m[key] = value
	return true, nil
}

const testSecret = "test-jwt-secret"

func newTestSession(blacklist BlacklistStore) *Session {
	if blacklist == nil {
		blacklist = newInmemoryBlacklistStore()
	}
	return NewSessionWithBlacklistAndRefresh(testSecret, time.Hour, time.Hour, CookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false}, blacklist)
}

func TestIssueVerify_RoundTrip(t *testing.T) {
	sess := newTestSession(nil)
	token, err := sess.Issue(42, "hrwai01", "hrwai_user")
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	claims, err := sess.verify(token)
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if claims.UserID != 42 || claims.Account != "hrwai01" || claims.Role != "hrwai_user" {
		t.Errorf("claims 解析异常: %+v", claims)
	}
}

func TestVerify_RejectsWrongSecret(t *testing.T) {
	sess := newTestSession(nil)
	other := NewSession("different-secret", time.Hour, CookieConfig{})
	token, _ := other.Issue(1, "u", "hrwai_user")
	if _, err := sess.verify(token); err == nil {
		t.Error("错误密钥应校验失败")
	}
}

func TestVerify_RejectsAlgNone(t *testing.T) {
	sess := newTestSession(nil)
	// 伪造 alg=none 的 token（防注入攻击校验）
	claims := jwt.MapClaims{"user_id": 1, "username": "u", "role": "hrwai_user"}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	raw, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := sess.verify(raw); err == nil {
		t.Error("alg=none 应被拒绝")
	}
}

func TestVerify_RejectsExpired(t *testing.T) {
	sess := newTestSession(nil)
	claims := &Claims{
		UserID: 1, Account: "u", Role: "hrwai_user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if _, err := sess.verify(token); err == nil {
		t.Error("过期 token 应校验失败")
	}
}

// blacklisted 同包断言辅助：token 是否已命中黑名单（黑名单读取不再暴露为 Session
// 公开面，轮换/吊销语义由 RotateRefresh / RevokeRefresh 表达，测试直接查存储）。
func blacklisted(t *testing.T, sess *Session, token string) bool {
	t.Helper()
	_, err := sess.blacklist.Get(context.Background(), sess.blacklistKey(token))
	return err == nil
}

func TestRevoke_ThenBlacklisted(t *testing.T) {
	sess := newTestSession(newInmemoryBlacklistStore())
	token, _ := sess.Issue(7, "user7", "hrwai_user")

	if blacklisted(t, sess, token) {
		t.Fatal("未吊销的 token 不应命中黑名单")
	}
	if err := sess.revoke(context.Background(), token); err != nil {
		t.Fatalf("吊销失败: %v", err)
	}
	if !blacklisted(t, sess, token) {
		t.Fatal("吊销后的 token 应命中黑名单")
	}
}

func TestRevoke_InvalidTokenIsNoop(t *testing.T) {
	store := newInmemoryBlacklistStore()
	sess := newTestSession(store)
	if err := sess.revoke(context.Background(), "not-a-token"); err != nil {
		t.Fatalf("无效 token 吊销应静默成功: %v", err)
	}
	if len(store.m) != 0 {
		t.Errorf("无效 token 不应写入黑名单，实际 %d 条", len(store.m))
	}
}

func TestRevoke_TwoTokensIndependent(t *testing.T) {
	sess := newTestSession(newInmemoryBlacklistStore())
	tokenA, _ := sess.Issue(1, "a", "hrwai_user")
	tokenB, _ := sess.Issue(2, "b", "hrwai_user")

	_ = sess.revoke(context.Background(), tokenA)
	if !blacklisted(t, sess, tokenA) || blacklisted(t, sess, tokenB) {
		t.Errorf("吊销应互不影响: A=%v B=%v", blacklisted(t, sess, tokenA), blacklisted(t, sess, tokenB))
	}
}

// errBlacklistStore 黑名单存储故障桩（模拟 Redis 不可用）。
type errBlacklistStore struct{}

func (errBlacklistStore) Get(context.Context, string) (string, error) {
	return "", errors.New("redis down")
}

func (errBlacklistStore) Set(context.Context, string, string, time.Duration) error {
	return errors.New("redis down")
}

func (errBlacklistStore) PutIfAbsent(context.Context, string, string, time.Duration) (bool, error) {
	return false, errors.New("redis down")
}

// TestRevoke_StoreErrorPropagatesGracefully 存储故障时行为与既有语义一致：
// revoke/RevokeRefresh 返回错误（登出调用方忽略，不阻塞）。
func TestRevoke_StoreErrorPropagatesGracefully(t *testing.T) {
	sess := newTestSession(errBlacklistStore{})
	token, _ := sess.Issue(7, "user7", "hrwai_user")

	if err := sess.revoke(context.Background(), token); err == nil {
		t.Error("黑名单写入失败时 revoke 应返回错误")
	}
	_, rt, _ := sess.IssuePair(7, "user7", "hrwai_user")
	if err := sess.RevokeRefresh(context.Background(), rt); err == nil {
		t.Error("黑名单写入失败时 RevokeRefresh 应返回错误")
	}
}

// recordingBlacklistStore 记录每次写入的 key 与 TTL。
type recordingBlacklistStore struct {
	mu    sync.Mutex
	items map[string]time.Duration
}

func newRecordingBlacklistStore() *recordingBlacklistStore {
	return &recordingBlacklistStore{items: make(map[string]time.Duration)}
}

func (r *recordingBlacklistStore) Get(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[key]; !ok {
		return "", errKeyNotFound
	}
	return "1", nil
}

func (r *recordingBlacklistStore) Set(_ context.Context, key, _ string, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[key] = ttl
	return nil
}

// PutIfAbsent 原子抢占并记录写入的 TTL（与 Set 同一记录面）。
func (r *recordingBlacklistStore) PutIfAbsent(_ context.Context, key, _ string, ttl time.Duration) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[key]; ok {
		return false, nil
	}
	r.items[key] = ttl
	return true, nil
}

// TestRevoke_BlacklistKeyFormatAndTTL 黑名单 key 格式与 TTL 语义保持不变：
// key = jwt:blacklist:<sha256 hex>，TTL = token 剩余有效期。
func TestRevoke_BlacklistKeyFormatAndTTL(t *testing.T) {
	store := newRecordingBlacklistStore()
	sess := newTestSession(store)
	token, _ := sess.Issue(7, "user7", "hrwai_user")

	if err := sess.revoke(context.Background(), token); err != nil {
		t.Fatalf("吊销失败: %v", err)
	}
	if len(store.items) != 1 {
		t.Fatalf("应写入 1 条黑名单，实际 %d 条", len(store.items))
	}
	var key string
	var ttl time.Duration
	for k, v := range store.items {
		key, ttl = k, v
	}
	if !strings.HasPrefix(key, "jwt:blacklist:") {
		t.Errorf("黑名单 key 应以 jwt:blacklist: 开头: %q", key)
	}
	if len(key) != len("jwt:blacklist:")+sha256.Size*2 {
		t.Errorf("黑名单 key 应为 sha256 十六进制: %q", key)
	}
	if ttl <= 55*time.Minute || ttl > time.Hour {
		t.Errorf("TTL 应约等于 token 剩余有效期 1h, got %v", ttl)
	}
}

func TestExtractToken(t *testing.T) {
	sess := newTestSession(nil)
	cases := []struct {
		name   string
		header string
		cookie string
		want   string
	}{
		{"无 Authorization 头", "", "", ""},
		{"Bearer 头优先", "Bearer abc123", "cookie-token", "abc123"},
		{"非 Bearer 前缀", "Basic abc123", "", ""},
		{"Cookie 兜底", "", "cookie-token", "cookie-token"},
		{"Bearer 头+空 Cookie", "Bearer abc123", "", "abc123"},
	}
	for _, c := range cases {
		if got := sess.ExtractToken(c.header, c.cookie); got != c.want {
			t.Errorf("%s: ExtractToken = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSetCookie_HttpOnlyParentDomain(t *testing.T) {
	sess := newTestSession(nil)
	w := httptest.NewRecorder()
	sess.SetCookie(w, "token-abc")

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("应写 1 个 cookie，实际 %d", len(cookies))
	}
	ck := cookies[0]
	if ck.Name != "hrwai_token" || ck.Value != "token-abc" {
		t.Errorf("cookie 内容异常: %+v", ck)
	}
	if !ck.HttpOnly {
		t.Error("登录态 cookie 必须 HttpOnly")
	}
	if ck.Domain != "example.com" {
		t.Errorf("Domain = %q", ck.Domain)
	}
}

func TestClearCookie_MaxAgeMinusOne(t *testing.T) {
	sess := newTestSession(nil)
	w := httptest.NewRecorder()
	sess.ClearCookie(w)
	ck := w.Result().Cookies()[0]
	if ck.MaxAge != -1 || ck.Value != "" {
		t.Errorf("清除 cookie 应 MaxAge=-1 且值为空: %+v", ck)
	}
}

// TestRotateRefresh_RotatesAndRejectsReplay 轮换语义：签发新对 + 旧 refresh 原子吊销，
// 重放与 access 传入均拒绝（ErrInvalidRefresh 可判定）。
func TestRotateRefresh_RotatesAndRejectsReplay(t *testing.T) {
	store := newInmemoryBlacklistStore()
	sess := newTestSession(store)
	_, rt, _ := sess.IssuePair(7, "user7", "hrwai_user")

	access2, rt2, err := sess.RotateRefresh(context.Background(), rt)
	if err != nil {
		t.Fatalf("首次轮换应成功: %v", err)
	}
	if access2 == "" || rt2 == "" {
		t.Fatal("轮换应返回新双令牌")
	}
	if rt2 == rt {
		t.Error("轮换后 refresh 必须变更（jti 随机）")
	}

	// 旧 refresh 重放：已被抢占即吊销
	if _, _, err := sess.RotateRefresh(context.Background(), rt); !errors.Is(err, ErrInvalidRefresh) {
		t.Errorf("旧 refresh 重放应返回 ErrInvalidRefresh, got %v", err)
	}

	// 新 refresh 可继续轮换（链式续期）
	if _, _, err := sess.RotateRefresh(context.Background(), rt2); err != nil {
		t.Errorf("新 refresh 应可继续轮换: %v", err)
	}

	// access token 传入刷新被类型分流拒绝
	access, _, _ := sess.IssuePair(8, "user8", "hrwai_user")
	if _, _, err := sess.RotateRefresh(context.Background(), access); !errors.Is(err, ErrInvalidRefresh) {
		t.Errorf("access 传入刷新应返回 ErrInvalidRefresh, got %v", err)
	}

	if len(store.m) != 2 {
		t.Errorf("应恰好留下两条黑名单记录（两次成功轮换）, 实际 %d 条", len(store.m))
	}
}

// TestRotateRefresh_ConcurrentExactlyOnce 并发双刷：goroutine 两路同时 RotateRefresh 同一
// refresh_token，恰一成功一失败——PutIfAbsent 原子抢占修复 check-then-act 竞态。
func TestRotateRefresh_ConcurrentExactlyOnce(t *testing.T) {
	sess := newTestSession(newInmemoryBlacklistStore())
	_, rt, _ := sess.IssuePair(9, "user9", "hrwai_user")

	const racers = 16
	var wg sync.WaitGroup
	errCh := make(chan error, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := sess.RotateRefresh(context.Background(), rt)
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)

	success := 0
	for err := range errCh {
		switch {
		case err == nil:
			success++
		case !errors.Is(err, ErrInvalidRefresh):
			t.Fatalf("失败方应统一为 ErrInvalidRefresh, got %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("并发双刷同一 refresh 应恰一成功, 实际 %d", success)
	}
}

// TestRotateRefresh_BlacklistKeyFormatAndTTL 轮换抢占写入的 key/TTL 与登出吊销同口径：
// key = jwt:blacklist:<sha256 hex>，TTL = 旧 refresh 剩余有效期。
func TestRotateRefresh_BlacklistKeyFormatAndTTL(t *testing.T) {
	store := newRecordingBlacklistStore()
	sess := newTestSession(store)
	_, rt, _ := sess.IssuePair(7, "user7", "hrwai_user")

	if _, _, err := sess.RotateRefresh(context.Background(), rt); err != nil {
		t.Fatalf("轮换失败: %v", err)
	}
	if len(store.items) != 1 {
		t.Fatalf("应写入 1 条黑名单，实际 %d 条", len(store.items))
	}
	var key string
	var ttl time.Duration
	for k, v := range store.items {
		key, ttl = k, v
	}
	if !strings.HasPrefix(key, "jwt:blacklist:") {
		t.Errorf("黑名单 key 应以 jwt:blacklist: 开头: %q", key)
	}
	if len(key) != len("jwt:blacklist:")+sha256.Size*2 {
		t.Errorf("黑名单 key 应为 sha256 十六进制: %q", key)
	}
	if ttl <= 55*time.Minute || ttl > time.Hour {
		t.Errorf("TTL 应约等于旧 refresh 剩余有效期 1h, got %v", ttl)
	}
}
