// Package api 实现 HTTP handlers。
// 本文件：全局搜索（ADR-0018）—— GET /api/search?keyword=&type=&page=&page_size=（公开）。
package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
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
	rg.GET("/search", middleware.CredentialScoped(rd.CredentialScope), h.Search)
}

// Search 全局搜索
// @Summary 全局搜索
// @Description 公开访问，keyword 模糊匹配 course/question/content/topic；type 缺省返回各分区聚合（courses/questions/contents/topics），
// @Description 指定 type 时返回该类型的分页结果 —— 同一端点两种响应形状（swag 无联合类型表达力，data 取聚合形状；
// @Description 分页形状 service.SearchPageDTO 同域生成，前端以联合类型消费）。
// @Tags 学员端-搜索
// @Accept json
// @Produce json
// @Param keyword query string true "关键词"
// @Param type query string false "类型 course|question|content|topic"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R{data=service.SearchPageDTO} "指定 type：分页结果"
// @Success 200 {object} response.R{data=service.SearchAllDTO} "type 缺省：各分区聚合（swag 同名状态码取最后一条，聚合形状为准）"
// @Failure 400 {object} response.R "参数错误"
// @Router /search [get]
func (h *SearchHandler) Search(c *gin.Context) {
	credID := middleware.CredentialIDPtr(c)
	resp, err := h.svc.Search(c.Query("keyword"), c.Query("type"),
		atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("page_size"), 20), credID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, resp)
}
