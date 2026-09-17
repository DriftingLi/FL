// Package api 实现 HTTP handlers。
// 本文件：列表端点**信封登记表**（ADR-0056 §1 / issue #1095）。
//
// 背景：26 个域各自声明 page struct，键集合/键序/方言（pages 还是 page_size）各不相同，
// 两处 api 层手拼 gin.H 信封，还有一处「只服务 swagger 的类型」而运行时出字节的是 map。
// 本表把「每个列表结果类型的键集合 + 键序 + 方言 + 出现的端点」收成一份声明：
//
//   - **只登记不改值**：两种方言（pages / page_size）原样保留，改名是破坏性契约变更
//     （ADR-0048「只增不破」），另案处理。
//   - 键序 = 输出字节序（ADR-0009 §2），由 envelope_registry_test.go 的形状锁逐行反射验证
//     （marshal 零值 → 取顶层 key 顺序 == Keys），字段声明序写错立刻红。
//   - 覆盖锁：每个「含 total + 切片字段」的导出结果类型都必须出现在本表里（信封或载荷，
//     载荷须写明理由）；漏登记直接红。
//   - 装配单点：新端点不再手拼 map —— 有域内 DTO 用域内 page struct；没有的用
//     paging.ItemsPage[T]（键序 items/page/page_size/total）。
//
// 一行 = 一个结果类型，它的全部端点在 Endpoints 里列全（端点跨行唯一，测试断言）。
package api

import (
	"forklift-training/internal/model"
	"forklift-training/internal/service"
	vmodel "forklift-training/internal/valuation/model"
	"forklift-training/pkg/paging"
)

// EnvelopeSpec 一条列表端点信封登记：结果类型 + 键集合/键序 + 方言 + 端点清单。
type EnvelopeSpec struct {
	// Result 结果类型全名（包名.类型名），必须与 Sample 的 reflect 类型名一致。
	Result string
	// Endpoints 该结果类型当前出现的端点（"METHOD 路径"，路径用 swagger @Router 形态）。
	Endpoints []string
	// Keys 键集合 + 键序（= 输出的字节序，ADR-0009 §2）。
	Keys []string
	// Dialect 分页元数据方言（paging.DialectPages / DialectPageSize / DialectNone）。
	Dialect paging.Dialect
	// Sample 该结果类型的零值：形状锁（reflect + json.Marshal）用它验证 Result / Keys。
	Sample any
}

// PayloadSpec 含 total 但**不是**分页信封的结果类型（登记理由）。
// 覆盖锁据此不把它们算作漏登记——它们的 total 是「总量」，不是「列表总条数」。
type PayloadSpec struct {
	Result string
	Reason string
}

