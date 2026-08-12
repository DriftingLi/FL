// Package api 实现 HTTP handlers。
// 本文件：站内信通知（P0 通知基础设施，当前仅站内信渠道）。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
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
func RegisterNotificationRoutes(rg *gin.RouterGroup, sess *security.Session, svc *service.NotificationService) {
	h := NewNotificationHandler(svc)

	g := rg.Group("/notifications", middleware.JWTAuth(sess))

	// GET /api/notifications?page=&page_size= 分页查询通知（含未读数）
	g.GET("", h.List)
	// GET /api/notifications/unread-count 未读数
	g.GET("/unread-count", h.UnreadCount)
	// POST /api/notifications/:id/read 单条标记已读
	g.POST("/:id/read", h.MarkRead)
	// POST /api/notifications/read-all 全部标记已读
	g.POST("/read-all", h.MarkAllRead)
}

// List 分页查询通知（含未读数）GET /api/notifications?page=&page_size=
func (h *NotificationHandler) List(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 10)
	result, err := h.svc.List(userID, page, pageSize)
	if err != nil {
		response.ServerError(c, "查询失败: "+err.Error())
		return
	}
	response.Success(c, result)
}

// UnreadCount 未读数 GET /api/notifications/unread-count
func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	count, err := h.svc.UnreadCount(userID)
	if err != nil {
		response.ServerError(c, "查询失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"count": count})
}

// MarkRead 单条标记已读 POST /api/notifications/:id/read
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "通知ID无效")
		return
	}
	if err := h.svc.MarkRead(userID, id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "已标记为已读", nil)
}

// MarkAllRead 全部标记已读 POST /api/notifications/read-all
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	if err := h.svc.MarkAllRead(userID); err != nil {
		response.ServerError(c, "操作失败: "+err.Error())
		return
	}
	response.SuccessWithMsg(c, "已全部标记为已读", nil)
}
