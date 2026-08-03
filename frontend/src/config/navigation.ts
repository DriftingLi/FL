import type { Component } from 'vue'
import {
  HomeFilled,
  Notebook,
  EditPen,
  Document,
  CircleCloseFilled,
  MagicStick,
  DataAnalysis,
  User,
  TrendCharts,
  UserFilled,
  Calendar,
  ChatDotRound,
  PriceTag,
  Finished,
  Setting,
  Memo,
  CircleCheck
} from '@element-plus/icons-vue'
// 注：MagicStick 仍用于管理员"内容生成"菜单项

export interface NavItem {
  key: string
  label: string
  path?: string
  icon?: Component
  children?: NavItem[]
  // exact=true 时仅精确匹配 route.path 才高亮，
  // 用于路径恰好是其他菜单父级的情况（如学员/导师仪表盘）。
  exact?: boolean
}

const studentNav: NavItem[] = [
  { key: 'dashboard', label: '仪表盘', path: '/training', icon: HomeFilled, exact: true },
  { key: 'courses', label: '课程中心', path: '/training/courses', icon: Notebook },
  { key: 'forum', label: '学员论坛', path: '/training/forum', icon: ChatDotRound },
  { key: 'question-bank', label: '题库练习', path: '/training/question-bank', icon: EditPen },
  { key: 'level-exam', label: '考试中心', path: '/training/level-exam', icon: Document },
  { key: 'wrong-questions', label: '错题本', path: '/training/wrong-questions', icon: CircleCloseFilled },
  { key: 'ai-assistant', label: 'AI助手', path: '/ai-assistant', icon: MagicStick },
  { key: 'valuation', label: '残值评估', path: '/valuation', icon: PriceTag },
  { key: 'profile', label: '个人资料', path: '/training/profile', icon: User }
]

const adminNav: NavItem[] = [
  { key: 'dashboard', label: '仪表盘', path: '/admin/dashboard', icon: DataAnalysis },
  { key: 'hrwai-users', label: '用户管理', path: '/admin/hrwai-users', icon: User },
  { key: 'profile-review', label: '资料审核', path: '/admin/profile-review', icon: CircleCheck },
  { key: 'forum-manage', label: '论坛管理', path: '/admin/forum-manage', icon: ChatDotRound },
  { key: 'tutors', label: '导师管理', path: '/admin/tutors', icon: UserFilled },
  { key: 'courses', label: '课程管理', path: '/admin/courses', icon: Notebook },
  { key: 'question-review', label: '题库审核', path: '/admin/question-review', icon: EditPen },
  { key: 'exam-sessions', label: '考试场次', path: '/admin/exam-sessions', icon: Calendar },
  { key: 'statistics', label: '统计分析', path: '/admin/statistics', icon: TrendCharts },
  { key: 'audit-logs', label: '审计日志', path: '/admin/audit-logs', icon: Memo },
  { key: 'valuation-config', label: '残值配置', path: '/admin/valuation-config', icon: PriceTag },
  { key: 'ai-settings', label: 'AI 配置', path: '/admin/ai-settings', icon: Setting },
  { key: 'content-generate', label: '内容生成', path: '/admin/content-generate', icon: MagicStick },
  { key: 'featured-content', label: '内容精选', path: '/admin/featured-content', icon: Document }
]

const tutorNav: NavItem[] = [
  { key: 'dashboard', label: '仪表盘', path: '/training/tutor', icon: HomeFilled, exact: true },
  { key: 'courses', label: '我的课程', path: '/training/tutor/courses', icon: Notebook },
  { key: 'question-manage', label: '题库管理', path: '/training/tutor/question-manage', icon: EditPen },
  { key: 'grading', label: '人工阅卷', path: '/training/tutor/grading', icon: Finished }
]

const portalNav: NavItem[] = [
  // portalNav 用于主域名官网导航：path 已经是相对于主域名的绝对路径，
  // 跨子域名调用方需自行拼接主域名 URL（如 buildSubdomainUrl('main', item.path)）。
  // 注意：子域名下不要直接渲染 portalNav，否则会跳到当前子域名的根路径。
  { key: 'home', label: '首页', path: '/' },
  { key: 'about', label: '关于我们', path: '/#about' },
  { key: 'products', label: '核心服务', path: '/#products' },
  { key: 'cooperation', label: '合作模式', path: '/#cooperation' },
  { key: 'service', label: '服务保障', path: '/#service' },
  { key: 'featured', label: '内容精选', path: '/#featured' },
  { key: 'contact', label: '加入我们', path: '/#footer' }
]

export const roleNavigation: Record<string, NavItem[]> = {
  student: studentNav,
  admin: adminNav,
  tutor: tutorNav,
  portal: portalNav
}
