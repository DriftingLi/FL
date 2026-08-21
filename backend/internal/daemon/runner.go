// Package daemon 提供进程内定时任务的通用守护骨架：panic 恢复、启动错峰、可注入 ticker 与 context 取消贯穿。
package daemon

import (
	"context"
	"math/rand"
	"time"

	"go.uber.org/zap"
)

// Runner 进程内守护 runner：固定 interval 周期执行 task，具备 panic 恢复、启动 jitter、ticker 可注入、context 取消。
type Runner struct {
	name     string
	interval time.Duration
	jitter   time.Duration
	logger   *zap.Logger
	task     func(context.Context)

	tickerFn func(time.Duration) (<-chan time.Time, func())
}

// RunnerOption 配置 Runner 的可选参数。
type RunnerOption func(*Runner)

// WithJitter 设置启动错峰最大值（0 表示不做 jitter）。
func WithJitter(j time.Duration) RunnerOption {
	return func(r *Runner) { r.jitter = j }
}

// WithTicker 注入 ticker 工厂（测试用：返回可控的 channel 与 stop 函数）。
// 默认使用 time.NewTicker。
func WithTicker(fn func(time.Duration) (<-chan time.Time, func())) RunnerOption {
	return func(r *Runner) { r.tickerFn = fn }
}

// NewRunner 构造守护 runner。
// interval 为固定周期；task 每周期执行一次，ctx 为 runner 的生命周期 context（取消时贯穿到 task）。
func NewRunner(name string, interval time.Duration, logger *zap.Logger, task func(context.Context), opts ...RunnerOption) *Runner {
	r := &Runner{
		name:     name,
		interval: interval,
		logger:   logger,
		task:     task,
		jitter:   interval / 10, // 默认 10% jitter，若 interval 为 0 则无 jitter
		tickerFn: defaultTicker,
	}
	for _, o := range opts {
		o(r)
	}
	// interval 为 0 时不做 jitter，避免 panic。
	if interval == 0 {
		r.jitter = 0
	}
	return r
}

func defaultTicker(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, func() { t.Stop() }
}

// Start 启动守护循环（非阻塞，内部起 goroutine）；ctx 取消时循环退出、ticker 停止。
// 首轮执行前会先等待 interval（带 jitter 错峰），与既有的「每 interval 扫描」语义一致。
func (r *Runner) Start(ctx context.Context) {
	go func() {
		// 启动错峰：避免多实例同时启动时惊群
		if r.jitter > 0 {
			jitter := time.Duration(rand.Int63n(int64(r.jitter) + 1))
			select {
			case <-time.After(jitter):
			case <-ctx.Done():
				return
			}
		}
		ch, stop := r.tickerFn(r.interval)
		if stop != nil {
			defer stop()
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				func() {
					defer func() {
						if rec := recover(); rec != nil {
							if r.logger != nil {
								r.logger.Error("daemon panic recovered", zap.String("runner", r.name), zap.Any("panic", rec))
							}
						}
					}()
					r.task(ctx)
				}()
			}
		}
	}()
}

// StartWithContext 与 Start 等价，显式命名便于测试可读性。
func (r *Runner) StartWithContext(ctx context.Context) { r.Start(ctx) }
