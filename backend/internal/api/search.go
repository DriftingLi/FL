// Package api 实现 HTTP handlers。
// 本文件：全局搜索（ADR-0018 引入 / ADR-0049 定口径）—— GET /api/search?keyword=&type=&page=&page_size=（公开）。
// 另有零结果词运营面 GET /api/admin/search-facts/zero-results（管理员，读匿名检索事实）。
package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
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

// RegisterSearchAdminRoutes 注册检索事实的运营面（管理员）。
// 事实本身是匿名的（无 user / 证件 / 设备列），但「谁可以看零结果词」仍是管理面能力。
func RegisterSearchAdminRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.SearchService) {
	h := NewSearchHandler(svc)
	g := rg.Group("/admin", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapAdminAccess))
	g.GET("/search-facts/zero-results", h.ZeroResults)
}

// Search 全局搜索
// @Summary 全局搜索
// @Description 公开访问，keyword 模糊匹配 course/chapter/question/content/topic（LIKE 元字符按字面处理）；type 缺省返回各分区聚合（courses/chapters/questions/contents/topics），
// @Description 指定 type 时返回该类型的分页结果 —— 同一端点两种响应形状（swag 无联合类型表达力，data 取聚合形状；
// @Description 分页形状 service.SearchPageDTO 同域生成，前端以联合类型消费）。
// @Description 每条结果带命中位置 hit_field（title|body|reply）与命中片段 snippet（源串窗口，投影与高亮由各端自行处理，ADR-0049 决策 6）。
// @Tags 学员端-搜索
// @Accept json
// @Produce json
// @Param keyword query string true "关键词"
// @Param type query string false "类型 course|chapter|question|content|topic"
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

// ZeroResults 零结果词（运营面）
// @Summary 零结果词列表
// @Description 按关键词聚合「总命中数为 0」的检索事实（ADR-0049 决策 7）。检索事实匿名：不含 user / 证件 / 设备，也不用于个性化。
// @Tags 学员端-搜索
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param days query int false "统计窗口（天）" default(30)
// @Param limit query int false "返回条数上限" default(50)
// @Success 200 {object} response.R{data=[]service.ZeroResultKeywordDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/search-facts/zero-results [get]
func (h *SearchHandler) ZeroResults(c *gin.Context) {
	rows, err := h.svc.ZeroResultKeywords(atoiDefault(c.Query("days"), 30), atoiDefault(c.Query("limit"), 50))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, rows)
}
