// Package api 实现 HTTP handlers。
package api

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// registerSettingsRoutes 注册 /admin/ai-configs/* 与 /admin/ai-feature-bindings/* 子路由组。
// 必须挂在 admin 路由组下（已应用 JWTAuth + RoleRequired("admin")）。
func registerSettingsRoutes(g *gin.RouterGroup, aiConfigSvc *service.AIConfigService, db *gorm.DB) {
	// ===== AI 多配置管理 =====
	cfg := g.Group("/ai-configs")

	// GET /api/admin/ai-configs  列出所有配置（API Key 脱敏）
	cfg.GET("", func(c *gin.Context) {
		list, err := aiConfigSvc.ListConfigs(c.Request.Context())
		if err != nil {
			response.ServerError(c, "查询失败: "+err.Error())
			return
		}
		response.Success(c, list)
	})

	// POST /api/admin/ai-configs  新建配置
	cfg.POST("", func(c *gin.Context) {
		var req struct {
			Name        string `json:"name" binding:"required"`
			APIKey      string `json:"api_key" binding:"required"`
			BaseURL     string `json:"base_url" binding:"required"`
			Model       string `json:"model" binding:"required"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误: "+err.Error())
			return
		}
		if err := aiConfigSvc.CreateConfig(c.Request.Context(), req.Name, req.APIKey, req.BaseURL, req.Model, req.Description); err != nil {
			response.ServerError(c, "创建失败: "+err.Error())
			return
		}
		response.SuccessWithMsg(c, "配置已创建", nil)
	})

	// PUT /api/admin/ai-configs/:id  更新配置（api_key 为空表示不修改）
	cfg.PUT("/:id", func(c *gin.Context) {
		cfgID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "无效的 id")
			return
		}
		var req struct {
			Name        string `json:"name" binding:"required"`
			APIKey      string `json:"api_key"`
			BaseURL     string `json:"base_url" binding:"required"`
			Model       string `json:"model" binding:"required"`
			Description string `json:"description"`
			IsActive    *bool  `json:"is_active"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误: "+err.Error())
			return
		}
		if err := aiConfigSvc.UpdateConfig(c.Request.Context(), cfgID, req.Name, req.APIKey, req.BaseURL, req.Model, req.Description, req.IsActive); err != nil {
			response.ServerError(c, "更新失败: "+err.Error())
			return
		}
		response.SuccessWithMsg(c, "配置已更新", nil)
	})

	// DELETE /api/admin/ai-configs/:id  删除配置（被绑定时拒绝）
	cfg.DELETE("/:id", func(c *gin.Context) {
		cfgID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "无效的 id")
			return
		}
		if err := aiConfigSvc.DeleteConfig(c.Request.Context(), cfgID); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "配置已删除", nil)
	})

	// POST /api/admin/ai-configs/:id/test  测试指定配置的连通性
	cfg.POST("/:id/test", func(c *gin.Context) {
		cfgID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "无效的 id")
			return
		}
		var row model.AIConfig
		if err := db.First(&row, cfgID).Error; err != nil {
			response.BadRequest(c, "配置不存在")
			return
		}
		oc := openai.DefaultConfig(row.APIKey)
		if row.BaseURL != "" {
			oc.BaseURL = row.BaseURL
		}
		client := openai.NewClientWithConfig(oc)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		_, err = client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model: row.Model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: "请回复 'OK'"},
			},
			MaxTokens: 10,
		})
		if err != nil {
			response.BadRequest(c, "连接失败: "+err.Error())
			return
		}
		response.SuccessWithMsg(c, "连接成功", nil)
	})

	// ===== 功能绑定 =====
	bind := g.Group("/ai-feature-bindings")

	// GET /api/admin/ai-feature-bindings  列出所有 AI 功能的绑定情况
	bind.GET("", func(c *gin.Context) {
		list, err := aiConfigSvc.ListBindings(c.Request.Context())
		if err != nil {
			response.ServerError(c, "查询失败: "+err.Error())
			return
		}
		response.Success(c, list)
	})

	// PUT /api/admin/ai-feature-bindings/:feature_key  绑定功能到指定配置
	// Body: {"config_id": 1}；config_id=0 表示解除绑定（单绑定清空，多绑定清空所有）
	bind.PUT("/:feature_key", func(c *gin.Context) {
		featureKey := c.Param("feature_key")
		var req struct {
			ConfigID int `json:"config_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误: "+err.Error())
			return
		}
		if err := aiConfigSvc.SetBinding(c.Request.Context(), featureKey, req.ConfigID); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "绑定已更新", nil)
	})

	// DELETE /api/admin/ai-feature-bindings/:feature_key/configs/:config_id  解除多绑定功能的单个配置绑定
	bind.DELETE("/:feature_key/configs/:config_id", func(c *gin.Context) {
		featureKey := c.Param("feature_key")
		cfgID, err := strconv.Atoi(c.Param("config_id"))
		if err != nil {
			response.BadRequest(c, "无效的 config_id")
			return
		}
		if err := aiConfigSvc.UnbindConfig(c.Request.Context(), featureKey, cfgID); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "已解除绑定", nil)
	})
}
