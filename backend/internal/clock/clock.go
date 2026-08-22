// Package clock 是全系统时间与时区政策的单点定义（spec #296）：
// 业务时间固定为 Asia/Shanghai 口径。业务服务经构造注入 Clock 接口以便测试定格；
// 遗留全局调用方（如 auth_service.beijingNow）使用包级 Now()。
package clock

import "time"

// loc 业务时区单点。LoadLocation 失败（无 tzdata 环境）时退化为固定东八区，语义保持一致。
var loc = func() *time.Location {
	l, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return l
}()

// Location 返回业务时区（Asia/Shanghai）。时区政策仅在此处定义，勿在他处硬编码。
func Location() *time.Location { return loc }

// Clock 提供当前业务时间的小接口，供服务构造注入（生产 Real / 测试 Fake）。
type Clock interface {
	Now() time.Time
}

// realClock 生产实钟实现。
type realClock struct{}

// Real 返回生产实钟（Asia/Shanghai）。
func Real() Clock { return realClock{} }

func (realClock) Now() time.Time { return time.Now().In(loc) }

// Now 包级便捷入口：返回当前业务时间（Asia/Shanghai）。
// 供未注入 Clock 的遗留调用方委托使用；新代码请经 Clock 注入。
func Now() time.Time { return time.Now().In(loc) }

// Fake 测试用时钟，定格在 T；T 可变以便跨日场景推进。
type Fake struct{ T time.Time }

// At 构造定格于 t 的 Fake 时钟。
func At(t time.Time) *Fake { return &Fake{T: t} }

// Now 实现 Clock 接口，恒定返回构造/更新时的时刻。
func (f *Fake) Now() time.Time { return f.T }
