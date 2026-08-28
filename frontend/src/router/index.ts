import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useCredentialStore } from '@/stores/credential'
import {
  getSubdomain,
  buildCrossDomainAuthUrl,
  getTargetSubdomainForPath,
  getDefaultWorkspaceBySubdomain,
  isIpDirectMode
} from '@/utils/subdomain'
import { resolveWorkspaceForRole } from '@/utils/authRedirect'

const routes: RouteRecordRaw[] = [
  // ========== 登录 / 注册 ==========
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/pages/auth/Login.vue'),
    meta: { requiresAuth: false, authPage: true }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/pages/auth/Register.vue'),
    meta: { requiresAuth: false, authPage: true }
  },
  {
    path: '/forgot-password',
    name: 'ForgotPassword',
    component: () => import('@/pages/auth/ForgotPassword.vue'),
    meta: { requiresAuth: false, authPage: true }
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
        path: 'search',
        name: 'StudentSearch',
        component: () => import('@/pages/student/SearchPage.vue')
      },
      {
        path: 'materials',
        name: 'StudentMaterials',
        component: () => import('@/pages/student/Materials.vue')
      },
      {
        path: 'favorites',
        name: 'StudentFavorites',
        component: () => import('@/pages/student/Favorites.vue')
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
        path: 'wrong-questions',
        name: 'WrongQuestions',
        component: () => import('@/pages/student/WrongQuestions.vue')
      },
      {
        path: 'real-exam',
        name: 'RealExamPapers',
        component: () => import('@/pages/student/RealExamPapers.vue')
      },
      // PROTOTYPE — throwaway route for 真题练习占位三变体对比 (__prototype 命名即标记)，验证后移除
      {
        path: '__prototype/real-exam',
        name: 'RealExamPrototype',
        component: () => import('@/pages/student/__prototype__/RealExamPrototype.vue')
      },
      // PROTOTYPE — throwaway route for 论坛浏览记录三变体
      {
        path: '__prototype/forum-history',
        name: 'ForumHistoryPrototype',
        component: () => import('@/pages/student/__prototype__/forum-history/ForumHistoryPrototype.vue')
      },
      // PROTOTYPE — throwaway route for 任务中心+积分占位三变体
      {
        path: '__prototype/task-center',
        name: 'TaskCenterPrototype',
        component: () => import('@/pages/student/__prototype__/task-center/TaskCenterPrototype.vue')
      },
      {
        path: 'task-center',
        name: 'TaskCenter',
        component: () => import('@/pages/student/TaskCenter.vue')
      },
      {
        path: 'profile',
        name: 'StudentProfile',
        component: () => import('@/pages/student/Profile.vue')
      },
      {
        path: 'onboarding/credential',
        name: 'CredentialOnboarding',
        component: () => import('@/pages/onboarding/CredentialOnboarding.vue'),
        meta: { requiresAuth: true, role: 'hrwai_user' }
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
    meta: { requiresAuth: false, isValuationAuthPage: true, authPage: true }
  },
  {
    path: '/valuation/register',
    name: 'ValuationRegister',
    component: () => import('@/pages/auth/Register.vue'),
    meta: { requiresAuth: false, isValuationAuthPage: true, authPage: true }
  },
  {
    path: '/valuation/forgot-password',
    name: 'ValuationForgotPassword',
    component: () => import('@/pages/auth/ForgotPassword.vue'),
    meta: { requiresAuth: false, isValuationAuthPage: true, authPage: true }
  },

  // ========== 原型预览（独立页面，无需登录/后端） ==========
  // PROTOTYPE — throwaway standalone preview for 真题练习占位三变体；kept outside TrainingLayout so it bypasses auth/credential guards
  {
    path: '/prototype/real-exam',
    name: 'RealExamPrototypeStandalone',
    component: () => import('@/pages/student/__prototype__/RealExamPrototype.vue'),
    meta: { requiresAuth: false }
  },
  // PROTOTYPE — standalone for 论坛浏览记录
  {
    path: '/prototype/forum-history',
    name: 'ForumHistoryPrototypeStandalone',
    component: () => import('@/pages/student/__prototype__/forum-history/ForumHistoryPrototype.vue'),
    meta: { requiresAuth: false }
  },
  // PROTOTYPE — standalone for 任务中心+积分占位
  {
    path: '/prototype/task-center',
    name: 'TaskCenterPrototypeStandalone',
    component: () => import('@/pages/student/__prototype__/task-center/TaskCenterPrototype.vue'),
    meta: { requiresAuth: false }
  },

  // ========== AI 助手模块（training 子域名，可选登录；登录后可保存历史会话） ==========
  // 官网门户重构后（ADR-0001），AI 助手归属学员工作区：由 www 迁至 training 子域名
  {
    path: '/ai-assistant',
    name: 'AIAssistant',
    component: () => import('@/pages/ai-assistant/AIAssistantPage.vue'),
    meta: { requiresAuth: false }
  },
  // 专项功能页（故障咨询/故障代码查询/维保知识/图纸识别/习题解答）
  {
    path: '/ai-assistant/:featureKey(fault-consult|fault-code|maintenance|drawing|exercise)',
    name: 'AIAssistantFeature',
    component: () => import('@/pages/ai-assistant/FeatureChatPage.vue'),
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
        path: 'credentials',
        name: 'CredentialManage',
        component: () => import('@/pages/admin/Credentials.vue')
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

  // ========== 根路径兜底（IP 直连模式） ==========
  // 官网已重构为独立 Nuxt 仓库（ADR-0001），Vue SPA 不再承载 '/'；
  // IP 直连模式下根路径按角色进入默认工作区
  {
    path: '/',
    redirect: () => {
      // valuation 子域根路径 → 估值首页（公开，无需登录）；
      // 原逻辑会跳 /login，守卫再把 valuation 子域的登录页转成 /valuation/login
      if (getSubdomain() === 'valuation') return '/valuation'
      const authStore = useAuthStore()
      const workspace = resolveWorkspaceForRole(authStore.userInfo?.role)
      // 未知角色 resolveWorkspaceForRole 返回 '/'，根路径按原逻辑回登录页
      return workspace === '/' ? '/login' : workspace
    }
  },

  // ========== 兼容旧路由 /dashboard/* ==========
  {
    path: '/dashboard',
    redirect: () => {
      const authStore = useAuthStore()
      return resolveWorkspaceForRole(authStore.userInfo?.role)
    }
  },
  {
    path: '/dashboard/:pathMatch(.*)*',
    redirect: to => {
      const authStore = useAuthStore()
      const subPath = (to.params.pathMatch as string[])?.[0] || ''

      // 特殊路径映射
      if (subPath === 'valuation' || subPath.startsWith('valuation/')) {
        return '/' + subPath
      }
      if (subPath === 'ai-generate') {
        return '/ai-assistant'
      }
      // 默认按角色跳转（单点函数 resolveWorkspaceForRole）
      return resolveWorkspaceForRole(authStore.userInfo?.role)
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

  // 等待认证初始化完成（main.ts 显式启动，幂等；同一 Promise 等待不重复执行）
  await authStore.initialize()

  const isValuationPath = to.path === '/valuation' || to.path.startsWith('/valuation/')
  // 认证页标记（meta.authPage）：/login、/register、/forgot-password 及 /valuation 下的独立登录/注册/找回密码
  const isAuthPage = to.matched.some(record => record.meta?.authPage)
  // valuation 子域名独立认证页（回跳 /valuation/history 特判）
  const isValuationLoginPage = isAuthPage && isValuationPath
  // 主体系认证页（training / 主域名通用登录流程，不含 valuation 认证页）
  const isLoginPath = isAuthPage && !isValuationPath

  // ===== 子域名边界检查 =====
  // 五类子域名：main（公共）、training（学员培训+AI助手）、valuation（残值评估）、
  // tutor（导师工作区）、admin（管理员后台）
  // 跨子域名访问会触发整页跳转（不同 origin，token 不共享）
  // IP 直连模式下跳过子域名边界检查（无 DNS 子域名环境，通过路径直接访问所有工作区）
  const currentSubdomain = getSubdomain()
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
        if (to.path === '/register') next('/valuation/register')
        else if (to.path === '/forgot-password') next('/valuation/forgot-password')
        else next('/valuation/login')
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
    // 管理员/导师回各自工作台（单点函数 resolveWorkspaceForRole）
    if (userRole === 'admin' || userRole === 'tutor') {
      next(resolveWorkspaceForRole(userRole))
    } else if (isValuationPath) {
      // 学员/未知角色访问估值受限页 → 回估值首页（公开，无需登录）
      next('/valuation')
    } else {
      // 其余 → 学员工作区
      next('/training')
    }
    return
  }

  // ===== 目标证件预筛选（ADR-0020）=====
  // 强制拦截：hrwai_user 且 current_credential_id 为空时，training 工作区内除 onboarding 外均重定向至 onboarding
  if (userRole === 'hrwai_user' && !isIpDirectMode()) {
    const targetSubdomain = getTargetSubdomainForPath(to.path)
    const isTrainingPath = targetSubdomain === 'training' || to.path.startsWith('/training')
    const isOnboarding = to.name === 'CredentialOnboarding'
    if (isTrainingPath) {
      try {
        const credStore = useCredentialStore()
        if (!credStore.initialized) {
          await credStore.loadCurrent()
        }
        if (credStore.current === null && !isOnboarding) {
          next({ name: 'CredentialOnboarding' })
          return
        }
        if (credStore.current !== null && isOnboarding) {
          next('/training')
          return
        }
      } catch {
        // 网络异常时放行，避免卡死登录
      }
    }
  }

  next()
})

export default router
