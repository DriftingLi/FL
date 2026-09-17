package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
)

// AdminPointsHandler 管理员积分扣罚
type AdminPointsHandler struct {
	pointsSvc *service.PointsService
}

func NewAdminPointsHandler(pointsSvc *service.PointsService) *AdminPointsHandler {
	return &AdminPointsHandler{pointsSvc: pointsSvc}
}

func RegisterAdminPointsRoutes(rg *gin.RouterGroup, rd RouterDeps, pointsSvc *service.PointsService) {
	h := NewAdminPointsHandler(pointsSvc)
	g := rg.Group("/admin/points", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapPointsAdmin))
	g.POST("/penalty", h.Penalty)
}

// adminPenaltyReq 管理员扣罚请求体（字段序不变，#1098 只把绑定文案搬进 Parse）。
type adminPenaltyReq struct {
	UserID int    `json:"user_id" binding:"required"`
	Delta  int    `json:"delta" binding:"required"`
	Reason string `json:"reason" binding:"required"`
}

// Penalty 管理员扣罚（#1098 扣罚即通知回归积分域）：
// 站内信编排（同事务强一致）已在 PointsService.AdminPenalty 内，handler 只剩
// 「解析 → 一行 invoke → 域表渲染」；通知写失败经 pointsErrStatus 渲染 500 + 可见原因。
// invoke 适配器不带 ctx，服务侧用请求作用域 ctx（罚分锁为瞬态护栏，Redis 不可用不阻断主流程）。
func (h *AdminPointsHandler) Penalty(c *gin.Context) {
	adminID := middleware.CurrentUserID(c)
	Endpoint[adminPenaltyReq, service.PointsPenaltyResultDTO]{
		Parse: bindJSONMsgFunc[adminPenaltyReq]("请求参数错误：user_id/delta/reason 必填"),
		Invoke: invoke(func(req adminPenaltyReq) (service.PointsPenaltyResultDTO, error) {
			deducted, err := h.pointsSvc.AdminPenalty(c.Request.Context(), adminID, req.UserID, req.Delta, req.Reason)
			return service.PointsPenaltyResultDTO{Deducted: deducted}, err
		}),
		ErrStatus: pointsErrStatus,
	}.Handle(c)
}
