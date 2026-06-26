// Package api 实现 HTTP handlers。
package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterAIRoutes 注册 /api/ai 蓝图（AI 文本生成与 Coze OAuth 令牌）。
// 对应 Python app/api/ai.py。
func RegisterAIRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := newAIService(cfg, db)

	g := rg.Group("/ai", middleware.JWTAuth(cfg))

	// GET /api/ai/test  AI 连通性测试
	g.GET("/test", func(c *gin.Context) {
		result := svc.TestConnection()
		status := 200
		if s, ok := result["status"].(string); ok && s != "success" {
			status = 503
		}
		msg, _ := result["message"].(string)
		c.JSON(status, response.R{Code: status, Message: msg, Data: result})
	})

	// POST /api/ai/generate/text  根据关键词生成文本
	g.POST("/generate/text", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		role, _ := c.Get(string(middleware.CtxUserRole))
		userID, _ := uid.(int)
		roleStr, _ := role.(string)
		var req struct {
			Keyword string `json:"keyword"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Keyword == "" {
			response.BadRequest(c, "请输入知识点关键词")
			return
		}
		result := svc.GenerateText(req.Keyword, userID, roleStr)
		response.Success(c, result)
	})

	// GET /api/ai/history  生成历史
	g.GET("/history", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		userID, _ := uid.(int)
		genType := c.Query("type")
		limit := atoiDefault(c.Query("limit"), 10)
		response.Success(c, svc.GetGenerationHistory(userID, genType, limit))
	})

	// GET /api/ai/coze/token  获取 Coze OAuth 访问令牌
	g.GET("/coze/token", func(c *gin.Context) {
		if cfg.Coze.ProjectID == "" || cfg.Coze.OAuthAppID == "" || cfg.Coze.OAuthKID == "" {
			response.ServerError(c, "扣子智能体 OAuth 未配置，请设置 COZE_PROJECT_ID、COZE_OAUTH_APP_ID、COZE_OAUTH_KID 环境变量")
			return
		}
		cozeSvc := service.NewCozeOAuthService(cfg.Coze.OAuthAppID, cfg.Coze.OAuthKID, cfg.Coze.OAuthPrivateKey, cfg.Coze.OAuthKeyPath)
		token, err := cozeSvc.GetAccessToken()
		if err != nil {
			response.ServerError(c, "获取扣子访问令牌失败: "+err.Error())
			return
		}
		accessToken, _ := token["access_token"].(string)
		expiresIn, _ := token["expires_in"]
		response.Success(c, gin.H{
			"token":      accessToken,
			"project_id": cfg.Coze.ProjectID,
			"expires_in": expiresIn,
		})
	})
}
