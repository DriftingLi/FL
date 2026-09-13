// Package credentialscope 「按证件重装」例外登记表（ADR-0047 §4 收尾 / spec #940 片六）。
//
// 背景：useAsyncPage 默认随当前证件切换重装（credentialScoped 缺省 true），少数域显式传
// false 关闭重装。这些例外此前只散在调用点（有的带注释、有的只留一句 #604 opt-out），
// 没有任何地方能回答「全站有哪些例外、各自为什么」。
//
// 本包把例外收成一张**声明表**并生成前端常量 frontend/src/config/credentialScope.ts；
// **运行期判据不变**（调用点仍传字面量，零行为变化）—— 本表只把「有哪些例外」变成可核对
// 的事实，并由 codegen_test.go 的覆盖锁扫描前端源码强制登记：新增一处未登记的 opt-out 即报红。
package credentialscope

// OptOut 一处「不随当前证件切换重装」的消费者。
type OptOut struct {
	// File 相对 frontend/src 的路径（覆盖锁按文件比对，不按行号 —— 行号会随无关编辑漂移）。
	File string
	// Reason 为什么它不随当前证件过滤：评审只看这一句，务必写「域理由」而不是「历史如此」。
	Reason string
}

// Marker 运行期判据的字面量（覆盖锁扫描它出现的位置）。
const Marker = "credentialScoped: false"

// DefinitionFile 判据的定义处：useAsyncPage 的文档注释里也会出现 Marker，但它不是消费者。
const DefinitionFile = "composables/useAsyncPage.ts"

// GeneratedFile 生成物自身：它的头部注释里引用了 Marker（说明这条锁在守什么），也不是消费者。
const GeneratedFile = "config/credentialScope.ts"

// OptOuts 例外登记表（渲染时按 File 排序，声明序不影响生成物）。
//
// 口径：只有**域本身不受当前证件分区**时才登记 —— 论坛域（帖子/回复/章节讨论）、招聘域
// （职位/投递）、积分域（流水/任务中心/打卡）、以及「证件目录」本身（它装载的就是证件清单，
// 自然不随当前证件变化）。任何「因为懒得重装」而加进来的条目都应被评审打回。
var OptOuts = []OptOut{
	{File: "components/student/ChapterDiscussion.vue", Reason: "章节讨论属论坛域，帖子不按证件分区（#604 opt-out）"},
	{File: "pages/onboarding/CredentialOnboarding.vue", Reason: "本页装载的就是证件目录，不随当前证件变化（且页面只存在于未选证件时）"},
	{File: "pages/student/ChapterView.vue", Reason: "章节页内嵌论坛域讨论区，随章节而非随证件装载（#604 opt-out）"},
	{File: "pages/student/CheckInPage.vue", Reason: "打卡不按当前证件分区（ADR-0028 独立蓝图）"},
	{File: "pages/student/ForumDetail.vue", Reason: "论坛域不受证件过滤（#604 opt-out）"},
	{File: "pages/student/ForumPage.vue", Reason: "论坛域不受证件过滤（#604 opt-out）"},
	{File: "pages/student/JobDetail.vue", Reason: "招聘域不受证件过滤（#604 opt-out）"},
	{File: "pages/student/JobPlaza.vue", Reason: "招聘域职位广场不受证件过滤（#604 opt-out）"},
	{File: "pages/student/MyApplications.vue", Reason: "招聘域投递记录不受证件过滤（#604 opt-out）"},
	{File: "pages/student/PointsLedger.vue", Reason: "积分流水不按当前证件分区，切换后不重置页码（#604 opt-out）"},
	{File: "pages/student/TaskCenter.vue", Reason: "积分任务中心不按当前证件分区（#604 opt-out）"},
}
