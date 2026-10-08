// 本文件：App 端微信登录（#1482）——开放平台「移动应用」凭证的 code 换取链路。
// 与同包 wechat_service.go 的小程序链路（jscode2session）是两种应用类型、两对凭证、两个端点，
// 严格不共用凭证配置，也不共用端点常量。
package auth

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
	"forklift-training/internal/core"
	"forklift-training/internal/model"
)

// 移动应用 code 换取端点与错误码语义（一手依据：微信开放平台《移动应用 / 微信登录 / 开发流程》）。
const (
	wechatAppAccessTokenURL = "https://api.weixin.qq.com/sns/oauth2/access_token"
	// appErrBadCode 同页示例：{"errcode":40029,"errmsg":"invalid code"}。
	appErrBadCode = 40029
	// appErrNotLaunchedQuota 同页原文：「已认证主体的未上架应用的微信登录用户次数限制为 100 次/天」，
	// 超限以错误码 10060 返回。它不是「稍后再试」能解决的，所以给独立文案。
	appErrNotLaunchedQuota = 10060
)

// WechatAppService App 端微信登录服务（开放平台移动应用凭证）。
type WechatAppService struct {
	cfg     config.WechatAppConfig
	db      *gorm.DB
	authSvc *Service
	logger  *zap.Logger

	// accessTokenBase 是 code 换取端点（默认官方地址；测试注入 httptest server 覆盖）。
	// 单独一个字段而不是复用 WechatService.apiBase：两条链路的端点不同源，合在一个字段上
	// 就等于允许「拿小程序凭证去调移动应用端点」这种必然失败的接法。
	accessTokenBase string
	httpCli         *http.Client
}

// NewWechatAppService 构造 App 端微信登录服务。
// db 用于按 unionid/openid 定位与自动建号；authSvc 复用登录签发骨架（双令牌 + 禁用校验）。
func NewWechatAppService(cfg config.WechatAppConfig, db *gorm.DB, authSvc *Service, logger *zap.Logger) *WechatAppService {
	return &WechatAppService{
		cfg:             cfg,
		db:              db,
		authSvc:         authSvc,
		logger:          logger,
		accessTokenBase: wechatAppAccessTokenURL,
		httpCli:         &http.Client{Timeout: 10 * time.Second},
	}
}

// appAccessTokenResponse /sns/oauth2/access_token 的响应体。
// 官方返回：access_token / expires_in / refresh_token / openid / scope / unionid；
// unionid「当且仅当该移动应用已获得该用户的 userinfo 授权时」才出现，缺席即按独立账号处理（见 resolveAccount）。
type appAccessTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	Scope        string `json:"scope"`
	UnionID      string `json:"unionid"`
	ErrCode      int    `json:"errcode"`
	ErrMsg       string `json:"errmsg"`
}

// AppLogin App 端微信登录：移动应用 code 换取 openid（+ 可能的 unionid）→ 定位或建号 → 签发双令牌。
//
// 不做的事，两条都写在案：
//  1. 不调 /sns/userinfo 取昵称头像。nickname/headimgurl 是用户可控字符串，直接写进
//     username/avatar 就绕过了本仓的昵称审核面（profile review）；要引入得先决定它落在哪条
//     审核链路上，那是另一个决策，不在 #1482 射程内。
//  2. 不落库 access_token/refresh_token。它们只用于这一次身份换取，存下来即凭空多一项
//     凭据静态保管义务（本仓无此需求，见 CONTEXT.md「AppSecret 绝不下发客户端」同源口径）。
func (s *WechatAppService) AppLogin(ctx context.Context, code string) (*WxLoginResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("缺少微信登录凭证 code")
	}
	if !s.cfg.Configured() {
		return nil, errors.New("微信登录未配置，请联系管理员")
	}

	session, err := s.exchangeToken(ctx, code)
	if err != nil {
		return nil, err
	}

	user, isNew, err := s.resolveAccount(session.OpenID, session.UnionID)
	if err != nil {
		return nil, err
	}

	login, err := s.authSvc.issueLogin(loginCredentials{
		id: user.ID, account: user.Account, username: user.Username, status: &user.Status,
	}, core.HrwaiRole)
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

// exchangeToken 用微信开放平台的移动应用端点把 code 换成 openid/unionid。
// 注意参数名是 code（小程序那条是 js_code），凭证是移动应用那对（不是小程序/网页扫码）。
func (s *WechatAppService) exchangeToken(ctx context.Context, code string) (*appAccessTokenResponse, error) {
	q := url.Values{}
	q.Set("appid", s.cfg.AppID)
	q.Set("secret", s.cfg.AppSecret)
	q.Set("code", code)
	q.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.accessTokenBase+"?"+q.Encode(), nil)
	if err != nil {
		return nil, errors.New("微信登录请求构建失败")
	}
	resp, err := s.httpCli.Do(req)
	if err != nil {
		s.logger.Warn("app oauth2/access_token 调用失败", zap.Error(err))
		return nil, errors.New("微信登录服务暂不可用，请稍后再试")
	}
	defer resp.Body.Close()

	var body appAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		s.logger.Warn("app oauth2/access_token 响应解析失败", zap.Error(err))
		return nil, errors.New("微信登录服务响应异常，请稍后再试")
	}
	if body.ErrCode != 0 {
		s.logger.Warn("app oauth2/access_token 业务失败",
			zap.Int("errcode", body.ErrCode), zap.String("errmsg", body.ErrMsg))
		switch body.ErrCode {
		case appErrBadCode:
			return nil, errors.New("微信登录凭证已失效，请重新登录")
		case appErrNotLaunchedQuota:
			return nil, errors.New("微信登录次数已达上限，请稍后再试")
		default:
			return nil, fmt.Errorf("微信登录失败（错误码 %d），请稍后再试", body.ErrCode)
		}
	}
	if body.OpenID == "" {
		return nil, errors.New("微信登录响应缺少 openid，请稍后再试")
	}
	return &body, nil
}

// resolveAccount 定位这个微信用户属于哪个账号，两条链路共用同一张 hrwai_users 表。
//
// 定位顺序（#1482 决策 2 的落地形态，取「能同人就同人、不能就独立」而不是二选一）：
//  1. unionid 非空就先按它找——命中即复用该账号，小程序与 App 落到同一个自然人。
//     多行同 unionid 时取 id 最小者（先注册的那个账号），口径确定、不随查询次序漂。
//  2. 否则按 App openid 找——unionid 拿不到的场合（移动应用与小程序不在同一开放平台账号下、
//     或用户未授权 userinfo scope）自动退化为「两端各自独立账号」，无需二次改判代码。
//  3. 都没有才建号。
//
// 为什么 unionid 上只建普通索引而不是唯一约束：存量库里同一 unionid 出现两行是可能的
// （历史上按其他方式建过号又绑了 unionid），加唯一约束会让迁移在生产库上直接失败，
// 而这条风险在仓内不可复算。冲突时的行为已经由第 1 条的 id 升序定序，不需要约束来兜。
func (s *WechatAppService) resolveAccount(openID, unionID string) (*model.HrwaiUser, bool, error) {
	st := wechatAccountStore{db: s.db, logger: s.logger}

	if unionID != "" {
		var byUnion model.HrwaiUser
		err := s.db.Where("wechat_unionid = ?", unionID).Order("id ASC").First(&byUnion).Error
		if err == nil {
			return &byUnion, false, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, errors.New("登录失败，请稍后再试")
		}
	}

	user, found, err := st.lookupByOpenID(openID)
	if err != nil {
		return nil, false, err
	}
	if found {
		return user, false, nil
	}
	return st.create(openID, unionID)
}
