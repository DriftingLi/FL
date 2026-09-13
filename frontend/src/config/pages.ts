// 页面描述符（ADR-0047 §2 / spec #930）：**一个页面的唯一事实源**。
//
// 一张描述符同时回答四件事——路由记录（path / component / meta）、导航项（分组 / 标签 /
// 图标 / 详情页归属）、可见性判据（capability，来自后端能力表的 codegen）、工作区（meta.workspace）。
// `router/index.ts` 与 `config/navigation.ts` 都从本文件派生，二者不再各自维护清单；
// `config/__tests__/pages.spec.ts` 断言「描述符 ↔ 路由 ↔ 导航」三者互等。
//
// 约定：
// - `path` 一律写**绝对路径**，派生子路由时按 layout 的 basePath 去掉前缀；
// - `layout` 缺省 = 顶层记录（无布局外壳，如认证页与 AI 助手）；
// - `requiresAuth` 缺省 true（与既有 router 逐字一致：公开页显式写 false）；
// - `capability` 是**可见性判据**（角色资格），数据级不变式（所有权/证件作用域/状态前置）不进这里。
import type { Component } from 'vue'
import { routeNames, type RouteName } from './routeNames'
import type { AuthzCapability } from './authz'
import {
  HomeFilled,
  Notebook,
  EditPen,
  Document,
  CircleCloseFilled,
  MagicStick,
  Search,
  Files,
  Star,
  DataAnalysis,
  User,
  TrendCharts,
  UserFilled,
  ChatDotRound,
  PriceTag,
  Setting,
  Memo,
  CircleCheck,
  FolderOpened,
  CollectionTag,
  Trophy,
  Calendar,
  OfficeBuilding
} from '@element-plus/icons-vue'

/** 布局键：一个页面最多属于一个布局外壳。 */
export type LayoutKey = 'training' | 'tutor' | 'manage' | 'recruit' | 'valuation'

/** 工作区（meta.workspace）：守卫据此判定子域名/回跳目标；与布局不必一一对应。 */
export type Workspace = 'auth' | 'training' | 'tutor' | 'manage' | 'recruit' | 'valuation'

/** 导航项描述（分组由 navGroups 提供标签与图标；parent 表达二级分组）。 */
export interface PageNav {
  /** 侧栏分组 key（见 navGroups） */
  group: string
  label: string
  icon?: Component
  /** 除本页 routeName 外还应高亮的路由（详情页归属） */
  activeRouteNames?: RouteName[]
  routeParams?: Record<string, string | number>
  /** 外链（跨子域）导航项：无路由，只有地址 */
  externalUrl?: string
  /** exact=true 时仅精确匹配 route.name 才高亮（兼容既有语义） */
  exact?: boolean
  /** 二级分组：父导航项的路由名（如真题练习挂题库练习之下） */
  parent?: RouteName
  /** 同级排序（缺省按描述符声明序） */
  order?: number
}

/** 页面描述符。 */
export interface PageDescriptor {
  name: RouteName
  /** 绝对路径（如 /training/forum/:topicId） */
  path: string
  component: () => Promise<unknown>
  /** 缺省 = 顶层记录 */
  layout?: LayoutKey
  workspace: Workspace
  /** 缺省 true */
  requiresAuth?: boolean
  /** 认证页标记（认证页外壳用） */
  authPage?: boolean
  /** 估值模块独立认证页标记 */
  isValuationAuthPage?: boolean
  /** 兼容存量 meta.roles（估值历史页） */
  roles?: string[]
  /** 可见性判据：角色需要拥有的能力 */
  capability?: AuthzCapability
  /** 出现在侧栏时填写 */
  nav?: PageNav
}

/** 布局外壳定义（basePath 用于派生子路由的路径前缀）。 */
export interface LayoutDef {
  key: LayoutKey
  basePath: string
  component: () => Promise<unknown>
  workspace: Workspace
  requiresAuth: boolean
  /** 布局级角色约束（缺省 = 任意已登录角色） */
  role?: string
}

