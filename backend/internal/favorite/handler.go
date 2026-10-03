// Package favorite 通用收藏域（ADR-0018）：/api/favorites 多态收藏（course/chapter/question/featured/topic）。
// 本包是 internal/<域> 形态的样板之一（ADR-0070）：handler.go 是 HTTP 出口（JWT + hrwai_user + 证件域），
// go 是域实现，nonnil_outlets_test.go 承载声明式证据表。
// 装配点：internal/api/routes_registry.go 调 favorite.RegisterRoutes(api, rd.Session, rd.CredentialScope, deps.FavoriteSvc)。
package favorite

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// handler 通用收藏 handler。
type handler struct {
	svc *Service
}

// newHandler 创建通用收藏 handler。
func newHandler(svc *Service) *handler {
	return &handler{svc: svc}
}

// RegisterRoutes 注册 /api/favorites 蓝图（JWT + hrwai_user + 证件域）。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, credRes middleware.CredentialResolver, svc *Service) {
	h := newHandler(svc)

	g := rg.Group("/favorites", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapFavoriteManage), middleware.CredentialScoped(credRes))

	// GET /api/favorites?target_type=&page=&page_size= 我的收藏列表（快照回填）
	g.GET("", h.List)
	// POST /api/favorites {target_type, target_id} 收藏（幂等）
	g.POST("", h.Add)
	// DELETE /api/favorites/:id 取消收藏（仅本人）
	g.DELETE("/:id", h.Remove)
	// GET /api/favorites/check?target_type=&target_id= 是否已收藏
	g.GET("/check", h.Check)
}

// List 我的收藏
// @Summary 我的收藏列表
// @Description 分页查询收藏，快照回填；支持按 target_type 过滤
// @Tags 学员端-收藏
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param target_type query string false "目标类型 course/chapter/question/featured/topic"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R{data=FavoritePageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /favorites [get]
func (h *handler) List(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	credID := middleware.CredentialIDPtr(c)
	resp, err := h.svc.List(userID, c.Query("target_type"),
		httpx.QueryIntDefault(c, "page", 1), httpx.QueryIntDefault(c, "page_size", 20), credID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// Add 收藏
// @Summary 收藏
// @Description 幂等收藏（user+type+id 唯一）
// @Tags 学员端-收藏
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "目标" example({"target_type":"course","target_id":1})
// @Success 201 {object} response.R{data=FavoriteDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 500 {object} response.R "服务端内部错误（含可见性/存在性查询读不动；不外发驱动原文）"
// @Router /favorites [post]
func (h *handler) Add(c *gin.Context) {
	var body struct {
		TargetType string `json:"target_type"`
		TargetID   int    `json:"target_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.TargetID <= 0 {
		response.BadRequest(c, "请求参数错误")
		return
	}
	resp, err := h.svc.Add(middleware.CurrentUserID(c), body.TargetType, body.TargetID, questionbank.StudentQuestionScope(c))
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.Created(c, "收藏成功", resp)
}

// ErrStatus 收藏域的写面错误表（复用 Endpoint 缝那张表，不重写第二份扫表算法）。
//
// fallback 500 的理由同批⑥：`validateFavoriteTarget` 五条支从前各自把查询错误丢在 Count 上，
// 「问不出能不能收藏」对外与「不能收藏」同一形状。默认面收窄到 500 的前提是那几条业务事实
// 各有名字——否则它们会跟故障一起被推上去。
// ErrFavTargetIDInvalid 不在本表：Add 的 handler 在进 service 之前就把 target_id <= 0 挡成
// 「请求参数错误」，那条哨兵只有 Check 走得到——而 Check 是另一张面。本批第一版把它登记进来过，
// 反向那半条锁（fact_face_producible_contract_test.go）当场判它「登记了却打不出」。
var ErrStatus = &httpx.ErrStatusTable{
	Entries: []httpx.ErrStatusEntry{
		{Sentinel: ErrFavTargetCourseRejected, Status: http.StatusBadRequest},
		{Sentinel: ErrFavTargetChapterRejected, Status: http.StatusBadRequest},
		{Sentinel: ErrFavTargetQuestionRejected, Status: http.StatusBadRequest},
		{Sentinel: ErrFavTargetFeaturedRejected, Status: http.StatusBadRequest},
		{Sentinel: ErrFavTargetTopicNotFound, Status: http.StatusBadRequest},
		{Sentinel: ErrFavTargetTypeUnsupported, Status: http.StatusBadRequest},
	},
	Fallback: http.StatusInternalServerError,
}

// Remove 取消收藏
// @Summary 取消收藏
// @Description 仅本人可取消
// @Tags 学员端-收藏
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "收藏ID"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /favorites/{id} [delete]
func (h *handler) Remove(c *gin.Context) {
	id, err := httpx.PathInt64(c, "id", "收藏 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.Remove(middleware.CurrentUserID(c), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "已取消收藏", nil)
}

// Check 是否已收藏
// @Summary 是否已收藏
// @Description 查询单目标是否已收藏
// @Tags 学员端-收藏
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param target_type query string true "目标类型"
// @Param target_id query int true "目标ID"
// @Success 200 {object} response.R{data=FavoriteCheckDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /favorites/check [get]
func (h *handler) Check(c *gin.Context) {
	var query struct {
		TargetType string `form:"target_type"`
		TargetID   int    `form:"target_id"`
	}
	if err := c.ShouldBindQuery(&query); err != nil || query.TargetID <= 0 {
		response.BadRequest(c, "请求参数错误")
		return
	}
	resp, err := h.svc.Check(middleware.CurrentUserID(c), query.TargetType, query.TargetID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, resp)
}
