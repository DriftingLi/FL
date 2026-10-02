// Package forum 论坛域的 HTTP 出口（公开 21 条 + 管理端 10 条，两蓝图分居 handler.go / handler_admin.go；
// ADR-0050 决策 3、ADR-0070 域包形态）。本文件：/api/forum 学员端蓝图 + 域哨兵状态码表 + 私有请求体。
package forum

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/course"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// handler 论坛 handler（帖子/回复/互动；打卡已迁独立蓝图 /api/check-in，ADR-0028）。
//
// 两片依赖（ADR-0050 决策 3）：svc = 学员交互 + 个人集合（学员端路由）；modSvc = 论坛治理
// （管理端 /admin/forum 路由）——管理端处置动作经治理 module 自己的 interface 组装，
// 不再穿学员交互的宽面。
type handler struct {
	svc      *Service
	modSvc   *ModerationService
	imageSvc *ImageService
}

// newHandler 创建论坛 handler。
func newHandler(svc *Service, modSvc *ModerationService, imageSvc *ImageService) *handler {
	return &handler{svc: svc, modSvc: modSvc, imageSvc: imageSvc}
}

// ErrStatus 论坛域哨兵→状态码表（第十二波票 5，#1168；形态照投稿域 #611/ADR-0024）：
// 存在性→404、所有权→403、状态前置/校验→400；未命中（DB 故障与未知错误）→ 500 默认信封。
// 例外：GetTopic（含 AdminGetTopic 委托）的 gorm.ErrRecordNotFound→404 手写映射不并入本表——
// 其理由是**渲染定制**（详情读面走 raw SQL Scan，未命中返回裸 gorm 哨兵，需固定文案「主题不存在」），
// 与哨兵缺失不同因（票面裁定保留，见 endpoint.go 例外清单）。
var ErrStatus = &httpx.ErrStatusTable{
	Entries: []httpx.ErrStatusEntry{
		// 存在性 → 404
		{Sentinel: ErrTopicNotFound, Status: http.StatusNotFound},
		{Sentinel: ErrReplyNotFound, Status: http.StatusNotFound},
		{Sentinel: ErrForumReportNotFound, Status: http.StatusNotFound},
		{Sentinel: course.ErrChapterNotFound, Status: http.StatusNotFound},
		// 所有权 → 403
		{Sentinel: ErrNotTopicOwner, Status: http.StatusForbidden},
		{Sentinel: ErrNotTopicAuthor, Status: http.StatusForbidden},
		{Sentinel: ErrNotReplyAuthor, Status: http.StatusForbidden},
		// 状态前置 → 400
		{Sentinel: ErrAcceptOwnReply, Status: http.StatusBadRequest},
		{Sentinel: ErrAcceptNotQuestion, Status: http.StatusBadRequest},
		{Sentinel: ErrCancelAcceptNotQuestion, Status: http.StatusBadRequest},
		{Sentinel: ErrAcceptExperienceTopic, Status: http.StatusBadRequest},
		{Sentinel: ErrDesignateAcceptedTopic, Status: http.StatusBadRequest},
		{Sentinel: ErrUnfeatureExperienceTopic, Status: http.StatusBadRequest},
		{Sentinel: ErrCategoryLockedByAccept, Status: http.StatusBadRequest},
		{Sentinel: ErrQuestionChapterConflict, Status: http.StatusBadRequest},
		{Sentinel: ErrParentReplyMismatch, Status: http.StatusBadRequest},
		{Sentinel: ErrReplyTopicMismatch, Status: http.StatusBadRequest},
		// 参数/校验 → 400
		{Sentinel: ErrContentFormatInvalid, Status: http.StatusBadRequest},
		{Sentinel: ErrCategoryInvalid, Status: http.StatusBadRequest},
		{Sentinel: ErrSolvedArgInvalid, Status: http.StatusBadRequest},
		{Sentinel: ErrFeaturedArgInvalid, Status: http.StatusBadRequest},
		{Sentinel: ErrExperienceArgInvalid, Status: http.StatusBadRequest},
		{Sentinel: ErrSolvedFilterScope, Status: http.StatusBadRequest},
		{Sentinel: ErrChapterIDRequired, Status: http.StatusBadRequest},
		{Sentinel: ErrTitleLength, Status: http.StatusBadRequest},
		{Sentinel: ErrContentLength, Status: http.StatusBadRequest},
		{Sentinel: ErrReplyContentLength, Status: http.StatusBadRequest},
		{Sentinel: ErrImagesTooMany, Status: http.StatusBadRequest},
		{Sentinel: ErrImageURLInvalid, Status: http.StatusBadRequest},
		{Sentinel: ErrReportReasonLength, Status: http.StatusBadRequest},
		{Sentinel: ErrReportTarget, Status: http.StatusBadRequest},
		{Sentinel: ErrReportStatusValue, Status: http.StatusBadRequest},
	},
}

