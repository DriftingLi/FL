package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

var errCodeNotFound = errors.New("code not found")

// memCodeStore 内存版验证码存储（测试用）。
type memCodeStore struct {
	mu sync.Mutex
	m  map[string]memCodeEntry
}

type memCodeEntry struct {
	value   string
	expires time.Time
}

func newMemCodeStore() *memCodeStore {
	return &memCodeStore{m: make(map[string]memCodeEntry)}
}

func (s *memCodeStore) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[key]
	if !ok || time.Now().After(e.expires) {
		delete(s.m, key)
		return "", errCodeNotFound
	}
	return e.value, nil
}

func (s *memCodeStore) Set(_ context.Context, key, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = memCodeEntry{value: value, expires: time.Now().Add(ttl)}
	return nil
}

func (s *memCodeStore) Del(_ context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range keys {
		delete(s.m, k)
	}
	return nil
}

// fakeMailer 记录发送内容的测试邮件发送器。
type fakeMailer struct {
	mu   sync.Mutex
	sent []string
}

func (f *fakeMailer) Send(to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, to+"|"+subject+"|"+body)
	return nil
}

func newEmailTestSvc(t *testing.T) (*EmailAuthService, *memCodeStore, *fakeMailer) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	authSvc := NewAuthService(db, "test-secret", time.Hour, "a", "t", "s")
	store := newMemCodeStore()
	mailer := &fakeMailer{}
	return &EmailAuthService{
		db: db, authSvc: authSvc, store: store, sender: mailer, codeTTL: 5 * time.Minute,
	}, store, mailer
}

func extractCode(t *testing.T, store *memCodeStore, purpose EmailCodePurpose, email string) string {
	t.Helper()
	raw, err := store.Get(context.Background(), emailCodeKey(purpose, email))
	if err != nil {
		t.Fatalf("读取验证码失败: %v", err)
	}
	var v authCodeValue
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("解析验证码失败: %v", err)
	}
	return v.Code
}