// ===== 布局外壳（顺序即注册顺序）=====
export const layouts: LayoutDef[] = [
  { key: 'training', basePath: '/training', component: () => import('@/layouts/TrainingLayout.vue'), workspace: 'training', requiresAuth: true, role: 'hrwai_user' },
  { key: 'tutor', basePath: '/training/tutor', component: () => import('@/layouts/TutorLayout.vue'), workspace: 'tutor', requiresAuth: true, role: 'tutor' },
  { key: 'manage', basePath: '/admin', component: () => import('@/layouts/AdminLayout.vue'), workspace: 'manage', requiresAuth: true, role: 'admin' },
  { key: 'recruit', basePath: '/recruit', component: () => import('@/layouts/RecruitLayout.vue'), workspace: 'recruit', requiresAuth: true, role: 'recruiter' },
  { key: 'valuation', basePath: '/valuation', component: () => import('@/layouts/ValuationLayout.vue'), workspace: 'valuation', requiresAuth: false },
]

/** 侧栏分组（按工作区分表；顺序即渲染顺序）。 */
export interface NavGroup {
  key: string
  label: string
  icon?: Component
}

export const navGroups: Record<string, NavGroup[]> = {
  training: [
    { key: 'learning', label: '学习中心', icon: HomeFilled },
    { key: 'exam', label: '题库与考试', icon: EditPen },
    { key: 'interactive', label: '互动与工具', icon: ChatDotRound },
    { key: 'personal', label: '个人', icon: User }
  ],
  manage: [
    { key: 'overview', label: '总览', icon: DataAnalysis },
    { key: 'user-content', label: '用户与内容', icon: User },
    { key: 'teaching', label: '教学管理', icon: FolderOpened },
    { key: 'system', label: '系统', icon: Setting }
  ]
}

