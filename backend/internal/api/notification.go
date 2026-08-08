// Package api 实现 HTTP handlers。
// 本文件：站内信通知（P0 通知基础设施，当前仅站内信渠道）。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterNotificationRoutes 注册 /api/notifications 蓝图（登录用户站内信）。
func RegisterNotificationRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB, logger *zap.Logger) {
	svc := service.NewNotificationService(db, logger)
	g := rg.Group("/notifications", middleware.JWTAuth(cfg))

	// GET /api/notifications?page=&page_size= 分页查询通知（含未读数）
	g.GET("", func(c *gin.Context) {
		userID := middleware.CurrentUserID(c)
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		result, err := svc.List(userID, page, pageSize)
		if err != nil {
			response.ServerError(c, "查询失败: "+err.Error())
			return
		}
		response.Success(c, result)
	})

	// GET /api/notifications/unread-count 未读数
	g.GET("/unread-count", func(c *gin.Context) {
		userID := middleware.CurrentUserID(c)
		count, err := svc.UnreadCount(userID)
		if err != nil {
			response.ServerError(c, "查询失败: "+err.Error())
			return
		}
		response.Success(c, gin.H{"count": count})
	})

	// POST /api/notifications/:id/read 单条标记已读
	g.POST("/:id/read", func(c *gin.Context) {
		userID := middleware.CurrentUserID(c)
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "通知ID无效")
			return
		}
		if err := svc.MarkRead(userID, id); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "已标记为已读", nil)
	})

	// POST /api/notifications/read-all 全部标记已读
	g.POST("/read-all", func(c *gin.Context) {
		userID := middleware.CurrentUserID(c)
		if err := svc.MarkAllRead(userID); err != nil {
			response.ServerError(c, "操作失败: "+err.Error())
			return
		}
		response.SuccessWithMsg(c, "已全部标记为已读", nil)
	})
}
