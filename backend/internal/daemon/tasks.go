// 本文件：守护的**登记面**。装配根（api.NewDeps）用 Task 表声明进程内周期任务，
// cmd/server 只负责 StartAll——「声明即装配」，加守护不再需要在 main 里手写一次 start
// （漏装配曾让 contact-request-expire 静默停摆三周，见 ADR-0061 §1 / #1197）。
package daemon

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Task 一个待启动的守护声明。Name 是启动日志与测试断言的标识，必须与 Runner 内的名字一致；
// Interval 为周期；Run 每周期执行一次，ctx 为生命周期 context（取消时贯穿）。
// panic 恢复、启动 jitter、ticker 停止与 ctx 取消全部由 StartAll 托管，登记方只写业务动作。
type Task struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context)
}

// StartAll 按登记表逐个起守护（非阻塞），返回 Runner 列表。
// 登记 ≠ 启动：NewDeps 只产出表，只有这里起 goroutine——契约测试构造 Deps 时不会拉起任何周期任务。
// opts 透传给每个 Runner（生产不传；测试用 WithTicker 换掉真时钟，见 tasks_test.go）。
func StartAll(ctx context.Context, logger *zap.Logger, tasks []Task, opts ...RunnerOption) []*Runner {
	out := make([]*Runner, 0, len(tasks))
	for _, t := range tasks {
		r := NewRunner(t.Name, t.Interval, logger, t.Run, opts...)
		r.Start(ctx)
		if logger != nil {
			logger.Info("守护任务已启动", zap.String("name", t.Name), zap.String("interval", t.Interval.String()))
		}
		out = append(out, r)
	}
	return out
}
