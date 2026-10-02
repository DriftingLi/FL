// 本文件：投稿域的管理端 HTTP 出口（ADR-0070）——/api/admin/contributions 蓝图（审核队列 + 举报队列）。
// 本域两条蓝图分居两文件，故注册函数带 Admin 后缀；handler 类型与 handler.go 共用。
package contribution

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

// RegisterAdminRoutes 注册 /api/admin/contributions 蓝图（管理端 6 条；讲师端 V1 无 UI）。
// 本域两条蓝图分居两文件，故注册函数带 Admin 后缀（同包不能有两个 RegisterRoutes）；handler 类型与 handler.go 共用。
func RegisterAdminRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service) {
	h := newHandler(svc)

	// ===== 管理端审核队列（admin + tutor；讲师前端二期）=====
	adminG := rg.Group("/admin/contributions", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapContributionReview))
	// GET /api/admin/contributions/pending 待审核队列
	adminG.GET("/pending", h.ListPending)
	// POST /api/admin/contributions/:id/approve 通过（发分）
	adminG.POST("/:id/approve", h.Approve)
	// POST /api/admin/contributions/:id/reject 驳回（必填原因）
	adminG.POST("/:id/reject", h.Reject)
	// POST /api/admin/contributions/:id/archive 下架（追回积分）
	adminG.POST("/:id/archive", h.Archive)
	// GET /api/admin/contributions/reports 举报队列
	adminG.GET("/reports", h.ListReports)
	// POST /api/admin/contributions/reports/:id/handle 处置举报
	adminG.POST("/reports/:id/handle", h.HandleReport)
}

