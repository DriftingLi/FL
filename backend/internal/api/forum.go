// Package api 实现 HTTP handlers。
// 本文件：学员端论坛（综合讨论区 + 章节讨论区，支持回复别人的回复）。
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// ForumHandler 论坛 handler（帖子/回复由 ForumService，打卡由 CheckInService）。
type ForumHandler struct {
	svc        *service.ForumService
	checkInSvc *service.CheckInService
	imageSvc   *service.ForumImageService
}

// NewForumHandler 创建论坛 handler。
func NewForumHandler(svc *service.ForumService, checkInSvc *service.CheckInService, imageSvc *service.ForumImageService) *ForumHandler {
	return &ForumHandler{svc: svc, checkInSvc: checkInSvc, imageSvc: imageSvc}
}

// RegisterForumRoutes 注册 /api/forum 蓝图（需登录，hrwai_user）。
func RegisterForumRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.ForumService, checkInSvc *service.CheckInService, imageSvc *service.ForumImageService) {
	h := NewForumHandler(svc, checkInSvc, imageSvc)

	g := rg.Group("/forum", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))

	// POST /api/forum/upload-image  上传论坛图片（图文分离，先传图后随发帖/回复提交 URL）
	g.POST("/upload-image", h.UploadImage)
	// GET /api/forum/topics?scope=all|general|chapter&chapter_id=&page=&page_size=&keyword=
	g.GET("/topics", h.ListTopics)
	// POST /api/forum/topics 发帖（chapter_id 为空/0 表示发到综合讨论区；images 为图片 URL 数组，最多 9 张）
	g.POST("/topics", h.CreateTopic)
	// GET /api/forum/topics/:id 主题详情（含回复）
	g.GET("/topics/:id", h.GetTopic)
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

	// ===== 打卡（spec #268）=====
	g.POST("/check-in", h.CheckIn)
	g.GET("/check-in/calendar", h.GetCheckInCalendar)
	g.GET("/check-in/rank", h.GetCheckInRank)
	// ===== 评论点赞（spec #268）=====
	g.POST("/replies/:id/like", h.LikeReply)
	g.DELETE("/replies/:id/like", h.UnlikeReply)

	// ===== 管理员论坛管理 =====
	adminG := rg.Group("/admin/forum", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin"))
	adminG.GET("/topics", h.ListTopics)
	adminG.GET("/topics/:id", h.AdminGetTopic)
	adminG.DELETE("/topics/:id", h.AdminDeleteTopic)
	adminG.DELETE("/replies/:id", h.AdminDeleteReply)
	// 举报管理（ADR-0018）：status query 0 待处理 / 1 已处理，缺省全部
	adminG.GET("/reports", h.ListReports)
	adminG.PUT("/reports/:id", h.HandleReport)
}

// UploadImage 上传论坛图片
// @Summary 上传论坛图片
// @Description 图文分离，先传图后随发帖/回复提交 URL；支持论坛图片
// @Tags 学员端-论坛
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "图片文件"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/upload-image [post]
func (h *ForumHandler) UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "未找到上传文件")
		return
	}
	url, err := h.imageSvc.Upload(c.Request.Context(), file)
	if err != nil {
		var fe *service.ForumImageError
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
	response.SuccessWithMsg(c, "图片上传成功", gin.H{"url": url})
}

