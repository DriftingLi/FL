// StartAll 的循环锁（ADR-0061 §1）。
//
// 为什么值得单测：`daemon` 包此前**一只测试都没有**，而 `WithTicker` 这个自称「测试用」的
// seam 全仓零调用者——一个适配器的 seam 就是假 seam。本文件是它的第二个适配器，
// 并锁住装配根登记表的关键性质：**表里有几条，就有几条在跑**（漏装 contact-request-expire
// 三周无人发现，靠的就是「登记与启动之间没有任何可断言的东西」）。
package daemon

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestStartAllRunsEveryRegisteredTask(t *testing.T) {
	const n = 3
	var mu sync.Mutex
	runs := make(map[string]int, n)
	mark := func(name string) {
		mu.Lock()
		runs[name]++
		mu.Unlock()
	}

	// 每条 runner 必须拿到**自己的** channel：共享一只的话一次 tick 只会投递给其中一个 reader。
	// tickerFn 在各 runner 的 goroutine 里被调用，故取号用 atomic、就绪用 WaitGroup。
	pool := make([]chan time.Time, n)
	for i := range pool {
		pool[i] = make(chan time.Time, 1)
	}
	var next int32
	var tickersReady sync.WaitGroup
	tickersReady.Add(n)
	tickerFn := func(time.Duration) (<-chan time.Time, func()) {
		i := int(atomic.AddInt32(&next, 1)) - 1
		tickersReady.Done()
		return pool[i], func() {}
	}

	tasks := []Task{
		{Name: "one", Interval: time.Hour, Run: func(context.Context) { mark("one") }},
		{Name: "two", Interval: time.Hour, Run: func(context.Context) { mark("two") }},
		{Name: "three", Interval: time.Hour, Run: func(context.Context) { mark("three") }},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if got := StartAll(ctx, zap.NewNop(), tasks, WithJitter(0), WithTicker(tickerFn)); len(got) != n {
		t.Fatalf("StartAll 应为每条登记各起一个 Runner, 实际 %d", len(got))
	}

	tickersReady.Wait() // 三条循环都已建好 ticker（此时 tickerFn 已调用 3 次，取号不会越界）
	for _, ch := range pool {
		ch <- time.Now()
	}

	// tick 是注入的、因果确定；这里只等有界传播，不设短超时。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := runs["one"] == 1 && runs["two"] == 1 && runs["three"] == 1
		mu.Unlock()
		if done {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	t.Fatalf("一次 tick 后三条登记都应各执行一次, 实际 %+v", runs)
}
