// Package api 实现 HTTP handlers。
// 本文件：站内信通知（P0 通知基础设施，当前仅站内信渠道）。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// NotificationHandler 站内信通知 handler。
type NotificationHandler struct {
	svc *service.NotificationService
}

// NewNotificationHandler 创建站内信通知 handler。
func NewNotificationHandler(svc *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

// RegisterNotificationRoutes 注册 /api/notifications 蓝图（登录用户站内信）。
func RegisterNotificationRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.NotificationService) {
	h := NewNotificationHandler(svc)

	g := rg.Group("/notifications", middleware.JWTAuth(rd.Session))

	// GET /api/notifications?page=&page_size= 分页查询通知（含未读数）
	g.GET("", h.List)
	// GET /api/notifications/unread-count 未读数
	g.GET("/unread-count", h.UnreadCount)
	// POST /api/notifications/:id/read 单条标记已读
	g.POST("/:id/read", h.MarkRead)
	// POST /api/notifications/read-all 全部标记已读
	g.POST("/read-all", h.MarkAllRead)
}

// notificationListReq 通知列表请求（用户 ID + 分页）。
type notificationListReq struct {
	UserID   int
	Page     int
	PageSize int
}

// List 分页查询通知（含未读数）GET /api/notifications?page=&page_size=
func (h *NotificationHandler) List(c *gin.Context) {
	Endpoint[notificationListReq, service.NotificationListPageResult]{
		Parse: func(c *gin.Context) (*notificationListReq, error) {
			return &notificationListReq{
				UserID:   middleware.CurrentUserID(c),
				Page:     atoiDefault(c.Query("page"), 1),
				PageSize: atoiDefault(c.Query("page_size"), 10),
			}, nil
		},
		Invoke: func(ctx context.Context, req *notificationListReq) (*service.NotificationListPageResult, error) {
			return h.svc.List(req.UserID, req.Page, req.PageSize)
		},
		Render: func(c *gin.Context, _ *notificationListReq, resp *service.NotificationListPageResult, err error) {
			if err != nil {
				response.ServerError(c, "查询失败: "+err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// UnreadCount 未读数 GET /api/notifications/unread-count
func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	Endpoint[notificationUserIDReq, int64]{
		Parse: func(c *gin.Context) (*notificationUserIDReq, error) {
			return &notificationUserIDReq{UserID: middleware.CurrentUserID(c)}, nil
		},
		Invoke: func(ctx context.Context, req *notificationUserIDReq) (*int64, error) {
			count, err := h.svc.UnreadCount(req.UserID)
			if err != nil {
				return nil, err
			}
			return &count, nil
		},
		Render: func(c *gin.Context, _ *notificationUserIDReq, resp *int64, err error) {
			if err != nil {
				response.ServerError(c, "查询失败: "+err.Error())
				return
			}
			response.Success(c, gin.H{"count": *resp})
		},
	}.Handle(c)
}

// markReadReq 单条标记已读请求（路径 ID + 用户 ID）。
type markReadReq struct {
	UserID int
	ID     int64
}

// notificationUserIDReq 仅带登录用户 ID 的请求（未读数 / 全部已读）。
type notificationUserIDReq struct {
	UserID int
}

// MarkRead 单条标记已读 POST /api/notifications/:id/read
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	Endpoint[markReadReq, struct{}]{
		Parse: func(c *gin.Context) (*markReadReq, error) {
			id, err := pathInt64(c, "id", "通知ID无效")
			if err != nil {
				return nil, err
			}
			return &markReadReq{UserID: middleware.CurrentUserID(c), ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *markReadReq) (*struct{}, error) {
			if err := h.svc.MarkRead(req.UserID, req.ID); err != nil {
				return nil, err
			}
			return nil, nil
		},
		Render: func(c *gin.Context, _ *markReadReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "已标记为已读", nil)
		},
	}.Handle(c)
}

// MarkAllRead 全部标记已读 POST /api/notifications/read-all
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	Endpoint[notificationUserIDReq, struct{}]{
		Parse: func(c *gin.Context) (*notificationUserIDReq, error) {
			return &notificationUserIDReq{UserID: middleware.CurrentUserID(c)}, nil
		},
		Invoke: func(ctx context.Context, req *notificationUserIDReq) (*struct{}, error) {
			if err := h.svc.MarkAllRead(req.UserID); err != nil {
				return nil, err
			}
			return nil, nil
		},
		Render: func(c *gin.Context, _ *notificationUserIDReq, _ *struct{}, err error) {
			if err != nil {
				response.ServerError(c, "操作失败: "+err.Error())
				return
			}
			response.SuccessWithMsg(c, "已全部标记为已读", nil)
		},
	}.Handle(c)
}
