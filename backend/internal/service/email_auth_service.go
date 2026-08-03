// Package service 实现业务服务层。
// 本文件：邮箱注册/登录（格式校验、唯一性校验、验证码发送与校验）。
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/cache"
	"forklift-training/internal/config"
	"forklift-training/internal/model"
)

// EmailCodePurpose 验证码用途。
type EmailCodePurpose string

const (
	// EmailCodeRegister 注册验证码。
	EmailCodeRegister EmailCodePurpose = "register"
	// EmailCodeLogin 登录验证码。
	EmailCodeLogin EmailCodePurpose = "login"
)

// EmailCodeStore 验证码存储接口（生产用 Redis，测试用内存实现）。
type EmailCodeStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

// RedisEmailCodeStore 基于全局 Redis 缓存的验证码存储。
type RedisEmailCodeStore struct{}

// Get 读取验证码。
func (RedisEmailCodeStore) Get(ctx context.Context, key string) (string, error) {
	return cache.Get(ctx, key)
}

// Set 写入验证码。
func (RedisEmailCodeStore) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return cache.Set(ctx, key, value, ttl)
}

// Del 删除验证码。
func (RedisEmailCodeStore) Del(ctx context.Context, keys ...string) error {
	return cache.Del(ctx, keys...)
}

// MailSender 邮件发送接口（SMTP 生产实现 / 日志降级实现 / 测试替身）。
type MailSender interface {
	Send(to, subject, body string) error
}

// SMTPMailSender 通过 SMTP 发送邮件。
type SMTPMailSender struct {
	cfg config.SMTPConfig
}

// Send 发送一封 UTF-8 纯文本邮件。
func (s SMTPMailSender) Send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	from := s.cfg.From
	msg := fmt.Sprintf(
		"From: %s <%s>\r\nTo: <%s>\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.cfg.FromName, from, to, subject, body,
	)
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

// LogMailSender 开发环境降级实现：验证码写入服务日志（未配置 SMTP 时便于本地验证）。
type LogMailSender struct{}

// Send 将邮件内容写入日志。
func (LogMailSender) Send(to, subject, body string) error {
	slog.Info("邮件发送（开发环境降级为日志）", "to", to, "subject", subject, "body", body)
	return nil
}

// emailCodeValue 验证码存储值（含错误次数限制，最多 5 次）。
type emailCodeValue struct {
	Code     string `json:"code"`
	Attempts int    `json:"attempts"`
}

// EmailAuthService 邮箱注册/登录服务。
type EmailAuthService struct {
	db      *gorm.DB
	authSvc *AuthService
	store   EmailCodeStore
	sender  MailSender
	codeTTL time.Duration
}

// NewEmailAuthService 构造邮箱认证服务。
// 未配置 SMTP 时：开发环境降级为日志发送验证码，生产环境发送接口返回明确错误。
func NewEmailAuthService(db *gorm.DB, authSvc *AuthService, smtpCfg config.SMTPConfig, codeTTL time.Duration, isProd bool) *EmailAuthService {
	var sender MailSender = LogMailSender{}
	if smtpCfg.Host != "" && smtpCfg.From != "" {
		sender = SMTPMailSender{cfg: smtpCfg}
	} else if isProd {
		sender = nil
	}
	if codeTTL <= 0 {
		codeTTL = 5 * time.Minute
	}
	return &EmailAuthService{
		db:      db,
		authSvc: authSvc,
		store:   RedisEmailCodeStore{},
		sender:  sender,
		codeTTL: codeTTL,
	}
}

// NormalizeEmail 邮箱归一化：去首尾空格并统一小写（邮箱不区分大小写）。
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// IsValidEmail 校验邮箱格式：非空、长度 ≤255、合法地址且域名含点。
func IsValidEmail(email string) bool {
	if email == "" || len(email) > 255 {
		return false
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return false
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return false
	}
	return strings.Contains(parts[1], ".")
}

// SendRegisterCode 发送注册验证码：校验格式与唯一性，通过后向邮箱发送验证码。
func (s *EmailAuthService) SendRegisterCode(ctx context.Context, email string) error {
	return s.sendCode(ctx, EmailCodeRegister, email)
}

// SendLoginCode 发送登录验证码：校验格式与邮箱是否已注册，通过后向邮箱发送验证码。
func (s *EmailAuthService) SendLoginCode(ctx context.Context, email string) error {
	return s.sendCode(ctx, EmailCodeLogin, email)
}

