// 限流器 lastSeen 的原子化契约（spec #1345 决策 11 / 真实缺陷 #7）。
//
// seam：ipLimiterPool 的 get（命中路径）与 cleanupOnce（清理路径）——这两条分支读写同一个
// 时间戳。测的是「持续请求的 IP 会不会被当成闲置清掉」这一外部可观察行为（同一个限流器、
// 令牌余额不重置），不测 rate.Limiter 本身（那是 golang.org/x/time 的语义）。
package middleware

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// stepClock 推进式时钟（单线程用例专用）：把「闲置多久」做成确定性的，不靠真实 sleep——
// Windows 的 sleep 粒度是 10ms 级，拿它测 50ms 阈值的清理会时好时坏。
type stepClock struct{ t time.Time }

func (c *stepClock) now() time.Time          { return c.t }
func (c *stepClock) advance(d time.Duration) { c.t = c.t.Add(d) }

// entryCount 读池内条目数（测试侧观察点，须持锁）。
func (p *ipLimiterPool) entryCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.entries)
}

// 判据 1：持续请求的 IP 经过一次 cleanup 之后**仍是同一个限流器**（令牌余额不重置）。
//
// 缺陷形态：lastSeen 只在创建时写一次 ⇒ 一直在打的 IP 也被 cleanup 判成闲置删掉，下一次请求
// 重建一个满桶的新限流器 ⇒ 撞库/爬虫每 maxIdle 就白拿一次「重新计数」。
func TestIPLimiterPool_持续请求的IP经过cleanup后仍是同一个限流器(t *testing.T) {
	clock := &stepClock{t: time.Unix(1700000000, 0)}
	const maxIdle = 50 * time.Millisecond
	p := newIPLimiterPool(0.0001, 1, maxIdle)
	p.now = clock.now

	// 前置：burst=1，这个 IP 现在已经被限住了。
	first := p.get("203.0.113.9")
	if !first.Allow() {
		t.Fatal("前置条件不成立：首枚令牌应可用")
	}
	if first.Allow() {
		t.Fatal("前置条件不成立：burst=1 的第二次请求应被拒")
	}

	// 每 20ms 打一次、共 5 次（累计 100ms > maxIdle），期间穿插两轮清理。
	for i := 0; i < 5; i++ {
		clock.advance(20 * time.Millisecond)
		if got := p.get("203.0.113.9"); got != first {
			t.Fatalf("第 %d 次命中换了限流器（桶被重建 ⇒ 令牌余额归零重来）", i+1)
		}
		if i%2 == 1 {
			p.cleanupOnce()
			if p.entryCount() != 1 {
				t.Fatalf("第 %d 轮清理删掉了正在被访问的 IP（池内 %d 条）", i/2+1, p.entryCount())
			}
		}
	}

	again := p.get("203.0.113.9")
	if again != first {
		t.Fatalf("cleanup 之后换了限流器：%p → %p（限流对持续流量失效）", first, again)
	}
	if again.Allow() {
		t.Fatal("同一个限流器的令牌余额被重置（应仍是耗尽态）")
	}
}

// 对照组（防恒绿）：真的闲置超过 maxIdle 的 IP 必须被清掉——上面那条用例不能靠「清理根本没跑」
// 通过，否则「lastSeen 从不刷新」和「lastSeen 一直刷新」两个相反的实现都能判绿。
func TestIPLimiterPool_真正闲置的IP仍被清理(t *testing.T) {
	clock := &stepClock{t: time.Unix(1700000000, 0)}
	p := newIPLimiterPool(0.0001, 1, 50*time.Millisecond)
	p.now = clock.now

	p.get("203.0.113.30")
	clock.advance(51 * time.Millisecond)
	p.cleanupOnce()
	if p.entryCount() != 0 {
		t.Fatalf("闲置 51ms > maxIdle 50ms 的条目应被清理，实际剩 %d 条", p.entryCount())
	}
	// 再访问则重建一个新限流器（内存不泄漏的另一半语义）。
	if got := p.get("203.0.113.30"); got == nil {
		t.Fatal("重建后应拿到新的限流器")
	}
}

// 判据 2：命中路径（不持写锁刷新 lastSeen）与清理路径（持写锁读 lastSeen）并发不形成数据竞争。
// lastSeen 退回普通字段时 `-race` 在这一对读写上报红（读侧只有 RLock，写侧与写侧之间也不互斥）。
func TestIPLimiterPool_命中与清理并发无数据竞争(t *testing.T) {
	// maxIdle 取 1ns：让清理真的反复命中条目，读写交错才有机会发生。
	p := newIPLimiterPool(10, 5, time.Nanosecond)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ip := fmt.Sprintf("203.0.113.%d", i)
			for n := 0; n < 500; n++ {
				if l := p.get(ip); l == nil {
					t.Error("get 返回了 nil 限流器")
					return
				}
			}
		}(i)
	}
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 300; n++ {
				p.cleanupOnce()
			}
		}()
	}
	wg.Wait()

	// 收尾再确认一次池仍可用（并发期间没有把结构跑坏）。
	if l := p.get("203.0.113.0"); l == nil {
		t.Fatal("并发后 get 返回 nil")
	}
}
