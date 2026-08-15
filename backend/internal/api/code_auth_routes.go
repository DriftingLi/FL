// Package api 实现 HTTP handlers。
// 本文件：验证码认证路由生成器——注册/登录/发送一份骨架，通道作为 adapter 注入
// （ADR-0001 的 CodeChannel seam 的自然收尾：handler 层不再按通道复制）。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/captcha"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// CodeChannelAuthHandler 验证码注册/登录 handler（通道注入，一份骨架两通道共用）。
type CodeChannelAuthHandler struct {
	sess           *security.Session
	codeSvc        *service.VerifyCodeService
	ch             service.CodeChannel
	targetField    string // 请求体中的目标字段名（email / phone）
	sentMsg        string // 发送成功提示文案
	captchaSvc     *captcha.Service
	captchaEnabled bool
}

// NewCodeChannelAuthHandler 创建验证码注册/登录 handler。
func NewCodeChannelAuthHandler(sess *security.Session, codeSvc *service.VerifyCodeService, ch service.CodeChannel, targetField, sentMsg string, captchaSvc *captcha.Service, captchaEnabled bool) *CodeChannelAuthHandler {
	return &CodeChannelAuthHandler{sess: sess, codeSvc: codeSvc, ch: ch, targetField: targetField, sentMsg: sentMsg, captchaSvc: captchaSvc, captchaEnabled: captchaEnabled}
}

// registerCodeChannelAuthRoutes 注册 /auth/<prefix> 蓝图（验证码注册/登录，通道注入）。
func registerCodeChannelAuthRoutes(g *gin.RouterGroup, sess *security.Session, codeSvc *service.VerifyCodeService,
	ch service.CodeChannel, targetField, sentMsg string, captchaSvc *captcha.Service, captchaEnabled bool) {
	h := NewCodeChannelAuthHandler(sess, codeSvc, ch, targetField, sentMsg, captchaSvc, captchaEnabled)

	// POST /auth/<prefix>/send-code {targetField, purpose: register|login}
	g.POST("/send-code", h.SendCode)
	// POST /auth/<prefix>/register {targetField, code, nickname, company?, password} 验证码通过后注册并自动登录
	g.POST("/register", h.Register)
	// POST /auth/<prefix>/login {targetField, code} 验证码通过后登录
	g.POST("/login", h.Login)
	// POST /auth/<prefix>/reset-password {targetField, code, password} 忘记密码：验证码通过后重置密码
	g.POST("/reset-password", h.ResetPassword)
}

// resolvePurpose 显式化 purpose 白名单校验：非法值报错（与既有文案逐字一致）。
func resolvePurpose(purpose string) (service.CodePurpose, error) {
	switch service.CodePurpose(purpose) {
	case service.CodePurposeRegister, service.CodePurposeLogin, service.CodePurposeResetPassword:
		return service.CodePurpose(purpose), nil
	default:
		return "", badRequest("purpose 必须为 register、login 或 reset_password")
	}
}

// codeSendReq 发码请求：Target/Purpose/Captcha* 由单次绑定填充（targetField 动态字段以两个小 struct 表达）。
type codeSendReq struct {
	Target       string
	Purpose      string
	CaptchaID    string
	CaptchaValue string
}

// parseSendReq 单次绑定目标字段 + purpose/captcha（避免 ShouldBindJSON 二次消费 body）。
func (h *CodeChannelAuthHandler) parseSendReq(c *gin.Context) (*codeSendReq, error) {
	if h.targetField == "phone" {
		var t struct {
			Phone        string `json:"phone"`
			Purpose      string `json:"purpose"`
			CaptchaID    string `json:"captcha_id"`
			CaptchaValue string `json:"captcha_value"`
		}
		if err := c.ShouldBindJSON(&t); err != nil {
			return nil, badRequest("请求参数错误")
		}
		return &codeSendReq{Target: t.Phone, Purpose: t.Purpose, CaptchaID: t.CaptchaID, CaptchaValue: t.CaptchaValue}, nil
	}
	var t struct {
		Email        string `json:"email"`
		Purpose      string `json:"purpose"`
		CaptchaID    string `json:"captcha_id"`
		CaptchaValue string `json:"captcha_value"`
	}
	if err := c.ShouldBindJSON(&t); err != nil {
		return nil, badRequest("请求参数错误")
	}
	return &codeSendReq{Target: t.Email, Purpose: t.Purpose, CaptchaID: t.CaptchaID, CaptchaValue: t.CaptchaValue}, nil
}

// SendCode 发送验证码 POST /auth/<prefix>/send-code
// body: {targetField, purpose, captcha_id, captcha_value}；开启人机验证时先校验图形验证码。
func (h *CodeChannelAuthHandler) SendCode(c *gin.Context) {
	Endpoint[codeSendReq, struct{}]{
		Parse:  h.parseSendReq,
		Invoke: h.invokeSendCode,
		Render: func(c *gin.Context, _ *codeSendReq, _ *struct{}, err error) {
			if err != nil {
				var pe *ParseError
				if asParseError(err, &pe) {
					renderStatus(c, pe.Status, pe.Message)
					return
				}
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, h.sentMsg, nil)
		},
	}.Handle(c)
}

