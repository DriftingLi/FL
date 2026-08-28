package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// PointsHandler 积分 handler
type PointsHandler struct {
	svc *service.PointsService
}

func NewPointsHandler(svc *service.PointsService) *PointsHandler {
	return &PointsHandler{svc: svc}
}

// RegisterPointsRoutes 注册 /api/points 蓝图（需登录，hrwai_user）
func RegisterPointsRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.PointsService) {
	h := NewPointsHandler(svc)
	g := rg.Group("/points", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))
	g.GET("/balance", h.GetBalance)
	g.GET("/ledger", h.GetLedger)
	g.GET("/tasks", h.GetTasks)
	g.POST("/tasks/:code/claim", h.Claim)
}

// GetBalance 获取余额
func (h *PointsHandler) GetBalance(c *gin.Context) {
	Endpoint[struct{}, service.PointsBalanceResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.PointsBalanceResult, error) {
			return h.svc.GetBalance(middleware.CurrentUserID(c))
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.PointsBalanceResult, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetLedger 流水
func (h *PointsHandler) GetLedger(c *gin.Context) {
	Endpoint[struct{}, service.PointsLedgerResult]{
		Parse: func(c *gin.Context) (*struct{}, error) { return &struct{}{}, nil },
		Invoke: func(ctx context.Context, _ *struct{}) (*service.PointsLedgerResult, error) {
			page := atoiDefault(c.Query("page"), 1)
			pageSize := atoiDefault(c.Query("page_size"), 20)
			return h.svc.GetLedger(middleware.CurrentUserID(c), page, pageSize)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.PointsLedgerResult, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetTasks 任务列表
func (h *PointsHandler) GetTasks(c *gin.Context) {
	Endpoint[struct{}, service.PointsTasksResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.PointsTasksResult, error) {
			return h.svc.GetTasks(middleware.CurrentUserID(c))
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.PointsTasksResult, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// Claim 领取
func (h *PointsHandler) Claim(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.BadRequest(c, "任务 code 不能为空")
		return
	}
	Endpoint[struct{}, service.PointsClaimResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.PointsClaimResult, error) {
			return h.svc.Claim(ctx, middleware.CurrentUserID(c), code)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.PointsClaimResult, err error) {
			if err != nil {
				if err.Error() == "已领取" || err.Error() == "今日已领取" {
					response.BadRequest(c, err.Error())
					return
				}
				if err.Error() == "任务不存在" {
					response.NotFound(c, err.Error())
					return
				}
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// helpers already in helpers.go (atoiDefault)
