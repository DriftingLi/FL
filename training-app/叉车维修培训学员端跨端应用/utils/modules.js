/**
 * 模块声明 —— 「一个模块是什么」的**唯一事实源**（ADR-0023 票 A / epic #1221）
 *
 * 这个文件**只放数据**：零逻辑、零断言、不读磁盘、不认识 jest。
 * 读取与对账机制在 `utils/contractHarness.js`；豁免名单的单点在 `utils/guardAllowlist.js`。
 * 为什么分家：合成一份会把「改了模块边界」与「改了检查逻辑」混进同一个 diff（ADR-0023 ②①）。
 *
 * ## 键与归属面
 *
 * - **键 = `pages/` 下的目录名**（23 个，一对一是默认规则）。
 * - `extraDirs` / `files` 里的目录外路径 = 「目录外的家」：域 api、共享组件与共享 composable。
 * - **归属规则（共享件归主消费方，ADR-0023 ③）**，按优先级：
 *   ① 该域有既有模块契约（`utils/*Contract.test.js`）⇒ 归该契约模块（例：`api/featured.uts` → `dashboard`）；
 *   ② 只有一个模块消费 ⇒ 归它（例：`api/job.uts` → `jobs`）；
 *   ③ 多消费且无契约 ⇒ 归「页面所在模块」并在行内写理由（例：`api/checkin.uts` → `forum`）。
 *   **多消费的其余模块登记在** `crossModuleConsumers`（消费者的登记面，不是所有权面）。
 * - 跨切面基础设施（`api/request.uts` / `api/auth.uts` / `api/helpers.uts` / `api/refreshGate.uts`）
 *   **不属于任何模块**，故不在本表内 —— 归属面只管「模块是什么」，不管全仓基础设施（ADR-0023 附录口径）。
 *
 * ## 三个口径（读本表前必须先知道，否则数字对不上）
 *
 * 1. **行数 = 总行数**（`readText(f).split('\n').length`），与既有模块契约的 600 预算一致。
 *    ⚠️ **不是**「非空行数」（PowerShell `Measure-Object -Line` 的口径）。两者在本仓差约 6%：
 *    `api/forum.uts` 总行 **654** / 非空 **618** —— ADR-0023 附录用的是非空行数，故其数字**小于**执法口径。
 * 2. **`budget` 是「单文件行数上限」**，不是模块总行数；`BUDGET = 600` 沿用 ADR-0007 软预算。
 * 3. **`budget: 'pending'` = 登记不执法**（ADR-0023 ②⑥）：未达标模块照样入表，让进度变成表上一列，
 *    但不参与预算判红。本票（票 A）**不执法**，故 pending 是「现状照实登记」，不是豁免或买绿。
 *
 * ## 改这个文件的纪律
 *
 * - 在模块目录里**新增文件**必须同步登记（否则对账判红，信息里直接指出该加哪一行）。
 * - 不得为已达标模块写 `budgetOverrides` 来买绿；每条覆盖必须带 `reason` + `issue`（缺一即登记非法）。
 * - 模块边界是显式决定；不得靠放宽对账消红（ADR-0023 ⑤）。
 */

/** 单文件行数软预算（ADR-0007 / ADR-0023 ②⑤）。口径见文件头「三个口径」第 1、2 条。 */
const BUDGET = 600;

/** 模块目录内最大相对层数：`<模块>/<文件>` = 1，`<模块>/<分组>/<文件>` = 2（不允许更深）。 */
const MAX_DEPTH = 2;

/**
 * 23 个 `pages/` 模块的声明。
 * 字段顺序统一为：extraDirs → files → extractDirs → crossModuleConsumers → budget → budgetOverrides → allowlistOwned → maxDepth。
 */
