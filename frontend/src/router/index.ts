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
        component: () => import('@/pages/student/Home.vue')
      },
      {
        path: 'courses',
        name: 'CourseList',
        component: () => import('@/pages/student/CourseList.vue')
      },
      {
        path: 'course/:id',
        name: 'CourseDetail',
        component: () => import('@/pages/student/CourseDetail.vue')
      },
      {
        path: 'course/:courseId/chapter/:chapterId',
        name: 'ChapterView',
        component: () => import('@/pages/student/ChapterView.vue')
      },
      {
        path: 'exam/:courseId',
        name: 'Exam',
        component: () => import('@/pages/student/Exam.vue')
      },
      {
        path: 'question-bank',
        name: 'QuestionBank',
        component: () => import('@/pages/student/QuestionBank.vue')
      },
      {
        path: 'practice-free',
        name: 'PracticeFree',
        component: () => import('@/pages/student/PracticeFree.vue')
      },
      {
        path: 'knowledge-practice',
        name: 'KnowledgePractice',
        component: () => import('@/pages/student/KnowledgePractice.vue')
      },
      {
        path: 'practice-stats',
        name: 'PracticeStats',
        component: () => import('@/pages/student/PracticeStats.vue')
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
        path: 'ai-generate',
        name: 'AIGenerate',
        component: () => import('@/pages/student/AIAssistant.vue')
      },
      {
        path: 'practice',
        name: 'Practice',
        component: () => import('@/pages/student/Practice.vue')
      },
      {
        path: 'profile',
        name: 'Profile',
        component: () => import('@/pages/student/Profile.vue')
      },
      {
        path: 'valuation',
        name: 'ValuationHome',
        component: () => import('@/pages/student/valuation/ValuationHome.vue')
      },
      {
        path: 'valuation/input',
        name: 'ValuationInput',
        component: () => import('@/pages/student/valuation/ValuationInputView.vue')
      },
      {
        path: 'valuation/result',
        name: 'ValuationResult',
        component: () => import('@/pages/student/valuation/ValuationResultView.vue')
      },
      {
        path: 'valuation/report/:id',
        name: 'ValuationReport',
        component: () => import('@/pages/student/valuation/ValuationReportView.vue')
      },
      {
        path: 'valuation/battery',
        name: 'ValuationBatteryInput',
        component: () => import('@/pages/student/valuation/BatteryInputView.vue')
      },
      {
        path: 'valuation/battery/result',
        name: 'ValuationBatteryResult',
        component: () => import('@/pages/student/valuation/BatteryResultView.vue')
      },
      {
        path: 'valuation/history',
        name: 'ValuationHistory',
        component: () => import('@/pages/student/valuation/ValuationHistoryView.vue')
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
        component: () => import('@/pages/admin/Dashboard.vue')
      },
      {
        path: 'students',
        name: 'StudentManage',
        component: () => import('@/pages/admin/StudentManage.vue')
      },
      {
        path: 'courses',
        name: 'CourseManage',
        component: () => import('@/pages/admin/CourseManage.vue')
      },
      {
        path: 'statistics',
        name: 'Statistics',
        component: () => import('@/pages/admin/Statistics.vue')
      },
      {
        path: 'content-generate',
        name: 'ContentGenerate',
        component: () => import('@/pages/admin/ContentGenerate.vue')
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
        path: 'admins',
        name: 'AdminManage',
        component: () => import('@/pages/admin/AdminManage.vue')
      },
      {
        path: 'valuation-config',
        name: 'ValuationConfigManage',
        component: () => import('@/pages/admin/ValuationConfigManage.vue')
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
        component: () => import('@/pages/tutor/TutorCourses.vue')
      },
      {
        path: 'course/:id/chapters',
        name: 'TutorChapterManage',
        component: () => import('@/pages/tutor/ChapterManage.vue')
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
