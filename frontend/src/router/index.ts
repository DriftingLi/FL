import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { watch } from 'vue'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  // ========== 官网 ==========
  {
    path: '/',
    component: () => import('@/layouts/PortalHomeLayout.vue'),
    meta: { requiresAuth: false },
    children: [
      {
        path: '',
        name: 'PortalHome',
        component: () => import('@/pages/portal/PortalHome.vue')
      }
    ]
  },

  // ========== 登录 / 注册 ==========
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/pages/auth/Login.vue'),
    meta: { requiresAuth: false }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/pages/auth/Register.vue'),
    meta: { requiresAuth: false }
  },

  // ========== 培训模块 - 学员子区 ==========
  {
    path: '/training',
    component: () => import('@/layouts/TrainingLayout.vue'),
    meta: { requiresAuth: true, role: 'student' },
    children: [
      {
        path: '',
        name: 'TrainingHome',
        component: () => import('@/pages/student/CourseList.vue'),
        meta: { navKey: 'courses', navLabel: '课程中心', navGroup: 'training' }
      },
      {
        path: 'course/:id',
        name: 'CourseDetail',
        component: () => import('@/pages/student/CourseDetail.vue'),
        meta: { navKey: 'course-detail', navLabel: '课程详情', navGroup: 'training' }
      },
      {
        path: 'course/:courseId/chapter/:chapterId',
        name: 'ChapterView',
        component: () => import('@/pages/student/ChapterView.vue'),
        meta: { navKey: 'chapter', navLabel: '章节学习', navGroup: 'training' }
      },
      {
        path: 'exam/:courseId',
        name: 'Exam',
        component: () => import('@/pages/student/Exam.vue'),
        meta: { navKey: 'exam', navLabel: '课程考试', navGroup: 'training' }
      },
      {
        path: 'question-bank',
        name: 'QuestionBank',
        component: () => import('@/pages/student/QuestionBank.vue'),
        meta: { navKey: 'question-bank', navLabel: '题库练习', navGroup: 'training' }
      },
      {
        path: 'practice-free',
        name: 'PracticeFree',
        component: () => import('@/pages/student/PracticeFree.vue'),
        meta: { navKey: 'practice-free', navLabel: '自由练习', navGroup: 'training' }
      },
      {
        path: 'knowledge-practice',
        name: 'KnowledgePractice',
        component: () => import('@/pages/student/KnowledgePractice.vue'),
        meta: { navKey: 'knowledge-practice', navLabel: '知识点练习', navGroup: 'training' }
      },
      {
        path: 'practice-stats',
        name: 'PracticeStats',
        component: () => import('@/pages/student/PracticeStats.vue'),
        meta: { navKey: 'practice-stats', navLabel: '练习统计', navGroup: 'training' }
      },
      {
        path: 'mock-exam',
        name: 'MockExam',
        component: () => import('@/pages/student/MockExam.vue'),
        meta: { navKey: 'mock-exam', navLabel: '模拟考试', navGroup: 'exam' }
      },
      {
        path: 'level-exam',
        name: 'LevelExam',
        component: () => import('@/pages/student/LevelExam.vue'),
        meta: { navKey: 'level-exam', navLabel: '考试中心', navGroup: 'exam' }
      },
      {
        path: 'wrong-questions',
        name: 'WrongQuestions',
        component: () => import('@/pages/student/WrongQuestions.vue'),
        meta: { navKey: 'wrong-questions', navLabel: '错题本', navGroup: 'exam' }
      }
    ]
  },

  // ========== 培训模块 - 导师子区 ==========
  {
    path: '/training/tutor',
    component: () => import('@/layouts/TutorLayout.vue'),
    meta: { requiresAuth: true, role: 'tutor' },
    children: [
      {
        path: '',
        redirect: '/training/tutor/courses'
      },
      {
        path: 'courses',
        name: 'TutorCourses',
        component: () => import('@/pages/tutor/TutorCourses.vue'),
        meta: { navKey: 'courses', navLabel: '我的课程', navGroup: 'courses' }
      },
      {
        path: 'course/:id/chapters',
        name: 'TutorChapterManage',
        component: () => import('@/pages/tutor/ChapterManage.vue'),
        meta: { navKey: 'chapters', navLabel: '章节管理', navGroup: 'courses' }
      },
      {
        path: 'question-manage',
        name: 'TutorQuestionManage',
        component: () => import('@/pages/tutor/QuestionManage.vue'),
        meta: { navKey: 'question-manage', navLabel: '题库管理', navGroup: 'question' }
      },
      {
        path: 'question-create',
        name: 'TutorQuestionCreate',
        component: () => import('@/pages/tutor/QuestionCreate.vue'),
        meta: { navKey: 'question-create', navLabel: '创建题目', navGroup: 'question' }
      },
      {
        path: 'grading',
        name: 'TutorGrading',
        component: () => import('@/pages/tutor/GradingPage.vue'),
        meta: { navKey: 'grading', navLabel: '人工阅卷', navGroup: 'grading' }
      }
    ]
  },

  // ========== 残值评估模块（核心功能公开，历史需登录）==========
  {
    path: '/valuation',
    component: () => import('@/layouts/ValuationLayout.vue'),
    meta: { requiresAuth: false },
    children: [
      {
        path: '',
        name: 'ValuationHome',
        component: () => import('@/pages/student/valuation/ValuationHome.vue'),
        meta: { requiresAuth: false, navKey: 'valuation', navLabel: '残值评估', navGroup: 'tools' }
      },
      {
        // 设计稿将表单提升为首页：访问 /valuation/input 等同于 /valuation
        path: 'input',
        redirect: { name: 'ValuationHome' }
      },
      {
        path: 'result',
        name: 'ValuationResult',
        component: () => import('@/pages/student/valuation/ValuationResultView.vue'),
        meta: { requiresAuth: false, navKey: 'valuation-result', navLabel: '评估结果', navGroup: 'tools' }
      },
      {
        path: 'report/:id',
        name: 'ValuationReport',
        component: () => import('@/pages/student/valuation/ValuationReportView.vue'),
        meta: { requiresAuth: false, navKey: 'valuation-report', navLabel: '评估报告', navGroup: 'tools' }
      },
      {
        path: 'battery',
        name: 'ValuationBatteryInput',
        component: () => import('@/pages/student/valuation/BatteryInputView.vue'),
        meta: { requiresAuth: false, navKey: 'valuation-battery', navLabel: '电池评估', navGroup: 'tools' }
      },
      {
        path: 'battery/result',
        name: 'ValuationBatteryResult',
        component: () => import('@/pages/student/valuation/BatteryResultView.vue'),
        meta: { requiresAuth: false, navKey: 'valuation-battery-result', navLabel: '电池评估结果', navGroup: 'tools' }
      },
      {
        path: 'history',
        name: 'ValuationHistory',
        component: () => import('@/pages/student/valuation/ValuationHistoryView.vue'),
        meta: { requiresAuth: true, roles: ['student', 'tutor', 'admin'], navKey: 'valuation-history', navLabel: '评估历史', navGroup: 'tools' }
      }
    ]
  },

  // ========== AI 助手模块（学员/导师/管理员均可）==========
  {
    path: '/ai-assistant',
    component: () => import('@/layouts/AIAssistantLayout.vue'),
    meta: { requiresAuth: true, roles: ['student', 'tutor', 'admin'] },
    children: [
      {
        path: '',
        name: 'AIAssistant',
        component: () => import('@/pages/student/AIAssistant.vue'),
        meta: { navKey: 'ai-assistant', navLabel: 'AI 助手', navGroup: 'tools' }
      }
    ]
  },

  // ========== 学员个人中心 ==========
  {
    path: '/profile',
    component: () => import('@/layouts/ProfileLayout.vue'),
    meta: { requiresAuth: true, role: 'student' },
    children: [
      {
        path: '',
        name: 'Profile',
        component: () => import('@/pages/student/Profile.vue'),
        meta: { navKey: 'profile', navLabel: '个人中心', navGroup: 'profile' }
      }
    ]
  },

  // ========== 管理员后台 ==========
  {
    path: '/admin',
    component: () => import('@/layouts/AdminLayout.vue'),
    meta: { requiresAuth: true, role: 'admin' },
    children: [
      {
        path: '',
        redirect: '/admin/dashboard'
      },
      {
        path: 'dashboard',
        name: 'AdminDashboard',
        component: () => import('@/pages/admin/Dashboard.vue'),
        meta: { navKey: 'dashboard', navLabel: '仪表盘', navGroup: 'dashboard' }
      },
      {
        path: 'students',
        name: 'StudentManage',
        component: () => import('@/pages/admin/StudentManage.vue'),
        meta: { navKey: 'students', navLabel: '学员管理', navGroup: 'education' }
      },
      {
        path: 'courses',
        name: 'CourseManage',
        component: () => import('@/pages/admin/CourseManage.vue'),
        meta: { navKey: 'courses', navLabel: '课程管理', navGroup: 'education' }
      },
      {
        path: 'statistics',
        name: 'Statistics',
        component: () => import('@/pages/admin/Statistics.vue'),
        meta: { navKey: 'statistics', navLabel: '统计分析', navGroup: 'data' }
      },
      {
        path: 'content-generate',
        name: 'ContentGenerate',
        component: () => import('@/pages/admin/ContentGenerate.vue'),
        meta: { navKey: 'content-generate', navLabel: '内容生成', navGroup: 'content' }
      },
      {
        path: 'exam-sessions',
        name: 'ExamSessionManage',
        component: () => import('@/pages/admin/ExamSessionManage.vue'),
        meta: { navKey: 'exam-sessions', navLabel: '考试场次', navGroup: 'education' }
      },
      {
        path: 'tutors',
        name: 'TutorManage',
        component: () => import('@/pages/admin/TutorManage.vue'),
        meta: { navKey: 'tutors', navLabel: '导师管理', navGroup: 'education' }
      },
      {
        path: 'valuation-config',
        name: 'ValuationConfigManage',
        component: () => import('@/pages/admin/ValuationConfigManage.vue'),
        meta: { navKey: 'valuation-config', navLabel: '残值配置', navGroup: 'data' }
      }
    ]
  },

  // ========== 派单系统占位（二手叉车交易相关，未来扩展）==========
  {
    path: '/dispatch',
    name: 'DispatchComingSoon',
    component: () => import('@/pages/student/DispatchComingSoon.vue'),
    meta: { requiresAuth: false }
  },

  // ========== 兼容旧路由 /dashboard/* ==========
  {
    path: '/dashboard',
    redirect: () => {
      const authStore = useAuthStore()
      const role = authStore.userInfo?.role
      if (role === 'admin') return '/admin/dashboard'
      if (role === 'tutor') return '/training/tutor'
      if (role === 'student') return '/training'
      return '/'
    }
  },
  {
    path: '/dashboard/:pathMatch(.*)*',
    redirect: to => {
      const authStore = useAuthStore()
      const role = authStore.userInfo?.role
      const subPath = (to.params.pathMatch as string[])?.[0] || ''

      // 特殊路径映射
      if (subPath === 'valuation' || subPath.startsWith('valuation/')) {
        return '/' + subPath
      }
      if (subPath === 'ai-generate') {
        return '/ai-assistant'
      }
      if (subPath === 'profile') {
        return '/profile'
      }

      // 默认按角色跳转
      if (role === 'admin') return '/admin/dashboard'
      if (role === 'tutor') return '/training/tutor'
      if (role === 'student') return '/training'
      return '/'
    }
  },

  // ========== 兼容旧路由 /tutor/* ==========
  {
    path: '/tutor',
    redirect: '/training/tutor'
  },
  {
    path: '/tutor/:pathMatch(.*)*',
    redirect: to => {
      const subPath = (to.params.pathMatch as string[])?.[0] || ''
      return subPath ? `/training/tutor/${subPath}` : '/training/tutor'
    }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()

  if (authStore.isInitializing) {
    await new Promise<void>(resolve => {
      const unwatch = watch(() => authStore.isInitializing, (val) => {
        if (!val) {
          unwatch()
          resolve()
        }
      })
    })
  }

  // 已登录用户访问 /login 或 /register：按角色处理
  const isPublicLogin = to.path === '/login' || to.path === '/register'
  if (isPublicLogin && authStore.isLoggedIn && authStore.userInfo.role) {
    const role = authStore.userInfo.role
    if (role === 'admin') {
      next('/admin/dashboard')
    } else {
      // 学员/导师留在官网首页
      next('/')
    }
    return
  }

  // 通过 to.matched 检查是否需要鉴权（支持子路由覆盖父路由 meta）
  const requiresAuth = to.matched.some(record => record.meta?.requiresAuth === true)

  if (!requiresAuth) {
    next()
    return
  }

  const hasValidToken = authStore.token &&
                        authStore.isLoggedIn &&
                        authStore.userInfo &&
                        authStore.userInfo.role

  if (!hasValidToken) {
    authStore.clearAuthData()
    next({ path: '/login', query: { redirect: to.fullPath } })
    return
  }

  // 角色校验：优先使用最内层匹配的 meta（to.meta 已是最终合并的 meta）
  const userRole = authStore.userInfo.role
  const requiredRole = to.meta?.role as string | undefined
  const requiredRoles = to.meta?.roles as string[] | undefined

  const roleMatched = requiredRoles
    ? requiredRoles.includes(userRole)
    : (requiredRole ? requiredRole === userRole : true)

  if (!roleMatched) {
    if (userRole === 'admin') {
      next('/admin/dashboard')
    } else if (userRole === 'tutor') {
      next('/training/tutor')
    } else {
      next('/training')
    }
    return
  }

  next()
})

export default router
