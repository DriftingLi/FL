// Package apitypes 前端契约类型的窄域 codegen（ADR-0019 专项第一步 / spec #940 片五③；
// 按域解冻的执行口径见 ADR-0048）。
//
// 与 ADR-0030 / ADR-0047 §1 / §5 的三条生成管道同构（声明表 → 渲染 → 生成物 → 字节级同步契约）：
// 输入是 swagger 产物（由注解经 make swagger 生成，CI 有新鲜度锁），
// 输出是 frontend/src/api/generated/<domain>.ts。
//
// 范围纪律：只做**声明表里登记的域**（ADR-0048 决策 2：Web 消费面 + 域内完整）。
package apitypes

// Endpoint 生成物头部列出的端点。
//
// 两个用途：一是写给生成物的读者看（这些类型从哪来），二是被 TestDomainEndpointsDeclareData
// 当作「本域注解完整性」的断言表 —— NoData 为真表示该端点**有意**没有返回载荷
// （注解显式为 response.R，浏览器只拿 errcode/errmsg），未登记又缺 data 指认的端点直接判红。
type Endpoint struct {
	Method string
	Path   string
	NoData bool
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

// Domains 声明表。顺序 = ADR-0048 决策 7 的分片顺序，也是生成物的输出顺序。
// 域内完整性（决策 2）体现在 Roots/Endpoints 覆盖该域**全部** Web 消费端点，
// 而不是只挑「注解已就绪」的那几个。
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
	{
		Name:  "contribution",
		Title: "投稿与审核（/api/contributions/*、/api/admin/contributions/*，含举报队列）",
		Roots: []string{
			"service.ContributionFileDTO",
			"service.ContributionItemDTO",
			"service.ContributionPageResult",
			"service.ContributionReportPageResult",
			"service.DownloadResult",
		},
		Endpoints: []Endpoint{
			{Method: "POST", Path: "/contributions/upload-file"},
			{Method: "POST", Path: "/contributions"},
			{Method: "GET", Path: "/contributions"},
			{Method: "GET", Path: "/contributions/mine"},
			{Method: "GET", Path: "/contributions/{id}"},
			{Method: "POST", Path: "/contributions/{id}/download"},
			{Method: "DELETE", Path: "/contributions/{id}", NoData: true},
			{Method: "POST", Path: "/contributions/{id}/report", NoData: true},
			{Method: "GET", Path: "/admin/contributions/pending"},
			{Method: "POST", Path: "/admin/contributions/{id}/approve"},
			{Method: "POST", Path: "/admin/contributions/{id}/reject"},
			{Method: "POST", Path: "/admin/contributions/{id}/archive"},
			{Method: "GET", Path: "/admin/contributions/reports"},
			{Method: "POST", Path: "/admin/contributions/reports/{id}/handle", NoData: true},
		},
	},
	{
		Name:  "practiceMode",
		Title: "题库练习模式（/api/practice-mode/*：随机 / 标签 / 顺序 / 进度 / 判定 / 统计 / 历史）",
		Roots: []string{
			"service.PracticeStartResultDTO",
			"service.ProgressResultDTO",
			"service.SubmitResultDTO",
			"service.PracticePracticeStatsDTO",
			"service.PracticeStatsDTO",
			"service.HistoryResultDTO",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/practice-mode/free"},
			{Method: "GET", Path: "/practice-mode/tag"},
			{Method: "GET", Path: "/practice-mode/sequential"},
			{Method: "GET", Path: "/practice-mode/sequential-progress"},
			{Method: "POST", Path: "/practice-mode/progress", NoData: true},
			{Method: "GET", Path: "/practice-mode/progress"},
			{Method: "POST", Path: "/practice-mode/submit"},
			{Method: "GET", Path: "/practice-mode/practice-stats"},
			{Method: "GET", Path: "/practice-mode/stats"},
			{Method: "GET", Path: "/practice-mode/history"},
		},
	},
	{
		Name:  "mockExam",
		Title: "模拟考试（/api/mock-exam/*，含真题卷整卷复用链路）",
		Roots: []string{
			"service.MockExamStartDTO",
			"service.MockExamResumeDTO",
			"service.MockExamSubmitDTO",
			"service.MockExamResultDTO",
			"service.MockExamHistoryDTO",
		},
		Endpoints: []Endpoint{
			{Method: "POST", Path: "/mock-exam/start"},
			{Method: "POST", Path: "/mock-exam/{mock_exam_id}/save", NoData: true},
			{Method: "GET", Path: "/mock-exam/{mock_exam_id}/resume"},
			{Method: "POST", Path: "/mock-exam/{mock_exam_id}/submit"},
			{Method: "GET", Path: "/mock-exam/{mock_exam_id}/result"},
			{Method: "GET", Path: "/mock-exam/history"},
		},
	},
}
