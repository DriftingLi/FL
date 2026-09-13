// Package apitypes 前端契约类型的窄域 codegen（ADR-0019 专项第一步 / spec #940 片五③）。
//
// 与 ADR-0030 / ADR-0047 §1 / §5 的三条生成管道同构（声明表 → 渲染 → 生成物 → 字节级同步契约）：
// 输入是 swagger 产物（由注解经 make swagger 生成，CI 有新鲜度锁），
// 输出是 frontend/src/api/generated/<domain>.ts。
//
// 范围纪律：只做**声明表里登记的域**。全量 API 层 codegen 仍等专项结论（ADR-0019 未解冻），
// 本包是那条结论的试点证据面。
package apitypes

// Endpoint 生成物头部列出的端点（文档用途：让读者知道这些类型从哪来）。
type Endpoint struct {
	Method string
	Path   string
}

// Domain 一个域的生成声明。
//
// Roots 是该域消费的根类型（swagger definitions 里的键，如 service.CheckInResult）：
// 渲染时取它们的**传递闭包**（引用到的类型一并生成），故新增嵌套类型不必改声明表。
// 只增不改：域内新增端点/字段由注解层驱动，声明表只在「新增一个域」时动。
type Domain struct {
	// Name 生成物文件名（frontend/src/api/generated/<Name>.ts）。
	Name string
	// Title 生成物头部的一句话说明。
	Title string
	// Roots 根类型（Go 类型名，带 swagger definitions 的包前缀）。
	Roots []string
	// Endpoints 该域端点，仅用于生成物头部注释。
	Endpoints []Endpoint
}

// Domains 声明表：当前只有打卡域一个试点（见 spec #940 片五③ 的选域判据：
// 字段面稳定 + 已有契约测试 + 无跨端分歧）。
var Domains = []Domain{
	{
		Name:  "checkin",
		Title: "每日打卡（/api/check-in/*，ADR-0028 独立蓝图）",
		Roots: []string{
			"service.CheckInResult",
			"service.CheckInCalendarResult",
			"service.CheckInRankResult",
		},
		Endpoints: []Endpoint{
			{Method: "POST", Path: "/check-in"},
			{Method: "GET", Path: "/check-in/calendar"},
			{Method: "GET", Path: "/check-in/rank"},
		},
	},
}
