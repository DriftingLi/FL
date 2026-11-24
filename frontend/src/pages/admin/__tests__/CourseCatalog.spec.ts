// 管理端课程目录测试：遗留桶、公开原因、空分类发布与分类维护。
// seam：组件层，mock API 层。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ElMessage } from 'element-plus'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/admin', () => ({
  adminApi: {
    getCourses: vi.fn(),
    getCourseDetail: vi.fn(),
    createCourse: vi.fn(),
    updateCourse: vi.fn(),
    deleteCourse: vi.fn(),
    swapCourse: vi.fn(),
    createChapter: vi.fn(),
    updateChapter: vi.fn(),
    deleteChapter: vi.fn()
  }
}))

vi.mock('@/api/credential', () => ({
  credentialApi: {
    listAdminCredentials: vi.fn()
  }
}))

vi.mock('@/api/training', () => ({
  trainingApi: {
    getAdminCategories: vi.fn(),
    getCertificateTemplates: vi.fn(),
    createDirection: vi.fn(),
    updateDirection: vi.fn(),
    swapDirection: vi.fn(),
    deleteDirection: vi.fn(),
    createLevel: vi.fn(),
    updateLevel: vi.fn(),
    deleteLevel: vi.fn(),
    createQuestionTag: vi.fn(),
    updateQuestionTag: vi.fn(),
    createCertificateTemplate: vi.fn(),
    updateCertificateTemplate: vi.fn(),
    deleteCertificateTemplate: vi.fn()
  }
}))

import { adminApi, type AdminCourseItem, type AdminCourseDetail } from '@/api/admin'
import { credentialApi } from '@/api/credential'
import { trainingApi } from '@/api/training'
import CourseCatalog from '../CourseCatalog.vue'
import UiSelect from '@/components/ui/UiSelect.vue'


const warnSpy = vi.spyOn(ElMessage, 'warning').mockImplementation(() => undefined as never)
const successSpy = vi.spyOn(ElMessage, 'success').mockImplementation(() => undefined as never)

// AdminCourseItem = 生成的 CourseDTO（注解产物，必填字段比旧手写类型多）：
// 用工厂补齐默认值，用例只写自己的差异面。
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