// ListPending 待审核队列 GET /api/admin/contributions/pending
// @Summary 待审核投稿队列
// @Description 管理端/讲师端（tutor+admin 鉴权）；V1 仅管理端有 UI
// @Tags 管理端-投稿
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R{data=contribution.ContributionPageResult} "success"
// @Router /admin/contributions/pending [get]
func (h *handler) ListPending(c *gin.Context) {
	httpx.Endpoint[struct{}, ContributionPageResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*ContributionPageResult, error) {
			return h.svc.ListPending(httpx.QueryIntDefault(c, "page", 1), httpx.QueryIntDefault(c, "page_size", 20))
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// Approve 通过投稿 POST /api/admin/contributions/:id/approve
// @Summary 审核通过（发分 +50）
// @Description pending → approved，直记 +50 积分（幂等占坑防双发）+ 站内信，同事务
// @Tags 管理端-投稿
// @Produce json
// @Security BearerAuth
// @Param id path int true "投稿ID"
// @Success 200 {object} response.R{data=contribution.ContributionItemDTO} "已通过"
// @Failure 400 {object} response.R "非 pending"
// @Router /admin/contributions/{id}/approve [post]
func (h *handler) Approve(c *gin.Context) {
	httpx.Endpoint[struct{}, ContributionItemDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*ContributionItemDTO, error) {
			id, err := httpx.PathInt64(c, "id", "投稿ID无效")
			if err != nil {
				return nil, err
			}
			rid, err := currentUserID(c)
			if err != nil {
				return nil, err
			}
			return h.svc.Approve(rid, id)
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *struct{}, resp *ContributionItemDTO) {
			response.SuccessWithMsg(c, "已通过", resp)
		},
	}.Handle(c)
}

// contributionRejectReq 驳回请求体（必填原因）。
type contributionRejectReq struct {
	Reason string `json:"reason"`
}

// Reject 驳回投稿 POST /api/admin/contributions/:id/reject
// @Summary 驳回投稿
// @Description pending → rejected，原因必填（送达作者站内信）。不发分。重提=新建投稿
// @Tags 管理端-投稿
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "投稿ID"
// @Param body body contributionRejectReq true "驳回原因"
// @Success 200 {object} response.R{data=contribution.ContributionItemDTO} "已驳回"
// @Failure 400 {object} response.R "原因必填或非 pending"
// @Router /admin/contributions/{id}/reject [post]
func (h *handler) Reject(c *gin.Context) {
	httpx.Endpoint[contributionRejectReq, ContributionItemDTO]{
		Parse: func(c *gin.Context) (*contributionRejectReq, error) { return httpx.BindJSON[contributionRejectReq](c) },
		Invoke: func(ctx context.Context, req *contributionRejectReq) (*ContributionItemDTO, error) {
			id, err := httpx.PathInt64(c, "id", "投稿ID无效")
			if err != nil {
				return nil, err
			}
			rid, err := currentUserID(c)
			if err != nil {
				return nil, err
			}
			return h.svc.Reject(rid, id, req.Reason)
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *contributionRejectReq, resp *ContributionItemDTO) {
			response.SuccessWithMsg(c, "已驳回", resp)
		},
	}.Handle(c)
}

// Archive 下架投稿 POST /api/admin/contributions/:id/archive
// @Summary 下架投稿（追回积分）
// @Description approved → archived，必填原因；追回该稿累计投稿分（过审+达阶，rollback 对冲封底 0）+ 站内信
// @Tags 管理端-投稿
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "投稿ID"
// @Param body body contributionRejectReq true "下架原因"
// @Success 200 {object} response.R{data=contribution.ContributionItemDTO} "已下架"
// @Failure 400 {object} response.R "原因必填或非 approved"
// @Router /admin/contributions/{id}/archive [post]
func (h *handler) Archive(c *gin.Context) {
	httpx.Endpoint[contributionRejectReq, ContributionItemDTO]{
		Parse: func(c *gin.Context) (*contributionRejectReq, error) { return httpx.BindJSON[contributionRejectReq](c) },
		Invoke: func(ctx context.Context, req *contributionRejectReq) (*ContributionItemDTO, error) {
			id, err := httpx.PathInt64(c, "id", "投稿ID无效")
			if err != nil {
				return nil, err
			}
			rid, err := currentUserID(c)
			if err != nil {
				return nil, err
			}
			return h.svc.Archive(rid, id, req.Reason)
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *contributionRejectReq, resp *ContributionItemDTO) {
			response.SuccessWithMsg(c, "已下架", resp)
		},
	}.Handle(c)
}

// ListReports 举报队列 GET /api/admin/contributions/reports
// @Summary 投稿举报队列
// @Description status 0 待处理 / 1 已处理，缺省全部
// @Tags 管理端-投稿
// @Produce json
// @Security BearerAuth
// @Param status query int false "状态 0|1"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R{data=contribution.ContributionReportPageResult} "success"
// @Router /admin/contributions/reports [get]
func (h *handler) ListReports(c *gin.Context) {
	httpx.Endpoint[struct{}, ContributionReportPageResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*ContributionReportPageResult, error) {
			var status *int
			if s := c.Query("status"); s != "" {
				// status 0 待处理 / 1 已处理；缺失表示不筛这一维（全部）
				if v, ok := httpx.PositiveID(s); ok {
					status = &v
				} else if s == "0" {
					z := 0
					status = &z
				}
			}
			return h.svc.ListReports(httpx.QueryIntDefault(c, "page", 1), httpx.QueryIntDefault(c, "page_size", 20), status)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// handleReportReq 处置举报请求体。
type handleReportReq struct {
	Action string `json:"action"` // archive=下架被举报投稿 / dismiss=驳回举报
}

// HandleReport 处置举报 POST /api/admin/contributions/reports/:id/handle
// @Summary 处置投稿举报
// @Description action=archive 下架被举报投稿（追回积分）并标记处理；action=dismiss 驳回举报
// @Tags 管理端-投稿
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "举报ID"
// @Param body body handleReportReq true "处置动作"
// @Success 200 {object} response.R "已处理"
// @Router /admin/contributions/reports/{id}/handle [post]
func (h *handler) HandleReport(c *gin.Context) {
	httpx.Endpoint[handleReportReq, struct{}]{
		Parse: func(c *gin.Context) (*handleReportReq, error) { return httpx.BindJSON[handleReportReq](c) },
		Invoke: func(ctx context.Context, req *handleReportReq) (*struct{}, error) {
			id, err := httpx.PathInt64(c, "id", "举报ID无效")
			if err != nil {
				return nil, err
			}
			rid, err := currentUserID(c)
			if err != nil {
				return nil, err
			}
			if err := h.svc.HandleReport(rid, id, req.Action); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
	}.WithSuccess(httpx.OkMsgNoData("已处理"), http.StatusBadRequest).Handle(c)
}
