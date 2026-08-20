// Package service 实现业务服务层。
// 本文件：微信登录。
// - 小程序登录（code2session）：uni.login 临时凭证换 openid → 按 openid 查/建用户 → 签发双令牌。
// - 扫码登录（开放平台）：框架占位，授权信息待接入。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
)

// 微信 code2session 端点与错误码语义（官方文档）。
const (
	wechatCode2SessionURL = "https://api.weixin.qq.com/sns/jscode2session"
	wechatErrBadCode      = 40029 // code 无效
	wechatErrRateLimit    = 45011 // API 分钟级频率限制
	wechatErrBlocked      = 40226 // 高风险用户，登录拦截
)

// WechatAuthService 微信登录服务。
type WechatAuthService struct {
	cfg     config.WechatConfig
	db      *gorm.DB
	authSvc *AuthService
	logger  *zap.Logger

	// code2session 基地址（默认官方端点；测试注入 httptest server 覆盖）
	apiBase string
	httpCli *http.Client
}

// NewWechatAuthService 构造微信服务。
// db 用于按 openid 查/建用户；authSvc 复用登录签发骨架（双令牌 + 禁用校验）。
func NewWechatAuthService(cfg config.WechatConfig, db *gorm.DB, authSvc *AuthService, logger *zap.Logger) *WechatAuthService {
	return &WechatAuthService{
		cfg:     cfg,
		db:      db,
		authSvc: authSvc,
		logger:  logger,
		apiBase: wechatCode2SessionURL,
		httpCli: &http.Client{Timeout: 10 * time.Second},
	}
}

