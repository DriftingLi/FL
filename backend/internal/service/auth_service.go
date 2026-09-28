// Package service 实现业务服务层。
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"forklift-training/internal/clock"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/pkg/paging"
)

// 招聘者账号域业务错误哨兵（ADR-0024 / spec #449 决定 4）：handler 以 errors.Is 映射状态码，不做字符串比对。
var (
	// ErrCreditCodeTaken 统一社会信用代码已被占用（同一企业只能有一个招聘者账号，账号即企业）。
	ErrCreditCodeTaken = errors.New("该企业已存在招聘者账号")
	// ErrUsernameTaken 登录账号已被占用。
	ErrUsernameTaken = errors.New("用户名已被注册")
)

// recruiterUsernameRe 招聘者用户名格式：4-20 位字母/数字/下划线（与登录表单 usernameRules 一致）。
var recruiterUsernameRe = regexp.MustCompile("^[a-zA-Z0-9_]{4,20}$")

// AuthService 认证服务，处理学员/管理员/导师的登录、注册与令牌签发。
type AuthService struct {
	db        *gorm.DB
	session   *security.Session
	reviewSvc *ProfileReviewService
	forumCnt  ForumCounter // 论坛计数唯一写入口（注销回扣点赞数用，spec #297）

	defaultAdminPwd   string
	defaultTutorPwd   string
	defaultStudentPwd string

	logger *zap.Logger
}

// NewAuthService 创建认证服务。sess 为装配根创建的唯一会话实例（签发/校验同实例）；
// forumCnt 与 ForumService 共享同一计数器实例（构造注入，注销同事务回扣 likes_count）。
func NewAuthService(db *gorm.DB, sess *security.Session, forumCnt ForumCounter, adminPwd, tutorPwd, studentPwd string, logger *zap.Logger) *AuthService {
	return &AuthService{
		db:                db,
		session:           sess,
		forumCnt:          forumCnt,
		defaultAdminPwd:   adminPwd,
		defaultTutorPwd:   tutorPwd,
		defaultStudentPwd: studentPwd,
		logger:            logger,
	}
}

// SetProfileReviewService 注入资料审核服务（GetProfile 组装待审资料状态用）。
func (s *AuthService) SetProfileReviewService(rs *ProfileReviewService) { s.reviewSvc = rs }

// GetProfile 组装 /auth/me 返回的用户资料（按角色查询对应账号表）。
// 响应字段为前端约定（auth store 依赖 user_id/account/role/uid/username、
// 学员资料字段与 has_password / pending_profile_change），保持稳定。
func (s *AuthService) GetProfile(userID int, role, account string) *ProfileDTO {
	dto := &ProfileDTO{
		UserID:  userID,
		Account: account,
		Role:    role,
	}
	switch role {
	case HrwaiRole:
		var u model.HrwaiUser
		if err := s.db.First(&u, userID).Error; err == nil {
			dto.Account = u.Account
			dto.UID = ptr(FormatUID(u.UID))
			dto.Username = ptr(u.Username)
			dto.AvatarURL = ptr(u.AvatarURL)
			dto.Phone = ptr(MaskedPhone(u.Phone))
			dto.Email = ptr(u.Email)
			dto.Company = ptr(u.Company)
			// 是否已设置密码（决定个人资料页"账号密码"卡片提示文案）
			dto.HasPassword = ptr(u.Password != "")
		}
		// 待审核的资料修改（昵称/头像），供前端展示"审核中"状态。
		// GetPendingForUser 无待审时返回 nil -> 序列化为 null（键存在）；出错时键缺失。
		if pending, err := s.reviewSvc.GetPendingForUser(userID); err == nil {
			dto.PendingProfileChange = &pending
		}
	case TutorRole:
		var t model.Tutor
		if err := s.db.First(&t, userID).Error; err == nil {
			dto.Name = ptr(t.Name)
			dto.Username = ptr(t.Username)
		}
	case "admin":
		var a model.Admin
		if err := s.db.First(&a, userID).Error; err == nil {
			dto.Name = ptr(a.Name)
			dto.Username = ptr(a.Username)
		}
	case RecruiterRole:
		var r model.RecruiterUser
		if err := s.db.First(&r, userID).Error; err == nil {
			dto.Account = r.Username
			dto.Username = ptr(r.Username)
			dto.Name = ptr(r.ContactName)
			dto.Company = ptr(r.CompanyName)
			dto.Email = ptr(r.ContactEmail)
			dto.Phone = ptr(r.ContactPhone)
		}
	}
	return dto
}

// ProfileDTO /auth/me 响应体（形状由契约测试 auth_me_contract_test.go 字节级锁定，
// 勿改 json tag 与字段声明顺序）。指针字段 + omitempty 表达「键存在/键缺失」两态；
// PendingProfileChange 用双指针表达三态：nil=键缺失、&nil=输出 null、&对象=输出对象。
type ProfileDTO struct {
	Account              string                    `json:"account"`
	Name                 *string                   `json:"name,omitempty" extensions:"x-optional"`
	AvatarURL            *string                   `json:"avatar_url,omitempty" extensions:"x-optional"`
	Company              *string                   `json:"company,omitempty" extensions:"x-optional"`
	Email                *string                   `json:"email,omitempty" extensions:"x-optional"`
	HasPassword          *bool                     `json:"has_password,omitempty" extensions:"x-optional"`
	PendingProfileChange **ProfileChangeRequestDTO `json:"pending_profile_change,omitempty" extensions:"x-optional,x-nullable"`
	Phone                *string                   `json:"phone,omitempty" extensions:"x-optional"`
	Role                 string                    `json:"role"`
	UID                  *string                   `json:"uid,omitempty" extensions:"x-optional"`
	UserID               int                       `json:"user_id"`
	Username             *string                   `json:"username,omitempty" extensions:"x-optional"`
}

