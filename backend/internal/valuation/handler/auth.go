// Package handler 实现残值评估模块的 HTTP 处理器。
// 本文件：估值模块认证 handler（/api/valuation/auth/*）。
// 已统一到主体系 AuthService 与 security 会话模块,本 handler 仅作为前端兼容入口。
package handler

import (
	"github.com/gin-gonic/gin"

	vcore "forklift-training/internal/core"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/response"
)

// ValuationAuthHandler 估值模块认证处理器（消费 ValuationAuth 窄接口，主体系 AuthService 直接满足）。
type ValuationAuthHandler struct {
	authSvc ValuationAuth
	sess    *security.Session
}

// NewValuationAuthHandler 构造估值认证处理器。sess 为装配根注入的唯一会话实例。
func NewValuationAuthHandler(authSvc ValuationAuth, sess *security.Session) *ValuationAuthHandler {
	return &ValuationAuthHandler{authSvc: authSvc, sess: sess}
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
