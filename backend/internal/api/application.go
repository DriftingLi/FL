// Package api 实现 HTTP handlers。
// 本文件：招聘域投递端点（spec #449 T3 #452）。
//   - 学员侧 /api/jobs/:id/apply：投递即授权
//   - 学员侧 /api/resume/applications*：我的投递（列表/撤回），对齐既有「学员侧招聘数据挂在简历前缀下」
package api

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterApplicationRoutes 注册投递相关路由。
func RegisterApplicationRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.JobApplicationService) {
	h := NewApplicationHandler(svc)
	// 学员侧职位投递动作挂在 /api/jobs 下
	studentJobG := rg.Group("/jobs", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))
	studentJobG.POST("/:id/apply", h.Apply)
	// 我的投递挂在 /api/resume 前缀下（对齐既有「学员侧招聘数据挂在简历前缀下」的写法）
	studentG := rg.Group("/resume", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))
	studentG.GET("/applications", h.ListMine)
	studentG.POST("/applications/:id/withdraw", h.Withdraw)
}

// ApplicationHandler 投递 handler。
type ApplicationHandler struct {
	svc *service.JobApplicationService
}

// NewApplicationHandler 创建投递 handler。
func NewApplicationHandler(svc *service.JobApplicationService) *ApplicationHandler {
	return &ApplicationHandler{svc: svc}
}

// Apply 学员投递职位 POST /api/jobs/:id/apply
func (h *ApplicationHandler) Apply(c *gin.Context) {
	Endpoint[struct{}, service.ApplicationDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.ApplicationDTO, error) {
			id, err := pathInt(c, "id", "职位 ID 无效")
			if err != nil {
				return nil, err
			}
			return h.svc.Apply(middleware.CurrentUserID(c), id)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.ApplicationDTO, err error) {
			if err != nil {
				renderApplyError(c, err)
				return
			}
			response.Created(c, "投递成功，企业已可查看你的联系方式", *resp)
		},
	}.Handle(c)
}

// ListMine 我的投递列表 GET /api/resume/applications
func (h *ApplicationHandler) ListMine(c *gin.Context) {
	Endpoint[struct{}, service.ApplicationListResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.ApplicationListResult, error) {
			page := atoiDefault(c.Query("page"), 1)
			pageSize := atoiDefault(c.Query("page_size"), 20)
			items, total, err := h.svc.ListForStudent(middleware.CurrentUserID(c), page, pageSize)
			if err != nil {
				return nil, err
			}
			return &service.ApplicationListResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
		},
	}.Handle(c)
}

// Withdraw 撤回投递 POST /api/resume/applications/:id/withdraw
// body: { revoke_contact?: boolean } 默认 false——撤回投递默认不连带收回联系方式授权。
func (h *ApplicationHandler) Withdraw(c *gin.Context) {
	Endpoint[struct{}, service.ApplicationDTO]{
		Parse: func(c *gin.Context) (*struct{}, error) {
			return &struct{}{}, nil
		},
		Invoke: func(ctx context.Context, _ *struct{}) (*service.ApplicationDTO, error) {
			id, err := pathInt64(c, "id", "投递 ID 无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				RevokeContact bool `json:"revoke_contact"`
			}
			_ = c.ShouldBindJSON(&body)
			return h.svc.Withdraw(middleware.CurrentUserID(c), id, body.RevokeContact)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.ApplicationDTO, err error) {
			if err != nil {
				renderApplyError(c, err)
				return
			}
			response.SuccessWithMsg(c, "投递已撤回", *resp)
		},
	}.Handle(c)
}

// renderApplyError 投递域错误映射（哨兵 → 状态码，禁文案比对）。
func renderApplyError(c *gin.Context, err error) {
	var pe *ParseError
	switch {
	case asParseError(err, &pe):
		renderStatus(c, pe.Status, pe.Message)
	case errors.Is(err, service.ErrApplyJobInactive), errors.Is(err, service.ErrJobNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrApplyNotYours):
		response.Forbidden(c, err.Error())
	default:
		response.BadRequest(c, err.Error())
	}
}
