// 管理端课程目录：服务端分页、legacy 桶、公开状态与分类维护。
// seam：组件层，mock API 层。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ElMessage } from 'element-plus'
import { epLite } from '@/test/element-lite'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'

vi.mock('@/api/admin', () => ({
  adminApi: {
    getCourses: vi.fn(),
    getCourseDetail: vi.fn(),
    createCourse: vi.fn(),
    updateCourse: vi.fn(),
    setPublicVisibility: vi.fn(),
    deleteCourse: vi.fn(),
    swapCourse: vi.fn(),
    getCourseSortNeighbors: vi.fn(),
    createChapter: vi.fn(),
    updateChapter: vi.fn(),
    deleteChapter: vi.fn()
  }
}))

vi.mock('@/api/credential', () => ({
  credentialApi: { listAdminCredentials: vi.fn() }
}))

vi.mock('@/api/training', () => ({
  trainingApi: {
    getAdminCategories: vi.fn(),
    getCertificateTemplates: vi.fn(),
    createTrainingCategory: vi.fn(),
    updateTrainingCategory: vi.fn()
  }
}))

import { adminApi, type AdminCourseItem, type AdminCourseDetail } from '@/api/admin'
import { credentialApi } from '@/api/credential'
import { trainingApi } from '@/api/training'
import CourseCatalog from '../CourseCatalog.vue'

const successSpy = vi.spyOn(ElMessage, 'success').mockImplementation(() => undefined as never)

function courseDTO(over: Partial<AdminCourseItem> = {}): AdminCourseItem {
  return {
    certificate_template_id: null,
    course_id: 0,
    course_type: 'illustrated',
    cover_image: '',
    created_at: '2026-09-01T10:00:00+08:00',
    credential_id: null,
    description: '',
    duration: 0,
    is_featured: false,
    is_hot: false,
    level_id: null,
    name: '',
    practice_hours: 0,
    sort_order: 0,
    specialty_id: null,
    status: 1,
    theory_hours: 0,
    ...over
  }
}

function courseDetailDTO(over: Partial<AdminCourseDetail> = {}): AdminCourseDetail {
  return { ...courseDTO(), chapters: [], ...over }
}

const courses = [
  courseDTO({
    course_id: 1,
    name: '液压系统',
    category_id: 9,
    public_state: {
      category: { id: 9, name: '维修', status: 1 },
      public_visibility: true,
      eligibility: { eligible: true, reasons: [] }
    }
  }),
  courseDTO({
    course_id: 2,
    name: '未挂载遗留课程',
    status: 0,
    public_state: {
      public_visibility: false,
      eligibility: { eligible: true, reasons: [] }
    },
    public_eligibility: { eligible: false, reasons: ['manually_hidden'] }
  }),
  courseDTO({
    course_id: 3,
    name: '草稿课程',
    category_id: 9,
    status: 0,
    public_state: {
      category: { id: 9, name: '维修', status: 1 },
      public_visibility: true,
      eligibility: { eligible: false, reasons: ['not_published'] }
    }
  }),
  courseDTO({
    course_id: 4,
    name: '停用分类课程',
    category_id: 8,
    public_state: {
      category: { id: 8, name: '旧分类', status: 0 },
      public_visibility: true,
      eligibility: { eligible: false, reasons: ['category_disabled'] }
    }
  }),
  courseDTO({
    course_id: 5,
    name: '来源排除课程',
    category_id: 9,
    public_state: {
      category: { id: 9, name: '维修', status: 1 },
      public_visibility: true,
      eligibility: { eligible: false, reasons: ['source_tag_excluded'] }
    }
  })
]

