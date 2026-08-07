// 导师端课程管理组件测试：新增课程对话框方向/等级必填字段、创建 payload 正确。
// seam：组件层，mock API 层。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus, { ElMessage } from 'element-plus'

vi.mock('@/api/tutor', () => ({
  tutorApi: {
    getCourses: vi.fn(),
    createCourse: vi.fn(),
    updateCourse: vi.fn()
  }
}))

vi.mock('@/api/training', () => ({
  trainingApi: {
    getCatalogTree: vi.fn(),
    getLevels: vi.fn(),
    getCertificateTemplates: vi.fn()
  }
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() })
}))

import { tutorApi } from '@/api/tutor'
import { trainingApi } from '@/api/training'
import TutorCourses from '../TutorCourses.vue'

const successSpy = vi.spyOn(ElMessage, 'success').mockImplementation(() => undefined as never)

function mountPage() {
  return mount(TutorCourses, {
    global: { plugins: [ElementPlus] }
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(tutorApi.getCourses).mockResolvedValue({
    code: 200,
    message: '',
    data: { total: 1, courses: [{ course_id: 1, name: '液压系统', specialty_id: 2, level_id: 2, chapter_count: 7 }] }
  })
  vi.mocked(tutorApi.createCourse).mockResolvedValue({ code: 201, message: '', data: { course_id: 2, name: '新课程' } })
  vi.mocked(trainingApi.getCatalogTree).mockResolvedValue({
    code: 200,
    message: '',
    data: { specialties: [{ specialty_id: 2, name: '维修', levels: [] }] }
  })
  vi.mocked(trainingApi.getLevels).mockResolvedValue({
    code: 200,
    message: '',
    data: { levels: [{ level_id: 2, name: '进阶' }] }
  })
  vi.mocked(trainingApi.getCertificateTemplates).mockResolvedValue({
    code: 200,
    message: '',
    data: { certificate_templates: [] }
  })
})

describe('TutorCourses 导师建课', () => {
  it('新增课程对话框含方向/等级必选字段', async () => {
    const wrapper = mountPage()
    await flushPromises()

    await wrapper.findAll('.el-button').find(b => b.text() === '新增课程')!.trigger('click')
    await flushPromises()

    const formItems = wrapper.findAllComponents({ name: 'ElFormItem' })
    const props = formItems.map(it => it.props('prop'))
    expect(props).toContain('name')
    expect(props).toContain('specialty_id')
    expect(props).toContain('level_id')
  })

  it('填写方向/等级后创建成功并携带完整 payload', async () => {
    const wrapper = mountPage()
    await flushPromises()

    await wrapper.findAll('.el-button').find(b => b.text() === '新增课程')!.trigger('click')
    await flushPromises()

    const inputs = wrapper.findAll('.el-dialog input')
    const nameInput = inputs.find(i => (i.element as HTMLInputElement).placeholder === '课程名称')
    await nameInput!.setValue('新课程')
    // 通过 ElSelect 组件 v-model 直接设置方向/等级（下拉 teleport + happy-dom 交互不稳定，payload 才是断言目标）
    const selects = wrapper.findAllComponents({ name: 'ElSelect' })
    await selects[0].vm.$emit('update:modelValue', 2)
    await flushPromises()
    await selects[1].vm.$emit('update:modelValue', 2)
    await flushPromises()

    await wrapper.find('.el-dialog .el-button--primary').trigger('click')
    await flushPromises()

    expect(tutorApi.createCourse).toHaveBeenCalledWith(
      expect.objectContaining({
        name: '新课程',
        specialty_id: 2,
        level_id: 2,
        status: 1
      })
    )
    expect(successSpy).toHaveBeenCalled()
  })
})
