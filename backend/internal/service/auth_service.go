// Package service 实现业务服务层。
package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
)

// AuthService 认证服务，处理学员/管理员/导师的登录、注册与令牌签发。
type AuthService struct {
	db        *gorm.DB
	session   *security.Session
	reviewSvc *ProfileReviewService

	defaultAdminPwd   string
	defaultTutorPwd   string
	defaultStudentPwd string

	logger *zap.Logger
}

// NewAuthService 创建认证服务。sess 为装配根创建的唯一会话实例（签发/校验同实例）。
func NewAuthService(db *gorm.DB, sess *security.Session, adminPwd, tutorPwd, studentPwd string, logger *zap.Logger) *AuthService {
	return &AuthService{
		db:                db,
		session:           sess,
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

// MaskedPhone 隐藏邮箱注册的占位手机号（email_ 前缀），/auth/me 源头过滤不下发客户端。
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

// GenerateToken 签发 access token（委托会话模块，双令牌会话 ADR-0012）。
func (s *AuthService) GenerateToken(userID int, account, role string) (string, error) {
	return s.session.Issue(userID, account, role)
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

// beijingNow 返回当前北京时间。
func beijingNow() time.Time {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	return time.Now().In(loc)
}