// ===== 页面描述符表（按工作区成组，组内声明序即导航序）=====
export const pages: PageDescriptor[] = [
  // ---------- 认证页（顶层，无布局）----------
  { name: routeNames.Login, path: '/login', component: () => import('@/pages/auth/Login.vue'), workspace: 'auth', requiresAuth: false, authPage: true },
  { name: routeNames.Register, path: '/register', component: () => import('@/pages/auth/Register.vue'), workspace: 'auth', requiresAuth: false, authPage: true },
  { name: routeNames.ForgotPassword, path: '/forgot-password', component: () => import('@/pages/auth/ForgotPassword.vue'), workspace: 'auth', requiresAuth: false, authPage: true },

  // ---------- 学员工作区（TrainingLayout）----------
  { name: routeNames.StudentDashboard, path: '/training', component: () => import('@/pages/student/Dashboard.vue'), layout: 'training', workspace: 'training', capability: 'student.access', nav: { group: 'learning', label: '仪表盘', icon: HomeFilled, exact: true, order: 1 } },
  { name: routeNames.CourseList, path: '/training/courses', component: () => import('@/pages/student/CourseList.vue'), layout: 'training', workspace: 'training', capability: 'course.learn', nav: { group: 'learning', label: '课程中心', icon: Notebook, activeRouteNames: [routeNames.ChapterView], order: 2 } },
  { name: routeNames.StudentSearch, path: '/training/search', component: () => import('@/pages/student/SearchPage.vue'), layout: 'training', workspace: 'training', capability: 'search.use', nav: { group: 'learning', label: '全局搜索', icon: Search, order: 4 } },
  { name: routeNames.StudentMaterials, path: '/training/materials', component: () => import('@/pages/student/Materials.vue'), layout: 'training', workspace: 'training', capability: 'material.read', nav: { group: 'learning', label: '学习资料', icon: Files, order: 3 } },
  { name: routeNames.StudentFavorites, path: '/training/favorites', component: () => import('@/pages/student/Favorites.vue'), layout: 'training', workspace: 'training', capability: 'favorite.manage', nav: { group: 'personal', label: '我的收藏', icon: Star, order: 3 } },
  { name: routeNames.ForumPage, path: '/training/forum', component: () => import('@/pages/student/ForumPage.vue'), layout: 'training', workspace: 'training', capability: 'forum.participate', nav: { group: 'interactive', label: '学员论坛', icon: ChatDotRound, activeRouteNames: [routeNames.ForumDetail], order: 1 } },
  { name: routeNames.ForumAsk, path: '/training/forum/ask', component: () => import('@/pages/student/ForumAskPage.vue'), layout: 'training', workspace: 'training', capability: 'forum.participate' },
  { name: routeNames.ForumDetail, path: '/training/forum/:topicId', component: () => import('@/pages/student/ForumDetail.vue'), layout: 'training', workspace: 'training', capability: 'forum.participate' },
  { name: routeNames.LinkOut, path: '/training/link-out', component: () => import('@/pages/student/LinkOutPage.vue'), layout: 'training', workspace: 'training' },
  { name: routeNames.ChapterView, path: '/training/course/:courseId/chapter/:chapterId', component: () => import('@/pages/student/ChapterView.vue'), layout: 'training', workspace: 'training', capability: 'course.learn' },
  { name: routeNames.QuestionBank, path: '/training/question-bank', component: () => import('@/pages/student/QuestionBank.vue'), layout: 'training', workspace: 'training', capability: 'question.practice', nav: { group: 'exam', label: '题库练习', icon: EditPen, order: 1 } },
  { name: routeNames.MockExam, path: '/training/mock-exam', component: () => import('@/pages/student/MockExam.vue'), layout: 'training', workspace: 'training', capability: 'mock_exam.take', nav: { group: 'exam', label: '模拟考试', icon: Document, order: 2 } },
  { name: routeNames.WrongQuestions, path: '/training/wrong-questions', component: () => import('@/pages/student/WrongQuestions.vue'), layout: 'training', workspace: 'training', capability: 'question.practice', nav: { group: 'exam', label: '错题本', icon: CircleCloseFilled, order: 3 } },
  { name: routeNames.RealExamPapers, path: '/training/real-exam', component: () => import('@/pages/student/RealExamPapers.vue'), layout: 'training', workspace: 'training', capability: 'real_exam.take', nav: { group: 'exam', label: '真题练习', icon: Document, parent: routeNames.QuestionBank } },
  { name: routeNames.RealExamPractice, path: '/training/real-exam/practice/:paperId', component: () => import('@/pages/student/RealExamPractice.vue'), layout: 'training', workspace: 'training', capability: 'real_exam.take' },
  { name: routeNames.CheckIn, path: '/training/check-in', component: () => import('@/pages/student/CheckInPage.vue'), layout: 'training', workspace: 'training', capability: 'check_in.use', nav: { group: 'personal', label: '每日打卡', icon: Calendar, order: 2 } },
  { name: routeNames.TaskCenter, path: '/training/task-center', component: () => import('@/pages/student/TaskCenter.vue'), layout: 'training', workspace: 'training', capability: 'points.use', nav: { group: 'personal', label: '任务中心', icon: Trophy, order: 1 } },
  { name: routeNames.PointsLedger, path: '/training/task-center/points', component: () => import('@/pages/student/PointsLedger.vue'), layout: 'training', workspace: 'training', capability: 'points.use' },
  { name: routeNames.StudentProfile, path: '/training/profile', component: () => import('@/pages/student/Profile.vue'), layout: 'training', workspace: 'training', capability: 'student.access', nav: { group: 'personal', label: '个人资料', icon: User, order: 4 } },
  { name: routeNames.StudentResume, path: '/training/resume', component: () => import('@/pages/student/ResumePage.vue'), layout: 'training', workspace: 'training', capability: 'resume.manage', nav: { group: 'personal', label: '我的简历', icon: Document, order: 5 } },
  { name: routeNames.StudentResumeEdit, path: '/training/resume/edit', component: () => import('@/pages/student/ResumeEdit.vue'), layout: 'training', workspace: 'training', capability: 'resume.manage' },
  { name: routeNames.JobPlaza, path: '/training/jobs', component: () => import('@/pages/student/JobPlaza.vue'), layout: 'training', workspace: 'training', capability: 'job.apply', nav: { group: 'personal', label: '职位广场', icon: OfficeBuilding, activeRouteNames: [routeNames.JobDetail], order: 6 } },
  { name: routeNames.JobDetail, path: '/training/jobs/:id', component: () => import('@/pages/student/JobDetail.vue'), layout: 'training', workspace: 'training', capability: 'job.apply' },
  { name: routeNames.MyApplications, path: '/training/applications', component: () => import('@/pages/student/MyApplications.vue'), layout: 'training', workspace: 'training', capability: 'job.apply', nav: { group: 'personal', label: '我的投递', icon: Document, order: 7 } },
  { name: routeNames.CredentialOnboarding, path: '/training/onboarding/credential', component: () => import('@/pages/onboarding/CredentialOnboarding.vue'), layout: 'training', workspace: 'training', capability: 'student.access' },

  // ---------- 导师工作区（TutorLayout）----------
  { name: routeNames.TutorDashboard, path: '/training/tutor', component: () => import('@/pages/tutor/Dashboard.vue'), layout: 'tutor', workspace: 'tutor', capability: 'tutor.access', nav: { group: 'tutor', label: '仪表盘', icon: HomeFilled, exact: true, order: 1 } },
  { name: routeNames.TutorCourses, path: '/training/tutor/courses', component: () => import('@/pages/tutor/TutorCourses.vue'), layout: 'tutor', workspace: 'tutor', capability: 'tutor.access', nav: { group: 'tutor', label: '我的课程', icon: Notebook, activeRouteNames: [routeNames.TutorChapterManage, routeNames.TutorChapterEdit], order: 2 } },
  { name: routeNames.TutorChapterManage, path: '/training/tutor/course/:id/chapters', component: () => import('@/pages/tutor/ChapterManage.vue'), layout: 'tutor', workspace: 'tutor', capability: 'tutor.access' },
  { name: routeNames.TutorChapterEdit, path: '/training/tutor/course/:courseId/chapter/:chapterId', component: () => import('@/pages/tutor/TutorChapterEdit.vue'), layout: 'tutor', workspace: 'tutor', capability: 'tutor.access' },
  { name: routeNames.TutorQuestionManage, path: '/training/tutor/question-manage', component: () => import('@/pages/tutor/QuestionManage.vue'), layout: 'tutor', workspace: 'tutor', capability: 'question.author', nav: { group: 'tutor', label: '题库管理', icon: EditPen, activeRouteNames: [routeNames.TutorQuestionCreate, routeNames.TutorQuestionTags], order: 3 } },
  { name: routeNames.TutorQuestionCreate, path: '/training/tutor/question-create', component: () => import('@/pages/tutor/QuestionCreate.vue'), layout: 'tutor', workspace: 'tutor', capability: 'question.author' },
  { name: routeNames.TutorQuestionTags, path: '/training/tutor/question-tags', component: () => import('@/pages/tutor/QuestionTags.vue'), layout: 'tutor', workspace: 'tutor', capability: 'question.author', nav: { group: 'tutor', label: '标签管理', icon: CollectionTag, order: 4 } },

  // ---------- 残值评估（ValuationLayout + 独立认证页）----------
  { name: routeNames.ValuationHome, path: '/valuation', component: () => import('@/pages/student/valuation/ValuationHome.vue'), layout: 'valuation', workspace: 'valuation', requiresAuth: false },
  { name: routeNames.ValuationResult, path: '/valuation/result', component: () => import('@/pages/student/valuation/ValuationResultView.vue'), layout: 'valuation', workspace: 'valuation', requiresAuth: false },
  { name: routeNames.ValuationReport, path: '/valuation/report/:id', component: () => import('@/pages/student/valuation/ValuationReportView.vue'), layout: 'valuation', workspace: 'valuation', requiresAuth: false },
  { name: routeNames.ValuationBatteryInput, path: '/valuation/battery', component: () => import('@/pages/student/valuation/BatteryInputView.vue'), layout: 'valuation', workspace: 'valuation', requiresAuth: false },
  { name: routeNames.ValuationBatteryResult, path: '/valuation/battery/result', component: () => import('@/pages/student/valuation/BatteryResultView.vue'), layout: 'valuation', workspace: 'valuation', requiresAuth: false },
  { name: routeNames.ValuationHistory, path: '/valuation/history', component: () => import('@/pages/student/valuation/ValuationHistoryView.vue'), layout: 'valuation', workspace: 'valuation', requiresAuth: true, roles: ['hrwai_user'], capability: 'valuation.use' },
  { name: routeNames.ValuationLogin, path: '/valuation/login', component: () => import('@/pages/auth/Login.vue'), workspace: 'valuation', requiresAuth: false, authPage: true, isValuationAuthPage: true },
  { name: routeNames.ValuationRegister, path: '/valuation/register', component: () => import('@/pages/auth/Register.vue'), workspace: 'valuation', requiresAuth: false, authPage: true, isValuationAuthPage: true },
  { name: routeNames.ValuationForgotPassword, path: '/valuation/forgot-password', component: () => import('@/pages/auth/ForgotPassword.vue'), workspace: 'valuation', requiresAuth: false, authPage: true, isValuationAuthPage: true },

  // ---------- AI 助手（顶层，可选登录；归属 training 工作区）----------
  { name: routeNames.AIAssistant, path: '/ai-assistant', component: () => import('@/pages/ai-assistant/AIAssistantPage.vue'), workspace: 'training', requiresAuth: false, capability: 'ai_assistant.use', nav: { group: 'interactive', label: 'AI助手', icon: MagicStick, activeRouteNames: [routeNames.AIAssistantFeature], order: 2 } },
  { name: routeNames.AIAssistantFeature, path: '/ai-assistant/:featureKey(maintenance|drawing|exercise|fault-diagnosis)', component: () => import('@/pages/ai-assistant/FeatureChatPage.vue'), workspace: 'training', requiresAuth: false, capability: 'ai_assistant.use' },

  // ---------- 管理端（AdminLayout）----------
  { name: routeNames.AdminDashboard, path: '/admin/dashboard', component: () => import('@/pages/admin/Dashboard.vue'), layout: 'manage', workspace: 'manage', capability: 'admin.access', nav: { group: 'overview', label: '仪表盘', icon: DataAnalysis, order: 1 } },
  { name: routeNames.Statistics, path: '/admin/statistics', component: () => import('@/pages/admin/Statistics.vue'), layout: 'manage', workspace: 'manage', capability: 'admin.access', nav: { group: 'overview', label: '统计分析', icon: TrendCharts, order: 2 } },
  { name: routeNames.HrwaiUserManage, path: '/admin/hrwai-users', component: () => import('@/pages/admin/HrwaiUserManage.vue'), layout: 'manage', workspace: 'manage', capability: 'admin.access', nav: { group: 'user-content', label: '用户管理', icon: User, order: 1 } },
  { name: routeNames.ProfileReview, path: '/admin/profile-review', component: () => import('@/pages/admin/ProfileReview.vue'), layout: 'manage', workspace: 'manage', capability: 'profile.review', nav: { group: 'user-content', label: '资料审核', icon: CircleCheck, order: 2 } },
  { name: routeNames.TutorManage, path: '/admin/tutors', component: () => import('@/pages/admin/TutorManage.vue'), layout: 'manage', workspace: 'manage', capability: 'admin.access', nav: { group: 'user-content', label: '导师管理', icon: UserFilled, order: 3 } },
  { name: routeNames.RecruiterManage, path: '/admin/recruiters', component: () => import('@/pages/admin/RecruiterManage.vue'), layout: 'manage', workspace: 'manage', capability: 'recruiter.manage', nav: { group: 'user-content', label: '招聘者管理', icon: OfficeBuilding, order: 4 } },
  { name: routeNames.ForumManage, path: '/admin/forum-manage', component: () => import('@/pages/admin/ForumManage.vue'), layout: 'manage', workspace: 'manage', capability: 'forum.moderate', nav: { group: 'user-content', label: '论坛管理', icon: ChatDotRound, order: 5 } },
  { name: routeNames.ContributionManage, path: '/admin/contribution-manage', component: () => import('@/pages/admin/ContributionManage.vue'), layout: 'manage', workspace: 'manage', capability: 'contribution.review', nav: { group: 'user-content', label: '投稿管理', icon: Document, order: 6 } },
  { name: routeNames.CourseCatalog, path: '/admin/course-catalog', component: () => import('@/pages/admin/CourseCatalog.vue'), layout: 'manage', workspace: 'manage', capability: 'catalog.manage', nav: { group: 'teaching', label: '课程管理', icon: FolderOpened, order: 1 } },
  { name: routeNames.PositionManage, path: '/admin/positions', component: () => import('@/pages/admin/PositionManage.vue'), layout: 'manage', workspace: 'manage', capability: 'catalog.manage', nav: { group: 'teaching', label: '岗位管理', icon: CollectionTag, order: 2 } },
  { name: routeNames.CredentialManage, path: '/admin/credentials', component: () => import('@/pages/admin/Credentials.vue'), layout: 'manage', workspace: 'manage', capability: 'catalog.manage', nav: { group: 'teaching', label: '证件管理', icon: CollectionTag, order: 3 } },
  { name: routeNames.QuestionReview, path: '/admin/question-review', component: () => import('@/pages/admin/QuestionReview.vue'), layout: 'manage', workspace: 'manage', capability: 'question.review', nav: { group: 'teaching', label: '题库审核', icon: EditPen, order: 4 } },
  { name: routeNames.AuditLogs, path: '/admin/audit-logs', component: () => import('@/pages/admin/AuditLogs.vue'), layout: 'manage', workspace: 'manage', capability: 'audit.read', nav: { group: 'system', label: '审计日志', icon: Memo, order: 1 } },
  { name: routeNames.AdminInspection, path: '/admin/inspection', component: () => import('@/pages/admin/Inspection.vue'), layout: 'manage', workspace: 'manage', capability: 'inspection.read', nav: { group: 'system', label: '巡检视图', icon: DataAnalysis, order: 2 } },
  { name: routeNames.ValuationConfigManage, path: '/admin/valuation-config', component: () => import('@/pages/admin/ValuationConfigManage.vue'), layout: 'manage', workspace: 'manage', capability: 'valuation.config', nav: { group: 'system', label: '残值配置', icon: PriceTag, order: 3 } },
  { name: routeNames.AISettings, path: '/admin/ai-settings', component: () => import('@/pages/admin/AISettings.vue'), layout: 'manage', workspace: 'manage', capability: 'admin.access', nav: { group: 'system', label: 'AI 配置', icon: Setting, order: 4 } },
  { name: routeNames.ContentGenerate, path: '/admin/content-generate', component: () => import('@/pages/admin/ContentGenerate.vue'), layout: 'manage', workspace: 'manage', capability: 'content.manage', nav: { group: 'system', label: '内容生成', icon: MagicStick, order: 5 } },
  { name: routeNames.AdminFeaturedContentList, path: '/admin/featured-content', component: () => import('@/pages/admin/FeaturedContentList.vue'), layout: 'manage', workspace: 'manage', capability: 'content.manage', nav: { group: 'system', label: '内容精选', icon: Document, order: 6 } },
  { name: routeNames.AdminFeaturedContentEdit, path: '/admin/featured-content/edit/:id?', component: () => import('@/pages/admin/FeaturedContentEdit.vue'), layout: 'manage', workspace: 'manage', capability: 'content.manage' },

  // ---------- 招聘端（RecruitLayout）----------
  { name: routeNames.RecruitDashboard, path: '/recruit', component: () => import('@/pages/recruit/Dashboard.vue'), layout: 'recruit', workspace: 'recruit', capability: 'recruit.access', nav: { group: 'recruit', label: '首页', icon: HomeFilled, exact: true, order: 1 } },
  { name: routeNames.RecruitResumes, path: '/recruit/resumes', component: () => import('@/pages/recruit/Resumes.vue'), layout: 'recruit', workspace: 'recruit', capability: 'application.review', nav: { group: 'recruit', label: '简历库', icon: Document, activeRouteNames: [routeNames.RecruitResumeDetail], order: 2 } },
  { name: routeNames.RecruitResumeDetail, path: '/recruit/resumes/:id', component: () => import('@/pages/recruit/ResumeDetail.vue'), layout: 'recruit', workspace: 'recruit', capability: 'application.review' },
  { name: routeNames.RecruitRequests, path: '/recruit/requests', component: () => import('@/pages/recruit/MyRequests.vue'), layout: 'recruit', workspace: 'recruit', capability: 'contact.exchange', nav: { group: 'recruit', label: '我的申请', icon: Document, order: 3 } },
  { name: routeNames.RecruitJobManage, path: '/recruit/jobs', component: () => import('@/pages/recruit/JobManage.vue'), layout: 'recruit', workspace: 'recruit', capability: 'job.manage', nav: { group: 'recruit', label: '职位管理', icon: OfficeBuilding, activeRouteNames: [routeNames.RecruitApplicationList], order: 4 } },
  { name: routeNames.RecruitApplicationList, path: '/recruit/jobs/:id/applications', component: () => import('@/pages/recruit/ApplicationList.vue'), layout: 'recruit', workspace: 'recruit', capability: 'application.review' }

]

