// Package api 实现 HTTP handlers。
package api

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"

	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// AIConfigHandler AI 配置管理 handler。
type AIConfigHandler struct {
	svc *service.AIConfigService
}

// NewAIConfigHandler 创建 AI 配置管理 handler。
func NewAIConfigHandler(svc *service.AIConfigService) *AIConfigHandler {
	return &AIConfigHandler{svc: svc}
}

// registerAIConfigRoutes 注册 /admin/ai-configs/* 与 /admin/ai-feature-bindings/* 子路由组。
// 必须挂在 admin 路由组下（已应用 JWTAuth + RoleRequired("admin")）。
func (h *AIConfigHandler) registerAIConfigRoutes(g *gin.RouterGroup) {
	// ===== AI 多配置管理 =====
	cfg := g.Group("/ai-configs")
	cfg.GET("", h.ListConfigs)
	cfg.POST("", h.CreateConfig)
	cfg.PUT("/:id", h.UpdateConfig)
	cfg.DELETE("/:id", h.DeleteConfig)
	cfg.POST("/:id/test", h.TestConfig)

	// ===== 功能绑定 =====
	bind := g.Group("/ai-feature-bindings")
	bind.GET("", h.ListBindings)
	bind.PUT("/:feature_key", h.SetBinding)
	bind.DELETE("/:feature_key/configs/:config_id", h.UnbindConfig)
}

// ListConfigs 列出所有配置（API Key 脱敏）GET /api/admin/ai-configs
func (h *AIConfigHandler) ListConfigs(c *gin.Context) {
	Endpoint[struct{}, []service.AIConfigDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.AIConfigDTO, error) {
			list, err := h.svc.ListConfigs(ctx)
			if err != nil {
				return nil, err
			}
			return &list, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.AIConfigDTO, err error) {
			if err != nil {
				response.ServerError(c, "查询失败: "+err.Error())
				return
			}
			response.Success(c, *resp)
		},
	}.Handle(c)
}

// CreateConfig 新建配置 POST /api/admin/ai-configs
func (h *AIConfigHandler) CreateConfig(c *gin.Context) {
	Endpoint[createConfigReq, struct{}]{
		Parse: func(c *gin.Context) (*createConfigReq, error) {
			var req struct {
				Name        string `json:"name" binding:"required"`
				APIKey      string `json:"api_key" binding:"required"`
				BaseURL     string `json:"base_url" binding:"required"`
				Model       string `json:"model" binding:"required"`
				Description string `json:"description"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, badRequest("请求参数错误: " + err.Error())
			}
			return &createConfigReq{Name: req.Name, APIKey: req.APIKey, BaseURL: req.BaseURL, Model: req.Model, Description: req.Description}, nil
		},
		Invoke: func(ctx context.Context, req *createConfigReq) (*struct{}, error) {
			if err := h.svc.CreateConfig(ctx, req.Name, req.APIKey, req.BaseURL, req.Model, req.Description); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *createConfigReq, _ *struct{}, err error) {
			if err != nil {
				var pe *ParseError
				if asParseError(err, &pe) {
					renderStatus(c, pe.Status, pe.Message)
					return
				}
				response.ServerError(c, "创建失败: "+err.Error())
				return
			}
			response.SuccessWithMsg(c, "配置已创建", nil)
		},
	}.Handle(c)
}

// UpdateConfig 更新配置（api_key 为空表示不修改）PUT /api/admin/ai-configs/:id
func (h *AIConfigHandler) UpdateConfig(c *gin.Context) {
	Endpoint[updateConfigReq, struct{}]{
		Parse: func(c *gin.Context) (*updateConfigReq, error) {
			id, err := pathInt(c, "id", "无效的 id")
			if err != nil {
				return nil, err
			}
			var body struct {
				Name        string `json:"name" binding:"required"`
				APIKey      string `json:"api_key"`
				BaseURL     string `json:"base_url" binding:"required"`
				Model       string `json:"model" binding:"required"`
				Description string `json:"description"`
				IsActive    *bool  `json:"is_active"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("请求参数错误: " + err.Error())
			}
			return &updateConfigReq{ID: id, Name: body.Name, APIKey: body.APIKey, BaseURL: body.BaseURL, Model: body.Model, Description: body.Description, IsActive: body.IsActive}, nil
		},
		Invoke: func(ctx context.Context, req *updateConfigReq) (*struct{}, error) {
			if err := h.svc.UpdateConfig(ctx, req.ID, req.Name, req.APIKey, req.BaseURL, req.Model, req.Description, req.IsActive); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *updateConfigReq, _ *struct{}, err error) {
			if err != nil {
				var pe *ParseError
				if asParseError(err, &pe) {
					renderStatus(c, pe.Status, pe.Message)
					return
				}
				response.ServerError(c, "更新失败: "+err.Error())
				return
			}
			response.SuccessWithMsg(c, "配置已更新", nil)
		},
	}.Handle(c)
}

