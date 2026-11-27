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
    getCatalogTree: vi.fn(),
    getCategories: vi.fn()
  }
}))

vi.mock('@/api/credential', () => ({
  credentialApi: { listCredentials: vi.fn() }
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() })
}))

vi.mock('@/composables/useLazyLoad', () => ({
  vLazy: {}
}))

import { tutorApi } from '@/api/tutor'
import { trainingApi } from '@/api/training'
import { credentialApi } from '@/api/credential'
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
    course_type: 'illustrated',
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
  vi.mocked(trainingApi.getCategories).mockResolvedValue({
    categories: [
      { id: 11, name: '叉车安全', status: 1 },
      { id: 12, name: '已停用分类', status: 0 }
    ]
  })
  vi.mocked(credentialApi.listCredentials).mockResolvedValue({ credentials: [] })
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
      category_id: 'all',
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
      category_id: 'all',
      specialty_id: 2
    })
  })

  it('分类筛选只列启用分类，并把分类 ID 与遗留桶传给后端', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const select = wrapper.findAllComponents({ name: 'UiSelect' })
      .find(item => item.props('placeholder') === '全部培训分类')!
    const options = select.props('options') as { label: string; value: string | number }[]
    expect(options.map(option => option.label)).toEqual(['全部培训分类', '叉车安全', '未分类/分类失效'])
    expect(options.map(option => option.value)).not.toContain(12)

    await select.vm.$emit('update:modelValue', 11)
    await select.vm.$emit('change', 11)
    await flushPromises()
    expect(tutorApi.getCourses).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 12,
      category_id: 11
    })

    await select.vm.$emit('update:modelValue', 'legacy')
    await select.vm.$emit('change', 'legacy')
    await flushPromises()
    expect(tutorApi.getCourses).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 12,
      category_id: 'legacy'
    })
  })

  it('课程卡并列展示 canonical 分类、课程类型、发布状态与全部公开阻断原因', async () => {
    vi.mocked(tutorApi.getCourses).mockResolvedValue({
      total: 1,
      page: 1,
      pages: 1,
      courses: [courseOf(1, '安全作业课程', {
        category: { id: 11, name: '叉车安全', status: 1 },
        course_type: 'video',
        status: 1,
        public_state: {
          category: { id: 11, name: '叉车安全', status: 1 },
          public_visibility: true,
          eligibility: {
            eligible: false,
            reasons: ['category_disabled', 'credential_disabled']
          }
        },
        public_eligibility: { eligible: true, reasons: [] }
      })]
    })
    const wrapper = mountPage()
    await flushPromises()

    const card = wrapper.findAll('.cc-card')[0]
    expect(card.text()).toContain('叉车安全')
    expect(card.text()).toContain('视频课程')
    expect(card.text()).toContain('已发布')
    expect(card.text()).toContain('未公开')
    expect(card.text()).toContain('培训分类已停用')
    expect(card.text()).toContain('映射证件已停用')
    expect(card.text()).not.toContain('已公开')
  })

  it('无 canonical 分类的课程回显到遗留桶文案', async () => {
    vi.mocked(tutorApi.getCourses).mockResolvedValue({
      total: 1,
      page: 1,
      pages: 1,
      courses: [courseOf(1, '历史课程', {
        public_state: {
          public_visibility: true,
          eligibility: { eligible: false, reasons: ['category_missing'] }
        }
      })]
    })
    const wrapper = mountPage()
    await flushPromises()

    expect(wrapper.findAll('.cc-card')[0].text()).toContain('未分类/分类失效')
    expect(wrapper.text()).toContain('未设置培训分类')
  })
})
