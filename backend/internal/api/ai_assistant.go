// Package api 实现 HTTP handlers。
// 本文件：AI 助手模块（会话管理 + 流式对话）Handler 与路由注册。
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// AIAssistantHandler AI 助手模块 Handler。
type AIAssistantHandler struct {
	svc       *service.AIAssistantService
	pointsSvc *service.PointsService
}

// NewAIAssistantHandler 构造 AIAssistantHandler。
func NewAIAssistantHandler(svc *service.AIAssistantService, pointsSvc *service.PointsService) *AIAssistantHandler {
	return &AIAssistantHandler{svc: svc, pointsSvc: pointsSvc}
}

// RegisterAIAssistantRoutes 注册 /api/ai-assistant 路由。
// 公开路由：GET /models、POST /chat（可选认证）。
// 登录路由：sessions CRUD、user-models CRUD（强制 middleware.JWTAuth + role=hrwai_user）。
func RegisterAIAssistantRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.AIAssistantService, pointsSvc *service.PointsService) {
	h := NewAIAssistantHandler(svc, pointsSvc)

	g := rg.Group("/ai-assistant")

	// 公开路由：列出管理员配置的可用模型（未登录可访问）
	g.GET("/models", h.ListPublicModels)
	g.GET("/modes", h.ListAssistantModes)
	// 流式对话：可选认证（未登录可临时对话，登录则可保存会话）
	g.POST("/chat", middleware.OptionalAuth(rd.Session), h.StreamChat)
	// 对话图片上传：可选认证（与 chat 一致，游客可上传）
	g.POST("/upload-image", middleware.OptionalAuth(rd.Session), h.UploadImage)

	// 需登录路由：会话管理 + 用户自定义模型管理（HRWAI 账号鉴权）
	authed := g.Group("")
	authed.Use(middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))
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

