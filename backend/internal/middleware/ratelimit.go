// Package middleware 提供 Gin 中间件：CORS、JWT 认证、请求日志、panic 恢复、限流。
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"forklift-training/internal/config"
)

// ipLimiterEntry 单个 IP 的限流器及其最后访问时间（用于惰性清理）。
type ipLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// ipLimiterPool 维护 IP → 限流器映射，定期清理过期条目避免内存泄漏。
type ipLimiterPool struct {
	mu      sync.RWMutex
	entries map[string]*ipLimiterEntry
	rps     rate.Limit
	burst   int
}

// newIPLimiterPool 创建 IP 限流池，rps 为每秒令牌数，burst 为突发上限。
// 启动后台 goroutine 每 cleanupInterval 清理超过 maxIdle 未访问的条目。
// 限流池生命周期等于进程，进程退出时 goroutine 自然终止，无需显式停止。
func newIPLimiterPool(rps float64, burst int) *ipLimiterPool {
	p := &ipLimiterPool{
		entries: make(map[string]*ipLimiterEntry),
		rps:     rate.Limit(rps),
		burst:   burst,
	}
	go p.cleanupLoop()
	return p
}

// get 为指定 IP 获取或创建限流器。
func (p *ipLimiterPool) get(ip string) *rate.Limiter {
	p.mu.RLock()
	entry, ok := p.entries[ip]
	p.mu.RUnlock()
	if ok {
		return entry.limiter
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// 双检锁：拿到写锁后再次检查，避免并发重复创建
	if entry, ok := p.entries[ip]; ok {
		return entry.limiter
	}
	limiter := rate.NewLimiter(p.rps, p.burst)
	p.entries[ip] = &ipLimiterEntry{limiter: limiter, lastSeen: time.Now()}
	return limiter
}

// cleanupLoop 定期清理长时间未访问的 IP 条目，避免内存无限增长。
func (p *ipLimiterPool) cleanupLoop() {
	const cleanupInterval = 5 * time.Minute
	const maxIdle = 10 * time.Minute
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		p.mu.Lock()
		now := time.Now()
		for ip, entry := range p.entries {
			if now.Sub(entry.lastSeen) > maxIdle {
				delete(p.entries, ip)
			}
		}
		p.mu.Unlock()
	}
}

// RateLimit 基于 IP 的全局限流中间件（token bucket 算法）。
// 生产环境防暴力枚举/撞库/爬虫；通过 RATE_LIMIT_RPS / RATE_LIMIT_BURST 调节。
// 健康检查端点 /api/health 不受限流影响（探活不应被限流拦截）。
func RateLimit(cfg *config.Config, logger *zap.Logger) gin.HandlerFunc {
	if !cfg.RateLimit.Enabled {
		return func(c *gin.Context) { c.Next() }
	}
	pool := newIPLimiterPool(cfg.RateLimit.RPS, cfg.RateLimit.Burst)
	logger.Info("rate limit 已启用",
		zap.Float64("rps", cfg.RateLimit.RPS),
		zap.Int("burst", cfg.RateLimit.Burst),
	)
	return func(c *gin.Context) {
		// 健康检查端点放行（容器编排探活不应被限流拦截）
		if c.Request.URL.Path == "/api/health" || c.Request.URL.Path == "/api/health/live" {
			c.Next()
			return
		}
		limiter := pool.get(c.ClientIP())
		if !limiter.Allow() {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
			})
			return
		}
		c.Next()
	}
}
