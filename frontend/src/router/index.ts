import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import {
  getSubdomain,
  buildCrossDomainAuthUrl,
  getTargetSubdomainForPath,
  getDefaultWorkspaceBySubdomain,
  isIpDirectMode
} from '@/utils/subdomain'

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
      },
      {
        path: 'content/:id',
        name: 'PortalContentDetail',
        component: () => import('@/pages/portal/ContentDetail.vue'),
        meta: { requiresAuth: false }
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
    meta: { requiresAuth: true, role: 'hrwai_user' },
    children: [
      {
        path: '',
        name: 'StudentDashboard',
        component: () => import('@/pages/student/Dashboard.vue')
      },
      {
        path: 'courses',
        name: 'CourseList',
        component: () => import('@/pages/student/CourseList.vue')
      },
      {
        path: 'forum',
        name: 'ForumPage',
        component: () => import('@/pages/student/ForumPage.vue')
      },
      {
        path: 'forum/:topicId',
        name: 'ForumDetail',
        component: () => import('@/pages/student/ForumDetail.vue')
      },
      {
        path: 'course/:courseId/chapter/:chapterId',
        name: 'ChapterView',
        component: () => import('@/pages/student/ChapterView.vue')
      },
      {
        path: 'question-bank',
        name: 'QuestionBank',
        component: () => import('@/pages/student/QuestionBank.vue')
      },
      {
        path: 'mock-exam',
        name: 'MockExam',
        component: () => import('@/pages/student/MockExam.vue')
      },
      {
        path: 'level-exam',
        name: 'LevelExam',
        component: () => import('@/pages/student/LevelExam.vue')
      },
      {
        path: 'wrong-questions',
        name: 'WrongQuestions',
        component: () => import('@/pages/student/WrongQuestions.vue')
      },
      {
        path: 'profile',
        name: 'StudentProfile',
        component: () => import('@/pages/student/Profile.vue')
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
        name: 'TutorDashboard',
        component: () => import('@/pages/tutor/Dashboard.vue')
      },
      {
        path: 'courses',
        name: 'TutorCourses',
        component: () => import('@/pages/tutor/TutorCourses.vue')
      },
      {
        path: 'course/:id/chapters',
        name: 'TutorChapterManage',
        component: () => import('@/pages/tutor/ChapterManage.vue')
      },
      {
        path: 'course/:courseId/chapter/:chapterId',
        name: 'TutorChapterEdit',
        component: () => import('@/pages/tutor/TutorChapterEdit.vue')
      },
      {
        path: 'question-manage',
        name: 'TutorQuestionManage',
        component: () => import('@/pages/tutor/QuestionManage.vue')
      },
      {
        path: 'question-create',
        name: 'TutorQuestionCreate',
        component: () => import('@/pages/tutor/QuestionCreate.vue')
      },
      {
        path: 'grading',
        name: 'TutorGrading',
        component: () => import('@/pages/tutor/GradingPage.vue')
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
        meta: { requiresAuth: false }
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
        meta: { requiresAuth: false }
      },
      {
        path: 'report/:id',
        name: 'ValuationReport',
        component: () => import('@/pages/student/valuation/ValuationReportView.vue'),
        meta: { requiresAuth: false }
      },
      {
        path: 'battery',
        name: 'ValuationBatteryInput',
        component: () => import('@/pages/student/valuation/BatteryInputView.vue'),
        meta: { requiresAuth: false }
      },
      {
        path: 'battery/result',
        name: 'ValuationBatteryResult',
        component: () => import('@/pages/student/valuation/BatteryResultView.vue'),
        meta: { requiresAuth: false }
      },
      {
        path: 'history',
        name: 'ValuationHistory',
        component: () => import('@/pages/student/valuation/ValuationHistoryView.vue'),
        meta: { requiresAuth: true, roles: ['hrwai_user'] }
      }
    ]
  },

  // ========== 残值评估独立登录 / 注册（独立全屏页，不挂 ValuationLayout）==========
  {
    path: '/valuation/login',
    name: 'ValuationLogin',
    component: () => import('@/pages/auth/Login.vue'),
    meta: { requiresAuth: false, isValuationAuthPage: true }
  },
  {
    path: '/valuation/register',
    name: 'ValuationRegister',
    component: () => import('@/pages/auth/Register.vue'),
    meta: { requiresAuth: false, isValuationAuthPage: true }
  },

  // ========== AI 助手模块（主域名，可选登录；登录后可保存历史会话）==========
  {
    path: '/ai-assistant',
    name: 'AIAssistant',
    component: () => import('@/pages/ai-assistant/AIAssistantPage.vue'),
    meta: { requiresAuth: false }
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
        component: () => import('@/pages/admin/Dashboard.vue')
      },
        {
          path: 'hrwai-users',
          name: 'HrwaiUserManage',
          component: () => import('@/pages/admin/HrwaiUserManage.vue')
        },
        {
          path: 'profile-review',
          name: 'ProfileReview',
          component: () => import('@/pages/admin/ProfileReview.vue')
        },
        {
          path: 'forum-manage',
          name: 'ForumManage',
          component: () => import('@/pages/admin/ForumManage.vue')
        },
      {
        path: 'course-catalog',
        name: 'CourseCatalog',
        component: () => import('@/pages/admin/CourseCatalog.vue')
      },
      {
        path: 'question-tags',
        name: 'QuestionTags',
        component: () => import('@/pages/admin/QuestionTags.vue')
      },
      {
        path: 'question-review',
        name: 'QuestionReview',
        component: () => import('@/pages/admin/QuestionReview.vue')
      },
      {
        path: 'statistics',
        name: 'Statistics',
        component: () => import('@/pages/admin/Statistics.vue')
      },
      {
        path: 'audit-logs',
        name: 'AuditLogs',
        component: () => import('@/pages/admin/AuditLogs.vue')
      },
      {
        path: 'content-generate',
        name: 'ContentGenerate',
        component: () => import('@/pages/admin/ContentGenerate.vue')
      },
      {
        path: 'featured-content',
        name: 'AdminFeaturedContentList',
        component: () => import('@/pages/admin/FeaturedContentList.vue')
      },
      {
        path: 'featured-content/edit/:id?',
        name: 'AdminFeaturedContentEdit',
        component: () => import('@/pages/admin/FeaturedContentEdit.vue')
      },
      {
        path: 'exam-sessions',
        name: 'ExamSessionManage',
        component: () => import('@/pages/admin/ExamSessionManage.vue')
      },
      {
        path: 'tutors',
        name: 'TutorManage',
        component: () => import('@/pages/admin/TutorManage.vue')
      },
      {
        path: 'valuation-config',
        name: 'ValuationConfigManage',
        component: () => import('@/pages/admin/ValuationConfigManage.vue')
      },
      {
        path: 'ai-settings',
        name: 'AISettings',
        component: () => import('@/pages/admin/AISettings.vue')
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
      if (role === 'hrwai_user') return '/training'
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
      // 默认按角色跳转
      if (role === 'admin') return '/admin/dashboard'
      if (role === 'tutor') return '/training/tutor'
      if (role === 'hrwai_user') return '/training'
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

router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()

  // 等待 auth store 初始化完成
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

  const isValuationPath = to.path === '/valuation' || to.path.startsWith('/valuation/')
  const isValuationLoginPage = to.name === 'ValuationLogin' || to.name === 'ValuationRegister'

  // ===== 子域名边界检查 =====
  // 五类子域名：main（公共）、training（学员培训+AI助手）、valuation（残值评估）、
  // tutor（导师工作区）、admin（管理员后台）
  // 跨子域名访问会触发整页跳转（不同 origin，token 不共享）
  // IP 直连模式下跳过子域名边界检查（无 DNS 子域名环境，通过路径直接访问所有工作区）
  const currentSubdomain = getSubdomain()
  const isLoginPath = to.path === '/login' || to.path === '/register'
  const skipSubdomainCheck = isIpDirectMode()

  if (!skipSubdomainCheck) {
    if (isValuationPath) {
      // 估值路径必须在 valuation 子域名下访问
      if (currentSubdomain !== 'valuation') {
        window.location.href = buildCrossDomainAuthUrl('valuation', to.fullPath)
        return
      }
    } else if (isLoginPath) {
      // /login 和 /register 在主域名上跳到 training 子域名（主域名不再承载登录）
      // valuation 子域名有独立的 /valuation/login 与 /valuation/register，主体系 /login 重定向过去
      if (currentSubdomain === 'main') {
        window.location.href = buildCrossDomainAuthUrl('training', to.fullPath)
        return
      }
      if (currentSubdomain === 'valuation') {
        next(to.path === '/register' ? '/valuation/register' : '/valuation/login')
        return
      }
    } else {
      // 非登录路径：按路径前缀映射到对应子域名
      const targetSubdomain = getTargetSubdomainForPath(to.path)
      if (currentSubdomain !== targetSubdomain) {
        if (targetSubdomain === 'main') {
          // AI 助手部署在主域名，允许从功能子域名跳转过去；
          // 其余公共路径（/、/dispatch 等）留在当前子域名默认工作区
          if (to.path.startsWith('/ai-assistant')) {
            window.location.href = buildCrossDomainAuthUrl('main', to.fullPath)
          } else {
            next(getDefaultWorkspaceBySubdomain())
          }
        } else {
          // 路径属于另一个功能子域名 → 跨子域名跳转
          window.location.href = buildCrossDomainAuthUrl(targetSubdomain, to.fullPath)
        }
        return
      }
    }
  }

  // 已登录用户访问登录页：按当前子域名跳转到对应工作区
  if (isLoginPath && authStore.isLoggedIn && authStore.userInfo.role) {
    next(getDefaultWorkspaceBySubdomain())
    return
  }

  // 已登录用户访问估值登录/注册页 → 跳回评估历史
  if (isValuationLoginPage && authStore.isLoggedIn && authStore.userInfo.role === 'hrwai_user') {
    next('/valuation/history')
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
    // 估值路径跳估值登录页，其余跳主登录页
    if (isValuationPath) {
      next({ path: '/valuation/login', query: { redirect: to.fullPath } })
    } else {
      next({ path: '/login', query: { redirect: to.fullPath } })
    }
    return
  }

  // 角色校验：优先使用最内层匹配的 meta（to.meta 已是最终合并的 meta）
  const userRole = authStore.userInfo?.role ?? ''
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
    } else if (isValuationPath) {
      next('/valuation')
    } else {
      next('/training')
    }
    return
  }

  next()
})

export default router
