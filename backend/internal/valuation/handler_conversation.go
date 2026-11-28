package valuation

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

type ConversationHandler struct {
	svc *ConversationService
}

func NewConversationHandler(svc *ConversationService) *ConversationHandler {
	return &ConversationHandler{svc: svc}
}

var conversationErrStatus = &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{
	{Sentinel: ErrDraftNotFound, Status: http.StatusNotFound},
	{Sentinel: ErrDraftVersionConflict, Status: http.StatusConflict},
	{Sentinel: ErrDraftNotEditing, Status: http.StatusConflict},
	{Sentinel: ErrDraftNothingToUndo, Status: http.StatusConflict},
	{Sentinel: ErrDraftIncomplete, Status: http.StatusConflict},
	{Sentinel: ErrInvalidDraftField, Status: http.StatusBadRequest},
	{Sentinel: ErrOperationNotFound, Status: http.StatusNotFound},
	{Sentinel: ErrOperationHashConflict, Status: http.StatusConflict},
	{Sentinel: ErrOperationDraftMismatch, Status: http.StatusConflict},
	{Sentinel: ErrEvaluationRejected, Status: http.StatusBadRequest},
	{Sentinel: ErrInvalidOperation, Status: http.StatusBadRequest},
	{Sentinel: ErrInvalidClientIP, Status: http.StatusBadRequest},
	{Sentinel: ErrSuggestionValidation, Status: http.StatusBadRequest},
	{Sentinel: ErrSuggestionQuotaExceeded, Status: http.StatusTooManyRequests},
	{Sentinel: ErrSuggestionUnavailable, Status: http.StatusInternalServerError},
}}

type draftPathRequest struct {
	ID string
}

func parseDraftPath(c *gin.Context) (*draftPathRequest, error) {
	id, err := normalizeDraftID(c.Param("id"))
	if err != nil {
		return nil, httpx.BadRequest("草稿 ID 非法")
	}
	return &draftPathRequest{ID: id}, nil
}

func parseCreateDraft(c *gin.Context) (*CreateDraftRequest, error) {
	if c.Request.ContentLength == 0 {
		return &CreateDraftRequest{}, nil
	}
	return httpx.BindJSON[CreateDraftRequest](c)
}

// CreateDraft 处理 POST /api/valuation/conversation/drafts。
// @Summary 创建估值草稿
// @Description 创建 30 分钟闲置过期的匿名可用草稿；value 状态保留真实 0/false 与来源依据。
// @Tags 估值-对话
// @Accept json
// @Produce json
// @Param body body CreateDraftRequest false "初始字段与补充资料"
// @Success 200 {object} response.R{data=ValuationDraft} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/conversation/drafts [post]
func (h *ConversationHandler) CreateDraft(c *gin.Context) {
	httpx.Endpoint[CreateDraftRequest, ValuationDraft]{
		Parse:     parseCreateDraft,
		Invoke:    h.svc.CreateDraft,
		ErrStatus: conversationErrStatus,
	}.Handle(c)
}

// GetDraft 处理 GET /api/valuation/conversation/drafts/:id。
// @Summary 读取估值草稿
// @Description 按随机能力 ID 读取未过期草稿；读取不延长闲置有效期。
// @Tags 估值-对话
// @Produce json
// @Param id path string true "草稿能力 ID"
// @Success 200 {object} response.R{data=ValuationDraft} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "草稿不存在或已过期"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/conversation/drafts/{id} [get]
func (h *ConversationHandler) GetDraft(c *gin.Context) {
	httpx.Endpoint[draftPathRequest, ValuationDraft]{
		Parse: parseDraftPath,
		Invoke: func(ctx context.Context, req *draftPathRequest) (*ValuationDraft, error) {
			return h.svc.GetDraft(ctx, req.ID)
		},
		ErrStatus: conversationErrStatus,
	}.Handle(c)
}

type draftPatchPathRequest struct {
	ID string `json:"-"`
	PatchDraftRequest
}

func parseDraftPatch(c *gin.Context) (*draftPatchPathRequest, error) {
	path, err := parseDraftPath(c)
	if err != nil {
		return nil, err
	}
	var req draftPatchPathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, httpx.BadRequest("请求参数错误")
	}
	req.ID = path.ID
	return &req, nil
}

// PatchDraft 处理 PATCH /api/valuation/conversation/drafts/:id。
// @Summary 修改估值草稿
// @Description 按 base_version 执行 CAS；过期版本返回 409。成功修改生成新版本并刷新闲置 TTL。
// @Tags 估值-对话
// @Accept json
// @Produce json
// @Param id path string true "草稿能力 ID"
// @Param body body PatchDraftRequest true "基准版本与字段变更"
// @Success 200 {object} response.R{data=ValuationDraft} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "草稿不存在或已过期"
// @Failure 409 {object} response.R "版本冲突或草稿不可编辑"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/conversation/drafts/{id} [patch]
func (h *ConversationHandler) PatchDraft(c *gin.Context) {
	httpx.Endpoint[draftPatchPathRequest, ValuationDraft]{
		Parse: parseDraftPatch,
		Invoke: func(ctx context.Context, req *draftPatchPathRequest) (*ValuationDraft, error) {
			return h.svc.PatchDraft(ctx, req.ID, &req.PatchDraftRequest)
		},
		ErrStatus: conversationErrStatus,
	}.Handle(c)
}

