// Package api 实现 HTTP handlers。
// 本文件：审计日志查询（管理员后台）。
package api

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// AuditLogPageResult 审计日志分页结果。
type AuditLogPageResult struct {
	Items []model.AuditLog `json:"items"`
	Page  int              `json:"page"`
	Pages int              `json:"pages"`
	Total int64            `json:"total"`
}

// AuditHandler 审计日志 handler。
type AuditHandler struct {
	svc *service.AuditService
}

// NewAuditHandler 创建审计日志 handler。
func NewAuditHandler(svc *service.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// RegisterAuditRoutes 注册 /api/admin/audit-logs 蓝图（仅管理员）。
func RegisterAuditRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.AuditService) {
	h := NewAuditHandler(svc)

	g := rg.Group("/admin/audit-logs", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin"))

	// GET /api/admin/audit-logs?page=&page_size=&actor_id=&role=&keyword=
	g.GET("", h.List)
}

// List 审计日志列表 GET /api/admin/audit-logs?page=&page_size=&actor_id=&role=&keyword=
func (h *AuditHandler) List(c *gin.Context) {
	// 分页钳制（含页大小上限 100）收进 AuditService.List，handler 只负责传参。
	Endpoint[struct{}, AuditLogPageResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*AuditLogPageResult, error) {
			page := atoiDefault(c.Query("page"), 1)
			pageSize := atoiDefault(c.Query("page_size"), 20)

			actorID := atoiDefault(c.Query("actor_id"), 0)
			role := strings.TrimSpace(c.Query("role"))
			keyword := strings.TrimSpace(c.Query("keyword"))

			logs, total, page, pageSize := h.svc.List(page, pageSize, actorID, role, keyword)
			return &AuditLogPageResult{
				Items: logs,
				Page:  page,
				Pages: response.PageCount(total, pageSize),
				Total: total,
			}, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *AuditLogPageResult, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}
