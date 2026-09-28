// Package middleware 提供 Gin 中间件：CORS、JWT 认证、请求日志、panic 恢复、限流。
package middleware

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"forklift-training/internal/config"
	"forklift-training/internal/daemon"
)

// defaultIPLimiterMaxIdle 闲置多久后回收某个 IP 的限流器（生产口径）。
const defaultIPLimiterMaxIdle = 10 * time.Minute

// ipLimiterEntry 单个 IP 的限流器及其最后访问时间（用于惰性清理）。
//
// lastSeen 是**原子时间戳**（unix Nano）而不是普通字段：命中分支必须能刷新它，且不能为此
// 取写锁——普通字段会与 cleanupOnce 的读形成数据竞争（-race 直接报红），取写锁则把热点路径
// 串行化。二者都不可接受。（spec #1345 决策 11 / 真实缺陷 #7）
type ipLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen atomic.Int64
}

// touch 记录一次访问（原子写，调用方无需持池锁）。
func (e *ipLimiterEntry) touch(now time.Time) { e.lastSeen.Store(now.UnixNano()) }

// idleFor 返回距最后一次访问过去了多久（原子读，cleanupOnce 用）。
func (e *ipLimiterEntry) idleFor(now time.Time) time.Duration {
	return now.Sub(time.Unix(0, e.lastSeen.Load()))
}

// ipLimiterPool 维护 IP → 限流器映射，定期清理过期条目避免内存泄漏。
type ipLimiterPool struct {
	mu      sync.RWMutex
	entries map[string]*ipLimiterEntry
	rps     rate.Limit
	burst   int
	// maxIdle 闲置回收阈值。生产走 defaultIPLimiterMaxIdle；测试压短它，才能跑到
	// 「持续请求的 IP 经过一次 cleanup」这一支（缺陷 #7 的正是要害的那一帧）。
	maxIdle time.Duration
	// now 时钟注入点（生产为 time.Now）。测试用它把闲置判定做成确定性的，
	// 不靠真实 sleep——sleep 的粒度在 Windows 上是 10ms 级，会让这条用例时好时坏。
	now func() time.Time
}

// newIPLimiterPool 创建限流池本体，不启动清理 runner（由调用方决定生命周期）。
func newIPLimiterPool(rps float64, burst int, maxIdle time.Duration) *ipLimiterPool {
	return &ipLimiterPool{
		entries: make(map[string]*ipLimiterEntry),
		rps:     rate.Limit(rps),
		burst:   burst,
		maxIdle: maxIdle,
		now:     time.Now,
	}
}

// newIPLimiterPoolWithLogger 创建 IP 限流池，rps 为每秒令牌数，burst 为突发上限，使用外部 logger。
// 启动守护 runner 每 cleanupInterval 清理超过 maxIdle 未访问的条目（panic 恢复 + jitter 错峰 + 可注入 ticker）。
// 限流池生命周期等于进程，进程退出时 goroutine 自然终止，无需显式停止。
func newIPLimiterPoolWithLogger(rps float64, burst int, logger *zap.Logger) *ipLimiterPool {
	p := newIPLimiterPool(rps, burst, defaultIPLimiterMaxIdle)
	if logger == nil {
		logger = zap.NewNop()
	}
	runner := daemon.NewRunner("rate-limit-cleanup", 5*time.Minute, logger, func(ctx context.Context) {
		p.cleanupOnce()
	})
	runner.Start(context.Background())
	return p
}

// get 为指定 IP 获取或创建限流器。
//
// 命中分支只原子刷新 lastSeen、不取写锁：不刷新就会出现「流量一直在、条目却被 cleanup 当成闲置
// 清掉」，下一次请求重建一个满桶的新限流器 ⇒ 限流对持续流量每 maxIdle 失效一次（缺陷 #7）。
func (p *ipLimiterPool) get(ip string) *rate.Limiter {
	now := p.now()
	p.mu.RLock()
	entry, ok := p.entries[ip]
	p.mu.RUnlock()
	if ok {
		entry.touch(now)
		return entry.limiter
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// 双检锁：拿到写锁后再次检查，避免并发重复创建
	if entry, ok := p.entries[ip]; ok {
		entry.touch(now)
		return entry.limiter
	}
	entry = &ipLimiterEntry{limiter: rate.NewLimiter(p.rps, p.burst)}
	entry.touch(now)
	p.entries[ip] = entry
	return entry.limiter
}

// cleanupOnce 单次清理长时间未访问的 IP 条目（由守护 runner 周期调用）。
// 判闲置读的是 entry 的原子 lastSeen——命中路径不持写锁刷新，这里也不能假设自己是唯一写者。
func (p *ipLimiterPool) cleanupOnce() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	for ip, entry := range p.entries {
		if entry.idleFor(now) > p.maxIdle {
			delete(p.entries, ip)
		}
	}
}

// RateLimit 基于 IP 的全局限流中间件（token bucket 算法）。
// 生产环境防暴力枚举/撞库/爬虫；通过 RATE_LIMIT_RPS / RATE_LIMIT_BURST 调节。
// 健康检查端点 /api/health 不受限流影响（探活不应被限流拦截）。
func RateLimit(cfg *config.Config, logger *zap.Logger) gin.HandlerFunc {
	if !cfg.RateLimit.Enabled {
		return func(c *gin.Context) { c.Next() }
	}
	pool := newIPLimiterPoolWithLogger(cfg.RateLimit.RPS, cfg.RateLimit.Burst, logger)
	logger.Info("rate limit 已启用",
		zap.Float64("rps", cfg.RateLimit.RPS),
		zap.Int("burst", cfg.RateLimit.Burst),
	)
	return func(c *gin.Context) {
		// 健康检查端点放行（容器编排探活不应被限流拦截）
		if _, skip := HealthPaths[c.Request.URL.Path]; skip {
			c.Next()
			return
		}
		limiter := pool.get(ClientIP(c))
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
