// 课程编辑抽屉的积分价写入契约（#2447）：定价随行携带、清空后显式 null（= 免费）、
// 免费课提交也带 null 键——后端靠「键携带 + 值 null」与「未携带 = 保留现状」分家。
// seam：组件层，mock API 层（口径同 pages/admin/__tests__/CourseCatalog.spec.ts）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ElMessage } from 'element-plus'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/admin', () => ({
  adminApi: {
    getCourseDetail: vi.fn(),
    createCourse: vi.fn(),
    updateCourse: vi.fn(),
    setPublicVisibility: vi.fn(),
    createChapter: vi.fn(),
    updateChapter: vi.fn(),
    deleteChapter: vi.fn()
  }
}))

import { adminApi, type AdminCourseItem, type AdminCourseDetail } from '@/api/admin'
import CourseCatalogCourseDrawer from '../CourseCatalogCourseDrawer.vue'
import UiSelect from '@/components/ui/UiSelect.vue'

vi.spyOn(ElMessage, 'success').mockImplementation(() => undefined as never)

// AdminCourseItem = 生成的 CourseDTO：工厂补齐默认值（必填面满足校验规则），用例只写差异面。
function courseDTO(over: Partial<AdminCourseItem> = {}): AdminCourseItem {
  return {
    certificate_template_id: null,
    course_id: 1,
    course_type: 'illustrated',
    category_id: 9,
    cover_image: '',
    created_at: '2026-09-01T10:00:00+08:00',
    credential_id: 1,
    description: '',
    duration: 0,
    is_featured: false,
    is_hot: false,
    level_id: 1,
    name: '液压系统',
    practice_hours: 0,
    sort_order: 0,
    specialty_id: 2,
    status: 1,
    theory_hours: 0,
    ...over
  }
}

function courseDetailDTO(over: Partial<AdminCourseDetail> = {}): AdminCourseDetail {
  return { ...courseDTO(), chapters: [], ...over }
}

function mountDrawer() {
  return mount(CourseCatalogCourseDrawer, {
    props: {
      certificateTemplates: [],
      courseOptions: [],
      categories: [{ id: 9, name: '维修', status: 1 }],
      defaultCategoryId: null,
      submitting: false
    },
    global: { plugins: [epLite()] }
  })
}

async function openAndSubmit(wrapper: ReturnType<typeof mountDrawer>, course: AdminCourseItem) {
  ;(wrapper.vm as unknown as { open: (c: AdminCourseItem) => void }).open(course)
  await flushPromises()
  const save = wrapper.findAll('.el-drawer__footer button').find((b) => b.text().trim() === '保存')
  if (!save) throw new Error('找不到保存按钮')
  await save.trigger('click')
  await flushPromises()
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(adminApi.getCourseDetail).mockResolvedValue(courseDetailDTO())
  vi.mocked(adminApi.updateCourse).mockResolvedValue(courseDTO())
  vi.mocked(adminApi.createCourse).mockResolvedValue(courseDTO())
})