func (h *CodeChannelAuthHandler) invokeSendCode(ctx context.Context, req *codeSendReq) (*struct{}, error) {
	if h.captchaEnabled {
		if !h.captchaSvc.Verify(ctx, req.CaptchaID, req.CaptchaValue) {
			return nil, badRequest("图形验证码错误或已过期")
		}
	}
	purpose, err := resolvePurpose(req.Purpose)
	if err != nil {
		return nil, err
	}
	if err := h.codeSvc.Send(ctx, h.ch, purpose, req.Target); err != nil {
		return nil, err
	}
	return &struct{}{}, nil
}

// codeRegisterReq 注册请求（Target 为动态字段解析结果）。
type codeRegisterReq struct {
	Target   string
	Code     string
	Nickname string
	Company  string
	Password string
}

// Register 验证码通过后注册并自动登录 POST /auth/<prefix>/register
func (h *CodeChannelAuthHandler) Register(c *gin.Context) {
	Endpoint[codeRegisterReq, service.LoginResult]{
		Parse: h.parseRegisterReq,
		Invoke: func(ctx context.Context, req *codeRegisterReq) (*service.LoginResult, error) {
			return h.codeSvc.RegisterWithCode(ctx, h.ch, req.Target, req.Code, req.Nickname, req.Company, req.Password)
		},
		Render: func(c *gin.Context, _ *codeRegisterReq, resp *service.LoginResult, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			setAuthCookie(c, h.sess, resp.Token)
			response.Created(c, "注册成功", resp)
		},
	}.Handle(c)
}

func (h *CodeChannelAuthHandler) parseRegisterReq(c *gin.Context) (*codeRegisterReq, error) {
	if h.targetField == "phone" {
		var t struct {
			Phone    string `json:"phone"`
			Code     string `json:"code"`
			Nickname string `json:"nickname"`
			Company  string `json:"company"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&t); err != nil {
			return nil, badRequest("请求参数错误")
		}
		return &codeRegisterReq{Target: t.Phone, Code: t.Code, Nickname: t.Nickname, Company: t.Company, Password: t.Password}, nil
	}
	var t struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Nickname string `json:"nickname"`
		Company  string `json:"company"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&t); err != nil {
		return nil, badRequest("请求参数错误")
	}
	return &codeRegisterReq{Target: t.Email, Code: t.Code, Nickname: t.Nickname, Company: t.Company, Password: t.Password}, nil
}

// codeLoginReq 登录请求（Target 为动态字段解析结果）。
type codeLoginReq struct {
	Target string
	Code   string
}

// Login 验证码通过后登录 POST /auth/<prefix>/login
func (h *CodeChannelAuthHandler) Login(c *gin.Context) {
	Endpoint[codeLoginReq, service.LoginResult]{
		Parse: h.parseLoginReq,
		Invoke: func(ctx context.Context, req *codeLoginReq) (*service.LoginResult, error) {
			return h.codeSvc.LoginWithCode(ctx, h.ch, req.Target, req.Code)
		},
		Render: func(c *gin.Context, _ *codeLoginReq, resp *service.LoginResult, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			setAuthCookie(c, h.sess, resp.Token)
			response.SuccessWithMsg(c, "登录成功", resp)
		},
	}.Handle(c)
}

func (h *CodeChannelAuthHandler) parseLoginReq(c *gin.Context) (*codeLoginReq, error) {
	if h.targetField == "phone" {
		var t struct {
			Phone string `json:"phone"`
			Code  string `json:"code"`
		}
		if err := c.ShouldBindJSON(&t); err != nil {
			return nil, badRequest("请求参数错误")
		}
		return &codeLoginReq{Target: t.Phone, Code: t.Code}, nil
	}
	var t struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := c.ShouldBindJSON(&t); err != nil {
		return nil, badRequest("请求参数错误")
	}
	return &codeLoginReq{Target: t.Email, Code: t.Code}, nil
}

// codeResetReq 重置密码请求（Target 为动态字段解析结果）。
type codeResetReq struct {
	Target   string
	Code     string
	Password string
}

// ResetPassword 忘记密码：验证码校验通过后重置密码（不自动登录）。
func (h *CodeChannelAuthHandler) ResetPassword(c *gin.Context) {
	Endpoint[codeResetReq, struct{}]{
		Parse: h.parseResetReq,
		Invoke: func(ctx context.Context, req *codeResetReq) (*struct{}, error) {
			if err := h.codeSvc.ResetPasswordWithCode(ctx, h.ch, req.Target, req.Code, req.Password); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *codeResetReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "密码已重置，请使用新密码登录", nil)
		},
	}.Handle(c)
}

func (h *CodeChannelAuthHandler) parseResetReq(c *gin.Context) (*codeResetReq, error) {
	if h.targetField == "phone" {
		var t struct {
			Phone    string `json:"phone"`
			Code     string `json:"code"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&t); err != nil {
			return nil, badRequest("请求参数错误")
		}
		return &codeResetReq{Target: t.Phone, Code: t.Code, Password: t.Password}, nil
	}
	var t struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&t); err != nil {
		return nil, badRequest("请求参数错误")
	}
	return &codeResetReq{Target: t.Email, Code: t.Code, Password: t.Password}, nil
}

// asParseError 判断 error 是否 *ParseError（供 render 分支区分参数错误与服务错误）。
func asParseError(err error, target **ParseError) bool {
	pe, ok := err.(*ParseError)
	if ok {
		*target = pe
	}
	return ok
}
