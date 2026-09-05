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
import { routeNames } from '@/config/routeNames'

const routes: RouteRecordRaw[] = [
  // ========== 登录 / 注册 ==========
  {
    path: '/login',
    name: routeNames.Login,
    component: () => import('@/pages/auth/Login.vue'),
    meta: { requiresAuth: false, authPage: true }
  },
  {
    path: '/register',
    name: routeNames.Register,
    component: () => import('@/pages/auth/Register.vue'),
    meta: { requiresAuth: false, authPage: true }
  },
  {
    path: '/forgot-password',
    name: routeNames.ForgotPassword,
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
        name: routeNames.StudentDashboard,
        component: () => import('@/pages/student/Dashboard.vue')
      },
      {
        path: 'courses',
        name: routeNames.CourseList,
        component: () => import('@/pages/student/CourseList.vue')
      },
      {
        path: 'search',
        name: routeNames.StudentSearch,
        component: () => import('@/pages/student/SearchPage.vue')
      },
      {
        path: 'materials',
        name: routeNames.StudentMaterials,
        component: () => import('@/pages/student/Materials.vue')
      },
      {
        path: 'favorites',
        name: routeNames.StudentFavorites,
        component: () => import('@/pages/student/Favorites.vue')
      },
      {
        path: 'forum',
        name: routeNames.ForumPage,
        component: () => import('@/pages/student/ForumPage.vue')
      },
      {
        path: 'forum/ask',
        name: routeNames.ForumAsk,
        component: () => import('@/pages/student/ForumAskPage.vue')
      },
      {
        path: 'forum/:topicId',
        name: routeNames.ForumDetail,
        component: () => import('@/pages/student/ForumDetail.vue')
      },
      {
        path: 'course/:courseId/chapter/:chapterId',
        name: routeNames.ChapterView,
        component: () => import('@/pages/student/ChapterView.vue')
      },
      {
        path: 'question-bank',
        name: routeNames.QuestionBank,
        component: () => import('@/pages/student/QuestionBank.vue')
      },
      {
        path: 'mock-exam',
        name: routeNames.MockExam,
        component: () => import('@/pages/student/MockExam.vue')
      },
      {
        path: 'wrong-questions',
        name: routeNames.WrongQuestions,
        component: () => import('@/pages/student/WrongQuestions.vue')
      },
      {
        path: 'real-exam',
        name: routeNames.RealExamPapers,
        component: () => import('@/pages/student/RealExamPapers.vue')
      },
      {
        // 真题卷按卷练习（不进侧栏，从真题列表进入）
        path: 'real-exam/practice/:paperId',
        name: routeNames.RealExamPractice,
        component: () => import('@/pages/student/RealExamPractice.vue')
      },
      {
        // 每日打卡独立页（ADR-0028：打卡从任务中心/论坛弹窗剥离为独立激励面）
        path: 'check-in',
        name: routeNames.CheckIn,
        component: () => import('@/pages/student/CheckInPage.vue')
      },
      {
        path: 'task-center',
        name: routeNames.TaskCenter,
        component: () => import('@/pages/student/TaskCenter.vue')
      },
      {
        // #512：任务中心二级页——积分明细（含流水与规则抽屉）
        path: 'task-center/points',
        name: routeNames.PointsLedger,
        component: () => import('@/pages/student/PointsLedger.vue')
      },
      {
        path: 'profile',
        name: routeNames.StudentProfile,
        component: () => import('@/pages/student/Profile.vue')
      },
      {
        path: 'resume',
        name: routeNames.StudentResume,
        component: () => import('@/pages/student/ResumePage.vue')
      },
      {
        // #491：独立编辑入口（表单内不含 PDF 上传项，保存后回预览）
        path: 'resume/edit',
        name: routeNames.StudentResumeEdit,
        component: () => import('@/pages/student/ResumeEdit.vue')
      },
      {
        path: 'jobs',
        name: routeNames.JobPlaza,
        component: () => import('@/pages/student/JobPlaza.vue')
      },
      {
        path: 'jobs/:id',
        name: routeNames.JobDetail,
        component: () => import('@/pages/student/JobDetail.vue')
      },
      {
        path: 'applications',
        name: routeNames.MyApplications,
        component: () => import('@/pages/student/MyApplications.vue')
      },
      {
        path: 'onboarding/credential',
        name: routeNames.CredentialOnboarding,
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
        name: routeNames.TutorDashboard,
        component: () => import('@/pages/tutor/Dashboard.vue')
      },
      {
        path: 'courses',
        name: routeNames.TutorCourses,
        component: () => import('@/pages/tutor/TutorCourses.vue')
      },
      {
        path: 'course/:id/chapters',
        name: routeNames.TutorChapterManage,
        component: () => import('@/pages/tutor/ChapterManage.vue')
      },
      {
        path: 'course/:courseId/chapter/:chapterId',
        name: routeNames.TutorChapterEdit,
        component: () => import('@/pages/tutor/TutorChapterEdit.vue')
      },
      {
        path: 'question-manage',
        name: routeNames.TutorQuestionManage,
        component: () => import('@/pages/tutor/QuestionManage.vue')
      },
      {
        path: 'question-create',
        name: routeNames.TutorQuestionCreate,
        component: () => import('@/pages/tutor/QuestionCreate.vue')
      },
      {
        path: 'question-tags',
        name: routeNames.TutorQuestionTags,
        component: () => import('@/pages/tutor/QuestionTags.vue')
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
        name: routeNames.ValuationHome,
        component: () => import('@/pages/student/valuation/ValuationHome.vue'),
        meta: { requiresAuth: false }
      },
      {
        // 设计稿将表单提升为首页：访问 /valuation/input 等同于 /valuation
        path: 'input',
        redirect: { name: routeNames.ValuationHome }
      },
      {
        path: 'result',
        name: routeNames.ValuationResult,
        component: () => import('@/pages/student/valuation/ValuationResultView.vue'),
        meta: { requiresAuth: false }
      },
      {
        path: 'report/:id',
        name: routeNames.ValuationReport,
        component: () => import('@/pages/student/valuation/ValuationReportView.vue'),
        meta: { requiresAuth: false }
      },
      {
        path: 'battery',
        name: routeNames.ValuationBatteryInput,
        component: () => import('@/pages/student/valuation/BatteryInputView.vue'),
        meta: { requiresAuth: false }
      },
      {
        path: 'battery/result',
        name: routeNames.ValuationBatteryResult,
        component: () => import('@/pages/student/valuation/BatteryResultView.vue'),
        meta: { requiresAuth: false }
      },
      {
        path: 'history',
        name: routeNames.ValuationHistory,
        component: () => import('@/pages/student/valuation/ValuationHistoryView.vue'),
        meta: { requiresAuth: true, roles: ['hrwai_user'] }
      }
    ]
  },

  // ========== 残值评估独立登录 / 注册（独立全屏页，不挂 ValuationLayout）==========
  {
    path: '/valuation/login',
    name: routeNames.ValuationLogin,
    component: () => import('@/pages/auth/Login.vue'),
    meta: { requiresAuth: false, isValuationAuthPage: true, authPage: true }
  },
  {
    path: '/valuation/register',
    name: routeNames.ValuationRegister,
    component: () => import('@/pages/auth/Register.vue'),
    meta: { requiresAuth: false, isValuationAuthPage: true, authPage: true }
  },
  {
    path: '/valuation/forgot-password',
    name: routeNames.ValuationForgotPassword,
    component: () => import('@/pages/auth/ForgotPassword.vue'),
    meta: { requiresAuth: false, isValuationAuthPage: true, authPage: true }
  },

  // ========== AI 助手模块（training 子域名，可选登录；登录后可保存历史会话） ==========
  // 官网门户重构后（ADR-0001），AI 助手归属学员工作区：由 www 迁至 training 子域名
  {
    path: '/ai-assistant',
    name: routeNames.AIAssistant,
    component: () => import('@/pages/ai-assistant/AIAssistantPage.vue'),
    meta: { requiresAuth: false }
  },
  // 专项功能页（故障咨询/故障代码查询/维保知识/图纸识别/习题解答）
  {
    path: '/ai-assistant/:featureKey(fault-consult|fault-code|maintenance|drawing|exercise)',
    name: routeNames.AIAssistantFeature,
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
        name: routeNames.AdminDashboard,
        component: () => import('@/pages/admin/Dashboard.vue')
      },
        {
          path: 'hrwai-users',
          name: routeNames.HrwaiUserManage,
          component: () => import('@/pages/admin/HrwaiUserManage.vue')
        },
        {
          path: 'profile-review',
          name: routeNames.ProfileReview,
          component: () => import('@/pages/admin/ProfileReview.vue')
        },
        {
          path: 'forum-manage',
          name: routeNames.ForumManage,
          component: () => import('@/pages/admin/ForumManage.vue')
        },
        {
          path: 'contribution-manage',
          name: routeNames.ContributionManage,
          component: () => import('@/pages/admin/ContributionManage.vue')
        },
      {
        path: 'course-catalog',
        name: routeNames.CourseCatalog,
        component: () => import('@/pages/admin/CourseCatalog.vue')
      },
      {
        path: 'credentials',
        name: routeNames.CredentialManage,
        component: () => import('@/pages/admin/Credentials.vue')
      },
      {
        path: 'positions',
        name: routeNames.PositionManage,
        component: () => import('@/pages/admin/PositionManage.vue')
      },
      {
        path: 'question-review',
        name: routeNames.QuestionReview,
        component: () => import('@/pages/admin/QuestionReview.vue')
      },
      {
        path: 'statistics',
        name: routeNames.Statistics,
        component: () => import('@/pages/admin/Statistics.vue')
      },
      {
        path: 'audit-logs',
        name: routeNames.AuditLogs,
        component: () => import('@/pages/admin/AuditLogs.vue')
      },
      {
        path: 'inspection',
        name: routeNames.AdminInspection,
        component: () => import('@/pages/admin/Inspection.vue')
      },
      {
        path: 'content-generate',
        name: routeNames.ContentGenerate,
        component: () => import('@/pages/admin/ContentGenerate.vue')
      },
      {
        path: 'featured-content',
        name: routeNames.AdminFeaturedContentList,
        component: () => import('@/pages/admin/FeaturedContentList.vue')
      },
      {
        path: 'featured-content/edit/:id?',
        name: routeNames.AdminFeaturedContentEdit,
        component: () => import('@/pages/admin/FeaturedContentEdit.vue')
      },
      {
        path: 'tutors',
        name: routeNames.TutorManage,
        component: () => import('@/pages/admin/TutorManage.vue')
      },
      {
        path: 'recruiters',
        name: routeNames.RecruiterManage,
        component: () => import('@/pages/admin/RecruiterManage.vue')
      },
      {
        path: 'valuation-config',
        name: routeNames.ValuationConfigManage,
        component: () => import('@/pages/admin/ValuationConfigManage.vue')
      },
      {
        path: 'ai-settings',
        name: routeNames.AISettings,
        component: () => import('@/pages/admin/AISettings.vue')
      }
    ]
  },

  // ========== 企业招聘端 ==========
  {
    path: '/recruit',
    component: () => import('@/layouts/RecruitLayout.vue'),
    meta: { requiresAuth: true, role: 'recruiter' },
    children: [
      {
        path: '',
        name: routeNames.RecruitDashboard,
        component: () => import('@/pages/recruit/Dashboard.vue')
      },
      {
        path: 'resumes',
        name: routeNames.RecruitResumes,
        component: () => import('@/pages/recruit/Resumes.vue')
      },
      {
        path: 'resumes/:id',
        name: routeNames.RecruitResumeDetail,
        component: () => import('@/pages/recruit/ResumeDetail.vue')
      },
      {
        path: 'requests',
        name: routeNames.RecruitRequests,
        component: () => import('@/pages/recruit/MyRequests.vue')
      },
      {
        path: 'jobs',
        name: routeNames.RecruitJobManage,
        component: () => import('@/pages/recruit/JobManage.vue')
      },
      {
        path: 'jobs/:id/applications',
        name: routeNames.RecruitApplicationList,
        component: () => import('@/pages/recruit/ApplicationList.vue')
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
      if (getSubdomain() === 'recruit') return '/recruit'
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
  // 六类子域名：main（公共）、training（学员培训+AI助手）、valuation（残值评估）、
  // tutor（导师工作区）、admin（管理员后台）、recruit（企业招聘）
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
    const isOnboarding = to.name === routeNames.CredentialOnboarding
    if (isTrainingPath) {
      try {
        const credStore = useCredentialStore()
        if (!credStore.initialized) {
          await credStore.loadCurrent()
        }
        if (credStore.current === null && !isOnboarding) {
          next({ name: routeNames.CredentialOnboarding })
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
