// 装配根守护登记表锁（ADR-0061 §1 / #1197）。
//
// 锁的是**数据**而不是源码文本：`cmd/server` 只把 deps.Daemons 交给 daemon.StartAll，
// 所以「表里有什么」就等于「跑着什么」。本仓标准是断言行为、不断言源码文本
// （静态扫 main.go 的守卫改名即可绕过，故否）。
//
// 新增守护必须同时改 expectedDaemonNames——这正是本锁的作用：登记表与期望名单一旦分叉就判红，
// 而「定义了 Start*Runner 却没人调」这个把 #1197 藏了三周的形状，在新结构里根本没有宿主
// （守护的声明本身就活在这张表里）。
package api

import (
	"context"
	"sort"
	"testing"
	"time"

	"forklift-training/internal/daemon"
	"forklift-training/internal/testutil"
)

// expectedDaemonNames 装配根应登记的守护（**按字典序**，与下面的排序比较同形）。
var expectedDaemonNames = []string{
	"contact-request-expire",
	"contribution-file-cleanup",
	"forum-image-cleanup",
}

func TestDaemonRegistryLock(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	d := newContractDeps(t, db, nil)

	names := make([]string, 0, len(d.Daemons))
	for _, task := range d.Daemons {
		if task.Name == "" {
			t.Fatalf("登记表里有无名守护：启动日志与断言都会失去标识")
		}
		if task.Interval <= 0 {
			t.Fatalf("%s 的周期非正数，Runner 会退化成无 jitter/无节奏", task.Name)
		}
		if task.Run == nil {
			t.Fatalf("%s 登记了名字却没有动作", task.Name)
		}
		names = append(names, task.Name)
	}

	sort.Strings(names)
	if len(names) != len(expectedDaemonNames) {
		t.Fatalf("守护登记表与期望名单条数不一致：登记 %v / 期望 %v", names, expectedDaemonNames)
	}
	for i := range names {
		if names[i] != expectedDaemonNames[i] {
			t.Fatalf("守护登记表漂移：第 %d 条是 %q，期望 %q（全表 %v）", i, names[i], expectedDaemonNames[i], names)
		}
	}
}

// 锁「登记 → 启动」这条接线本身（复核后补：#1197 的 bug 类正是「有登记，没人启动」）。
// 只锁条数——每个 tick 跑几次由 daemon 包自己锁（tasks_test.go），两层合起来才是全链。
// 假时钟永不投递，故这里不会执行任何业务动作。
func TestStartDaemonsStartsEveryRegisteredTask(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	d := newContractDeps(t, db, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runners := d.StartDaemons(ctx, daemon.WithJitter(0), daemon.WithTicker(func(time.Duration) (<-chan time.Time, func()) {
		return make(chan time.Time), func() {}
	}))
	if len(runners) != len(d.Daemons) {
		t.Fatalf("登记 %d 条守护、实际起了 %d 个 Runner（登记与启动之间又裂开了）", len(d.Daemons), len(runners))
	}
	if len(runners) == 0 {
		t.Fatalf("一条都没起——本锁会因空集合恒绿")
	}
}
