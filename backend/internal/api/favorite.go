// Package api 实现 HTTP handlers。
// 本文件：通用收藏（ADR-0018）—— /api/favorites 多态收藏（course/chapter/question/featured/topic）。
package api

import (
	"github.com/gin-gonic/gin"

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

	g := rg.Group("/favorites", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))

	// GET /api/favorites?target_type=&page=&page_size= 我的收藏列表（快照回填）
	g.GET("", h.List)
	// POST /api/favorites {target_type, target_id} 收藏（幂等）
	g.POST("", h.Add)
	// DELETE /api/favorites/:id 取消收藏（仅本人）
	g.DELETE("/:id", h.Remove)
	// GET /api/favorites/check?target_type=&target_id= 是否已收藏
	g.GET("/check", h.Check)
}

// List 我的收藏 GET /api/favorites
func (h *FavoriteHandler) List(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	resp, err := h.svc.List(userID, c.Query("target_type"),
		atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("page_size"), 20))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// Add 收藏 POST /api/favorites
func (h *FavoriteHandler) Add(c *gin.Context) {
	var body struct {
		TargetType string `json:"target_type"`
		TargetID   int    `json:"target_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.TargetID <= 0 {
		response.BadRequest(c, "请求参数错误")
		return
	}
	resp, err := h.svc.Add(middleware.CurrentUserID(c), body.TargetType, body.TargetID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "收藏成功", resp)
}

// Remove 取消收藏 DELETE /api/favorites/:id
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

// Check 收藏状态 GET /api/favorites/check
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
