// 本文件：AI 助手模块（会话管理 + 流式对话）Handler 与路由注册。
package aiassistant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/points"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// handler AI 助手模块 Handler。
// 计量闸门内移模型端口（ADR-0031）：本 Handler 无预检/扣费编排、无 prompt 事实选取，
// 只透传请求标识与转发 SSE 事件——积分域依赖不再进入 HTTP 层。
type handler struct {
	svc *Service
}

// newHandler 构造 handler。
func newHandler(svc *Service) *handler {
	return &handler{svc: svc}
}

// RegisterRoutes 注册 /api/ai-assistant 蓝图（公开面 + 登录面）与诊断只读代理子组。
// 公开路由：GET /models、GET /modes、POST /chat、POST /upload-image（可选认证）。
// 登录路由：sessions CRUD、user-models CRUD（JWTAuth + CapabilityRequired(authz.CapAIAssistantUse)）。
// 诊断子组（/diagnosis/*）沿用 OptionalAuth，见 handler_diagnosis.go。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service, proxy *DiagnosisProxyService) {
	h := newHandler(svc)

	g := rg.Group("/ai-assistant")

	// 公开路由：列出管理员配置的可用模型（未登录可访问）
	g.GET("/models", h.ListPublicModels)
	g.GET("/modes", h.ListAssistantModes)
	// 流式对话：可选认证（未登录可临时对话，登录则可保存会话）
	g.POST("/chat", middleware.OptionalAuth(session), h.StreamChat)
	// 对话图片上传：可选认证（与 chat 一致，游客可上传）
	g.POST("/upload-image", middleware.OptionalAuth(session), h.UploadImage)

	// 需登录路由：会话管理 + 用户自定义模型管理（HRWAI 账号鉴权）
	authed := g.Group("")
	authed.Use(middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapAIAssistantUse))
	authed.GET("/sessions", h.ListSessions)
	authed.POST("/sessions", h.CreateSession)
	authed.DELETE("/sessions/:id", h.DeleteSession)
	authed.PATCH("/sessions/:id/title", h.RenameSession)
	authed.GET("/sessions/:id/messages", h.GetSessionMessages)
	authed.GET("/user-models", h.ListUserModels)
	authed.POST("/user-models", h.SaveUserModel)
	authed.DELETE("/user-models/:id", h.DeleteUserModel)

	registerDiagnosisRoutes(g, session, proxy)
}

// ===== Handler 方法 =====

// ListPublicModels 可用模型列表
// @Summary AI 可用模型（公开）
// @Description 列出管理员配置的 is_active=true 模型
// @Tags 学员端-AI助手
// @Produce json
// @Success 200 {object} response.R{data=[]aiassistant.ModelOption} "success"
// @Router /ai-assistant/models [get]
func (h *handler) ListPublicModels(c *gin.Context) {
	httpx.Endpoint[struct{}, []ModelOption]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]ModelOption, error) {
			models, err := h.svc.ListPublicModels(ctx)
			if err != nil {
				return nil, err
			}
			return &models, nil
		},
	}.Handle(c)
}