const MODULES = {
  'ai-assistant': {
    /** `components/ai-chat/**` 是 AI 助手的私有拆出物，但它住在 `components/` 而非 `pages/ai-assistant/` */
    extraDirs: ['components/ai-chat'],
    files: [
      'api/aiAssistant.uts',
      'components/ai-chat/ai-chat-bubble.uvue',
      'components/ai-chat/ai-chat-custom-form.uvue',
      'components/ai-chat/ai-chat-drawer-left.uvue',
      'components/ai-chat/ai-chat-drawer-right.uvue',
      'components/ai-chat/ai-chat-input.uvue',
      'components/ai-chat/ai-chat-model-picker.uvue',
      'components/ai-chat/ai-chat-nav.uvue',
      'components/ai-chat/ai-chat-pro-sheet.uvue',
      'components/ai-chat/ai-chat-sources.uvue',
      'composables/useAiChat.uts',
      'composables/useAiPro.uts',
      'pages/ai-assistant/ai-assistant-constants.uts',
      'pages/ai-assistant/ai-assistant.uvue',
      'pages/ai-assistant/ai-feature.uvue',
      'pages/ai-assistant/ai-settings.uvue',
      'pages/ai-assistant/custom-models.uvue',
    ],
    extractDirs: ['components/ai-chat'],
    /**
     * ADR-0023 ③ 记 `search` 为 `components/ai-chat/**` 的跨模块消费者；
     * 实测（本文件落笔时）`pages/search/search.uvue` 只 import `components/app-chip` 与
     * `components/app-empty-state`，**未引用 ai-chat 任何文件** ⇒ 登记面照实测写 `[]`。
     * 决策意图由对账兜底：`search` 真的接线那天，`consumerFacts().unregistered` 会判红并要求登记
     * （不拿一条假声明去保真）。
     */
    crossModuleConsumers: [],
    /** 超预算：ai-assistant.uvue 839 / ai-feature 629 / ai-settings 621 / api/aiAssistant.uts 667 */
    budget: 'pending',
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  courses: {
    extraDirs: [],
    files: [
      'api/course.uts',
      'pages/courses/chapter-view.uvue',
      'pages/courses/components/chapter-file-list.uvue',
      'pages/courses/components/chapter-markdown.uvue',
      'pages/courses/components/chapter-nav.uvue',
      'pages/courses/components/course-chapter-list.uvue',
      'pages/courses/components/course-cover-section.uvue',
      'pages/courses/components/course-info-grid.uvue',
      'pages/courses/components/course-progress-section.uvue',
      'pages/courses/composables/useChapterStudy.uts',
      'pages/courses/course-detail.uvue',
      'pages/courses/courses.uvue',
    ],
    extractDirs: ['pages/courses/components', 'pages/courses/composables'],
    crossModuleConsumers: ['jobs', 'mall', 'practice'],
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  dashboard: {
    extraDirs: [],
    files: [
      'api/credential.uts',
      'api/featured.uts',
      'api/notification.uts',
      'pages/dashboard/components/dashboard-cert-dropdown.uvue',
      'pages/dashboard/components/dashboard-continue-card.uvue',
      'pages/dashboard/components/dashboard-course-section.uvue',
      'pages/dashboard/components/dashboard-menu-grid.uvue',
      'pages/dashboard/components/dashboard-news-section.uvue',
      'pages/dashboard/composables/use-dashboard-credential.uts',
      'pages/dashboard/composables/use-dashboard-feeds.uts',
      'pages/dashboard/dashboard.uvue',
    ],
    extractDirs: ['pages/dashboard/components', 'pages/dashboard/composables'],
    crossModuleConsumers: ['courses', 'featured', 'forum', 'mall', 'notifications', 'practice', 'profile', 'search'],
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  exam: {
    extraDirs: [],
    files: [
      'api/mockExam.uts',
      'pages/exam/components/exam-action-bar.uvue',
      'pages/exam/components/exam-question-card.uvue',
      'pages/exam/components/exam-result-detail-list.uvue',
      'pages/exam/components/exam-result-summary.uvue',
      'pages/exam/composables/useMockExamResult.uts',
      'pages/exam/composables/useMockExamSession.uts',
      'pages/exam/mock-exam-result.uvue',
      'pages/exam/mock-exam.uvue',
    ],
    extractDirs: ['pages/exam/components', 'pages/exam/composables'],
    crossModuleConsumers: ['profile'],
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  'exam-info': {
    extraDirs: [],
    files: ['pages/exam-info/exam-info.uvue'],
    extractDirs: [],
    crossModuleConsumers: [],
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  featured: {
    extraDirs: [],
    files: ['pages/featured/featured-detail.uvue', 'pages/featured/featured-list.uvue'],
    extractDirs: [],
    /** 消费 `dashboard` 的 `api/featured.uts`（登记在 dashboard 的消费者面） */
    crossModuleConsumers: [],
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  'forgot-password': {
    extraDirs: [],
    files: ['pages/forgot-password/forgot-password.uvue'],
    extractDirs: [],
    crossModuleConsumers: [],
    /** 超预算：forgot-password.uvue 676 */
    budget: 'pending',
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  forum: {
    extraDirs: [],
    files: [
      'api/checkin.uts',
      'api/forum.uts',
      'composables/useReplyComposer.uts',
      'composables/useReport.uts',
      'composables/useResourcePoints.uts',
      'composables/useTopicDetail.uts',
      'composables/useTopicFeed.uts',
      'pages/forum/check-in.uvue',
      'pages/forum/components/forum-checkin-card.uvue',
      'pages/forum/components/forum-contribution-form.uvue',
      'pages/forum/components/forum-experience-sort-bar.uvue',
      'pages/forum/components/forum-featured-filter.uvue',
      'pages/forum/components/forum-qa-header.uvue',
      'pages/forum/components/forum-reply-list.uvue',
      'pages/forum/components/forum-report-dialog.uvue',
      'pages/forum/components/forum-resource-panel.uvue',
      'pages/forum/components/forum-square-sort-bar.uvue',
      'pages/forum/components/forum-tab-bar.uvue',
      'pages/forum/components/forum-topic-body.uvue',
      'pages/forum/components/forum-topic-card.uvue',
      'pages/forum/components/forum-topic-header.uvue',
      'pages/forum/forum-create.uvue',
      'pages/forum/forum-detail.uvue',
      'pages/forum/forum.uvue',
      'pages/forum/my-forum.uvue',
    ],
    extractDirs: ['pages/forum/components'],
    crossModuleConsumers: ['profile'],
    /**
     * 超预算：`api/forum.uts` 654（唯一一处**活违例**，ADR-0023 的实证；票 B 收口到 ≤600 后本行改 `BUDGET`）。
     * 注意 `pages/forum/**` 自身已全部达标（最大 583）—— 这正是「预算不覆盖域 api」的失效形态。
     */
    budget: 'pending',
    budgetOverrides: {},
    /** 规则 H 的存量豁免；归属理由：`pages/forum/check-in.uvue` 与它同属本模块（另一个消费者是 profile） */
    allowlistOwned: ['api/checkin.uts'],
    maxDepth: MAX_DEPTH,
  },

  guide: {
    extraDirs: [],
    files: ['pages/guide/choose-cert.uvue'],
    extractDirs: [],
    crossModuleConsumers: [],
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  index: {
    extraDirs: [],
    files: ['pages/index/index.uvue'],
    extractDirs: [],
    crossModuleConsumers: [],
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  jobs: {
    extraDirs: [],
    files: ['api/job.uts', 'pages/jobs/job-detail.uvue', 'pages/jobs/job-list.uvue'],
    extractDirs: [],
    /** 消费 `courses` 的 `api/course.uts`（登记在 courses 的消费者面） */
    crossModuleConsumers: [],
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  login: {
    extraDirs: [],
    files: ['composables/useBiometric.uts', 'pages/login/login.uvue'],
    extractDirs: [],
    crossModuleConsumers: [],
    /** 超预算：login.uvue 977 */
    budget: 'pending',
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  mall: {
    extraDirs: [],
    files: [
      'pages/mall/components/mall-category-sidebar.uvue',
      'pages/mall/components/mall-float-actions.uvue',
      'pages/mall/components/mall-sort-bar.uvue',
      'pages/mall/mall.uvue',
    ],
    extractDirs: ['pages/mall/components'],
    /** 消费 `courses` 的 `api/course.uts` / `dashboard` 的 `api/credential.uts`（登记在各自所有者） */
    crossModuleConsumers: [],
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  notifications: {
    extraDirs: [],
    files: ['pages/notifications/notifications.uvue'],
    extractDirs: [],
    /** 消费 `dashboard` 的 `api/notification.uts`（登记在 dashboard 的消费者面） */
    crossModuleConsumers: [],
    budget: BUDGET,
    budgetOverrides: {},
    /** 规则 H 的存量豁免（本模块唯一的页） */
    allowlistOwned: ['pages/notifications/notifications.uvue'],
    maxDepth: MAX_DEPTH,
  },

  points: {
    extraDirs: [],
    files: ['api/points.uts', 'pages/points/points-detail.uvue', 'pages/points/task-center.uvue'],
    extractDirs: [],
    crossModuleConsumers: ['profile'],
    /** 超预算：task-center.uvue 617 */
    budget: 'pending',
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  practice: {
    extraDirs: [],
    files: [
      'api/practice.uts',
      'api/questionInteraction.uts',
      'pages/practice/components/practice-banner-card.uvue',
      'pages/practice/components/practice-feature-grid.uvue',
      'pages/practice/components/practice-module-list.uvue',
      'pages/practice/components/practice-stats-bar.uvue',
      'pages/practice/composables/usePracticeComments.uts',
      'pages/practice/composables/usePracticeNotes.uts',
      'pages/practice/composables/usePracticeOverview.uts',
      'pages/practice/composables/usePracticeSession.uts',
      'pages/practice/practice-do.uvue',
      'pages/practice/practice-report.uvue',
      'pages/practice/practice.uvue',
    ],
    extractDirs: ['pages/practice/components', 'pages/practice/composables'],
    crossModuleConsumers: ['profile'],
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  profile: {
    extraDirs: [],
    files: [
      'api/favorite.uts',
      'api/student.uts',
      'api/wrongQuestion.uts',
      'pages/profile/components/activity-tab-bar.uvue',
      'pages/profile/components/activity-topic-card.uvue',
      'pages/profile/components/activity-user-card.uvue',
      'pages/profile/components/info-dialog.uvue',
      'pages/profile/components/profile-user-row.uvue',
      'pages/profile/components/wrong-filter-bar.uvue',
      'pages/profile/components/wrong-question-card.uvue',
      'pages/profile/components/wrong-stats-card.uvue',
      'pages/profile/composables/personal-info-flows.uts',
      'pages/profile/credential-switch.uvue',
      'pages/profile/favorites.uvue',
      'pages/profile/help-center.uvue',
      'pages/profile/mock-exam-records.uvue',
      'pages/profile/notebook.uvue',
      'pages/profile/personal-activity.uvue',
      'pages/profile/personal-info.uvue',
      'pages/profile/practice-records.uvue',
      'pages/profile/profile.uvue',
      'pages/profile/records.uvue',
      'pages/profile/settings.uvue',
      'pages/profile/wrong-questions.uvue',
    ],
    extractDirs: ['pages/profile/components', 'pages/profile/composables'],
    crossModuleConsumers: ['courses', 'dashboard', 'featured', 'practice', 'resources', 'resume'],
    /** 达标但贴近上限：`pages/profile/wrong-questions.uvue` 恰 600 行（先例 profileContract 的落袋锁） */
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  'profile-setup': {
    extraDirs: [],
    files: ['pages/profile-setup/profile-setup.uvue'],
    extractDirs: [],
    crossModuleConsumers: [],
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  recruiter: {
    extraDirs: [],
    files: [
      'api/recruit.uts',
      'pages/recruiter/applications.uvue',
      'pages/recruiter/components/recruiter-filter-drawer.uvue',
      'pages/recruiter/components/recruiter-tab-bar.uvue',
      'pages/recruiter/contacts.uvue',
      'pages/recruiter/jobs.uvue',
      'pages/recruiter/login.uvue',
      'pages/recruiter/me.uvue',
      'pages/recruiter/resume-detail.uvue',
      'pages/recruiter/resumes.uvue',
    ],
    extractDirs: ['pages/recruiter/components'],
    crossModuleConsumers: [],
    /** 超预算：resume-detail.uvue 702 / api/recruit.uts 675 */
    budget: 'pending',
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  register: {
    extraDirs: [],
    files: ['pages/register/register.uvue'],
    extractDirs: [],
    crossModuleConsumers: [],
    /** 超预算：register.uvue 723 */
    budget: 'pending',
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  resources: {
    extraDirs: [],
    files: [
      'api/contribution.uts',
      'api/material.uts',
      'pages/resources/materials.uvue',
      'pages/resources/my-purchases.uvue',
      'pages/resources/my-uploads.uvue',
    ],
    extractDirs: [],
    crossModuleConsumers: ['forum'],
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  resume: {
    extraDirs: [],
    files: [
      'api/resume.uts',
      'pages/resume/components/resume-progress-card.uvue',
      'pages/resume/composables/useResumeEdit.uts',
      'pages/resume/resume-attach.uvue',
      'pages/resume/resume-edit.uvue',
      'pages/resume/resume.uvue',
    ],
    extractDirs: ['pages/resume/components', 'pages/resume/composables'],
    /** 消费 `profile` 的 `api/student.uts` / `api/favorite.uts`（登记在 profile 的消费者面） */
    crossModuleConsumers: [],
    /** T09 手术（#647）：resume-edit 922→460、模块 7 文件全 ≤600 ⇒ 执法面随之上线（原为「超预算 923」的 pending） */
    budget: BUDGET,
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },

  search: {
    extraDirs: [],
    files: ['api/search.uts', 'pages/search/search.uvue'],
    extractDirs: [],
    /** 消费 `dashboard` 的 `api/credential.uts`（登记在 dashboard 的消费者面） */
    crossModuleConsumers: [],
    /** 超预算：search.uvue 702 */
    budget: 'pending',
    budgetOverrides: {},
    allowlistOwned: [],
    maxDepth: MAX_DEPTH,
  },
};

module.exports = { BUDGET, MAX_DEPTH, MODULES };
