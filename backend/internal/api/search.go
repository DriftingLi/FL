// Package api 实现 HTTP handlers。
// 本文件：全局搜索（ADR-0018）—— GET /api/search?keyword=&type=&page=&page_size=（公开）。
package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// SearchHandler 全局搜索 handler。
type SearchHandler struct {
	svc *service.SearchService
}

// NewSearchHandler 创建全局搜索 handler。
func NewSearchHandler(svc *service.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

// RegisterSearchRoutes 注册 /api/search 蓝图（公开访问）。
func RegisterSearchRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.SearchService) {
	h := NewSearchHandler(svc)
	rg.GET("/search", h.Search)
}

// Search 全局搜索
// @Summary 全局搜索
// @Description 公开访问，keyword 模糊匹配 course/question/content/topic，type 可选过滤
// @Tags 学员端-搜索
// @Accept json
// @Produce json
// @Param keyword query string true "关键词"
// @Param type query string false "类型 course|question|content|topic"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Router /search [get]
func (h *SearchHandler) Search(c *gin.Context) {
	credID := queryIDPtr(c, "credential_id")
	resp, err := h.svc.Search(c.Query("keyword"), c.Query("type"),
		atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("page_size"), 20), credID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, resp)
}