// ListPublicModels 可用模型列表
// @Summary AI 可用模型（公开）
// @Description 列出管理员配置的 is_active=true 模型
// @Tags 学员端-AI助手
// @Produce json
// @Success 200 {object} response.R "success"
// @Router /ai-assistant/models [get]
func (h *AIAssistantHandler) ListPublicModels(c *gin.Context) {
	Endpoint[struct{}, []service.ModelOption]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.ModelOption, error) {
			models, err := h.svc.ListPublicModels(ctx)
			if err != nil {
				return nil, err
			}
			return &models, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.ModelOption, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// ListAssistantModes 双模式可用模型（新）
// @Summary AI 双模式模型（公开）
// @Description 返回普通/专家分别绑定的可用模型（隐藏底层 model 细节，前端仅暴露模式）
// @Tags 学员端-AI助手
// @Produce json
// @Success 200 {object} response.R "success"
// @Router /ai-assistant/modes [get]
func (h *AIAssistantHandler) ListAssistantModes(c *gin.Context) {
	Endpoint[struct{}, service.AIAssistantModeModels]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.AIAssistantModeModels, error) {
			modes, err := h.svc.ListAssistantModes(ctx)
			if err != nil {
				return nil, err
			}
			return &modes, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.AIAssistantModeModels, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// ListUserModels 用户自定义模型列表
// @Summary 用户自定义模型
// @Description 列出登录用户自定义模型（api_key 脱敏）
// @Tags 学员端-AI助手
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /ai-assistant/user-models [get]
func (h *AIAssistantHandler) ListUserModels(c *gin.Context) {
	Endpoint[aiUserIDReq, []service.UserModelDTO]{
		Parse: func(c *gin.Context) (*aiUserIDReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			return &aiUserIDReq{UserID: uid}, nil
		},
		Invoke: func(ctx context.Context, req *aiUserIDReq) (*[]service.UserModelDTO, error) {
			models, err := h.svc.ListUserModels(c.Request.Context(), req.UserID)
			if err != nil {
				return nil, err
			}
			return &models, nil
		},
		Render: func(c *gin.Context, _ *aiUserIDReq, resp *[]service.UserModelDTO, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// SaveUserModel 保存用户模型
// @Summary 保存用户自定义模型
// @Description 创建或更新用户自定义模型
// @Tags 学员端-AI助手
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "模型" example({"name":"my-model","api_key":"sk-...","base_url":"https://api.example.com","model":"gpt-4"})
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /ai-assistant/user-models [post]
func (h *AIAssistantHandler) SaveUserModel(c *gin.Context) {
	Endpoint[aiUserModelSaveReq, struct{}]{
		Parse: func(c *gin.Context) (*aiUserModelSaveReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			req, err := bindJSONMsg[service.SaveUserModelReq](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			if req.Name == "" || req.APIKey == "" || req.BaseURL == "" || req.Model == "" {
				return nil, badRequest("name/api_key/base_url/model 均为必填")
			}
			return &aiUserModelSaveReq{UserID: uid, Req: *req}, nil
		},
		Invoke: func(ctx context.Context, req *aiUserModelSaveReq) (*struct{}, error) {
			if err := h.svc.SaveUserModel(c.Request.Context(), req.UserID, req.Req); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *aiUserModelSaveReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, nil)
		},
	}.Handle(c)
}

// DeleteUserModel 删除用户模型
// @Summary 删除用户自定义模型
// @Description 删除指定用户模型
// @Tags 学员端-AI助手
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "模型ID"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "不存在"
// @Router /ai-assistant/user-models/{id} [delete]
func (h *AIAssistantHandler) DeleteUserModel(c *gin.Context) {
	Endpoint[aiModelIDReq, struct{}]{
		Parse: func(c *gin.Context) (*aiModelIDReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			id, _ := strconv.Atoi(c.Param("id"))
			if id <= 0 {
				return nil, badRequest("无效的模型 ID")
			}
			return &aiModelIDReq{UserID: uid, ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *aiModelIDReq) (*struct{}, error) {
			if err := h.svc.DeleteUserModel(c.Request.Context(), req.UserID, req.ID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *aiModelIDReq, _ *struct{}, err error) {
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					response.NotFound(c, "模型不存在")
					return
				}
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, nil)
		},
	}.Handle(c)
}

// ListSessions 会话列表
// @Summary AI 会话列表
// @Description 列出登录用户的会话
// @Tags 学员端-AI助手
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /ai-assistant/sessions [get]
func (h *AIAssistantHandler) ListSessions(c *gin.Context) {
	Endpoint[aiUserIDReq, []service.AIChatSessionDTO]{
		Parse: func(c *gin.Context) (*aiUserIDReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			return &aiUserIDReq{UserID: uid}, nil
		},
		Invoke: func(ctx context.Context, req *aiUserIDReq) (*[]service.AIChatSessionDTO, error) {
			sessions, err := h.svc.ListSessions(c.Request.Context(), req.UserID, c.Query("feature_key"))
			if err != nil {
				return nil, err
			}
			return &sessions, nil
		},
		Render: func(c *gin.Context, _ *aiUserIDReq, resp *[]service.AIChatSessionDTO, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// CreateSession 创建会话
// @Summary 创建 AI 会话
// @Description 创建新会话
// @Tags 学员端-AI助手
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object false "标题/模型" example({"title":"新对话","model_name":"gpt-4"})
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /ai-assistant/sessions [post]
func (h *AIAssistantHandler) CreateSession(c *gin.Context) {
	Endpoint[aiSessionCreateReq, service.AIChatSessionDTO]{
		Parse: func(c *gin.Context) (*aiSessionCreateReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			var body struct {
				Title      string `json:"title"`
				ModelName  string `json:"model_name"`
				FeatureKey string `json:"feature_key"`
			}
			_ = c.ShouldBindJSON(&body)
			return &aiSessionCreateReq{UserID: uid, Title: body.Title, ModelName: body.ModelName, FeatureKey: body.FeatureKey}, nil
		},
		Invoke: func(ctx context.Context, req *aiSessionCreateReq) (*service.AIChatSessionDTO, error) {
			return h.svc.CreateSession(c.Request.Context(), req.UserID, req.Title, req.ModelName, req.FeatureKey)
		},
		Render: func(c *gin.Context, _ *aiSessionCreateReq, resp *service.AIChatSessionDTO, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// DeleteSession 删除会话
// @Summary 删除会话
// @Description 删除会话及其消息
// @Tags 学员端-AI助手
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "会话ID"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "不存在"
// @Router /ai-assistant/sessions/{id} [delete]
func (h *AIAssistantHandler) DeleteSession(c *gin.Context) {
	Endpoint[aiModelIDReq, struct{}]{
		Parse: func(c *gin.Context) (*aiModelIDReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			id, _ := strconv.Atoi(c.Param("id"))
			if id <= 0 {
				return nil, badRequest("无效的会话 ID")
			}
			return &aiModelIDReq{UserID: uid, ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *aiModelIDReq) (*struct{}, error) {
			if err := h.svc.DeleteSession(c.Request.Context(), req.UserID, req.ID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *aiModelIDReq, _ *struct{}, err error) {
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					response.NotFound(c, "会话不存在")
					return
				}
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, nil)
		},
	}.Handle(c)
}

// RenameSession 重命名会话
// @Summary 重命名会话
// @Description 修改会话标题，非空最多 100 字符
// @Tags 学员端-AI助手
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "会话ID"
// @Param body body object true "标题" example({"title":"新标题"})
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /ai-assistant/sessions/{id}/title [patch]
func (h *AIAssistantHandler) RenameSession(c *gin.Context) {
	Endpoint[aiSessionRenameReq, struct{}]{
		Parse: func(c *gin.Context) (*aiSessionRenameReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			id, _ := strconv.Atoi(c.Param("id"))
			if id <= 0 {
				return nil, badRequest("无效的会话 ID")
			}
			req, err := bindJSONMsg[aiSessionRenameReqBody](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			return &aiSessionRenameReq{UserID: uid, ID: id, Title: req.Title}, nil
		},
		Invoke: func(ctx context.Context, req *aiSessionRenameReq) (*struct{}, error) {
			if err := h.svc.RenameSession(c.Request.Context(), req.UserID, req.ID, req.Title); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *aiSessionRenameReq, _ *struct{}, err error) {
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					response.NotFound(c, "会话不存在")
					return
				}
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, map[string]string{"message": "已更新会话标题"})
		},
	}.Handle(c)
}

// GetSessionMessages 会话消息
// @Summary 会话消息列表
// @Description 获取指定会话的消息列表
// @Tags 学员端-AI助手
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "会话ID"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "不存在"
// @Router /ai-assistant/sessions/{id}/messages [get]
func (h *AIAssistantHandler) GetSessionMessages(c *gin.Context) {
	Endpoint[aiModelIDReq, []service.AIChatMessageDTO]{
		Parse: func(c *gin.Context) (*aiModelIDReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			id, _ := strconv.Atoi(c.Param("id"))
			if id <= 0 {
				return nil, badRequest("无效的会话 ID")
			}
			return &aiModelIDReq{UserID: uid, ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *aiModelIDReq) (*[]service.AIChatMessageDTO, error) {
			msgs, err := h.svc.GetSessionMessages(c.Request.Context(), req.UserID, req.ID)
			if err != nil {
				return nil, err
			}
			return &msgs, nil
		},
		Render: func(c *gin.Context, _ *aiModelIDReq, resp *[]service.AIChatMessageDTO, err error) {
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					response.NotFound(c, "会话不存在")
					return
				}
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// StreamChat 流式对话
// @Summary AI 流式对话（SSE）
// @Description 可选认证的 SSE 流式响应，不走统一 JSON 信封；事件 message/error/done
// @Tags 学员端-AI助手
// @Accept json
// @Produce text/event-stream
// @Param body body object true "消息" example({"feature_key":"fault_consult","messages":[{"role":"user","content":"你好"}]})
// @Success 200 {string} string "SSE stream"
// @Failure 400 {object} response.R "参数错误"
// @Router /ai-assistant/chat [post]
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
	// 最后一条用户消息须有文本或图片（支持纯图片提问）
	last := req.Messages[len(req.Messages)-1]
	if last.Role != "user" || (strings.TrimSpace(last.Content) == "" && len(last.Images) == 0) {
		response.BadRequest(c, "消息不能为空")
		return
	}

	// 积分预检：已登录用户需余额 >=5 否则阻断
	if userID > 0 && h.pointsSvc != nil {
		if bal, err := h.pointsSvc.GetBalance(userID); err == nil && bal.Balance < 5 {
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			c.Writer.Header().Set("Cache-Control", "no-cache")
			c.Writer.Header().Set("Connection", "keep-alive")
			c.Writer.Header().Set("X-Accel-Buffering", "no")
			c.Writer.WriteHeader(http.StatusOK)
			payload, _ := json.Marshal(map[string]string{"message": "积分不足，请先去任务中心完成任务"})
			_, _ = c.Writer.WriteString("event: error\n")
			_, _ = c.Writer.WriteString("data: " + string(payload) + "\n\n")
			c.Writer.Flush()
			return
		}
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

	fullContent, err := h.svc.StreamChat(c.Request.Context(), userID, req, func(content string) {
		sendEvent("message", map[string]string{"content": content})
	})

	if err != nil {
		sendEvent("error", map[string]string{"message": err.Error()})
		return
	}
	// 后计量扣费：已登录用户按 tokens 扣分，末尾附消耗
	if userID > 0 && h.pointsSvc != nil && fullContent != "" {
		requestID := fmt.Sprintf("ai-%d-%d", userID, time.Now().UnixNano())
		// 估算 tokens：prompt+completion，简化为 content 长度/4 + 最后用户消息长度/4
		promptLen := 0
		if len(req.Messages) > 0 {
			promptLen = len(req.Messages[len(req.Messages)-1].Content)
		}
		tokens := (len(fullContent) + promptLen + 3) / 4
		if tokens < 10 {
			tokens = 100
		}
		if points, balance, err := h.pointsSvc.DeductAI(c.Request.Context(), userID, requestID, tokens, fullContent); err == nil {
			sendEvent("usage", map[string]any{
				"points_cost":       points,
				"total_tokens":      tokens,
				"balance":           balance,
				"prompt_tokens":     (promptLen + 3) / 4,
				"completion_tokens": (len(fullContent) + 3) / 4,
			})
		} else if err.Error() == "积分不足" {
			sendEvent("error", map[string]string{"message": "积分不足"})
		}
	}
	sendEvent("done", nil)
}

// UploadImage 上传对话图片
// @Summary 上传 AI 对话图片
// @Description 可选认证；校验格式/大小后保存，返回可访问 URL（随消息提交）
// @Tags 学员端-AI助手
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "图片文件"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Router /ai-assistant/upload-image [post]
func (h *AIAssistantHandler) UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "未找到上传文件")
		return
	}
	url, err := h.svc.UploadImage(c.Request.Context(), file)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "图片上传成功", gin.H{"url": url})
}

// ===== typed request structs =====

// aiUserIDReq 仅带登录用户 ID 的请求。
type aiUserIDReq struct {
	UserID int
}

// aiModelIDReq 带用户 ID + 路径 id 的请求。
type aiModelIDReq struct {
	UserID int
	ID     int
}

// aiUserModelSaveReq 保存用户模型请求（含用户 ID + service 请求体）。
type aiUserModelSaveReq struct {
	UserID int
	Req    service.SaveUserModelReq
}

// aiSessionCreateReq 创建会话请求。
type aiSessionCreateReq struct {
	UserID     int
	Title      string
	ModelName  string
	FeatureKey string
}

// aiSessionRenameReq 重命名会话请求。
type aiSessionRenameReq struct {
	UserID int
	ID     int
	Title  string
}

// aiSessionRenameReqBody 重命名会话请求体。
type aiSessionRenameReqBody struct {
	Title string `json:"title"`
}
