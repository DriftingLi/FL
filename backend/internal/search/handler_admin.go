// 本文件：搜索域的运营面 HTTP 出口（ADR-0070）—— GET /api/admin/search-facts/zero-results。
// 本域两条蓝图分居两文件，故注册函数带 Admin 后缀（同包不能有两个 RegisterRoutes）。
// 装配点：internal/api/routes_registry.go 调 search.RegisterAdminRoutes(api, rd.Session, deps.SearchSvc)。
package search

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// RegisterAdminRoutes 注册检索事实的运营面（管理员）。
// 事实本身是匿名的（无 user / 证件 / 设备列），但「谁可以看零结果词」仍是管理面能力。
func RegisterAdminRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service) {
	h := newHandler(svc)
	g := rg.Group("/admin", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapAdminAccess))
	g.GET("/search-facts/zero-results", h.ZeroResults)
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
// @Success 200 {object} response.R{data=[]ZeroResultKeywordDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/search-facts/zero-results [get]
func (h *handler) ZeroResults(c *gin.Context) {
	rows, err := h.svc.ZeroResultKeywords(httpx.QueryIntDefault(c, "days", 30), httpx.QueryIntDefault(c, "limit", 50))
	if err != nil {
		response.ServerErrorCause(c, "", err)
		return
	}
	response.Success(c, rows)
}