// ptr 构造 T 的指针（ProfileDTO 指针字段表达键缺失/存在两态）。
func ptr[T any](v T) *T { return &v }

// MaskedPhone 隐藏占位手机号（邮箱注册 email_ / 微信建号 wxp_ / 注销哨兵 deleted__sentinel，
// IsPlaceholderPhone 单点判定），/auth/me 源头过滤不下发客户端——修复微信建号用户
// /auth/me 泄漏 wxp_ 串的问题。
func MaskedPhone(phone string) string {
	if IsPlaceholderPhone(phone) {
		return ""
	}
	return phone
}

// HashPassword 使用 bcrypt 加密密码。
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword 校验密码。
func VerifyPassword(password, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) == nil
}

// RefreshResultDTO 双令牌轮换的响应 {"refresh_token": "...", "token": "..."}。
// 字段声明按 JSON key 字母序 —— 与改造前 raw handler 里 map[string]string 的序列化字节序一致（#959 auth 域收口）。
type RefreshResultDTO struct {
	RefreshToken string `json:"refresh_token"`
	Token        string `json:"token"`
}

// LoginResult 登录返回结构（双令牌：access token + refresh token）。
// 旧字段 token 保留，前端向后兼容；refresh_token 仅前端本地存储，不写入 Cookie。
type LoginResult struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int    `json:"user_id"`
	Account      string `json:"account"`
	Username     string `json:"username"`
	Role         string `json:"role"`
}

// HrwaiRole 统一 HRWAI 账号角色名(替代原 "student" 和 "valuation_user")。
const HrwaiRole = "hrwai_user"

// RecruiterRole 企业招聘者角色名（第四角色，独立表 recruiter_users，邀约制）。
const RecruiterRole = "recruiter"

// TutorRole 讲师角色名。**与 HrwaiRole/RecruiterRole 同住一处**（ADR-0064 判据）：这个字符串
// 同时是 JWT 的角色 claim 与全会话吊销的命名空间键片段，此前只以字面量散在登录分派
// （auth_service.go:94 / :307），吊销侧一用就得再抄一遍——同一个事实的两个住处。
// 注意与 authz.RoleTutor 不是一回事：那一层是能力角色名，这一层是凭证命名空间。
const TutorRole = "tutor"

// ErrRecruiterNotFound 「招聘者账号不存在」这一事实的唯一载体（ADR-0064 决策 1/2）。
// 与吊销命名空间 RecruiterRole 同处一地，api 侧据此把它与「查不动」分档。
var ErrRecruiterNotFound = errors.New("招聘者不存在")

// loginCredentials 登录骨架按角色差异点：查表结果（密码/禁用语义）。
// status 为 nil 表示该角色无禁用语义（admin 表无 status 字段）。
type loginCredentials struct {
	id       int
	account  string
	username string
	password string
	status   *int16
}

// verifyAndIssue 登录共享骨架：验密 → 禁用校验 → 签发 → 组结果。
// plainPassword 为用户输入的明文，errMessage 为验密失败的统一文案（防账号枚举）。
func (s *AuthService) verifyAndIssue(plainPassword string, c loginCredentials, role, errMessage string) (*LoginResult, error) {
	if !VerifyPassword(plainPassword, c.password) {
		return nil, errors.New(errMessage)
	}
	return s.issueLogin(c, role)
}

// issueLogin 登录骨架后半段：禁用校验 → 签发 → 组结果。
// 密码三入口与验证码登录/注册共用（ADR-0011 向验证码路径的延伸，ADR-0012 §5）。
func (s *AuthService) issueLogin(c loginCredentials, role string) (*LoginResult, error) {
	if c.status != nil && *c.status != 1 {
		return nil, errors.New("账号已被禁用，请联系管理员")
	}
	access, refresh, err := s.session.IssuePair(c.id, c.account, role)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token:        access,
		RefreshToken: refresh,
		UserID:       c.id,
		Account:      c.account,
		Username:     c.username,
		Role:         role,
	}, nil
}

// HrwaiLogin 统一 HRWAI 账号登录,支持账号或手机号。
// 三套前端(培训学员端 / 残值评估 / AI 助手)共用此登录方法。
func (s *AuthService) HrwaiLogin(account, password string) (*LoginResult, error) {
	var user model.HrwaiUser
	// 同一输入既可能是登录账号也可能是手机号，二者择一命中即可
	if err := s.db.Where("account = ? OR phone = ?", account, account).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("账号或密码错误")
		}
		return nil, err
	}
	status := user.Status
	return s.verifyAndIssue(password, loginCredentials{
		id: user.ID, account: user.Account, username: user.Username,
		password: user.Password, status: &status,
	}, HrwaiRole, "账号或密码错误")
}

