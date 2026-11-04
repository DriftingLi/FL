// 本文件：智能维修诊断的两类周边面 —— 回答配图的回源代理（/ossimg/*）与 v2 引导只读面
// （intents / systems / faults / code-hint / health）。与 chat 同簇、鉴权沿用 OptionalAuth。
//
// ADR-0063 时期的品牌/车型/故障码/手册四条只读代理随 v1 通道退役删除（票 2237 C 节）；
// 票 2242 按 v2 契约重新落了五个端点，**形状与语义都换了一套**：v1 是「关键词+分页查码表」，
// v2 是「品牌→系统/型号→故障现象」三级点选 + 单码体检 + 上游健康检查。
package aiassistant

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/response"
)

// diagnosisCatalogCacheControl 引导面三个字典端点的缓存头（票 2242 决策）。
//
// 品牌、系统、故障现象入口都是**慢变**数据（几个月才动一次），而进页会连打三次上游，
// 对方默认限流只有 60 次/分钟。5 分钟是「够挡住反复进页」与「改了东西不用等一天」的折中；
// 比 ossimg 的 86400 短得多，因为那边是 OSS 上的规范名文件，这边是可能新增条目的字典。
//
// health 与 code-hint 不加缓存：前者必须实时（它就是用来回答「现在通不通」的），
// 后者跟着输入框逐字符变。
const diagnosisCatalogCacheControl = "public, max-age=300"

// diagnosisHandler 诊断周边面 handler（配图代理 + 引导只读面）。
type diagnosisHandler struct {
	proxy   *DiagnosisProxyService
	catalog *DiagnosisCatalogService
}

// newDiagnosisHandler 构造 diagnosisHandler。
func newDiagnosisHandler(proxy *DiagnosisProxyService, catalog *DiagnosisCatalogService) *diagnosisHandler {
	return &diagnosisHandler{proxy: proxy, catalog: catalog}
}

