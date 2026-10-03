// Package audit 审计域：HTTP 出口（handler.go）与审计日志实现（service.go）。
// 本文件：审计日志查询（管理员后台）。
// 装配点：internal/api/routes_registry.go 调 audit.RegisterRoutes(api, rd.Session, deps.AuditSvc)。
package audit

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// AuditLogPageResult 审计日志分页结果。
type AuditLogPageResult struct {
	// Items 恒非 null：空表发出 []（pkg/paging 的 queryFind 走 gorm Find，零行给空切片）。
	Items []model.AuditLog `json:"items" nullability:"nonnil"`
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

// handler 审计日志 handler。
type handler struct {
	svc *Service
}

// newHandler 创建审计日志 handler。
func newHandler(svc *Service) *handler {
	return &handler{svc: svc}
}

// RegisterRoutes 注册 /api/admin/audit-logs 蓝图（仅管理员）。
// 只吃 *security.Session：原来的 RouterDeps 形参在本文件里只用到 Session 一格。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service) {
	h := newHandler(svc)

	g := rg.Group("/admin/audit-logs", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapAuditRead))

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
// @Success 200 {object} response.R{data=audit.AuditLogPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/audit-logs [get]
// List 审计日志列表 GET /api/admin/audit-logs?page=&page_size=&actor_id=&role=&keyword=
func (h *handler) List(c *gin.Context) {
	// 分页钳制（含页大小上限 100）收进 audit.Service.List，handler 只负责传参。
	httpx.Endpoint[auditLogListReq, AuditLogPageResult]{
		Parse: func(c *gin.Context) (*auditLogListReq, error) {
			return &auditLogListReq{
				Page:     httpx.QueryIntDefault(c, "page", 1),
				PageSize: httpx.QueryIntDefault(c, "page_size", 20),
				ActorID:  httpx.QueryIntDefault(c, "actor_id", 0),
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
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}