describe('CourseCatalogCourseDrawer 积分价写入契约', () => {
  it('定价课提交：payload 携带 points_price 值', async () => {
    vi.mocked(adminApi.getCourseDetail).mockResolvedValue(courseDetailDTO({ points_price: 100 }))
    const wrapper = mountDrawer()
    await openAndSubmit(wrapper, courseDTO({ course_id: 1, points_price: 100 }))
    expect(adminApi.updateCourse).toHaveBeenCalledWith(
      1,
      expect.objectContaining({ category_id: 9, points_price: 100 })
    )
  })

  it('免费课提交：payload 显式携带 null 键（不是缺键）', async () => {
    const wrapper = mountDrawer()
    await openAndSubmit(wrapper, courseDTO({ course_id: 1 }))
    expect(adminApi.updateCourse).toHaveBeenCalledWith(
      1,
      expect.objectContaining({ points_price: null })
    )
  })

  it('清空输入提交：payload 为 null（清空 = 免费，后端落 NULL）', async () => {
    vi.mocked(adminApi.getCourseDetail).mockResolvedValue(courseDetailDTO({ points_price: 100 }))
    const wrapper = mountDrawer()
    ;(wrapper.vm as unknown as { open: (c: AdminCourseItem) => void }).open(
      courseDTO({ course_id: 1, points_price: 100 })
    )
    await flushPromises()
    // 模拟 el-input-number 清空后的形态（EP 空输入即 update:modelValue(null)）
    const priceNumber = wrapper.findAllComponents({ name: 'ElInputNumber' })[3]
    priceNumber.vm.$emit('update:modelValue', null)
    await flushPromises()
    const save = wrapper.findAll('.el-drawer__footer button').find((b) => b.text().trim() === '保存')
    await save!.trigger('click')
    await flushPromises()
    expect(adminApi.updateCourse).toHaveBeenCalledWith(
      1,
      expect.objectContaining({ points_price: null })
    )
  })

  it('编辑停用分类仍回显并保留当前归属，新建时不提供停用分类', async () => {
    vi.mocked(adminApi.getCourseDetail).mockResolvedValue(courseDetailDTO({
      category_id: 8,
      category: { id: 8, name: '旧分类', status: 0 }
    }))
    const wrapper = mount(CourseCatalogCourseDrawer, {
      props: {
        certificateTemplates: [],
        courseOptions: [],
        categories: [
          { id: 9, name: '维修', status: 1 },
          { id: 8, name: '旧分类', status: 0 }
        ],
        defaultCategoryId: null,
        submitting: false
      },
      global: { plugins: [epLite()] }
    })
    await openAndSubmit(wrapper, courseDTO({
      category_id: 8,
      category: { id: 8, name: '旧分类', status: 0 }
    }))

    const categorySelect = wrapper.findAllComponents(UiSelect)
      .find(select => select.props('placeholder') === '请选择课程分类')!
    expect(categorySelect.props('modelValue')).toBe(8)
    expect(categorySelect.props('options')).toEqual([
      { label: '维修', value: 9 },
      { label: '旧分类（已停用，保留当前归属）', value: 8 }
    ])
    expect(adminApi.updateCourse).toHaveBeenCalledWith(1, expect.objectContaining({
      category_id: 8,
      course_type: 'illustrated'
    }))

    ;(wrapper.vm as unknown as { open: (c: null) => void }).open(null)
    await flushPromises()
    expect(categorySelect.props('options')).toEqual([{ label: '维修', value: 9 }])
  })
})

describe('CourseCatalogCourseDrawer 字段映射', () => {
  it('列表首开保留假值回退，不被详情模式的空值语义覆盖', async () => {
    vi.mocked(adminApi.getCourseDetail).mockReturnValue(new Promise(() => {}))
    const wrapper = mountDrawer()
    const vm = wrapper.vm as unknown as { open: (course: AdminCourseItem) => void; drawerForm: Record<string, unknown> }
    vm.open(courseDTO({
      name: undefined as never,
      category_id: undefined as never,
      status: undefined as never,
      theory_hours: undefined as never,
      practice_hours: 0,
      duration: 0,
      certificate_template_id: undefined as never,
      description: undefined as never,
      is_hot: undefined as never,
      is_featured: undefined as never
    }))

    expect(vm.drawerForm).toMatchObject({
      course_id: 1,
      category_id: null,
      theory_hours: 0,
      practice_hours: 0,
      duration: 0,
      description: '',
      is_hot: false,
      is_featured: false
    })
    expect(vm.drawerForm.certificate_template_id).toBeUndefined()
    wrapper.unmount()
  })

  it('详情回填保留姓名与类型回退，并显式落 null、数值零和简介空串', async () => {
    let resolveDetail!: (detail: AdminCourseDetail) => void
    vi.mocked(adminApi.getCourseDetail).mockReturnValue(new Promise(resolve => { resolveDetail = resolve }))
    const wrapper = mountDrawer()
    const vm = wrapper.vm as unknown as { open: (course: AdminCourseItem) => void; drawerForm: Record<string, unknown> }
    vm.open(courseDTO({
      name: '列表名称',
      description: '列表简介',
      theory_hours: 5,
      certificate_template_id: 3,
      points_price: 100
    }))
    expect(vm.drawerForm.description).toBe('列表简介')
    const detail = courseDetailDTO({
      name: undefined as never,
      category_id: null as never,
      course_type: undefined as never,
      status: undefined as never,
      theory_hours: undefined as never,
      practice_hours: 0,
      duration: 0,
      description: undefined as never,
      certificate_template_id: null,
      points_price: null as never
    })
    expect(detail.description).toBeUndefined()
    resolveDetail(detail)
    await flushPromises()

    expect(vm.drawerForm).toMatchObject({
      name: '列表名称',
      category_id: null,
      course_type: 'illustrated',
      status: 1,
      theory_hours: 0,
      practice_hours: 0,
      duration: 0,
      description: '',
      certificate_template_id: null,
      points_price: null
    })
    wrapper.unmount()
  })

  it('提交 payload 保留证书空值转 0 与数组、布尔字段形状', async () => {
    vi.mocked(adminApi.getCourseDetail).mockResolvedValue(courseDetailDTO({
      certificate_template_id: null,
      prerequisite_course_ids: [2, 3],
      is_hot: true,
      is_featured: false
    }))
    const wrapper = mountDrawer()
    await openAndSubmit(wrapper, courseDTO({
      certificate_template_id: null,
      prerequisite_course_ids: [2, 3],
      is_hot: true,
      is_featured: false
    }))

    expect(adminApi.updateCourse).toHaveBeenCalledWith(1, expect.objectContaining({
      certificate_template_id: 0,
      prerequisite_course_ids: [2, 3],
      is_hot: true,
      is_featured: false
    }))
    wrapper.unmount()
  })
})

