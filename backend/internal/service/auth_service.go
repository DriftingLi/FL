// Package service 实现业务服务层。
package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"forklift-training/internal/clock"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
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
	case "tutor":
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
	Name                 *string                   `json:"name,omitempty"`
	AvatarURL            *string                   `json:"avatar_url,omitempty"`
	Company              *string                   `json:"company,omitempty"`
	Email                *string                   `json:"email,omitempty"`
	HasPassword          *bool                     `json:"has_password,omitempty"`
	PendingProfileChange **ProfileChangeRequestDTO `json:"pending_profile_change,omitempty"`
	Phone                *string                   `json:"phone,omitempty"`
	Role                 string                    `json:"role"`
	UID                  *string                   `json:"uid,omitempty"`
	UserID               int                       `json:"user_id"`
	Username             *string                   `json:"username,omitempty"`
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

// UpdatePassword 设置/修改当前用户密码（账号密码登录用）。
func (s *AuthService) UpdatePassword(userID int, password string) error {
	if len(password) < 6 || len(password) > 20 {
		return errors.New("密码长度需为 6-20 位")
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.db.Model(&model.HrwaiUser{}).Where("id = ?", userID).Update("password", hashed).Error
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
			return nil, errors.New("导师账号或密码错误")
		}
		return nil, err
	}
	status := tutor.Status
	return s.verifyAndIssue(password, loginCredentials{
		id: tutor.TutorID, account: tutor.Username, username: tutor.Username,
		password: tutor.Password, status: &status,
	}, "tutor", "导师账号或密码错误")
}

