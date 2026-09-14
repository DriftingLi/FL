// 管理端课程管理（左树右表）组件测试：未挂载课程标红、上架被拦。
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

vi.mock('@/api/training', () => ({
  trainingApi: {
    getAdminCatalogTree: vi.fn(),
    getLevels: vi.fn(),
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

import { adminApi, type AdminCourseItem } from '@/api/admin'
import { trainingApi } from '@/api/training'
import type { CatalogDirectionNode, CatalogLevelNode, LevelDict } from '@/api/training'
import type { CourseDTO } from '@/api/course'
import CourseCatalog from '../CourseCatalog.vue'

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
function levelDictOf(levelId: number, name: string, over: Partial<LevelDict> = {}): LevelDict {
  return { code: '', created_at: '', description: '', level_id: levelId, name, sort_order: 0, status: 1, ...over }
}
function specialtyOf(specialtyId: number, name: string, levels: CatalogLevelNode[]): CatalogDirectionNode {
  return { code: '', created_at: '', description: '', levels, name, sort_order: 0, specialty_id: specialtyId, status: 1 }
}


const warnSpy = vi.spyOn(ElMessage, 'warning').mockImplementation(() => undefined as never)
const successSpy = vi.spyOn(ElMessage, 'success').mockImplementation(() => undefined as never)

function mountPage() {
  return mount(CourseCatalog, {
    global: { plugins: [epLite()] }
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(adminApi.getCourses).mockResolvedValue({
    total: 3,
    courses: [
      courseOf(1, '液压系统', { specialty_id: 2, level_id: 2, status: 1, chapter_count: 7, theory_hours: 24, practice_hours: 16 }),
      courseOf(2, '未挂载遗留课程', { specialty_id: null, level_id: null, status: 0, chapter_count: 2 }),
      courseOf(3, '草稿课程', { specialty_id: 2, level_id: 1, status: 0, chapter_count: 0 })
    ]
  })
  vi.mocked(adminApi.getCourseDetail).mockResolvedValue({
    ...courseOf(1, '液压系统', { specialty_id: 2, level_id: 2, chapter_count: 7 }),
    chapters: []
  })
  vi.mocked(adminApi.updateCourse).mockResolvedValue({} as AdminCourseItem)
  vi.mocked(trainingApi.getAdminCatalogTree).mockResolvedValue({
    specialties: [specialtyOf(2, '维修', [])]
  })
  vi.mocked(trainingApi.getLevels).mockResolvedValue({
    levels: [levelDictOf(1, '入门'), levelDictOf(2, '进阶')]
  })
  vi.mocked(trainingApi.getCertificateTemplates).mockResolvedValue({
    certificate_templates: []
  })
})

describe('CourseCatalog 左树右表', () => {
  it('未挂载课程出现在左侧导航并标红待补全', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const navNames = wrapper.findAll('.cc-nav-item').map(n => n.text())
    expect(navNames.some(t => t.includes('未挂载课程'))).toBe(true)

    expect(wrapper.text()).toContain('待补全')
  })

  it('未挂载课程点「上架」被拦截且不调用更新接口', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const unmountedRow = wrapper.findAll('.el-table__row').find(r => r.text().includes('未挂载遗留课程'))
    expect(unmountedRow).toBeTruthy()
    // 下拉 teleport + happy-dom 交互不稳定，直接对 ElDropdown emit command（测 handleAction → toggleStatus 逻辑）
    await unmountedRow!.findComponent({ name: 'ElDropdown' }).vm.$emit('command', 'toggle')
    await flushPromises()

    expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('未挂载'))
    expect(adminApi.updateCourse).not.toHaveBeenCalled()
  })

  it('已挂载课程可以正常上架/下架', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const mountedRow = wrapper.findAll('.el-table__row').find(r => r.text().includes('草稿课程'))
    await mountedRow!.findComponent({ name: 'ElDropdown' }).vm.$emit('command', 'toggle')
    await flushPromises()

    expect(adminApi.updateCourse).toHaveBeenCalledWith(3, { status: 1 })
    expect(successSpy).toHaveBeenCalled()
  })

  it('左侧导航渲染筛选卡片计数（全量/方向/未挂载 facet）', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const navItems = wrapper.findAll('.cc-nav-item')
    // 「全部课程」= 3 门全量；「维修」方向 = 2；「未挂载课程」= 1
    expect(navItems[0].text()).toContain('全部课程')
    expect(navItems[0].find('.cc-nav-count').text()).toBe('3')
    expect(navItems[1].text()).toContain('维修')
    expect(navItems[1].find('.cc-nav-count').text()).toBe('2')
    const unmountedItem = navItems.find(n => n.text().includes('未挂载课程'))
    expect(unmountedItem).toBeTruthy()
    expect(unmountedItem!.find('.cc-nav-count').text()).toBe('1')
  })

  it('先筛选再清除筛选（el-select 清空置 undefined）列表恢复全量，不为空', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const statusSelect = wrapper
      .findAllComponents({ name: 'ElSelect' })
      .find(s => s.props('placeholder') === '状态')
    expect(statusSelect).toBeTruthy()

    // 筛选下架（status=0）：只剩 2 门
    await statusSelect!.vm.$emit('update:modelValue', 0)
    await flushPromises()
    expect(wrapper.findAll('.el-table__row').length).toBe(2)

    // 清除筛选：el-select clearable 置 undefined（valueOnClear 默认），应恢复 3 门
    await statusSelect!.vm.$emit('update:modelValue', undefined)
    await flushPromises()
    expect(wrapper.findAll('.el-table__row').length).toBe(3)
  })
})