// generateRandomAccount 生成随机登录账号（如 hr1a2b3c4d5e6f78）。
func generateRandomAccount() (string, error) {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "hr" + hex.EncodeToString(b), nil
}

// GetHrwaiUserByID 用于 /me 接口查询用户信息。
func (s *AuthService) GetHrwaiUserByID(id int) (*model.HrwaiUser, error) {
	var user model.HrwaiUser
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdatePassword 登录态改密入口（账号密码登录用）：口令落库与全会话吊销都交给
// SetNewPassword（ADR-0062 票7 两条口令写面合一），本方法只声明自己这一族的失败策略。
func (s *AuthService) UpdatePassword(ctx context.Context, userID int, password string) error {
	res := s.SetNewPassword(ctx, userID, password)
	if !res.Applied() {
		return res.Err
	}
	// 尽力而为族：快捷登录的静默续登在改密后立即失效，回退密码登录。
	// 吊销标记写失败不阻断改密（密码已生效、不可回退），记日志暴露缺口。
	if res.RevokeErr != nil {
		s.logger.Warn("改密后 refresh 吊销标记写入失败", zap.Int("user_id", userID), zap.Error(res.RevokeErr))
	}
	return nil
}

// AdminLogin 管理员登录（admin 表无 status 字段，无禁用语义）。
func (s *AuthService) AdminLogin(username, password string) (*LoginResult, error) {
	var admin model.Admin
	if err := s.db.Where("username = ?", username).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("管理员账号或密码错误")
		}
		return nil, err
	}
	return s.verifyAndIssue(password, loginCredentials{
		id: admin.AdminID, account: admin.Username, username: admin.Username,
		password: admin.Password,
	}, "admin", "管理员账号或密码错误")
}

// TutorLogin 导师登录。
func (s *AuthService) TutorLogin(username, password string) (*LoginResult, error) {
	var tutor model.Tutor
	if err := s.db.Where("username = ?", username).First(&tutor).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("讲师账号或密码错误")
		}
		return nil, err
	}
	status := tutor.Status
	return s.verifyAndIssue(password, loginCredentials{
		id: tutor.TutorID, account: tutor.Username, username: tutor.Username,
		password: tutor.Password, status: &status,
	}, TutorRole, "讲师账号或密码错误")
}

// TutorRegisterResultDTO 导师建号结果（ADR-0009 §2 typed DTO / spec #940 片三）。
//
// 字段按 JSON key 字母序声明（name / tutor_id / username）：旧形态是 map[string]any，
// encoding/json 对 map 按 key 排序输出 —— 字母序保证换成 struct 后字节序不变。
type TutorRegisterResultDTO struct {
	Name     string `json:"name"`
	TutorID  int    `json:"tutor_id"`
	Username string `json:"username"`
}

// TutorRegister 导师注册。
func (s *AuthService) TutorRegister(username, password, name string) (*TutorRegisterResultDTO, error) {
	var count int64
	s.db.Model(&model.Tutor{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		return nil, errors.New("用户名已被注册")
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	tutor := model.Tutor{
		Username:  username,
		Password:  hashed,
		Name:      name,
		Status:    1,
		CreatedAt: beijingNow(),
	}
	if err := s.db.Create(&tutor).Error; err != nil {
		return nil, err
	}
	return &TutorRegisterResultDTO{
		Name:     tutor.Name,
		TutorID:  tutor.TutorID,
		Username: tutor.Username,
	}, nil
}

// RecruiterLogin 企业招聘者登录（第四角色，邀约制独立表）。
func (s *AuthService) RecruiterLogin(username, password string) (*LoginResult, error) {
	var r model.RecruiterUser
	if err := s.db.Where("username = ?", username).First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("招聘者账号或密码错误")
		}
		return nil, err
	}
	status := r.Status
	return s.verifyAndIssue(password, loginCredentials{
		id: r.ID, account: r.Username, username: r.Username,
		password: r.Password, status: &status,
	}, RecruiterRole, "招聘者账号或密码错误")
}

// RecruiterCreateInput 管理员创建招聘者账号的输入（企业信息全部必填）。
type RecruiterCreateInput struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	CompanyName   string `json:"company_name"`
	CreditCode    string `json:"credit_code"`
	BusinessScope string `json:"business_scope"`
	ContactName   string `json:"contact_name"`
	ContactPhone  string `json:"contact_phone"`
	ContactEmail  string `json:"contact_email"`
	// Wechat 企业微信（#487：可空，管理员录入）
	Wechat string `json:"wechat"`
}