// DeleteConfig 删除配置（被绑定时拒绝）DELETE /api/admin/ai-configs/:id
func (h *AIConfigHandler) DeleteConfig(c *gin.Context) {
	Endpoint[idParam, struct{}]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "id", "无效的 id")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*struct{}, error) {
			if err := h.svc.DeleteConfig(ctx, req.ID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *idParam, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "配置已删除", nil)
		},
	}.Handle(c)
}

// TestConfig 测试指定配置的连通性 POST /api/admin/ai-configs/:id/test
func (h *AIConfigHandler) TestConfig(c *gin.Context) {
	cfgID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 id")
		return
	}
	row, err := h.svc.GetConfigByID(c.Request.Context(), cfgID)
	if err != nil {
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
}

// ListBindings 列出所有 AI 功能的绑定情况 GET /api/admin/ai-feature-bindings
func (h *AIConfigHandler) ListBindings(c *gin.Context) {
	Endpoint[struct{}, []service.FeatureBindingDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.FeatureBindingDTO, error) {
			list, err := h.svc.ListBindings(ctx)
			if err != nil {
				return nil, err
			}
			return &list, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.FeatureBindingDTO, err error) {
			if err != nil {
				response.ServerError(c, "查询失败: "+err.Error())
				return
			}
			response.Success(c, *resp)
		},
	}.Handle(c)
}

// SetBinding 绑定功能到指定配置 PUT /api/admin/ai-feature-bindings/:feature_key
// Body: {"config_id": 1}；config_id=0 表示解除绑定（单绑定清空，多绑定清空所有）
func (h *AIConfigHandler) SetBinding(c *gin.Context) {
	Endpoint[setBindingReq, struct{}]{
		Parse: func(c *gin.Context) (*setBindingReq, error) {
			var body struct {
				ConfigID int `json:"config_id"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("请求参数错误: " + err.Error())
			}
			return &setBindingReq{FeatureKey: c.Param("feature_key"), ConfigID: body.ConfigID}, nil
		},
		Invoke: func(ctx context.Context, req *setBindingReq) (*struct{}, error) {
			if err := h.svc.SetBinding(ctx, req.FeatureKey, req.ConfigID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *setBindingReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "绑定已更新", nil)
		},
	}.Handle(c)
}

// UnbindConfig 解除多绑定功能的单个配置绑定 DELETE /api/admin/ai-feature-bindings/:feature_key/configs/:config_id
func (h *AIConfigHandler) UnbindConfig(c *gin.Context) {
	Endpoint[unbindConfigReq, struct{}]{
		Parse: func(c *gin.Context) (*unbindConfigReq, error) {
			id, err := pathInt(c, "config_id", "无效的 config_id")
			if err != nil {
				return nil, err
			}
			return &unbindConfigReq{FeatureKey: c.Param("feature_key"), ConfigID: id}, nil
		},
		Invoke: func(ctx context.Context, req *unbindConfigReq) (*struct{}, error) {
			if err := h.svc.UnbindConfig(ctx, req.FeatureKey, req.ConfigID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *unbindConfigReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "已解除绑定", nil)
		},
	}.Handle(c)
}

// ===== Endpoint 请求类型 =====

// createConfigReq 新建 AI 配置请求体。
type createConfigReq struct {
	Name        string
	APIKey      string
	BaseURL     string
	Model       string
	Description string
}

// updateConfigReq 更新 AI 配置请求（路径 id + body）。
type updateConfigReq struct {
	ID          int
	Name        string
	APIKey      string
	BaseURL     string
	Model       string
	Description string
	IsActive    *bool
}

// setBindingReq 设置功能绑定请求（路径 feature_key + body config_id）。
type setBindingReq struct {
	FeatureKey string
	ConfigID   int
}

// unbindConfigReq 解除配置绑定请求（路径 feature_key + config_id）。
type unbindConfigReq struct {
	FeatureKey string
	ConfigID   int
}