// WxLoginResult 小程序登录结果。
// 契约（《微信小程序登录-文档说明.md》）：token/user_id/username/name/role/avatar/is_new 平铺结构；
// 在其之上补双令牌字段（refresh_token，ADR-0016）与 account，与密码/验证码登录同构。
type WxLoginResult struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int    `json:"user_id"`
	Account      string `json:"account"`
	Username     string `json:"username"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Avatar       string `json:"avatar"`
	IsNew        bool   `json:"isNew"`
}

// wxSessionResponse code2session 响应（errcode=0 时 openid/session_key 有效）。
type wxSessionResponse struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// MiniProgramLogin 微信小程序登录：code2session 换 openid → 按 openid 查用户；
// 未注册则自动建账号并绑定 openid；签发双令牌返回（含 is_new 标记）。
func (s *WechatAuthService) MiniProgramLogin(ctx context.Context, code string) (*WxLoginResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("缺少微信登录凭证 code")
	}
	if s.cfg.AppID == "" || s.cfg.AppSecret == "" {
		return nil, errors.New("微信登录未配置，请联系管理员")
	}

	session, err := s.code2Session(ctx, code)
	if err != nil {
		return nil, err
	}

	user, isNew, err := s.findOrCreateByOpenID(session.OpenID, session.UnionID)
	if err != nil {
		return nil, err
	}

	login, err := s.authSvc.issueLogin(loginCredentials{
		id: user.ID, account: user.Account, username: user.Username, status: &user.Status,
	}, HrwaiRole)
	if err != nil {
		return nil, err
	}
	return &WxLoginResult{
		Token:        login.Token,
		RefreshToken: login.RefreshToken,
		UserID:       login.UserID,
		Account:      login.Account,
		Username:     login.Username,
		Name:         login.Username,
		Role:         login.Role,
		Avatar:       user.AvatarURL,
		IsNew:        isNew,
	}, nil
}

// code2Session 调用微信登录凭证校验接口，换取 openid/unionid。
func (s *WechatAuthService) code2Session(ctx context.Context, code string) (*wxSessionResponse, error) {
	q := url.Values{}
	q.Set("appid", s.cfg.AppID)
	q.Set("secret", s.cfg.AppSecret)
	q.Set("js_code", code)
	q.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiBase+"?"+q.Encode(), nil)
	if err != nil {
		return nil, errors.New("微信登录请求构建失败")
	}
	resp, err := s.httpCli.Do(req)
	if err != nil {
		s.logger.Warn("code2session 调用失败", zap.Error(err))
		return nil, errors.New("微信登录服务暂不可用，请稍后再试")
	}
	defer resp.Body.Close()

	var body wxSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		s.logger.Warn("code2session 响应解析失败", zap.Error(err))
		return nil, errors.New("微信登录服务响应异常，请稍后再试")
	}
	if body.ErrCode != 0 {
		s.logger.Warn("code2session 业务失败",
			zap.Int("errcode", body.ErrCode), zap.String("errmsg", body.ErrMsg))
		switch body.ErrCode {
		case wechatErrBadCode:
			return nil, errors.New("微信登录凭证已失效，请重新登录")
		case wechatErrRateLimit:
			return nil, errors.New("操作过于频繁，请稍后再试")
		case wechatErrBlocked:
			return nil, errors.New("当前账号存在风险，登录被拦截")
		default:
			return nil, fmt.Errorf("微信登录失败（错误码 %d），请稍后再试", body.ErrCode)
		}
	}
	if body.OpenID == "" {
		return nil, errors.New("微信登录响应缺少 openid，请稍后再试")
	}
	return &body, nil
}

// findOrCreateByOpenID 按 openid 查用户；未注册则自动建账号并绑定。
// account/username 由 openid 派生（openid 全局唯一，派生值天然唯一）。
// 并发首登竞争由 wechat_openid 唯一索引兜底：撞唯一约束时按已存在用户处理。
func (s *WechatAuthService) findOrCreateByOpenID(openID, unionID string) (*model.HrwaiUser, bool, error) {
	var user model.HrwaiUser
	err := s.db.Where("wechat_openid = ?", openID).First(&user).Error
	if err == nil {
		return &user, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, errors.New("登录失败，请稍后再试")
	}

	// openid 截段：account 取前 12 位、昵称取后 6 位（同源不同段，避免与账号撞名）。
	suffix := openID
	if len(suffix) > 12 {
		suffix = suffix[:12]
	}
	tail := openID
	if len(tail) > 6 {
		tail = tail[len(tail)-6:]
	}
	newUser := model.HrwaiUser{
		UID:           NextUID(),
		Account:       "wx_" + suffix,
		Username:      "微信学员" + tail,
		WechatOpenID:  openID,
		WechatUnionID: unionID,
		Status:        1,
		CreatedAt:     beijingNow(),
	}
	if err := s.db.Create(&newUser).Error; err != nil {
		// 并发首登：唯一索引冲突 → 回查既有用户
		var again model.HrwaiUser
		if qErr := s.db.Where("wechat_openid = ?", openID).First(&again).Error; qErr == nil {
			return &again, false, nil
		}
		s.logger.Warn("微信自动注册失败", zap.Error(err))
		return nil, false, errors.New("微信登录注册失败，请稍后再试")
	}
	return &newUser, true, nil
}

// QRCodeInfo 返回扫码登录占位信息：未配置授权时 enabled=false，前端展示占位二维码。
func (s *WechatAuthService) QRCodeInfo() map[string]any {
	if s.cfg.AppID == "" || s.cfg.AppSecret == "" {
		return map[string]any{
			"enabled": false,
			"qr_url":  "",
			"message": "微信授权暂未配置，请等待开放平台配置完成后使用",
		}
	}
	return map[string]any{
		"enabled": true,
		"qr_url":  "",
		"message": "微信扫码登录待接入（二维码生成接口占位）",
	}
}

// LoginWithQRCode 微信扫码登录占位：真实授权流程待接入（与小程序 code2session 登录不同流）。
func (s *WechatAuthService) LoginWithQRCode(code string) (*LoginResult, error) {
	return nil, errors.New("微信扫码登录尚未接入，请使用其他登录方式")
}
