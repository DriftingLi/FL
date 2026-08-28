package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// AdminPointsHandler 管理员积分扣罚
type AdminPointsHandler struct {
	pointsSvc       *service.PointsService
	notificationSvc *service.NotificationService
}

func NewAdminPointsHandler(pointsSvc *service.PointsService, notificationSvc *service.NotificationService) *AdminPointsHandler {
	return &AdminPointsHandler{pointsSvc: pointsSvc, notificationSvc: notificationSvc}
}

func RegisterAdminPointsRoutes(rg *gin.RouterGroup, rd RouterDeps, pointsSvc *service.PointsService, notificationSvc *service.NotificationService) {
	h := NewAdminPointsHandler(pointsSvc, notificationSvc)
	g := rg.Group("/admin/points", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin"))
	g.POST("/penalty", h.Penalty)
}

// Penalty 管理员扣罚
func (h *AdminPointsHandler) Penalty(c *gin.Context) {
	adminID := middleware.CurrentUserID(c)
	var body struct {
		UserID int    `json:"user_id" binding:"required"`
		Delta  int    `json:"delta" binding:"required"`
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误：user_id/delta/reason 必填")
		return
	}
	Endpoint[struct{}, map[string]any]{
		Invoke: func(ctx context.Context, _ *struct{}) (*map[string]any, error) {
			deducted, err := h.pointsSvc.AdminPenalty(ctx, adminID, body.UserID, body.Delta, body.Reason)
			if err != nil {
				return nil, err
			}
			// 站内信通知
			if h.notificationSvc != nil {
				b, _ := json.Marshal(map[string]any{"deducted": deducted, "reason": body.Reason})
				payload := model.JSONB(b)
				_ = h.notificationSvc.Create(body.UserID, "system", "积分扣罚", fmt.Sprintf("您的积分因“%s”被扣除 %d 分", body.Reason, deducted), "", payload)
			}
			m := map[string]any{"deducted": deducted}
			return &m, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *map[string]any, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}
