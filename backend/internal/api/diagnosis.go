// Package api 实现 HTTP handlers。
// 本文件：智能维修诊断周边只读代理（品牌/车型联动、故障码查询、手册静态资源），
// 与 chat 同簇（/api/ai-assistant/diagnosis/*），鉴权沿用 OptionalAuth。
package api

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// DiagnosisHandler 诊断周边代理 handler。
// 数据源为外部 RAG 助手（DiagnosisProxyService 直连，绕过 nginx Basic Auth 层）。
type DiagnosisHandler struct {
	proxy *service.DiagnosisProxyService
}

// NewDiagnosisHandler 构造 DiagnosisHandler。
func NewDiagnosisHandler(proxy *service.DiagnosisProxyService) *DiagnosisHandler {
	return &DiagnosisHandler{proxy: proxy}
}

// RegisterDiagnosisRoutes 注册 /api/ai-assistant/diagnosis 路由（均可选认证，与 chat 一致）。
func RegisterDiagnosisRoutes(g *gin.RouterGroup, rd RouterDeps, proxy *service.DiagnosisProxyService) {
	h := NewDiagnosisHandler(proxy)
	group := g.Group("/diagnosis")
	group.Use(middleware.OptionalAuth(rd.Session))
	group.GET("/brands", h.ListBrands)
	group.GET("/models", h.ListModels)
	group.GET("/fault-codes", h.ListFaultCodes)
	group.GET("/manual/*filepath", h.OpenManual)
}

// ListBrands GET /diagnosis/brands 品牌列表（全量，无级联依赖）。
func (h *DiagnosisHandler) ListBrands(c *gin.Context) {
	brands, err := h.proxy.ListBrands(c.Request.Context())
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, brands)
}

// ListModels GET /diagnosis/models?brand= 某品牌车型列表；brand 缺省为全部（助手返回全量车型）。
func (h *DiagnosisHandler) ListModels(c *gin.Context) {
	models, err := h.proxy.ListModels(c.Request.Context(), c.Query("brand"))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, models)
}

// ListFaultCodes GET /diagnosis/fault-codes?brand&keyword&page&page_size 故障码分页查询。
func (h *DiagnosisHandler) ListFaultCodes(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	resp, err := h.proxy.ListFaultCodes(c.Request.Context(), c.Query("brand"), c.Query("keyword"), page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// OpenManual GET /diagnosis/manual/* 手册静态资源流式代理（溯源图片 <<IMAGE:...>> 渲染面）。
// 子路径白名单（防 SSRF）在 proxy 层；可选认证可达，缓存 1 天（手册文件不可变）。
func (h *DiagnosisHandler) OpenManual(c *gin.Context) {
	subpath := c.Param("filepath")
	body, contentType, err := h.proxy.OpenManual(c.Request.Context(), subpath)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	defer body.Close()
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, body)
}