func parseDraftUndo(c *gin.Context) (*draftUndoPathRequest, error) {
	path, err := parseDraftPath(c)
	if err != nil {
		return nil, err
	}
	req := &DraftUndoRequest{}
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(req); err != nil {
			return nil, httpx.BadRequest("请求参数错误")
		}
	}
	return &draftUndoPathRequest{ID: path.ID, BaseVersion: req.BaseVersion}, nil
}

type draftUndoPathRequest struct {
	ID          string
	BaseVersion *int
}

// UndoDraft 处理 POST /api/valuation/conversation/drafts/:id/undo。
// @Summary 撤回最近一次草稿修改
// @Description 恢复上一版字段与依据，生成新版本并回到 editing；仅支持一级撤回。
// @Tags 估值-对话
// @Accept json
// @Produce json
// @Param id path string true "草稿能力 ID"
// @Param body body DraftUndoRequest false "可选基准版本；省略时使用当前版本"
// @Success 200 {object} response.R{data=ValuationDraft} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "草稿不存在或已过期"
// @Failure 409 {object} response.R "版本冲突或无可撤回修改"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/conversation/drafts/{id}/undo [post]
func (h *ConversationHandler) UndoDraft(c *gin.Context) {
	httpx.Endpoint[draftUndoPathRequest, ValuationDraft]{
		Parse: parseDraftUndo,
		Invoke: func(ctx context.Context, req *draftUndoPathRequest) (*ValuationDraft, error) {
			return h.svc.UndoDraft(ctx, req.ID, req.BaseVersion)
		},
		ErrStatus: conversationErrStatus,
	}.Handle(c)
}

// DeleteDraft 处理 DELETE /api/valuation/conversation/drafts/:id。
// @Summary 删除估值草稿
// @Description 主动清除草稿；重复删除保持幂等。
// @Tags 估值-对话
// @Produce json
// @Param id path string true "草稿能力 ID"
// @Success 204 "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/conversation/drafts/{id} [delete]
func (h *ConversationHandler) DeleteDraft(c *gin.Context) {
	httpx.Endpoint[draftPathRequest, struct{}]{
		Parse: parseDraftPath,
		Invoke: func(ctx context.Context, req *draftPathRequest) (*struct{}, error) {
			return &struct{}{}, h.svc.DeleteDraft(ctx, req.ID)
		},
		Render: func(c *gin.Context, _ *draftPathRequest, _ *struct{}) {
			c.Status(http.StatusNoContent)
		},
		ErrStatus: conversationErrStatus,
	}.Handle(c)
}

type operationPathRequest struct {
	ID string
}

func parseOperationPath(c *gin.Context) (*operationPathRequest, error) {
	id, err := normalizeConversationOperationKey(c.Param("id"))
	if err != nil {
		return nil, httpx.BadRequest("操作 ID 非法")
	}
	return &operationPathRequest{ID: id}, nil
}

type draftSuggestionPathRequest struct {
	ID string `json:"-"`
	SuggestDraftRequest
}

func parseDraftSuggestion(c *gin.Context) (*draftSuggestionPathRequest, error) {
	path, err := parseDraftPath(c)
	if err != nil {
		return nil, err
	}
	req := &draftSuggestionPathRequest{ID: path.ID}
	if err := c.ShouldBindJSON(&req.SuggestDraftRequest); err != nil {
		return nil, httpx.BadRequest("请求参数错误")
	}
	return req, nil
}

// SuggestDraft 处理 POST /api/valuation/conversation/drafts/:id/suggestions。
// @Summary 异步生成估值草稿建议
// @Description 按 base_version 受理一次结构化模型建议；同键同负载恢复、不同负载 409。模型结果仅预填可靠唯一值，多候选与冲突留给用户，晚响应判 stale。匿名与登录共享草稿 8 次实际调用上限，另有可信 IP 与全站预算。
// @Tags 估值-对话
// @Accept json
// @Produce json
// @Param id path string true "草稿能力 ID"
// @Param body body SuggestDraftRequest true "操作标识、基准版本与待整理资料"
// @Success 200 {object} response.R{data=ConversationOperation} "success"
// @Success 202 {object} response.R{data=ConversationOperation} "已受理"
// @Failure 400 {object} response.R "参数错误或 IP 不可解析"
// @Failure 404 {object} response.R "草稿不存在或已过期"
// @Failure 409 {object} response.R "版本冲突或幂等负载冲突"
// @Failure 429 {object} response.R "草稿、IP 或全站预算耗尽"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/conversation/drafts/{id}/suggestions [post]
func (h *ConversationHandler) SuggestDraft(c *gin.Context) {
	httpx.Endpoint[draftSuggestionPathRequest, ConversationOperation]{
		Parse: parseDraftSuggestion,
		Invoke: func(ctx context.Context, req *draftSuggestionPathRequest) (*ConversationOperation, error) {
			return h.svc.RequestSuggestions(ctx, req.ID, &req.SuggestDraftRequest, middleware.CurrentUserID(c), middleware.ClientIP(c))
		},
		Render: func(c *gin.Context, _ *draftSuggestionPathRequest, operation *ConversationOperation) {
			c.JSON(http.StatusAccepted, response.R{Code: http.StatusAccepted, Message: "accepted", Data: operation})
		},
		ErrStatus: conversationErrStatus,
	}.Handle(c)
}

