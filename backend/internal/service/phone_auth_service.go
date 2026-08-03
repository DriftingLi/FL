// Package service 实现业务服务层。
// 本文件：手机号注册/登录/绑定（框架实现：表单与邮箱对齐，短信通道待接入）。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"forklift-training/internal/cache"
	"forklift-training/internal/model"
)

// PhoneCodePurpose 手机号验证码用途。
type PhoneCodePurpose string

const (
	// PhoneCodeRegister 注册验证码。
	PhoneCodeRegister PhoneCodePurpose = "register"
	// PhoneCodeLogin 登录验证码。
	PhoneCodeLogin PhoneCodePurpose = "login"
	// PhoneCodeBind 绑定/修改手机号验证码。
	PhoneCodeBind PhoneCodePurpose = "bind"
)

// phonePattern 中国大陆手机号格式。
var phonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

// SMSProvider 短信发送接口（真实短信通道待接入，开发环境用日志降级）。
type SMSProvider interface {
	Send(to, content string) error
}

// LogSMSProvider 开发环境降级实现：验证码写入服务日志。
type LogSMSProvider struct{}

// Send 将短信内容写入日志。
func (LogSMSProvider) Send(to, content string) error {
	slog.Info("短信发送（开发环境降级为日志）", "to", to, "content", content)
	return nil
}

// PhoneAuthService 手机号注册/登录/绑定服务。
type PhoneAuthService struct {
	db      *gorm.DB
	authSvc *AuthService
	store   AuthCodeStore
	sender  SMSProvider
	codeTTL time.Duration
}

// NewPhoneAuthService 构造手机号认证服务。
// 真实短信通道待接入：开发环境降级为日志打印验证码，生产环境未配置时发送接口报错。
func NewPhoneAuthService(db *gorm.DB, authSvc *AuthService, codeTTL time.Duration, isProd bool) *PhoneAuthService {
	var sender SMSProvider = LogSMSProvider{}
	if isProd {
		sender = nil
	}
	if codeTTL <= 0 {
		codeTTL = 5 * time.Minute
	}
	return &PhoneAuthService{
		db: db, authSvc: authSvc, store: RedisAuthCodeStore{}, sender: sender, codeTTL: codeTTL,
	}
}

// NormalizePhone 手机号归一化（去首尾空格）。
func NormalizePhone(phone string) string {
	return strings.TrimSpace(phone)
}

// IsValidPhone 校验中国大陆手机号格式（1 开头 11 位数字）。
func IsValidPhone(phone string) bool {
	return phonePattern.MatchString(strings.TrimSpace(phone))
}

// SendRegisterCode 发送注册验证码。
func (s *PhoneAuthService) SendRegisterCode(ctx context.Context, phone string) error {
	return s.sendCode(ctx, PhoneCodeRegister, phone)
}

// SendLoginCode 发送登录验证码。
func (s *PhoneAuthService) SendLoginCode(ctx context.Context, phone string) error {
	return s.sendCode(ctx, PhoneCodeLogin, phone)
}

