package api

import (
	"context"
	"errors"
	"net/http"

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

// interactionErrStatus 题目互动域（评论 + 题内笔记）的错误面——直接复用 Endpoint 缝那张表，
// 不在本文件重写第二份「扫表 → 命中回 entry 的码与文案 / 未命中回 fallback」的算法。
//
// fallback 由 handler 既有的 400/500 混答改成 500（ADR-0065 决策 7）：`VisibleByID` 不再丢弃
// .Error 之后，「问不出可见性」必须有一档可落，否则它会退回冒充上面某一句。而默认面收窄的前提
// 是其余业务事实**各有名字**——两件事必须同时做，不是二选一。
//
// ErrQuestionNotFound 带 message：呈现层要说「题目不存在」（越权不泄漏存在性），不是哨兵自己那句；
// 这正是 errStatusEntry.message 这一格存在的理由（WithSentinelsMsg 的同形规则在裸 handler 一侧）。
var interactionErrStatus = &errStatusTable{
	entries: []errStatusEntry{
		{sentinel: service.ErrQuestionNotFound, status: http.StatusNotFound, message: "题目不存在"},
		{sentinel: service.ErrCommentContentEmpty, status: http.StatusBadRequest},
		{sentinel: service.ErrCommentTooLong, status: http.StatusBadRequest},
		{sentinel: service.ErrCommentNotFound, status: http.StatusBadRequest},
		{sentinel: service.ErrCommentNotOwned, status: http.StatusBadRequest},
		{sentinel: service.ErrNoteContentEmpty, status: http.StatusBadRequest},
		{sentinel: service.ErrNoteContentTooLong, status: http.StatusBadRequest},
	},
	fallback: http.StatusInternalServerError,
}

// renderOutOfPoolQuestion 池外题的 HTTP 出口：命中 ErrQuestionNotFound 即渲染 404 并返回 true。
// 保留给本文件里那条不过 scope 的读面（knowledge）沿用；评论/笔记两支已归进 interactionErrStatus。
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
// @Failure 500 {object} response.R "服务端内部错误（含可见性/存在性查询读不动；不外发驱动原文）"
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
		interactionErrStatus.renderError(c, err)
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
// @Failure 500 {object} response.R "服务端内部错误（含可见性/存在性查询读不动；不外发驱动原文）"
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
		interactionErrStatus.renderError(c, err)
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
// @Failure 500 {object} response.R "服务端内部错误（含可见性/存在性查询读不动；不外发驱动原文）"
// @Router /questions/comments/{comment_id} [delete]
func (h *QuestionInteractionHandler) DeleteComment(c *gin.Context) {
	cid, err := pathInt(c, "comment_id", "评论ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := middleware.CurrentUserID(c)
	if err := h.commentSvc.Delete(cid, uid); err != nil {
		// 从前对任何 err 都答 400 + err.Error()，而 service 的 Delete 又把「读不动」折进
		// 「评论不存在」⇒ 两处叠起来，故障与业务事实同形。两面一起改（决策 7 的同一条）。
		// 这一格是本批第二版补上的：第一版漏改，由 TestFaultFacesAllSayFault 注故障当场判红
		// （它报出的正是 400 + `SQL logic error: no such table: question_comment`）。
		interactionErrStatus.renderError(c, err)
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
// @Failure 500 {object} response.R "服务端内部错误（含可见性/存在性查询读不动；不外发驱动原文）"
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
		interactionErrStatus.renderError(c, err)
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
// @Failure 500 {object} response.R "服务端内部错误（含可见性/存在性查询读不动；不外发驱动原文）"
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
		interactionErrStatus.renderError(c, err)
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
// @Failure 500 {object} response.R "服务端内部错误（含可见性/存在性查询读不动；不外发驱动原文）"
// @Router /questions/{question_id}/note [delete]
func (h *QuestionInteractionHandler) DeleteNote(c *gin.Context) {
	qid, err := pathInt(c, "question_id", "题目ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := middleware.CurrentUserID(c)
	if err := h.noteSvc.DeleteForQuestion(qid, uid, studentQuestionScope(c)); err != nil {
		interactionErrStatus.renderError(c, err)
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
