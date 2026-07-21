// Package service 实现残值评估模块的业务服务层。
// 本文件：估值模块独立认证服务（与培训 AuthService 完全隔离）。
package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// ValuationRole 估值模块固定角色。
const ValuationRole = "valuation_user"

// ValuationClaims 估值模块独立 JWT 声明。
// 使用独立 secret 签发，与主体系 middleware.Claims 互不兼容。
type ValuationClaims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"` // 固定 "valuation_user"
	jwt.RegisteredClaims
}

// ValuationAuthService 估值模块独立认证服务。
// 与主 AuthService 隔离：独立用户表、独立 JWT secret、独立 claims。
type ValuationAuthService struct {
	db        *gorm.DB
	jwtSecret string
	jwtExpiry time.Duration
}

// NewValuationAuthService 创建估值认证服务。
func NewValuationAuthService(db *gorm.DB, jwtSecret string, jwtExpiry time.Duration) *ValuationAuthService {
	return &ValuationAuthService{db: db, jwtSecret: jwtSecret, jwtExpiry: jwtExpiry}
}

// ValuationLoginResult 估值登录返回结构。
type ValuationLoginResult struct {
	Token    string `json:"token"`
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Role     string `json:"role"` // 固定 "valuation_user"
}

// HashValuationPassword 使用 bcrypt 加密密码。
// 与主 service.HashPassword 等价，但放在本包内避免跨包依赖。
func HashValuationPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyValuationPassword 校验密码。
func VerifyValuationPassword(password, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) == nil
}

// GenerateToken 签发估值专属 JWT。
func (s *ValuationAuthService) GenerateToken(userID int, username string) (string, error) {
	claims := &ValuationClaims{
		UserID:   userID,
		Username: username,
		Role:     ValuationRole,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

// Login 支持用户名或手机号登录。
func (s *ValuationAuthService) Login(account, password string) (*ValuationLoginResult, error) {
	var user model.ValuationUser
	// 同一输入既可能是用户名也可能是手机号，二者择一命中即可
	if err := s.db.Where("username = ? OR phone = ?", account, account).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, err
	}
	if !VerifyValuationPassword(password, user.Password) {
		return nil, errors.New("用户名或密码错误")
	}
	if user.Status != 1 {
		return nil, errors.New("账号已被禁用，请联系管理员")
	}
	token, err := s.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, err
	}
	return &ValuationLoginResult{
		Token:    token,
		UserID:   user.ID,
		Username: user.Username,
		Name:     user.Name,
		Role:     ValuationRole,
	}, nil
}

// Register username 由手机号自动生成，避免前端再单独填写用户名。
func (s *ValuationAuthService) Register(phone, password, name, email, company string) (map[string]any, error) {
	var count int64
	s.db.Model(&model.ValuationUser{}).Where("phone = ?", phone).Count(&count)
	if count > 0 {
		return nil, errors.New("手机号已被注册")
	}
	hashed, err := HashValuationPassword(password)
	if err != nil {
		return nil, err
	}
	user := model.ValuationUser{
		Username:  phone,
		Password:  hashed,
		Name:      name,
		Phone:     phone,
		Email:     email,
		Company:   company,
		Status:    1,
		CreatedAt: beijingTimeNow(),
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"name":     user.Name,
		"phone":    user.Phone,
	}, nil
}

// GetByID 用于 /me 接口查询用户信息。
func (s *ValuationAuthService) GetByID(id int) (*model.ValuationUser, error) {
	var user model.ValuationUser
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// beijingTimeNow 返回当前北京时间。
func beijingTimeNow() time.Time {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	return time.Now().In(loc)
}
