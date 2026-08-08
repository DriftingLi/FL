// Package service 实现业务服务层。
package service

import (
	"context"
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
	db                *gorm.DB
	session           *security.Session
	defaultAdminPwd   string
	defaultTutorPwd   string
	defaultStudentPwd string

	logger *zap.Logger
}

// NewAuthService 创建认证服务。
func NewAuthService(db *gorm.DB, jwtSecret string, jwtExpiry time.Duration, adminPwd, tutorPwd, studentPwd string, logger *zap.Logger) *AuthService {
	return &AuthService{
		db:                db,
		session:           security.NewSession(jwtSecret, jwtExpiry, security.CookieConfig{}),
		defaultAdminPwd:   adminPwd,
		defaultTutorPwd:   tutorPwd,
		defaultStudentPwd: studentPwd,
		logger:            logger,
	}
}

// DB 返回底层 *gorm.DB，供 handler 复用查询。
func (s *AuthService) DB() *gorm.DB { return s.db }

// Session 返回会话模块（供 handler 消费统一接口）。
func (s *AuthService) Session() *security.Session { return s.session }

// RevokeToken 吊销会话（登出），委托会话模块写入黑名单。
func (s *AuthService) RevokeToken(ctx context.Context, tokenStr string) error {
	return s.session.Revoke(ctx, tokenStr)
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

// GenerateToken 签发 JWT（委托会话模块，claims 结构：user_id/username/role）。
func (s *AuthService) GenerateToken(userID int, username, role string) (string, error) {
	return s.session.Issue(userID, username, role)
}

// LoginResult 登录返回结构。
type LoginResult struct {
	Token    string `json:"token"`
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

// HrwaiRole 统一 HRWAI 账号角色名(替代原 "student" 和 "valuation_user")。
const HrwaiRole = "hrwai_user"

// HrwaiLogin 统一 HRWAI 账号登录,支持用户名或手机号。
// 三套前端(培训学员端 / 残值评估 / AI 助手)共用此登录方法。
func (s *AuthService) HrwaiLogin(account, password string) (*LoginResult, error) {
	var user model.HrwaiUser
	// 同一输入既可能是用户名也可能是手机号，二者择一命中即可
	if err := s.db.Where("username = ? OR phone = ?", account, account).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, err
	}
	if !VerifyPassword(password, user.Password) {
		return nil, errors.New("用户名或密码错误")
	}
	if user.Status != 1 {
		return nil, errors.New("账号已被禁用，请联系管理员")
	}
	token, err := s.GenerateToken(user.ID, user.Username, HrwaiRole)
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

// HrwaiRegister 统一 HRWAI 账号注册（手机号+密码，兼容旧流程）。
// 账号（username）注册时随机生成，与手机号/邮箱概念解耦。
func (s *AuthService) HrwaiRegister(phone, password, name, email, company string) (map[string]any, error) {
	var count int64
	s.db.Model(&model.HrwaiUser{}).Where("phone = ?", phone).Count(&count)
	if count > 0 {
		return nil, errors.New("手机号已被注册")
	}
	// 手机号注册时若填写邮箱：校验格式与唯一性（同一邮箱不能注册多个账户）
	email = NormalizeEmail(email)
	if email != "" {
		if !IsValidEmail(email) {
			return nil, errors.New("邮箱格式不正确")
		}
		var emailCount int64
		if err := s.db.Model(&model.HrwaiUser{}).
			Where("LOWER(email) = ?", email).Count(&emailCount).Error; err != nil {
			return nil, err
		}
		if emailCount > 0 {
			return nil, errors.New("该邮箱已被注册")
		}
	}
	username, err := generateRandomUsername()
	if err != nil {
		return nil, errors.New("注册失败，请稍后再试")
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := model.HrwaiUser{
		Username:  username,
		Password:  hashed,
		Name:      name,
		Nickname:  generateDefaultNickname(s.db),
		Phone:     phone,
		Email:     email,
		Company:   company,
		Status:    1,
		CreatedAt: beijingNow(),
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"phone":    user.Phone,
		"name":     user.Name,
	}, nil
}

// generateRandomUsername 生成随机账号（如 hr1a2b3c4d5e6f78）。
func generateRandomUsername() (string, error) {
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

// AdminLogin 管理员登录。
func (s *AuthService) AdminLogin(username, password string) (*LoginResult, error) {
	var admin model.Admin
	if err := s.db.Where("username = ?", username).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("管理员账号或密码错误")
		}
		return nil, err
	}
	if !VerifyPassword(password, admin.Password) {
		return nil, errors.New("管理员账号或密码错误")
	}
	token, err := s.GenerateToken(admin.AdminID, admin.Username, "admin")
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token:    token,
		UserID:   admin.AdminID,
		Username: admin.Username,
		Name:     admin.Name,
		Role:     "admin",
	}, nil
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
	if !VerifyPassword(password, tutor.Password) {
		return nil, errors.New("导师账号或密码错误")
	}
	if tutor.Status != 1 {
		return nil, errors.New("账号已被禁用，请联系管理员")
	}
	token, err := s.GenerateToken(tutor.TutorID, tutor.Username, "tutor")
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token:    token,
		UserID:   tutor.TutorID,
		Username: tutor.Username,
		Name:     tutor.Name,
		Role:     "tutor",
	}, nil
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

	// 3. 默认学员 student
	var studentCount int64
	if err := s.db.Model(&model.Student{}).Where("username = ?", "student").Count(&studentCount).Error; err != nil {
		return err
	}
	if studentCount == 0 {
		hashed, err := HashPassword(s.defaultStudentPwd)
		if err != nil {
			return err
		}
		student := model.Student{
			Username:  "student",
			Password:  hashed,
			Name:      "测试学员",
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
