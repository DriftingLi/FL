// HTTP 出口（原 internal/valuation/handler 子包，#1514 波 9 并回域包）：实现 HTTP 处理器
package valuation

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthHandler 健康检查处理器
// 用于部署健康探测、负载均衡探活、前后端联调验证
type HealthHandler struct{}

// NewHealthHandler 构造健康检查处理器
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Response 健康检查统一响应
type HealthResponse struct {
	Status    string `json:"status"`    // 服务状态：ok
	Service   string `json:"service"`   // 服务名称
	Timestamp string `json:"timestamp"` // 响应时间戳
}

// Check 处理 GET /api/valuation/health 请求
// @Summary 估值子模块健康检查
// @Description 返回子模块状态/服务名/时间戳，不连数据库也不连 Redis，仅证明该蓝图可响应。非统一信封（裸 HealthResponse，字段全为字符串）。公开端点：无需登录
// @Tags 估值-系统
// @Produce json
// @Success 200 {object} map[string]any "{status:ok,service:forklift-valuation-backend,timestamp:RFC3339}"
// @Router /valuation/health [get]
func (h *HealthHandler) Check(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:    "ok",
		Service:   "forklift-valuation-backend",
		Timestamp: time.Now().Format(time.RFC3339),
	})
}