// RequestExplanation 处理 POST /api/valuation/conversation/explanations。
// @Summary 异步解释估值卡片
// @Description 仅允许草稿能力 ID、匿名正式 operation 或登录 owner、试算 request/snapshot 能力对。模型只选模板键与证据 ID，事实由原快照回填；草稿卡携带 snapshot 时按卡内锁定事实解释，当前草稿已变更也不返回 409，未携带快照才要求 draft_version 与当前草稿一致。工时反事实返回 recalculate，由用户触发重算。建议与解释共享草稿/IP/全站额度。
// @Tags 估值-对话
// @Accept json
// @Produce json
// @Param body body ExplainCardRequest true "操作标识、卡片版本、原卡片（草稿可附字段快照）与问题"
// @Success 200 {object} response.R{data=ConversationOperation} "success"
// @Success 202 {object} response.R{data=ConversationOperation} "已受理"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "卡片不存在或无权访问"
// @Failure 409 {object} response.R "未携带快照的旧卡与当前草稿版本不一致，或幂等负载冲突"
// @Failure 429 {object} response.R "共享额度耗尽"
// @Failure 500 {object} response.R "解释暂不可用或已关停"
// @Router /valuation/conversation/explanations [post]
func (h *ConversationHandler) RequestExplanation(c *gin.Context) {
	httpx.Endpoint[ExplainCardRequest, ConversationOperation]{
		Parse: httpx.BindJSON[ExplainCardRequest],
		Invoke: func(ctx context.Context, req *ExplainCardRequest) (*ConversationOperation, error) {
			return h.svc.RequestExplanation(ctx, req, middleware.CurrentUserID(c), middleware.ClientIP(c))
		},
		Render: func(c *gin.Context, _ *ExplainCardRequest, operation *ConversationOperation) {
			c.JSON(http.StatusAccepted, response.R{Code: http.StatusAccepted, Message: "accepted", Data: operation})
		},
		ErrStatus: conversationErrStatus,
	}.Handle(c)
}

// CreateOperation 处理 POST /api/valuation/conversation/operations。
// @Summary 确认草稿并创建正式评估
// @Description 仅接受已核对的完整草稿版本；operation 与 evaluation 同事务。同键同哈希重试返回首次结果，同键不同哈希返回 409。匿名首次受理的归属不因之后登录而改变。操作去重凭据随正式评估持久保留，不适用草稿的 30 分钟闲置 TTL。
// @Tags 估值-对话
// @Accept json
// @Produce json
// @Param body body ConversationOperationRequest true "操作标识、草稿版本与正式评估请求"
// @Success 200 {object} response.R{data=ConversationOperation} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "草稿不存在或已过期"
// @Failure 409 {object} response.R "草稿不完整、版本冲突或请求哈希冲突"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/conversation/operations [post]
func (h *ConversationHandler) CreateOperation(c *gin.Context) {
	httpx.Endpoint[ConversationOperationRequest, ConversationOperation]{
		Parse: httpx.BindJSON[ConversationOperationRequest],
		Invoke: func(ctx context.Context, req *ConversationOperationRequest) (*ConversationOperation, error) {
			return h.svc.ExecuteOperation(ctx, req, middleware.CurrentUserID(c))
		},
		ErrStatus: conversationErrStatus,
	}.Handle(c)
}

// GetOperation 处理 GET /api/valuation/conversation/operations/:id。
// @Summary 读取估值操作
// @Description 按操作能力 ID 返回首次受理状态与正式评估结果。
// @Tags 估值-对话
// @Produce json
// @Param id path string true "操作能力 ID"
// @Success 200 {object} response.R{data=ConversationOperation} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "操作不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/conversation/operations/{id} [get]
func (h *ConversationHandler) GetOperation(c *gin.Context) {
	httpx.Endpoint[operationPathRequest, ConversationOperation]{
		Parse: parseOperationPath,
		Invoke: func(ctx context.Context, req *operationPathRequest) (*ConversationOperation, error) {
			return h.svc.GetOperation(ctx, req.ID, middleware.CurrentUserID(c))
		},
		ErrStatus: conversationErrStatus,
	}.Handle(c)
}
