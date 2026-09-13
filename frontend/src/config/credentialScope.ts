// 生成文件，勿手改（ADR-0047 §4 收尾 / spec #940 片六）。
// 唯一事实源：backend/internal/credentialscope/registry.go。
// 再生成：cd backend && go run ./cmd/gen-credscope
// 同步契约：backend/internal/credentialscope/codegen_test.go（字节全等 + 覆盖锁）。
//
// 「不随当前证件切换重装」的消费者登记表：前端源码里每一处 credentialScoped: false
// 都必须在下表出现 —— 覆盖锁扫前端源码比对，新增例外不登记即报红。
//
// 运行期判据本身**不变**（调用点仍传字面量，零行为变化）：本文件不是运行时开关，
// 而是「有哪些例外、各自为什么」的唯一可核对答案。判据定义处见
// composables/useAsyncPage.ts（它自己的文档注释里也会出现该字面量，不计入消费者）。

export interface CredentialScopeOptOut {
  /** 相对 frontend/src 的路径。 */
  readonly file: string
  /** 为什么它不随当前证件过滤（域理由，评审看这一句）。 */
  readonly reason: string
}

/** 例外登记表（按路径排序，生成序稳定）。 */
export const CREDENTIAL_SCOPE_OPT_OUTS: readonly CredentialScopeOptOut[] = [
  { file: 'components/student/ChapterDiscussion.vue', reason: '章节讨论属论坛域，帖子不按证件分区（#604 opt-out）' },
  { file: 'pages/onboarding/CredentialOnboarding.vue', reason: '本页装载的就是证件目录，不随当前证件变化（且页面只存在于未选证件时）' },
  { file: 'pages/student/ChapterView.vue', reason: '章节页内嵌论坛域讨论区，随章节而非随证件装载（#604 opt-out）' },
  { file: 'pages/student/CheckInPage.vue', reason: '打卡不按当前证件分区（ADR-0028 独立蓝图）' },
  { file: 'pages/student/ForumDetail.vue', reason: '论坛域不受证件过滤（#604 opt-out）' },
  { file: 'pages/student/ForumPage.vue', reason: '论坛域不受证件过滤（#604 opt-out）' },
  { file: 'pages/student/JobDetail.vue', reason: '招聘域不受证件过滤（#604 opt-out）' },
  { file: 'pages/student/JobPlaza.vue', reason: '招聘域职位广场不受证件过滤（#604 opt-out）' },
  { file: 'pages/student/MyApplications.vue', reason: '招聘域投递记录不受证件过滤（#604 opt-out）' },
  { file: 'pages/student/PointsLedger.vue', reason: '积分流水不按当前证件分区，切换后不重置页码（#604 opt-out）' },
  { file: 'pages/student/TaskCenter.vue', reason: '积分任务中心不按当前证件分区（#604 opt-out）' },
]
