import type { Component } from 'vue'
import {
  Notebook,
  EditPen,
  SetUp,
  Document,
  DocumentCopy,
  CircleCloseFilled,
  MagicStick,
  DataAnalysis,
  User,
  TrendCharts,
  UserFilled,
  Calendar,
  PriceTag,
  Finished
} from '@element-plus/icons-vue'

export interface NavItem {
  key: string
  label: string
  path?: string
  icon?: Component
  children?: NavItem[]
}

const studentNav: NavItem[] = [
  {
    key: 'training',
    label: '培训',
    path: '/training',
    icon: Notebook,
    children: [
      { key: 'courses', label: '课程中心', path: '/training', icon: Notebook },
      { key: 'question-bank', label: '题库练习', path: '/training/question-bank', icon: EditPen },
      { key: 'practice', label: '虚拟实操', path: '/training/practice', icon: SetUp }
    ]
  },
  {
    key: 'exam',
    label: '考试',
    path: '/training/level-exam',
    icon: Document,
    children: [
      { key: 'level-exam', label: '考试中心', path: '/training/level-exam', icon: Document },
      { key: 'mock-exam', label: '模拟考试', path: '/training/mock-exam', icon: DocumentCopy },
      { key: 'wrong-questions', label: '错题本', path: '/training/wrong-questions', icon: CircleCloseFilled }
    ]
  },
  {
    key: 'tools',
    label: '工具',
    path: '/valuation',
    icon: DataAnalysis,
    children: [
      { key: 'valuation', label: '残值评估', path: '/valuation', icon: DataAnalysis },
      { key: 'ai-assistant', label: 'AI 助手', path: '/ai-assistant', icon: MagicStick }
    ]
  },
  { key: 'profile', label: '个人中心', path: '/profile', icon: User }
]

const adminNav: NavItem[] = [
  { key: 'dashboard', label: '仪表盘', path: '/admin/dashboard', icon: DataAnalysis },
  {
    key: 'education',
    label: '教务',
    icon: Notebook,
    children: [
      { key: 'students', label: '学员管理', path: '/admin/students', icon: User },
      { key: 'tutors', label: '导师管理', path: '/admin/tutors', icon: UserFilled },
      { key: 'courses', label: '课程管理', path: '/admin/courses', icon: Notebook },
      { key: 'exam-sessions', label: '考试场次', path: '/admin/exam-sessions', icon: Calendar }
    ]
  },
  {
    key: 'data',
    label: '数据',
    icon: TrendCharts,
    children: [
      { key: 'statistics', label: '统计分析', path: '/admin/statistics', icon: TrendCharts },
      { key: 'valuation-config', label: '残值配置', path: '/admin/valuation-config', icon: PriceTag }
    ]
  },
  { key: 'content-generate', label: '内容生成', path: '/admin/content-generate', icon: MagicStick }
]

const tutorNav: NavItem[] = [
  { key: 'courses', label: '我的课程', path: '/training/tutor/courses', icon: Notebook },
  { key: 'question-manage', label: '题库管理', path: '/training/tutor/question-manage', icon: EditPen },
  { key: 'grading', label: '人工阅卷', path: '/training/tutor/grading', icon: Finished }
]

const portalNav: NavItem[] = [
  { key: 'home', label: '首页', path: '/' },
  { key: 'about', label: '关于我们', path: '/#about' },
  { key: 'products', label: '核心服务', path: '/#products' },
  { key: 'cooperation', label: '合作模式', path: '/#cooperation' },
  { key: 'service', label: '服务保障', path: '/#service' },
  { key: 'contact', label: '加入我们', path: '/#footer' }
]

export const roleNavigation: Record<string, NavItem[]> = {
  student: studentNav,
  admin: adminNav,
  tutor: tutorNav,
  portal: portalNav
}
