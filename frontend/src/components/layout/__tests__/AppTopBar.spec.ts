// AppTopBar 行为锁定（#1619 / ADR-0072）：
// 身份格（头像/昵称/首字母）、移动端汉堡、通知入口的角色分流、工具位的主题入口、退出登录。
//
// 2026-10-28 改判：外观三态从用户菜单里搬到了工具位（ThemeToggle topbar 档），
// 于是「命令驱动 setMode」这条契约移到了 ThemeToggle.spec，这里只守**位置**与
// 「菜单里不再有外观项」——位置错了用户就得先点开自己的头像才能换外观。
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ElDropdown } from 'element-plus/es/components/dropdown/index.mjs'

const h = vi.hoisted(() => ({
  push: vi.fn(),
  confirm: vi.fn(),
  signOut: vi.fn(),
  setMode: vi.fn(),
  mode: 'system' as string,
  userInfo: {} as Record<string, unknown>,
  isLoggedIn: true
}))

vi.mock('vue-router', () => ({ useRouter: () => ({ push: h.push }) }))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    get userInfo() {
      return h.userInfo
    },
    get isLoggedIn() {
      return h.isLoggedIn
    },
    signOut: h.signOut
  })
}))
vi.mock('@/stores/theme', () => ({
  useThemeStore: () => ({
    get mode() {
      return h.mode
    },
    setMode: h.setMode
  })
}))
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ confirm: h.confirm }) }))
vi.mock('@/config/pages', () => ({ href: (name: string) => ({ name }) }))

import { epLite } from '@/test/element-lite'
import AppTopBar from '../AppTopBar.vue'

function mountBar(collapsed = false) {
  return mount(AppTopBar, {
    props: { collapsed },
    // ThemeToggle 单独有 spec（三档外观 + setMode 命令）；这里 stub 掉，
    // 免得它内部的 el-dropdown 抢走 findComponent(ElDropdown) 的第一个命中
    global: { plugins: [epLite()], stubs: { NotificationPanel: true, ThemeToggle: true } }
  })
}

let wrapper: ReturnType<typeof mountBar> | null = null

beforeEach(() => {
  vi.clearAllMocks()
  h.mode = 'system'
  h.isLoggedIn = true
  h.userInfo = { username: '张师傅', role: 'hrwai_user' }
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('顶栏身份格', () => {
  it('无头像时用昵称首字母，有头像时渲染图片', () => {
    wrapper = mountBar()
    expect(wrapper.find('.topbar-avatar.is-fallback').text()).toBe('张')

    wrapper.unmount()
    h.userInfo = { username: '张师傅', role: 'hrwai_user', avatar_url: '/static/a.png' }
    wrapper = mountBar()
    expect(wrapper.find('img.topbar-avatar').attributes('src')).toBe('/static/a.png')
  })

  it('昵称进 title（长昵称省略号截断时仍可悬停看全）', () => {
    wrapper = mountBar()
    expect(wrapper.find('.topbar-nickname').attributes('title')).toBe('张师傅')
  })

  it('折叠态在根节点标记 is-collapsed（左格随之收窄）', () => {
    wrapper = mountBar(true)
    expect(wrapper.find('.app-topbar').classes()).toContain('is-collapsed')
  })

  it('移动端汉堡发出 open-mobile（不做导航）', async () => {
    wrapper = mountBar()
    await wrapper.find('.topbar-burger').trigger('click')
    expect(wrapper.emitted('open-mobile')).toHaveLength(1)
    expect(h.push).not.toHaveBeenCalled()
  })
})

describe('顶栏工具位', () => {
  it('通知入口只对学员端出现', () => {
    wrapper = mountBar()
    expect(wrapper.find('notification-panel-stub').exists()).toBe(true)

    wrapper.unmount()
    h.userInfo = { username: '导师甲', role: 'tutor' }
    wrapper = mountBar()
    expect(wrapper.find('notification-panel-stub').exists()).toBe(false)
  })

  it('主题入口在右区工具位（不在用户菜单里）', () => {
    wrapper = mountBar()
    expect(wrapper.find('.topbar-actions theme-toggle-stub').exists()).toBe(true)
    // 菜单里只剩身份只读行与退出登录：外观三项与分组标题都不该回来
    expect(wrapper.text()).not.toContain('外观')
    expect(wrapper.text()).not.toContain('跟随系统')
    expect(wrapper.text()).not.toContain('浅色')
  })

  it('主题入口对所有角色出现（通知才分角色）', () => {
    h.userInfo = { username: '导师甲', role: 'tutor' }
    wrapper = mountBar()
    expect(wrapper.find('theme-toggle-stub').exists()).toBe(true)
    expect(wrapper.find('notification-panel-stub').exists()).toBe(false)
  })

  it('退出登录：确认后 revoke + 跳登录页', async () => {
    h.confirm.mockResolvedValue(undefined)
    h.signOut.mockResolvedValue({ revoked: true })
    wrapper = mountBar()
    await wrapper.findAllComponents(ElDropdown).at(-1)!.vm.$emit('command', 'logout')
    await flushPromises()
    expect(h.confirm).toHaveBeenCalled()
    expect(h.signOut).toHaveBeenCalledTimes(1)
    expect(h.push).toHaveBeenCalledWith({ name: 'Login' })
  })

  it('退出登录：用户取消则不动登录态', async () => {
    h.confirm.mockRejectedValue(new Error('cancel'))
    wrapper = mountBar()
    await wrapper.findAllComponents(ElDropdown).at(-1)!.vm.$emit('command', 'logout')
    await flushPromises()
    expect(h.signOut).not.toHaveBeenCalled()
    expect(h.push).not.toHaveBeenCalled()
  })
})