function mountPage() {
  return mount(CourseCatalog, {
    global: { plugins: [epLite()] }
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(adminApi.getCourses).mockResolvedValue({
    page: 1,
    pages: 1,
    total: 5,
    courses: [
      courseDTO({
        course_id: 1,
        name: '液压系统',
        category_id: 9,
        category: { id: 9, name: '维修', status: 1 },
        status: 1,
        chapter_count: 7,
        theory_hours: 24,
        practice_hours: 16,
        public_eligibility: { eligible: true, reasons: [] }
      }),
      courseDTO({
        course_id: 2,
        name: '未挂载遗留课程',
        specialty_id: null,
        level_id: null,
        status: 0,
        chapter_count: 2,
        public_eligibility: { eligible: false, reasons: ['not_published', 'category_missing'] }
      }),
      courseDTO({
        course_id: 3,
        name: '草稿课程',
        category_id: 9,
        status: 0,
        chapter_count: 0,
        public_eligibility: { eligible: false, reasons: ['not_published'] }
      }),
      courseDTO({
        course_id: 4,
        name: '停用分类课程',
        category_id: 8,
        category: { id: 8, name: '旧分类', status: 0 },
        status: 1,
        public_eligibility: { eligible: false, reasons: ['category_disabled'] }
      }),
      courseDTO({
        course_id: 5,
        name: '来源排除课程',
        category_id: 9,
        category: { id: 9, name: '维修', status: 1 },
        status: 1,
        public_eligibility: { eligible: false, reasons: ['source_tag_excluded'] }
      })
    ]
  })
  vi.mocked(adminApi.getCourseDetail).mockResolvedValue(courseDetailDTO({
    course_id: 1,
    name: '液压系统',
    category_id: 9
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
  vi.mocked(trainingApi.getCertificateTemplates).mockResolvedValue({
    certificate_templates: []
  })
})

describe('CourseCatalog 分类与公开状态', () => {
  it('筛选只列启用分类，遗留桶仅包含分类异常，维护入口保留停用分类', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const navNames = wrapper.findAll('.cc-nav-item').map(n => n.text())
    expect(navNames.some(t => t.includes('维修'))).toBe(true)
    expect(navNames.some(t => t.includes('旧分类'))).toBe(false)

    const legacy = navNames.find(t => t.includes('未分类/分类失效'))
    expect(legacy).toBeTruthy()
    expect(legacy).toContain('2')

    const management = wrapper.findAllComponents(UiSelect)
      .find(select => select.props('placeholder') === '维护全部分类')
    expect(management?.props('options')).toEqual(expect.arrayContaining([
      expect.objectContaining({ label: '旧分类（停用）', value: 8 })
    ]))
  })

  it('遗留桶不把仅未发布或来源排除的课程算入', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const legacy = wrapper.findAll('.cc-nav-item').find(item => item.text().includes('未分类/分类失效'))
    await legacy!.trigger('click')
    await flushPromises()

    const rows = wrapper.findAll('.el-table__row').map(row => row.text())
    expect(rows).toHaveLength(2)
    expect(rows.some(text => text.includes('未挂载遗留课程'))).toBe(true)
    expect(rows.some(text => text.includes('停用分类课程'))).toBe(true)
    expect(rows.some(text => text.includes('草稿课程'))).toBe(false)
    expect(rows.some(text => text.includes('来源排除课程'))).toBe(false)
  })

  it('列表展示 canonical 分类、课程类型、发布状态与全部公开原因', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const legacyRow = wrapper.findAll('.el-table__row').find(row => row.text().includes('未挂载遗留课程'))!
    expect(legacyRow.text()).toContain('未发布')
    expect(legacyRow.text()).toContain('未设置培训分类')
    expect(legacyRow.text()).toContain('图文课程')

    const sourceRow = wrapper.findAll('.el-table__row').find(row => row.text().includes('来源排除课程'))!
    expect(sourceRow.text()).toContain('来源标签题不计入公开题库')
  })

  it('空分类课程可直接发布，不触发前端硬门', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const legacyRow = wrapper.findAll('.el-table__row').find(row => row.text().includes('未挂载遗留课程'))
    await legacyRow!.findComponent({ name: 'ElDropdown' }).vm.$emit('command', 'toggle')
    await flushPromises()

    expect(adminApi.updateCourse).toHaveBeenCalledWith(2, { status: 1 })
    expect(warnSpy).not.toHaveBeenCalledWith(expect.stringContaining('请先补全课程分类'))
  })

  it('已挂载课程可以正常发布/下架', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const draftRow = wrapper.findAll('.el-table__row').find(r => r.text().includes('草稿课程'))
    await draftRow!.findComponent({ name: 'ElDropdown' }).vm.$emit('command', 'toggle')
    await flushPromises()

    expect(adminApi.updateCourse).toHaveBeenCalledWith(3, { status: 1 })
    expect(successSpy).toHaveBeenCalled()
  })

  it('facet 计数按全量、启用分类与遗留桶展示', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const navItems = wrapper.findAll('.cc-nav-item')
    expect(navItems[0].find('.cc-nav-count').text()).toBe('5')
    expect(navItems[1].find('.cc-nav-count').text()).toBe('3')
    expect(navItems.find(item => item.text().includes('未分类/分类失效'))!.find('.cc-nav-count').text()).toBe('2')
  })

  it('先筛选再清除状态筛选恢复全量，不为空', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const statusSelect = wrapper
      .findAllComponents({ name: 'ElSelect' })
      .find(s => s.props('placeholder') === '状态')
    await statusSelect!.vm.$emit('update:modelValue', 0)
    await flushPromises()
    expect(wrapper.findAll('.el-table__row').length).toBe(2)

    await statusSelect!.vm.$emit('update:modelValue', undefined)
    await flushPromises()
    expect(wrapper.findAll('.el-table__row').length).toBe(5)
  })
})
