import { describe, expect, it, vi } from 'vitest'

vi.mock('@/pages/student/PublicCourseCatalog.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/pages/student/PublicCourseDetail.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/pages/auth/Login.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/layouts/TrainingLayout.vue', () => ({ default: { template: '<router-view />' } }))

const auth = vi.hoisted(() => ({
  initialize: async () => undefined,
  isLoggedIn: false,
  userInfo: null as { role: 'hrwai_user' } | null,
  token: '',
  clearAuthData: () => undefined
}))

vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))

vi.mock('@/stores/credential', () => ({
  useCredentialStore: () => ({ initialized: true, current: null })
}))

vi.mock('@/utils/subdomain', async importOriginal => {
  const actual = await importOriginal<typeof import('@/utils/subdomain')>()
  return { ...actual, getSubdomain: () => 'training', isIpDirectMode: () => false }
})

import router from '../index'

describe('公开课程入口', () => {
  it('匿名访问课程目录时进入登录页并保留原筛选', async () => {
    await router.push('/')
    expect(router.currentRoute.value.name).toBe('Login')
    expect(router.currentRoute.value.query.redirect).toBe('/training/courses')

    await router.push('/training/courses?category_id=4&filter=hot')
    expect(router.currentRoute.value.name).toBe('Login')
    expect(router.currentRoute.value.query.redirect).toBe('/training/courses?category_id=4&filter=hot')
  })

  it('旧课程链接仍可匿名进入独立详情', async () => {
    await router.push('/training/courses?course_id=17')
    expect(router.currentRoute.value.fullPath).toBe('/training/course/17')
    expect(router.currentRoute.value.name).toBe('PublicCourseDetail')
  })

  it('学员登录后进入课程目录', async () => {
    Object.assign(auth, { isLoggedIn: true, token: 'student-token', userInfo: { role: 'hrwai_user' } })
    try {
      await router.push('/training/courses?category_id=4')
      expect(router.currentRoute.value.name).toBe('CourseList')
      expect(router.currentRoute.value.query.category_id).toBe('4')
    } finally {
      Object.assign(auth, { isLoggedIn: false, token: '', userInfo: null })
    }
  })

  it('匿名访问章节时仅保留合法站内章节回跳', async () => {
    await router.push('/training/course/17/chapter/22?extra=discarded')
    expect(router.currentRoute.value.name).toBe('Login')
    expect(router.currentRoute.value.query.redirect).toBe('/training/course/17/chapter/22')
  })
})
