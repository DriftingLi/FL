// Package api 实现 HTTP handlers。
// 本文件：图形验证码（人机验证）接口。
package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/captcha"
	"forklift-training/pkg/response"
)

// RegisterCaptchaRoutes 注册 GET /api/captcha（无需鉴权）。
// 返回 {id, image}；image 为 PNG 的 base64 data URL，id 随 send-code 请求提交。
func RegisterCaptchaRoutes(r *gin.Engine, svc *captcha.Service) {
	r.GET("/api/captcha", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		id, imageURL, err := svc.Generate(ctx)
		if err != nil {
			response.ServerError(c, "图形验证码生成失败，请重试")
			return
		}
		response.Success(c, gin.H{"id": id, "image": imageURL})
	})
}
