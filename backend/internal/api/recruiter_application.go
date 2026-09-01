// Package api 实现 HTTP handlers。
// 本文件：招聘域企业处理投递（spec #449 T4 #453）。
//   - GET /api/recruit/jobs/:id/applications：按职位分页查看投递（越权 403）+ 未读计数
//   - GET /api/recruit/applications/:id：投递详情（记录已读，脱敏候选人）
//   - POST /api/recruit/applications/:id/reject：标记不合适
package api

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterRecruiterApplicationRoutes 注册企业侧投递处理路由。
func RegisterRecruiterApplicationRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.JobApplicationService) {
	h := NewRecruiterApplicationHandler(svc)
	g := rg.Group("/recruit", middleware.JWTAuth(rd.Session), middleware.RoleRequired("recruiter"))
	g.GET("/jobs/:id/applications", h.ListByJob)
	g.GET("/applications/:id", h.GetDetail)
	g.POST("/applications/:id/reject", h.Reject)
}

// RecruiterApplicationHandler 企业侧投递处理 handler。
type RecruiterApplicationHandler struct {
	svc *service.JobApplicationService
}

// NewRecruiterApplicationHandler 创建企业侧投递处理 handler。
func NewRecruiterApplicationHandler(svc *service.JobApplicationService) *RecruiterApplicationHandler {
	return &RecruiterApplicationHandler{svc: svc}
}

// ListByJob 按职位分页查看投递 GET /api/recruit/jobs/:id/applications
// @Summary 按职位查看投递
// @Description 企业按职位分页查看投递（只能看自己的职位，越权 403；含未读投递数；候选人脱敏——姓名打码、无手机/微信/PDF/证书原图）
// @Tags 招聘域-投递处理
// @Produce json
// @Security BearerAuth
// @Param id path int true "职位 ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.R "列表（含 unread_count）"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "越权"
// @Router /recruit/jobs/{id}/applications [get]
func (h *RecruiterApplicationHandler) ListByJob(c *gin.Context) {
	Endpoint[struct{}, service.RecruiterApplicationListResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.RecruiterApplicationListResult, error) {
			jobID, err := pathInt(c, "id", "职位 ID 无效")
			if err != nil {
				return nil, err
			}
			page := atoiDefault(c.Query("page"), 1)
			pageSize := atoiDefault(c.Query("page_size"), 20)
			return h.svc.ListForRecruiter(middleware.CurrentUserID(c), jobID, page, pageSize)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.RecruiterApplicationListResult, err error) {
			if err != nil {
				renderRecruiterApplicationError(c, err)
				return
			}
			response.Success(c, *resp)
		},
	}.Handle(c)
}

// GetDetail 投递详情 GET /api/recruit/applications/:id（记录已读）
// @Summary 投递详情
// @Description 企业查看投递详情（打开即记录已读 employer_viewed_at；回显投递时刻简历更新时间，内容读最新不落快照；脱敏候选人）
// @Tags 招聘域-投递处理
// @Produce json
// @Security BearerAuth
// @Param id path int true "投递 ID"
// @Success 200 {object} response.R "详情"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "越权"
// @Router /recruit/applications/{id} [get]
func (h *RecruiterApplicationHandler) GetDetail(c *gin.Context) {
	Endpoint[struct{}, service.ApplicationDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.ApplicationDTO, error) {
			id, err := pathInt64(c, "id", "投递 ID 无效")
			if err != nil {
				return nil, err
			}
			return h.svc.GetForRecruiter(middleware.CurrentUserID(c), id)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.ApplicationDTO, err error) {
			if err != nil {
				renderRecruiterApplicationError(c, err)
				return
			}
			response.Success(c, *resp)
		},
	}.Handle(c)
}

// Reject 标记不合适 POST /api/recruit/applications/:id/reject
// @Summary 标记不合适
// @Description 企业把投递标记为不合适 → 终态 rejected（仅 applied 可拒；同一学员 30 天内不能再投）
// @Tags 招聘域-投递处理
// @Produce json
// @Security BearerAuth
// @Param id path int true "投递 ID"
// @Success 200 {object} response.R "已标记"
// @Failure 400 {object} response.R "状态不允许"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "越权"
// @Router /recruit/applications/{id}/reject [post]
func (h *RecruiterApplicationHandler) Reject(c *gin.Context) {
	Endpoint[struct{}, service.ApplicationDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.ApplicationDTO, error) {
			id, err := pathInt64(c, "id", "投递 ID 无效")
			if err != nil {
				return nil, err
			}
			return h.svc.Reject(middleware.CurrentUserID(c), id)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.ApplicationDTO, err error) {
			if err != nil {
				renderRecruiterApplicationError(c, err)
				return
			}
			response.SuccessWithMsg(c, "已标记为不合适", *resp)
		},
	}.Handle(c)
}

// renderRecruiterApplicationError 企业侧投递错误映射。
func renderRecruiterApplicationError(c *gin.Context, err error) {
	var pe *ParseError
	switch {
	case asParseError(err, &pe):
		renderStatus(c, pe.Status, pe.Message)
	case errors.Is(err, service.ErrJobNotFound), errors.Is(err, service.ErrApplyNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrApplyNotYours):
		response.Forbidden(c, err.Error())
	default:
		response.BadRequest(c, err.Error())
	}
}
