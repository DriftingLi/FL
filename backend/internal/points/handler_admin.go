// 本文件：积分域的管理端 HTTP 出口（ADR-0070）——/api/admin/points 蓝图（POST /admin/points/penalty）。
// 本域两条蓝图分居两文件，故 handler 类型与注册函数都带 Admin 后缀（同包不能有两个 RegisterRoutes）。
// 装配点：internal/api/routes_registry.go 调 points.RegisterAdminRoutes(api, rd.Session, deps.PointsSvc)。
package points

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
)

// adminHandler 管理员积分扣罚
type adminHandler struct {
	pointsSvc *Service
}

func newAdminHandler(pointsSvc *Service) *adminHandler {
	return &adminHandler{pointsSvc: pointsSvc}
}

func RegisterAdminRoutes(rg *gin.RouterGroup, session *security.Session, pointsSvc *Service) {
	h := newAdminHandler(pointsSvc)
	g := rg.Group("/admin/points", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapPointsAdmin))
	g.POST("/penalty", h.Penalty)
}

// adminPenaltyReq 管理员扣罚请求体（字段序不变，#1098 只把绑定文案搬进 Parse）。
type adminPenaltyReq struct {
	UserID int    `json:"user_id" binding:"required"`
	Delta  int    `json:"delta" binding:"required"`
	Reason string `json:"reason" binding:"required"`
}

// Penalty 管理员扣罚（#1098 扣罚即通知回归积分域）：
// 站内信编排（同事务强一致）已在 Service.AdminPenalty 内，handler 只剩
// 「解析 → 一行 invoke → 域表渲染」；通知写失败经 ErrStatus 渲染 500 + 可见原因。
// invoke 适配器不带 ctx，服务侧用请求作用域 ctx（罚分锁为瞬态护栏，Redis 不可用不阻断主流程）。
func (h *adminHandler) Penalty(c *gin.Context) {
	adminID := middleware.CurrentUserID(c)
	httpx.Endpoint[adminPenaltyReq, PointsPenaltyResultDTO]{
		Parse: httpx.BindJSONMsgFunc[adminPenaltyReq]("请求参数错误：user_id/delta/reason 必填"),
		Invoke: httpx.Invoke(func(req adminPenaltyReq) (PointsPenaltyResultDTO, error) {
			deducted, err := h.pointsSvc.AdminPenalty(c.Request.Context(), adminID, req.UserID, req.Delta, req.Reason)
			return PointsPenaltyResultDTO{Deducted: deducted}, err
		}),
		ErrStatus: ErrStatus,
	}.Handle(c)
}