// ValidateRecruiterInput 校验企业信息字段全部必填（缺任一项 400）。
func ValidateRecruiterInput(in RecruiterCreateInput) error {
	username := strings.TrimSpace(in.Username)
	if username == "" {
		return errors.New("账号不能为空")
	}
	// 用户名格式：4-20 位字母/数字/下划线（与登录表单 usernameRules 一致，问题6）
	if !recruiterUsernameRe.MatchString(username) {
		return errors.New("账号只能包含字母、数字和下划线，长度 4-20 位")
	}
	if strings.TrimSpace(in.Password) == "" {
		return errors.New("密码不能为空")
	}
	if len(in.Password) < 6 || len(in.Password) > 20 {
		return errors.New("密码长度需为 6-20 位")
	}
	if strings.TrimSpace(in.CompanyName) == "" {
		return errors.New("企业名称不能为空")
	}
	if strings.TrimSpace(in.CreditCode) == "" {
		return errors.New("统一社会信用代码不能为空")
	}
	if strings.TrimSpace(in.BusinessScope) == "" {
		return errors.New("主营不能为空")
	}
	if strings.TrimSpace(in.ContactName) == "" {
		return errors.New("对外联系人姓名不能为空")
	}
	if strings.TrimSpace(in.ContactPhone) == "" {
		return errors.New("联系电话不能为空")
	}
	if strings.TrimSpace(in.ContactEmail) == "" {
		return errors.New("联系邮箱不能为空")
	}
	if len([]rune(strings.TrimSpace(in.Wechat))) > 100 {
		return errors.New("微信号过长（最多 100 字符）")
	}
	return nil
}

// RecruiterCreatedDTO 招聘者账号创建（201）的响应形状；password 不在内。
// 字段声明按 JSON key 字母序 —— 与改造前 map[string]any 的序列化字节序一致（#954 片二）。
type RecruiterCreatedDTO struct {
	BusinessScope string `json:"business_scope"`
	CompanyName   string `json:"company_name"`
	ContactEmail  string `json:"contact_email"`
	ContactName   string `json:"contact_name"`
	ContactPhone  string `json:"contact_phone"`
	CreditCode    string `json:"credit_code"`
	ID            int    `json:"id"`
	Status        int16  `json:"status"`
	Username      string `json:"username"`
	Wechat        string `json:"wechat"`
}

// RecruiterUpdatedDTO 招聘者编辑（200）的响应形状：与创建**同一个投影少一个 status**。
// 这个差异是现状（改造前两处 map 就不一致），本片按「字节不变」保留，是否统一由 admin 片决定。
type RecruiterUpdatedDTO struct {
	BusinessScope string `json:"business_scope"`
	CompanyName   string `json:"company_name"`
	ContactEmail  string `json:"contact_email"`
	ContactName   string `json:"contact_name"`
	ContactPhone  string `json:"contact_phone"`
	CreditCode    string `json:"credit_code"`
	ID            int    `json:"id"`
	Username      string `json:"username"`
	Wechat        string `json:"wechat"`
}

// NewRecruiterCreatedDTO / NewRecruiterUpdatedDTO 把招聘者模型投影为对外形状。
// 投影折叠进 DTO 构造（ADR-0009 §2）：两个 handler 不再各抄一遍字段，
// 也把「Edit 比 Create 少一个 status」这条差异摆在同一个地方（是否统一交 admin 片）。
func NewRecruiterCreatedDTO(rec *model.RecruiterUser) RecruiterCreatedDTO {
	return RecruiterCreatedDTO{
		BusinessScope: rec.BusinessScope,
		CompanyName:   rec.CompanyName,
		ContactEmail:  rec.ContactEmail,
		ContactName:   rec.ContactName,
		ContactPhone:  rec.ContactPhone,
		CreditCode:    rec.CreditCode,
		ID:            rec.ID,
		Status:        rec.Status,
		Username:      rec.Username,
		Wechat:        rec.Wechat,
	}
}

func NewRecruiterUpdatedDTO(rec *model.RecruiterUser) RecruiterUpdatedDTO {
	return RecruiterUpdatedDTO{
		BusinessScope: rec.BusinessScope,
		CompanyName:   rec.CompanyName,
		ContactEmail:  rec.ContactEmail,
		ContactName:   rec.ContactName,
		ContactPhone:  rec.ContactPhone,
		CreditCode:    rec.CreditCode,
		ID:            rec.ID,
		Username:      rec.Username,
		Wechat:        rec.Wechat,
	}
}