function mountPage() {
  return mount(CourseCatalog, { global: { plugins: [epLite()] } })
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(adminApi.getCourseSortNeighbors).mockResolvedValue({ previous_id: null, next_id: null })
  vi.mocked(adminApi.swapCourse).mockResolvedValue(null)
  vi.mocked(adminApi.getCourses).mockImplementation(async params => {
    const facets = { total: 5, legacy: 2, categories: [{ category_id: 9, total: 3 }, { category_id: 8, total: 1 }] }
    if (params.page_size === 1) {
      const counts: Record<string, number> = { all: 5, legacy: 2, '8': 1, '9': 3 }
      const total = counts[String(params.category_id ?? 'all')] ?? 0
      return { items: [], total, facets }
    }
    const category = params.category_id ?? 'all'
    const filtered = courses.filter(course => {
      if (category === 'legacy') return course.course_id === 2 || course.course_id === 4
      if (category !== 'all' && course.category_id !== category) return false
      if (params.keyword && !course.name.includes(params.keyword)) return false
      if (params.status != null && course.status !== params.status) return false
      if (params.filter === 'hot' && !course.is_hot) return false
      if (params.filter === 'featured' && !course.is_featured) return false
      return true
    })
    return { items: filtered, total: filtered.length, facets }
  })
  vi.mocked(adminApi.getCourseDetail).mockResolvedValue(courseDetailDTO({
    course_id: 1,
    name: '液压系统',
    category_id: 9,
    public_state: courses[0].public_state
  }))
  vi.mocked(adminApi.updateCourse).mockResolvedValue(courseDTO())
  vi.mocked(trainingApi.getAdminCategories).mockResolvedValue({
    categories: [
      { id: 9, name: '维修', legacy_credential_id: 1, status: 1 },
      { id: 8, name: '旧分类', legacy_credential_id: 2, status: 0 }
    ]
  })
  vi.mocked(credentialApi.listAdminCredentials).mockResolvedValue({
    credentials: [{
      id: 1,
      code: 'forklift',
      name: '叉车司机',
      category: 'special_operation',
      level: null,
      description: '',
      sort_order: 1,
      status: 1,
      created_at: '',
      updated_at: ''
    }]
  })
  vi.mocked(trainingApi.getCertificateTemplates).mockResolvedValue({ certificate_templates: [] })
})