// sendCode 发送验证码公共流程（与邮箱流程对齐）。
func (s *PhoneAuthService) sendCode(ctx context.Context, purpose PhoneCodePurpose, phone string) error {
	if s.sender == nil {
		return errors.New("短信服务未配置，请联系管理员")
	}
	phone = NormalizePhone(phone)
	if !IsValidPhone(phone) {
		return errors.New("手机号格式不正确")
	}
	var count int64
	if err := s.db.Model(&model.HrwaiUser{}).Where("phone = ?", phone).Count(&count).Error; err != nil {
		return err
	}
	switch purpose {
	case PhoneCodeRegister:
		if count > 0 {
			return errors.New("该手机号已注册，请直接登录")
		}
	case PhoneCodeLogin:
		if count == 0 {
			return errors.New("该手机号尚未注册")
		}
	default:
		return errors.New("无效的验证码用途")
	}
	if _, err := s.store.Get(ctx, phoneSendThrottleKey(purpose, phone)); err == nil {
		return errors.New("验证码发送过于频繁，请稍后再试")
	}
	code, err := generatePhoneCode()
	if err != nil {
		return errors.New("验证码生成失败，请稍后再试")
	}
	val, _ := json.Marshal(authCodeValue{Code: code})
	if err := s.store.Set(ctx, phoneCodeKey(purpose, phone), string(val), s.codeTTL); err != nil {
		return errors.New("验证码生成失败，请稍后再试")
	}
	if err := s.store.Set(ctx, phoneSendThrottleKey(purpose, phone), "1", time.Minute); err != nil {
		return errors.New("验证码生成失败，请稍后再试")
	}
	purposeLabel := "注册"
	if purpose == PhoneCodeLogin {
		purposeLabel = "登录"
	}
	content := fmt.Sprintf(
		"【和润天下】您的%s验证码为：%s，%d 分钟内有效，请勿泄露给他人。",
		purposeLabel, code, int(s.codeTTL.Minutes()),
	)
	if err := s.sender.Send(phone, content); err != nil {
		slog.Error("验证码短信发送失败", "phone", phone, "error", err)
		return errors.New("验证码发送失败，请稍后再试")
	}
	return nil
}

// RegisterWithCode 手机号注册：验证码通过后创建账号（昵称 + 账号随机生成 + 可设置密码）并自动登录。
func (s *PhoneAuthService) RegisterWithCode(ctx context.Context, phone, code, nickname, company, password string) (*LoginResult, error) {
	phone = NormalizePhone(phone)
	if !IsValidPhone(phone) {
		return nil, errors.New("手机号格式不正确")
	}
	nickname = strings.TrimSpace(nickname)
	if nickname == "" {
		return nil, errors.New("昵称不能为空")
	}
	if utf8.RuneCountInString(nickname) > 30 {
		return nil, errors.New("昵称不能超过 30 个字符")
	}
	if len(password) < 6 || len(password) > 20 {
		return nil, errors.New("密码长度需为 6-20 位")
	}
	if err := s.verifyCode(ctx, PhoneCodeRegister, phone, code); err != nil {
		return nil, err
	}
	var count int64
	if err := s.db.Model(&model.HrwaiUser{}).Where("phone = ?", phone).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("该手机号已注册，请直接登录")
	}
	username, err := generateRandomUsername()
	if err != nil {
		return nil, errors.New("注册失败，请稍后再试")
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, errors.New("注册失败，请稍后再试")
	}
	user := model.HrwaiUser{
		Username:  username,
		Password:  hashed,
		Name:      "",
		Nickname:  nickname,
		Phone:     phone,
		Company:   strings.TrimSpace(company),
		Status:    1,
		CreatedAt: beijingNow(),
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, errors.New("注册失败，该手机号可能已被注册")
	}
	token, err := s.authSvc.GenerateToken(user.ID, user.Username, HrwaiRole)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token: token, UserID: user.ID, Username: user.Username, Name: user.Name, Role: HrwaiRole,
	}, nil
}

// LoginWithCode 手机号验证码登录。
func (s *PhoneAuthService) LoginWithCode(ctx context.Context, phone, code string) (*LoginResult, error) {
	phone = NormalizePhone(phone)
	if !IsValidPhone(phone) {
		return nil, errors.New("手机号格式不正确")
	}
	if err := s.verifyCode(ctx, PhoneCodeLogin, phone, code); err != nil {
		return nil, err
	}
	var user model.HrwaiUser
	if err := s.db.Where("phone = ?", phone).First(&user).Error; err != nil {
		return nil, errors.New("该手机号尚未注册")
	}
	if user.Status != 1 {
		return nil, errors.New("账号已被禁用，请联系管理员")
	}
	token, err := s.authSvc.GenerateToken(user.ID, user.Username, HrwaiRole)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token: token, UserID: user.ID, Username: user.Username, Name: user.Name, Role: HrwaiRole,
	}, nil
}