// envelopeRegistry 信封登记表（按 Result 字典序）。
var envelopeRegistry = []EnvelopeSpec{
	{Result: "api.AuditLogPageResult", Endpoints: []string{"GET /admin/audit-logs"},
		Keys: []string{"items", "page", "pages", "total"}, Dialect: paging.DialectPages,
		Sample: AuditLogPageResult{}},
	{Result: "model.ListBatteryResponse", Endpoints: []string{"GET /valuation/battery/evaluations"},
		Keys: []string{"total", "items"}, Dialect: paging.DialectNone,
		Sample: vmodel.ListBatteryResponse{}},
	{Result: "paging.ItemsPage[model.ContactRequest]", Endpoints: []string{"GET /admin/recruit/requests"},
		Keys: []string{"items", "page", "page_size", "total"}, Dialect: paging.DialectPageSize,
		Sample: paging.ItemsPage[model.ContactRequest]{}},
	{Result: "paging.ItemsPage[model.RecruitResumeView]", Endpoints: []string{"GET /admin/recruit/views"},
		Keys: []string{"items", "page", "page_size", "total"}, Dialect: paging.DialectPageSize,
		Sample: paging.ItemsPage[model.RecruitResumeView]{}},
	{Result: "service.ApplicationListResult", Endpoints: []string{"GET /resume/applications"},
		Keys: []string{"items", "total", "page", "page_size"}, Dialect: paging.DialectPageSize,
		Sample: service.ApplicationListResult{}},
	{Result: "service.CheckInRankResult", Endpoints: []string{"GET /check-in/rank"},
		Keys: []string{"items", "total", "page", "pages", "me"}, Dialect: paging.DialectPages,
		Sample: service.CheckInRankResult{}},
	{Result: "service.ContactRequestListResult", Endpoints: []string{"GET /recruit/contact-requests", "GET /resume/contact-requests"},
		Keys: []string{"items", "page", "page_size", "total"}, Dialect: paging.DialectPageSize,
		Sample: service.ContactRequestListResult{}},
	{Result: "service.ContributionPageResult", Endpoints: []string{"GET /contributions", "GET /contributions/mine", "GET /admin/contributions/pending"},
		Keys: []string{"items", "total", "page", "page_size"}, Dialect: paging.DialectPageSize,
		Sample: service.ContributionPageResult{}},
	{Result: "service.ContributionReportPageResult", Endpoints: []string{"GET /admin/contributions/reports"},
		Keys: []string{"items", "total", "page", "page_size"}, Dialect: paging.DialectPageSize,
		Sample: service.ContributionReportPageResult{}},
	{Result: "service.CoursePageResult", Endpoints: []string{"GET /courses", "GET /admin/courses", "GET /tutor/courses"},
		Keys: []string{"courses", "page", "pages", "total"}, Dialect: paging.DialectPages,
		Sample: service.CoursePageResult{}},
	{Result: "service.DiagnosisFaultCodePage", Endpoints: []string{"GET /ai-assistant/diagnosis/fault-codes"},
		Keys: []string{"items", "total"}, Dialect: paging.DialectNone,
		Sample: service.DiagnosisFaultCodePage{}},
	{Result: "service.FavoritePageResult", Endpoints: []string{"GET /favorites"},
		Keys: []string{"page", "pages", "total", "favorites"}, Dialect: paging.DialectPages,
		Sample: service.FavoritePageResult{}},
	{Result: "service.FeaturedContentPageResult", Endpoints: []string{"GET /featured-contents", "GET /admin/featured-contents"},
		Keys: []string{"items", "page", "pages", "total"}, Dialect: paging.DialectPages,
		Sample: service.FeaturedContentPageResult{}},
	{Result: "service.ForumReportPageResult", Endpoints: []string{"GET /admin/forum/reports"},
		Keys: []string{"page", "pages", "total", "reports"}, Dialect: paging.DialectPages,
		Sample: service.ForumReportPageResult{}},
	{Result: "service.ForumTopicDetailDTO", Endpoints: []string{"GET /forum/topics/{id}", "GET /admin/forum/topics/{id}"},
		Keys: []string{"page", "pages", "replies", "topic", "total"}, Dialect: paging.DialectPages,
		Sample: service.ForumTopicDetailDTO{}},
	{Result: "service.ForumTopicPageResult", Endpoints: []string{
		"GET /forum/topics", "GET /admin/forum/topics", "GET /forum/my-topics",
		"GET /forum/my-liked-topics", "GET /forum/my-observed", "GET /forum/my-view-history"},
		Keys: []string{"page", "pages", "topics", "total"}, Dialect: paging.DialectPages,
		Sample: service.ForumTopicPageResult{}},
	{Result: "service.HistoryResultDTO", Endpoints: []string{"GET /practice-mode/history"},
		Keys: []string{"total", "page", "page_size", "records"}, Dialect: paging.DialectPageSize,
		Sample: service.HistoryResultDTO{}},
	{Result: "service.HrwaiUserPageResult", Endpoints: []string{"GET /admin/hrwai-users"},
		Keys: []string{"list", "page", "page_size", "total"}, Dialect: paging.DialectPageSize,
		Sample: service.HrwaiUserPageResult{}},
	{Result: "service.JobListResult", Endpoints: []string{"GET /jobs", "GET /recruit/jobs"},
		Keys: []string{"items", "total"}, Dialect: paging.DialectNone,
		Sample: service.JobListResult{}},
	{Result: "service.MaterialPageResult", Endpoints: []string{"GET /materials", "GET /student/materials"},
		Keys: []string{"page", "pages", "total", "materials"}, Dialect: paging.DialectPages,
		Sample: service.MaterialPageResult{}},
	{Result: "service.MockExamHistoryDTO", Endpoints: []string{"GET /mock-exam/history"},
		Keys: []string{"total", "page", "page_size", "exams"}, Dialect: paging.DialectPageSize,
		Sample: service.MockExamHistoryDTO{}},
	{Result: "service.MyReplyPageResult", Endpoints: []string{"GET /forum/my-replies"},
		Keys: []string{"page", "pages", "total", "replies"}, Dialect: paging.DialectPages,
		Sample: service.MyReplyPageResult{}},
	{Result: "service.NotePageDTO", Endpoints: []string{"GET /notes"},
		Keys: []string{"items", "page", "page_size", "total"}, Dialect: paging.DialectPageSize,
		Sample: service.NotePageDTO{}},
	{Result: "service.NotificationListPageResult", Endpoints: []string{"GET /notifications"},
		Keys: []string{"items", "page", "pages", "total", "unread_count"}, Dialect: paging.DialectPages,
		Sample: service.NotificationListPageResult{}},
	{Result: "service.PointsLedgerResult", Endpoints: []string{"GET /points/ledger", "GET /admin/points/ledger"},
		Keys: []string{"items", "total", "page", "pages"}, Dialect: paging.DialectPages,
		Sample: service.PointsLedgerResult{}},
	{Result: "service.ProfileChangeRequestPageResult", Endpoints: []string{"GET /admin/profile-reviews"},
		Keys: []string{"page", "pages", "requests", "total"}, Dialect: paging.DialectPages,
		Sample: service.ProfileChangeRequestPageResult{}},
	{Result: "service.QuestionCommentPageResult", Endpoints: []string{"GET /questions/{question_id}/comments"},
		Keys: []string{"items", "page", "page_size", "total"}, Dialect: paging.DialectPageSize,
		Sample: service.QuestionCommentPageResult{}},
	{Result: "service.QuestionPageDTO", Endpoints: []string{"GET /question-bank/questions"},
		Keys: []string{"page", "page_size", "questions", "total"}, Dialect: paging.DialectPageSize,
		Sample: service.QuestionPageDTO{}},
	{Result: "service.RecruitListResult", Endpoints: []string{"GET /recruit/resumes"},
		Keys: []string{"items", "total"}, Dialect: paging.DialectNone,
		Sample: service.RecruitListResult{}},
	{Result: "service.RecruiterApplicationListResult", Endpoints: []string{"GET /recruit/jobs/{id}/applications"},
		Keys: []string{"items", "total", "page", "page_size", "unread_count", "job_title"}, Dialect: paging.DialectPageSize,
		Sample: service.RecruiterApplicationListResult{}},
	{Result: "service.RecruiterListResult", Endpoints: []string{"GET /admin/recruiters"},
		Keys: []string{"total", "page", "items"}, Dialect: paging.DialectNone,
		Sample: service.RecruiterListResult{}},
	{Result: "service.ReportListResult", Endpoints: []string{"GET /admin/job-reports"},
		Keys: []string{"items", "total", "page", "page_size"}, Dialect: paging.DialectPageSize,
		Sample: service.ReportListResult{}},
	{Result: "service.SearchPageDTO", Endpoints: []string{"GET /search"},
		Keys: []string{"keyword", "type", "total", "page", "pages", "items"}, Dialect: paging.DialectPages,
		Sample: service.SearchPageDTO{}},
	{Result: "service.StudyRecordPageResult", Endpoints: []string{"GET /student/records"},
		Keys: []string{"page", "pages", "records", "total"}, Dialect: paging.DialectPages,
		Sample: service.StudyRecordPageResult{}},
	{Result: "service.TutorListDTO", Endpoints: []string{"GET /admin/tutors"},
		Keys: []string{"total", "page", "tutors"}, Dialect: paging.DialectNone,
		Sample: service.TutorListDTO{}},
	{Result: "service.WrongQuestionPageDTO", Endpoints: []string{"GET /wrong-questions"},
		Keys: []string{"items", "page", "page_size", "total"}, Dialect: paging.DialectPageSize,
		Sample: service.WrongQuestionPageDTO{}},
}

