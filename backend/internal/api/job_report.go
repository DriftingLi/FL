// Package api 实现 HTTP handlers。
// 本文件：招聘域举报与强制下架（spec #449 T5 #454）。
//   - 学员侧 POST /api/jobs/:id/report：举报职位
//   - 管理端 /api/admin/jobs*：只读巡检职位列表（可按企业筛）+ 举报队列 + 强制下架 + 标记已处理
package api

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterJobReportRoutes 注册举报与治理路由。
// jobSvc 提供职位巡检列表（JobReportService 只管举报与下架动作）。
func RegisterJobReportRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.JobReportService, jobSvc *service.JobPostingService) {
	h := NewJobReportHandler(svc, jobSvc)
	// 学员侧举报
	studentG := rg.Group("/jobs", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))
	studentG.POST("/:id/report", h.Report)
	// 管理端只读巡检 + 处置
	adminG := rg.Group("/admin", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin"))
	adminG.GET("/jobs", h.ListAll)
	adminG.GET("/job-reports", h.ListReports)
	adminG.POST("/job-reports/:id/handle", h.MarkHandled)
	adminG.POST("/jobs/:id/force-offline", h.ForceOffline)
}

// JobReportHandler 举报治理 handler。
type JobReportHandler struct {
	svc    *service.JobReportService
	jobSvc *service.JobPostingService
}

// NewJobReportHandler 创建举报治理 handler。
func NewJobReportHandler(svc *service.JobReportService, jobSvc *service.JobPostingService) *JobReportHandler {
	return &JobReportHandler{svc: svc, jobSvc: jobSvc}
}

// Report 学员举报职位 POST /api/jobs/:id/report
// @Summary 举报职位
// @Description 学员举报可疑职位（同一学员对同一职位唯一，重复举报合并；不挂论坛举报表）
// @Tags 招聘域-内容治理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "职位 ID"
// @Param body body service.ReportInput true "举报原因"
// @Success 201 {object} response.R "举报已提交"
// @Failure 400 {object} response.R "原因不能为空"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "职位不存在或已下架"
// @Router /jobs/{id}/report [post]
func (h *JobReportHandler) Report(c *gin.Context) {
	Endpoint[service.ReportInput, service.ReportDTO]{
		Parse: func(c *gin.Context) (*service.ReportInput, error) {
			return bindJSON[service.ReportInput](c)
		},
		Invoke: func(ctx context.Context, req *service.ReportInput) (*service.ReportDTO, error) {
			id, err := pathInt(c, "id", "职位 ID 无效")
			if err != nil {
				return nil, err
			}
			return h.svc.Report(middleware.CurrentUserID(c), id, req.Reason)
		},
		Render: func(c *gin.Context, _ *service.ReportInput, resp *service.ReportDTO, err error) {
			if err != nil {
				renderReportError(c, err)
				return
			}
			response.Created(c, "举报已提交，感谢你的反馈", *resp)
		},
	}.Handle(c)
}

// ListAll 管理端职位列表（只读巡检，可按企业筛）GET /api/admin/jobs
// @Summary 职位巡检
// @Description 管理端只读巡检职位（全量含 closed/强制下架，可按企业筛）
// @Tags 招聘域-内容治理
// @Produce json
// @Security BearerAuth
// @Param recruiter_id query int false "企业 ID"
// @Param specialty_id query int false "专业方向 ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.R "列表"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/jobs [get]
func (h *JobReportHandler) ListAll(c *gin.Context) {
	Endpoint[struct{}, service.JobListResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.JobListResult, error) {
			params := service.JobListParams{
				Page:     atoiDefault(c.Query("page"), 1),
				PageSize: atoiDefault(c.Query("page_size"), 20),
				All:      true,
			}
			if v := queryIDPtr(c, "recruiter_id"); v != nil {
				params.RecruiterID = *v
			}
			if v := queryIDPtr(c, "specialty_id"); v != nil {
				params.SpecialtyID = v
			}
			// 管理端跨企业全量（recruiterID=0 且非 MineOnly 时服务层不加企业过滤）
			return h.jobSvc.List(0, params)
		},
	}.Handle(c)
}

// ListReports 管理端举报队列 GET /api/admin/job-reports
// @Summary 举报队列
// @Description 管理端查看待处理举报队列
// @Tags 招聘域-内容治理
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.R "列表"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/job-reports [get]
func (h *JobReportHandler) ListReports(c *gin.Context) {
	Endpoint[struct{}, service.ReportListResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.ReportListResult, error) {
			page := atoiDefault(c.Query("page"), 1)
			pageSize := atoiDefault(c.Query("page_size"), 20)
			items, total, err := h.svc.ListPendingReports(page, pageSize)
			if err != nil {
				return nil, err
			}
			return &service.ReportListResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
		},
	}.Handle(c)
}

// MarkHandled 管理端标记举报已处理 POST /api/admin/job-reports/:id/handle
// @Summary 标记举报已处理
// @Description 管理端把举报标记为已处理（处理后不再出现在待处理队列）
// @Tags 招聘域-内容治理
// @Produce json
// @Security BearerAuth
// @Param id path int true "举报 ID"
// @Success 200 {object} response.R "已处理"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "举报不存在"
// @Router /admin/job-reports/{id}/handle [post]
func (h *JobReportHandler) MarkHandled(c *gin.Context) {
	Endpoint[struct{}, service.ReportDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.ReportDTO, error) {
			id, err := pathInt64(c, "id", "举报 ID 无效")
			if err != nil {
				return nil, err
			}
			return h.svc.MarkHandled(id)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.ReportDTO, err error) {
			if err != nil {
				renderReportError(c, err)
				return
			}
			response.SuccessWithMsg(c, "举报已标记为已处理", *resp)
		},
	}.Handle(c)
}

// ForceOffline 管理端强制下架职位 POST /api/admin/jobs/:id/force-offline
// @Summary 强制下架职位
// @Description 管理端带原因强制下架职位（学员侧立即不可见；企业不能自行重新上架；处置入审计日志、邮件通知企业）
// @Tags 招聘域-内容治理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "职位 ID"
// @Param body body object false "下架原因 {reason: string}"
// @Success 200 {object} response.R "已强制下架"
// @Failure 400 {object} response.R "原因不能为空"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "职位不存在"
// @Router /admin/jobs/{id}/force-offline [post]
// body: { reason: string }。处置动作经 AuditLog 中间件自动记入审计日志。
func (h *JobReportHandler) ForceOffline(c *gin.Context) {
	Endpoint[struct{}, service.JobPostingDTO]{
		Parse: func(c *gin.Context) (*struct{}, error) {
			return &struct{}{}, nil
		},
		Invoke: func(ctx context.Context, _ *struct{}) (*service.JobPostingDTO, error) {
			id, err := pathInt(c, "id", "职位 ID 无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				Reason string `json:"reason"`
			}
			_ = c.ShouldBindJSON(&body)
			return h.svc.ForceOffline(id, body.Reason)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.JobPostingDTO, err error) {
			if err != nil {
				renderReportError(c, err)
				return
			}
			response.SuccessWithMsg(c, "职位已强制下架", *resp)
		},
	}.Handle(c)
}

// renderReportError 举报治理错误映射（哨兵 → 状态码，禁文案比对）。
func renderReportError(c *gin.Context, err error) {
	var pe *ParseError
	switch {
	case asParseError(err, &pe):
		renderStatus(c, pe.Status, pe.Message)
	case errors.Is(err, service.ErrReportJobNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrReportNotFound):
		response.NotFound(c, err.Error())
	default:
		response.BadRequest(c, err.Error())
	}
}
