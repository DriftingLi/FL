// 导师端课程认领交互（#2596 D2/D5）。
//
// seam：组件层 + mock API。锁三件在规格里点名过的事实：
//  ① 未认领 → 卡上出现「认领课程」，点它发一次认领请求并**就地**更新这张卡（不整页重载）；
//  ② 他人已认领 → 显示「N 位讲师在编」但仍可加入共认（共认是正常形态，不拦人）；
//  ③ 已认领 → 显示「已认领」标签与「管理章节」入口，不再显示认领按钮。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/tutor', () => ({
  tutorApi: {
    getCourses: vi.fn(),
    claimCourse: vi.fn()
  }
}))

vi.mock('@/api/training', () => ({
  trainingApi: {
    getCatalogTree: vi.fn()
  }
}))

const pushSpy = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushSpy })
}))

vi.mock('@/composables/useLazyLoad', () => ({
  vLazy: {}
}))

import { tutorApi } from '@/api/tutor'
import { trainingApi } from '@/api/training'
import type { CourseDTO } from '@/api/tutor'
import TutorCourses from '../TutorCourses.vue'

function courseOf(courseId: number, name: string, over: Partial<CourseDTO> = {}): CourseDTO {
  return {
    certificate_name: '',
    certificate_template_id: null,
    course_id: courseId,
    cover_image: '',
    created_at: '',
    credential_id: null,
    description: '',
    duration: 0,
    is_featured: false,
    is_hot: false,
    level_id: null,
    name,
    practice_hours: 0,
    sort_order: 0,
    specialty_id: null,
    status: 1,
    theory_hours: 0,
    ...over
  }
}

function setCourses(...courses: CourseDTO[]) {
  vi.mocked(tutorApi.getCourses).mockResolvedValue({
    total: courses.length,
    page: 1,
    pages: 1,
    courses
  })
}

beforeEach(() => {
  pushSpy.mockReset()
  vi.mocked(tutorApi.claimCourse).mockReset()
  vi.mocked(trainingApi.getCatalogTree).mockResolvedValue({
    specialties: [
      {
        code: '',
        created_at: '',
        description: '',
        levels: [],
        name: '维修',
        sort_order: 0,
        specialty_id: 2,
        status: 1
      }
    ]
  })
})

async function mountPage() {
  const wrapper = mount(TutorCourses, { global: { plugins: [epLite()] } })
  await flushPromises()
  return wrapper
}

describe('TutorCourses 课程认领', () => {
  it('未认领课程显示「认领课程」，点击后就地更新该卡且不整页重载', async () => {
    setCourses(courseOf(1, '未认领课程', { claim_count: 0, claimed_by_me: false }))
    vi.mocked(tutorApi.claimCourse).mockResolvedValue({ course_id: 1, claimed: true })
    const wrapper = await mountPage()

    const card = wrapper.findAll('.cc-card')[0]
    expect(card.text()).toContain('认领课程')
    expect(card.text()).not.toContain('已认领')

    const getCoursesCallsBefore = vi.mocked(tutorApi.getCourses).mock.calls.length
    const claimBtn = card.findAll('button').find(b => b.text().includes('认领课程'))
    expect(claimBtn, '未认领卡上应有认领按钮').toBeTruthy()
    await claimBtn!.trigger('click')
    await flushPromises()

    expect(tutorApi.claimCourse).toHaveBeenCalledWith(1)
    // 就地更新：这张卡变成已认领，且没有整页重载列表
    const after = wrapper.findAll('.cc-card')[0]
    expect(after.text()).toContain('已认领')
    expect(after.text()).toContain('管理章节')
    expect(vi.mocked(tutorApi.getCourses).mock.calls.length).toBe(getCoursesCallsBefore)
  })

  it('他人已认领的课程显示「N 位讲师在编」且仍可加入共认', async () => {
    setCourses(courseOf(2, '别人在编课程', { claim_count: 2, claimed_by_me: false }))
    vi.mocked(tutorApi.claimCourse).mockResolvedValue({ course_id: 2, claimed: true })
    const wrapper = await mountPage()

    const card = wrapper.findAll('.cc-card')[0]
    expect(card.text()).toContain('2 位讲师在编')
    // 共认是正常形态：仍显示认领按钮（不拦人）
    expect(card.findAll('button').some(b => b.text().includes('认领课程'))).toBe(true)
  })

  it('已认领课程显示标签与「管理章节」，点击进入章节管理', async () => {
    setCourses(courseOf(3, '我在编课程', { claim_count: 1, claimed_by_me: true }))
    const wrapper = await mountPage()

    const card = wrapper.findAll('.cc-card')[0]
    expect(card.text()).toContain('已认领')
    expect(card.findAll('button').some(b => b.text().includes('认领课程'))).toBe(false)

    const manageBtn = card.findAll('button').find(b => b.text().includes('管理章节'))
    expect(manageBtn, '已认领卡应有管理章节入口').toBeTruthy()
    await manageBtn!.trigger('click')
    expect(pushSpy).toHaveBeenCalled()
  })

  it('认领按钮点击不冒泡到卡片本身（不会误跳章节页）', async () => {
    setCourses(courseOf(4, '点击测试课程', { claim_count: 0, claimed_by_me: false }))
    vi.mocked(tutorApi.claimCourse).mockResolvedValue({ course_id: 4, claimed: true })
    const wrapper = await mountPage()

    const claimBtn = wrapper.findAll('.cc-card')[0].findAll('button').find(b => b.text().includes('认领课程'))
    await claimBtn!.trigger('click')
    await flushPromises()

    expect(pushSpy).not.toHaveBeenCalled()
  })
})
