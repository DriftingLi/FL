// Package api 实现 HTTP handlers。
// 本文件：管理员审核用户资料（昵称/头像）修改。
package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// ProfileReviewHandler 资料审核 handler。
type ProfileReviewHandler struct {
	svc *service.ProfileReviewService
}

// NewProfileReviewHandler 创建资料审核 handler。
func NewProfileReviewHandler(svc *service.ProfileReviewService) *ProfileReviewHandler {
	return &ProfileReviewHandler{svc: svc}
}

// RegisterProfileReviewRoutes 注册 /api/admin/profile-reviews 蓝图（仅管理员）。
func RegisterProfileReviewRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.ProfileReviewService) {
	h := NewProfileReviewHandler(svc)

	g := rg.Group("/admin/profile-reviews", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapProfileReview))

	// GET /api/admin/profile-reviews?status=pending|approved|rejected|all&page=&page_size=
	g.GET("", h.ListRequests)
	// POST /api/admin/profile-reviews/:id/approve 通过审核
	g.POST("/:id/approve", h.Approve)
	// POST /api/admin/profile-reviews/:id/reject 驳回（body: {"reason": "..."}）
	g.POST("/:id/reject", h.Reject)
}

// listRequestsReq 审核请求列表查询参数。
type listRequestsReq struct {
	Status   string
	Page     int
	PageSize int
}

// @Summary 资料审核队列
// @Description 管理员分页查询昵称/头像修改审核单（status 缺省 pending）
// @Tags 管理端-资料审核
// @Produce json
// @Security BearerAuth
// @Param status query string false "状态 pending|approved|rejected|all" default(pending)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Success 200 {object} response.R{data=service.ProfileChangeRequestPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 500 {object} response.R "查询失败"
// @Router /admin/profile-reviews [get]
// ListRequests 审核请求列表 GET /api/admin/profile-reviews?status=pending|approved|rejected|all&page=&page_size=
func (h *ProfileReviewHandler) ListRequests(c *gin.Context) {
	Endpoint[listRequestsReq, service.ProfileChangeRequestPageResult]{
		Parse: func(c *gin.Context) (*listRequestsReq, error) {
			status := c.Query("status")
			if status == "" {
				status = service.ProfileStatusPending
			}
			return &listRequestsReq{
				Status:   status,
				Page:     atoiDefault(c.Query("page"), 1),
				PageSize: atoiDefault(c.Query("page_size"), 10),
			}, nil
		},
		Invoke: func(ctx context.Context, req *listRequestsReq) (*service.ProfileChangeRequestPageResult, error) {
			return h.svc.ListRequests(req.Status, req.Page, req.PageSize)
		},
		ErrStatus: errStatusAllPrefix(http.StatusInternalServerError, "查询失败: "),
		Render: func(c *gin.Context, _ *listRequestsReq, resp *service.ProfileChangeRequestPageResult) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// approveReq 通过审核请求（含路径 id 与审核人 id）。
type approveReq struct {
	RequestID  int64
	ReviewerID int
}

// @Summary 通过资料修改审核
// @Description 审核通过并生效（头像换新清旧），返回审核单
// @Tags 管理端-资料审核
// @Produce json
// @Security BearerAuth
// @Param id path int true "审核单 ID"
// @Success 200 {object} response.R{data=service.ProfileChangeRequestDTO} "已通过审核，修改已生效"
// @Failure 400 {object} response.R "审核失败"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/profile-reviews/{id}/approve [post]
// Approve 通过审核 POST /api/admin/profile-reviews/:id/approve
func (h *ProfileReviewHandler) Approve(c *gin.Context) {
	Endpoint[approveReq, service.ProfileChangeRequestDTO]{
		Parse: func(c *gin.Context) (*approveReq, error) {
			adminID, _ := c.Get(string(middleware.CtxUserID))
			reviewerID, _ := adminID.(int)
			requestID, err := pathInt64(c, "id", "请求ID无效")
			if err != nil {
				return nil, err
			}
			return &approveReq{RequestID: requestID, ReviewerID: reviewerID}, nil
		},
		Invoke: func(ctx context.Context, req *approveReq) (*service.ProfileChangeRequestDTO, error) {
			return h.svc.Approve(req.RequestID, req.ReviewerID)
		},
	}.WithSuccess(okMsg("已通过审核，修改已生效"), http.StatusBadRequest).Handle(c)
}

// rejectReq 驳回请求（含路径 id、审核人 id 与 reason）。
type rejectReq struct {
	RequestID  int64
	ReviewerID int
	Reason     string
}

// @Summary 驳回资料修改审核
// @Description 驳回并清理待审头像文件，返回审核单
// @Tags 管理端-资料审核
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "审核单 ID"
// @Param body body object false "驳回请求 {reason}"
// @Success 200 {object} response.R{data=service.ProfileChangeRequestDTO} "已驳回"
// @Failure 400 {object} response.R "驳回失败"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/profile-reviews/{id}/reject [post]
// Reject 驳回 POST /api/admin/profile-reviews/:id/reject（body: {"reason": "..."}）
func (h *ProfileReviewHandler) Reject(c *gin.Context) {
	Endpoint[rejectReq, service.ProfileChangeRequestDTO]{
		Parse: func(c *gin.Context) (*rejectReq, error) {
			adminID, _ := c.Get(string(middleware.CtxUserID))
			reviewerID, _ := adminID.(int)
			requestID, err := pathInt64(c, "id", "请求ID无效")
			if err != nil {
				return nil, err
			}
			var req struct {
				Reason string `json:"reason"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, badRequest("请求参数错误")
			}
			return &rejectReq{RequestID: requestID, ReviewerID: reviewerID, Reason: req.Reason}, nil
		},
		Invoke: func(ctx context.Context, req *rejectReq) (*service.ProfileChangeRequestDTO, error) {
			return h.svc.Reject(req.RequestID, req.ReviewerID, req.Reason)
		},
	}.WithSuccess(okMsg("已驳回"), http.StatusBadRequest).Handle(c)
}