// TutorRegister 导师注册。
func (s *AuthService) TutorRegister(username, password, name string) (map[string]any, error) {
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
	return map[string]any{
		"tutor_id": tutor.TutorID,
		"username": tutor.Username,
		"name":     tutor.Name,
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
	return nil
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
func (s *AuthService) ToggleRecruiterStatus(id int) (int16, error) {
	var r model.RecruiterUser
	if err := s.db.First(&r, id).Error; err != nil {
		return 0, errors.New("招聘者不存在")
	}
	next := int16(1)
	if r.Status == 1 {
		next = 0
	}
	if err := s.db.Model(&model.RecruiterUser{}).Where("id = ?", id).Update("status", next).Error; err != nil {
		return 0, err
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
	Status        int16     `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// RecruiterListResult 招聘者分页列表。
type RecruiterListResult struct {
	Total int64               `json:"total"`
	Page  int                 `json:"page"`
	Items []RecruiterListItem `json:"items"`
}

// ListRecruiters 招聘者列表（分页 + 关键字过滤企业名/账号；#416 真实现替换硬编码空数组桩）。
// 响应只含白名单字段（无 Password 等凭据）。
func (s *AuthService) ListRecruiters(page, pageSize int, keyword string) (*RecruiterListResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	q := s.db.Model(&model.RecruiterUser{})
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR company_name LIKE ?", kw, kw)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []model.RecruiterUser
	if err := q.Order("created_at DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
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
	}); err != nil {
		return nil, err
	}
	var r model.RecruiterUser
	if err := s.db.First(&r, id).Error; err != nil {
		return nil, errors.New("招聘者不存在")
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

// ResetRecruiterPassword 重置招聘者口令（#417）：旧口令立即失效，响应不回显任何口令字段。
func (s *AuthService) ResetRecruiterPassword(id int, password string) error {
	if len(password) < 6 || len(password) > 20 {
		return errors.New("密码长度需为 6-20 位")
	}
	var cnt int64
	s.db.Model(&model.RecruiterUser{}).Where("id = ?", id).Count(&cnt)
	if cnt == 0 {
		return errors.New("招聘者不存在")
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.db.Model(&model.RecruiterUser{}).Where("id = ?", id).Update("password", hashed).Error
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

// DeleteAccount 硬删除学员账号并级联清理相关数据，论坛内容匿名化。
func (s *AuthService) DeleteAccount(userID int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var user model.HrwaiUser
		if err := tx.First(&user, userID).Error; err != nil {
			return errors.New("用户不存在")
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
				return err
			}
		}
		// 论坛内容匿名化：重分配给占位用户，避免 CASCADE 删除
		if err := tx.Model(&model.ForumTopic{}).Where("user_id = ?", userID).Update("user_id", sentinel.ID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ForumReply{}).Where("user_id = ?", userID).Update("user_id", sentinel.ID).Error; err != nil {
			return err
		}
		// 显式清理无外键或需额外处理的关联数据（有 CASCADE 的表亦显式删除以确保无残留）
		tx.Where("user_id = ?", userID).Delete(&model.Favorite{})
		// 点赞回扣（spec #297）：先按主题/回复聚合该用户点赞行数，同事务删除后按行数
		// 回扣对应计数列——DELETE 行数与受影响主题集合一一对应，注销不再污染计数。
		if err := s.refundLikesOnDelete(tx, userID); err != nil {
			return err
		}
		tx.Where("reporter_id = ?", userID).Delete(&model.ForumReport{})
		// 有 CASCADE 的表显式删除以兼容测试内存库
		tx.Where("student_id = ?", userID).Delete(&model.QuestionPracticeRecord{})
		tx.Where("student_id = ?", userID).Delete(&model.WrongQuestion{})
		tx.Where("student_id = ?", userID).Delete(&model.MockExam{})
		tx.Where("student_id = ?", userID).Delete(&model.PracticeProgress{})
		tx.Where("student_id = ?", userID).Delete(&model.StudyRecord{})
		tx.Where("user_id = ?", userID).Delete(&model.ForumCheckIn{})
		tx.Where("user_id = ?", userID).Delete(&model.Notification{})
		tx.Where("user_id = ?", userID).Delete(&model.ProfileChangeRequest{})
		tx.Where("user_id = ?", userID).Delete(&model.AIChatSession{})
		tx.Where("user_id = ?", userID).Delete(&model.AIUserModel{})
		tx.Where("user_id = ?", userID).Delete(&model.QuestionComment{})
		tx.Where("user_id = ?", userID).Delete(&model.QuestionNote{})
		tx.Where("user_id = ?", userID).Delete(&model.JobCard{})
		tx.Where("student_user_id = ?", userID).Delete(&model.ContactRequest{})
		// #452：注销时投递一并失效（投递产生的授权随 ContactRequest 已级联/显式删除）
		tx.Where("student_user_id = ?", userID).Delete(&model.JobApplication{})
		// 删除用户本体（剩余 CASCADE 关联自动清理）
		if err := tx.Delete(&model.HrwaiUser{}, userID).Error; err != nil {
			return err
		}
		return nil
	})
}

// refundLikesOnDelete 注销事务内的点赞回扣：先查后删（先按主题/回复聚合该用户点赞行数，
// 再 DELETE），同事务内经 ForumCounter 按行数回扣对应计数列，保证 DELETE 行数与
// 受影响主题/回复集合一一对应（spec #297）。
func (s *AuthService) refundLikesOnDelete(tx *gorm.DB, userID int) error {
	var topicLikes []struct {
		TopicID int64
		Cnt     int
	}
	if err := tx.Model(&model.ForumTopicLike{}).
		Select("topic_id, COUNT(*) AS cnt").
		Where("user_id = ?", userID).
		Group("topic_id").
		Scan(&topicLikes).Error; err != nil {
		return err
	}
	if err := tx.Where("user_id = ?", userID).Delete(&model.ForumTopicLike{}).Error; err != nil {
		return err
	}
	for _, agg := range topicLikes {
		if err := s.forumCnt.AdjustLikes(tx, agg.TopicID, -agg.Cnt); err != nil {
			return err
		}
	}

	var replyLikes []struct {
		ReplyID int64
		Cnt     int
	}
	if err := tx.Model(&model.ForumReplyLike{}).
		Select("reply_id, COUNT(*) AS cnt").
		Where("user_id = ?", userID).
		Group("reply_id").
		Scan(&replyLikes).Error; err != nil {
		return err
	}
	if err := tx.Where("user_id = ?", userID).Delete(&model.ForumReplyLike{}).Error; err != nil {
		return err
	}
	for _, agg := range replyLikes {
		if err := s.forumCnt.AdjustReplyLikes(tx, agg.ReplyID, -agg.Cnt); err != nil {
			return err
		}
	}
	return nil
}

// beijingNow 返回当前北京时间。时区政策已单点归位 internal/clock 包（spec #296），此函数仅作遗留调用方的一行委托。
func beijingNow() time.Time { return clock.Now() }
