// Package api 实现 HTTP handlers。
// 本文件：系统与健康检查端点（API.md §1）——根路由服务信息 + 两条探活。
//
// 为什么是包级具名函数而不是 NewRouter 里的内联闭包（issue #1417）：
// swag 只解析**具名函数上方的 @ 注释块**，贴在闭包上的注释不会被抓取，于是这三条真实注册在路由上的
// 端点对「注解 → swagger → 前端生成物」整条管道隐形。抽函数只改承载形式，响应体逐字不变。
//
// 探活路径的限流豁免与访问日志豁免由 middleware.HealthPaths 按**路径字符串**判定（那里还列着
// /api/health 与 /api/health/live，本文件不复制或改动那个集合）。
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/cache"
)

// HealthCheck 健康检查：探测 Redis 连通性，异常时返回 503 便于容器编排重启。
// @Summary 服务健康检查
// @Description 探测 Redis 连通性；不可达时返回 503 degraded，供容器编排重启决策。非统一信封（裸 gin.H）。公开端点：无需鉴权，不受限流且不进访问日志（middleware.HealthPaths）
// @Tags 系统
// @Produce json
// @Success 200 {object} map[string]any "正常：{status:ok,message:backend is running}"
// @Failure 503 {object} map[string]any "降级：{status:degraded,redis:unreachable,error:原因}"
// @Router /health [get]
func HealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := cache.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "degraded",
			"redis":  "unreachable",
			"error":  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "backend is running"})
}

// HealthLive 存活探针（liveness）：仅表示进程存活，不依赖外部组件。
// @Summary 存活探针（liveness）
// @Description 仅表示进程存活，不依赖 Redis 等外部组件；容器编排探活应使用本端点，避免 Redis 抖动导致容器被重启。非统一信封。公开端点：无需鉴权，不受限流且不进访问日志（middleware.HealthPaths）
// @Tags 系统
// @Produce json
// @Success 200 {object} map[string]any "{status:ok}"
// @Router /health/live [get]
func HealthLive(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// APIRoot 根路由：返回服务名与版本号（API.md §1 的「服务信息」）。
// @Summary 服务信息（根路由）
// @Description 返回服务名与版本号。注意本文档 basePath 即 /api，故本端点在 paths 里落在 "/" 下（basePath + path = /api，与注册路径一致）。非统一信封。公开端点：无需鉴权
// @Tags 系统
// @Produce json
// @Success 200 {object} map[string]any "{message:Forklift Training System API,version:1.0.0}"
// @Router / [get]
func APIRoot(c *gin.Context) {
	c.JSON(200, gin.H{"message": "Forklift Training System API", "version": "1.0.0"})
}
