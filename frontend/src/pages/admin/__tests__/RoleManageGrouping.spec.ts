// 授权界面的分组（#1639）：能力键与侧栏叶子一一对应之后，分组标题必须就是**侧栏分组的标题**。
//
// 收敛前这里按资源域分组，于是「课程管理」「岗位管理」「证件管理」三片叶子挤在「培训目录」
// 一个标题下，超管看到的仍是一团；现在组标题与顺序都从 config/pages.ts 的侧栏声明派生。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const h = vi.hoisted(() => ({
  listRoles: vi.fn(),
  createRole: vi.fn(),
  updateRole: vi.fn(),
  deleteRole: vi.fn(),
  confirm: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminApi: {
    listAdminRoles: h.listRoles,
    createAdminRole: h.createRole,
    updateAdminRole: h.updateRole,
    deleteAdminRole: h.deleteRole
  }
}))

vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: h.confirm, confirmDanger: h.confirm, prompt: vi.fn() })
}))

import { epLite } from '@/test/element-lite'
import { navGroups } from '@/config/pages'
import RoleManage from '../RoleManage.vue'

/** 动作能力组（挂不到任何管理页上的能力键）的标题，与 RoleManage.vue 里的判定同词。 */
const ACTION_GROUP_TITLE = '动作能力（不占侧栏页面）'

// 一个角色持有四个侧栏分组各一片 + 四枚动作能力：分组渲染的覆盖面由它一次打满。
const opsRole = {
  role_id: 2,
  name: '运营',
  protected: false,
  remark: '',
  capabilities: [
    'admin.access',
    'hrwai_user.manage',
    'tutor.manage',
    'course.manage',
    'position.manage',
    'credential.manage',
    'ai_config.manage',
    'content.generate',
    'featured.manage',
    'audit.read',
    'export.run',
    'points.admin',
    'job_report.handle',
    'catalog.author'
  ]
}

beforeEach(() => {
  vi.clearAllMocks()
  setActivePinia(createPinia())
  h.listRoles.mockResolvedValue({ roles: [opsRole] })
  h.updateRole.mockResolvedValue({})
  h.createRole.mockResolvedValue({})
  h.deleteRole.mockResolvedValue(null)
  h.confirm.mockResolvedValue(true)
})

/** 某个分组标题下的原始能力键（勾选框的 title 上留的就是键，展示文案是中文名）。 */
function keysOf(w: ReturnType<typeof mount>, title: string): string[] {
  const group = w.findAll('.cap-group').find(g => g.find('.cap-group-title').text() === title)
  expect(group, `没有渲染分组「${title}」`).toBeTruthy()
  return group!.findAll('span[title]').map(s => s.attributes('title') ?? '')
}

describe('角色权限页的分组', () => {
  it('分组标题与顺序都取侧栏分组，动作能力收在最后一组', async () => {
    const w = mount(RoleManage, { global: { plugins: [epLite()] } })
    await flushPromises()
    const titles = w.findAll('.cap-group-title').map(g => g.text())
    expect(titles).toEqual([...(navGroups.manage ?? []).map(g => g.label), ACTION_GROUP_TITLE])
  })

  it('每个能力键恰好落在一个分组里（不漏不重）', async () => {
    const w = mount(RoleManage, { global: { plugins: [epLite()] } })
    await flushPromises()
    const all = w.findAll('.cap-group').flatMap(g => g.findAll('span[title]').map(s => s.attributes('title') ?? ''))
    expect(all.slice().sort()).toEqual([...opsRole.capabilities].sort())
  })

  it('页面归属决定组：用户管理不在总览、证件管理与课程管理同组', async () => {
    const w = mount(RoleManage, { global: { plugins: [epLite()] } })
    await flushPromises()
    expect(keysOf(w, '总览')).toEqual(['admin.access'])
    expect(keysOf(w, '用户与内容').sort()).toEqual(['hrwai_user.manage', 'tutor.manage'])
    expect(keysOf(w, '教学管理').sort()).toEqual(['course.manage', 'credential.manage', 'position.manage'])
    expect(keysOf(w, '系统').sort()).toEqual(['ai_config.manage', 'audit.read', 'content.generate', 'featured.manage'])
  })

  it('动作能力（导出/积分/举报处置/目录作者）不占侧栏页面，单独成组', async () => {
    const w = mount(RoleManage, { global: { plugins: [epLite()] } })
    await flushPromises()
    expect(keysOf(w, ACTION_GROUP_TITLE).sort()).toEqual([
      'catalog.author',
      'export.run',
      'job_report.handle',
      'points.admin'
    ])
    // 组标题不再是资源域词（那是收敛前的形态）
    const titles = w.findAll('.cap-group-title').map(g => g.text())
    expect(titles).not.toContain('培训目录')
    expect(titles).not.toContain('内容')
  })
})
