package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"forklift-training/internal/cache"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
	"go.uber.org/zap"
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

// fakeSMSProvider 记录发送内容的测试短信发送器。
type fakeSMSProvider struct {
	sent []string
}

func (f *fakeSMSProvider) Send(to, code string, _ int, _ CodePurpose) error {
	f.sent = append(f.sent, to+"|"+code)
	return nil
}

// newCodeTestSvc 构造测试用验证码 engine：内存 store + 可注入发送器的通道。
func newCodeTestSvc(t *testing.T) (*VerifyCodeService, *memCodeStore) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	authSvc := NewAuthService(db, security.NewSession("test-secret", time.Hour, security.CookieConfig{}), NewForumCounter(), "a", "t", "s", zap.NewNop())
	store := newMemCodeStore()
	svc := NewVerifyCodeService(db, authSvc, 5*time.Minute, store, zap.NewNop())
	return svc, store
}

func testEmailChannel(mailer MailSender) *EmailChannel {
	return &EmailChannel{mailer: mailer}
}

func testSmsChannel(sms SMSProvider) *SmsChannel {
	return &SmsChannel{sms: sms}
}

// codeKeyForTest 与生产 codeKey 等价（同前缀，验证线上 key 兼容性）。
func codeKeyForTest(ch CodeChannel, purpose CodePurpose, target string) string {
	return cache.SafeKey(ch.KeyPrefix(), string(purpose), target)
}

