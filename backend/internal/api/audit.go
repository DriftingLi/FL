// Package api 实现 HTTP handlers。
// 本文件：审计日志查询（管理员后台）。
package api

import (
	"strings"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/pkg/response"
)

// AuditLogPageResult 审计日志分页结果。
type AuditLogPageResult struct {
	Items []model.AuditLog `json:"items"`
	Page  int              `json:"page"`
	Pages int              `json:"pages"`
	Total int64            `json:"total"`
}

// RegisterAuditRoutes 注册 /api/admin/audit-logs 蓝图（仅管理员）。
func RegisterAuditRoutes(rg *gin.RouterGroup, deps *Deps) {
	g := rg.Group("/admin/audit-logs", middleware.JWTAuth(deps.Session), middleware.RoleRequired("admin"))
	db := deps.DB

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
		response.Success(c, AuditLogPageResult{
			Items: logs,
			Page:  page,
			Pages: response.PageCount(total, pageSize),
			Total: total,
		})
	})
}
