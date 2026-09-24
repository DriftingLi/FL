package api

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

type QuestionInteractionHandler struct {
	commentSvc   *service.QuestionCommentService
	noteSvc      *service.NoteService
	knowledgeSvc *service.QuestionKnowledgeService
}

func NewQuestionInteractionHandler(c *service.QuestionCommentService, n *service.NoteService, k *service.QuestionKnowledgeService) *QuestionInteractionHandler {
	return &QuestionInteractionHandler{commentSvc: c, noteSvc: n, knowledgeSvc: k}
}

func RegisterQuestionInteractionRoutes(rg *gin.RouterGroup, rd RouterDeps, commentSvc *service.QuestionCommentService, noteSvc *service.NoteService, knowledgeSvc *service.QuestionKnowledgeService) {
	h := NewQuestionInteractionHandler(commentSvc, noteSvc, knowledgeSvc)
	// CredentialScoped 在此蓝图的目的不是给评论/笔记本身分区，而是装配题目读 scope
	// （ADR-0062 决策 4）：评论与笔记都挂在题上，判据 = 「这道题对该学员可读吗」。
	g := rg.Group("/questions", middleware.JWTAuth(rd.Session), middleware.CredentialScoped(rd.CredentialScope))

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

// renderOutOfPoolQuestion 池外题的 HTTP 出口：命中 ErrQuestionNotFound 即渲染 404 并返回 true
// （scope 判据的表现形式，越权按「不存在」不泄漏存在性）；其余错误交调用方按既有口径渲染。
func renderOutOfPoolQuestion(c *gin.Context, err error) bool {
	if !errors.Is(err, service.ErrQuestionNotFound) {
		return false
	}
	response.NotFound(c, "题目不存在")
	return true
}

// ListComments 题目评论列表
// @Summary 题目评论列表
// @Description 分页查询题目评论，含作者昵称/头像
// @Tags 学员端-题目互动
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Success 200 {object} response.R{data=service.QuestionCommentPageResult} "success"
// @Failure 400 {object} response.R "题目ID无效"
// @Router /questions/{question_id}/comments [get]
func (h *QuestionInteractionHandler) ListComments(c *gin.Context) {
	qid, err := pathInt(c, "question_id", "题目ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 10)
	items, total, err := h.commentSvc.List(qid, page, pageSize, studentQuestionScope(c))
	if err != nil {
		if renderOutOfPoolQuestion(c, err) {
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, service.QuestionCommentPageResult{Items: items, Page: page, PageSize: pageSize, Total: total})
}

// CreateComment 发表题目评论
// @Summary 发表评论
// @Description 学员发表题目评论，直发不审核
// @Tags 学员端-题目互动
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Param body body object true "内容" example({"content":"这题易错"})
// @Success 201 {object} response.R{data=service.QuestionCommentDTO} "success"
// @Failure 400 {object} response.R "题目ID无效"
// @Router /questions/{question_id}/comments [post]
func (h *QuestionInteractionHandler) CreateComment(c *gin.Context) {
	qid, err := pathInt(c, "question_id", "题目ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := middleware.CurrentUserID(c)
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	m, err := h.commentSvc.Create(qid, uid, req.Content, studentQuestionScope(c))
	if err != nil {
		if renderOutOfPoolQuestion(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "评论成功", m)
}

// DeleteComment 删除本人评论
// @Summary 删除评论
// @Tags 学员端-题目互动
// @Security BearerAuth
// @Param comment_id path int true "评论ID"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "评论ID无效"
// @Router /questions/comments/{comment_id} [delete]
func (h *QuestionInteractionHandler) DeleteComment(c *gin.Context) {
	cid, err := pathInt(c, "comment_id", "评论ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := middleware.CurrentUserID(c)
	if err := h.commentSvc.Delete(cid, uid); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "已删除", nil)
}

// GetNote 获取本人笔记
// @Summary 获取笔记
// @Tags 学员端-题目互动
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Success 200 {object} response.R{data=model.Note} "success"
// @Failure 400 {object} response.R "题目ID无效"
// @Router /questions/{question_id}/note [get]
func (h *QuestionInteractionHandler) GetNote(c *gin.Context) {
	qid, err := pathInt(c, "question_id", "题目ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := middleware.CurrentUserID(c)
	n, err := h.noteSvc.GetForQuestion(qid, uid, studentQuestionScope(c))
	if err != nil {
		if renderOutOfPoolQuestion(c, err) {
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	if n == nil {
		response.Success(c, nil)
		return
	}
	response.Success(c, n)
}

// UpsertNote 保存笔记（每人每题一条）
// @Summary 保存笔记
// @Tags 学员端-题目互动
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Param body body object true "笔记" example({"content":"我的笔记"})
// @Success 200 {object} response.R{data=model.Note} "success"
// @Failure 400 {object} response.R "题目ID无效"
// @Router /questions/{question_id}/note [put]
func (h *QuestionInteractionHandler) UpsertNote(c *gin.Context) {
	qid, err := pathInt(c, "question_id", "题目ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := middleware.CurrentUserID(c)
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	n, err := h.noteSvc.UpsertForQuestion(qid, uid, req.Content, studentQuestionScope(c))
	if err != nil {
		if renderOutOfPoolQuestion(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, n)
}

// DeleteNote 删除笔记
// @Summary 删除笔记
// @Tags 学员端-题目互动
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "题目ID无效"
// @Router /questions/{question_id}/note [delete]
func (h *QuestionInteractionHandler) DeleteNote(c *gin.Context) {
	qid, err := pathInt(c, "question_id", "题目ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := middleware.CurrentUserID(c)
	if err := h.noteSvc.DeleteForQuestion(qid, uid, studentQuestionScope(c)); err != nil {
		if renderOutOfPoolQuestion(c, err) {
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "已删除", nil)
}

// ListKnowledge 题目考点（题库标签）
// @Summary 考点标签
// @Description 只读返回题目挂载的题库标签，无标签返回空数组
// @Tags 学员端-题目互动
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Success 200 {object} response.R{data=[]model.QuestionTag} "success"
// @Failure 400 {object} response.R "题目ID无效"
// @Router /questions/{question_id}/knowledge [get]
func (h *QuestionInteractionHandler) ListKnowledge(c *gin.Context) {
	qid, err := pathInt(c, "question_id", "题目ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	tags, err := h.knowledgeSvc.ListForQuestion(qid)
	if err != nil {
		// 「查不动」不得被读成「这题没有考点」（ADR-0062 票6）：旧写法把 error 丢给 _ 后照样 200。
		response.ServerError(c, "查询考点失败")
		return
	}
	response.Success(c, tags)
}

// endpoint helper to avoid unused import warning
var _ = context.Background
