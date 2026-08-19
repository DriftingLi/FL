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

// Search 全局搜索 GET /api/search
func (h *SearchHandler) Search(c *gin.Context) {
	resp, err := h.svc.Search(c.Query("keyword"), c.Query("type"),
		atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("page_size"), 20))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, resp)
}
