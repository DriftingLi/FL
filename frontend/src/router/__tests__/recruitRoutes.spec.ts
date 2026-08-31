import { describe, it, expect } from 'vitest'
import router from '../index'

describe('recruit routes', () => {
  it('存在 /recruit 布局路由且守卫要求 recruiter 角色', () => {
    const all = router.getRoutes()
    const rec = all.find(r => r.path === '/recruit' && r.meta?.requiresAuth === true)
    expect(rec).toBeDefined()
    expect(rec!.meta.requiresAuth).toBe(true)
    expect(rec!.meta.role).toBe('recruiter')
  })

  it('子路由包含 Dashboard / Resumes / ResumeDetail', () => {
    const names = router.getRoutes().map(r => r.name)
    expect(names).toContain('RecruitDashboard')
    expect(names).toContain('RecruitResumes')
    expect(names).toContain('RecruitResumeDetail')
  })

  it('/recruit/resumes 详情页 activeRouteNames 正确', async () => {
    // 检查导航配置的 recruiter 是否包含 activeRouteNames
    const { roleNavigation } = await import('@/config/navigation')
    const nav = roleNavigation.recruiter
    const resumes = nav.find(i => i.routeName === 'RecruitResumes')
    expect(resumes).toBeDefined()
    expect(resumes!.activeRouteNames).toContain('RecruitResumeDetail')
  })
})
