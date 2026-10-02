// Package search 全局搜索域（ADR-0018 引入 / ADR-0049 定口径）：/api/search 关键词检索。
// 本域两条蓝图分居两文件（ADR-0070）：handler.go 是公开出口（无 JWT，只挂证件域），
// handler_admin.go 是运营面出口（零结果词，JWT + CapAdminAccess），service.go / partitions.go 是域实现。
// 装配点：internal/api/routes_registry.go 调 search.RegisterRoutes(api, rd.CredentialScope, deps.SearchSvc)
// 与 search.RegisterAdminRoutes(api, rd.Session, deps.SearchSvc)。
package search

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// handler 全局搜索 handler（公开面）。
type handler struct {
	svc *Service
}

// newHandler 创建全局搜索 handler。
func newHandler(svc *Service) *handler {
	return &handler{svc: svc}
}

// RegisterRoutes 注册 /api/search 公开蓝图（无 JWT，只挂证件域）。
func RegisterRoutes(rg *gin.RouterGroup, credRes middleware.CredentialResolver, svc *Service) {
	h := newHandler(svc)
	rg.GET("/search", middleware.CredentialScoped(credRes), h.Search)
}

// Search 全局搜索
// @Summary 全局搜索
// @Description 公开访问，keyword 模糊匹配 course/chapter/question/content/topic（LIKE 元字符按字面处理）；type 缺省返回各分区聚合（courses/chapters/questions/contents/topics），
// @Description 指定 type 时返回该类型的分页结果 —— 同一端点两种响应形状（swag 无联合类型表达力，data 取聚合形状；
// @Description 分页形状 SearchPageDTO 同域生成，前端以联合类型消费）。
// @Description 每条结果带命中位置 hit_field（title|body|reply）与命中片段 snippet（源串窗口，投影与高亮由各端自行处理，ADR-0049 决策 6）。
// @Tags 学员端-搜索
// @Accept json
// @Produce json
// @Param keyword query string true "关键词"
// @Param type query string false "类型 course|chapter|question|content|topic"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R{data=SearchPageDTO} "指定 type：分页结果"
// @Success 200 {object} response.R{data=SearchAllDTO} "type 缺省：各分区聚合（swag 同名状态码取最后一条，聚合形状为准）"
// @Failure 400 {object} response.R "参数错误"
// @Router /search [get]
func (h *handler) Search(c *gin.Context) {
	credID := middleware.CredentialIDPtr(c)
	resp, err := h.svc.Search(c.Query("keyword"), c.Query("type"),
		httpx.QueryIntDefault(c, "page", 1), httpx.QueryIntDefault(c, "page_size", 20), credID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, resp)
}
