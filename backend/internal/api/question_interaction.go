package api

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

type QuestionInteractionHandler struct {
	commentSvc   *service.QuestionCommentService
	noteSvc      *service.QuestionNoteService
	knowledgeSvc *service.QuestionKnowledgeService
}

func NewQuestionInteractionHandler(c *service.QuestionCommentService, n *service.QuestionNoteService, k *service.QuestionKnowledgeService) *QuestionInteractionHandler {
	return &QuestionInteractionHandler{commentSvc: c, noteSvc: n, knowledgeSvc: k}
}

func RegisterQuestionInteractionRoutes(rg *gin.RouterGroup, rd RouterDeps, commentSvc *service.QuestionCommentService, noteSvc *service.QuestionNoteService, knowledgeSvc *service.QuestionKnowledgeService) {
	h := NewQuestionInteractionHandler(commentSvc, noteSvc, knowledgeSvc)
	g := rg.Group("/questions", middleware.JWTAuth(rd.Session))

	// 评论
	g.GET("/:question_id/comments", h.ListComments)
	g.POST("/:question_id/comments", h.CreateComment)
	g.DELETE("/comments/:comment_id", h.DeleteComment)

	// 笔记
	g.GET("/:question_id/note", h.GetNote)
	g.PUT("/:question_id/note", h.UpsertNote)
	g.DELETE("/:question_id/note", h.DeleteNote)

	// 考点
	g.GET("/:question_id/knowledge", h.ListKnowledge)
}

func (h *QuestionInteractionHandler) ListComments(c *gin.Context) {
	qid, _ := strconv.Atoi(c.Param("question_id"))
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 10)
	items, total, err := h.commentSvc.List(qid, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *QuestionInteractionHandler) CreateComment(c *gin.Context) {
	qid, _ := strconv.Atoi(c.Param("question_id"))
	uid := middleware.CurrentUserID(c)
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	m, err := h.commentSvc.Create(qid, uid, req.Content)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "评论成功", m)
}

func (h *QuestionInteractionHandler) DeleteComment(c *gin.Context) {
	cid, _ := strconv.Atoi(c.Param("comment_id"))
	uid := middleware.CurrentUserID(c)
	if err := h.commentSvc.Delete(cid, uid); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "已删除", nil)
}

func (h *QuestionInteractionHandler) GetNote(c *gin.Context) {
	qid, _ := strconv.Atoi(c.Param("question_id"))
	uid := middleware.CurrentUserID(c)
	n, err := h.noteSvc.Get(qid, uid)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	if n == nil {
		response.Success(c, nil)
		return
	}
	response.Success(c, n)
}

func (h *QuestionInteractionHandler) UpsertNote(c *gin.Context) {
	qid, _ := strconv.Atoi(c.Param("question_id"))
	uid := middleware.CurrentUserID(c)
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	n, err := h.noteSvc.Upsert(qid, uid, req.Content)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, n)
}

func (h *QuestionInteractionHandler) DeleteNote(c *gin.Context) {
	qid, _ := strconv.Atoi(c.Param("question_id"))
	uid := middleware.CurrentUserID(c)
	if err := h.noteSvc.Delete(qid, uid); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "已删除", nil)
}

func (h *QuestionInteractionHandler) ListKnowledge(c *gin.Context) {
	qid, _ := strconv.Atoi(c.Param("question_id"))
	tags, _ := h.knowledgeSvc.ListForQuestion(qid)
	response.Success(c, tags)
}

// endpoint helper to avoid unused import warning
var _ = context.Background