// totalPayloadRegistry 含 total + 切片字段、但不是分页信封的结果类型（理由逐条登记）。
var totalPayloadRegistry = []PayloadSpec{
	{Result: "service.CheckInCalendarResult",
		Reason: "打卡日历：days 是整月逐日数组、total 是累计打卡天数，不是列表页"},
	{Result: "service.GenTaskStatus",
		Reason: "内容生成任务进度：results 是章节生成结果、total/completed 是任务进度，不是列表页"},
	{Result: "service.PracticeStartResultDTO",
		Reason: "练习会话载荷：questions 是本次会话题集、total/completed 是会话进度，不是列表页"},
	{Result: "service.SearchSectionDTO",
		Reason: "搜索分区片段：由 SearchPageDTO 信封承载，分区自身不分页（无 page 元数据、无独立端点）"},
}

// Envelopes 返回信封登记表副本（调用方改不动登记表）。
func Envelopes() []EnvelopeSpec {
	out := make([]EnvelopeSpec, len(envelopeRegistry))
	copy(out, envelopeRegistry)
	return out
}

// Payloads 返回「含 total 的非信封载荷」登记副本。
func Payloads() []PayloadSpec {
	out := make([]PayloadSpec, len(totalPayloadRegistry))
	copy(out, totalPayloadRegistry)
	return out
}
