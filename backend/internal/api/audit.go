// Package api 实现 HTTP handlers。
// 本文件：审计日志查询（管理员后台）。
package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// AuditLogPageResult 审计日志分页结果。
type AuditLogPageResult struct {
	Items []model.AuditLog `json:"items" extensions:"x-nullable" nullability:"nullable"`
	Page  int              `json:"page"`
	Pages int              `json:"pages"`
	Total int64            `json:"total"`
}

// auditLogListReq 审计日志列表查询参数。
type auditLogListReq struct {
	Page     int
	PageSize int
	ActorID  int
	Role     string
	Keyword  string
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

	g := rg.Group("/admin/audit-logs", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapAuditRead))

	// GET /api/admin/audit-logs?page=&page_size=&actor_id=&role=&keyword=
	g.GET("", h.List)
}

// @Summary 审计日志列表
// @Description 管理员分页查询审计日志（actor/角色/关键字过滤，页大小上限 100）
// @Tags 管理端-审计
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param actor_id query int false "操作人 ID"
// @Param role query string false "操作人角色"
// @Param keyword query string false "关键字"
// @Success 200 {object} response.R{data=api.AuditLogPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/audit-logs [get]
// List 审计日志列表 GET /api/admin/audit-logs?page=&page_size=&actor_id=&role=&keyword=
func (h *AuditHandler) List(c *gin.Context) {
	// 分页钳制（含页大小上限 100）收进 AuditService.List，handler 只负责传参。
	Endpoint[auditLogListReq, AuditLogPageResult]{
		Parse: func(c *gin.Context) (*auditLogListReq, error) {
			return &auditLogListReq{
				Page:     atoiDefault(c.Query("page"), 1),
				PageSize: atoiDefault(c.Query("page_size"), 20),
				ActorID:  atoiDefault(c.Query("actor_id"), 0),
				Role:     strings.TrimSpace(c.Query("role")),
				Keyword:  strings.TrimSpace(c.Query("keyword")),
			}, nil
		},
		Invoke: func(ctx context.Context, req *auditLogListReq) (*AuditLogPageResult, error) {
			logs, total, page, pageSize, err := h.svc.List(req.Page, req.PageSize, req.ActorID, req.Role, req.Keyword)
			if err != nil {
				return nil, err
			}
			return &AuditLogPageResult{
				Items: logs,
				Page:  page,
				Pages: response.PageCount(total, pageSize),
				Total: total,
			}, nil
		},
	}.WithSuccess(okMsg("success"), http.StatusInternalServerError).Handle(c)
}
