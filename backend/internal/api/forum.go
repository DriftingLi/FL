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

// ForumHandler 论坛 handler。
type ForumHandler struct {
	svc      *service.ForumService
	imageSvc *service.ForumImageService
}

// NewForumHandler 创建论坛 handler。
func NewForumHandler(svc *service.ForumService, imageSvc *service.ForumImageService) *ForumHandler {
	return &ForumHandler{svc: svc, imageSvc: imageSvc}
}

// RegisterForumRoutes 注册 /api/forum 蓝图（需登录，hrwai_user）。
func RegisterForumRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.ForumService, imageSvc *service.ForumImageService) {
	h := NewForumHandler(svc, imageSvc)

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

	// ===== 管理员论坛管理 =====
	adminG := rg.Group("/admin/forum", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin"))
	adminG.GET("/topics", h.ListTopics)
	adminG.GET("/topics/:id", h.AdminGetTopic)
	adminG.DELETE("/topics/:id", h.AdminDeleteTopic)
	adminG.DELETE("/replies/:id", h.AdminDeleteReply)
}

// UploadImage 上传论坛图片 POST /api/forum/upload-image
// 返回统一信封：{ code: 0, message: "图片上传成功", data: { url } }
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

// ListTopics 主题列表 GET /api/forum/topics?scope=all|general|chapter&chapter_id=&page=&page_size=&keyword=
func (h *ForumHandler) ListTopics(c *gin.Context) {
	Endpoint[listTopicsReq, service.ForumTopicPageResult]{
		Invoke: func(ctx context.Context, req *listTopicsReq) (*service.ForumTopicPageResult, error) {
			return h.svc.ListTopics(req.Scope, req.ChapterID, req.Page, req.PageSize, req.Keyword)
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

// CreateTopic 发帖 POST /api/forum/topics
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

// GetTopic 主题详情（含回复）GET /api/forum/topics/:id
func (h *ForumHandler) GetTopic(c *gin.Context) {
	Endpoint[topicGetReq, map[string]any]{
		Parse: func(c *gin.Context) (*topicGetReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			topicID, err := pathInt64(c, "id", "主题ID无效")
			if err != nil {
				return nil, err
			}
			return &topicGetReq{TopicID: topicID, UserID: userID}, nil
		},
		Invoke: func(ctx context.Context, req *topicGetReq) (*map[string]any, error) {
			result, err := h.svc.GetTopic(req.TopicID, req.UserID)
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

// ReplyTopic 回复 POST /api/forum/topics/:id/replies
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

// DeleteTopic 删除自己的主题 DELETE /api/forum/topics/:id
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

// DeleteReply 删除自己的回复 DELETE /api/forum/replies/:id
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

// AdminGetTopic 管理员查看帖子详情（含回复）GET /api/admin/forum/topics/:id
func (h *ForumHandler) AdminGetTopic(c *gin.Context) {
	Endpoint[topicGetReq, map[string]any]{
		Parse: func(c *gin.Context) (*topicGetReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			topicID, err := pathInt64(c, "id", "主题ID无效")
			if err != nil {
				return nil, err
			}
			return &topicGetReq{TopicID: topicID, UserID: userID}, nil
		},
		Invoke: func(ctx context.Context, req *topicGetReq) (*map[string]any, error) {
			result, err := h.svc.GetTopic(req.TopicID, req.UserID)
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
