// Package api 实现 HTTP handlers。
// 本文件：个人信息页手机号/邮箱绑定修改（验证码校验 + 格式/唯一性校验，走统一验证码 engine）。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// ProfileBindHandler 个人信息绑定修改 handler。
type ProfileBindHandler struct {
	codeSvc *service.VerifyCodeService
	emailCh service.CodeChannel
	phoneCh service.CodeChannel
}

// NewProfileBindHandler 创建个人信息绑定修改 handler。
func NewProfileBindHandler(codeSvc *service.VerifyCodeService, emailCh, phoneCh service.CodeChannel) *ProfileBindHandler {
	return &ProfileBindHandler{codeSvc: codeSvc, emailCh: emailCh, phoneCh: phoneCh}
}

// RegisterProfileBindRoutes 注册 /api/auth/profile 蓝图（登录后绑定/修改手机号、邮箱）
// 与 /api/auth/account 蓝图（短信验证码确认修改登录账号）。
func RegisterProfileBindRoutes(rg *gin.RouterGroup, rd RouterDeps, codeSvc *service.VerifyCodeService, emailCh, phoneCh service.CodeChannel) {
	h := NewProfileBindHandler(codeSvc, emailCh, phoneCh)

	g := rg.Group("/auth/profile", middleware.JWTAuth(rd.Session))

	// POST /api/auth/profile/send-code {channel: email|phone, target}
	g.POST("/send-code", h.SendCode)
	// 绑定/修改端点按通道生成（一份骨架，通道作为 adapter 注入）
	g.POST("/email", h.bindEmail)
	g.POST("/phone", h.bindPhone)
	// POST /api/auth/profile/password/send-code 发送修改密码验证码
	g.POST("/password/send-code", h.SendChangePasswordCode)
	// POST /api/auth/profile/password {code, password} 设置/修改密码（短信验证码确认）
	g.POST("/password", h.UpdatePassword)

	// 修改登录账号（短信验证码确认）：PUT /api/auth/account、POST /api/auth/account/send-code
	acct := rg.Group("/auth/account", middleware.JWTAuth(rd.Session))
	acct.PUT("", h.UpdateAccount)
	acct.POST("/send-code", h.SendAccountChangeCode)
}

// sendCodeReq 发送绑定验证码请求 {channel, target}。
type sendCodeReq struct {
	Channel string `json:"channel"`
	Target  string `json:"target"`
}

// SendCode 发送绑定验证码 POST /api/auth/profile/send-code {channel: email|phone, target}
func (h *ProfileBindHandler) SendCode(c *gin.Context) {
	Endpoint[sendCodeReq, struct{}]{
		Parse: func(c *gin.Context) (*sendCodeReq, error) {
			return bindJSON[sendCodeReq](c)
		},
		Invoke: func(ctx context.Context, req *sendCodeReq) (*struct{}, error) {
			channels := map[string]service.CodeChannel{"email": h.emailCh, "phone": h.phoneCh}
			ch, ok := channels[req.Channel]
			if !ok {
				return nil, badRequest("channel 必须为 email 或 phone")
			}
			if err := h.codeSvc.SendBind(ctx, ch, middleware.CurrentUserID(c), req.Target); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *sendCodeReq, _ *struct{}, err error) {
			if err != nil {
				var pe *ParseError
				if asParseError(err, &pe) {
					renderStatus(c, pe.Status, pe.Message)
					return
				}
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "验证码已发送，请查收", nil)
		},
	}.Handle(c)
}

// bindEmail 绑定/修改邮箱 POST /api/auth/profile/email
func (h *ProfileBindHandler) bindEmail(c *gin.Context) {
	handleCodeChannelBind(c, h.codeSvc, h.emailCh, "email", "邮箱修改成功")
}

// bindPhone 绑定/修改手机号 POST /api/auth/profile/phone
func (h *ProfileBindHandler) bindPhone(c *gin.Context) {
	handleCodeChannelBind(c, h.codeSvc, h.phoneCh, "phone", "手机号修改成功")
}