func extractCode(t *testing.T, store *memCodeStore, ch CodeChannel, purpose CodePurpose, target string) string {
	t.Helper()
	raw, err := store.Get(context.Background(), codeKeyForTest(ch, purpose, target))
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

func TestIsValidPhone(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"13800138000", true},
		{" 13800138000 ", true},
		{"12345", false},
		{"23800138000", false},
		{"1380013800", false},
		{"138001380000", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsValidPhone(c.in); got != c.want {
			t.Errorf("IsValidPhone(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSendRegisterCode(t *testing.T) {
	svc, store := newCodeTestSvc(t)
	ch := testEmailChannel(&fakeMailer{})
	ctx := context.Background()

	// 非法邮箱
	if err := svc.Send(ctx, ch, CodePurposeRegister, "bad-email"); err == nil || !strings.Contains(err.Error(), "格式") {
		t.Errorf("非法邮箱应报格式错误: %v", err)
	}

	// 正常发送
	mailer := ch.mailer.(*fakeMailer)
	if err := svc.Send(ctx, ch, CodePurposeRegister, "new@example.com"); err != nil {
		t.Fatalf("发送注册验证码失败: %v", err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("应发送 1 封邮件，实际 %d", len(mailer.sent))
	}
	if code := extractCode(t, store, ch, CodePurposeRegister, "new@example.com"); len(code) != 6 {
		t.Fatalf("验证码应为 6 位，实际 %q", code)
	}

	// 同一邮箱 60 秒内重复发送被限流
	if err := svc.Send(ctx, ch, CodePurposeRegister, "new@example.com"); err == nil || !strings.Contains(err.Error(), "频繁") {
		t.Errorf("重复发送应被限流: %v", err)
	}
	// 已注册邮箱不能发送注册验证码
	_ = svc.db.Create(&model.HrwaiUser{
		UID: 700000000000000001, Account: "exists", Username: "已注册", Password: "x",
		Phone: "test_mail_exists", Email: "taken@example.com", Status: 1, CreatedAt: time.Now(),
	})
	if err := svc.Send(ctx, ch, CodePurposeRegister, "taken@example.com"); err == nil || !strings.Contains(err.Error(), "已注册") {
		t.Errorf("已注册邮箱应报已注册: %v", err)
	}
}

func TestSendLoginCode(t *testing.T) {
	svc, _ := newCodeTestSvc(t)
	ch := testEmailChannel(&fakeMailer{})
	db := svc.db
	ctx := context.Background()
	_ = db.Create(&model.HrwaiUser{
		UID: 700000000000000002, Account: "mail_login_user", Username: "登录用户", Password: "x",
		Phone: "test_mail_login", Email: "login@example.com", Status: 1, CreatedAt: time.Now(),
	})

	// 未注册邮箱
	if err := svc.Send(ctx, ch, CodePurposeLogin, "nobody@example.com"); err == nil || !strings.Contains(err.Error(), "尚未注册") {
		t.Errorf("未注册邮箱应报尚未注册: %v", err)
	}
	// 正常发送
	mailer := ch.mailer.(*fakeMailer)
	if err := svc.Send(ctx, ch, CodePurposeLogin, "LOGIN@example.com"); err != nil {
		t.Fatalf("发送登录验证码失败: %v", err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("应发送 1 封邮件，实际 %d", len(mailer.sent))
	}
	// 60 秒内重复发送被限流
	if err := svc.Send(ctx, ch, CodePurposeLogin, "login@example.com"); err == nil || !strings.Contains(err.Error(), "频繁") {
		t.Errorf("重复发送应被限流: %v", err)
	}
}

func TestRegisterAndLoginWithCode(t *testing.T) {
	svc, store := newCodeTestSvc(t)
	ch := testEmailChannel(&fakeMailer{})
	ctx := context.Background()
	email := "newuser@example.com"

	// 未获取验证码直接注册
	if _, err := svc.RegisterWithCode(ctx, ch, email, "123456", "张三", "", "pass123"); err == nil {
		t.Error("未获取验证码直接注册应失败")
	}

	// 获取注册验证码
	if err := svc.Send(ctx, ch, CodePurposeRegister, email); err != nil {
		t.Fatalf("发送注册验证码失败: %v", err)
	}
	code := extractCode(t, store, ch, CodePurposeRegister, email)

	// 错误验证码
	if _, err := svc.RegisterWithCode(ctx, ch, email, "000000", "张三", "", "pass123"); err == nil || !strings.Contains(err.Error(), "验证码") {
		t.Errorf("错误验证码应失败: %v", err)
	}
	// 正确验证码注册（自动登录）
	regRes, err := svc.RegisterWithCode(ctx, ch, email, code, "张三", "测试公司", "pass123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if regRes.Token == "" || regRes.Role != HrwaiRole || regRes.Account == "" || regRes.Account == email || regRes.Username != "张三" {
		t.Errorf("注册结果异常: %+v", regRes)
	}
	var created model.HrwaiUser
	if err := svc.db.First(&created, regRes.UserID).Error; err != nil || created.UID <= 0 {
		t.Errorf("注册用户应有非零 uid: %+v err=%v", created, err)
	}
	if mailer := ch.mailer.(*fakeMailer); len(mailer.sent) != 1 {
		t.Fatalf("注册应发送 1 封邮件，实际 %d", len(mailer.sent))
	}

	// 同一邮箱不能重复注册（手动写入验证码绕过发送，验证注册接口的唯一性兜底）
	dupCodeVal, _ := json.Marshal(authCodeValue{Code: "123456"})
	_ = store.Set(ctx, codeKeyForTest(ch, CodePurposeRegister, email), string(dupCodeVal), time.Minute)
	if _, err := svc.RegisterWithCode(ctx, ch, email, "123456", "李四", "", "pass123"); err == nil || !strings.Contains(err.Error(), "已注册") {
		t.Errorf("重复注册应失败: %v", err)
	}
	// 已注册邮箱发送注册验证码被拒绝
	if err := svc.Send(ctx, ch, CodePurposeRegister, email); err == nil || !strings.Contains(err.Error(), "已注册") {
		t.Errorf("重复注册发送验证码应失败: %v", err)
	}

	// 邮箱登录
	if err := svc.Send(ctx, ch, CodePurposeLogin, email); err != nil {
		t.Fatalf("发送登录验证码失败: %v", err)
	}
	loginCode := extractCode(t, store, ch, CodePurposeLogin, email)
	loginRes, err := svc.LoginWithCode(ctx, ch, email, loginCode)
	if err != nil {
		t.Fatalf("邮箱登录失败: %v", err)
	}
	if loginRes.UserID != regRes.UserID || loginRes.Token == "" {
		t.Errorf("登录结果异常: %+v", loginRes)
	}

	// 禁用账号不能登录
	_ = svc.db.Model(&model.HrwaiUser{}).Where("email = ?", email).Update("status", 0).Error
	_ = store.Del(ctx, cache.SafeKey(ch.KeyPrefix()+"_send", "login", email))
	if err := svc.Send(ctx, ch, CodePurposeLogin, email); err != nil {
		t.Fatalf("发送登录验证码失败: %v", err)
	}
	loginCode2 := extractCode(t, store, ch, CodePurposeLogin, email)
	if _, err := svc.LoginWithCode(ctx, ch, email, loginCode2); err == nil || !strings.Contains(err.Error(), "禁用") {
		t.Errorf("禁用账号应无法登录: %v", err)
	}
}

func TestVerifyCodeAttemptLimit(t *testing.T) {
	svc, store := newCodeTestSvc(t)
	ch := testEmailChannel(&fakeMailer{})
	ctx := context.Background()
	email := "attempt@example.com"
	if err := svc.Send(ctx, ch, CodePurposeRegister, email); err != nil {
		t.Fatalf("发送验证码失败: %v", err)
	}
	code := extractCode(t, store, ch, CodePurposeRegister, email)

	for i := 0; i < 5; i++ {
		if _, err := svc.RegisterWithCode(ctx, ch, email, "000000", "张三", "", "pass123"); err == nil || !strings.Contains(err.Error(), "验证码") {
			t.Fatalf("第 %d 次错误验证码应失败: %v", i+1, err)
		}
	}
	// 错误 5 次后，即使输入正确验证码也要求重新获取
	if _, err := svc.RegisterWithCode(ctx, ch, email, code, "张三", "", "pass123"); err == nil || !strings.Contains(err.Error(), "次数过多") {
		t.Errorf("超过错误次数应要求重新获取: %v", err)
	}
}

func TestRegisterWithCode_BadEmail(t *testing.T) {
	svc, _ := newCodeTestSvc(t)
	ch := testEmailChannel(&fakeMailer{})
	if _, err := svc.RegisterWithCode(context.Background(), ch, "not-an-email", "123456", "张三", "", "pass123"); err == nil || !strings.Contains(err.Error(), "格式") {
		t.Errorf("非法邮箱应报格式错误: %v", err)
	}
}

func TestRegisterWithCode_BadPassword(t *testing.T) {
	svc, store := newCodeTestSvc(t)
	ch := testEmailChannel(&fakeMailer{})
	ctx := context.Background()
	email := "pwd@example.com"
	if err := svc.Send(ctx, ch, CodePurposeRegister, email); err != nil {
		t.Fatalf("发送验证码失败: %v", err)
	}
	code := extractCode(t, store, ch, CodePurposeRegister, email)
	if _, err := svc.RegisterWithCode(ctx, ch, email, code, "张三", "", "123"); err == nil || !strings.Contains(err.Error(), "密码长度") {
		t.Errorf("过短密码应报错: %v", err)
	}
}

func TestRegisterWithCode_EmptyNickname(t *testing.T) {
	svc, store := newCodeTestSvc(t)
	ch := testEmailChannel(&fakeMailer{})
	ctx := context.Background()
	email := "nn@example.com"
	if err := svc.Send(ctx, ch, CodePurposeRegister, email); err != nil {
		t.Fatalf("发送验证码失败: %v", err)
	}
	code := extractCode(t, store, ch, CodePurposeRegister, email)
	if _, err := svc.RegisterWithCode(ctx, ch, email, code, "  ", "", "pass123"); err == nil || !strings.Contains(err.Error(), "昵称") {
		t.Errorf("空昵称应报错: %v", err)
	}
}

func TestBindEmail(t *testing.T) {
	svc, store := newCodeTestSvc(t)
	ch := testEmailChannel(&fakeMailer{})
	db := svc.db
	ctx := context.Background()
	u1 := &model.HrwaiUser{UID: 700000000000000003, Account: "ebind1", Username: "用户一", Password: "x", Phone: "test_ebind_1", Email: "old1@example.com", Status: 1, CreatedAt: time.Now()}
	u2 := &model.HrwaiUser{UID: 700000000000000004, Account: "ebind2", Username: "用户二", Password: "x", Phone: "test_ebind_2", Email: "taken2@example.com", Status: 1, CreatedAt: time.Now()}
	_ = db.Create(u1)
	_ = db.Create(u2)

	// 目标邮箱已被他人使用
	if err := svc.SendBind(ctx, ch, u1.ID, "taken2@example.com"); err == nil || !strings.Contains(err.Error(), "已被其他账号") {
		t.Errorf("占用邮箱应报错: %v", err)
	}
	// 非法格式
	if err := svc.SendBind(ctx, ch, u1.ID, "bad-email"); err == nil || !strings.Contains(err.Error(), "格式") {
		t.Errorf("非法邮箱应报格式错误: %v", err)
	}
	// 正常绑定
	if err := svc.SendBind(ctx, ch, u1.ID, "new@example.com"); err != nil {
		t.Fatalf("发送绑定验证码失败: %v", err)
	}
	code := extractCode(t, store, ch, CodePurposeBind, "new@example.com")
	if err := svc.Bind(ctx, ch, u1.ID, "new@example.com", code); err != nil {
		t.Fatalf("绑定邮箱失败: %v", err)
	}
	var after model.HrwaiUser
	_ = db.First(&after, u1.ID).Error
	if after.Email != "new@example.com" {
		t.Errorf("绑定后邮箱 = %q", after.Email)
	}
}

func TestPhoneRegisterAndLogin(t *testing.T) {
	svc, store := newCodeTestSvc(t)
	ch := testSmsChannel(&fakeSMSProvider{})
	ctx := context.Background()
	phone := "13900001111"

	// 非法手机号
	if err := svc.Send(ctx, ch, CodePurposeRegister, "123"); err == nil || !strings.Contains(err.Error(), "格式") {
		t.Errorf("非法手机号应报格式错误: %v", err)
	}
	// 未获取验证码直接注册
	if _, err := svc.RegisterWithCode(ctx, ch, phone, "123456", "张三", "", "pass123"); err == nil {
		t.Error("未获取验证码直接注册应失败")
	}
	// 发送注册验证码
	if err := svc.Send(ctx, ch, CodePurposeRegister, phone); err != nil {
		t.Fatalf("发送注册验证码失败: %v", err)
	}
	if sms := ch.sms.(*fakeSMSProvider); len(sms.sent) != 1 {
		t.Fatalf("应发送 1 条短信，实际 %d", len(sms.sent))
	}
	code := extractCode(t, store, ch, CodePurposeRegister, phone)
	// 错误验证码
	if _, err := svc.RegisterWithCode(ctx, ch, phone, "000000", "张三", "", "pass123"); err == nil || !strings.Contains(err.Error(), "验证码") {
		t.Errorf("错误验证码应失败: %v", err)
	}
	// 正确验证码注册（账号随机生成）
	regRes, err := svc.RegisterWithCode(ctx, ch, phone, code, "张三", "测试公司", "pass123")
	if err != nil {
		t.Fatalf("手机号注册失败: %v", err)
	}
	if regRes.Token == "" || regRes.Role != HrwaiRole || regRes.Account == "" || regRes.Account == phone || regRes.Username != "张三" {
		t.Errorf("注册结果异常（账号应随机生成）: %+v", regRes)
	}
	var created model.HrwaiUser
	if err := svc.db.First(&created, regRes.UserID).Error; err != nil || created.UID <= 0 {
		t.Errorf("注册用户应有非零 uid: %+v err=%v", created, err)
	}
	// 重复注册
	if err := svc.Send(ctx, ch, CodePurposeRegister, phone); err == nil || !strings.Contains(err.Error(), "已注册") {
		t.Errorf("重复注册应报已注册: %v", err)
	}
	// 登录
	if err := svc.Send(ctx, ch, CodePurposeLogin, phone); err != nil {
		t.Fatalf("发送登录验证码失败: %v", err)
	}
	loginCode := extractCode(t, store, ch, CodePurposeLogin, phone)
	loginRes, err := svc.LoginWithCode(ctx, ch, phone, loginCode)
	if err != nil {
		t.Fatalf("手机号登录失败: %v", err)
	}
	if loginRes.UserID != regRes.UserID {
		t.Errorf("登录用户不一致: %+v", loginRes)
	}
	// 未注册手机号登录
	if err := svc.Send(ctx, ch, CodePurposeLogin, "13700009999"); err == nil || !strings.Contains(err.Error(), "尚未注册") {
		t.Errorf("未注册手机号应报尚未注册: %v", err)
	}
}

// TestPhoneResetPassword 手机号忘记密码：发码→校验→重置，密码生效且验证码消费。
func TestPhoneResetPassword(t *testing.T) {
	svc, store := newCodeTestSvc(t)
	ch := testSmsChannel(&fakeSMSProvider{})
	ctx := context.Background()
	phone := "13900002222"

	// 未注册手机号找回密码 → 发码报「尚未注册」
	if err := svc.Send(ctx, ch, CodePurposeResetPassword, phone); err == nil || !strings.Contains(err.Error(), "尚未注册") {
		t.Fatalf("未注册手机号找回应报尚未注册: %v", err)
	}

	// 注册一个手机号账号
	if err := svc.Send(ctx, ch, CodePurposeRegister, phone); err != nil {
		t.Fatalf("注册发码失败: %v", err)
	}
	regCode := extractCode(t, store, ch, CodePurposeRegister, phone)
	if _, err := svc.RegisterWithCode(ctx, ch, phone, regCode, "李四", "", "oldpass123"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	// 发找回密码验证码
	if err := svc.Send(ctx, ch, CodePurposeResetPassword, phone); err != nil {
		t.Fatalf("找回密码发码失败: %v", err)
	}
	resetCode := extractCode(t, store, ch, CodePurposeResetPassword, phone)

	// 错误验证码
	if err := svc.ResetPasswordWithCode(ctx, ch, phone, "000000", "newpass123"); err == nil || !strings.Contains(err.Error(), "验证码") {
		t.Fatalf("错误验证码应失败: %v", err)
	}
	// 密码过短（先于验证码校验，不消费验证码）
	if err := svc.ResetPasswordWithCode(ctx, ch, phone, resetCode, "123"); err == nil || !strings.Contains(err.Error(), "6-20") {
		t.Fatalf("密码过短应失败: %v", err)
	}
	// 正确验证码 + 合法密码
	if err := svc.ResetPasswordWithCode(ctx, ch, phone, resetCode, "newpass123"); err != nil {
		t.Fatalf("重置密码失败: %v", err)
	}

	// 新密码已生效、旧密码失效
	var user model.HrwaiUser
	if err := svc.db.Where("phone = ?", phone).First(&user).Error; err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}
	if !VerifyPassword("newpass123", user.Password) {
		t.Error("新密码未生效")
	}
	if VerifyPassword("oldpass123", user.Password) {
		t.Error("旧密码仍有效")
	}

	// 验证码已消费，复用应失败
	if err := svc.ResetPasswordWithCode(ctx, ch, phone, resetCode, "another123"); err == nil {
		t.Error("验证码应已被消费")
	}
}

// TestPhoneChangePassword 修改密码：发码→校验→更新，密码生效且验证码消费。
func TestPhoneChangePassword(t *testing.T) {
	svc, store := newCodeTestSvc(t)
	ch := testSmsChannel(&fakeSMSProvider{})
	ctx := context.Background()
	phone := "13900003333"

	// 注册手机号账号
	if err := svc.Send(ctx, ch, CodePurposeRegister, phone); err != nil {
		t.Fatalf("注册发码失败: %v", err)
	}
	regCode := extractCode(t, store, ch, CodePurposeRegister, phone)
	regRes, err := svc.RegisterWithCode(ctx, ch, phone, regCode, "王五", "", "oldpass123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	// 发修改密码验证码
	if err := svc.SendChangePasswordCode(ctx, ch, regRes.UserID); err != nil {
		t.Fatalf("修改密码发码失败: %v", err)
	}
	changeCode := extractCode(t, store, ch, CodePurposeChangePassword, phone)

	// 密码过短（先于验证码校验，不消费验证码）
	if err := svc.ChangePassword(ctx, ch, regRes.UserID, changeCode, "123"); err == nil || !strings.Contains(err.Error(), "6-20") {
		t.Fatalf("密码过短应失败: %v", err)
	}
	// 错误验证码
	if err := svc.ChangePassword(ctx, ch, regRes.UserID, "000000", "newpass123"); err == nil || !strings.Contains(err.Error(), "验证码") {
		t.Fatalf("错误验证码应失败: %v", err)
	}
	// 正确验证码 + 合法密码
	if err := svc.ChangePassword(ctx, ch, regRes.UserID, changeCode, "newpass123"); err != nil {
		t.Fatalf("修改密码失败: %v", err)
	}

	// 新密码已生效、旧密码失效
	var user model.HrwaiUser
	if err := svc.db.Where("phone = ?", phone).First(&user).Error; err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}
	if !VerifyPassword("newpass123", user.Password) {
		t.Error("新密码未生效")
	}
	if VerifyPassword("oldpass123", user.Password) {
		t.Error("旧密码仍有效")
	}
}

func TestPhoneBind(t *testing.T) {
	svc, store := newCodeTestSvc(t)
	ch := testSmsChannel(&fakeSMSProvider{})
	db := svc.db
	ctx := context.Background()
	u1 := &model.HrwaiUser{UID: 700000000000000005, Account: "pbind1", Username: "用户一", Password: "x", Phone: "13800001111", Status: 1, CreatedAt: time.Now()}
	u2 := &model.HrwaiUser{UID: 700000000000000006, Account: "pbind2", Username: "用户二", Password: "x", Phone: "13800002222", Status: 1, CreatedAt: time.Now()}
	_ = db.Create(u1)
	_ = db.Create(u2)

	// 目标手机号已被他人使用
	if err := svc.SendBind(ctx, ch, u1.ID, "13800002222"); err == nil || !strings.Contains(err.Error(), "已被其他账号") {
		t.Errorf("占用手机号应报错: %v", err)
	}
	// 非法格式
	if err := svc.SendBind(ctx, ch, u1.ID, "123"); err == nil || !strings.Contains(err.Error(), "格式") {
		t.Errorf("非法手机号应报格式错误: %v", err)
	}
	// 正常绑定
	if err := svc.SendBind(ctx, ch, u1.ID, "13800003333"); err != nil {
		t.Fatalf("发送绑定验证码失败: %v", err)
	}
	code := extractCode(t, store, ch, CodePurposeBind, "13800003333")
	if err := svc.Bind(ctx, ch, u1.ID, "13800003333", code); err != nil {
		t.Fatalf("绑定手机号失败: %v", err)
	}
	var after model.HrwaiUser
	_ = db.First(&after, u1.ID).Error
	if after.Phone != "13800003333" {
		t.Errorf("绑定后手机号 = %q", after.Phone)
	}
}

func TestPhoneRegister_EmptyNickname(t *testing.T) {
	svc, store := newCodeTestSvc(t)
	ch := testSmsChannel(&fakeSMSProvider{})
	ctx := context.Background()
	phone := "13700009998"
	if err := svc.Send(ctx, ch, CodePurposeRegister, phone); err != nil {
		t.Fatalf("发送验证码失败: %v", err)
	}
	code := extractCode(t, store, ch, CodePurposeRegister, phone)
	if _, err := svc.RegisterWithCode(ctx, ch, phone, code, "  ", "", "pass123"); err == nil || !strings.Contains(err.Error(), "昵称") {
		t.Errorf("空昵称应报错: %v", err)
	}
}

// TestCodeKey_CompatibleWithLegacy 验证新 engine 的缓存 key 与旧实现完全一致（线上 Redis 数据兼容）。
func TestCodeKey_CompatibleWithLegacy(t *testing.T) {
	svc, _ := newCodeTestSvc(t)
	emailCh := testEmailChannel(&fakeMailer{})
	phoneCh := testSmsChannel(&fakeSMSProvider{})

	if got, want := svc.codeKey(emailCh, CodePurposeRegister, "a@b.com", false), cache.SafeKey("email_code", "register", "a@b.com"); got != want {
		t.Errorf("email code key = %q, want %q", got, want)
	}
	if got, want := svc.codeKey(emailCh, CodePurposeLogin, "a@b.com", true), cache.SafeKey("email_code_send", "login", "a@b.com"); got != want {
		t.Errorf("email throttle key = %q, want %q", got, want)
	}
	if got, want := svc.codeKey(phoneCh, CodePurposeBind, "13800000000", false), cache.SafeKey("phone_code", "bind", "13800000000"); got != want {
		t.Errorf("phone code key = %q, want %q", got, want)
	}
}

// TestSenderUnconfigured_Production 生产未配置发送器时，最先报"未配置"，且不落任何存储。
func TestSenderUnconfigured_Production(t *testing.T) {
	svc, store := newCodeTestSvc(t)
	ch := &SmsChannel{sms: nil} // 生产未接入短信
	ctx := context.Background()

	if err := svc.Send(ctx, ch, CodePurposeRegister, "13800000000"); err == nil || !strings.Contains(err.Error(), "未配置") {
		t.Errorf("未配置通道应报未配置: %v", err)
	}
	if _, err := store.Get(ctx, codeKeyForTest(ch, CodePurposeRegister, "13800000000")); err == nil {
		t.Error("未配置通道不应写入验证码")
	}
}

// TestVerifyWrongCodeDoesNotExtendTTL 锁定 ADR-0012 §5：
// 错输验证码不得重写完整 TTL（错误输入不能延长验证码生命周期）。
func TestVerifyWrongCodeDoesNotExtendTTL(t *testing.T) {
	svc, store := newCodeTestSvc(t)
	ch := testEmailChannel(&fakeMailer{})
	ctx := context.Background()
	email := "ttl@example.com"
	if err := svc.Send(ctx, ch, CodePurposeRegister, email); err != nil {
		t.Fatalf("发送验证码失败: %v", err)
	}
	key := codeKeyForTest(ch, CodePurposeRegister, email)

	// 读取发送时锁定的到期时间
	raw, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("读取验证码失败: %v", err)
	}
	var v0 authCodeValue
	if err := json.Unmarshal([]byte(raw), &v0); err != nil {
		t.Fatalf("解析验证码失败: %v", err)
	}
	if v0.ExpiresAt.IsZero() {
		t.Fatal("发送的验证码应携带 ExpiresAt")
	}

	// 错输一次：ExpiresAt 保持不变（生命周期不被延长）
	if err := svc.Verify(ctx, ch, CodePurposeRegister, email, "000000"); err == nil {
		t.Fatal("错输验证码应失败")
	}
	raw2, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("错输后验证码应仍在: %v", err)
	}
	var v1 authCodeValue
	if err := json.Unmarshal([]byte(raw2), &v1); err != nil {
		t.Fatalf("解析验证码失败: %v", err)
	}
	if !v1.ExpiresAt.Equal(v0.ExpiresAt) {
		t.Fatalf("错输后 ExpiresAt 漂移: %v → %v", v0.ExpiresAt, v1.ExpiresAt)
	}
	if v1.Attempts != 1 {
		t.Fatalf("Attempts = %d, want 1", v1.Attempts)
	}
}
