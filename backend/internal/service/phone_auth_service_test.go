package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newPhoneTestSvc(t *testing.T) (*PhoneAuthService, *memCodeStore, *fakeSMSProvider) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	authSvc := NewAuthService(db, "test-secret", time.Hour, "a", "t", "s")
	store := newMemCodeStore()
	sms := &fakeSMSProvider{}
	return &PhoneAuthService{
		db: db, authSvc: authSvc, store: store, sender: sms, codeTTL: 5 * time.Minute,
	}, store, sms
}

type fakeSMSProvider struct {
	sent []string
}

func (f *fakeSMSProvider) Send(to, content string) error {
	f.sent = append(f.sent, to+"|"+content)
	return nil
}

func extractPhoneCode(t *testing.T, store *memCodeStore, purpose PhoneCodePurpose, phone string) string {
	t.Helper()
	raw, err := store.Get(context.Background(), phoneCodeKey(purpose, phone))
	if err != nil {
		t.Fatalf("读取手机验证码失败: %v", err)
	}
	var v authCodeValue
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("解析手机验证码失败: %v", err)
	}
	return v.Code
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

func TestPhoneRegisterAndLogin(t *testing.T) {
	svc, store, sms := newPhoneTestSvc(t)
	ctx := context.Background()
	phone := "13900001111"

	// 非法手机号
	if err := svc.SendRegisterCode(ctx, "123"); err == nil || !strings.Contains(err.Error(), "格式") {
		t.Errorf("非法手机号应报格式错误: %v", err)
	}
	// 未获取验证码直接注册
	if _, err := svc.RegisterWithCode(ctx, phone, "123456", "张三", "", "pass123"); err == nil {
		t.Error("未获取验证码直接注册应失败")
	}
	// 发送注册验证码
	if err := svc.SendRegisterCode(ctx, phone); err != nil {
		t.Fatalf("发送注册验证码失败: %v", err)
	}
	if len(sms.sent) != 1 {
		t.Fatalf("应发送 1 条短信，实际 %d", len(sms.sent))
	}
	code := extractPhoneCode(t, store, PhoneCodeRegister, phone)
	// 错误验证码
	if _, err := svc.RegisterWithCode(ctx, phone, "000000", "张三", "", "pass123"); err == nil || !strings.Contains(err.Error(), "验证码") {
		t.Errorf("错误验证码应失败: %v", err)
	}
	// 正确验证码注册（账号随机生成）
	regRes, err := svc.RegisterWithCode(ctx, phone, code, "张三", "测试公司", "pass123")
	if err != nil {
		t.Fatalf("手机号注册失败: %v", err)
	}
	if regRes.Token == "" || regRes.Role != HrwaiRole || regRes.Username == "" || regRes.Username == phone {
		t.Errorf("注册结果异常（账号应随机生成）: %+v", regRes)
	}
	// 重复注册
	if err := svc.SendRegisterCode(ctx, phone); err == nil || !strings.Contains(err.Error(), "已注册") {
		t.Errorf("重复注册应报已注册: %v", err)
	}
	// 登录
	if err := svc.SendLoginCode(ctx, phone); err != nil {
		t.Fatalf("发送登录验证码失败: %v", err)
	}
	loginCode := extractPhoneCode(t, store, PhoneCodeLogin, phone)
	loginRes, err := svc.LoginWithCode(ctx, phone, loginCode)
	if err != nil {
		t.Fatalf("手机号登录失败: %v", err)
	}
	if loginRes.UserID != regRes.UserID {
		t.Errorf("登录用户不一致: %+v", loginRes)
	}
	// 未注册手机号登录
	if err := svc.SendLoginCode(ctx, "13700009999"); err == nil || !strings.Contains(err.Error(), "尚未注册") {
		t.Errorf("未注册手机号应报尚未注册: %v", err)
	}
}

func TestPhoneBind(t *testing.T) {
	svc, store, _ := newPhoneTestSvc(t)
	db := svc.db
	ctx := context.Background()
	u1 := &model.HrwaiUser{Username: "pbind1", Password: "x", Name: "用户一", Phone: "13800001111", Status: 1, CreatedAt: time.Now()}
	u2 := &model.HrwaiUser{Username: "pbind2", Password: "x", Name: "用户二", Phone: "13800002222", Status: 1, CreatedAt: time.Now()}
	_ = db.Create(u1)
	_ = db.Create(u2)

	// 目标手机号已被他人使用
	if err := svc.SendBindCode(ctx, u1.ID, "13800002222"); err == nil || !strings.Contains(err.Error(), "已被其他账号") {
		t.Errorf("占用手机号应报错: %v", err)
	}
	// 非法格式
	if err := svc.SendBindCode(ctx, u1.ID, "123"); err == nil || !strings.Contains(err.Error(), "格式") {
		t.Errorf("非法手机号应报格式错误: %v", err)
	}
	// 正常绑定
	if err := svc.SendBindCode(ctx, u1.ID, "13800003333"); err != nil {
		t.Fatalf("发送绑定验证码失败: %v", err)
	}
	code := extractPhoneCode(t, store, PhoneCodeBind, "13800003333")
	if err := svc.BindPhone(ctx, u1.ID, "13800003333", code); err != nil {
		t.Fatalf("绑定手机号失败: %v", err)
	}
	var after model.HrwaiUser
	_ = db.First(&after, u1.ID).Error
	if after.Phone != "13800003333" {
		t.Errorf("绑定后手机号 = %q", after.Phone)
	}
}