// SendChangePasswordCode 发送修改密码验证码 POST /api/auth/profile/password/send-code
func (h *ProfileBindHandler) SendChangePasswordCode(c *gin.Context) {
	Endpoint[profileUserIDReq, struct{}]{
		Parse: func(c *gin.Context) (*profileUserIDReq, error) {
			return &profileUserIDReq{UserID: middleware.CurrentUserID(c)}, nil
		},
		Invoke: func(ctx context.Context, req *profileUserIDReq) (*struct{}, error) {
			if err := h.codeSvc.SendChangePasswordCode(ctx, h.phoneCh, req.UserID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: successMsgRenderer[profileUserIDReq]("验证码已发送，请查收"),
	}.Handle(c)
}

// profileUserIDReq 仅带登录用户 ID 的请求（发送验证码类）。
type profileUserIDReq struct {
	UserID int
}

// changePasswordReq 设置/修改密码请求 {code, password}。
type changePasswordReq struct {
	Code     string `json:"code"`
	Password string `json:"password"`
}

// UpdatePassword 设置/修改密码 POST /api/auth/profile/password {code, password}
func (h *ProfileBindHandler) UpdatePassword(c *gin.Context) {
	Endpoint[changePasswordReq, struct{}]{
		Parse: func(c *gin.Context) (*changePasswordReq, error) {
			return bindJSON[changePasswordReq](c)
		},
		Invoke: func(ctx context.Context, req *changePasswordReq) (*struct{}, error) {
			if err := h.codeSvc.ChangePassword(ctx, h.phoneCh, middleware.CurrentUserID(c), req.Code, req.Password); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: successMsgRenderer[changePasswordReq]("密码设置成功"),
	}.Handle(c)
}

// SendAccountChangeCode 发送修改登录账号验证码 POST /api/auth/account/send-code
func (h *ProfileBindHandler) SendAccountChangeCode(c *gin.Context) {
	Endpoint[profileUserIDReq, struct{}]{
		Parse: func(c *gin.Context) (*profileUserIDReq, error) {
			return &profileUserIDReq{UserID: middleware.CurrentUserID(c)}, nil
		},
		Invoke: func(ctx context.Context, req *profileUserIDReq) (*struct{}, error) {
			if err := h.codeSvc.SendAccountChange(ctx, h.phoneCh, req.UserID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: successMsgRenderer[profileUserIDReq]("验证码已发送，请查收"),
	}.Handle(c)
}

// changeAccountReq 修改登录账号请求 {account, code}。
type changeAccountReq struct {
	Account string `json:"account"`
	Code    string `json:"code"`
}

// UpdateAccount 修改登录账号 PUT /api/auth/account {account, code}
// 短信验证码确认 + 格式 4~20 位字母/数字/下划线 + 唯一性校验（复用 profile_bind 验证码模式）。
func (h *ProfileBindHandler) UpdateAccount(c *gin.Context) {
	Endpoint[changeAccountReq, service.LoginResult]{
		Parse: func(c *gin.Context) (*changeAccountReq, error) {
			return bindJSON[changeAccountReq](c)
		},
		Invoke: func(ctx context.Context, req *changeAccountReq) (*service.LoginResult, error) {
			return h.codeSvc.ChangeAccount(ctx, h.phoneCh, middleware.CurrentUserID(c), req.Account, req.Code)
		},
		Render: func(c *gin.Context, _ *changeAccountReq, resp *service.LoginResult, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "账号修改成功", resp)
		},
	}.Handle(c)
}

// handleCodeChannelBind 绑定/修改目标字段的公共实现（通道注入）。
// body: {email|phone, code}；targetField 动态字段名以两个小 struct（bindEmailFields/bindPhoneFields）表达。
func handleCodeChannelBind(c *gin.Context, codeSvc *service.VerifyCodeService, ch service.CodeChannel, targetField, successMsg string) {
	Endpoint[codeBindReq, struct{}]{
		Parse: func(c *gin.Context) (*codeBindReq, error) {
			return parseCodeBindReq(c, targetField)
		},
		Invoke: func(ctx context.Context, req *codeBindReq) (*struct{}, error) {
			userID := middleware.CurrentUserID(c)
			if err := codeSvc.Bind(ctx, ch, userID, req.Target, req.Code); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *codeBindReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, successMsg, nil)
		},
	}.Handle(c)
}

// codeBindReq 绑定请求（Target 为动态字段 email/phone，按 targetField 解析）。
type codeBindReq struct {
	Target string
	Code   string
}

// parseCodeBindReq 解析绑定请求：targetField 动态字段 + code。
// 类型错误显式报错（绑定失败返回 400），不再静默取空串。
func parseCodeBindReq(c *gin.Context, targetField string) (*codeBindReq, error) {
	if targetField == "phone" {
		var t struct {
			Phone string `json:"phone"`
			Code  string `json:"code"`
		}
		if err := c.ShouldBindJSON(&t); err != nil {
			return nil, badRequest("请求参数错误")
		}
		return &codeBindReq{Target: t.Phone, Code: t.Code}, nil
	}
	var t struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := c.ShouldBindJSON(&t); err != nil {
		return nil, badRequest("请求参数错误")
	}
	return &codeBindReq{Target: t.Email, Code: t.Code}, nil
}

// successMsgRenderer 渲染器：成功时输出自定义 message + nil data，失败委托 defaultRender。
func successMsgRenderer[Req any](msg string) RenderFunc[Req, struct{}] {
	return func(c *gin.Context, _ *Req, _ *struct{}, err error) {
		renderNilOrError(c, err, msg)
	}
}

// renderNilOrError 渲染 nil data 成功信封（msg）或错误信封（parse/serve 区分）。
func renderNilOrError(c *gin.Context, err error, msg string) {
	if err != nil {
		var pe *ParseError
		if asParseError(err, &pe) {
			renderStatus(c, pe.Status, pe.Message)
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, msg, nil)
}