describe('CourseCatalog 服务端分页与公开状态', () => {
  it('课程加载失败显示错误态，重试成功后恢复表格', async () => {
    vi.mocked(adminApi.getCourses).mockRejectedValueOnce(new Error('课程加载失败'))
    const wrapper = mountPage()
    await flushPromises()

    const asyncSection = wrapper.getComponent(UiAsyncSection)
    expect(asyncSection.props()).toMatchObject({
      loading: false,
      error: true,
      retrying: false,
      skeleton: false,
      errorTitle: '课程加载失败',
      errorDescription: '网络或服务端异常，可重试'
    })
    const errorState = asyncSection.getComponent(UiErrorState)
    expect(errorState.text()).toContain('课程加载失败')
    expect(wrapper.findComponent({ name: 'ElTable' }).exists()).toBe(false)

    await errorState.get('button').trigger('click')
    await flushPromises()

    expect(wrapper.findComponent(UiErrorState).exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'ElTable' }).exists()).toBe(true)
    expect(vi.mocked(adminApi.getCourses).mock.calls.filter(([params]) => params.page_size === 10)).toHaveLength(2)
  })

  it('首屏只请求当前页，分类计数由后端分页信封提供', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const calls = vi.mocked(adminApi.getCourses).mock.calls.map(([params]) => params)
    expect(calls).toContainEqual(expect.objectContaining({ page: 1, page_size: 10, category_id: 'all' }))
    expect(calls.some(params => params.page_size === 500)).toBe(false)
    expect(calls.filter(params => params.page_size === 10)).toHaveLength(1)
    expect(calls.some(params => params.page_size === 1)).toBe(false)

    const nav = wrapper.findAll('.cc-nav-count')
    expect(nav[0]?.text()).toBe('5')
    expect(nav[1]?.text()).toBe('3')
    expect(nav[2]?.text()).toBe('2')
  })

  it('选择 legacy 桶由后端筛选并返回对应页', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const legacy = wrapper.findAll('.cc-nav-item').find(item => item.text().includes('未分类/分类失效'))!
    await legacy.trigger('click')
    await flushPromises()

    expect(adminApi.getCourses).toHaveBeenLastCalledWith(expect.objectContaining({
      page: 1,
      page_size: 10,
      category_id: 'legacy'
    }))
    const rows = wrapper.findAll('.el-table__row').map(row => row.text())
    expect(rows).toHaveLength(2)
    expect(rows.some(text => text.includes('未挂载遗留课程'))).toBe(true)
    expect(rows.some(text => text.includes('停用分类课程'))).toBe(true)
  })

  it('跨页移动通过后端同分类全局邻居定位目标，并保留当前页刷新', async () => {
    vi.mocked(adminApi.getCourses).mockImplementation(async params => {
      const facets = { total: 2, legacy: 0, categories: [{ category_id: 9, total: 2 }] }
      if (params.page_size === 50) return { items: [courses[0], courses[2]], total: 2, facets }
      if (params.category_id === 9 && params.page === 2) return { items: [courses[2]], total: 2, facets }
      return { items: [courses[0]], total: 2, facets }
    })
    vi.mocked(adminApi.getCourseSortNeighbors).mockResolvedValue({ previous_id: null, next_id: 3 })
    const wrapper = mountPage()
    await flushPromises()

    const category = wrapper.findAll('.cc-nav-item').find(item => item.text().includes('维修'))!
    await category.trigger('click')
    await flushPromises()
    const row = wrapper.get('.el-table__row')
    await row.getComponent({ name: 'ElDropdown' }).vm.$emit('command', 'moveDown')
    await flushPromises()

    expect(adminApi.getCourseSortNeighbors).toHaveBeenCalledWith(1)
    expect(adminApi.swapCourse).toHaveBeenCalledWith(1, 3)
    expect(adminApi.getCourses).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, page_size: 10, category_id: 9 }))
  })

  it('状态筛选值传给后端', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const statusSelect = wrapper.findAllComponents({ name: 'ElSelect' })[1]!
    statusSelect.vm.$emit('update:modelValue', 0)
    statusSelect.vm.$emit('change', 0)
    await flushPromises()

    expect(adminApi.getCourses).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, page_size: 10, status: 0 }))
    const rows = wrapper.findAll('.el-table__row').map(row => row.text())
    expect(rows).toHaveLength(2)
    expect(rows.every(text => text.includes('未发布'))).toBe(true)
  })

  it('公开开关只读 public_state.public_visibility，不从旧 reasons 反推', async () => {
    vi.mocked(adminApi.getCourseDetail).mockResolvedValue(courseDetailDTO({
      course_id: 2,
      public_state: courses[1].public_state,
      public_eligibility: courses[1].public_eligibility
    }))
    vi.mocked(adminApi.setPublicVisibility).mockResolvedValue({ eligible: false, reasons: ['not_published'] })
    const wrapper = mountPage()
    await flushPromises()

    const row = wrapper.findAll('.el-table__row').find(item => item.text().includes('未挂载遗留课程'))!
    expect(row.text()).toContain('已公开')
    await row.get('.cc-cell-name').trigger('click')
    await flushPromises()

    const drawer = wrapper.getComponent({ name: 'ElDrawer' })
    const toggle = drawer.getComponent({ name: 'ElSwitch' })
    expect(toggle.props('modelValue')).toBe(false)
    toggle.vm.$emit('change', true)
    await flushPromises()

    expect(adminApi.setPublicVisibility).toHaveBeenCalledWith(2, true)
    expect(toggle.props('modelValue')).toBe(true)
    expect(row.text()).toContain('未公开')
  })

  it('空分类课程可直接发布，不触发前端硬门', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const row = wrapper.findAll('.el-table__row').find(item => item.text().includes('未挂载遗留课程'))!
    await row.findComponent({ name: 'ElDropdown' }).vm.$emit('command', 'toggle')
    await flushPromises()

    expect(adminApi.updateCourse).toHaveBeenCalledWith(2, { status: 1 })
    expect(successSpy).toHaveBeenCalled()
  })
})
