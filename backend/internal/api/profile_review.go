// Package api 实现 HTTP handlers。
// 本文件：管理员审核用户资料（昵称/头像）修改。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/internal/storage"
	"forklift-training/pkg/response"
)

// RegisterProfileReviewRoutes 注册 /api/admin/profile-reviews 蓝图（仅管理员）。
func RegisterProfileReviewRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB, st storage.Storage, logger *zap.Logger) {
	svc := service.NewProfileReviewService(db, service.NewNotificationService(db, logger), st, logger)

	g := rg.Group("/admin/profile-reviews", middleware.JWTAuth(cfg), middleware.RoleRequired("admin"))

	// GET /api/admin/profile-reviews?status=pending|approved|rejected|all&page=&page_size=
	g.GET("", func(c *gin.Context) {
		status := c.Query("status")
		if status == "" {
			status = service.ProfileStatusPending
		}
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		result, err := svc.ListRequests(status, page, pageSize)
		if err != nil {
			response.ServerError(c, "查询失败: "+err.Error())
			return
		}
		response.Success(c, result)
	})

	// POST /api/admin/profile-reviews/:id/approve 通过审核
	g.POST("/:id/approve", func(c *gin.Context) {
		adminID, _ := c.Get(string(middleware.CtxUserID))
		reviewerID, _ := adminID.(int)
		requestID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || requestID <= 0 {
			response.BadRequest(c, "请求ID无效")
			return
		}
		reqDTO, err := svc.Approve(requestID, reviewerID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "已通过审核，修改已生效", reqDTO)
	})

	// POST /api/admin/profile-reviews/:id/reject 驳回（body: {"reason": "..."}）
	g.POST("/:id/reject", func(c *gin.Context) {
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
		reqDTO, err := svc.Reject(requestID, reviewerID, req.Reason)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		// 头像文件清理已下沉到审核模块内部（approve 清旧头像 / reject 清待审文件）
		response.SuccessWithMsg(c, "已驳回", reqDTO)
	})
}