// RegisterRoutes 注册 /api/forum 蓝图（需登录，hrwai_user）。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service, modSvc *ModerationService, imageSvc *ImageService) {
	h := newHandler(svc, modSvc, imageSvc)

	g := rg.Group("/forum", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapForumParticipate))

	// POST /api/forum/upload-image  上传论坛图片（图文分离，先传图后随发帖/回复提交 URL）
	g.POST("/upload-image", h.UploadImage)
	// GET /api/forum/topics?scope=all|general|chapter&chapter_id=&page=&page_size=&keyword=
	g.GET("/topics", h.ListTopics)
	// POST /api/forum/topics 发帖（chapter_id 为空/0 表示发到综合讨论区；images 为图片 URL 数组，最多 9 张）
	g.POST("/topics", h.CreateTopic)
	// GET /api/forum/topics/:id 主题详情（含回复）
	g.GET("/topics/:id", h.GetTopic)
	// PUT /api/forum/topics/:id 编辑帖子（#811，仅作者本人；可改 title/content/images/category）
	g.PUT("/topics/:id", h.UpdateTopic)
	// POST /api/forum/topics/:id/replies 回复（images 为图片 URL 数组，最多 3 张）
	g.POST("/topics/:id/replies", h.ReplyTopic)
	// DELETE /api/forum/topics/:id 删除自己的主题
	g.DELETE("/topics/:id", h.DeleteTopic)
	// DELETE /api/forum/replies/:id 删除自己的回复
	g.DELETE("/replies/:id", h.DeleteReply)

	// ===== 互动（ADR-0018）=====
	// POST /api/forum/topics/:id/like 点赞（幂等）
	g.POST("/topics/:id/like", h.LikeTopic)
	// DELETE /api/forum/topics/:id/like 取消点赞（幂等）
	g.DELETE("/topics/:id/like", h.UnlikeTopic)
	// POST /api/forum/topics/:id/report 举报主题
	g.POST("/topics/:id/report", h.ReportTopic)
	// POST /api/forum/replies/:id/report 举报回复
	g.POST("/replies/:id/report", h.ReportReply)
	// GET /api/forum/my-topics 我的帖子
	g.GET("/my-topics", h.MyTopics)
	// GET /api/forum/my-replies 我的回复
	g.GET("/my-replies", h.MyReplies)
	// ===== 个人动态（#701：赞过 / 围观 / 浏览记录，响应逐字沿用 my-topics 形态）=====
	// GET /api/forum/my-liked-topics 赞过（按点赞时间倒序）
	g.GET("/my-liked-topics", h.MyLikedTopics)
	// GET /api/forum/my-observed 围观（浏览减去四项直接互动，按最近浏览倒序）
	g.GET("/my-observed", h.MyObservedTopics)
	// GET /api/forum/my-view-history 浏览记录（按主题去重，按最近浏览倒序）
	g.GET("/my-view-history", h.MyViewHistory)

	// ===== 评论点赞（spec #268）=====
	g.POST("/replies/:id/like", h.LikeReply)
	g.DELETE("/replies/:id/like", h.UnlikeReply)
	// ===== 采纳（#366）=====
	g.POST("/topics/:id/accept", h.AcceptTopic)
	g.DELETE("/topics/:id/accept", h.CancelAccept)

}