func TestIsValidEmail(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"user@example.com", true},
		{"USER@Example.COM", true},
		{"a@b.c", true},
		{"", false},
		{"plain", false},
		{"a@b", false},
		{"a b@example.com", false},
		{"user@example", false},
		{"@example.com", false},
		{"user@.com", false},
	}
	for _, c := range cases {
		if got := IsValidEmail(c.in); got != c.want {
			t.Errorf("IsValidEmail(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  User@Example.COM  "); got != "user@example.com" {
		t.Errorf("NormalizeEmail = %q", got)
	}
}

func TestSendRegisterCode(t *testing.T) {
	svc, store, mailer := newEmailTestSvc(t)
	ctx := context.Background()

	// 非法邮箱
	if err := svc.SendRegisterCode(ctx, "bad-email"); err == nil || !strings.Contains(err.Error(), "格式") {
		t.Errorf("非法邮箱应报格式错误: %v", err)
	}

	// 正常发送
	if err := svc.SendRegisterCode(ctx, "new@example.com"); err != nil {
		t.Fatalf("发送注册验证码失败: %v", err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("应发送 1 封邮件，实际 %d", len(mailer.sent))
	}
	if code := extractCode(t, store, EmailCodeRegister, "new@example.com"); len(code) != 6 {
		t.Fatalf("验证码应为 6 位，实际 %q", code)
	}

	// 同一邮箱 60 秒内重复发送被限流
	if err := svc.SendRegisterCode(ctx, "new@example.com"); err == nil || !strings.Contains(err.Error(), "频繁") {
		t.Errorf("重复发送应被限流: %v", err)
	}
	// 已注册邮箱不能发送注册验证码
	_ = svc.db.Create(&model.HrwaiUser{
		Username: "exists", Password: "x", Name: "已注册",
		Phone: "test_mail_exists", Email: "taken@example.com", Status: 1, CreatedAt: time.Now(),
	})
	if err := svc.SendRegisterCode(ctx, "taken@example.com"); err == nil || !strings.Contains(err.Error(), "已注册") {
		t.Errorf("已注册邮箱应报已注册: %v", err)
	}
}

func TestSendLoginCode(t *testing.T) {
	svc, _, mailer := newEmailTestSvc(t)
	db := svc.db
	ctx := context.Background()
	_ = db.Create(&model.HrwaiUser{
		Username: "mail_login_user", Password: "x", Name: "登录用户",
		Phone: "test_mail_login", Email: "login@example.com", Status: 1, CreatedAt: time.Now(),
	})

	// 未注册邮箱
	if err := svc.SendLoginCode(ctx, "nobody@example.com"); err == nil || !strings.Contains(err.Error(), "尚未注册") {
		t.Errorf("未注册邮箱应报尚未注册: %v", err)
	}
	// 正常发送
	if err := svc.SendLoginCode(ctx, "LOGIN@example.com"); err != nil {
		t.Fatalf("发送登录验证码失败: %v", err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("应发送 1 封邮件，实际 %d", len(mailer.sent))
	}
	// 60 秒内重复发送被限流
	if err := svc.SendLoginCode(ctx, "login@example.com"); err == nil || !strings.Contains(err.Error(), "频繁") {
		t.Errorf("重复发送应被限流: %v", err)
	}
}

func TestRegisterAndLoginWithCode(t *testing.T) {
	svc, store, mailer := newEmailTestSvc(t)
	ctx := context.Background()
	email := "newuser@example.com"

	// 未获取验证码直接注册
	if _, err := svc.RegisterWithCode(ctx, email, "123456", "张三", "", "pass123"); err == nil {
		t.Error("未获取验证码直接注册应失败")
	}

	// 获取注册验证码
	if err := svc.SendRegisterCode(ctx, email); err != nil {
		t.Fatalf("发送注册验证码失败: %v", err)
	}
	code := extractCode(t, store, EmailCodeRegister, email)

	// 错误验证码
	if _, err := svc.RegisterWithCode(ctx, email, "000000", "张三", "", "pass123"); err == nil || !strings.Contains(err.Error(), "验证码") {
		t.Errorf("错误验证码应失败: %v", err)
	}
	// 正确验证码注册（自动登录）
	regRes, err := svc.RegisterWithCode(ctx, email, code, "张三", "测试公司", "pass123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if regRes.Token == "" || regRes.Role != HrwaiRole || regRes.Username == "" || regRes.Username == email {
		t.Errorf("注册结果异常: %+v", regRes)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("注册应发送 1 封邮件，实际 %d", len(mailer.sent))
	}

	// 同一邮箱不能重复注册（手动写入验证码绕过发送，验证注册接口的唯一性兜底）
	dupCodeVal, _ := json.Marshal(authCodeValue{Code: "123456"})
	_ = store.Set(ctx, emailCodeKey(EmailCodeRegister, email), string(dupCodeVal), time.Minute)
	if _, err := svc.RegisterWithCode(ctx, email, "123456", "李四", "", "pass123"); err == nil || !strings.Contains(err.Error(), "已注册") {
		t.Errorf("重复注册应失败: %v", err)
	}
	// 已注册邮箱发送注册验证码被拒绝
	if err := svc.SendRegisterCode(ctx, email); err == nil || !strings.Contains(err.Error(), "已注册") {
		t.Errorf("重复注册发送验证码应失败: %v", err)
	}

	// 邮箱登录
	if err := svc.SendLoginCode(ctx, email); err != nil {
		t.Fatalf("发送登录验证码失败: %v", err)
	}
	loginCode := extractCode(t, store, EmailCodeLogin, email)
	loginRes, err := svc.LoginWithCode(ctx, email, loginCode)
	if err != nil {
		t.Fatalf("邮箱登录失败: %v", err)
	}
	if loginRes.UserID != regRes.UserID || loginRes.Token == "" {
		t.Errorf("登录结果异常: %+v", loginRes)
	}

	// 禁用账号不能登录
	_ = svc.db.Model(&model.HrwaiUser{}).Where("email = ?", email).Update("status", 0).Error
	_ = store.Del(ctx, emailSendThrottleKey(EmailCodeLogin, email))
	if err := svc.SendLoginCode(ctx, email); err != nil {
		t.Fatalf("发送登录验证码失败: %v", err)
	}
	loginCode2 := extractCode(t, store, EmailCodeLogin, email)
	if _, err := svc.LoginWithCode(ctx, email, loginCode2); err == nil || !strings.Contains(err.Error(), "禁用") {
		t.Errorf("禁用账号应无法登录: %v", err)
	}
}

func TestVerifyCodeAttemptLimit(t *testing.T) {
	svc, store, _ := newEmailTestSvc(t)
	ctx := context.Background()
	email := "attempt@example.com"
	if err := svc.SendRegisterCode(ctx, email); err != nil {
		t.Fatalf("发送验证码失败: %v", err)
	}
	code := extractCode(t, store, EmailCodeRegister, email)

	for i := 0; i < 5; i++ {
		if _, err := svc.RegisterWithCode(ctx, email, "000000", "张三", "", "pass123"); err == nil || !strings.Contains(err.Error(), "验证码") {
			t.Fatalf("第 %d 次错误验证码应失败: %v", i+1, err)
		}
	}
	// 错误 5 次后，即使输入正确验证码也要求重新获取
	if _, err := svc.RegisterWithCode(ctx, email, code, "张三", "", "pass123"); err == nil || !strings.Contains(err.Error(), "次数过多") {
		t.Errorf("超过错误次数应要求重新获取: %v", err)
	}
}

func TestRegisterWithCode_BadEmail(t *testing.T) {
	svc, _, _ := newEmailTestSvc(t)
	if _, err := svc.RegisterWithCode(context.Background(), "not-an-email", "123456", "张三", "", "pass123"); err == nil || !strings.Contains(err.Error(), "格式") {
		t.Errorf("非法邮箱应报格式错误: %v", err)
	}
}

func TestRegisterWithCode_BadPassword(t *testing.T) {
	svc, store, _ := newEmailTestSvc(t)
	ctx := context.Background()
	email := "pwd@example.com"
	if err := svc.SendRegisterCode(ctx, email); err != nil {
		t.Fatalf("发送验证码失败: %v", err)
	}
	code := extractCode(t, store, EmailCodeRegister, email)
	if _, err := svc.RegisterWithCode(ctx, email, code, "张三", "", "123"); err == nil || !strings.Contains(err.Error(), "密码长度") {
		t.Errorf("过短密码应报错: %v", err)
	}
}

func TestBindEmail(t *testing.T) {
	svc, store, _ := newEmailTestSvc(t)
	db := svc.db
	ctx := context.Background()
	u1 := &model.HrwaiUser{Username: "ebind1", Password: "x", Name: "用户一", Phone: "test_ebind_1", Email: "old1@example.com", Status: 1, CreatedAt: time.Now()}
	u2 := &model.HrwaiUser{Username: "ebind2", Password: "x", Name: "用户二", Phone: "test_ebind_2", Email: "taken2@example.com", Status: 1, CreatedAt: time.Now()}
	_ = db.Create(u1)
	_ = db.Create(u2)

	// 目标邮箱已被他人使用
	if err := svc.SendBindCode(ctx, u1.ID, "taken2@example.com"); err == nil || !strings.Contains(err.Error(), "已被其他账号") {
		t.Errorf("占用邮箱应报错: %v", err)
	}
	// 非法格式
	if err := svc.SendBindCode(ctx, u1.ID, "bad-email"); err == nil || !strings.Contains(err.Error(), "格式") {
		t.Errorf("非法邮箱应报格式错误: %v", err)
	}
	// 正常绑定
	if err := svc.SendBindCode(ctx, u1.ID, "new@example.com"); err != nil {
		t.Fatalf("发送绑定验证码失败: %v", err)
	}
	code := extractCode(t, store, EmailCodeBind, "new@example.com")
	if err := svc.BindEmail(ctx, u1.ID, "new@example.com", code); err != nil {
		t.Fatalf("绑定邮箱失败: %v", err)
	}
	var after model.HrwaiUser
	_ = db.First(&after, u1.ID).Error
	if after.Email != "new@example.com" {
		t.Errorf("绑定后邮箱 = %q", after.Email)
	}
}

func TestHrwaiRegister_EmailUniqueAndFormat(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	authSvc := NewAuthService(db, "s", time.Hour, "a", "t", "s")

	if _, err := authSvc.HrwaiRegister("13800001111", "pass123", "张三", "DUP@Example.com", ""); err != nil {
		t.Fatalf("首次注册失败: %v", err)
	}
	// 同一邮箱（忽略大小写）不能重复注册
	if _, err := authSvc.HrwaiRegister("13800002222", "pass123", "李四", "dup@example.com", ""); err == nil || !strings.Contains(err.Error(), "已被注册") {
		t.Errorf("重复邮箱应报已被注册: %v", err)
	}
	// 非法邮箱格式
	if _, err := authSvc.HrwaiRegister("13800003333", "pass123", "王五", "bad-email", ""); err == nil || !strings.Contains(err.Error(), "格式") {
		t.Errorf("非法邮箱应报格式错误: %v", err)
	}
}
