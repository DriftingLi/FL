// 学员端课程中心（左右分栏）组件测试：方向导航/等级筛选/章节数渲染。
// seam：组件层，mock API 层（不依赖真实后端）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('@/api/course', () => ({
  courseApi: {
    getCourses: vi.fn(),
    getCourseDetail: vi.fn()
  }
}))

vi.mock('@/api/training', () => ({
  trainingApi: {
    getCatalogTree: vi.fn(),
    getLevels: vi.fn()
  }
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() })
}))

vi.mock('@/composables/useLazyLoad', () => ({
  vLazy: {}
}))

import { courseApi } from '@/api/course'
import { trainingApi } from '@/api/training'
import CourseList from '../CourseList.vue'

function mountPage() {
  return mount(CourseList, {
    global: { plugins: [ElementPlus] }
  })
}

beforeEach(() => {
  vi.mocked(courseApi.getCourses).mockResolvedValue({
    total: 2,
    courses: [
      {
        course_id: 1,
        name: '液压系统原理与维护',
        description: '液压传动原理',
        specialty_id: 2,
        level_id: 2,
        chapter_count: 7,
        theory_hours: 24,
        practice_hours: 16,
        certificate_name: '叉车维修技能培训合格证书'
      },
      {
        course_id: 2,
        name: '叉车基础知识概述',
        description: '入门知识',
        specialty_id: 2,
        level_id: 1,
        chapter_count: 6,
        theory_hours: 16,
        practice_hours: 8
      }
    ]
  })
  vi.mocked(courseApi.getCourseDetail).mockResolvedValue({
    course_info: { course_id: 1, name: '液压系统原理与维护' },
    chapters: []
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
  vi.mocked(trainingApi.getLevels).mockResolvedValue({
    levels: [
      { level_id: 1, name: '入门' },
      { level_id: 2, name: '进阶' }
    ]
  })
})

describe('CourseList 左右分栏课程中心', () => {
  it('渲染方向/等级双卡片导航（全部+各方向带课程数）', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const navNames = wrapper.findAll('.cc-nav-name').map(n => n.text())
    expect(navNames).toContain('全部课程')
    expect(navNames).toContain('维修')
    expect(navNames).toContain('全部等级')
    expect(navNames).toContain('入门')
    expect(navNames).toContain('进阶')
    // 侧栏第一张卡片（专业方向）首个导航项计数 = 树内课程总数
    expect(wrapper.find('.cc-filter-card .cc-nav-count').text()).toBe('2')
  })

  it('课程卡片渲染章节数/学时/证书标签', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const cards = wrapper.findAll('.cc-card')
    expect(cards.length).toBe(2)
    expect(cards[0].text()).toContain('7 章节')
    expect(cards[0].text()).toContain('理论24学时 · 实操16学时')
    expect(cards[0].text()).toContain('叉车维修技能培训合格证书')
  })

  it('点击等级导航项后按等级重新请求列表', async () => {
    const wrapper = mountPage()
    await flushPromises()

    // 第二张侧栏卡片（课程等级）中第 2 项 = 「入门」
    await wrapper.findAll('.cc-filter-card')[1].findAll('.cc-nav-item')[1].trigger('click')
    await flushPromises()

    expect(courseApi.getCourses).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 12,
      level_id: 1
    })
  })

  it('选中等级后方向卡片计数反向联动（只数该等级课程）', async () => {
    const wrapper = mountPage()
    await flushPromises()

    // 树内：维修方向 入门 1 门 + 进阶 1 门 → 选中「入门」后各计数收敛为 1
    await wrapper.findAll('.cc-filter-card')[1].findAll('.cc-nav-item')[1].trigger('click')
    await flushPromises()

    const navNames = wrapper.findAll('.cc-nav-name').map(n => n.text())
    expect(navNames[0]).toBe('全部课程')
    // 「全部课程」计数随等级筛选变为 1
    const counts = wrapper.findAll('.cc-nav-count').map(n => n.text())
    expect(counts[0]).toBe('1')
    // 「维修」方向计数随等级筛选变为 1
    expect(counts[1]).toBe('1')
  })
})