// CreateRecruiter 管理员创建招聘者账号（邀约制，企业字段全部必填）。
func (s *AuthService) CreateRecruiter(in RecruiterCreateInput) (*model.RecruiterUser, error) {
	if err := ValidateRecruiterInput(in); err != nil {
		return nil, err
	}
	in.Username = strings.TrimSpace(in.Username)
	in.CompanyName = strings.TrimSpace(in.CompanyName)
	in.CreditCode = strings.TrimSpace(in.CreditCode)
	in.BusinessScope = strings.TrimSpace(in.BusinessScope)
	in.ContactName = strings.TrimSpace(in.ContactName)
	in.ContactPhone = strings.TrimSpace(in.ContactPhone)
	in.ContactEmail = strings.TrimSpace(in.ContactEmail)
	in.Wechat = strings.TrimSpace(in.Wechat)
	var count int64
	s.db.Model(&model.RecruiterUser{}).Where("username = ?", in.Username).Count(&count)
	if count > 0 {
		return nil, errors.New("用户名已被注册")
	}
	// #450：同一企业 1:1 —— 统一社会信用代码唯一，一个信用代码开不出第二个号。
	var creditCnt int64
	s.db.Model(&model.RecruiterUser{}).Where("credit_code = ?", in.CreditCode).Count(&creditCnt)
	if creditCnt > 0 {
		return nil, ErrCreditCodeTaken
	}
	hashed, err := HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	rec := model.RecruiterUser{
		Username:      in.Username,
		Password:      hashed,
		CompanyName:   in.CompanyName,
		CreditCode:    in.CreditCode,
		BusinessScope: in.BusinessScope,
		ContactName:   in.ContactName,
		ContactPhone:  in.ContactPhone,
		ContactEmail:  in.ContactEmail,
		Wechat:        in.Wechat,
		Status:        1,
		CreatedAt:     beijingNow(),
		UpdatedAt:     beijingNow(),
	}
	if err := s.db.Create(&rec).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

// ToggleRecruiterStatus 切换招聘者启用/禁用状态（禁用后登录被 verifyAndIssue 拦截）。
// 禁用同时吊销该身份全部 refresh（ADR-0060 票2，spec #1201 场景 29）：只改状态列
// 不构成「停用真实生效」——手上仍持 refresh 的会话能继续换新 access。与改密同族，
// 状态列已生效故吊销失败只记日志、不回退。
func (s *AuthService) ToggleRecruiterStatus(ctx context.Context, id int) (int16, error) {
	var r model.RecruiterUser
	if err := s.db.First(&r, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrRecruiterNotFound
		}
		return 0, err
	}
	next := int16(1)
	if r.Status == 1 {
		next = 0
	}
	if err := s.db.Model(&model.RecruiterUser{}).Where("id = ?", id).Update("status", next).Error; err != nil {
		return 0, err
	}
	if next == 0 {
		if err := s.session.RevokeIdentity(ctx, RecruiterRole, id); err != nil {
			s.logger.Warn("招聘员禁用后 refresh 吊销标记写入失败", zap.Int("recruiter_id", id), zap.Error(err))
		}
	}
	return next, nil
}

