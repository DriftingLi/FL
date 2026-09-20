package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// noteErrStatus 笔记域哨兵→状态码表（ADR-0024 口径：按哨兵映射，不比对文案）。
// 正文校验类错误（空/超长）不是哨兵，走 fallback 400；删改他人笔记按「不存在」404。
var noteErrStatus = &errStatusTable{
	entries: []errStatusEntry{
		{sentinel: service.ErrNoteNotFound, status: http.StatusNotFound},
	},
	fallback: http.StatusBadRequest,
}

// NoteHandler 学员笔记 handler（ADR-0055）：题目笔记的汇集读面 + 独立笔记 CRUD。
// 题目维度上的单条读写仍由 QuestionInteractionHandler 承载（/api/questions/:id/note，
// 契约不变）；本蓝图只服务「我的笔记」列表页。
type NoteHandler struct {
	svc *service.NoteService
}

// NewNoteHandler 创建笔记 handler。
func NewNoteHandler(svc *service.NoteService) *NoteHandler { return &NoteHandler{svc: svc} }

// RegisterNoteRoutes 注册 /api/notes 蓝图。
//
// 门禁与既有 /api/questions/:id/note **一致：只要求登录，不挂能力点**——笔记是纯用户私有
// 数据，读写一律以 user_id 收口、越权按「不存在」处理；给同一资源的两条路径挂两套门才是
// 真正的不一致（该观察记在 ADR-0055）。
func RegisterNoteRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.NoteService) {
	h := NewNoteHandler(svc)
	g := rg.Group("/notes", middleware.JWTAuth(rd.Session))

	// GET    /api/notes          我的笔记（分页 + scope 筛选）
	g.GET("", h.List)
	// POST   /api/notes          新建独立笔记
	g.POST("", h.Create)
	// PUT    /api/notes/:id      改笔记正文（只认本人）
	g.PUT("/:id", h.Update)
	// DELETE /api/notes/:id      删笔记（只认本人）
	g.DELETE("/:id", h.Delete)
}

// listNotesReq 我的笔记列表查询。
type listNotesReq struct {
	UserID   int
	Scope    string
	Page     int
	PageSize int
}

// List 我的笔记
// @Summary 我的笔记
// @Description 分页查询本人笔记（按更新时间倒序）；scope 筛全部/题目笔记/独立笔记
// @Tags 学员端-笔记
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param scope query string false "筛选：all/question/standalone" default(all)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R{data=service.NotePageDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /notes [get]
func (h *NoteHandler) List(c *gin.Context) {
	Endpoint[listNotesReq, service.NotePageDTO]{
		Parse: func(c *gin.Context) (*listNotesReq, error) {
			return &listNotesReq{
				UserID:   middleware.CurrentUserID(c),
				Scope:    c.Query("scope"),
				Page:     atoiDefault(c.Query("page"), 1),
				PageSize: atoiDefault(c.Query("page_size"), 20),
			}, nil
		},
		Invoke: func(ctx context.Context, req *listNotesReq) (*service.NotePageDTO, error) {
			return h.svc.List(req.UserID, req.Scope, req.Page, req.PageSize)
		},
		ErrStatus: noteErrStatus,
	}.Handle(c)
}

// createNoteReq 新建独立笔记请求。
type createNoteReq struct {
	UserID  int
	Content string `json:"content"`
}

// Create 新建独立笔记
// @Summary 新建独立笔记
// @Description 新建一条与题目无关的笔记（question_id 为空）
// @Tags 学员端-笔记
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "笔记" example({"content":"我的笔记"})
// @Success 201 {object} response.R{data=service.NoteDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /notes [post]
func (h *NoteHandler) Create(c *gin.Context) {
	Endpoint[createNoteReq, service.NoteDTO]{
		Parse: func(c *gin.Context) (*createNoteReq, error) {
			var body struct {
				Content string `json:"content"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("参数错误")
			}
			return &createNoteReq{UserID: middleware.CurrentUserID(c), Content: body.Content}, nil
		},
		Invoke: func(ctx context.Context, req *createNoteReq) (*service.NoteDTO, error) {
			n, err := h.svc.Create(req.UserID, req.Content)
			if err != nil {
				return nil, err
			}
			dto := service.NoteToDTO(n, "")
			return &dto, nil
		},
		// 201 定制成功信封保留；错误路径退表（照 job.go 先例）
		ErrStatus: noteErrStatus,
		Render: func(c *gin.Context, _ *createNoteReq, resp *service.NoteDTO) {
			response.Created(c, "笔记已保存", *resp)
		},
	}.Handle(c)
}

// updateNoteReq 改笔记正文请求。
type updateNoteReq struct {
	UserID  int
	ID      int
	Content string `json:"content"`
}

// Update 改笔记正文
// @Summary 改笔记正文
// @Description 按笔记 id 改正文（只认本人；他人笔记按不存在处理）
// @Tags 学员端-笔记
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "笔记 ID"
// @Param body body object true "笔记" example({"content":"我的笔记"})
// @Success 200 {object} response.R{data=service.NoteDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "笔记不存在"
// @Router /notes/{id} [put]
func (h *NoteHandler) Update(c *gin.Context) {
	Endpoint[updateNoteReq, service.NoteDTO]{
		Parse: func(c *gin.Context) (*updateNoteReq, error) {
			id, err := pathInt(c, "id", "笔记 ID 无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				Content string `json:"content"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("参数错误")
			}
			return &updateNoteReq{UserID: middleware.CurrentUserID(c), ID: id, Content: body.Content}, nil
		},
		Invoke: func(ctx context.Context, req *updateNoteReq) (*service.NoteDTO, error) {
			n, err := h.svc.Update(req.ID, req.UserID, req.Content)
			if err != nil {
				return nil, err
			}
			dto := service.NoteToDTO(n, "")
			return &dto, nil
		},
		ErrStatus: noteErrStatus,
	}.Handle(c)
}

// deleteNoteReq 删笔记请求。
type deleteNoteReq struct {
	UserID int
	ID     int
}

// Delete 删笔记
// @Summary 删笔记
// @Description 按笔记 id 删除（只认本人）
// @Tags 学员端-笔记
// @Produce json
// @Security BearerAuth
// @Param id path int true "笔记 ID"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "笔记不存在"
// @Router /notes/{id} [delete]
func (h *NoteHandler) Delete(c *gin.Context) {
	Endpoint[deleteNoteReq, struct{}]{
		Parse: func(c *gin.Context) (*deleteNoteReq, error) {
			id, err := pathInt(c, "id", "笔记 ID 无效")
			if err != nil {
				return nil, err
			}
			return &deleteNoteReq{UserID: middleware.CurrentUserID(c), ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *deleteNoteReq) (*struct{}, error) {
			if err := h.svc.Delete(req.ID, req.UserID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		ErrStatus: noteErrStatus,
	}.Handle(c)
}
