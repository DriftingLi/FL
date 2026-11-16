// 讲师课程分配抽屉的关键行为（#2597 / 规格 D4）。
//
// 四条 AC 里前端能自证的三条：
//   - 抽屉列出课程并标出已分配状态（含来源：管理员分配 / 他自己认领）；
//   - 多选后「确定」把**差集**提交：新增走 assign、移除走 remove，且不重发未改动的；
//   - 移除有「不回溯」确认文案，取消则整体不动。
// 后端契约（代分配后讲师可写、移除后不可写、幂等、403）在
// backend/internal/api/admin_tutor_course_contract_test.go 里逐条测过。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const h = vi.hoisted(() => ({
  getCourses: vi.fn(),
  getTutorCourses: vi.fn(),
  assignTutorCourses: vi.fn(),
  removeTutorCourses: vi.fn(),
  confirmDanger: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminApi: {
    getCourses: h.getCourses,
    getTutorCourses: h.getTutorCourses,
    assignTutorCourses: h.assignTutorCourses,
    removeTutorCourses: h.removeTutorCourses
  }
}))

vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: vi.fn(), confirmDanger: h.confirmDanger, prompt: vi.fn() })
}))

import { epLite } from '@/test/element-lite'
import TutorCourseAssignDrawer from '../TutorCourseAssignDrawer.vue'

const coursesPage = {
  courses: [
    { course_id: 1, name: '叉车安全' },
    { course_id: 2, name: '叉车维修' },
    { course_id: 3, name: '电池维护' }
  ],
  page: 1,
  pages: 1,
  total: 3
}

function mountDrawer() {
  return mount(TutorCourseAssignDrawer, {
    props: { tutorId: 9, tutorName: '张讲师', modelValue: true },
    global: { plugins: [epLite()] }
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  h.getCourses.mockResolvedValue(coursesPage)
  h.getTutorCourses.mockResolvedValue({
    course_ids: [1],
    items: [{ course_id: 1, course_name: '叉车安全', source: 'admin', created_at: '2026-11-16T10:00:00+08:00' }]
  })
  h.assignTutorCourses.mockResolvedValue({ applied: 1, skipped: 0 })
  h.removeTutorCourses.mockResolvedValue({ applied: 1, skipped: 0 })
  h.confirmDanger.mockResolvedValue(undefined)
})

describe('TutorCourseAssignDrawer', () => {
  it('打开时同时拉课程候选与已分配名单，并标出来源', async () => {
    const wrapper = mountDrawer()
    await flushPromises()

    expect(h.getCourses).toHaveBeenCalledWith({ page: 1, page_size: 200, filter: 'all' })
    expect(h.getTutorCourses).toHaveBeenCalledWith(9)
    // 已分配的「叉车安全」带「管理员分配」标签，其余两门没有
    expect(wrapper.text()).toContain('管理员分配')
    expect(wrapper.text()).toContain('叉车安全')
    expect(wrapper.text()).toContain('叉车维修')
    expect(wrapper.text()).toContain('电池维护')
  })

  it('自助认领的课标成「他自己认领」，与管理员分配区分', async () => {
    h.getTutorCourses.mockResolvedValue({
      course_ids: [2],
      items: [{ course_id: 2, course_name: '叉车维修', source: 'self', created_at: '2026-11-16T10:00:00+08:00' }]
    })
    const wrapper = mountDrawer()
    await flushPromises()

    expect(wrapper.text()).toContain('他自己认领')
    expect(wrapper.text()).not.toContain('管理员分配')
  })

  it('只新增时走 assign，不调 remove', async () => {
    const wrapper = mountDrawer()
    await flushPromises()

    // 勾上课程 3（原本未分配）
    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    await checkboxes[2].setValue(true)
    await wrapper.findComponent({ name: 'UiDialog' }).vm.$emit('confirm')
    await flushPromises()

    expect(h.assignTutorCourses).toHaveBeenCalledWith(9, [3])
    expect(h.removeTutorCourses).not.toHaveBeenCalled()
    // 新增不是危险动作，不该弹确认框
    expect(h.confirmDanger).not.toHaveBeenCalled()
  })

  it('只移除时先弹「不回溯」确认，再走 remove', async () => {
    const wrapper = mountDrawer()
    await flushPromises()

    // 取消勾选课程 1（原本已分配）
    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    await checkboxes[0].setValue(false)
    await wrapper.findComponent({ name: 'UiDialog' }).vm.$emit('confirm')
    await flushPromises()

    // 确认文案必须含不回溯口径（B6/D4）
    expect(h.confirmDanger).toHaveBeenCalledTimes(1)
    const msg = h.confirmDanger.mock.calls[0][0] as string
    expect(msg).toContain('不能再编辑本课程')
    expect(msg).toContain('已写章节的归属与在审内容不受影响')

    expect(h.removeTutorCourses).toHaveBeenCalledWith(9, [1])
    expect(h.assignTutorCourses).not.toHaveBeenCalled()
  })

  it('确认框取消时一条请求都不发', async () => {
    h.confirmDanger.mockRejectedValue(new Error('cancel'))
    const wrapper = mountDrawer()
    await flushPromises()

    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    await checkboxes[0].setValue(false)
    await wrapper.findComponent({ name: 'UiDialog' }).vm.$emit('confirm')
    await flushPromises()

    expect(h.removeTutorCourses).not.toHaveBeenCalled()
    expect(h.assignTutorCourses).not.toHaveBeenCalled()
  })

  it('同时新增与移除时两条请求都发，且只带差集', async () => {
    const wrapper = mountDrawer()
    await flushPromises()

    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    await checkboxes[0].setValue(false) // 移除课程 1
    await checkboxes[2].setValue(true) // 新增课程 3
    await wrapper.findComponent({ name: 'UiDialog' }).vm.$emit('confirm')
    await flushPromises()

    expect(h.removeTutorCourses).toHaveBeenCalledWith(9, [1])
    expect(h.assignTutorCourses).toHaveBeenCalledWith(9, [3])
  })

  it('没有任何改动时不发请求', async () => {
    const wrapper = mountDrawer()
    await flushPromises()

    await wrapper.findComponent({ name: 'UiDialog' }).vm.$emit('confirm')
    await flushPromises()

    expect(h.assignTutorCourses).not.toHaveBeenCalled()
    expect(h.removeTutorCourses).not.toHaveBeenCalled()
  })
})
