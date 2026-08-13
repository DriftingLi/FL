// 导师端「我的课程」冒烟测试：双卡片导航渲染、课程卡片渲染、筛选触发列表请求。
// seam：组件层，mock API 层。计数联动语义已收敛至 useCourseCatalog 接口测试。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

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
import TutorCourses from '../TutorCourses.vue'

function mountPage() {
  return mount(TutorCourses, {
    global: { plugins: [ElementPlus] }
  })
}

beforeEach(() => {
  vi.mocked(tutorApi.getCourses).mockResolvedValue({
    total: 2,
    courses: [
      { course_id: 1, name: '液压系统原理与维护', specialty_id: 2, level_id: 2, chapter_count: 7 },
      { course_id: 2, name: '叉车基础知识概述', specialty_id: 2, level_id: 1, chapter_count: 6 }
    ]
  })
  vi.mocked(trainingApi.getCatalogTree).mockResolvedValue({
    specialties: [
      {
        specialty_id: 2,
        name: '维修',
        levels: [
          { level_id: 1, name: '入门', courses: [{ course_id: 2, name: '叉车基础知识概述' }] },
          { level_id: 2, name: '进阶', courses: [{ course_id: 1, name: '液压系统原理与维护' }] }
        ]
      }
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
