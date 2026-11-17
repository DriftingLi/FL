// 讲师角色面板（#2599 E7）：受保护角色只读、能力键取接口并集、新建/删除走对应端点。
//
// 为什么与 RoleManage 的用例分开写：两者数据源与可授权集合都不同（讲师侧 7 键 vs 管理侧全集），
// 合在一个文件里会让「哪条断言属于哪条链」只能靠 describe 名字分辨。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const h = vi.hoisted(() => ({
  listTutorRoles: vi.fn(),
  createTutorRole: vi.fn(),
  updateTutorRole: vi.fn(),
  deleteTutorRole: vi.fn(),
  confirm: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminApi: {
    listTutorRoles: h.listTutorRoles,
    createTutorRole: h.createTutorRole,
    updateTutorRole: h.updateTutorRole,
    deleteTutorRole: h.deleteTutorRole
  }
}))

vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: h.confirm, confirmDanger: h.confirm, prompt: vi.fn() })
}))

import { epLite } from '@/test/element-lite'
import TutorRolePanel from '../TutorRolePanel.vue'

const fullRole = {
  role_id: 1,
  name: '全权讲师',
  protected: true,
  remark: '受保护角色',
  capabilities: ['tutor.access', 'course.author', 'tutorfile.manage', 'question.author', 'catalog.author', 'contribution.review', 'notification.use']
}

const bankRole = {
  role_id: 2,
  name: '题库讲师',
  protected: false,
  remark: '',
  capabilities: ['tutor.access', 'question.author', 'catalog.author', 'notification.use']
}

beforeEach(() => {
  vi.clearAllMocks()
  setActivePinia(createPinia())
  h.listTutorRoles.mockResolvedValue({ roles: [fullRole, bankRole] })
  h.createTutorRole.mockResolvedValue({})
  h.updateTutorRole.mockResolvedValue({})
  h.deleteTutorRole.mockResolvedValue(null)
  h.confirm.mockResolvedValue(true)
})

describe('讲师角色面板', () => {
  it('能力勾选框取所有角色能力的并集，不漏不重', async () => {
    const w = mount(TutorRolePanel, { global: { plugins: [epLite()] } })
    await flushPromises()
    // 每张卡都渲染同一份并集，故先按卡取一份再比对（跨卡重复是刻意的：每卡独立勾选）。
    const firstCard = w.findAll('.role-card')[0]
    const keys = firstCard.findAll('span[title]').map(s => s.attributes('title') ?? '')
    const union = [...new Set([...fullRole.capabilities, ...bankRole.capabilities])]
    expect(keys.slice().sort()).toEqual(union.sort())
    // 且两张卡渲染的键集合一致（同一个并集，不是各自的子集）
    const secondCard = w.findAll('.role-card')[1]
    expect(secondCard.findAll('span[title]').map(s => s.attributes('title') ?? '').sort()).toEqual(union.sort())
  })

  it('受保护的全权讲师没有保存与删除按钮（能力恒为全集，不可改不可删）', async () => {
    const w = mount(TutorRolePanel, { global: { plugins: [epLite()] } })
    await flushPromises()
    const cards = w.findAll('.role-card')
    expect(cards).toHaveLength(2)
    // 第一张卡是全权讲师：动作区为空
    expect(cards[0].findAll('button')).toHaveLength(0)
    // 第二张卡是题库讲师：有保存与删除
    expect(cards[1].findAll('button').length).toBeGreaterThanOrEqual(2)
  })

  it('受保护角色的勾选框是禁用态（只读）', async () => {
    const w = mount(TutorRolePanel, { global: { plugins: [epLite()] } })
    await flushPromises()
    const cards = w.findAll('.role-card')
    const protectedBoxes = cards[0].findAll('input[type="checkbox"]')
    expect(protectedBoxes.length).toBeGreaterThan(0)
    for (const box of protectedBoxes) {
      expect((box.element as HTMLInputElement).disabled).toBe(true)
    }
  })

  it('新建角色后调 createTutorRole 并重载列表', async () => {
    const w = mount(TutorRolePanel, { global: { plugins: [epLite()] } })
    await flushPromises()
    h.listTutorRoles.mockClear()
    await w.find('.role-create input').setValue('实习讲师')
    const createBtn = w.findAll('.role-create button').at(-1)!
    await createBtn.trigger('click')
    await flushPromises()
    expect(h.createTutorRole).toHaveBeenCalledWith({ name: '实习讲师', capabilities: [] })
    expect(h.listTutorRoles).toHaveBeenCalled()
  })

  it('删除角色先确认再调 deleteTutorRole', async () => {
    const w = mount(TutorRolePanel, { global: { plugins: [epLite()] } })
    await flushPromises()
    const cards = w.findAll('.role-card')
    const deleteBtn = cards[1].findAll('button').at(-1)!
    await deleteBtn.trigger('click')
    await flushPromises()
    expect(h.confirm).toHaveBeenCalled()
    expect(h.deleteTutorRole).toHaveBeenCalledWith(bankRole.role_id)
  })

  it('未改动时保存按钮禁用（isDirty 为假）', async () => {
    const w = mount(TutorRolePanel, { global: { plugins: [epLite()] } })
    await flushPromises()
    const cards = w.findAll('.role-card')
    const saveBtn = cards[1].findAll('button')[0]
    expect(saveBtn.attributes('disabled')).toBeDefined()
  })
})
