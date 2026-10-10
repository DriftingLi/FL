// 授权两页的关键行为（#1621 段4 立页；#1630 补中文名与新建/删除；#1640 补代重置口令）。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const h = vi.hoisted(() => ({
  listRoles: vi.fn(),
  createRole: vi.fn(),
  updateRole: vi.fn(),
  deleteRole: vi.fn(),
  listAccounts: vi.fn(),
  assignRole: vi.fn(),
  createAccount: vi.fn(),
  deleteAccount: vi.fn(),
  resetPassword: vi.fn(),
  confirm: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminApi: {
    listAdminRoles: h.listRoles,
    createAdminRole: h.createRole,
    updateAdminRole: h.updateRole,
    deleteAdminRole: h.deleteRole,
    listAdminAccounts: h.listAccounts,
    assignAdminRole: h.assignRole,
    createAdminAccount: h.createAccount,
    deleteAdminAccount: h.deleteAccount,
    resetAdminPassword: h.resetPassword
  }
}))

vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: h.confirm, confirmDanger: h.confirm, prompt: vi.fn() })
}))

import { epLite } from '@/test/element-lite'
import RoleManage from '../RoleManage.vue'
import AccountManage from '../AccountManage.vue'

// 能力键取自真实能力表（#1630 起页面把它们翻成中文，域分组标题同样走中文词表）
const protectedRole = {
  role_id: 1,
  name: '超级管理员',
  protected: true,
  remark: '',
  capabilities: ['admin.access', 'admin_role.manage', 'audit.read']
}
const opsRole = { role_id: 2, name: '运营', protected: false, remark: '', capabilities: ['audit.read'] }

beforeEach(() => {
  vi.clearAllMocks()
  setActivePinia(createPinia())
  h.listRoles.mockResolvedValue({ roles: [protectedRole, opsRole] })
  h.updateRole.mockResolvedValue({})
  h.listAccounts.mockResolvedValue({
    accounts: [
      { admin_id: 1, username: 'super', name: '超管', role_id: 1, role_name: '超级管理员', protected: true },
      { admin_id: 2, username: 'ops', name: '运营甲', role_id: 2, role_name: '运营', protected: false }
    ]
  })
  h.assignRole.mockResolvedValue({})
  h.createRole.mockResolvedValue({})
  h.deleteRole.mockResolvedValue(null)
  h.createAccount.mockResolvedValue({})
  h.deleteAccount.mockResolvedValue(null)
  h.resetPassword.mockResolvedValue(null)
  h.confirm.mockResolvedValue(true)
})

describe('角色权限页', () => {
  it('受保护角色只读：无保存/删除按钮，勾选框禁用', async () => {
    const w = mount(RoleManage, { global: { plugins: [epLite()] } })
    await flushPromises()
    const cards = w.findAll('.role-card')
    expect(cards).toHaveLength(2)
    expect(cards[0].findAll('button')).toHaveLength(0)
    expect(cards[0].find('.el-checkbox.is-disabled').exists()).toBe(true)
    expect(cards[1].findAll('button').length).toBeGreaterThan(0)
  })

  it('能力按资源域分组，标题与勾选框都是中文（不印能力键）', async () => {
    const w = mount(RoleManage, { global: { plugins: [epLite()] } })
    await flushPromises()
    const groups = w.findAll('.cap-group-title').map(g => g.text())
    expect(groups).toContain('管理端')
    expect(groups).toContain('管理角色')
    expect(groups).toContain('审计')
    expect(groups.join(' ')).not.toContain('admin.access')
    expect(w.text()).toContain('管理端入口与用户管理')
    expect(w.text()).toContain('审计日志')
    expect(w.text()).not.toContain('admin_role.manage')
  })
})

