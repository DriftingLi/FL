// Package job 实现 HTTP handlers。
// 本文件：招聘域举报与强制下架（spec #449 T5 #454）。
//   - 学员侧 POST /api/jobs/:id/report：举报职位
//   - 管理端 /api/admin/jobs*：只读巡检职位列表（可按企业筛）+ 举报队列 + 强制下架 + 标记已处理
package job

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// ReportErrStatus 举报治理域哨兵→状态码表（#611）：职位/举报不存在 → 404，
// 其余（原因为空等业务校验）兜底 400。
var ReportErrStatus = &httpx.ErrStatusTable{
	Entries: []httpx.ErrStatusEntry{
		{Sentinel: ErrReportJobNotFound, Status: http.StatusNotFound},
		{Sentinel: ErrReportNotFound, Status: http.StatusNotFound},
	},
	Fallback: http.StatusBadRequest,
}

// RegisterJobReportRoutes 注册举报与治理路由。
// jobSvc 提供职位巡检列表（*ReportService 只管举报与下架动作）。
func RegisterReportRoutes(rg *gin.RouterGroup, session *security.Session, svc *ReportService, jobSvc *Service) {
	h := newReportHandler(svc, jobSvc)
	// 学员侧举报
	studentG := rg.Group("/jobs", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapJobReport))
	studentG.POST("/:id/report", h.Report)
	// 管理端只读巡检 + 处置（#1639：组级不再挂单一能力，两条读面各挂各的候选）
	adminG := rg.Group("/admin", middleware.JWTAuth(session))
	adminG.GET("/jobs", middleware.CapabilityRequired(authz.CapJobReportHandle), h.ListAll)
	adminG.POST("/jobs/:id/force-offline", middleware.CapabilityRequired(authz.CapJobReportHandle), h.ForceOffline)
	adminG.POST("/job-reports/:id/handle", middleware.CapabilityRequired(authz.CapJobReportHandle), h.MarkHandled)
	// 举报队列是巡检视图的一格：只挂 job_report.handle 会把只读巡检的人挡在这条读面之外。
	adminG.GET("/job-reports", middleware.CapabilityRequired(authz.CapJobReportHandle, authz.CapInspectionRead), h.ListReports)
}

// JobReportHandler 举报治理 handler。
type reportHandler struct {
	svc    *ReportService
	jobSvc *Service
}

// NewJobReportHandler 创建举报治理 handler。
func newReportHandler(svc *ReportService, jobSvc *Service) *reportHandler {
	return &reportHandler{svc: svc, jobSvc: jobSvc}
}

// Report 学员举报职位 POST /api/jobs/:id/report
// @Summary 举报职位
// @Description 学员举报可疑职位（同一学员对同一职位唯一，重复举报合并；不挂论坛举报表）
// @Tags 招聘域-内容治理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "职位 ID"
// @Param body body ReportInput true "举报原因"
// @Success 201 {object} response.R{data=job.ReportDTO} "举报已提交"
// @Failure 400 {object} response.R "原因不能为空"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "职位不存在或已下架"
// @Router /jobs/{id}/report [post]
func (h *reportHandler) Report(c *gin.Context) {
	httpx.Endpoint[ReportInput, ReportDTO]{
		Parse: func(c *gin.Context) (*ReportInput, error) {
			return httpx.BindJSON[ReportInput](c)
		},
		Invoke: func(ctx context.Context, req *ReportInput) (*ReportDTO, error) {
			id, err := httpx.PathInt(c, "id", "职位 ID 无效")
			if err != nil {
				return nil, err
			}
			return h.svc.Report(middleware.CurrentUserID(c), id, req.Reason)
		},
		ErrStatus: ReportErrStatus,
		Render: func(c *gin.Context, _ *ReportInput, resp *ReportDTO) {
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
// @Success 200 {object} response.R{data=job.JobListResult} "列表"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/jobs [get]
func (h *reportHandler) ListAll(c *gin.Context) {
	httpx.Endpoint[struct{}, JobListResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*JobListResult, error) {
			params := JobListParams{
				Page:     httpx.QueryIntDefault(c, "page", 1),
				PageSize: httpx.QueryIntDefault(c, "page_size", 20),
				All:      true,
			}
			if v := httpx.QueryIDPtr(c, "recruiter_id"); v != nil {
				params.RecruiterID = *v
			}
			if v := httpx.QueryIDPtr(c, "position_id"); v != nil {
				params.PositionID = v
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
// @Success 200 {object} response.R{data=job.ReportListResult} "列表"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/job-reports [get]
func (h *reportHandler) ListReports(c *gin.Context) {
	httpx.Endpoint[struct{}, ReportListResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*ReportListResult, error) {
			page := httpx.QueryIntDefault(c, "page", 1)
			pageSize := httpx.QueryIntDefault(c, "page_size", 20)
			items, total, err := h.svc.ListPendingReports(page, pageSize)
			if err != nil {
				return nil, err
			}
			return &ReportListResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
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
// @Success 200 {object} response.R{data=job.ReportDTO} "已处理（响应为处理后的举报行）"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "举报不存在"
// @Router /admin/job-reports/{id}/handle [post]
func (h *reportHandler) MarkHandled(c *gin.Context) {
	httpx.Endpoint[struct{}, ReportDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*ReportDTO, error) {
			id, err := httpx.PathInt64(c, "id", "举报 ID 无效")
			if err != nil {
				return nil, err
			}
			return h.svc.MarkHandled(id)
		},
		ErrStatus: ReportErrStatus,
		Render: func(c *gin.Context, _ *struct{}, resp *ReportDTO) {
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
// @Success 200 {object} response.R{data=job.JobPostingDTO} "已强制下架（响应为下架后的职位行）"
// @Failure 400 {object} response.R "原因不能为空"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "职位不存在"
// @Router /admin/jobs/{id}/force-offline [post]
// body: { reason: string }。处置动作经 AuditLog 中间件自动记入审计日志。
func (h *reportHandler) ForceOffline(c *gin.Context) {
	httpx.Endpoint[struct{}, JobPostingDTO]{
		Parse: func(c *gin.Context) (*struct{}, error) {
			return &struct{}{}, nil
		},
		Invoke: func(ctx context.Context, _ *struct{}) (*JobPostingDTO, error) {
			id, err := httpx.PathInt(c, "id", "职位 ID 无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				Reason string `json:"reason"`
			}
			_ = c.ShouldBindJSON(&body)
			return h.svc.ForceOffline(id, body.Reason)
		},
		ErrStatus: ReportErrStatus,
		Render: func(c *gin.Context, _ *struct{}, resp *JobPostingDTO) {
			response.SuccessWithMsg(c, "职位已强制下架", *resp)
		},
	}.Handle(c)
}
