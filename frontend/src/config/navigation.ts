import type { Component } from 'vue'
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
  Trophy
} from '@element-plus/icons-vue'
// 注：MagicStick 仍用于管理员"内容生成"菜单项

export interface NavItem {
  key: string
  label: string
  /** 目标路由的 name（见 src/router/index.ts 各路由的 name 字段），而非硬编码路径。 */
  routeName?: string
  /**
   * 除 `routeName` 外，还应让本项高亮的路由 name。
   *
   * 用于「列表页 → 详情页」这类**详情页没有独立导航项**的场景：
   * ForumPage → ForumDetail、CourseList → ChapterView、
   * TutorCourses → TutorChapterManage / TutorChapterEdit、
   * TutorQuestionManage → TutorQuestionCreate。
   * 不配的话，进入详情后侧栏整条（含父分组）都不高亮。
   */
  activeRouteNames?: string[]
  /** 目标路由需要的动态参数（如章节页 ChapterView 需要 courseId/chapterId）。 */
  routeParams?: Record<string, string | number>
  icon?: Component
  children?: NavItem[]
  /** 外链地址（用于内容精选等跨子域跳转） */
  externalUrl?: string
  // exact=true 时仅精确匹配 route.name 才高亮。
  // name 匹配天然精确，因此该标记保留以兼容既有语义，但不再参与前缀判断。
  exact?: boolean
}

type RouteParamsLike = Record<string, string | string[] | undefined>

/**
 * 判断当前路由是否命中某个导航项。
 *
 * 抽成纯函数的原因：侧栏高亮出过两次线上问题（一次双高亮、一次详情页不高亮），
 * 而 `isRouteActive` 原先是 `AppSidebar.vue` 里的私有函数，无法单测覆盖。
 *
 * 匹配规则：
 * 1. `routeName` 精确相等，或命中 `activeRouteNames` 中任一名字；
 * 2. 若声明了 `routeParams`，需再逐个比对 params —— 避免同名路由（如不同课程的
 *    章节页）被一起点亮。
 */
export function isNavRouteActive(
  item: NavItem,
  routeName: unknown,
  routeParams?: RouteParamsLike
): boolean {
  if (!item.routeName) return false

  const names = [item.routeName, ...(item.activeRouteNames ?? [])]
  if (!names.includes(String(routeName ?? ''))) return false

  if (item.routeParams) {
    for (const [k, v] of Object.entries(item.routeParams)) {
      const actual = routeParams?.[k]
      const actualValue = Array.isArray(actual) ? actual[0] : actual
      if (String(actualValue ?? '') !== String(v)) return false
    }
  }
  return true
}

const studentNav: NavItem[] = [
  {
    key: 'learning',
    label: '学习中心',
    icon: HomeFilled,
    children: [
      { key: 'dashboard', label: '仪表盘', routeName: 'StudentDashboard', icon: HomeFilled, exact: true },
      {
        key: 'courses',
        label: '课程中心',
        routeName: 'CourseList',
        activeRouteNames: ['ChapterView'],
        icon: Notebook
      },
      { key: 'materials', label: '学习资料', routeName: 'StudentMaterials', icon: Files },
      { key: 'search', label: '全局搜索', routeName: 'StudentSearch', icon: Search }
    ]
  },
  {
    key: 'exam',
    label: '题库与考试',
    icon: EditPen,
    children: [
      {
        key: 'question-bank',
        label: '题库练习',
        routeName: 'QuestionBank',
        icon: EditPen,
        children: [{ key: 'real-exam', label: '真题练习', routeName: 'RealExamPapers', icon: Document }]
      },
      { key: 'mock-exam', label: '模拟考试', routeName: 'MockExam', icon: Document },
      { key: 'wrong-questions', label: '错题本', routeName: 'WrongQuestions', icon: CircleCloseFilled }
    ]
  },
  {
    key: 'interactive',
    label: '互动与工具',
    icon: ChatDotRound,
    children: [
      {
        key: 'forum',
        label: '学员论坛',
        routeName: 'ForumPage',
        activeRouteNames: ['ForumDetail'],
        icon: ChatDotRound
      },
      {
        key: 'ai-assistant',
        label: 'AI助手',
        routeName: 'AIAssistant',
        // 专项功能页（故障咨询/故障代码/维保知识/图纸识别/习题解答）是 AIAssistant 的
        // 兄弟路由（router 里未嵌套），由 AIAssistantPage 的卡片 router.push 进入
        activeRouteNames: ['AIAssistantFeature'],
        icon: MagicStick
      },
      { key: 'featured', label: '内容精选', icon: Document, externalUrl: 'https://www.gccsmile.com/news' }
    ]
  },
  {
    key: 'personal',
    label: '个人',
    icon: User,
    children: [
      { key: 'task-center', label: '任务中心', routeName: 'TaskCenter', icon: Trophy },
      { key: 'favorites', label: '我的收藏', routeName: 'StudentFavorites', icon: Star },
      { key: 'profile', label: '个人资料', routeName: 'StudentProfile', icon: User },
      { key: 'resume', label: '我的简历', routeName: 'StudentResume', icon: Document }
    ]
  }
]