// registerDiagnosisRoutes 注册 /api/ai-assistant/diagnosis 路由（可选认证，与 chat 一致）。
func registerDiagnosisRoutes(g *gin.RouterGroup, session *security.Session, proxy *DiagnosisProxyService, catalog *DiagnosisCatalogService) {
	h := newDiagnosisHandler(proxy, catalog)
	group := g.Group("/diagnosis")
	group.Use(middleware.OptionalAuth(session))
	group.GET("/ossimg/*filepath", h.OpenOSSImage)
	group.GET("/intents", h.Intents)
	group.GET("/systems", h.Systems)
	group.GET("/faults", h.Faults)
	group.GET("/code-hint", h.CodeHint)
	group.GET("/health", h.Health)
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

// Intents 诊断引导第一层
// @Summary 诊断引导：品牌卡与高频问题
// @Description 可选认证；代理 v2 `GET /api/v2/intents`。品牌卡带 picker（system=第二层选系统 / model=选型号），高频问题点一下即作为 content 发问。集合恒非 null（空集为 []）
// @Tags 学员端-AI助手
// @Accept json
// @Produce json
// @Success 200 {object} response.R{data=aiassistant.DiagnosisIntents} "success"
// @Failure 502 {object} response.R "诊断服务不可用"
// @Failure 429 {object} response.R "上游限流"
// @Router /ai-assistant/diagnosis/intents [get]
func (h *diagnosisHandler) Intents(c *gin.Context) {
	out, err := h.catalog.Intents(c.Request.Context())
	if err != nil {
		diagnosisCatalogError(c, err)
		return
	}
	c.Header("Cache-Control", diagnosisCatalogCacheControl)
	response.Success(c, out)
}

// Systems 诊断引导第二层
// @Summary 诊断引导：系统或型号列表
// @Description 可选认证；代理 v2 `GET /api/v2/systems?brand=`。中力的 brand 返回系统（带 icon），林德的返回型号（无 icon）。集合恒非 null
// @Tags 学员端-AI助手
// @Accept json
// @Produce json
// @Param brand query string true "品牌标识（zhongli / linde，取自 /diagnosis/intents 的 brands[].id）"
// @Success 200 {object} response.R{data=aiassistant.DiagnosisCatalogItems} "success"
// @Failure 400 {object} response.R "缺少 brand"
// @Failure 502 {object} response.R "诊断服务不可用"
// @Failure 429 {object} response.R "上游限流"
// @Router /ai-assistant/diagnosis/systems [get]
func (h *diagnosisHandler) Systems(c *gin.Context) {
	brand := c.Query("brand")
	if brand == "" {
		response.BadRequest(c, "缺少品牌参数")
		return
	}
	out, err := h.catalog.Systems(c.Request.Context(), brand)
	if err != nil {
		diagnosisCatalogError(c, err)
		return
	}
	c.Header("Cache-Control", diagnosisCatalogCacheControl)
	response.Success(c, out)
}

// Faults 诊断引导第三层
// @Summary 诊断引导：故障现象列表
// @Description 可选认证；代理 v2 `GET /api/v2/faults?brand=&second=`。second 为空时返回该品牌的根列表；条目是**现象入口**（如「仪表报故障码」），选中后把 name 拼进问题。集合恒非 null
// @Tags 学员端-AI助手
// @Accept json
// @Produce json
// @Param brand query string true "品牌标识"
// @Param second query string false "第二层选中的 id（系统/型号）；空 = 品牌根列表"
// @Success 200 {object} response.R{data=aiassistant.DiagnosisCatalogItems} "success"
// @Failure 400 {object} response.R "缺少 brand"
// @Failure 502 {object} response.R "诊断服务不可用"
// @Failure 429 {object} response.R "上游限流"
// @Router /ai-assistant/diagnosis/faults [get]
func (h *diagnosisHandler) Faults(c *gin.Context) {
	brand := c.Query("brand")
	if brand == "" {
		response.BadRequest(c, "缺少品牌参数")
		return
	}
	out, err := h.catalog.Faults(c.Request.Context(), brand, c.Query("second"))
	if err != nil {
		diagnosisCatalogError(c, err)
		return
	}
	c.Header("Cache-Control", diagnosisCatalogCacheControl)
	response.Success(c, out)
}

// CodeHint 故障码实时体检
// @Summary 诊断引导：故障码实时体检
// @Description 可选认证；代理 v2 `GET /api/v2/code_hint?q=`。status 五态：known（码表命中）/ unknown（格式对但未收录）/ format（格式不合法，samples 给出填法）/ none（没输入或太短）/ typo（形似字符笔误，fix 给出应改成什么）。不加缓存：跟着输入框逐字符变
// @Tags 学员端-AI助手
// @Accept json
// @Produce json
// @Param q query string false "用户当前输入内容（空 = 尚未输入，上游回 status=none）"
// @Success 200 {object} response.R{data=aiassistant.DiagnosisCodeHint} "success"
// @Failure 502 {object} response.R "诊断服务不可用"
// @Failure 429 {object} response.R "上游限流"
// @Router /ai-assistant/diagnosis/code-hint [get]
func (h *diagnosisHandler) CodeHint(c *gin.Context) {
	out, err := h.catalog.CodeHint(c.Request.Context(), c.Query("q"))
	if err != nil {
		diagnosisCatalogError(c, err)
		return
	}
	response.Success(c, out)
}

// Health 诊断服务健康检查
// @Summary 诊断服务健康检查
// @Description 可选认证；代理 v2 `GET /api/health`，**只投影两个字段**（ok / app_configured）——上游其余 18 个字段含 Key 指纹、数据目录与工作流 ID，属对方内部拓扑，不得出现在未登录可读的端点里。**永远 200**：连不上就是 ok=false，让前端拿到结构化结果而不是一句 errmsg
// @Tags 学员端-AI助手
// @Accept json
// @Produce json
// @Success 200 {object} response.R{data=aiassistant.DiagnosisHealth} "success（上游不可达时 ok/app_configured 均为 false）"
// @Router /ai-assistant/diagnosis/health [get]
func (h *diagnosisHandler) Health(c *gin.Context) {
	response.Success(c, h.catalog.Health(c.Request.Context()))
}

// diagnosisCatalogError 把引导面的错误映射成 HTTP 状态码（票 2242 决策）：
//   - 上游限流 → 429（前端能区分「等会儿再试」与「服务坏了」）；
//   - 未配置、鉴权失败、上游 5xx、超时、解析失败 → 502（我方是网关，锅在上游）。
//
// 502 用固定文案 + c.Error 记账：错误原文由 response.ClientErrorText 决定不外发 5xx 尾巴
// （ADR-0064 决策 9），真实原因留在日志面。
func diagnosisCatalogError(c *gin.Context, err error) {
	if errors.Is(err, ErrDiagnosisRateLimited) {
		response.TooManyRequests(c, err.Error())
		return
	}
	if errors.Is(err, ErrDiagnosisNotConfigured) {
		response.BadGateway(c, err.Error())
		return
	}
	response.BadGatewayWithCause(c, "诊断服务暂时不可用", err)
}
