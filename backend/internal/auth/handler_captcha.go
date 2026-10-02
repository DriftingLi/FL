// 本文件：图形验证码（人机验证）接口。
package auth

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/captcha"
	"forklift-training/pkg/httpx"
)

// GenerateCaptchaDTO 图形验证码生成结果的展示对象（shape-lock：顶层键集 {id, image}）。
type GenerateCaptchaDTO struct {
	ID    string `json:"id"`
	Image string `json:"image"`
}

// captchaHandler 图形验证码 handler。
type captchaHandler struct {
	svc *captcha.Service
}

// newCaptchaHandler 构造图形验证码 handler。
func newCaptchaHandler(svc *captcha.Service) *captchaHandler {
	return &captchaHandler{svc: svc}
}

// RegisterCaptchaRoutes 注册 GET /api/captcha（无需鉴权）。
// 返回 {id, image}；image 为 PNG 的 base64 data URL，id 随 send-code 请求提交。
func RegisterCaptchaRoutes(r *gin.Engine, svc *captcha.Service) {
	h := newCaptchaHandler(svc)
	r.GET("/api/captcha", h.Generate)
}

// Generate 生成图形验证码
// @Summary 图形验证码
// @Description 生成 id 与 base64 图片，用于人机验证
// @Tags 学员端-认证
// @Produce json
// @Success 200 {object} response.R{data=GenerateCaptchaDTO} "success"
// @Failure 500 {object} response.R "失败"
// @Router /captcha [get]
func (h *captchaHandler) Generate(c *gin.Context) {
	httpx.Endpoint[struct{}, GenerateCaptchaDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*GenerateCaptchaDTO, error) {
			id, imageURL, err := h.svc.Generate(ctx)
			if err != nil {
				return nil, err
			}
			return &GenerateCaptchaDTO{ID: id, Image: imageURL}, nil
		},
		ErrStatus: httpx.ErrStatusAllMsg(http.StatusInternalServerError, "图形验证码生成失败，请重试"),
	}.Handle(c)
}
