// Package handler 实现 HTTP 处理器
package handler

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
	Status    string `json:"status"`     // 服务状态：ok
	Service   string `json:"service"`    // 服务名称
	Timestamp string `json:"timestamp"`  // 响应时间戳
}

// Check 处理 GET /api/v1/health 请求
func (h *HealthHandler) Check(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:    "ok",
		Service:   "forklift-valuation-backend",
		Timestamp: time.Now().Format(time.RFC3339),
	})
}
