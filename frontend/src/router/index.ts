import { createRouter, createWebHistory } from 'vue-router'
import { watch } from 'vue'
import { useAuthStore } from '@/stores/auth'

const routes = [
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
  {
    path: '/',
    component: () => import('@/layouts/DefaultLayout.vue'),
    meta: { requiresAuth: true, role: 'student' },
    children: [
      {
        path: '',
        name: 'Home',
        component: () => import('@/pages/student/Home.vue'),
        meta: { navKey: 'home', navLabel: '首页', navGroup: 'home' }
      },
      {
        path: 'courses',
        name: 'CourseList',
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
      },
      {
        path: 'ai-generate',
        name: 'AIGenerate',
        component: () => import('@/pages/student/AIAssistant.vue'),
        meta: { navKey: 'ai-generate', navLabel: 'AI 助手', navGroup: 'tools' }
      },
      {
        path: 'practice',
        name: 'Practice',
        component: () => import('@/pages/student/Practice.vue'),
        meta: { navKey: 'practice', navLabel: '虚拟实操', navGroup: 'training' }
      },
      {
        path: 'profile',
        name: 'Profile',
        component: () => import('@/pages/student/Profile.vue'),
        meta: { navKey: 'profile', navLabel: '个人中心', navGroup: 'profile' }
      },
      {
        path: 'valuation',
        name: 'ValuationHome',
        component: () => import('@/pages/student/valuation/ValuationHome.vue'),
        meta: { navKey: 'valuation', navLabel: '残值评估', navGroup: 'tools' }
      },
      {
        path: 'valuation/input',
        name: 'ValuationInput',
        component: () => import('@/pages/student/valuation/ValuationInputView.vue'),
        meta: { navKey: 'valuation-input', navLabel: '整车评估', navGroup: 'tools' }
      },
      {
        path: 'valuation/result',
        name: 'ValuationResult',
        component: () => import('@/pages/student/valuation/ValuationResultView.vue'),
        meta: { navKey: 'valuation-result', navLabel: '评估结果', navGroup: 'tools' }
      },
      {
        path: 'valuation/report/:id',
        name: 'ValuationReport',
        component: () => import('@/pages/student/valuation/ValuationReportView.vue'),
        meta: { navKey: 'valuation-report', navLabel: '评估报告', navGroup: 'tools' }
      },
      {
        path: 'valuation/battery',
        name: 'ValuationBatteryInput',
        component: () => import('@/pages/student/valuation/BatteryInputView.vue'),
        meta: { navKey: 'valuation-battery', navLabel: '电池评估', navGroup: 'tools' }
      },
      {
        path: 'valuation/battery/result',
        name: 'ValuationBatteryResult',
        component: () => import('@/pages/student/valuation/BatteryResultView.vue'),
        meta: { navKey: 'valuation-battery-result', navLabel: '电池评估结果', navGroup: 'tools' }
      },
      {
        path: 'valuation/history',
        name: 'ValuationHistory',
        component: () => import('@/pages/student/valuation/ValuationHistoryView.vue'),
        meta: { navKey: 'valuation-history', navLabel: '评估历史', navGroup: 'tools' }
      },
      {
        path: 'dispatch',
        name: 'DispatchComingSoon',
        component: () => import('@/pages/student/DispatchComingSoon.vue'),
        meta: { navKey: 'dispatch', navLabel: '派单系统', navGroup: 'tools' }
      }
    ]
  },
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
  {
    path: '/tutor',
    component: () => import('@/layouts/TutorLayout.vue'),
    meta: { requiresAuth: true, role: 'tutor' },
    children: [
      {
        path: '',
        redirect: '/tutor/courses'
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
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()
  const whiteList = ['/login', '/register']

  if (authStore.isInitializing) {
    await new Promise(resolve => {
      const unwatch = watch(() => authStore.isInitializing, (val) => {
        if (!val) {
          unwatch()
          resolve()
        }
      })
    })
  }

  if (whiteList.includes(to.path)) {
    if (authStore.isLoggedIn && authStore.userInfo.role) {
      const role = authStore.userInfo.role
      if (role === 'admin' && to.path === '/login') {
        next('/admin/dashboard')
      } else if (role === 'student' && to.path === '/login') {
        next('/')
      } else if (role === 'tutor' && to.path === '/login') {
        next('/tutor/courses')
      } else {
        next()
      }
    } else {
      next()
    }
  } else {
    const hasValidToken = authStore.token &&
                          authStore.isLoggedIn &&
                          authStore.userInfo &&
                          authStore.userInfo.role

    if (hasValidToken) {
      const requiredRole = to.meta?.role
      const userRole = authStore.userInfo.role

      if (requiredRole && requiredRole !== userRole) {
        if (userRole === 'admin') {
          next('/admin/dashboard')
        } else if (userRole === 'tutor') {
          next('/tutor/courses')
        } else {
          next('/')
        }
      } else {
        next()
      }
    } else {
      authStore.clearAuthData()
      next('/login')
    }
  }
})

export default router
