// 本文件：微信登录。
// - 小程序登录（code2session）：uni.login 临时凭证换 openid → 按 openid 查/建用户 → 签发双令牌。
// - 扫码登录（开放平台）：框架占位，授权信息待接入。
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

	"forklift-training/internal/clock"
	"forklift-training/internal/config"
	"forklift-training/internal/core"
	"forklift-training/internal/dberr"
	"forklift-training/internal/model"
)

// 微信 code2session 端点与错误码语义（官方文档）。
const (
	wechatCode2SessionURL = "https://api.weixin.qq.com/sns/jscode2session"
	wechatErrBadCode      = 40029 // code 无效
	wechatErrRateLimit    = 45011 // API 分钟级频率限制
	wechatErrBlocked      = 40226 // 高风险用户，登录拦截
)

// WechatService 微信登录服务。
type WechatService struct {
	cfg     config.WechatAppConfig
	db      *gorm.DB
	authSvc *Service
	logger  *zap.Logger

	// code2session 基地址（默认官方端点；测试注入 httptest server 覆盖）
	apiBase string
	httpCli *http.Client
}

// NewWechatService 构造微信服务。
// db 用于按 openid 查/建用户；authSvc 复用登录签发骨架（双令牌 + 禁用校验）。
func NewWechatService(cfg config.WechatAppConfig, db *gorm.DB, authSvc *Service, logger *zap.Logger) *WechatService {
	return &WechatService{
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
func (s *WechatService) MiniProgramLogin(ctx context.Context, code string) (*WxLoginResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("缺少微信登录凭证 code")
	}
	if !s.cfg.Configured() {
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

// code2Session 调用微信登录凭证校验接口，换取 openid/unionid。
func (s *WechatService) code2Session(ctx context.Context, code string) (*wxSessionResponse, error) {
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
		// 只记去掉 URL 的原因：*url.Error 的字符串含完整请求 URL，这条 URL 的查询串里带着 AppSecret。
		s.logger.Warn("code2session 调用失败", zap.Error(outboundCause(err)))
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
// 建号派生与并发兜底都在 wechatAccountStore 里（小程序与 App 两条链路共用同一份真源）。
func (s *WechatService) findOrCreateByOpenID(openID, unionID string) (*model.HrwaiUser, bool, error) {
	st := wechatAccountStore{db: s.db, logger: s.logger}
	user, found, err := st.lookupByOpenID(openID)
	if err != nil {
		return nil, false, err
	}
	if found {
		st.backfillUnionID(user, unionID)
		return user, false, nil
	}
	return st.create(openID, unionID)
}

// backfillUnionID 把这次换取到的 unionid 补写进已有账号，只补空位、不覆盖已有值。
//
// 为什么必须在「按 openid 命中」这一支上做：unionid 是 App 端认人的第一优先键（#1482），
// 而开放平台绑定**之前**建的那批行 `wechat_unionid` 恒为空串——那时 code2session 根本不返回它，
// 建号写进去的就是空。不在老用户登录时补上，「同人」这一臂就只对绑定之后新注册的账号成立，
// 恰好把最有价值的一批人（已经在小程序里买过课、学过进度的人）漏掉。
//
// 失败不阻断登录：补不上只是这次没认出同人，不该把一次已经成功的登录打回去（下次登录再补）。
func (st wechatAccountStore) backfillUnionID(user *model.HrwaiUser, unionID string) {
	if unionID == "" || user.WechatUnionID != "" {
		return
	}
	if err := st.db.Model(user).Update("wechat_unionid", unionID).Error; err != nil {
		st.logger.Warn("微信登录回填 unionid 失败", zap.Int("user_id", user.ID), zap.Error(err))
		return
	}
	user.WechatUnionID = unionID
}

// outboundCause 取外呼错误里**不含请求 URL** 的那层原因。
// Go 的 *url.Error 把完整 URL 拼进 Error()，而这两条微信链路（小程序 code2session、
// 移动应用 oauth2/access_token）都把 AppSecret 放在查询串里 ⇒ 直接 zap.Error(err) 等于
// 把服务端凭据写进日志文件。传输层失败的原因本身（DNS / 连接拒绝 / 超时）不含 URL，够定位了。
func outboundCause(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) && ue.Err != nil {
		return ue.Err
	}
	return err
}

// wechatAccountStore 微信登录的账号落库面：按标识定位用户 + 自动建号。
// 拆出来是因为 App 端链路（#1482）只改「先按什么认人」——openid 之外还要先试 unionid——
// 而建号派生规则（截段、冲突重试、并发回查）必须仍只有一处真源，两条链路各抄一份就会漂。
type wechatAccountStore struct {
	db     *gorm.DB
	logger *zap.Logger
}

// lookupByOpenID 按 wechat_openid 定位用户。三种返回各自独立可辨：
// 命中 → (user, true, nil)；未命中 → (nil, false, nil)；查询故障 → (nil, false, err)。
func (st wechatAccountStore) lookupByOpenID(openID string) (*model.HrwaiUser, bool, error) {
	var user model.HrwaiUser
	err := st.db.Where("wechat_openid = ?", openID).First(&user).Error
	if err == nil {
		return &user, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, errors.New("登录失败，请稍后再试")
	}
	return nil, false, nil
}

// create 自动建号并绑定 openid/unionid。
// account/username 由 openid 派生；account 前缀冲突时追加 openid 后段或序号重试（spec #279），
// 数据库唯一约束冲突与其他错误分类处理：冲突走回查/重试，其他错误透传可观测原因。
// 并发首登竞争由 wechat_openid 唯一索引兜底：撞唯一约束时按已存在用户处理。
func (st wechatAccountStore) create(openID, unionID string) (*model.HrwaiUser, bool, error) {
	// openid 截段：account 取前 12 位、昵称取后 6 位（同源不同段，避免与账号撞名）。
	baseSuffix := openID
	if len(baseSuffix) > 12 {
		baseSuffix = baseSuffix[:12]
	}
	tail := openID
	if len(tail) > 6 {
		tail = tail[len(tail)-6:]
	}
	// 候选账号/昵称序列：首选 "wx_"+前12，冲突时追加后6或序号重试
	prefix4 := baseSuffix
	if len(prefix4) > 4 {
		prefix4 = prefix4[:4]
	}
	candidates := []struct{ account, username string }{
		{"wx_" + baseSuffix, "微信学员" + tail},
		{"wx_" + baseSuffix + "_" + tail, "微信学员" + tail + "_" + prefix4},
	}
	// 再补充序号变体以覆盖极小概率的连续碰撞
	for i := 1; i <= 3; i++ {
		candidates = append(candidates, struct{ account, username string }{
			account:  fmt.Sprintf("wx_%s_%s_%d", baseSuffix, tail, i),
			username: fmt.Sprintf("微信学员%s_%d", tail, i),
		})
	}

	// 非手机号注册时手机号置空（允许空串多用户并存，唯一约束仅对非空手机号生效）
	phoneBase := ""
	var lastErr error
	for idx, cand := range candidates {
		newUser := model.HrwaiUser{
			UID:           core.NextUID(),
			Account:       cand.account,
			Username:      cand.username,
			Phone:         phoneBase,
			WechatOpenID:  openID,
			WechatUnionID: unionID,
			Status:        1,
			CreatedAt:     clock.Now(),
		}
		if err := st.db.Create(&newUser).Error; err == nil {
			return &newUser, true, nil
		} else {
			lastErr = err
			if dberr.IsDuplicateError(err) {
				// 并发首登：wechat_openid 已被其他请求抢先插入
				var again model.HrwaiUser
				if qErr := st.db.Where("wechat_openid = ?", openID).First(&again).Error; qErr == nil {
					return &again, false, nil
				}
				// 非 wechat_openid 的唯一冲突（大概率 account/username 前缀碰撞）则尝试下一候选
				// 若已是最后候选，继续循环会透传错误
				if idx < len(candidates)-1 {
					st.logger.Warn("微信自动建号账号冲突重试", zap.String("candidate", cand.account), zap.Error(err))
					continue
				}
			}
			// 非唯一冲突或候选耗尽：透传真实原因，便于可观测与区分「系统繁忙」与「注册失败」
			st.logger.Warn("微信自动注册失败", zap.String("candidate", cand.account), zap.Error(err))
			if dberr.IsDuplicateError(err) {
				return nil, false, errors.New("微信登录注册失败，请稍后再试")
			}
			return nil, false, fmt.Errorf("微信登录注册失败: %w", err)
		}
	}
	st.logger.Warn("微信自动建号候选耗尽", zap.Error(lastErr))
	return nil, false, errors.New("微信登录注册失败，请稍后再试")
}

// WechatQRCodeInfoDTO 扫码登录占位信息（ADR-0009 §2 typed DTO / spec #940 片三）。
// 字段按 JSON key 字母序声明（enabled / message / qr_url）—— 旧 map 的序列化序。
type WechatQRCodeInfoDTO struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
	QRURL   string `json:"qr_url"`
}

// QRCodeInfo 返回扫码登录占位信息：未配置授权时 enabled=false，前端展示占位二维码。
func (s *WechatService) QRCodeInfo() *WechatQRCodeInfoDTO {
	if !s.cfg.Configured() {
		return &WechatQRCodeInfoDTO{
			Enabled: false,
			Message: "微信授权暂未配置，请等待开放平台配置完成后使用",
			QRURL:   "",
		}
	}
	return &WechatQRCodeInfoDTO{
		Enabled: true,
		Message: "微信扫码登录待接入（二维码生成接口占位）",
		QRURL:   "",
	}
}

// LoginWithQRCode 微信扫码登录占位：真实授权流程待接入（与小程序 code2session 登录不同流）。
func (s *WechatService) LoginWithQRCode(code string) (*LoginResult, error) {
	return nil, errors.New("微信扫码登录尚未接入，请使用其他登录方式")
}
