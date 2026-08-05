// Package service 实现残值评估模块的业务服务层。
// 本文件:估值模块认证服务(已并入主体系 AuthService,本文件保留作为薄包装降低下游改动量)。
package service

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
	vmain "forklift-training/internal/service"
)

// ValuationRole 估值模块角色(已统一为 HrwaiRole)。
// deprecated: 请改用 service.HrwaiRole
const ValuationRole = vmain.HrwaiRole

// ValuationLoginResult 估值登录返回结构(与主体系 LoginResult 等价)。
type ValuationLoginResult = vmain.LoginResult

// ValuationAuthService 估值模块认证服务(薄包装,内部代理到主体系 AuthService)。
// 保留此类型以减少下游 handler 改动量,后续可逐步删除。
type ValuationAuthService struct {
	main *vmain.AuthService
}

// WrapValuationAuthService 用已存在的主体系 AuthService 包装为薄包装。
func WrapValuationAuthService(main *vmain.AuthService) *ValuationAuthService {
	return &ValuationAuthService{main: main}
}

// Main 返回底层主体系 AuthService,供需要直接调用主体系的场景使用。
func (s *ValuationAuthService) Main() *vmain.AuthService { return s.main }

// DB 返回底层 *gorm.DB,供 handler 复用查询。
func (s *ValuationAuthService) DB() *gorm.DB { return s.main.DB() }

// Login 估值登录(代理到主体系 HrwaiLogin)。
func (s *ValuationAuthService) Login(account, password string) (*ValuationLoginResult, error) {
	return s.main.HrwaiLogin(account, password)
}

// Register 估值注册(代理到主体系 HrwaiRegister)。
func (s *ValuationAuthService) Register(phone, password, name, email, company string) (map[string]any, error) {
	return s.main.HrwaiRegister(phone, password, name, email, company)
}

// GetByID 用于 /me 接口查询用户信息。
func (s *ValuationAuthService) GetByID(id int) (*model.HrwaiUser, error) {
	return s.main.GetHrwaiUserByID(id)
}

// GenerateToken 签发 JWT(代理到主体系,role 固定 HrwaiRole)。
func (s *ValuationAuthService) GenerateToken(userID int, username string) (string, error) {
	return s.main.GenerateToken(userID, username, vmain.HrwaiRole)
}

// =====================================================
// 管理员用户管理已统一到主体系 /api/admin/hrwai-users/*,
// 由 AdminService 直接操作 hrwai_users 表,本模块不再维护用户管理方法。
// =====================================================
