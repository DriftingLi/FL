// Package service 实现业务服务层。
// 本文件：验证码 engine（邮箱/短信/绑定共用一套验证码状态机）。
// 邮箱与短信是同一状态机两侧的 channel adapter：唯一差异是归一化、
// 账号查询、文案与发送通道。
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"math/big"
	"net/mail"
	"net/smtp"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"forklift-training/internal/cache"
	"forklift-training/internal/config"
	"forklift-training/internal/model"
)

// CodePurpose 验证码用途（注册 / 登录 / 绑定 / 修改账号）。
type CodePurpose string

const (
	// CodePurposeRegister 注册验证码。
	CodePurposeRegister CodePurpose = "register"
	// CodePurposeLogin 登录验证码。
	CodePurposeLogin CodePurpose = "login"
	// CodePurposeBind 绑定/修改邮箱或手机号验证码。
	CodePurposeBind CodePurpose = "bind"
	// CodePurposeAccountChange 修改登录账号验证码。
	CodePurposeAccountChange CodePurpose = "account_change"
	// CodePurposeResetPassword 忘记密码/重置密码验证码。
	CodePurposeResetPassword CodePurpose = "reset_password"
)

// CodeChannel 验证码通道 adapter：归一化、账号查询、文案与发送的差异收敛于此。
type CodeChannel interface {
	// SenderReady 返回发送器是否可用；生产未配置通道时最先拦截（与旧行为一致：不存储验证码）。
	SenderReady() error
	// Normalize 归一化 target 并校验格式；非法时返回用户可见错误。
	Normalize(target string) (string, error)
	// Noun 通道目标的中文名（用于错误文案）。
	Noun() string
	// KeyPrefix 验证码缓存 key 前缀（"email_code" / "phone_code"，与线上 Redis key 兼容）。
	KeyPrefix() string
	// FindAccount 统计匹配 target 的账号数；excludeUserID > 0 时排除该用户（绑定场景）。
	FindAccount(ctx context.Context, db *gorm.DB, target string, excludeUserID int) (int64, error)
	// FindUser 按 target 查询账号（登录场景）。
	FindUser(ctx context.Context, db *gorm.DB, target string) (*model.HrwaiUser, error)
	// Render 生成发送文案（邮箱通道 title 为邮件主题，短信通道忽略 title）。
	Render(purpose CodePurpose, code string, ttl time.Duration) (title, body string)
	// Send 发送验证码。title/body 为 Render 生成的文案（邮件通道使用）；
	// code/ttl 供模板类通道（短信）填充模板参数。
	Send(target, title, body, code string, ttl time.Duration) error
	// ApplyTarget 把 target 写入新注册用户（邮箱注册需生成 phone 占位值）。
	ApplyTarget(user *model.HrwaiUser, target string)
	// BindColumn 绑定/修改时写入的用户表字段（"email" / "phone"）。
	BindColumn() string
}

// AuthCodeStore 验证码存储接口（生产用 Redis，测试用内存实现）。
type AuthCodeStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

// RedisAuthCodeStore 基于全局 Redis 缓存的验证码存储。
type RedisAuthCodeStore struct{}

// Get 读取验证码。
func (RedisAuthCodeStore) Get(ctx context.Context, key string) (string, error) {
	return cache.Get(ctx, key)
}

// Set 写入验证码。
func (RedisAuthCodeStore) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return cache.Set(ctx, key, value, ttl)
}

