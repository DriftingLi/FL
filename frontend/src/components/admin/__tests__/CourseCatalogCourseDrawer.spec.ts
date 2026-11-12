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
    createChapter: vi.fn(),
    updateChapter: vi.fn(),
    deleteChapter: vi.fn()
  }
}))

import { adminApi, type AdminCourseItem, type AdminCourseDetail } from '@/api/admin'
import CourseCatalogCourseDrawer from '../CourseCatalogCourseDrawer.vue'

vi.spyOn(ElMessage, 'success').mockImplementation(() => undefined as never)

// AdminCourseItem = 生成的 CourseDTO：工厂补齐默认值（必填面满足校验规则），用例只写差异面。
function courseDTO(over: Partial<AdminCourseItem> = {}): AdminCourseItem {
  return {
    certificate_template_id: null,
    course_id: 1,
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
      directions: [],
      levels: [],
      certificateTemplates: [],
      courseOptions: [],
      credentials: [],
      defaultSpecialtyId: null,
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
      expect.objectContaining({ points_price: 100 })
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
})
