// 学员端课程中心（左右分栏）冒烟测试：双卡片导航渲染、课程卡片渲染、筛选触发列表请求。
// seam：组件层，mock API 层（不依赖真实后端）。计数联动语义已收敛至 useCourseCatalog 接口测试。
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
    getCatalogTree: vi.fn()
  }
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  // CourseList 读取 query.course_id（搜索/收藏跳转自动打开详情）
  useRoute: () => ({ query: {} })
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
})

describe('CourseList 左右分栏课程中心', () => {
  it('渲染方向/等级双卡片导航（全部+各方向带课程数）', async () => {
    const wrapper = mountPage()
    await flushPromises()

    // Spec #326 Q10：顶部 Tab 顺序 热门/精品/所有（所有最右），默认热门时隐藏 FacetCard
    expect(wrapper.find('.cc-tabs').exists()).toBe(true)
    expect(wrapper.findAll('.cc-filter-card').length).toBe(0)

    // 切到「所有」后展示双卡片导航
    ;(wrapper.vm as any).activeTab = 'all'
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

    // 切到「所有」以展示 FacetCard，再点击等级「入门」
    ;(wrapper.vm as any).activeTab = 'all'
    await flushPromises()
    await wrapper.findAll('.cc-filter-card')[1].findAll('.cc-nav-item')[1].trigger('click')
    await flushPromises()

    expect(courseApi.getCourses).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 12,
      filter: 'all',
      level_id: 1
    })
  })

  it('点击方向导航项后按方向重新请求列表', async () => {
    const wrapper = mountPage()
    await flushPromises()

    ;(wrapper.vm as any).activeTab = 'all'
    await flushPromises()
    await wrapper.findAll('.cc-filter-card')[0].findAll('.cc-nav-item')[1].trigger('click')
    await flushPromises()

    expect(courseApi.getCourses).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 12,
      filter: 'all',
      specialty_id: 2
    })
  })
})