// UploadImage 上传论坛图片
// @Summary 上传论坛图片
// @Description 图文分离，先传图后随发帖/回复提交 URL；支持论坛图片
// @Tags 学员端-论坛
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "图片文件"
// @Success 200 {object} response.R{data=forum.ForumImageUploadResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/upload-image [post]
func (h *handler) UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "未找到上传文件")
		return
	}
	url, err := h.imageSvc.Upload(c.Request.Context(), file)
	if err != nil {
		var fe *ForumImageError
		if errors.As(err, &fe) {
			if fe.Status == http.StatusBadRequest {
				response.BadRequest(c, fe.Message)
				return
			}
			response.ServerError(c, fe.Message)
			return
		}
		response.ServerError(c, "图片上传失败")
		return
	}
	response.SuccessWithMsg(c, "图片上传成功", ForumImageUploadResultDTO{URL: url})
}

// ListTopics 帖子列表
// @Summary 帖子列表
// @Description 支持 scope=all|general|chapter，按 category=all|discussion|question|experience、chapter_id/keyword/sort=latest|hot|created 过滤
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param scope query string false "范围 all|general|chapter"
// @Param category query string false "类别 discussion|question|experience，省略表示不过滤（向后兼容）"
// @Param featured query string false "精选过滤 true|false，省略表示不过滤（#742）"
// @Param chapter_id query int false "章节ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param keyword query string false "关键词"
// @Param sort query string false "排序 latest|hot|created（created=发帖时间，#722）"
// @Param order query string false "排序方向 asc|desc"
// @Success 200 {object} response.R{data=forum.ForumTopicPageResult} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/topics [get]
func (h *handler) ListTopics(c *gin.Context) {
	httpx.Endpoint[listTopicsReq, ForumTopicPageResult]{
		Parse: func(c *gin.Context) (*listTopicsReq, error) {
			return &listTopicsReq{
				Scope:    c.Query("scope"),
				Category: c.Query("category"),
				Solved:   c.Query("solved"),
				Featured: c.Query("featured"),
				// is_experience 是经验 Tab 的判据（ADR-0040）；category=experience 是遗留意图值，
				// 两者**不是**同一件事，详见 parseForumExperienceArg 的注释。
				IsExperience: c.Query("is_experience"),
				ChapterID:    httpx.QueryIntDefault(c, "chapter_id", 0),
				Page:         httpx.QueryIntDefault(c, "page", 1),
				PageSize:     httpx.QueryIntDefault(c, "page_size", 10),
				Keyword:      c.Query("keyword"),
				Sort:         c.Query("sort"),
				Order:        c.Query("order"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *listTopicsReq) (*ForumTopicPageResult, error) {
			return h.svc.ListTopics(TopicListInput{
				Scope:        req.Scope,
				Category:     req.Category,
				Solved:       req.Solved,
				Featured:     req.Featured,
				IsExperience: req.IsExperience,
				ChapterID:    req.ChapterID,
				Page:         req.Page,
				PageSize:     req.PageSize,
				Keyword:      req.Keyword,
				Sort:         req.Sort,
				Order:        req.Order,
			})
		},
		ErrStatus: ErrStatus,
	}.Handle(c)
}

// CreateTopic 发帖
// @Summary 发帖
// @Description chapter_id 为空表示综合讨论区；category=question 时不得带 chapter_id；experience（备考经验）可挂章节、不可被采纳；images 最多 9 张 URL
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "帖子" example({"title":"标题","content":"内容","category":"discussion","images":[]})
// @Success 201 {object} response.R{data=forum.ForumTopicDTO} "success"
// @Failure 400 {object} response.R "参数错误（含类别非法、问答帖带章节）"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "章节不存在（票5 存在性档）"
// @Router /forum/topics [post]
func (h *handler) CreateTopic(c *gin.Context) {
	httpx.Endpoint[createTopicReq, ForumTopicDTO]{
		Parse: func(c *gin.Context) (*createTopicReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			var body struct {
				ChapterID *int     `json:"chapter_id"`
				Category  string   `json:"category"`
				Title     string   `json:"title"`
				Content   string   `json:"content"`
				Images    []string `json:"images"`
				// ContentFormat 正文格式声明（ADR-0044）：text | markdown，缺省 text。
				ContentFormat string `json:"content_format"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, httpx.BadRequest("请求参数错误")
			}
			return &createTopicReq{
				UserID: userID, ChapterID: body.ChapterID, Category: body.Category,
				Title: body.Title, Content: body.Content, ContentFormat: body.ContentFormat,
				Images: body.Images,
				// 属地快照的输入走可信取 IP 单点：伪造 X-Forwarded-For 改不动它。
				ClientIP: middleware.ClientIP(c),
			}, nil
		},
		Invoke: func(ctx context.Context, req *createTopicReq) (*ForumTopicDTO, error) {
			return h.svc.CreateTopic(CreateTopicInput{
				UserID:        req.UserID,
				ChapterID:     req.ChapterID,
				Category:      req.Category,
				Title:         req.Title,
				Content:       req.Content,
				ContentFormat: req.ContentFormat,
				Images:        req.Images,
				ClientIP:      req.ClientIP,
			})
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *createTopicReq, resp *ForumTopicDTO) {
			response.Created(c, "发布成功", resp)
		},
	}.Handle(c)
}

// GetTopic 帖子详情
// @Summary 帖子详情
// @Description 含回复，sort=time|hot 控制回复排序
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题ID"
// @Param sort query string false "排序 time|hot|latest"
// @Param order query string false "排序方向 asc|desc"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页回复数" default(20)
// @Success 200 {object} response.R{data=forum.ForumTopicDetailDTO} "详情"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "不存在"
// @Router /forum/topics/{id} [get]
func (h *handler) GetTopic(c *gin.Context) {
	httpx.Endpoint[topicGetReq, ForumTopicDetailDTO]{
		Parse: func(c *gin.Context) (*topicGetReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			topicID, err := httpx.PathInt64(c, "id", "主题ID无效")
			if err != nil {
				return nil, err
			}
			return &topicGetReq{
				TopicID: topicID, UserID: userID,
				Sort: c.Query("sort"), Order: c.Query("order"),
				Page: httpx.QueryIntDefault(c, "page", 1), PageSize: httpx.QueryIntDefault(c, "page_size", 0),
			}, nil
		},
		Invoke: func(ctx context.Context, req *topicGetReq) (*ForumTopicDetailDTO, error) {
			return h.svc.GetTopic(req.toDetailInput())
		},
		ErrStatus: &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{
			{Sentinel: gorm.ErrRecordNotFound, Status: http.StatusNotFound, Message: "主题不存在"},
			{Sentinel: nil, Status: http.StatusInternalServerError},
		}},
	}.Handle(c)
}

// ReplyTopic 回复
// @Summary 回复帖子
// @Description 支持 parent_reply_id 回复他人回复；images 最多 3 张
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题ID"
// @Param body body object true "回复" example({"content":"内容","images":[]})
// @Success 201 {object} response.R{data=forum.ForumReplyDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "主题/被回复的回复不存在（票5 存在性档）"
// @Router /forum/topics/{id}/replies [post]
func (h *handler) ReplyTopic(c *gin.Context) {
	httpx.Endpoint[replyTopicReq, ForumReplyDTO]{
		Parse: func(c *gin.Context) (*replyTopicReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			topicID, err := httpx.PathInt64(c, "id", "主题ID无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				Content       string   `json:"content"`
				ParentReplyID *int64   `json:"parent_reply_id"`
				Images        []string `json:"images"`
				// ContentFormat 正文格式声明（ADR-0044）：text | markdown，缺省 text。
				ContentFormat string `json:"content_format"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, httpx.BadRequest("请求参数错误")
			}
			return &replyTopicReq{
				UserID: userID, TopicID: topicID, Content: body.Content,
				ParentReplyID: body.ParentReplyID, Images: body.Images,
				ContentFormat: body.ContentFormat,
				// 与发帖同口径：属地快照的输入走可信取 IP 单点。
				ClientIP: middleware.ClientIP(c),
			}, nil
		},
		Invoke: func(ctx context.Context, req *replyTopicReq) (*ForumReplyDTO, error) {
			return h.svc.ReplyTopic(ReplyTopicInput{
				UserID:        req.UserID,
				TopicID:       req.TopicID,
				Content:       req.Content,
				ParentReplyID: req.ParentReplyID,
				Images:        req.Images,
				ContentFormat: req.ContentFormat,
				ClientIP:      req.ClientIP,
			})
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *replyTopicReq, resp *ForumReplyDTO) {
			response.Created(c, "回复成功", resp)
		},
	}.Handle(c)
}

// UpdateTopic 编辑自己的帖子（#811）
// @Summary 编辑自己的帖子
// @Description 仅作者本人可改 title/content/images/category；非本人 403，主题不存在 404；类别值域 discussion|question|experience，空串归一 discussion；问答帖不得挂章节（与发帖同规则）
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题ID"
// @Success 200 {object} response.R{data=forum.ForumTopicDTO} "修改成功"
// @Failure 400 {object} response.R "参数错误（类别非法/长度越界/图片非法）"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "非作者本人"
// @Failure 404 {object} response.R "主题不存在"
// @Router /forum/topics/{id} [put]
func (h *handler) UpdateTopic(c *gin.Context) {
	httpx.Endpoint[updateTopicReq, ForumTopicDTO]{
		Parse: func(c *gin.Context) (*updateTopicReq, error) {
			topicID, err := httpx.PathInt64(c, "id", "主题ID无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				Category string   `json:"category"`
				Title    string   `json:"title"`
				Content  string   `json:"content"`
				Images   []string `json:"images"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, httpx.BadRequest("请求参数错误")
			}
			return &updateTopicReq{
				UserID:   middleware.CurrentUserID(c),
				TopicID:  topicID,
				Category: body.Category,
				Title:    body.Title,
				Content:  body.Content,
				Images:   body.Images,
			}, nil
		},
		Invoke: func(ctx context.Context, req *updateTopicReq) (*ForumTopicDTO, error) {
			return h.svc.UpdateTopic(UpdateTopicInput{
				UserID:   req.UserID,
				TopicID:  req.TopicID,
				Category: req.Category,
				Title:    req.Title,
				Content:  req.Content,
				Images:   req.Images,
			})
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *updateTopicReq, resp *ForumTopicDTO) {
			response.SuccessWithMsg(c, "修改成功", resp)
		},
	}.Handle(c)
}

// DeleteTopic 删除自己的帖子
// @Summary 删除自己的帖子
// @Description 仅本人可删
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题ID"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "非作者本人（票5 所有权档）"
// @Failure 404 {object} response.R "主题不存在"
// @Router /forum/topics/{id} [delete]
func (h *handler) DeleteTopic(c *gin.Context) {
	httpx.Endpoint[topicDeleteReq, struct{}]{
		Parse: func(c *gin.Context) (*topicDeleteReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			topicID, err := httpx.PathInt64(c, "id", "主题ID无效")
			if err != nil {
				return nil, err
			}
			return &topicDeleteReq{TopicID: topicID, UserID: userID}, nil
		},
		Invoke: func(ctx context.Context, req *topicDeleteReq) (*struct{}, error) {
			if err := h.svc.DeleteTopic(req.UserID, req.TopicID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *topicDeleteReq, _ *struct{}) {
			response.SuccessWithMsg(c, "已删除", nil)
		},
	}.Handle(c)
}

// DeleteReply 删除自己的回复
// @Summary 删除自己的回复
// @Description 仅本人可删
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "回复ID"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "非作者本人（票5 所有权档）"
// @Failure 404 {object} response.R "回复不存在"
// @Router /forum/replies/{id} [delete]
func (h *handler) DeleteReply(c *gin.Context) {
	httpx.Endpoint[replyDeleteReq, struct{}]{
		Parse: func(c *gin.Context) (*replyDeleteReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			replyID, err := httpx.PathInt64(c, "id", "回复ID无效")
			if err != nil {
				return nil, err
			}
			return &replyDeleteReq{ReplyID: replyID, UserID: userID}, nil
		},
		Invoke: func(ctx context.Context, req *replyDeleteReq) (*struct{}, error) {
			if err := h.svc.DeleteReply(req.UserID, req.ReplyID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *replyDeleteReq, _ *struct{}) {
			response.SuccessWithMsg(c, "已删除", nil)
		},
	}.Handle(c)
}

// listTopicsReq 主题列表请求（查询参数）。
type listTopicsReq struct {
	Scope    string
	Category string
	Solved   string
	Featured string
	// IsExperience 经验认定筛选（ADR-0040）：空 = 不过滤；true = 仅经验；false = 仅非经验。
	IsExperience string
	ChapterID    int
	Page         int
	PageSize     int
	Keyword      string
	Sort         string
	Order        string
}

// createTopicReq 发帖请求。
type createTopicReq struct {
	UserID    int
	ChapterID *int
	Category  string
	Title     string
	Content   string
	// ContentFormat 正文格式声明（ADR-0044）。空串由 service 归一为 text。
	ContentFormat string
	Images        []string
	// ClientIP 发布请求的客户端 IP（可信取 IP 单点），属地快照的输入（ADR-0045）。
	ClientIP string
}

// updateTopicReq 编辑帖子请求（#811）：chapter_id 不在契约内（编辑不迁移章节归属）。
// 编辑路径**不**携带 ClientIP：属地是发布那一刻的事实，编辑不写该字段。
type updateTopicReq struct {
	UserID   int
	TopicID  int64
	Category string
	Title    string
	Content  string
	Images   []string
}

// topicGetReq 主题详情请求。
type topicGetReq struct {
	TopicID int64
	UserID  int
	Sort    string
	Order   string
	// Page/PageSize 回复分页（ADR-0042）：回复列表的唯一读取形态是分页，旧的「一次性全量」已退役。
	Page     int
	PageSize int
}

// toDetailInput 学员端与管理端详情共用同一份 req→service 入参映射（两处逐字重复会漂移）。
func (r *topicGetReq) toDetailInput() TopicDetailInput {
	return TopicDetailInput{
		TopicID: r.TopicID, ViewerID: r.UserID,
		ReplySort: r.Sort, Order: r.Order,
		Page: r.Page, PageSize: r.PageSize,
	}
}

// replyTopicReq 回复请求。
type replyTopicReq struct {
	UserID        int
	TopicID       int64
	Content       string
	ParentReplyID *int64
	Images        []string
	// ContentFormat 正文格式声明（ADR-0044）。空串由 service 归一为 text。
	ContentFormat string
	// ClientIP 发布请求的客户端 IP（可信取 IP 单点），属地快照的输入（ADR-0045）。
	ClientIP string
}

// topicDeleteReq 删除自己主题请求。
type topicDeleteReq struct {
	TopicID int64
	UserID  int
}

// replyDeleteReq 删除自己回复请求。
type replyDeleteReq struct {
	ReplyID int64
	UserID  int
}

// topicIDReq 管理员删除主题请求。
type topicIDReq struct {
	TopicID int64
}

// replyIDReq 管理员删除回复请求。
type replyIDReq struct {
	ReplyID int64
}

// LikeTopic 点赞帖子
// @Summary 点赞帖子
// @Description 幂等
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题ID"
// @Success 200 {object} response.R{data=forum.ForumLikeResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "主题不存在（票5 存在性档）"
// @Router /forum/topics/{id}/like [post]
func (h *handler) LikeTopic(c *gin.Context) {
	topicID, err := httpx.PathInt64(c, "id", "主题 ID 无效")
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	count, err := h.svc.LikeTopic(middleware.CurrentUserID(c), topicID)
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.SuccessWithMsg(c, "点赞成功", ForumLikeResultDTO{Liked: true, LikesCount: count})
}

// UnlikeTopic 取消点赞帖子
// @Summary 取消点赞帖子
// @Description 幂等
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题ID"
// @Success 200 {object} response.R{data=forum.ForumLikeResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "主题不存在（票5 存在性档）"
// @Router /forum/topics/{id}/like [delete]
func (h *handler) UnlikeTopic(c *gin.Context) {
	topicID, err := httpx.PathInt64(c, "id", "主题 ID 无效")
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	count, err := h.svc.UnlikeTopic(middleware.CurrentUserID(c), topicID)
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.SuccessWithMsg(c, "已取消点赞", ForumLikeResultDTO{Liked: false, LikesCount: count})
}

// ReportTopic 举报帖子
// @Summary 举报帖子
// @Description 提交举报，等待审核
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题ID"
// @Param body body object true "原因" example({"reason":"违规"})
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "主题不存在（票5 存在性档）"
// @Router /forum/topics/{id}/report [post]
func (h *handler) ReportTopic(c *gin.Context) {
	h.report(c, "topic")
}

// ReportReply 举报回复
// @Summary 举报回复
// @Description 提交举报，等待审核
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "回复ID"
// @Param body body object true "原因" example({"reason":"违规"})
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "回复不存在（票5 存在性档）"
// @Router /forum/replies/{id}/report [post]
func (h *handler) ReportReply(c *gin.Context) {
	h.report(c, "reply")
}

// report 举报公共实现（kind: topic / reply，目标 ID 取路径参数 id）。
func (h *handler) report(c *gin.Context, kind string) {
	id, err := httpx.PathInt64(c, "id", "目标 ID 无效")
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	var v int64 = id
	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	var topicID, replyID *int64
	if kind == "topic" {
		topicID = &v
	} else {
		replyID = &v
	}
	if err := h.svc.CreateReport(middleware.CurrentUserID(c), topicID, replyID, body.Reason); err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.SuccessWithMsg(c, "举报已提交，等待处理", nil)
}

// MyTopics 我的帖子
// @Summary 我的帖子
// @Description 分页查询本人帖子
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Success 200 {object} response.R{data=forum.ForumTopicPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/my-topics [get]
func (h *handler) MyTopics(c *gin.Context) {
	resp, err := h.svc.MyTopics(middleware.CurrentUserID(c),
		httpx.QueryIntDefault(c, "page", 1), httpx.QueryIntDefault(c, "page_size", 10))
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.Success(c, resp)
}

// MyReplies 我的回复
// @Summary 我的回复
// @Description 分页查询本人回复
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Success 200 {object} response.R{data=forum.MyReplyPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/my-replies [get]
func (h *handler) MyReplies(c *gin.Context) {
	resp, err := h.svc.MyReplies(middleware.CurrentUserID(c),
		httpx.QueryIntDefault(c, "page", 1), httpx.QueryIntDefault(c, "page_size", 10))
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.Success(c, resp)
}

// MyLikedTopics 赞过（#701：响应逐字沿用 my-topics 形态）
// @Summary 赞过的帖子
// @Description 按点赞时间倒序；主题被删时条目保留、标题回空串
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Success 200 {object} response.R{data=forum.ForumTopicPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/my-liked-topics [get]
func (h *handler) MyLikedTopics(c *gin.Context) {
	resp, err := h.svc.MyLikedTopics(middleware.CurrentUserID(c),
		httpx.QueryIntDefault(c, "page", 1), httpx.QueryIntDefault(c, "page_size", 10))
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.Success(c, resp)
}

// MyObservedTopics 围观（#701：响应逐字沿用 my-topics 形态）
// @Summary 围观的帖子
// @Description 浏览过但未互动（排除本人发帖/回复/主题点赞/主题收藏）；按最近浏览倒序
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Success 200 {object} response.R{data=forum.ForumTopicPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/my-observed [get]
func (h *handler) MyObservedTopics(c *gin.Context) {
	resp, err := h.svc.MyObservedTopics(middleware.CurrentUserID(c),
		httpx.QueryIntDefault(c, "page", 1), httpx.QueryIntDefault(c, "page_size", 10))
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.Success(c, resp)
}

// MyViewHistory 浏览记录（#701：响应逐字沿用 my-topics 形态）
// @Summary 浏览记录
// @Description 按主题去重取最近一次浏览，按最近浏览倒序；主题被删时条目保留
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Success 200 {object} response.R{data=forum.ForumTopicPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/my-view-history [get]
func (h *handler) MyViewHistory(c *gin.Context) {
	resp, err := h.svc.MyViewHistory(middleware.CurrentUserID(c),
		httpx.QueryIntDefault(c, "page", 1), httpx.QueryIntDefault(c, "page_size", 10))
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.Success(c, resp)
}

// LikeReply 点赞回复
// @Summary 点赞回复
// @Description 幂等
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "回复ID"
// @Success 200 {object} response.R{data=forum.ForumLikeResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "回复不存在（票5 存在性档）"
// @Router /forum/replies/{id}/like [post]
func (h *handler) LikeReply(c *gin.Context) {
	replyID, err := httpx.PathInt64(c, "id", "回复 ID 无效")
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	count, err := h.svc.LikeReply(middleware.CurrentUserID(c), replyID)
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.SuccessWithMsg(c, "点赞成功", ForumLikeResultDTO{Liked: true, LikesCount: count})
}

// UnlikeReply 取消点赞回复
// @Summary 取消点赞回复
// @Description 幂等
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "回复ID"
// @Success 200 {object} response.R{data=forum.ForumLikeResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "回复不存在（票5 存在性档）"
// @Router /forum/replies/{id}/like [delete]
func (h *handler) UnlikeReply(c *gin.Context) {
	replyID, err := httpx.PathInt64(c, "id", "回复 ID 无效")
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	count, err := h.svc.UnlikeReply(middleware.CurrentUserID(c), replyID)
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.SuccessWithMsg(c, "已取消点赞", ForumLikeResultDTO{Liked: false, LikesCount: count})
}

// AcceptTopic 采纳回答（#366）
// @Summary 采纳回答
// @Description 仅楼主可采纳；首次采纳给答主+40/楼主+5（每帖只发一次分），更换不发分，楼主采纳自己零分
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题ID"
// @Param body body object true "采纳" example({"reply_id":123})
// @Success 200 {object} response.R{data=forum.ForumTopicDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "主题/回复不存在（票5 存在性档）"
// @Router /forum/topics/{id}/accept [post]
func (h *handler) AcceptTopic(c *gin.Context) {
	topicID, err := httpx.PathInt64(c, "id", "主题 ID 无效")
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	var body struct {
		ReplyID int64 `json:"reply_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.ReplyID <= 0 {
		response.BadRequest(c, "reply_id 不能为空且需大于 0")
		return
	}
	topic, err := h.svc.AcceptReply(middleware.CurrentUserID(c), topicID, body.ReplyID)
	if err != nil {
		ErrStatus.RenderError(c, err) // 票5：#366 手写 owner→403 链收编进域表
		return
	}
	response.SuccessWithMsg(c, "已采纳", topic)
}

// CancelAccept 取消采纳（#366）
// @Summary 取消采纳
// @Description 仅楼主可取消；状态回到未解决，已发分不回滚
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题ID"
// @Success 200 {object} response.R{data=forum.ForumTopicDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "主题不存在（票5 存在性档）"
// @Router /forum/topics/{id}/accept [delete]
func (h *handler) CancelAccept(c *gin.Context) {
	topicID, err := httpx.PathInt64(c, "id", "主题 ID 无效")
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	topic, err := h.svc.CancelAccept(middleware.CurrentUserID(c), topicID)
	if err != nil {
		ErrStatus.RenderError(c, err) // 票5：#366 手写 owner→403 链收编进域表
		return
	}
	response.SuccessWithMsg(c, "已取消采纳", topic)
}
