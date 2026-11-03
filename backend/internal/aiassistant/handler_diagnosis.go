// 本文件：智能维修诊断回答配图的回源代理（/api/ai-assistant/diagnosis/ossimg/*）。
// 与 chat 同簇、鉴权沿用 OptionalAuth。ADR-0063 时期这里还有品牌/车型/故障码/手册四条
// 只读代理，随 v1 通道退役一并删除（票 2237 C 节）：v2 不再提供这些周边端点，
// 需要的能力由票 2242 按 v2 契约重新落地。
package aiassistant

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/response"
)

// diagnosisHandler 回答配图代理 handler。
type diagnosisHandler struct {
	proxy *DiagnosisProxyService
}

// newDiagnosisHandler 构造 diagnosisHandler。
func newDiagnosisHandler(proxy *DiagnosisProxyService) *diagnosisHandler {
	return &diagnosisHandler{proxy: proxy}
}

// registerDiagnosisRoutes 注册 /api/ai-assistant/diagnosis 路由（可选认证，与 chat 一致）。
func registerDiagnosisRoutes(g *gin.RouterGroup, session *security.Session, proxy *DiagnosisProxyService) {
	h := newDiagnosisHandler(proxy)
	group := g.Group("/diagnosis")
	group.Use(middleware.OptionalAuth(session))
	group.GET("/ossimg/*filepath", h.OpenOSSImage)
}

// OpenOSSImage 诊断回答配图
// @Summary 诊断回答配图（白名单代理）
// @Description 可选认证；AI 生成正文里白名单内图片的回源代理（**非统一信封**：原样字节流 + Content-Type）。上游仅允许配置的 OSS 基准 host + /forklift/repair_img 前缀 + 图片扩展名，子路径按段白名单防 SSRF
// @Tags 学员端-AI助手
// @Produce image/png
// @Param filepath path string true "相对子路径（带图片扩展名；如 images_all/xxx.png、linde/型号__原名.jpg）"
// @Success 200 {string} binary "图片字节流（非统一信封）"
// @Failure 404 {object} response.R "图片不存在"
// @Router /ai-assistant/diagnosis/ossimg/{filepath} [get]
// OpenOSSImage GET /diagnosis/ossimg/* 回答配图流式代理（ADR-0091）。
// 子路径白名单（五道闸）在 proxy 层；缓存 1 天、不用 immutable —— OSS 上是规范名而非
// 内容哈希，同名文件可能换内容（ADR-0091 决策 6）。
func (h *diagnosisHandler) OpenOSSImage(c *gin.Context) {
	subpath := c.Param("filepath")
	body, contentType, err := h.proxy.OpenOSSImage(c.Request.Context(), subpath)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	defer func() { _ = body.Close() }()
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, body)
}
