// 本文件：积分域的管理端 HTTP 出口（ADR-0070）——/api/admin/points 蓝图
// （POST /admin/points/penalty、GET /admin/points/shop/items、PUT /admin/points/shop/items/{sku}）。
// 本域两条蓝图分居两文件，故 handler 类型与注册函数都带 Admin 后缀（同包不能有两个 RegisterRoutes）。
// 装配点：internal/api/routes_registry.go 调 points.RegisterAdminRoutes(api, rd.Session, deps.PointsSvc)。
package points

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
)

// adminHandler 管理员积分扣罚与商品目录维护
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
	g.GET("/shop/items", h.ListShopItems)
	g.PUT("/shop/items/:sku", h.UpsertShopItem)
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
// invoke 适配器不带 ctx，服务侧用请求作用域 ctx（罚分锁为瞬态护栏，存储不可用不阻断主流程）。
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

// ListShopItems 管理端商品目录（#2548）：含下架行，Redeemable 暴露「有行无读者」的漏登记。
// @Summary 商品目录（管理端）
// @Description 管理员查看全部积分商品（含下架行）与可兑状态；改价/上下架/换图/改文案走本组端点，不靠发版
// @Tags 管理端-积分
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=points.AdminShopItemsResult} "商品目录"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无积分管理能力"
// @Router /admin/points/shop/items [get]
func (h *adminHandler) ListShopItems(c *gin.Context) {
	httpx.Endpoint[struct{}, AdminShopItemsResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*AdminShopItemsResult, error) {
			return h.pointsSvc.AdminListShopItems(ctx)
		},
		ErrStatus: ErrStatus,
	}.Handle(c)
}

// adminShopUpsertReq 建/改商品行的请求体（PUT 全量替换；sku 走路径，不在体内重复）。
type adminShopUpsertReq struct {
	Title       string  `json:"title" binding:"required"`
	Price       int     `json:"price" binding:"required,gt=0"`
	Stock       *int    `json:"stock"`
	Enabled     *bool   `json:"enabled" binding:"required"`
	ImageURL    *string `json:"image_url"`
	Description *string `json:"description"`
	Category    string  `json:"category"`
	SortWeight  int     `json:"sort_weight"`
}

// UpsertShopItem 建/改一行商品（sku 是主键，PUT 幂等）。
// 「新增可兑 SKU」的登记路径见 shop_admin.go 头注：先登记 shopSKUDecls，再经本端点建行；
// 未登记就上架会被 ErrShopSKUUnregistered 拒掉，不会建出永远不出现在在售列表的行。
// @Summary 建/改商品（管理端）
// @Description 管理员建/改商品行：改价、上下架、换图、改文案、调分类与排序权重；sku 是主键，重复 PUT 幂等
// @Tags 管理端-积分
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param sku path string true "商品 SKU"
// @Param body body object false "商品输入 {title,price,stock,enabled,image_url,description,category,sort_weight}"
// @Success 200 {object} response.R{data=points.AdminShopItemDTO} "已保存"
// @Failure 400 {object} response.R "参数非法/分类非法/SKU 未登记/价格事实源不可下架"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无积分管理能力"
// @Router /admin/points/shop/items/{sku} [put]
func (h *adminHandler) UpsertShopItem(c *gin.Context) {
	httpx.Endpoint[adminShopUpsertReq, AdminShopItemDTO]{
		Parse: httpx.BindJSONMsgFunc[adminShopUpsertReq]("请求参数错误：title/price/enabled 必填，price>0"),
		Invoke: httpx.Invoke(func(req adminShopUpsertReq) (AdminShopItemDTO, error) {
			enabled := req.Enabled != nil && *req.Enabled
			dto, err := h.pointsSvc.AdminUpsertShopItem(c.Request.Context(), c.Param("sku"), AdminShopItemInput{
				Title:       req.Title,
				Price:       req.Price,
				Stock:       req.Stock,
				Enabled:     enabled,
				ImageURL:    req.ImageURL,
				Description: req.Description,
				Category:    req.Category,
				SortWeight:  req.SortWeight,
			})
			if err != nil {
				return AdminShopItemDTO{}, err
			}
			return *dto, nil
		}),
		ErrStatus: ErrStatus,
	}.Handle(c)
}
