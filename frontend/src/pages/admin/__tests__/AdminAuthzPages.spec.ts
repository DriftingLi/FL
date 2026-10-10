// 授权两页的关键行为（#1621 段4 立页；#1630 补中文名）。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const h = vi.hoisted(() => ({
  listRoles: vi.fn(),
  updateRole: vi.fn(),
  listAccounts: vi.fn(),
  assignRole: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminApi: {
    listAdminRoles: h.listRoles,
    createAdminRole: vi.fn(),
    updateAdminRole: h.updateRole,
    deleteAdminRole: vi.fn(),
    listAdminAccounts: h.listAccounts,
    assignAdminRole: h.assignRole
  }
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
})
