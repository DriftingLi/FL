// HTTP 出口（原 internal/valuation/handler 子包）：实现残值评估模块的 HTTP 处理器。
// 本文件：估值模块认证 handler（/api/valuation/auth/*）。
// 已统一到主体系 AuthService 与 security 会话模块,本 handler 仅作为前端兼容入口。
package valuation

import (
	"github.com/gin-gonic/gin"

	vcore "forklift-training/internal/core"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/response"
)

// ValuationLoginResult 登录成功后的最小结果面：只含 BFF 需要的字段，
// 不把主体系 LoginResult 的形状抄进来（域包之间只递数据，不递对方的类型）。
type ValuationLoginResult struct {
	Token        string
	RefreshToken string
	UserID       int
	Account      string
	Username     string
	Role         string
}

// ValuationLoginFunc 按令牌族验密并签发的窄接口 —— 由主体系 auth.Service.HrwaiLoginFor 适配。
// 之所以是函数而不是接口：估值域只需要「一个能力」，多一个接口就多一层要维护的实现面。
type ValuationLoginFunc func(family security.Family, account, password string) (ValuationLoginResult, error)

// ValuationAuthHandler 估值模块认证处理器（消费 ValuationAuth 窄接口，主体系 AuthService 直接满足）。
type ValuationAuthHandler struct {
	authSvc ValuationAuth
	sess    *security.Session
	// login 为 nil 时 /auth/login 返回 503：装配根没接这条线时**不静默降级**成 401，
	// 否则「端点没接」会被误读成「凭证不对」。
	login ValuationLoginFunc
}

// NewValuationAuthHandler 构造估值认证处理器。sess 为装配根注入的唯一会话实例。
func NewValuationAuthHandler(authSvc ValuationAuth, sess *security.Session) *ValuationAuthHandler {
	return &ValuationAuthHandler{authSvc: authSvc, sess: sess}
}

// NewValuationAuthHandlerWithLogin 带上按族登录能力的构造函数（#1756 第二批）。
func NewValuationAuthHandlerWithLogin(authSvc ValuationAuth, sess *security.Session, login ValuationLoginFunc) *ValuationAuthHandler {
	return &ValuationAuthHandler{authSvc: authSvc, sess: sess, login: login}
}

// Login 处理 POST /api/valuation/auth/login：签发**估值族**双令牌。
//
// 与主站 /api/auth/login 的差别只有两处，都重要：
//  1. 令牌带 aud=valuation —— 主站入口不认它（会话隔离的落点）；
//  2. **不下发 Cookie**：令牌由调用方（Nuxt BFF）持有，浏览器侧不出现跨站 Cookie。
func (h *ValuationAuthHandler) Login(c *gin.Context) {
	if h.login == nil {
		response.ServerError(c, "估值登录通道未装配")
		return
	}
	var req struct {
		Account  string `json:"account" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "账号与密码不能为空")
		return
	}
	res, err := h.login(security.FamilyValuation, req.Account, req.Password)
	if err != nil {
		// 与主站同口径：验密失败一律「账号或密码错误」，不区分账号是否存在（防枚举）。
		response.Unauthorized(c, err.Error())
		return
	}
	response.Success(c, map[string]interface{}{
		"token":         res.Token,
		"refresh_token": res.RefreshToken,
		"user_id":       res.UserID,
		"account":       res.Account,
		"username":      res.Username,
		"role":          res.Role,
	})
}

// Me 处理 GET /api/valuation/auth/me（需 middleware.JWTAuth）
// @Summary 当前估值用户
// @Description 返回当前登录估值用户的基础资料（user_id/uid/account/username/phone/email/company/role），手机号脱敏。需登录（估值鉴权组）。
// @Tags 估值-认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=object{user_id=integer,uid=string,account=string,username=string,phone=string,email=string,company=string,role=string}} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "用户不存在"
// @Router /valuation/auth/me [get]
func (h *ValuationAuthHandler) Me(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	if uid == 0 {
		response.Unauthorized(c, "Token无效或已过期，请重新登录")
		return
	}
	user, err := h.authSvc.GetHrwaiUserByID(uid)
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	response.Success(c, map[string]interface{}{
		"user_id":  user.ID,
		"uid":      vcore.FormatUID(user.UID),
		"account":  user.Account,
		"username": user.Username,
		"phone":    vcore.MaskedPhone(user.Phone),
		"email":    user.Email,
		"company":  user.Company,
		"role":     vcore.HrwaiRole,
	})
}

// Logout 处理 POST /api/valuation/auth/logout（#1388 起收敛到主站同一个动作 Session.SignOut）：
// 吊销手上那支 refresh（请求体优先，回退 Bearer 头）**并清除登录态 Cookie**（access + 两族 refresh）。
// 原形状只吊销、一枚 Cookie 都不清 ⇒ 还在用这个入口的客户端「登出」之后，浏览器里那枚 7 天
// refresh 原封不动，登出退化成前端自己把状态擦了（ADR-0067 要降的那层后果一件都没降）。
//
// 这里**不读** refresh Cookie，也不是遗漏：它的 `Path=/api/auth`（ADR-0067 决策 2 的最小暴露面）
// 结构上覆盖不到 `/api/valuation/auth/logout`，写那一路就是一枚恒空的死代码。可达性由
// TestValuationLogout_刷新Cookie到不了本端点 钉住 —— 谁把 Path 放宽到 `/`（把 7 天凭证挂到全站
// 每一个请求上）就会在那里判红，被迫在这里重新决策，而不是让代码与 ADR 各说一套。
// 仓内的估值工作区不走这个入口（`ValuationLayout.vue` 的退出调 `authStore.signOut()` →
// 主 `/api/auth/logout`，Cookie 与族判定都在那里生效）；本端点是 `API.md` 对外列着的兼容入口。
//
// #1412 起标废弃（维护者 2026-09-30 定 B 方案：先对外宣告、留一个发布周期的观察窗口，真正的
// 移除另票且由人执行）。`@Deprecated` 让它进生成契约而不只是散文 —— 观察窗口结束时按「有无带
// 凭证调用」复取证（判据不是「恒为 0」：33 天里那 1 次是无凭证的人为试探，见 PR #1399 收尾评论）。
// @Summary 估值用户登出
// @Description 吊销 refresh_token（请求体优先，回退 Bearer 头）并清除登录态 Cookie（access + 两族 refresh）；不依赖 JWTAuth。公开端点：无需登录。已废弃：将在下一版移除，请改用 POST /api/auth/logout
// @Deprecated
// @Tags 估值-认证
// @Accept json
// @Produce json
// @Param body body object false "refresh_token" example({"refresh_token":"eyJhbGciOi..."})
// @Success 200 {object} response.R "success"
// @Router /valuation/auth/logout [post]
func (h *ValuationAuthHandler) Logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&req)
	tokenStr := req.RefreshToken
	if tokenStr == "" {
		tokenStr = h.sess.ExtractToken(c.GetHeader("Authorization"), "")
	}
	// 吊销失败仍清 Cookie：本地登录态已不可用，凭证缺口由 SignOut 的返回值暴露（主站同口径）。
	_ = h.sess.SignOut(c.Request.Context(), c.Writer, tokenStr)
	response.Success(c, nil)
}