describe('课程抽屉公开资格', () => {
  it('先前课程的详情晚到时，保留当前课程的表单与公开原因', async () => {
    let resolveFirst!: (detail: AdminCourseDetail) => void
    const first = new Promise<AdminCourseDetail>(resolve => { resolveFirst = resolve })
    vi.mocked(adminApi.getCourseDetail)
      .mockReturnValueOnce(first)
      .mockResolvedValueOnce(courseDetailDTO({
        course_id: 2,
        name: '当前课程',
        public_eligibility: { eligible: false, reasons: ['manually_hidden'] }
      }))
    const wrapper = mountDrawer()
    const drawer = wrapper.vm as unknown as { open: (course: AdminCourseItem) => void }
    drawer.open(courseDTO())
    drawer.open(courseDTO({ course_id: 2 }))
    await flushPromises()
    resolveFirst(courseDetailDTO({
      name: '前一门课程',
      public_eligibility: { eligible: false, reasons: ['category_missing'] }
    }))
    await flushPromises()
    expect(wrapper.text()).toContain('管理员已关闭公开')
    expect(wrapper.text()).not.toContain('未设置培训分类')
    expect(wrapper.findAll('input').some(input => input.element.value === '当前课程')).toBe(true)
    wrapper.unmount()
  })

  it('开关更新失败保留原公开状态，打开同一开关可以重试', async () => {
    const publicEligibility = { eligible: false, reasons: ['manually_hidden'] } as const
    vi.mocked(adminApi.getCourseDetail).mockResolvedValue(courseDetailDTO({
      public_eligibility: { ...publicEligibility, reasons: [...publicEligibility.reasons] }
    }))
    vi.mocked(adminApi.setPublicVisibility)
      .mockRejectedValueOnce(new Error('请求失败'))
      .mockResolvedValueOnce({ eligible: true, reasons: [] })
    const log = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const wrapper = mountDrawer()
    try {
      ;(wrapper.vm as unknown as { open: (course: AdminCourseItem) => void }).open(courseDTO())
      await flushPromises()
      const toggle = wrapper.getComponent({ name: 'ElSwitch' })
      toggle.vm.$emit('change', true)
      await flushPromises()
      expect(toggle.props('modelValue')).toBe(false)
      expect(wrapper.emitted('visibilityChanged')).toBeUndefined()
      toggle.vm.$emit('change', true)
      await flushPromises()
      expect(toggle.props('modelValue')).toBe(true)
      expect(wrapper.emitted('visibilityChanged')).toEqual([[1, { eligible: true, reasons: [] }]])
    } finally {
      log.mockRestore()
      wrapper.unmount()
    }
  })
})
