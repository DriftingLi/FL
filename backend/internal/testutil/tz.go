// Package testutil 提供测试用内存数据库与工厂方法，避免每个测试重复搭建环境。
// 使用纯 Go 的 glebarez/sqlite 驱动，无需 CGO，适合在 Windows/Linux/macOS 本地与 CI 运行。
package testutil

import "time"

// 把测试进程的 time.Local 拨到业务时区（#2204）。
//
// 为什么必须在这里拨：测试库是 sqlite（db.go 的 NewMemoryDB），它把 time.Time 存成**带偏移量的
// 字面量字符串**，比较走字符串字典序而非瞬时比较。业务日界由 clock.DayStart 恒在 Asia/Shanghai
// 取，而行时刻来自 Now()（time.Now()，跟随 time.Local）。GitHub runner 的 time.Local 是 UTC ⇒
// 行写成 "...23:28:00+00:00"、边界写成 "...00:00:00+08:00"，字典序把「行晚于边界」判反：
//
//	row   = "2026-11-01 23:28:00+00:00"
//	bound = "2026-11-02 00:00:00+08:00"   ⇒  created_at >= bound 为假，当日计数恒 0
//
// 后果是「上海 00:00–08:00 这 8 小时里所有日界计数判反」：forum 采纳日额度与 points 任务进度
// 三条契约断言无差别假红（实测窗口 8/24 个时刻，红 run 41362241396 落在 07:28）。
//
// 为什么不是改调用点：testutil.Now() 有 309 个调用点 / 100 个文件，且改成 clock.Now() 实测
// 无效（行带 +08 后另有 6 处 time.Local 日期构造仍按 UTC，两套口径并存）。
// 为什么不在 clock 包里改：那会进生产二进制。本包零生产引用（只有测试与分层守卫的注释提到它），
// 因此拨 Local 只影响测试进程。生产不受影响：PG 存真瞬时，容器又都设 TZ=Asia/Shanghai。
func init() {
	if l, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		time.Local = l
	}
}