// ListAssistantModes 双模式可用模型（新）
// @Summary AI 双模式模型（公开）
// @Description 返回普通/专家分别绑定的可用模型（隐藏底层 model 细节，前端仅暴露模式）
// @Tags 学员端-AI助手
// @Produce json
// @Success 200 {object} response.R{data=aiassistant.AIAssistantModeModels} "success"
// @Router /ai-assistant/modes [get]
func (h *handler) ListAssistantModes(c *gin.Context) {
	httpx.Endpoint[struct{}, AIAssistantModeModels]{
		Invoke: func(ctx context.Context, _ *struct{}) (*AIAssistantModeModels, error) {
			modes, err := h.svc.ListAssistantModes(ctx)
			if err != nil {
				return nil, err
			}
			return &modes, nil
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
// @Success 200 {object} response.R{data=[]aiassistant.UserModelDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /ai-assistant/user-models [get]
func (h *handler) ListUserModels(c *gin.Context) {
	httpx.Endpoint[aiUserIDReq, []UserModelDTO]{
		Parse: func(c *gin.Context) (*aiUserIDReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &httpx.ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			return &aiUserIDReq{UserID: uid}, nil
		},
		Invoke: func(ctx context.Context, req *aiUserIDReq) (*[]UserModelDTO, error) {
			models, err := h.svc.ListUserModels(c.Request.Context(), req.UserID)
			if err != nil {
				return nil, err
			}
			return &models, nil
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
func (h *handler) SaveUserModel(c *gin.Context) {
	httpx.Endpoint[aiUserModelSaveReq, struct{}]{
		Parse: func(c *gin.Context) (*aiUserModelSaveReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &httpx.ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			req, err := httpx.BindJSONMsg[SaveUserModelReq](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			if req.Name == "" || req.APIKey == "" || req.BaseURL == "" || req.Model == "" {
				return nil, httpx.BadRequest("name/api_key/base_url/model 均为必填")
			}
			return &aiUserModelSaveReq{UserID: uid, Req: *req}, nil
		},
		Invoke: func(ctx context.Context, req *aiUserModelSaveReq) (*struct{}, error) {
			if err := h.svc.SaveUserModel(c.Request.Context(), req.UserID, req.Req); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		ErrStatus: httpx.ErrStatusAll(http.StatusBadRequest),
		Render: func(c *gin.Context, _ *aiUserModelSaveReq, _ *struct{}) {
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
func (h *handler) DeleteUserModel(c *gin.Context) {
	httpx.Endpoint[aiModelIDReq, struct{}]{
		Parse: func(c *gin.Context) (*aiModelIDReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &httpx.ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			id, err := httpx.PathInt(c, "id", "无效的模型 ID")
			if err != nil {
				return nil, err
			}
			return &aiModelIDReq{UserID: uid, ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *aiModelIDReq) (*struct{}, error) {
			if err := h.svc.DeleteUserModel(c.Request.Context(), req.UserID, req.ID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		ErrStatus: &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{
			{Sentinel: gorm.ErrRecordNotFound, Status: http.StatusNotFound, Message: "模型不存在"},
			{Sentinel: nil, Status: http.StatusInternalServerError},
		}},
		Render: func(c *gin.Context, _ *aiModelIDReq, _ *struct{}) {
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
// @Success 200 {object} response.R{data=[]aiassistant.AIChatSessionDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /ai-assistant/sessions [get]
func (h *handler) ListSessions(c *gin.Context) {
	httpx.Endpoint[aiUserIDReq, []AIChatSessionDTO]{
		Parse: func(c *gin.Context) (*aiUserIDReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &httpx.ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			return &aiUserIDReq{UserID: uid}, nil
		},
		Invoke: func(ctx context.Context, req *aiUserIDReq) (*[]AIChatSessionDTO, error) {
			sessions, err := h.svc.ListSessions(c.Request.Context(), req.UserID, c.Query("feature_key"))
			if err != nil {
				return nil, err
			}
			return &sessions, nil
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
// @Success 200 {object} response.R{data=aiassistant.AIChatSessionDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /ai-assistant/sessions [post]
func (h *handler) CreateSession(c *gin.Context) {
	httpx.Endpoint[aiSessionCreateReq, AIChatSessionDTO]{
		Parse: func(c *gin.Context) (*aiSessionCreateReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &httpx.ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			var body struct {
				Title      string `json:"title"`
				ModelName  string `json:"model_name"`
				FeatureKey string `json:"feature_key"`
			}
			_ = c.ShouldBindJSON(&body)
			return &aiSessionCreateReq{UserID: uid, Title: body.Title, ModelName: body.ModelName, FeatureKey: body.FeatureKey}, nil
		},
		Invoke: func(ctx context.Context, req *aiSessionCreateReq) (*AIChatSessionDTO, error) {
			return h.svc.CreateSession(c.Request.Context(), req.UserID, req.Title, req.ModelName, req.FeatureKey)
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
func (h *handler) DeleteSession(c *gin.Context) {
	httpx.Endpoint[aiModelIDReq, struct{}]{
		Parse: func(c *gin.Context) (*aiModelIDReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &httpx.ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			id, err := httpx.PathInt(c, "id", "无效的会话 ID")
			if err != nil {
				return nil, err
			}
			return &aiModelIDReq{UserID: uid, ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *aiModelIDReq) (*struct{}, error) {
			if err := h.svc.DeleteSession(c.Request.Context(), req.UserID, req.ID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		ErrStatus: &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{
			{Sentinel: gorm.ErrRecordNotFound, Status: http.StatusNotFound, Message: "会话不存在"},
			{Sentinel: nil, Status: http.StatusInternalServerError},
		}},
		Render: func(c *gin.Context, _ *aiModelIDReq, _ *struct{}) {
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
// @Success 200 {object} response.R{data=aiassistant.AISessionRenameResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /ai-assistant/sessions/{id}/title [patch]
func (h *handler) RenameSession(c *gin.Context) {
	httpx.Endpoint[aiSessionRenameReq, struct{}]{
		Parse: func(c *gin.Context) (*aiSessionRenameReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &httpx.ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			id, err := httpx.PathInt(c, "id", "无效的会话 ID")
			if err != nil {
				return nil, err
			}
			req, err := httpx.BindJSONMsg[aiSessionRenameReqBody](c, "请求数据无效")
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
		ErrStatus: &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{
			{Sentinel: gorm.ErrRecordNotFound, Status: http.StatusNotFound, Message: "会话不存在"},
			{Sentinel: nil, Status: http.StatusBadRequest},
		}},
		Render: func(c *gin.Context, _ *aiSessionRenameReq, _ *struct{}) {
			response.Success(c, AISessionRenameResultDTO{Message: "已更新会话标题"})
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
// @Success 200 {object} response.R{data=[]aiassistant.AIChatMessageDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "不存在"
// @Router /ai-assistant/sessions/{id}/messages [get]
func (h *handler) GetSessionMessages(c *gin.Context) {
	httpx.Endpoint[aiModelIDReq, []AIChatMessageDTO]{
		Parse: func(c *gin.Context) (*aiModelIDReq, error) {
			uid := middleware.CurrentUserID(c)
			if uid == 0 {
				return nil, &httpx.ParseError{Status: http.StatusUnauthorized, Message: "请先登录"}
			}
			id, err := httpx.PathInt(c, "id", "无效的会话 ID")
			if err != nil {
				return nil, err
			}
			return &aiModelIDReq{UserID: uid, ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *aiModelIDReq) (*[]AIChatMessageDTO, error) {
			msgs, err := h.svc.GetSessionMessages(c.Request.Context(), req.UserID, req.ID)
			if err != nil {
				return nil, err
			}
			return &msgs, nil
		},
		ErrStatus: &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{
			{Sentinel: gorm.ErrRecordNotFound, Status: http.StatusNotFound, Message: "会话不存在"},
			{Sentinel: nil, Status: http.StatusInternalServerError},
		}},
	}.Handle(c)
}

// writeSSEHeaders SSE 响应头（唯一写点：预检阻断分支与主路径共用，禁用 nginx 缓冲）。
func writeSSEHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
}

// StreamChat 流式对话
// @Summary AI 流式对话（SSE）
// @Description 可选认证的 SSE 流式响应（ADR-0048 决策 6：**不走统一 JSON 信封，不在契约生成面**）；事件 message/sources/usage/error/done
// @Tags 学员端-AI助手
// @Accept json
// @Produce text/event-stream
// @Param body body object true "消息" example({"feature_key":"fault_consult","messages":[{"role":"user","content":"你好"}]})
// @Success 200 {string} string "SSE stream"
// @Failure 400 {object} response.R "参数错误"
// @Router /ai-assistant/chat [post]
func (h *handler) StreamChat(c *gin.Context) {
	userID := middleware.CurrentUserID(c) // 可选认证，未登录为 0

	var req StreamChatReq
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

	writeSSEHeaders(c)

	sendEvent := func(event string, data any) {
		payload, _ := json.Marshal(data)
		_, _ = c.Writer.WriteString("event: " + event + "\n")
		_, _ = c.Writer.WriteString("data: " + string(payload) + "\n\n")
		c.Writer.Flush()
	}

	// 计量闸门在模型端口内单点（ADR-0031）：余额预检、后计量扣费、「什么算 prompt」的
	// 事实选取与请求标识降级键全部在 meter，handler 只透传请求标识（幂等键事实，
	// RequestID 中间件注入）并按产出转发 SSE 事件。
	ctx := c.Request.Context()
	if requestID := c.GetString(string(middleware.CtxRequestID)); requestID != "" {
		ctx = WithRequestID(ctx, requestID)
	}
	// 诊断来源容器初始化（fault_diagnosis 响应 answer_sources 经此透传；其他功能恒空）
	ctx = WithDiagnosisSources(ctx)

	_, usage, err := h.svc.StreamChat(ctx, userID, req, func(content string) {
		sendEvent("message", map[string]string{"content": content})
	})

	if err != nil {
		if errors.Is(err, points.ErrInsufficientPoints) {
			// 闸门预检阻断（文案与迁移前 handler 逐字一致）；不扣费、无 done 事件
			sendEvent("error", map[string]string{"message": "积分不足，请先去任务中心完成任务"})
			return
		}
		sendEvent("error", map[string]string{"message": err.Error()})
		return
	}
	// 智能维修诊断来源资料（answer_sources）透传：message 事件只搬 content，来源独立事件
	if sources := DiagnosisSourcesFrom(ctx); len(sources) > 0 {
		sendEvent("sources", map[string]any{"sources": sources})
	}
	// usage 事件与扣费结果形状不变（移动端契约无感）：扣费失败时沿用迁移前行为——
	// 余额不足发 error（仍发 done），其余错误静默跳过 usage 事件
	if usage != nil {
		if usage.Err == nil {
			sendEvent("usage", usage.Res)
		} else if errors.Is(usage.Err, points.ErrInsufficientPoints) {
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
// @Success 200 {object} response.R{data=aiassistant.AIImageUploadResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Router /ai-assistant/upload-image [post]
func (h *handler) UploadImage(c *gin.Context) {
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
	response.SuccessWithMsg(c, "图片上传成功", AIImageUploadResultDTO{URL: url})
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
	Req    SaveUserModelReq
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
