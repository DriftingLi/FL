// 导师端「我的课程」冒烟测试：双卡片导航渲染、课程卡片渲染、筛选触发列表请求。
// seam：组件层，mock API 层。计数联动语义已收敛至 useCourseCatalog 接口测试。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/tutor', () => ({
  tutorApi: {
    getCourses: vi.fn()
  }
}))

vi.mock('@/api/training', () => ({
  trainingApi: {
    getCatalogTree: vi.fn()
  }
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() })
}))

vi.mock('@/composables/useLazyLoad', () => ({
  vLazy: {}
}))

import { tutorApi } from '@/api/tutor'
import { trainingApi } from '@/api/training'
import type { CatalogDirectionNode, CatalogLevelNode } from '@/api/training'
import type { CourseDTO } from '@/api/tutor'
import TutorCourses from '../TutorCourses.vue'

// 生成 DTO 的最小测试夹具：只填测试关心的字段，其余取 DTO 零值
// （注解成为唯一事实源后，手写时代「只给两个字段」的 fixture 不再合法）。
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
function levelNodeOf(levelId: number, name: string, sortOrder: number, courses: CourseDTO[]): CatalogLevelNode {
  return { code: '', created_at: '', description: '', level_id: levelId, name, sort_order: sortOrder, status: 1, courses }
}
function specialtyOf(specialtyId: number, name: string, levels: CatalogLevelNode[]): CatalogDirectionNode {
  return { code: '', created_at: '', description: '', levels, name, sort_order: 0, specialty_id: specialtyId, status: 1 }
}

function mountPage() {
  return mount(TutorCourses, {
    global: { plugins: [epLite()] }
  })
}

beforeEach(() => {
  vi.mocked(tutorApi.getCourses).mockResolvedValue({
    total: 2,
    page: 1,
    pages: 1,
    courses: [
      courseOf(1, '液压系统原理与维护', { specialty_id: 2, level_id: 2, chapter_count: 7 }),
      courseOf(2, '叉车基础知识概述', { specialty_id: 2, level_id: 1, chapter_count: 6 })
    ]
  })
  vi.mocked(trainingApi.getCatalogTree).mockResolvedValue({
    specialties: [
      specialtyOf(2, '维修', [
        levelNodeOf(1, '入门', 0, [courseOf(2, '叉车基础知识概述')]),
        levelNodeOf(2, '进阶', 0, [courseOf(1, '液压系统原理与维护')])
      ])
    ]
  })
})

describe('TutorCourses 导师端我的课程', () => {
  it('渲染方向/等级双卡片导航与课程卡片', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const navNames = wrapper.findAll('.cc-nav-name').map(n => n.text())
    expect(navNames).toContain('全部课程')
    expect(navNames).toContain('维修')
    expect(navNames).toContain('全部等级')
    expect(navNames).toContain('入门')
    expect(navNames).toContain('进阶')

    const cards = wrapper.findAll('.cc-card')
    expect(cards.length).toBe(2)
    expect(cards[0].text()).toContain('液压系统原理与维护')
    expect(cards[0].text()).toContain('7 个章节')
  })

  it('点击等级导航项后按等级重新请求列表', async () => {
    const wrapper = mountPage()
    await flushPromises()

    await wrapper.findAll('.cc-filter-card')[1].findAll('.cc-nav-item')[1].trigger('click')
    await flushPromises()

    expect(tutorApi.getCourses).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 12,
      level_id: 1
    })
  })

  it('点击方向导航项后按方向重新请求列表', async () => {
    const wrapper = mountPage()
    await flushPromises()

    await wrapper.findAll('.cc-filter-card')[0].findAll('.cc-nav-item')[1].trigger('click')
    await flushPromises()

    expect(tutorApi.getCourses).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 12,
      specialty_id: 2
    })
  })
})