// ListTopics 帖子列表
// @Summary 帖子列表
// @Description 支持 scope=all|general|chapter，按 chapter_id/keyword/sort=latest|hot 过滤
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param scope query string false "范围 all|general|chapter"
// @Param chapter_id query int false "章节ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param keyword query string false "关键词"
// @Param sort query string false "排序 latest|hot"
// @Success 200 {object} response.R{data=service.ForumTopicPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/topics [get]
func (h *ForumHandler) ListTopics(c *gin.Context) {
	Endpoint[listTopicsReq, service.ForumTopicPageResult]{
		Parse: func(c *gin.Context) (*listTopicsReq, error) {
			return &listTopicsReq{
				Scope:     c.Query("scope"),
				ChapterID: atoiDefault(c.Query("chapter_id"), 0),
				Page:      atoiDefault(c.Query("page"), 1),
				PageSize:  atoiDefault(c.Query("page_size"), 10),
				Keyword:   c.Query("keyword"),
				Sort:      c.Query("sort"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *listTopicsReq) (*service.ForumTopicPageResult, error) {
			return h.svc.ListTopics(req.Scope, req.ChapterID, req.Page, req.PageSize, req.Keyword, req.Sort)
		},
		Render: func(c *gin.Context, _ *listTopicsReq, resp *service.ForumTopicPageResult, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// CreateTopic 发帖
// @Summary 发帖
// @Description chapter_id 为空表示综合讨论区；images 最多 9 张 URL
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "帖子" example({"title":"标题","content":"内容","images":[]})
// @Success 201 {object} response.R{data=service.ForumTopicDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/topics [post]
func (h *ForumHandler) CreateTopic(c *gin.Context) {
	Endpoint[createTopicReq, service.ForumTopicDTO]{
		Parse: func(c *gin.Context) (*createTopicReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			var body struct {
				ChapterID *int     `json:"chapter_id"`
				Title     string   `json:"title"`
				Content   string   `json:"content"`
				Images    []string `json:"images"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("请求参数错误")
			}
			return &createTopicReq{UserID: userID, ChapterID: body.ChapterID, Title: body.Title, Content: body.Content, Images: body.Images}, nil
		},
		Invoke: func(ctx context.Context, req *createTopicReq) (*service.ForumTopicDTO, error) {
			return h.svc.CreateTopic(req.UserID, req.ChapterID, req.Title, req.Content, req.Images)
		},
		Render: func(c *gin.Context, _ *createTopicReq, resp *service.ForumTopicDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
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
// @Param sort query string false "排序 time|hot"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "不存在"
// @Router /forum/topics/{id} [get]
func (h *ForumHandler) GetTopic(c *gin.Context) {
	Endpoint[topicGetReq, map[string]any]{
		Parse: func(c *gin.Context) (*topicGetReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			topicID, err := pathInt64(c, "id", "主题ID无效")
			if err != nil {
				return nil, err
			}
			return &topicGetReq{TopicID: topicID, UserID: userID, Sort: c.Query("sort")}, nil
		},
		Invoke: func(ctx context.Context, req *topicGetReq) (*map[string]any, error) {
			result, err := h.svc.GetTopic(req.TopicID, req.UserID, req.Sort)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *topicGetReq, resp *map[string]any, err error) {
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					response.NotFound(c, "主题不存在")
					return
				}
				response.ServerError(c, "查询失败: "+err.Error())
				return
			}
			response.Success(c, resp)
		},
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
// @Success 201 {object} response.R{data=service.ForumReplyDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/topics/{id}/replies [post]
func (h *ForumHandler) ReplyTopic(c *gin.Context) {
	Endpoint[replyTopicReq, service.ForumReplyDTO]{
		Parse: func(c *gin.Context) (*replyTopicReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			topicID, err := pathInt64(c, "id", "主题ID无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				Content       string   `json:"content"`
				ParentReplyID *int64   `json:"parent_reply_id"`
				Images        []string `json:"images"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("请求参数错误")
			}
			return &replyTopicReq{UserID: userID, TopicID: topicID, Content: body.Content, ParentReplyID: body.ParentReplyID, Images: body.Images}, nil
		},
		Invoke: func(ctx context.Context, req *replyTopicReq) (*service.ForumReplyDTO, error) {
			return h.svc.ReplyTopic(req.UserID, req.TopicID, req.Content, req.ParentReplyID, req.Images)
		},
		Render: func(c *gin.Context, _ *replyTopicReq, resp *service.ForumReplyDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "回复成功", resp)
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
// @Router /forum/topics/{id} [delete]
func (h *ForumHandler) DeleteTopic(c *gin.Context) {
	Endpoint[topicDeleteReq, struct{}]{
		Parse: func(c *gin.Context) (*topicDeleteReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			topicID, err := pathInt64(c, "id", "主题ID无效")
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
		Render: func(c *gin.Context, _ *topicDeleteReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
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
// @Router /forum/replies/{id} [delete]
func (h *ForumHandler) DeleteReply(c *gin.Context) {
	Endpoint[replyDeleteReq, struct{}]{
		Parse: func(c *gin.Context) (*replyDeleteReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			replyID, err := pathInt64(c, "id", "回复ID无效")
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
		Render: func(c *gin.Context, _ *replyDeleteReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "已删除", nil)
		},
	}.Handle(c)
}

// AdminGetTopic 管理员查看帖子详情（含回复）GET /api/admin/forum/topics/:id?sort=time|hot
func (h *ForumHandler) AdminGetTopic(c *gin.Context) {
	Endpoint[topicGetReq, map[string]any]{
		Parse: func(c *gin.Context) (*topicGetReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			topicID, err := pathInt64(c, "id", "主题ID无效")
			if err != nil {
				return nil, err
			}
			return &topicGetReq{TopicID: topicID, UserID: userID, Sort: c.Query("sort")}, nil
		},
		Invoke: func(ctx context.Context, req *topicGetReq) (*map[string]any, error) {
			result, err := h.svc.GetTopic(req.TopicID, req.UserID, req.Sort)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *topicGetReq, resp *map[string]any, err error) {
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					response.NotFound(c, "主题不存在")
					return
				}
				response.ServerError(c, "查询失败: "+err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// AdminDeleteTopic 管理员删除任意主题 DELETE /api/admin/forum/topics/:id
func (h *ForumHandler) AdminDeleteTopic(c *gin.Context) {
	Endpoint[topicIDReq, struct{}]{
		Parse: func(c *gin.Context) (*topicIDReq, error) {
			topicID, err := pathInt64(c, "id", "主题ID无效")
			if err != nil {
				return nil, err
			}
			return &topicIDReq{TopicID: topicID}, nil
		},
		Invoke: func(ctx context.Context, req *topicIDReq) (*struct{}, error) {
			if err := h.svc.AdminDeleteTopic(req.TopicID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *topicIDReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "已删除", nil)
		},
	}.Handle(c)
}

// AdminDeleteReply 管理员删除任意回复 DELETE /api/admin/forum/replies/:id
func (h *ForumHandler) AdminDeleteReply(c *gin.Context) {
	Endpoint[replyIDReq, struct{}]{
		Parse: func(c *gin.Context) (*replyIDReq, error) {
			replyID, err := pathInt64(c, "id", "回复ID无效")
			if err != nil {
				return nil, err
			}
			return &replyIDReq{ReplyID: replyID}, nil
		},
		Invoke: func(ctx context.Context, req *replyIDReq) (*struct{}, error) {
			if err := h.svc.AdminDeleteReply(req.ReplyID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *replyIDReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "已删除", nil)
		},
	}.Handle(c)
}

// listTopicsReq 主题列表请求（查询参数）。
type listTopicsReq struct {
	Scope     string
	ChapterID int
	Page      int
	PageSize  int
	Keyword   string
	Sort      string
}

// createTopicReq 发帖请求。
type createTopicReq struct {
	UserID    int
	ChapterID *int
	Title     string
	Content   string
	Images    []string
}

// topicGetReq 主题详情请求。
type topicGetReq struct {
	TopicID int64
	UserID  int
	Sort    string
}

// replyTopicReq 回复请求。
type replyTopicReq struct {
	UserID        int
	TopicID       int64
	Content       string
	ParentReplyID *int64
	Images        []string
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
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/topics/{id}/like [post]
func (h *ForumHandler) LikeTopic(c *gin.Context) {
	topicID, err := pathInt64(c, "id", "主题 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	count, err := h.svc.LikeTopic(middleware.CurrentUserID(c), topicID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "点赞成功", gin.H{"likes_count": count, "liked": true})
}

// UnlikeTopic 取消点赞帖子
// @Summary 取消点赞帖子
// @Description 幂等
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题ID"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/topics/{id}/like [delete]
func (h *ForumHandler) UnlikeTopic(c *gin.Context) {
	topicID, err := pathInt64(c, "id", "主题 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	count, err := h.svc.UnlikeTopic(middleware.CurrentUserID(c), topicID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "已取消点赞", gin.H{"likes_count": count, "liked": false})
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
// @Router /forum/topics/{id}/report [post]
func (h *ForumHandler) ReportTopic(c *gin.Context) {
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
// @Router /forum/replies/{id}/report [post]
func (h *ForumHandler) ReportReply(c *gin.Context) {
	h.report(c, "reply")
}

// report 举报公共实现（kind: topic / reply，目标 ID 取路径参数 id）。
func (h *ForumHandler) report(c *gin.Context, kind string) {
	id, err := pathInt64(c, "id", "目标 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
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
		response.BadRequest(c, err.Error())
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
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/my-topics [get]
func (h *ForumHandler) MyTopics(c *gin.Context) {
	resp, err := h.svc.MyTopics(middleware.CurrentUserID(c),
		atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("page_size"), 10))
	if err != nil {
		response.BadRequest(c, err.Error())
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
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/my-replies [get]
func (h *ForumHandler) MyReplies(c *gin.Context) {
	resp, err := h.svc.MyReplies(middleware.CurrentUserID(c),
		atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("page_size"), 10))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// ListReports 管理端举报列表 GET /api/admin/forum/reports?status=&page=&page_size=
func (h *ForumHandler) ListReports(c *gin.Context) {
	var status *int16
	if raw := c.Query("status"); raw != "" {
		v := int16(atoiDefault(raw, -1))
		if v != 0 && v != 1 {
			response.BadRequest(c, "status 仅支持 0（待处理）/ 1（已处理）")
			return
		}
		status = &v
	}
	resp, err := h.svc.ListReports(atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("page_size"), 20), status)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// HandleReport 处理举报 PUT /api/admin/forum/reports/:id
func (h *ForumHandler) HandleReport(c *gin.Context) {
	id, err := pathInt64(c, "id", "举报 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	var body struct {
		Status int16 `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := h.svc.HandleReport(id, body.Status); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "举报状态已更新", nil)
}

// CheckIn 每日打卡
// @Summary 每日打卡
// @Description Asia/Shanghai 每日一次，返回连击/排名
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "已打卡"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/check-in [post]
func (h *ForumHandler) CheckIn(c *gin.Context) {
	res, err := h.checkInSvc.CheckIn(middleware.CurrentUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "打卡成功", res)
}

// GetCheckInCalendar 打卡日历
// @Summary 打卡日历
// @Description 按年月查询打卡日历
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param year query int false "年份"
// @Param month query int false "月份 1-12"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/check-in/calendar [get]
func (h *ForumHandler) GetCheckInCalendar(c *gin.Context) {
	year := atoiDefault(c.Query("year"), 0)
	month := atoiDefault(c.Query("month"), 0)
	res, err := h.checkInSvc.GetCheckInCalendar(middleware.CurrentUserID(c), year, month)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// GetCheckInRank 打卡排行榜
// @Summary 打卡排行榜
// @Description 分页查询打卡排行榜
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/check-in/rank [get]
func (h *ForumHandler) GetCheckInRank(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	res, err := h.checkInSvc.GetCheckInRank(middleware.CurrentUserID(c), page, pageSize)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// LikeReply 点赞回复
// @Summary 点赞回复
// @Description 幂等
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "回复ID"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/replies/{id}/like [post]
func (h *ForumHandler) LikeReply(c *gin.Context) {
	replyID, err := pathInt64(c, "id", "回复 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	count, err := h.svc.LikeReply(middleware.CurrentUserID(c), replyID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "点赞成功", gin.H{"likes_count": count, "liked": true})
}

// UnlikeReply 取消点赞回复
// @Summary 取消点赞回复
// @Description 幂等
// @Tags 学员端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "回复ID"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /forum/replies/{id}/like [delete]
func (h *ForumHandler) UnlikeReply(c *gin.Context) {
	replyID, err := pathInt64(c, "id", "回复 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	count, err := h.svc.UnlikeReply(middleware.CurrentUserID(c), replyID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "已取消点赞", gin.H{"likes_count": count, "liked": false})
}
