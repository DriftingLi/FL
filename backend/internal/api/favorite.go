// Package api 实现 HTTP handlers。
// 本文件：通用收藏（ADR-0018）—— /api/favorites 多态收藏（course/chapter/question/featured/topic）。
package api

import (
	"errors"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// FavoriteHandler 通用收藏 handler。
type FavoriteHandler struct {
	svc *service.FavoriteService
}

// NewFavoriteHandler 创建通用收藏 handler。
func NewFavoriteHandler(svc *service.FavoriteService) *FavoriteHandler {
	return &FavoriteHandler{svc: svc}
}

// RegisterFavoriteRoutes 注册 /api/favorites 蓝图（JWT + hrwai_user）。
func RegisterFavoriteRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.FavoriteService) {
	h := NewFavoriteHandler(svc)

	g := rg.Group("/favorites", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapFavoriteManage), middleware.CredentialScoped(rd.CredentialScope))

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
// @Success 200 {object} response.R{data=service.FavoritePageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /favorites [get]
func (h *FavoriteHandler) List(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	credID := middleware.CredentialIDPtr(c)
	resp, err := h.svc.List(userID, c.Query("target_type"),
		atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("page_size"), 20), credID)
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
// @Success 201 {object} response.R{data=service.FavoriteDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 500 {object} response.R "服务端内部错误（含可见性/存在性查询读不动；不外发驱动原文）"
// @Router /favorites [post]
func (h *FavoriteHandler) Add(c *gin.Context) {
	var body struct {
		TargetType string `json:"target_type"`
		TargetID   int    `json:"target_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.TargetID <= 0 {
		response.BadRequest(c, "请求参数错误")
		return
	}
	resp, err := h.svc.Add(middleware.CurrentUserID(c), body.TargetType, body.TargetID, studentQuestionScope(c))
	if err != nil {
		renderFavoriteError(c, err)
		return
	}
	response.Created(c, "收藏成功", resp)
}

// favoriteFacts400 收藏域的业务事实全集。表里没有的一律按未具名库故障渲染 500
// （ADR-0065 决策 7：`validateFavoriteTarget` 五条支从前都丢查询错误，「问不出能不能收藏」与
// 「不能收藏」同一形状；故障要落 500 的前提是这几条各有名字，否则它们会跟故障一起被推上去）。
var favoriteFacts400 = []error{
	service.ErrFavTargetCourseUnreadable,
	service.ErrFavTargetChapterUnreadable,
	service.ErrFavTargetQuestionUnreadable,
	service.ErrFavTargetFeaturedUnreadable,
	service.ErrFavTargetTopicNotFound,
	service.ErrFavTargetTypeUnsupported,
	service.ErrFavTargetIDInvalid,
	service.ErrFavoriteNotFound,
}

// renderFavoriteError 裸 handler 一侧的档位分流：业务事实 400 各说自己的句子，其余 500 且不带
// 驱动原文。形状与 question_interaction.go 的 renderInteractionError 相同，两张表各归各域——
// 事实集不同，合成一张会让「这道题不在池内」与「这条收藏不存在」共用一次遍历。
func renderFavoriteError(c *gin.Context, err error) {
	for _, f := range favoriteFacts400 {
		if errors.Is(err, f) {
			response.BadRequest(c, err.Error())
			return
		}
	}
	c.Error(err) //nolint:errcheck // gin 的 Error 只记账，返回值是链式用的
	response.ServerError(c, "服务器内部错误")
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
func (h *FavoriteHandler) Remove(c *gin.Context) {
	id, err := pathInt64(c, "id", "收藏 ID 无效")
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
// @Success 200 {object} response.R{data=service.FavoriteCheckDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /favorites/check [get]
func (h *FavoriteHandler) Check(c *gin.Context) {
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