describe('管理员管理页', () => {
  it('渲染账号与身份，未挂角色显示未授权', async () => {
    h.listAccounts.mockResolvedValue({
      accounts: [{ admin_id: 3, username: 'ghost', name: '', role_id: 0, role_name: '', protected: false }]
    })
    const w = mount(AccountManage, { global: { plugins: [epLite()] } })
    await flushPromises()
    expect(w.text()).toContain('未授权')
  })

  it('新建：填完表单提交 → 调 createAdminAccount（role_id 一并带上）', async () => {
    const w = mount(AccountManage, { global: { plugins: [epLite()] } })
    await flushPromises()
    await w.find('.account-actions button').trigger('click')
    await flushPromises()
    const inputs = w.findAll('.account-form input')
    expect(inputs.length).toBeGreaterThanOrEqual(3)
    await inputs[0].setValue('newops')
    await inputs[1].setValue('新运营')
    await inputs[2].setValue('newpass123')
    // 表单级校验通过后由对话框的「创建」按钮提交
    const confirmBtn = w.findAll('button').find(b => b.text().includes('创建'))
    expect(confirmBtn).toBeTruthy()
    await confirmBtn!.trigger('click')
    await flushPromises()
    expect(h.createAccount).toHaveBeenCalledWith({
      username: 'newops',
      name: '新运营',
      password: 'newpass123',
      role_id: 0
    })
  })

  it('新建：口令不合规在入口被拦下（不发请求）', async () => {
    const w = mount(AccountManage, { global: { plugins: [epLite()] } })
    await flushPromises()
    await w.find('.account-actions button').trigger('click')
    await flushPromises()
    const inputs = w.findAll('.account-form input')
    await inputs[0].setValue('newops')
    await inputs[1].setValue('新运营')
    await inputs[2].setValue('123')
    const confirmBtn = w.findAll('button').find(b => b.text().includes('创建'))
    await confirmBtn!.trigger('click')
    await flushPromises()
    expect(h.createAccount).not.toHaveBeenCalled()
    expect(w.find('.account-form-error').text()).toContain('6-20')
  })

  it('重置口令：填新口令提交 → 调 resetAdminPassword（口令不合规不发请求）', async () => {
    const w = mount(AccountManage, { global: { plugins: [epLite()] } })
    await flushPromises()
    // 每行两个动作：先「重置口令」，后「删除」
    const resetBtns = w.findAll('button').filter(b => b.text().includes('重置口令'))
    expect(resetBtns.length).toBe(2)
    await resetBtns[resetBtns.length - 1].trigger('click')
    await flushPromises()
    const input = w.find('#admin-account-new-password')
    expect(input.exists()).toBe(true)
    // 对话框页脚的「重置」按钮（行内那个是「重置口令」，两者文案不同，这里按页脚 + 精确文案取）
    const confirmBtn = () => w.findAll('.el-dialog__footer button').find(b => b.text().trim() === '重置')
    // 口令过短：入口拦下，不发请求
    await input.setValue('123')
    await confirmBtn()!.trigger('click')
    await flushPromises()
    expect(h.resetPassword).not.toHaveBeenCalled()
    expect(w.find('.account-form-error').text()).toContain('6-20')
    // 合规口令：按行上的账号 id 提交
    await input.setValue('resetpass456')
    await confirmBtn()!.trigger('click')
    await flushPromises()
    expect(h.resetPassword).toHaveBeenCalledWith(2, 'resetpass456')
  })

  it('删除：确认后调 deleteAdminAccount', async () => {
    const w = mount(AccountManage, { global: { plugins: [epLite()] } })
    await flushPromises()
    // 每行一个删除按钮：取最后一行（运营甲，非受保护）——受保护账号由后端 409 拦，前端不预判
    const delBtns = w.findAll('button').filter(b => b.text().includes('删除'))
    expect(delBtns.length).toBe(2)
    await delBtns[delBtns.length - 1].trigger('click')
    await flushPromises()
    expect(h.confirm).toHaveBeenCalled()
    expect(h.deleteAccount).toHaveBeenCalledWith(2)
  })
})
