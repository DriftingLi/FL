// 本文件：题目互动域的 HTTP 出口（ADR-0070）—— /api/questions 下的评论 / 笔记 / 考点三组端点。
//
// 装配点：internal/api/routes_registry.go 的 RegisterRoutes 调用。
// 题内笔记三端点转投 internal/note 的 Service（questionInteraction → note 单向边）。
package questioninteraction

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/note"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

type handler struct {
	commentSvc   *Service
	noteSvc      *note.Service
	knowledgeSvc *KnowledgeService
}

func newHandler(c *Service, n *note.Service, k *KnowledgeService) *handler {
	return &handler{commentSvc: c, noteSvc: n, knowledgeSvc: k}
}

func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, credRes middleware.CredentialResolver, commentSvc *Service, noteSvc *note.Service, knowledgeSvc *KnowledgeService) {
	h := newHandler(commentSvc, noteSvc, knowledgeSvc)
	// CredentialScoped 在此蓝图的目的不是给评论/笔记本身分区，而是装配题目读 scope
	// （ADR-0062 决策 4）：评论与笔记都挂在题上，判据 = 「这道题对该学员可读吗」。
	g := rg.Group("/questions", middleware.JWTAuth(session), middleware.CredentialScoped(credRes))

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

// ErrStatus 题目互动域（评论 + 题内笔记）的错误面——直接复用 Endpoint 缝那张表，
// 不在本文件重写第二份「扫表 → 命中回 entry 的码与文案 / 未命中回 fallback」的算法。
//
// fallback 由 handler 既有的 400/500 混答改成 500（ADR-0065 决策 7）：`VisibleByID` 不再丢弃
// .Error 之后，「问不出可见性」必须有一档可落，否则它会退回冒充上面某一句。而默认面收窄的前提
// 是其余业务事实**各有名字**——两件事必须同时做，不是二选一。
//
// ErrQuestionNotFound 带 message：呈现层要说「题目不存在」（越权不泄漏存在性），不是哨兵自己那句；
// 这正是 httpx.ErrStatusEntry.Message 这一格存在的理由（WithSentinelsMsg 的同形规则在裸 handler 一侧）。
// 旧的那枚 helper `renderOutOfPoolQuestion` 被本表第一条 entry 完整取代，随本批删除——留在文件里
// 就是一处「两个宿主说同一件事」，而且 CI 的 unused 检查也当场把它点了出来。
var ErrStatus = &httpx.ErrStatusTable{
	Entries: []httpx.ErrStatusEntry{
		{Sentinel: questionbank.ErrQuestionNotFound, Status: http.StatusNotFound, Message: "题目不存在"},
		{Sentinel: ErrCommentContentEmpty, Status: http.StatusBadRequest},
		{Sentinel: ErrCommentTooLong, Status: http.StatusBadRequest},
		{Sentinel: ErrCommentNotFound, Status: http.StatusBadRequest},
		{Sentinel: ErrCommentNotOwned, Status: http.StatusBadRequest},
		{Sentinel: note.ErrNoteContentEmpty, Status: http.StatusBadRequest},
		{Sentinel: note.ErrNoteContentTooLong, Status: http.StatusBadRequest},
	},
	Fallback: http.StatusInternalServerError,
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
// @Success 200 {object} response.R{data=QuestionCommentPageResult} "success"
// @Failure 400 {object} response.R "题目ID无效"
// @Failure 500 {object} response.R "服务端内部错误（含可见性/存在性查询读不动；不外发驱动原文）"
// @Router /questions/{question_id}/comments [get]
func (h *handler) ListComments(c *gin.Context) {
	qid, err := httpx.PathInt(c, "question_id", "题目ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	page := httpx.QueryIntDefault(c, "page", 1)
	pageSize := httpx.QueryIntDefault(c, "page_size", 10)
	items, total, err := h.commentSvc.List(qid, page, pageSize, questionbank.StudentQuestionScope(c))
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.Success(c, QuestionCommentPageResult{Items: items, Page: page, PageSize: pageSize, Total: total})
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
// @Success 201 {object} response.R{data=QuestionCommentDTO} "success"
// @Failure 400 {object} response.R "题目ID无效"
// @Failure 500 {object} response.R "服务端内部错误（含可见性/存在性查询读不动；不外发驱动原文）"
// @Router /questions/{question_id}/comments [post]
func (h *handler) CreateComment(c *gin.Context) {
	qid, err := httpx.PathInt(c, "question_id", "题目ID无效")
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
	m, err := h.commentSvc.Create(qid, uid, req.Content, questionbank.StudentQuestionScope(c))
	if err != nil {
		ErrStatus.RenderError(c, err)
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
func (h *handler) DeleteComment(c *gin.Context) {
	cid, err := httpx.PathInt(c, "comment_id", "评论ID无效")
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
		ErrStatus.RenderError(c, err)
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
func (h *handler) GetNote(c *gin.Context) {
	qid, err := httpx.PathInt(c, "question_id", "题目ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := middleware.CurrentUserID(c)
	n, err := h.noteSvc.GetForQuestion(qid, uid, questionbank.StudentQuestionScope(c))
	if err != nil {
		ErrStatus.RenderError(c, err)
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
func (h *handler) UpsertNote(c *gin.Context) {
	qid, err := httpx.PathInt(c, "question_id", "题目ID无效")
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
	n, err := h.noteSvc.UpsertForQuestion(qid, uid, req.Content, questionbank.StudentQuestionScope(c))
	if err != nil {
		ErrStatus.RenderError(c, err)
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
func (h *handler) DeleteNote(c *gin.Context) {
	qid, err := httpx.PathInt(c, "question_id", "题目ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := middleware.CurrentUserID(c)
	if err := h.noteSvc.DeleteForQuestion(qid, uid, questionbank.StudentQuestionScope(c)); err != nil {
		ErrStatus.RenderError(c, err)
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
func (h *handler) ListKnowledge(c *gin.Context) {
	qid, err := httpx.PathInt(c, "question_id", "题目ID无效")
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