// Del 删除验证码。
func (RedisAuthCodeStore) Del(ctx context.Context, keys ...string) error {
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
type LogMailSender struct {
	logger *zap.Logger
}

// Send 将邮件内容写入日志。
func (s LogMailSender) Send(to, subject, body string) error {
	s.logger.Info("邮件发送（开发环境降级为日志）", zap.String("to", to), zap.String("subject", subject), zap.String("body", body))
	return nil
}

// SMSProvider 短信发送接口。Send 接收目标手机号、验证码与有效期（分钟），
// 由实现方按已审核模板填充 TemplateParamSet 后发送。
type SMSProvider interface {
	Send(to, code string, minutes int) error
}

// LogSMSProvider 开发环境降级实现：验证码写入服务日志。
type LogSMSProvider struct {
	logger *zap.Logger
}

// Send 将验证码短信写入日志（开发环境降级）。
func (s LogSMSProvider) Send(to, code string, minutes int) error {
	s.logger.Info("短信发送（开发环境降级为日志）", zap.String("to", to), zap.String("code", code), zap.Int("minutes", minutes))
	return nil
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

// phonePattern 中国大陆手机号格式。
var phonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

// NormalizePhone 手机号归一化（去首尾空格）。
func NormalizePhone(phone string) string {
	return strings.TrimSpace(phone)
}

// IsValidPhone 校验中国大陆手机号格式（1 开头 11 位数字）。
func IsValidPhone(phone string) bool {
	return phonePattern.MatchString(strings.TrimSpace(phone))
}

// authCodeValue 验证码存储值（含错误次数限制，最多 5 次）。
// ExpiresAt 为发送时锁定的到期时间：错输重写 Attempts 时按剩余时长续存，
// 错误输入不得延长验证码生命周期（ADR-0012 §5）；零值为旧数据，按旧语义整段重写。
type authCodeValue struct {
	Code      string    `json:"code"`
	Attempts  int       `json:"attempts"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
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

// emailPlaceholderPhone 为邮箱注册账号生成唯一的 phone 占位值（phone 列 NOT NULL）。
func emailPlaceholderPhone(email string) string {
	sum := sha256.Sum256([]byte(email))
	return "email_" + hex.EncodeToString(sum[:])[:32]
}

// =====================================================
// 通道 adapter：邮箱
// =====================================================

// EmailChannel 邮箱验证码通道。
type EmailChannel struct {
	mailer MailSender
	logger *zap.Logger
}

// NewEmailChannel 构造邮箱通道。
// 未配置 SMTP 时：开发环境降级为日志发送验证码，生产环境发送接口返回明确错误。
func NewEmailChannel(smtpCfg config.SMTPConfig, isProd bool, logger *zap.Logger) *EmailChannel {
	var mailer MailSender = LogMailSender{logger: logger}
	if smtpCfg.Host != "" && smtpCfg.From != "" {
		mailer = SMTPMailSender{cfg: smtpCfg}
	} else if isProd {
		mailer = nil
	}
	return &EmailChannel{mailer: mailer, logger: logger}
}

// SenderReady 邮件服务未配置时报错。
func (c *EmailChannel) SenderReady() error {
	if c.mailer == nil {
		return errors.New("邮件服务未配置，请联系管理员")
	}
	return nil
}

// Normalize 归一化邮箱并校验格式。
func (c *EmailChannel) Normalize(target string) (string, error) {
	email := NormalizeEmail(target)
	if !IsValidEmail(email) {
		return "", errors.New("邮箱格式不正确")
	}
	return email, nil
}

// Noun 返回目标中文名。
func (c *EmailChannel) Noun() string { return "邮箱" }

// KeyPrefix 验证码缓存 key 前缀。
func (c *EmailChannel) KeyPrefix() string { return "email_code" }

// FindAccount 统计匹配邮箱的账号数。
func (c *EmailChannel) FindAccount(ctx context.Context, db *gorm.DB, target string, excludeUserID int) (int64, error) {
	var count int64
	query := db.WithContext(ctx).Model(&model.HrwaiUser{}).Where("LOWER(email) = ? AND email <> ''", target)
	if excludeUserID > 0 {
		query = query.Where("id <> ?", excludeUserID)
	}
	err := query.Count(&count).Error
	return count, err
}

// FindUser 按邮箱查询账号。
func (c *EmailChannel) FindUser(ctx context.Context, db *gorm.DB, target string) (*model.HrwaiUser, error) {
	var user model.HrwaiUser
	err := db.WithContext(ctx).Where("LOWER(email) = ?", target).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Render 生成邮件主题与正文。
func (c *EmailChannel) Render(purpose CodePurpose, code string, ttl time.Duration) (string, string) {
	title, op := "【和润天下】注册验证码", "注册"
	switch purpose {
	case CodePurposeLogin:
		title, op = "【和润天下】登录验证码", "登录"
	case CodePurposeBind:
		title, op = "【和润天下】邮箱绑定验证码", "绑定/修改邮箱"
	case CodePurposeAccountChange:
		title, op = "【和润天下】修改登录账号验证码", "修改登录账号"
	case CodePurposeResetPassword:
		title, op = "【和润天下】找回密码验证码", "找回密码"
	}
	body := fmt.Sprintf(
		"您好！\n\n您正在进行%s操作，本次验证码为：%s\n验证码 %d 分钟内有效，请勿泄露给他人。\n\n如非本人操作，请忽略本邮件。",
		op, code, int(ttl.Minutes()),
	)
	return title, body
}

// Send 发送验证码邮件（code/ttl 已由 Render 拼入 body，此处忽略）。
func (c *EmailChannel) Send(target, title, body string, _ string, _ time.Duration) error {
	if err := c.mailer.Send(target, title, body); err != nil {
		c.logger.Error("验证码邮件发送失败", zap.String("email", target), zap.Error(err))
		return errors.New("验证码发送失败，请稍后再试")
	}
	return nil
}

// ApplyTarget 写入邮箱并生成 phone 占位值。
func (c *EmailChannel) ApplyTarget(user *model.HrwaiUser, target string) {
	user.Email = target
	user.Phone = emailPlaceholderPhone(target)
}

// BindColumn 绑定写入 email 字段。
func (c *EmailChannel) BindColumn() string { return "email" }

// =====================================================
// 通道 adapter：短信
// =====================================================

// SmsChannel 手机号验证码通道。
type SmsChannel struct {
	sms    SMSProvider
	logger *zap.Logger
}

// NewSmsChannel 构造短信通道。
// 已配置腾讯云短信时接入真实 provider；未配置时生产报错、开发降级为日志打印验证码。
func NewSmsChannel(smsCfg config.SMSConfig, isProd bool, logger *zap.Logger) *SmsChannel {
	var sms SMSProvider = LogSMSProvider{logger: logger}
	if smsCfg.Configured() {
		sms = NewTencentSMSProvider(smsCfg)
	} else if isProd {
		sms = nil
	}
	return &SmsChannel{sms: sms, logger: logger}
}

// ValidateReady 校验短信签名/模板审核状态（未接入真实 provider 时跳过，返回 nil）。
func (c *SmsChannel) ValidateReady(ctx context.Context) error {
	if p, ok := c.sms.(*TencentSMSProvider); ok {
		return p.ValidateReady(ctx)
	}
	return nil
}

// SenderReady 短信服务未配置时报错。
func (c *SmsChannel) SenderReady() error {
	if c.sms == nil {
		return errors.New("短信服务未配置，请联系管理员")
	}
	return nil
}

// Normalize 归一化手机号并校验格式。
func (c *SmsChannel) Normalize(target string) (string, error) {
	phone := NormalizePhone(target)
	if !IsValidPhone(phone) {
		return "", errors.New("手机号格式不正确")
	}
	return phone, nil
}

// Noun 返回目标中文名。
func (c *SmsChannel) Noun() string { return "手机号" }

// KeyPrefix 验证码缓存 key 前缀。
func (c *SmsChannel) KeyPrefix() string { return "phone_code" }

// FindAccount 统计匹配手机号的账号数。
func (c *SmsChannel) FindAccount(ctx context.Context, db *gorm.DB, target string, excludeUserID int) (int64, error) {
	var count int64
	query := db.WithContext(ctx).Model(&model.HrwaiUser{}).Where("phone = ?", target)
	if excludeUserID > 0 {
		query = query.Where("id <> ?", excludeUserID)
	}
	err := query.Count(&count).Error
	return count, err
}

// FindUser 按手机号查询账号。
func (c *SmsChannel) FindUser(ctx context.Context, db *gorm.DB, target string) (*model.HrwaiUser, error) {
	var user model.HrwaiUser
	err := db.WithContext(ctx).Where("phone = ?", target).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Render 生成短信内容（title 忽略）。
func (c *SmsChannel) Render(purpose CodePurpose, code string, ttl time.Duration) (string, string) {
	op := "注册"
	switch purpose {
	case CodePurposeLogin:
		op = "登录"
	case CodePurposeBind:
		op = "绑定/修改手机号"
	case CodePurposeAccountChange:
		op = "修改登录账号"
	case CodePurposeResetPassword:
		op = "找回密码"
	}
	var content string
	if purpose == CodePurposeBind {
		content = fmt.Sprintf(
			"【和润天下】您正在绑定/修改手机号，验证码为：%s，%d 分钟内有效，请勿泄露给他人。",
			code, int(ttl.Minutes()),
		)
	} else if purpose == CodePurposeAccountChange {
		content = fmt.Sprintf(
			"【和润天下】您正在修改登录账号，验证码为：%s，%d 分钟内有效，请勿泄露给他人。",
			code, int(ttl.Minutes()),
		)
	} else {
		content = fmt.Sprintf(
			"【和润天下】您的%s验证码为：%s，%d 分钟内有效，请勿泄露给他人。",
			op, code, int(ttl.Minutes()),
		)
	}
	return "", content
}

// Send 发送验证码短信（title/body 不使用，短信走已审核模板 + code/ttl 参数）。
func (c *SmsChannel) Send(target string, _, _ string, code string, ttl time.Duration) error {
	if err := c.sms.Send(target, code, int(ttl.Minutes())); err != nil {
		c.logger.Error("验证码短信发送失败", zap.String("phone", target), zap.Error(err))
		return errors.New("验证码发送失败，请稍后再试")
	}
	return nil
}

// ApplyTarget 写入手机号。
func (c *SmsChannel) ApplyTarget(user *model.HrwaiUser, target string) {
	user.Phone = target
}

// BindColumn 绑定写入 phone 字段。
func (c *SmsChannel) BindColumn() string { return "phone" }

// =====================================================
// 验证码 engine：发送 / 校验 / 注册 / 登录 / 绑定
// =====================================================

// VerifyCodeService 验证码服务，邮箱/手机号/绑定共用一套状态机。
type VerifyCodeService struct {
	db      *gorm.DB
	authSvc *AuthService
	store   AuthCodeStore
	codeTTL time.Duration
	logger  *zap.Logger
}

// NewVerifyCodeService 构造验证码服务。
// NewVerifyCodeService 构造验证码 engine。
// store 为验证码存储 adapter（生产 Redis，测试内存），接口存在即接线。
func NewVerifyCodeService(db *gorm.DB, authSvc *AuthService, codeTTL time.Duration, store AuthCodeStore, logger *zap.Logger) *VerifyCodeService {
	if codeTTL <= 0 {
		codeTTL = 5 * time.Minute
	}
	return &VerifyCodeService{
		db:      db,
		authSvc: authSvc,
		store:   store,
		codeTTL: codeTTL,
		logger:  logger,
	}
}

// Send 发送验证码（注册/登录用途）。
func (s *VerifyCodeService) Send(ctx context.Context, ch CodeChannel, purpose CodePurpose, target string) error {
	return s.send(ctx, ch, purpose, target, 0)
}

// SendBind 发送绑定/修改目标字段验证码（唯一性校验排除当前用户）。
func (s *VerifyCodeService) SendBind(ctx context.Context, ch CodeChannel, userID int, target string) error {
	return s.send(ctx, ch, CodePurposeBind, target, userID)
}

// send 发送验证码公共流程：发送器可用 → 格式 → 用途唯一性 → 节流(60s) → 生成存储 → 发送。
func (s *VerifyCodeService) send(ctx context.Context, ch CodeChannel, purpose CodePurpose, target string, excludeUserID int) error {
	if err := ch.SenderReady(); err != nil {
		return err
	}
	target, err := ch.Normalize(target)
	if err != nil {
		return err
	}

	// 注册/绑定：目标必须未被占用；登录：目标必须已注册
	count, err := ch.FindAccount(ctx, s.db, target, excludeUserID)
	if err != nil {
		return err
	}
	switch purpose {
	case CodePurposeRegister:
		if count > 0 {
			return errors.New("该" + ch.Noun() + "已注册，请直接登录")
		}
	case CodePurposeBind:
		if count > 0 {
			return errors.New("该" + ch.Noun() + "已被其他账号使用")
		}
	case CodePurposeLogin:
		if count == 0 {
			return errors.New("该" + ch.Noun() + "尚未注册")
		}
	case CodePurposeResetPassword:
		// 忘记密码：账号必须已存在
		if count == 0 {
			return errors.New("该" + ch.Noun() + "尚未注册")
		}
	case CodePurposeAccountChange:
		// 目标是当前用户自己的手机号，无需占用校验
	default:
		return errors.New("无效的验证码用途")
	}

	// 发送频率限制：同一目标同一用途 60 秒内只能发送一次
	throttleKey := s.codeKey(ch, purpose, target, true)
	if _, err := s.store.Get(ctx, throttleKey); err == nil {
		return errors.New("验证码发送过于频繁，请稍后再试")
	}

	code, err := generateEmailCode()
	if err != nil {
		return errors.New("验证码生成失败，请稍后再试")
	}
	val, _ := json.Marshal(authCodeValue{Code: code, ExpiresAt: time.Now().Add(s.codeTTL)})
	if err := s.store.Set(ctx, s.codeKey(ch, purpose, target, false), string(val), s.codeTTL); err != nil {
		return errors.New("验证码生成失败，请稍后再试")
	}
	if err := s.store.Set(ctx, throttleKey, "1", time.Minute); err != nil {
		return errors.New("验证码生成失败，请稍后再试")
	}

	title, body := ch.Render(purpose, code, s.codeTTL)
	return ch.Send(target, title, body, code, s.codeTTL)
}

// Verify 校验验证码并限制错误次数（最多 5 次），成功后删除验证码。
func (s *VerifyCodeService) Verify(ctx context.Context, ch CodeChannel, purpose CodePurpose, target, code string) error {
	target, err := ch.Normalize(target)
	if err != nil {
		return err
	}
	key := s.codeKey(ch, purpose, target, false)
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
		// 按发送时锁定的到期时间续存（错输不得延长生命周期，ADR-0012 §5）；
		// 旧数据无 ExpiresAt：按旧语义整段重写 TTL
		ttl := s.codeTTL
		if !v.ExpiresAt.IsZero() {
			ttl = time.Until(v.ExpiresAt)
			if ttl <= 0 {
				_ = s.store.Del(ctx, key)
				return errors.New("验证码错误或已过期")
			}
		}
		_ = s.store.Set(ctx, key, string(val), ttl)
		return errors.New("验证码错误或已过期")
	}
	_ = s.store.Del(ctx, key)
	return nil
}

// RegisterWithCode 验证码注册：校验通过后创建账号（昵称 + 账号随机生成 + 可设置密码）并自动登录。
func (s *VerifyCodeService) RegisterWithCode(ctx context.Context, ch CodeChannel, target, code, nickname, company, password string) (*LoginResult, error) {
	target, err := ch.Normalize(target)
	if err != nil {
		return nil, err
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
	if err := s.Verify(ctx, ch, CodePurposeRegister, target, code); err != nil {
		return nil, err
	}

	// 再次校验唯一性（防并发重复注册；数据库唯一索引兜底）
	count, err := ch.FindAccount(ctx, s.db, target, 0)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("该" + ch.Noun() + "已注册，请直接登录")
	}

	account, err := generateRandomAccount()
	if err != nil {
		return nil, errors.New("注册失败，请稍后再试")
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, errors.New("注册失败，请稍后再试")
	}
	user := model.HrwaiUser{
		UID:       NextUID(),
		Account:   account,
		Username:  nickname,
		Password:  hashed,
		Company:   strings.TrimSpace(company),
		Status:    1,
		CreatedAt: beijingNow(),
	}
	ch.ApplyTarget(&user, target)
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, errors.New("注册失败，该" + ch.Noun() + "可能已被注册")
	}

	return s.authSvc.issueLogin(loginCredentials{
		id: user.ID, account: user.Account, username: user.Username, status: &user.Status,
	}, HrwaiRole)
}

// LoginWithCode 验证码登录：校验通过后签发登录令牌。
func (s *VerifyCodeService) LoginWithCode(ctx context.Context, ch CodeChannel, target, code string) (*LoginResult, error) {
	target, err := ch.Normalize(target)
	if err != nil {
		return nil, err
	}
	if err := s.Verify(ctx, ch, CodePurposeLogin, target, code); err != nil {
		return nil, err
	}

	user, err := ch.FindUser(ctx, s.db, target)
	if err != nil {
		return nil, errors.New("该" + ch.Noun() + "尚未注册")
	}
	return s.authSvc.issueLogin(loginCredentials{
		id: user.ID, account: user.Account, username: user.Username, status: &user.Status,
	}, HrwaiRole)
}

// ResetPasswordWithCode 忘记密码：验证码校验通过后重置密码（不自动登录，返回 nil）。
func (s *VerifyCodeService) ResetPasswordWithCode(ctx context.Context, ch CodeChannel, target, code, password string) error {
	target, err := ch.Normalize(target)
	if err != nil {
		return err
	}
	if len(password) < 6 || len(password) > 20 {
		return errors.New("密码长度需为 6-20 位")
	}
	if err := s.Verify(ctx, ch, CodePurposeResetPassword, target, code); err != nil {
		return err
	}
	user, err := ch.FindUser(ctx, s.db, target)
	if err != nil {
		return errors.New("该" + ch.Noun() + "尚未注册")
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return errors.New("密码重置失败，请稍后再试")
	}
	return s.db.WithContext(ctx).Model(&model.HrwaiUser{}).Where("id = ?", user.ID).Update("password", hashed).Error
}

// Bind 校验验证码后绑定/修改当前用户目标字段（格式与唯一性双重校验）。
func (s *VerifyCodeService) Bind(ctx context.Context, ch CodeChannel, userID int, target, code string) error {
	target, err := ch.Normalize(target)
	if err != nil {
		return err
	}
	if err := s.Verify(ctx, ch, CodePurposeBind, target, code); err != nil {
		return err
	}
	count, err := ch.FindAccount(ctx, s.db, target, userID)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该" + ch.Noun() + "已被其他账号使用")
	}
	return s.db.WithContext(ctx).Model(&model.HrwaiUser{}).
		Where("id = ?", userID).Update(ch.BindColumn(), target).Error
}

// SendAccountChange 发送修改登录账号验证码到当前用户已绑定手机号（短信通道）。
func (s *VerifyCodeService) SendAccountChange(ctx context.Context, ch CodeChannel, userID int) error {
	phone, err := s.currentUserPhone(ctx, userID)
	if err != nil {
		return err
	}
	return s.send(ctx, ch, CodePurposeAccountChange, phone, userID)
}

// ChangeAccount 修改当前用户登录账号（短信验证码确认 + 格式/唯一性校验）。
// 成功后重签 JWT（claim 随新账号同步，审计与 /me 口径不再陈旧，ADR-0012 §5）。
func (s *VerifyCodeService) ChangeAccount(ctx context.Context, ch CodeChannel, userID int, newAccount, code string) (*LoginResult, error) {
	newAccount = strings.TrimSpace(newAccount)
	if !IsValidAccount(newAccount) {
		return nil, errors.New("账号需为 4-20 位字母、数字或下划线")
	}
	phone, err := s.currentUserPhone(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := s.Verify(ctx, ch, CodePurposeAccountChange, phone, code); err != nil {
		return nil, err
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.HrwaiUser{}).
		Where("account = ? AND id <> ?", newAccount, userID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("该账号已被占用")
	}
	if err := s.db.WithContext(ctx).Model(&model.HrwaiUser{}).
		Where("id = ?", userID).Update("account", newAccount).Error; err != nil {
		return nil, err
	}
	var user model.HrwaiUser
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, errors.New("用户不存在")
	}
	return s.authSvc.issueLogin(loginCredentials{
		id: user.ID, account: user.Account, username: user.Username, status: &user.Status,
	}, HrwaiRole)
}

// currentUserPhone 读取当前用户手机号并校验格式（未绑定手机号时报错）。
func (s *VerifyCodeService) currentUserPhone(ctx context.Context, userID int) (string, error) {
	var user model.HrwaiUser
	if err := s.db.WithContext(ctx).Select("phone").First(&user, userID).Error; err != nil {
		return "", errors.New("用户不存在")
	}
	// 显式拒绝邮箱注册的占位手机号（email_ 前缀），不依赖 IsValidPhone 巧合兜底
	if strings.HasPrefix(user.Phone, "email_") || !IsValidPhone(user.Phone) {
		return "", errors.New("请先绑定手机号")
	}
	return user.Phone, nil
}

// codeKey 构造验证码缓存 key；throttle=true 时为发送频率限制 key。
// 前缀与旧实现完全一致（email_code / email_code_send），保证线上 Redis 数据兼容。
func (s *VerifyCodeService) codeKey(ch CodeChannel, purpose CodePurpose, target string, throttle bool) string {
	prefix := ch.KeyPrefix()
	if throttle {
		prefix += "_send"
	}
	return cache.SafeKey(prefix, string(purpose), target)
}
