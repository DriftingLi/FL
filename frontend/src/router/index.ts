import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useCredentialStore } from '@/stores/credential'
import {
  getSubdomain,
  buildCrossDomainAuthUrl,
  getDefaultWorkspaceBySubdomain,
  isIpDirectMode
} from '@/utils/subdomain'
import { resolveGuardDecision, type GuardInput, type GuardState } from './guard'
import { resolveWorkspaceForRole } from '@/utils/authRedirect'
import { routeNames } from '@/config/routeNames'

// workspace（工作区）声明约定（#618）：「这条路由属于哪个工作区」单点写在各布局/页面路由的
// meta.workspace（子路由经 vue-router meta 合并继承，无需逐条重复）；守卫与登录回跳读声明，
// 子域前缀表（authRedirect.PATH_AUTH_ENTRIES）只兜底未声明路径（404）。工作区语义而非子域名字面，
// 派生关系（workspace → 子域名）单点在 guard.WORKSPACE_SUBDOMAIN。
// redirect 型记录（/ /dashboard /tutor 等旧路径兼容）在路由解析期即被改写为目标路由，
// 不会成为守卫的 to，因此无需 workspace 声明。

const routes: RouteRecordRaw[] = [
  // ========== 登录 / 注册 ==========
  {
    path: '/login',
    name: routeNames.Login,
    component: () => import('@/pages/auth/Login.vue'),
    meta: { requiresAuth: false, authPage: true, workspace: 'auth' }
  },
  {
    path: '/register',
    name: routeNames.Register,
    component: () => import('@/pages/auth/Register.vue'),
    meta: { requiresAuth: false, authPage: true, workspace: 'auth' }
  },
  {
    path: '/forgot-password',
    name: routeNames.ForgotPassword,
    component: () => import('@/pages/auth/ForgotPassword.vue'),
    meta: { requiresAuth: false, authPage: true, workspace: 'auth' }
  },

  // ========== 培训模块 - 学员子区 ==========
  {
    path: '/training',
    component: () => import('@/layouts/TrainingLayout.vue'),
    meta: { requiresAuth: true, role: 'hrwai_user', workspace: 'training' },
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
    meta: { requiresAuth: true, role: 'tutor', workspace: 'tutor' },
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
    meta: { requiresAuth: false, workspace: 'valuation' },
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
    meta: { requiresAuth: false, isValuationAuthPage: true, authPage: true, workspace: 'valuation' }
  },
  {
    path: '/valuation/register',
    name: routeNames.ValuationRegister,
    component: () => import('@/pages/auth/Register.vue'),
    meta: { requiresAuth: false, isValuationAuthPage: true, authPage: true, workspace: 'valuation' }
  },
  {
    path: '/valuation/forgot-password',
    name: routeNames.ValuationForgotPassword,
    component: () => import('@/pages/auth/ForgotPassword.vue'),
    meta: { requiresAuth: false, isValuationAuthPage: true, authPage: true, workspace: 'valuation' }
  },

  // ========== AI 助手模块（training 子域名，可选登录；登录后可保存历史会话） ==========
  // 官网门户重构后（ADR-0001），AI 助手归属学员工作区：由 www 迁至 training 子域名
  {
    path: '/ai-assistant',
    name: routeNames.AIAssistant,
    component: () => import('@/pages/ai-assistant/AIAssistantPage.vue'),
    meta: { requiresAuth: false, workspace: 'training' }
  },
  // 专项功能页（维保知识/图纸识别/习题解答/智能维修诊断）
  {
    path: '/ai-assistant/:featureKey(maintenance|drawing|exercise|fault-diagnosis)',
    name: routeNames.AIAssistantFeature,
    component: () => import('@/pages/ai-assistant/FeatureChatPage.vue'),
    meta: { requiresAuth: false, workspace: 'training' }
  },

  // ========== 管理员后台 ==========
  {
    path: '/admin',
    component: () => import('@/layouts/AdminLayout.vue'),
    meta: { requiresAuth: true, role: 'admin', workspace: 'manage' },
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
    meta: { requiresAuth: true, role: 'recruiter', workspace: 'recruit' },
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

/**
 * 全局守卫 orchestrator（#618）：决策逻辑全部在 guard.ts 的纯函数决策管线
 * （(目标路由， 认证/环境状态) ⇒ 决策，全分支单测覆盖），这里只做三件事——
 * 1. 注入环境事实：等待认证初始化、投影 to 为 GuardInput、读取 auth/credential/子域名状态；
 * 2. 执行决策：next / 清登录态重定向 / 跨子域名整页跳转 / 当前子域名默认工作区；
 * 3. load-credential 决策时补拉一次证件数据后重跑管线（管线纯函数，重跑无副作用；
 *    loadCurrent 内部吞错，失败记为无证件，与既有行为一致）。
 */
router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()

  // 等待认证初始化完成（main.ts 显式启动，幂等；同一 Promise 等待不重复执行）
  await authStore.initialize()

  const credStore = useCredentialStore()
  let credentialLoadAttempted = false

  const input = (): GuardInput => ({
    path: to.path,
    fullPath: to.fullPath,
    name: to.name,
    meta: to.meta as Record<string, unknown>,
    // 逐条匹配记录的 meta（保持既有 some() 语义：requiresAuth/authPage 按逐条判断）
    matched: to.matched.map(record => record.meta as Record<string, unknown>)
  })

  const state = (): GuardState => ({
    isLoggedIn: authStore.isLoggedIn,
    role: authStore.userInfo?.role ?? '',
    hasValidToken: !!(authStore.token && authStore.isLoggedIn && authStore.userInfo && authStore.userInfo.role),
    subdomain: getSubdomain(),
    ipDirect: isIpDirectMode(),
    credential: !credStore.initialized ? 'unloaded' : credStore.current === null ? 'none' : 'present'
  })

  for (;;) {
    const decision = resolveGuardDecision(input(), state())
    switch (decision.action) {
      case 'allow':
        next()
        return
      case 'redirect':
        // clearAuth = token 失效口径：先清登录态再回认证页
        if (decision.clearAuth) authStore.clearAuthData()
        next(decision.to)
        return
      case 'external':
        // 跨子域名整页跳转（不同 origin，token 不共享；经 auth_token 参数交接登录态）
        window.location.href = buildCrossDomainAuthUrl(decision.target, decision.path)
        return
      case 'workspace-home':
        // 留在当前子域名的默认工作区（valuation/recruit 子域名有独立入口）
        next(getDefaultWorkspaceBySubdomain())
        return
      case 'load-credential':
        // 证件数据未加载：补拉一次后重跑管线；credentialLoadAttempted 防御性兜底死循环
        if (credentialLoadAttempted) {
          next()
          return
        }
        credentialLoadAttempted = true
        await credStore.loadCurrent()
        break
    }
  }
})

export default router