// sendCode 发送验证码的公共流程。
func (s *EmailAuthService) sendCode(ctx context.Context, purpose EmailCodePurpose, email string) error {
	if s.sender == nil {
		return errors.New("邮件服务未配置，请联系管理员")
	}
	email = NormalizeEmail(email)
	if !IsValidEmail(email) {
		return errors.New("邮箱格式不正确")
	}

	// 注册：邮箱必须未注册；登录：邮箱必须已注册
	var count int64
	if err := s.db.Model(&model.HrwaiUser{}).
		Where("LOWER(email) = ? AND email <> ''", email).Count(&count).Error; err != nil {
		return err
	}
	switch purpose {
	case EmailCodeRegister:
		if count > 0 {
			return errors.New("该邮箱已注册，请直接登录")
		}
	case EmailCodeLogin:
		if count == 0 {
			return errors.New("该邮箱尚未注册")
		}
	default:
		return errors.New("无效的验证码用途")
	}

	// 发送频率限制：同一邮箱同一用途 60 秒内只能发送一次
	if _, err := s.store.Get(ctx, emailSendThrottleKey(purpose, email)); err == nil {
		return errors.New("验证码发送过于频繁，请稍后再试")
	}

	code, err := generateEmailCode()
	if err != nil {
		return errors.New("验证码生成失败，请稍后再试")
	}
	val, _ := json.Marshal(emailCodeValue{Code: code})
	if err := s.store.Set(ctx, emailCodeKey(purpose, email), string(val), s.codeTTL); err != nil {
		return errors.New("验证码生成失败，请稍后再试")
	}
	if err := s.store.Set(ctx, emailSendThrottleKey(purpose, email), "1", time.Minute); err != nil {
		return errors.New("验证码生成失败，请稍后再试")
	}

	purposeLabel := "注册"
	if purpose == EmailCodeLogin {
		purposeLabel = "登录"
	}
	subject := "【和润天下】" + purposeLabel + "验证码"
	body := fmt.Sprintf(
		"您好！\n\n您正在进行%s操作，本次验证码为：%s\n验证码 %d 分钟内有效，请勿泄露给他人。\n\n如非本人操作，请忽略本邮件。",
		purposeLabel, code, int(s.codeTTL.Minutes()),
	)
	if err := s.sender.Send(email, subject, body); err != nil {
		slog.Error("验证码邮件发送失败", "email", email, "error", err)
		return errors.New("验证码发送失败，请稍后再试")
	}
	return nil
}

// RegisterWithCode 邮箱注册：验证码校验通过后才创建账号，并直接签发登录令牌。
func (s *EmailAuthService) RegisterWithCode(ctx context.Context, email, code, name, company string) (*LoginResult, error) {
	email = NormalizeEmail(email)
	if !IsValidEmail(email) {
		return nil, errors.New("邮箱格式不正确")
	}
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("姓名不能为空")
	}
	if err := s.verifyCode(ctx, EmailCodeRegister, email, code); err != nil {
		return nil, err
	}

	// 再次校验唯一性（防并发重复注册；数据库唯一索引兜底）
	var count int64
	if err := s.db.Model(&model.HrwaiUser{}).
		Where("LOWER(email) = ?", email).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("该邮箱已注册，请直接登录")
	}

	// 邮箱注册用户通过验证码登录，密码随机生成（用户不可知，也无需记忆）
	randomPwd, err := randomPassword()
	if err != nil {
		return nil, errors.New("注册失败，请稍后再试")
	}
	hashed, err := HashPassword(randomPwd)
	if err != nil {
		return nil, errors.New("注册失败，请稍后再试")
	}
	user := model.HrwaiUser{
		Username:  email,
		Password:  hashed,
		Name:      strings.TrimSpace(name),
		Nickname:  generateDefaultNickname(s.db),
		Phone:     emailPlaceholderPhone(email),
		Email:     email,
		Company:   strings.TrimSpace(company),
		Status:    1,
		CreatedAt: beijingNow(),
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, errors.New("注册失败，该邮箱可能已被注册")
	}

	token, err := s.authSvc.GenerateToken(user.ID, user.Username, HrwaiRole)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token:    token,
		UserID:   user.ID,
		Username: user.Username,
		Name:     user.Name,
		Role:     HrwaiRole,
	}, nil
}

// LoginWithCode 邮箱验证码登录：验证码校验通过后签发登录令牌。
func (s *EmailAuthService) LoginWithCode(ctx context.Context, email, code string) (*LoginResult, error) {
	email = NormalizeEmail(email)
	if !IsValidEmail(email) {
		return nil, errors.New("邮箱格式不正确")
	}
	if err := s.verifyCode(ctx, EmailCodeLogin, email, code); err != nil {
		return nil, err
	}

	var user model.HrwaiUser
	if err := s.db.Where("LOWER(email) = ?", email).First(&user).Error; err != nil {
		return nil, errors.New("该邮箱尚未注册")
	}
	if user.Status != 1 {
		return nil, errors.New("账号已被禁用，请联系管理员")
	}
	token, err := s.authSvc.GenerateToken(user.ID, user.Username, HrwaiRole)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token:    token,
		UserID:   user.ID,
		Username: user.Username,
		Name:     user.Name,
		Role:     HrwaiRole,
	}, nil
}

// verifyCode 校验验证码并限制错误次数（最多 5 次），成功后删除验证码。
func (s *EmailAuthService) verifyCode(ctx context.Context, purpose EmailCodePurpose, email, code string) error {
	email = NormalizeEmail(email)
	key := emailCodeKey(purpose, email)
	raw, err := s.store.Get(ctx, key)
	if err != nil {
		return errors.New("验证码错误或已过期")
	}
	var v emailCodeValue
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

// emailCodeKey 验证码缓存 key。
func emailCodeKey(purpose EmailCodePurpose, email string) string {
	return cache.SafeKey("email_code", string(purpose), email)
}

// emailSendThrottleKey 发送频率限制缓存 key。
func emailSendThrottleKey(purpose EmailCodePurpose, email string) string {
	return cache.SafeKey("email_code_send", string(purpose), email)
}

// generateEmailCode 生成 6 位数字验证码（加密安全随机数）。
func generateEmailCode() (string, error) {
	const digits = "0123456789"
	b := make([]byte, 6)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		b[i] = digits[n.Int64()]
	}
	return string(b), nil
}

// randomPassword 生成随机密码（邮箱注册用户不可知，仅占位使用）。
func randomPassword() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// emailPlaceholderPhone 为邮箱注册账号生成唯一的 phone 占位值（phone 列 NOT NULL）。
func emailPlaceholderPhone(email string) string {
	sum := sha256.Sum256([]byte(email))
	return "email_" + hex.EncodeToString(sum[:])[:32]
}
