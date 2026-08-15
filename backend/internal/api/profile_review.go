// Package api 实现 HTTP handlers。
// 本文件：管理员审核用户资料（昵称/头像）修改。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

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

	g := rg.Group("/admin/profile-reviews", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin"))

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
		Render: func(c *gin.Context, _ *listRequestsReq, resp *service.ProfileChangeRequestPageResult, err error) {
			if err != nil {
				response.ServerError(c, "查询失败: "+err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// approveReq 通过审核请求（含路径 id 与审核人 id）。
type approveReq struct {
	RequestID  int64
	ReviewerID int
}

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
		Render: func(c *gin.Context, _ *approveReq, resp *service.ProfileChangeRequestDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "已通过审核，修改已生效", resp)
		},
	}.Handle(c)
}

// rejectReq 驳回请求（含路径 id、审核人 id 与 reason）。
type rejectReq struct {
	RequestID  int64
	ReviewerID int
	Reason     string
}

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
		Render: func(c *gin.Context, _ *rejectReq, resp *service.ProfileChangeRequestDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			// 头像文件清理已下沉到审核模块内部（approve 清旧头像 / reject 清待审文件）
			response.SuccessWithMsg(c, "已驳回", resp)
		},
	}.Handle(c)
}
