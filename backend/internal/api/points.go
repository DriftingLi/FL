package api

import (
	"context"
	"errors"

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
	g.POST("/shop/course/:courseId/redeem", h.RedeemCourse)
	g.POST("/shop/:sku/redeem", h.RedeemShop)
}

// GetBalance 获取余额
// GetBalance 积分余额 GET /api/points/balance
// @Summary 积分余额
// @Description 学员查看自己的积分余额
// @Tags 学员端-积分
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "余额"
// @Failure 401 {object} response.R "未认证"
// @Router /points/balance [get]
func (h *PointsHandler) GetBalance(c *gin.Context) {
	Endpoint[struct{}, service.PointsBalanceResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.PointsBalanceResult, error) {
			return h.svc.GetBalance(middleware.CurrentUserID(c))
		},
	}.Handle(c)
}

// GetLedger 流水
// GetLedger 积分流水 GET /api/points/ledger
// @Summary 积分流水
// @Description 学员查看自己的积分流水（分页）
// @Tags 学员端-积分
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.R "流水"
// @Failure 401 {object} response.R "未认证"
// @Router /points/ledger [get]
func (h *PointsHandler) GetLedger(c *gin.Context) {
	Endpoint[struct{}, service.PointsLedgerResult]{
		Parse: func(c *gin.Context) (*struct{}, error) { return &struct{}{}, nil },
		Invoke: func(ctx context.Context, _ *struct{}) (*service.PointsLedgerResult, error) {
			page := atoiDefault(c.Query("page"), 1)
			pageSize := atoiDefault(c.Query("page_size"), 20)
			return h.svc.GetLedger(middleware.CurrentUserID(c), page, pageSize, "")
		},
	}.Handle(c)
}

// GetTasks 任务列表
// GetTasks 积分任务中心 GET /api/points/tasks
// @Summary 积分任务中心
// @Description 学员查看积分任务列表（todo/claimable/claimed 三态）
// @Tags 学员端-积分
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "任务列表"
// @Failure 401 {object} response.R "未认证"
// @Router /points/tasks [get]
func (h *PointsHandler) GetTasks(c *gin.Context) {
	Endpoint[struct{}, service.PointsTasksResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.PointsTasksResult, error) {
			return h.svc.GetTasks(middleware.CurrentUserID(c))
		},
	}.Handle(c)
}

// Claim 领取
// Claim 领取积分任务 POST /api/points/tasks/:code/claim
// @Summary 领取任务积分
// @Description 学员领取积分任务奖励（防双领 + 幂等）
// @Tags 学员端-积分
// @Produce json
// @Security BearerAuth
// @Param code path string true "任务编码"
// @Success 200 {object} response.R "已领取"
// @Failure 400 {object} response.R "已领取/额度不足"
// @Failure 401 {object} response.R "未认证"
// @Router /points/tasks/{code}/claim [post]
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
				// ADR-0024：哨兵映射状态码（已领取类 400、不存在类 404），文案零漂移
				if errors.Is(err, service.ErrAlreadyClaimed) || errors.Is(err, service.ErrDailyClaimLimit) {
					response.BadRequest(c, err.Error())
					return
				}
				if errors.Is(err, service.ErrTaskNotFound) {
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

// RedeemCourse 兑换课程
// RedeemCourse 兑换课程 POST /api/points/shop/course/:courseId/redeem
// @Summary 兑换课程
// @Description 学员用积分兑换课程（锁 + 已拥有校验 + 幂等）
// @Tags 学员端-积分
// @Produce json
// @Security BearerAuth
// @Param courseId path int true "课程 ID"
// @Success 200 {object} response.R "兑换成功"
// @Failure 400 {object} response.R "余额不足/已拥有"
// @Failure 401 {object} response.R "未认证"
// @Router /points/shop/course/{courseId}/redeem [post]
func (h *PointsHandler) RedeemCourse(c *gin.Context) {
	courseID, err := pathInt(c, "courseId", "课程ID无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	Endpoint[struct{}, service.RedeemResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.RedeemResult, error) {
			return h.svc.RedeemCourse(ctx, middleware.CurrentUserID(c), courseID)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.RedeemResult, err error) {
			if err != nil {
				if errors.Is(err, service.ErrInsufficientPoints) {
					response.BadRequest(c, err.Error())
					return
				}
				if errors.Is(err, service.ErrAlreadyRedeemed) {
					response.BadRequest(c, err.Error())
					return
				}
				if errors.Is(err, service.ErrCourseNotFound) || errors.Is(err, service.ErrCourseNotRedeemable) {
					response.BadRequest(c, err.Error())
					return
				}
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// RedeemShop 兑换商城
// RedeemShop 兑换商城商品 POST /api/points/shop/:sku/redeem
// @Summary 兑换商城商品
// @Description 学员用积分兑换商城商品（锁 + 已拥有校验 + 幂等）
// @Tags 学员端-积分
// @Produce json
// @Security BearerAuth
// @Param sku path string true "商品 SKU"
// @Success 200 {object} response.R "兑换成功"
// @Failure 400 {object} response.R "余额不足/已拥有"
// @Failure 401 {object} response.R "未认证"
// @Router /points/shop/{sku}/redeem [post]
func (h *PointsHandler) RedeemShop(c *gin.Context) {
	sku := c.Param("sku")
	if sku == "" {
		response.BadRequest(c, "sku 不能为空")
		return
	}
	Endpoint[struct{}, service.RedeemResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.RedeemResult, error) {
			return h.svc.RedeemShop(ctx, middleware.CurrentUserID(c), sku)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.RedeemResult, err error) {
			if err != nil {
				if errors.Is(err, service.ErrInsufficientPoints) || errors.Is(err, service.ErrAlreadyRedeemed) {
					response.BadRequest(c, err.Error())
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