// SendBindCode 发送绑定/修改手机号验证码：校验格式与唯一性（排除当前用户）。
func (s *PhoneAuthService) SendBindCode(ctx context.Context, userID int, phone string) error {
	if s.sender == nil {
		return errors.New("短信服务未配置，请联系管理员")
	}
	phone = NormalizePhone(phone)
	if !IsValidPhone(phone) {
		return errors.New("手机号格式不正确")
	}
	var count int64
	if err := s.db.Model(&model.HrwaiUser{}).
		Where("phone = ? AND id <> ?", phone, userID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该手机号已被其他账号使用")
	}
	if _, err := s.store.Get(ctx, phoneSendThrottleKey(PhoneCodeBind, phone)); err == nil {
		return errors.New("验证码发送过于频繁，请稍后再试")
	}
	code, err := generatePhoneCode()
	if err != nil {
		return errors.New("验证码生成失败，请稍后再试")
	}
	val, _ := json.Marshal(authCodeValue{Code: code})
	if err := s.store.Set(ctx, phoneCodeKey(PhoneCodeBind, phone), string(val), s.codeTTL); err != nil {
		return errors.New("验证码生成失败，请稍后再试")
	}
	if err := s.store.Set(ctx, phoneSendThrottleKey(PhoneCodeBind, phone), "1", time.Minute); err != nil {
		return errors.New("验证码生成失败，请稍后再试")
	}
	content := fmt.Sprintf(
		"【和润天下】您正在绑定/修改手机号，验证码为：%s，%d 分钟内有效，请勿泄露给他人。",
		code, int(s.codeTTL.Minutes()),
	)
	if err := s.sender.Send(phone, content); err != nil {
		slog.Error("验证码短信发送失败", "phone", phone, "error", err)
		return errors.New("验证码发送失败，请稍后再试")
	}
	return nil
}

// BindPhone 校验验证码后绑定/修改当前用户手机号（格式与唯一性双重校验）。
func (s *PhoneAuthService) BindPhone(ctx context.Context, userID int, phone, code string) error {
	phone = NormalizePhone(phone)
	if !IsValidPhone(phone) {
		return errors.New("手机号格式不正确")
	}
	if err := s.verifyCode(ctx, PhoneCodeBind, phone, code); err != nil {
		return err
	}
	var count int64
	if err := s.db.Model(&model.HrwaiUser{}).
		Where("phone = ? AND id <> ?", phone, userID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该手机号已被其他账号使用")
	}
	return s.db.Model(&model.HrwaiUser{}).Where("id = ?", userID).Update("phone", phone).Error
}

// verifyCode 校验验证码并限制错误次数（最多 5 次），成功后删除验证码。
func (s *PhoneAuthService) verifyCode(ctx context.Context, purpose PhoneCodePurpose, phone, code string) error {
	phone = NormalizePhone(phone)
	key := phoneCodeKey(purpose, phone)
	raw, err := s.store.Get(ctx, key)
	if err != nil {
		return errors.New("验证码错误或已过期")
	}
	var v authCodeValue
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return errors.New("验证码错误或已过期")
	}
	if v.Attempts >= 5 {
		_ = s.store.Del(ctx, key)
		return errors.New("验证码错误次数过多，请重新获取")
	}
	if v.Code != strings.TrimSpace(code) {
		v.Attempts++
		val, _ := json.Marshal(v)
		_ = s.store.Set(ctx, key, string(val), s.codeTTL)
		return errors.New("验证码错误或已过期")
	}
	_ = s.store.Del(ctx, key)
	return nil
}

// phoneCodeKey 手机号验证码缓存 key。
func phoneCodeKey(purpose PhoneCodePurpose, phone string) string {
	return cache.SafeKey("phone_code", string(purpose), phone)
}

// phoneSendThrottleKey 手机号发送频率限制缓存 key。
func phoneSendThrottleKey(purpose PhoneCodePurpose, phone string) string {
	return cache.SafeKey("phone_code_send", string(purpose), phone)
}

// generatePhoneCode 生成 6 位数字验证码（与邮箱共用生成逻辑）。
func generatePhoneCode() (string, error) {
	return generateEmailCode()
}
