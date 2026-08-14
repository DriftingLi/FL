// Package api 实现 HTTP handlers。
// 本文件：管理员审核用户资料（昵称/头像）修改。
package api

import (
	"strconv"

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

// ListRequests 审核请求列表 GET /api/admin/profile-reviews?status=pending|approved|rejected|all&page=&page_size=
func (h *ProfileReviewHandler) ListRequests(c *gin.Context) {
	status := c.Query("status")
	if status == "" {
		status = service.ProfileStatusPending
	}
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 10)
	result, err := h.svc.ListRequests(status, page, pageSize)
	if err != nil {
		response.ServerError(c, "查询失败: "+err.Error())
		return
	}
	response.Success(c, result)
}

// Approve 通过审核 POST /api/admin/profile-reviews/:id/approve
func (h *ProfileReviewHandler) Approve(c *gin.Context) {
	adminID, _ := c.Get(string(middleware.CtxUserID))
	reviewerID, _ := adminID.(int)
	requestID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || requestID <= 0 {
		response.BadRequest(c, "请求ID无效")
		return
	}
	reqDTO, err := h.svc.Approve(requestID, reviewerID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "已通过审核，修改已生效", reqDTO)
}

// Reject 驳回 POST /api/admin/profile-reviews/:id/reject（body: {"reason": "..."}）
func (h *ProfileReviewHandler) Reject(c *gin.Context) {
	adminID, _ := c.Get(string(middleware.CtxUserID))
	reviewerID, _ := adminID.(int)
	requestID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || requestID <= 0 {
		response.BadRequest(c, "请求ID无效")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	reqDTO, err := h.svc.Reject(requestID, reviewerID, req.Reason)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// 头像文件清理已下沉到审核模块内部（approve 清旧头像 / reject 清待审文件）
	response.SuccessWithMsg(c, "已驳回", reqDTO)
}