/** 导航项（含外链）派生结果：一个工作区下按分组组织的扁平清单。 */
export interface DerivedNavItem {
  key: string
  label: string
  icon?: Component
  routeName?: RouteName
  activeRouteNames?: RouteName[]
  routeParams?: Record<string, string | number>
  externalUrl?: string
  exact?: boolean
  parent?: RouteName
  children?: DerivedNavItem[]
}

/** 该工作区声明了导航的页面描述符（保持声明序）。 */
export function navPages(workspace: Workspace): PageDescriptor[] {
  return pages.filter(p => p.workspace === workspace && !!p.nav)
}

/** 描述的叶子导航项（含外链）。 */
export function toNavItem(page: PageDescriptor): DerivedNavItem {
  const n = page.nav!
  const isExternal = !!n.externalUrl
  return {
    key: page.name,
    label: n.label,
    icon: n.icon,
    routeName: isExternal ? undefined : page.name,
    activeRouteNames: n.activeRouteNames,
    routeParams: n.routeParams,
    externalUrl: n.externalUrl,
    exact: n.exact,
    parent: n.parent
  }
}

/** 跨子域外链导航项：没有路由，只有地址（同样只在这里声明一次）。 */
export interface ExternalNavItem {
  key: string
  label: string
  group: string
  icon?: Component
  externalUrl: string
}

export const externalNavItems: Record<string, ExternalNavItem[]> = {
  training: [
    { key: 'featured', label: '内容精选', group: 'interactive', icon: Document, externalUrl: 'https://www.gccsmile.com/news' }
  ]
}
