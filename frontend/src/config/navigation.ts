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
  CollectionTag
} from '@element-plus/icons-vue'
// 注：MagicStick 仍用于管理员"内容生成"菜单项

export interface NavItem {
  key: string
  label: string
  /** 目标路由的 name（见 src/router/index.ts 各路由的 name 字段），而非硬编码路径。 */
  routeName?: string
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

const studentNav: NavItem[] = [
  { key: 'dashboard', label: '仪表盘', routeName: 'StudentDashboard', icon: HomeFilled, exact: true },
  { key: 'courses', label: '课程中心', routeName: 'CourseList', icon: Notebook },
  { key: 'search', label: '全局搜索', routeName: 'StudentSearch', icon: Search },
  { key: 'materials', label: '学习资料', routeName: 'StudentMaterials', icon: Files },
  { key: 'forum', label: '学员论坛', routeName: 'ForumPage', icon: ChatDotRound },
  { key: 'favorites', label: '我的收藏', routeName: 'StudentFavorites', icon: Star },
  { key: 'question-bank', label: '题库练习', routeName: 'QuestionBank', icon: EditPen },
  { key: 'wrong-questions', label: '错题本', routeName: 'WrongQuestions', icon: CircleCloseFilled },
  { key: 'ai-assistant', label: 'AI助手', routeName: 'AIAssistant', icon: MagicStick },
  { key: 'valuation', label: '残值评估', routeName: 'ValuationHome', icon: PriceTag },
  { key: 'featured', label: '内容精选', icon: Document, externalUrl: 'https://www.hrwai.com/news' },
  { key: 'profile', label: '个人资料', routeName: 'StudentProfile', icon: User }
]

const adminNav: NavItem[] = [
  { key: 'dashboard', label: '仪表盘', routeName: 'AdminDashboard', icon: DataAnalysis },
  { key: 'hrwai-users', label: '用户管理', routeName: 'HrwaiUserManage', icon: User },
  { key: 'profile-review', label: '资料审核', routeName: 'ProfileReview', icon: CircleCheck },
  { key: 'forum-manage', label: '论坛管理', routeName: 'ForumManage', icon: ChatDotRound },
  { key: 'tutors', label: '导师管理', routeName: 'TutorManage', icon: UserFilled },
  { key: 'course-catalog', label: '课程管理', routeName: 'CourseCatalog', icon: FolderOpened },
  { key: 'credentials', label: '证件管理', routeName: 'CredentialManage', icon: CollectionTag },
  { key: 'question-review', label: '题库审核', routeName: 'QuestionReview', icon: EditPen },
  { key: 'question-tags', label: '题库标签', routeName: 'QuestionTags', icon: CollectionTag },
  { key: 'statistics', label: '统计分析', routeName: 'Statistics', icon: TrendCharts },
  { key: 'audit-logs', label: '审计日志', routeName: 'AuditLogs', icon: Memo },
  { key: 'valuation-config', label: '残值配置', routeName: 'ValuationConfigManage', icon: PriceTag },
  { key: 'ai-settings', label: 'AI 配置', routeName: 'AISettings', icon: Setting },
  { key: 'content-generate', label: '内容生成', routeName: 'ContentGenerate', icon: MagicStick },
  { key: 'featured-content', label: '内容精选', routeName: 'AdminFeaturedContentList', icon: Document }
]

const tutorNav: NavItem[] = [
  { key: 'dashboard', label: '仪表盘', routeName: 'TutorDashboard', icon: HomeFilled, exact: true },
  { key: 'courses', label: '我的课程', routeName: 'TutorCourses', icon: Notebook },
  { key: 'question-manage', label: '题库管理', routeName: 'TutorQuestionManage', icon: EditPen }
]

export const roleNavigation: Record<string, NavItem[]> = {
  student: studentNav,
  admin: adminNav,
  tutor: tutorNav
}
