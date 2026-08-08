// Package api 实现 HTTP handlers。
// 本文件：审计日志查询（管理员后台）。
package api

import (
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/pkg/response"
)

// RegisterAuditRoutes 注册 /api/admin/audit-logs 蓝图（仅管理员）。
func RegisterAuditRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB, logger *zap.Logger) {
	g := rg.Group("/admin/audit-logs", middleware.JWTAuth(cfg), middleware.RoleRequired("admin"))

	// GET /api/admin/audit-logs?page=&page_size=&actor_id=&role=&keyword=
	g.GET("", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 20)
		if pageSize > 100 {
			pageSize = 100
		}

		q := db.Model(&model.AuditLog{})
		if actorID := atoiDefault(c.Query("actor_id"), 0); actorID > 0 {
			q = q.Where("actor_id = ?", actorID)
		}
		if role := strings.TrimSpace(c.Query("role")); role != "" {
			q = q.Where("actor_role = ?", role)
		}
		if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
			like := "%" + keyword + "%"
			q = q.Where("path ILIKE ? OR action ILIKE ? OR actor_name ILIKE ?", like, like, like)
		}

		var total int64
		if err := q.Count(&total).Error; err != nil {
			response.ServerError(c, "查询失败: "+err.Error())
			return
		}
		var logs []model.AuditLog
		if err := q.Order("id DESC").
			Offset((page - 1) * pageSize).
			Limit(pageSize).
			Find(&logs).Error; err != nil {
			response.ServerError(c, "查询失败: "+err.Error())
			return
		}
		pages := int((total + int64(pageSize) - 1) / int64(pageSize))
		response.Success(c, gin.H{
			"total": total,
			"page":  page,
			"pages": pages,
			"items": logs,
		})
	})
}