// RecruiterListItem 招聘者列表项（管理面白名单：不含任何凭据字段，口令哈希永不出管理面）。
type RecruiterListItem struct {
	ID            int       `json:"id"`
	Username      string    `json:"username"`
	CompanyName   string    `json:"company_name"`
	CreditCode    string    `json:"credit_code"`
	BusinessScope string    `json:"business_scope"`
	ContactName   string    `json:"contact_name"`
	ContactPhone  string    `json:"contact_phone"`
	ContactEmail  string    `json:"contact_email"`
	Wechat        string    `json:"wechat"`
	Status        int16     `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// RecruiterListResult 招聘者分页列表。
type RecruiterListResult struct {
	Total int64               `json:"total"`
	Page  int                 `json:"page"`
	Items []RecruiterListItem `json:"items" nullability:"nonnil"`
}

// ListRecruiters 招聘者列表（分页 + 关键字过滤企业名/账号；#416 真实现替换硬编码空数组桩）。
// 响应只含白名单字段（无 Password 等凭据）。
func (s *AuthService) ListRecruiters(page, pageSize int, keyword string) (*RecruiterListResult, error) {
	rows, total, page, _, err := paging.QueryWithMax[model.RecruiterUser](s.db, page, pageSize, 20, 100,
		"created_at DESC", func(q *gorm.DB) *gorm.DB {
			if keyword != "" {
				kw := "%" + keyword + "%"
				q = q.Where("username LIKE ? OR company_name LIKE ?", kw, kw)
			}
			return q
		})
	if err != nil {
		return nil, err
	}
	items := make([]RecruiterListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, RecruiterListItem{
			ID:            r.ID,
			Username:      r.Username,
			CompanyName:   r.CompanyName,
			CreditCode:    r.CreditCode,
			BusinessScope: r.BusinessScope,
			ContactName:   r.ContactName,
			ContactPhone:  r.ContactPhone,
			ContactEmail:  r.ContactEmail,
			Wechat:        r.Wechat,
			Status:        r.Status,
			CreatedAt:     r.CreatedAt,
		})
	}
	return &RecruiterListResult{Total: total, Page: page, Items: items}, nil
}

// RecruiterEditInput 编辑招聘者企业信息的输入（#417）：不涉及账号归属与角色，密码走独立端点。
type RecruiterEditInput struct {
	Username      string `json:"username"`
	CompanyName   string `json:"company_name"`
	CreditCode    string `json:"credit_code"`
	BusinessScope string `json:"business_scope"`
	ContactName   string `json:"contact_name"`
	ContactPhone  string `json:"contact_phone"`
	ContactEmail  string `json:"contact_email"`
	// Wechat 企业微信（#487：可空，管理员编辑时录入）
	Wechat string `json:"wechat"`
}

// EditRecruiter 编辑招聘者企业信息与联系人（#417）：与创建同源校验（必填判定单点），
// 不允许改动账号归属与角色；启停仍走 ToggleRecruiterStatus 独立端点。
func (s *AuthService) EditRecruiter(id int, in RecruiterEditInput) (*model.RecruiterUser, error) {
	// 必填校验复用 ValidateRecruiterInput 的字段集（账号/密码位忽略，企业字段逐条同源）
	if err := ValidateRecruiterInput(RecruiterCreateInput{
		Username:      "keep",
		Password:      "keep123",
		CompanyName:   in.CompanyName,
		CreditCode:    in.CreditCode,
		BusinessScope: in.BusinessScope,
		ContactName:   in.ContactName,
		ContactPhone:  in.ContactPhone,
		ContactEmail:  in.ContactEmail,
		Wechat:        in.Wechat,
	}); err != nil {
		return nil, err
	}
	var r model.RecruiterUser
	if err := s.db.First(&r, id).Error; err != nil {
		return nil, ErrRecruiterNotFound
	}
	// #450：编辑把信用代码改成别家已占用的值 → 同样被拒（自己保持原值不算占用）。
	credit := strings.TrimSpace(in.CreditCode)
	if credit != r.CreditCode {
		var creditCnt int64
		s.db.Model(&model.RecruiterUser{}).Where("credit_code = ? AND id <> ?", credit, id).Count(&creditCnt)
		if creditCnt > 0 {
			return nil, ErrCreditCodeTaken
		}
	}
	// 问题6：管理员可修改登录用户名（格式 + 唯一校验；空串 = 不改）。
	newUsername := strings.TrimSpace(in.Username)
	if newUsername != "" && newUsername != r.Username {
		if !recruiterUsernameRe.MatchString(newUsername) {
			return nil, errors.New("账号只能包含字母、数字和下划线，长度 4-20 位")
		}
		var nameCnt int64
		s.db.Model(&model.RecruiterUser{}).Where("username = ? AND id <> ?", newUsername, id).Count(&nameCnt)
		if nameCnt > 0 {
			return nil, ErrUsernameTaken
		}
	}
	updates := map[string]any{
		"company_name":   strings.TrimSpace(in.CompanyName),
		"credit_code":    strings.TrimSpace(in.CreditCode),
		"business_scope": strings.TrimSpace(in.BusinessScope),
		"contact_name":   strings.TrimSpace(in.ContactName),
		"wechat":         strings.TrimSpace(in.Wechat),
		"contact_phone":  strings.TrimSpace(in.ContactPhone),
		"contact_email":  strings.TrimSpace(in.ContactEmail),
		"updated_at":     beijingNow(),
	}
	if newUsername != "" && newUsername != r.Username {
		updates["username"] = newUsername
	}
	if err := s.db.Model(&model.RecruiterUser{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.db.First(&r, id).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

// RecruiterPasswordResetResult 重置密码的响应体：改造前是空 map（信封里 data 为 {}），
// 空结构体保形 —— 序列化仍是 {} 而不是 null（#954 片二：字节不变）。
type RecruiterPasswordResetResult struct{}

// ResetRecruiterPassword 重置招聘者口令（#417）：旧口令立即失效，响应不回显任何口令字段。
// 招聘者写面在 recruiter 命名空间里自建（SetNewPassword 落的是 hrwai_users），但长度规则
// 与吊销族策略同源：validatePasswordLength + 落库后尽力而为吊销。
func (s *AuthService) ResetRecruiterPassword(ctx context.Context, id int, password string) error {
	res := applyNewPassword(ctx, s.db, s.session, recruiterPasswordSubject, id, password)
	if !res.Applied() {
		return res.Err
	}
	if res.RevokeErr != nil {
		s.logger.Warn("招聘员口令重置后 refresh 吊销标记写入失败", zap.Int("recruiter_id", id), zap.Error(res.RevokeErr))
	}
	return nil
}

// EnsureDefaultUsers 确保默认账号存在（admin/tutor/student），密码由环境变量配置。
// 已存在的账号会被跳过（不会重置密码）。
func (s *AuthService) EnsureDefaultUsers() error {
	// 1. 默认管理员 admin
	var adminCount int64
	if err := s.db.Model(&model.Admin{}).Where("username = ?", "admin").Count(&adminCount).Error; err != nil {
		return err
	}
	if adminCount == 0 {
		hashed, err := HashPassword(s.defaultAdminPwd)
		if err != nil {
			return err
		}
		admin := model.Admin{
			Username:  "admin",
			Password:  hashed,
			Name:      "系统管理员",
			CreatedAt: beijingNow(),
		}
		if err := s.db.Create(&admin).Error; err != nil {
			return err
		}
	}

	// 2. 默认导师 tutor
	var tutorCount int64
	if err := s.db.Model(&model.Tutor{}).Where("username = ?", "tutor").Count(&tutorCount).Error; err != nil {
		return err
	}
	if tutorCount == 0 {
		hashed, err := HashPassword(s.defaultTutorPwd)
		if err != nil {
			return err
		}
		tutor := model.Tutor{
			Username:  "tutor",
			Password:  hashed,
			Name:      "导师",
			Status:    1,
			CreatedAt: beijingNow(),
		}
		if err := s.db.Create(&tutor).Error; err != nil {
			return err
		}
	}

	// 3. 默认学员 student（hrwai_users）
	var studentCount int64
	if err := s.db.Model(&model.HrwaiUser{}).Where("account = ?", "student").Count(&studentCount).Error; err != nil {
		return err
	}
	if studentCount == 0 {
		hashed, err := HashPassword(s.defaultStudentPwd)
		if err != nil {
			return err
		}
		student := model.HrwaiUser{
			UID:       NextUID(),
			Account:   "student",
			Username:  "测试学员",
			Password:  hashed,
			Phone:     "13800000000",
			Status:    1,
			CreatedAt: beijingNow(),
		}
		if err := s.db.Create(&student).Error; err != nil {
			return err
		}
	}

	return nil
}

// UpdateCompany 更新学员单位信息，立即生效不走审核。
func (s *AuthService) UpdateCompany(userID int, company string) error {
	if len(company) > 50 {
		return errors.New("单位名称不能超过 50 个字符")
	}
	return s.db.Model(&model.HrwaiUser{}).Where("id = ?", userID).Update("company", company).Error
}

// accountCleanupStep 注销清理表的一行：一张按「条件列 = 本人 userID」归属的依赖表。
type accountCleanupStep struct {
	// table 表名，只用于报错与日志指向；真正决定删哪张表的是 dest（GORM 由模型推表名），
	// 所以这里写错不会改变行为——锁见 TestDeleteAccount_清理表是删除序列的唯一事实源。
	table string
	// column 该表指向 hrwai_users.id 的条件列。
	column string
	// dest 每次执行现造一个零值模型指针：GORM 需要它推表名与主键，复用一个实例会被条件污染。
	dest func() any
	// likesTarget 非空 ⇒ 这一行不是「纯删除」：点赞行必须在同一事务里按行数回扣对应计数列
	// （spec #297），本字段就是聚合与回扣的目标列（topic_id / reply_id）。
	likesTarget string
}

// accountCleanupSteps 注销清理表——**唯一事实源**（spec #1345 决策 10 / 真实缺陷 #6）。
//
// 新增一张依赖表 = 只在这里加一行；删除序列（runAccountCleanup 的循环）不动，也不许在别处
// 再抄一份表名清单。顺序沿用旧实现：论坛内容先匿名化（在循环之前，见 DeleteAccount）、
// 主行最后删（剩余 CASCADE 由库兜）。有 CASCADE 的表亦显式删除，以兼容测试内存库并确保无残留。
var accountCleanupSteps = []accountCleanupStep{
	{table: "favorite", column: "user_id", dest: func() any { return &model.Favorite{} }},
	// 点赞两张表带计数回扣（spec #297），执行形态见 deleteLikesWithRefund。
	{table: "forum_topic_like", column: "user_id", dest: func() any { return &model.ForumTopicLike{} }, likesTarget: "topic_id"},
	{table: "forum_reply_like", column: "user_id", dest: func() any { return &model.ForumReplyLike{} }, likesTarget: "reply_id"},
	{table: "forum_report", column: "reporter_id", dest: func() any { return &model.ForumReport{} }},
	{table: "question_practice_record", column: "student_id", dest: func() any { return &model.QuestionPracticeRecord{} }},
	{table: "wrong_question", column: "student_id", dest: func() any { return &model.WrongQuestion{} }},
	{table: "mock_exam", column: "student_id", dest: func() any { return &model.MockExam{} }},
	{table: "practice_progress", column: "student_id", dest: func() any { return &model.PracticeProgress{} }},
	{table: "study_record", column: "student_id", dest: func() any { return &model.StudyRecord{} }},
	{table: "forum_checkin", column: "user_id", dest: func() any { return &model.ForumCheckIn{} }},
	{table: "notifications", column: "user_id", dest: func() any { return &model.Notification{} }},
	{table: "profile_change_requests", column: "user_id", dest: func() any { return &model.ProfileChangeRequest{} }},
	{table: "ai_chat_sessions", column: "user_id", dest: func() any { return &model.AIChatSession{} }},
	{table: "ai_user_models", column: "user_id", dest: func() any { return &model.AIUserModel{} }},
	{table: "question_comment", column: "user_id", dest: func() any { return &model.QuestionComment{} }},
	{table: "note", column: "user_id", dest: func() any { return &model.Note{} }},
	{table: "job_cards", column: "user_id", dest: func() any { return &model.JobCard{} }},
	{table: "contact_requests", column: "student_user_id", dest: func() any { return &model.ContactRequest{} }},
	// #452：注销时投递一并失效（投递产生的授权随 contact_requests 已级联/显式删除）
	{table: "job_applications", column: "student_user_id", dest: func() any { return &model.JobApplication{} }},
}

// DeleteAccount 硬删除学员账号并级联清理相关数据，论坛内容匿名化。
//
// 全会话吊销不在此处：注销走「先 RevokeIdentity、标记失败即不调本方法」，
// 由 handler 承担（会话终止两族归 security.Session，资料层删除归本方法）。ADR-0060 票2。
//
// 两段自证（spec #1345 决策 10）：事务内一次（失败即整笔回滚）、提交后再一次（失败即报错）。
// 旧形态里逐条手写的删除**从不读 .Error**，PG 上一旦某条语句把事务打进 aborted 态、后续语句与
// 提交全部空转，接口却照样回 200「注销成功」而数据仍在（真实缺陷 #6）。
func (s *AuthService) DeleteAccount(userID int) error {
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var user model.HrwaiUser
		if err := tx.First(&user, userID).Error; err != nil {
			return ErrHrwaiUserNotFound
		}
		// 确保匿名占位用户存在
		var sentinel model.HrwaiUser
		if err := tx.Where("account = ?", "__deleted_user").First(&sentinel).Error; err != nil {
			sentinel = model.HrwaiUser{
				UID:       NextUID(),
				Account:   "__deleted_user",
				Username:  "已注销用户",
				Password:  "",
				Phone:     "deleted__sentinel",
				Status:    0,
				CreatedAt: beijingNow(),
			}
			if err := tx.Create(&sentinel).Error; err != nil {
				return fmt.Errorf("注销建匿名占位用户失败（表 hrwai_users）: %w", err)
			}
		}
		// 论坛内容匿名化：重分配给占位用户，避免 CASCADE 删除。
		// 这两条是 UPDATE 不是 DELETE，故不在清理表里（表只管「按归属清掉的依赖表」）。
		if err := tx.Model(&model.ForumTopic{}).Where("user_id = ?", userID).Update("user_id", sentinel.ID).Error; err != nil {
			return fmt.Errorf("注销匿名化论坛帖子失败（表 forum_topics）: %w", err)
		}
		if err := tx.Model(&model.ForumReply{}).Where("user_id = ?", userID).Update("user_id", sentinel.ID).Error; err != nil {
			return fmt.Errorf("注销匿名化论坛回复失败（表 forum_replies）: %w", err)
		}
		if err := s.runAccountCleanup(tx, userID); err != nil {
			return err
		}
		// 删除主行（剩余 CASCADE 关联由库兜）
		if err := tx.Delete(&model.HrwaiUser{}, userID).Error; err != nil {
			return fmt.Errorf("注销删除主行失败（表 hrwai_users，id=%d）: %w", userID, err)
		}
		// 自证第一遍（事务内）：主行仍在就判失败 ⇒ 整笔回滚，绝不留「一半删了、一半没删」。
		return s.proveAccountGone(tx, userID)
	}); err != nil {
		return err
	}
	// 自证第二遍（提交后、换一条连接读）：兜住「事务没真提交、函数却回了 nil」这一类静默失败
	// ——缺陷 #6 的原始形态正是它。此时已无法回滚，只能明确报错，让接口非 2xx。
	return s.proveAccountGone(s.db, userID)
}

// runAccountCleanup 单点执行清理表：逐条删除、逐条判错。
// 判错统一在这里做（旧实现逐条手写、错误各判各的，13 条删除里只有 2 条读了 .Error）。
func (s *AuthService) runAccountCleanup(tx *gorm.DB, userID int) error {
	for i := range accountCleanupSteps {
		step := accountCleanupSteps[i]
		var err error
		if step.likesTarget != "" {
			err = s.deleteLikesWithRefund(tx, userID, step)
		} else {
			err = tx.Where(step.column+" = ?", userID).Delete(step.dest()).Error
		}
		if err != nil {
			return fmt.Errorf("注销清理第 %d/%d 步失败（表 %s，条件列 %s）: %w",
				i+1, len(accountCleanupSteps), step.table, step.column, err)
		}
	}
	return nil
}

// proveAccountGone 自证主行已不存在。exec 由调用方给（事务内传 tx、提交后传 s.db）。
func (s *AuthService) proveAccountGone(exec *gorm.DB, userID int) error {
	var left int64
	if err := exec.Model(&model.HrwaiUser{}).Where("id = ?", userID).Count(&left).Error; err != nil {
		return fmt.Errorf("注销自证读不出用户 %d 的主行状态（表 hrwai_users）: %w", userID, err)
	}
	if left != 0 {
		return fmt.Errorf("注销未生效：用户 %d 的主行仍在（清理序列没有真的删掉它，表 hrwai_users）", userID)
	}
	return nil
}

// deleteLikesWithRefund 执行带计数回扣的那两行（forum_topic_like / forum_reply_like）：
// 先聚合（删之前才知道回扣多少）、再删、最后同事务回扣目标计数列，保证删除行数与
// 受影响主题/回复集合一一对应（spec #297）。列名全部取自行声明，不另抄清单。
func (s *AuthService) deleteLikesWithRefund(tx *gorm.DB, userID int, step accountCleanupStep) error {
	var agg []struct {
		TargetID int64
		Cnt      int
	}
	if err := tx.Model(step.dest()).
		Select(step.likesTarget+" AS target_id, COUNT(*) AS cnt").
		Where(step.column+" = ?", userID).
		Group(step.likesTarget).
		Scan(&agg).Error; err != nil {
		return err
	}
	if err := tx.Where(step.column+" = ?", userID).Delete(step.dest()).Error; err != nil {
		return err
	}
	for _, a := range agg {
		if err := s.applyLikesRefund(tx, step, a.TargetID, a.Cnt); err != nil {
			return err
		}
	}
	return nil
}

// applyLikesRefund 按行声明的回扣目标列选计数出口（新增点赞表时的唯一分派点）。
func (s *AuthService) applyLikesRefund(tx *gorm.DB, step accountCleanupStep, targetID int64, count int) error {
	switch step.likesTarget {
	case "topic_id":
		return s.forumCnt.AdjustLikes(tx, targetID, -count)
	case "reply_id":
		return s.forumCnt.AdjustReplyLikes(tx, targetID, -count)
	default:
		return fmt.Errorf("注销点赞回扣：表 %s 声明了未知目标列 %q", step.table, step.likesTarget)
	}
}

// beijingNow 返回当前北京时间。时区政策已单点归位 internal/clock 包（spec #296），此函数仅作遗留调用方的一行委托。
func beijingNow() time.Time { return clock.Now() }