const adminNav: NavItem[] = [
  {
    key: 'overview',
    label: '总览',
    icon: DataAnalysis,
    children: [
      { key: 'dashboard', label: '仪表盘', routeName: 'AdminDashboard', icon: DataAnalysis },
      { key: 'statistics', label: '统计分析', routeName: 'Statistics', icon: TrendCharts }
    ]
  },
  {
    key: 'user-content',
    label: '用户与内容',
    icon: User,
    children: [
      { key: 'hrwai-users', label: '用户管理', routeName: 'HrwaiUserManage', icon: User },
      { key: 'profile-review', label: '资料审核', routeName: 'ProfileReview', icon: CircleCheck },
      { key: 'tutors', label: '导师管理', routeName: 'TutorManage', icon: UserFilled },
      { key: 'forum-manage', label: '论坛管理', routeName: 'ForumManage', icon: ChatDotRound }
    ]
  },
  {
    key: 'teaching',
    label: '教学管理',
    icon: FolderOpened,
    children: [
      { key: 'course-catalog', label: '课程管理', routeName: 'CourseCatalog', icon: FolderOpened },
      { key: 'credentials', label: '证件管理', routeName: 'CredentialManage', icon: CollectionTag },
      { key: 'question-review', label: '题库审核', routeName: 'QuestionReview', icon: EditPen },
      { key: 'question-tags', label: '题库标签', routeName: 'QuestionTags', icon: CollectionTag }
    ]
  },
  {
    key: 'system',
    label: '系统',
    icon: Setting,
    children: [
      { key: 'audit-logs', label: '审计日志', routeName: 'AuditLogs', icon: Memo },
      { key: 'valuation-config', label: '残值配置', routeName: 'ValuationConfigManage', icon: PriceTag },
      { key: 'ai-settings', label: 'AI 配置', routeName: 'AISettings', icon: Setting },
      { key: 'content-generate', label: '内容生成', routeName: 'ContentGenerate', icon: MagicStick },
      { key: 'featured-content', label: '内容精选', routeName: 'AdminFeaturedContentList', icon: Document }
    ]
  }
]

const tutorNav: NavItem[] = [
  { key: 'dashboard', label: '仪表盘', routeName: 'TutorDashboard', icon: HomeFilled, exact: true },
  {
    key: 'courses',
    label: '我的课程',
    routeName: 'TutorCourses',
    // 章节列表与章节编辑都在「我的课程」之下，两级都没有独立导航项
    activeRouteNames: ['TutorChapterManage', 'TutorChapterEdit'],
    icon: Notebook
  },
  {
    key: 'question-manage',
    label: '题库管理',
    routeName: 'TutorQuestionManage',
    // 新增题目 / 编辑题目共用 TutorQuestionCreate（带 query.id 即编辑）
    activeRouteNames: ['TutorQuestionCreate'],
    icon: EditPen
  }
]

export const roleNavigation: Record<string, NavItem[]> = {
  student: studentNav,
  admin: adminNav,
  tutor: tutorNav
}
