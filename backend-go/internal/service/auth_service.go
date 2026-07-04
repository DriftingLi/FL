// Package service 实现业务服务层。
package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
)

// AuthService 认证服务，处理学员/管理员/导师的登录、注册与令牌签发。
type AuthService struct {
	db              *gorm.DB
	jwtSecret       string
	jwtExpiry       time.Duration
	defaultAdminPwd string
	defaultTutorPwd string
	defaultStudentPwd string
}

// NewAuthService 创建认证服务。
func NewAuthService(db *gorm.DB, jwtSecret string, jwtExpiry time.Duration, adminPwd, tutorPwd, studentPwd string) *AuthService {
	return &AuthService{
		db:               db,
		jwtSecret:        jwtSecret,
		jwtExpiry:        jwtExpiry,
		defaultAdminPwd:  adminPwd,
		defaultTutorPwd:  tutorPwd,
		defaultStudentPwd: studentPwd,
	}
}

// DB 返回底层 *gorm.DB，供 handler 复用查询。
func (s *AuthService) DB() *gorm.DB { return s.db }

// HashPassword 使用 bcrypt 加密密码，与原 Python 版 bcrypt.hashpw 兼容。
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword 校验密码，兼容原 Python 版生成的 bcrypt 哈希。
func VerifyPassword(password, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) == nil
}

// GenerateToken 签发 JWT，claims 结构与原 Python 版一致：user_id/username/role，过期时长由 JWT_EXPIRES_HOURS 配置（默认 24 小时）。
func (s *AuthService) GenerateToken(userID int, username, role string) (string, error) {
	claims := &middleware.Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

// LoginResult 登录返回结构。
type LoginResult struct {
	Token    string `json:"token"`
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Level    string `json:"level,omitempty"`
}

// StudentLogin 学员登录，支持用户名或手机号。
func (s *AuthService) StudentLogin(account, password string) (*LoginResult, error) {
	var student model.Student
	// 同一输入既可能是用户名也可能是手机号，二者择一命中即可
	if err := s.db.Where("username = ? OR phone = ?", account, account).First(&student).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, err
	}
	if !VerifyPassword(password, student.Password) {
		return nil, errors.New("用户名或密码错误")
	}
	if student.Status != 1 {
		return nil, errors.New("账号已被禁用，请联系管理员")
	}
	token, err := s.GenerateToken(student.StudentID, student.Username, "student")
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token:    token,
		UserID:   student.StudentID,
		Username: student.Username,
		Name:     student.Name,
		Role:     "student",
		Level:    student.Level,
	}, nil
}

// StudentRegister 学员注册，username 由手机号自动生成，避免前端再单独填写用户名。
func (s *AuthService) StudentRegister(phone, password, name, email, company string) (map[string]interface{}, error) {
	var count int64
	s.db.Model(&model.Student{}).Where("phone = ?", phone).Count(&count)
	if count > 0 {
		return nil, errors.New("手机号已被注册")
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	student := model.Student{
		Username:  phone,
		Password:  hashed,
		Name:      name,
		Phone:     phone,
		Email:     email,
		Company:   company,
		Status:    1,
		Level:     "beginner",
		CreatedAt: beijingNow(),
	}
	if err := s.db.Create(&student).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"student_id": student.StudentID,
		"username":   student.Username,
		"name":       student.Name,
		"phone":      student.Phone,
	}, nil
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

// TutorRegister 导师注册，对应原 Python 版 tutor_register()。
func (s *AuthService) TutorRegister(username, password, name string) (map[string]interface{}, error) {
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
	return map[string]interface{}{
		"tutor_id": tutor.TutorID,
		"username": tutor.Username,
		"name":     tutor.Name,
	}, nil
}

// EnsureDefaultUsers 确保默认账号存在（admin/tutor/student），密码由环境变量配置。
// 对应原 Python 版 seed_railway.py / init_db.py 的默认账号初始化逻辑。
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
			Status:    1,
			Level:     "beginner",
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
