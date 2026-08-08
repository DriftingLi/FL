// Package api 实现 HTTP handlers。
// 本文件：AI 助手模块（会话管理 + 流式对话）Handler 与路由注册。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// AIAssistantHandler AI 助手模块 Handler。
type AIAssistantHandler struct {
	svc *service.AIAssistantService
	cfg *config.Config
}

// NewAIAssistantHandler 构造 AIAssistantHandler。
func NewAIAssistantHandler(svc *service.AIAssistantService, cfg *config.Config) *AIAssistantHandler {
	return &AIAssistantHandler{svc: svc, cfg: cfg}
}

// RegisterAIAssistantRoutes 注册 /api/ai-assistant 路由。
// 公开路由：GET /models、POST /chat（可选认证）。
// 登录路由：sessions CRUD、user-models CRUD（强制 middleware.JWTAuth + role=hrwai_user）。
func RegisterAIAssistantRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB, aiConfigSvc *service.AIConfigService, logger *zap.Logger) {
	assistantSvc := service.NewAIAssistantService(db, aiConfigSvc, cfg.SecretKey, logger)
	h := NewAIAssistantHandler(assistantSvc, cfg)

	g := rg.Group("/ai-assistant")

	// 公开路由：列出管理员配置的可用模型（未登录可访问）
	g.GET("/models", h.ListPublicModels)
	// 流式对话：可选认证（未登录可临时对话，登录则可保存会话）
	g.POST("/chat", middleware.OptionalAuth(cfg), h.StreamChat)

	// 需登录路由：会话管理 + 用户自定义模型管理（HRWAI 账号鉴权）
	authed := g.Group("")
	authed.Use(middleware.JWTAuth(cfg), middleware.RoleRequired("hrwai_user"))
	authed.GET("/sessions", h.ListSessions)
	authed.POST("/sessions", h.CreateSession)
	authed.DELETE("/sessions/:id", h.DeleteSession)
	authed.PATCH("/sessions/:id/title", h.RenameSession)
	authed.GET("/sessions/:id/messages", h.GetSessionMessages)
	authed.GET("/user-models", h.ListUserModels)
	authed.POST("/user-models", h.SaveUserModel)
	authed.DELETE("/user-models/:id", h.DeleteUserModel)
}

// ===== Handler 方法 =====

// ListPublicModels 列出管理员配置的 is_active=true 模型（公开）。
func (h *AIAssistantHandler) ListPublicModels(c *gin.Context) {
	models, err := h.svc.ListPublicModels(c.Request.Context())
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, models)
}

// ListUserModels 列出登录用户的自定义模型（api_key 脱敏）。
func (h *AIAssistantHandler) ListUserModels(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	models, err := h.svc.ListUserModels(c.Request.Context(), userID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, models)
}

// SaveUserModel 创建/更新用户自定义模型。
func (h *AIAssistantHandler) SaveUserModel(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req service.SaveUserModelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	if req.Name == "" || req.APIKey == "" || req.BaseURL == "" || req.Model == "" {
		response.BadRequest(c, "name/api_key/base_url/model 均为必填")
		return
	}
	if err := h.svc.SaveUserModel(c.Request.Context(), userID, req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// DeleteUserModel 删除用户自定义模型。
func (h *AIAssistantHandler) DeleteUserModel(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	modelID, _ := strconv.Atoi(c.Param("id"))
	if modelID <= 0 {
		response.BadRequest(c, "无效的模型 ID")
		return
	}
	if err := h.svc.DeleteUserModel(c.Request.Context(), userID, modelID); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "模型不存在")
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ListSessions 列出登录用户的会话。
func (h *AIAssistantHandler) ListSessions(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	sessions, err := h.svc.ListSessions(c.Request.Context(), userID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, sessions)
}

// CreateSession 创建会话。
func (h *AIAssistantHandler) CreateSession(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req struct {
		Title     string `json:"title"`
		ModelName string `json:"model_name"`
	}
	_ = c.ShouldBindJSON(&req)
	session, err := h.svc.CreateSession(c.Request.Context(), userID, req.Title, req.ModelName)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, session)
}

// DeleteSession 删除会话及其消息。
func (h *AIAssistantHandler) DeleteSession(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	sessionID, _ := strconv.Atoi(c.Param("id"))
	if sessionID <= 0 {
		response.BadRequest(c, "无效的会话 ID")
		return
	}
	if err := h.svc.DeleteSession(c.Request.Context(), userID, sessionID); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "会话不存在")
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// RenameSession 修改会话标题。
// Body: {"title": "新标题"}；标题非空，最多 100 字符。
func (h *AIAssistantHandler) RenameSession(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	sessionID, _ := strconv.Atoi(c.Param("id"))
	if sessionID <= 0 {
		response.BadRequest(c, "无效的会话 ID")
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	if err := h.svc.RenameSession(c.Request.Context(), userID, sessionID, req.Title); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "会话不存在")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, map[string]string{"message": "已更新会话标题"})
}

// GetSessionMessages 获取指定会话的消息列表。
func (h *AIAssistantHandler) GetSessionMessages(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	sessionID, _ := strconv.Atoi(c.Param("id"))
	if sessionID <= 0 {
		response.BadRequest(c, "无效的会话 ID")
		return
	}
	messages, err := h.svc.GetSessionMessages(c.Request.Context(), userID, sessionID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "会话不存在")
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, messages)
}

// StreamChat 流式对话（SSE 推送）。
// 事件类型：
//   - message: 增量内容 {"content": "..."}
//   - error:   错误信息 {"message": "..."}
//   - done:    正常结束（无 data）
func (h *AIAssistantHandler) StreamChat(c *gin.Context) {
	userID := middleware.CurrentUserID(c) // 可选认证，未登录为 0

	var req service.StreamChatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	if len(req.Messages) == 0 {
		response.BadRequest(c, "消息不能为空")
		return
	}

	// 设置 SSE 响应头
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // 禁用 nginx 缓冲
	c.Writer.WriteHeader(http.StatusOK)

	sendEvent := func(event string, data any) {
		payload, _ := json.Marshal(data)
		_, _ = c.Writer.WriteString("event: " + event + "\n")
		_, _ = c.Writer.WriteString("data: " + string(payload) + "\n\n")
		c.Writer.Flush()
	}

	_, err := h.svc.StreamChat(c.Request.Context(), userID, req, func(content string) {
		sendEvent("message", map[string]string{"content": content})
	})

	if err != nil {
		sendEvent("error", map[string]string{"message": err.Error()})
		return
	}
	sendEvent("done", nil)
}
