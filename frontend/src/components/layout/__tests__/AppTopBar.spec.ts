// AppTopBar 行为锁定（#1619 / ADR-0072）：
// 身份格（头像/昵称/首字母）、移动端汉堡、通知入口的角色分流、用户菜单的外观三态与退出登录。
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
    global: { plugins: [epLite()], stubs: { NotificationPanel: true } }
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

  it('外观三项：命令驱动 setMode，当前项打勾', async () => {
    h.mode = 'dark'
    wrapper = mountBar()
    await wrapper.findComponent(ElDropdown).vm.$emit('command', 'theme:light')
    expect(h.setMode).toHaveBeenCalledWith('light')

    await wrapper.findComponent(ElDropdown).vm.$emit('command', 'theme:system')
    expect(h.setMode).toHaveBeenCalledWith('system')
  })

  it('退出登录：确认后 revoke + 跳登录页', async () => {
    h.confirm.mockResolvedValue(undefined)
    h.signOut.mockResolvedValue({ revoked: true })
    wrapper = mountBar()
    await wrapper.findComponent(ElDropdown).vm.$emit('command', 'logout')
    await flushPromises()
    expect(h.confirm).toHaveBeenCalled()
    expect(h.signOut).toHaveBeenCalledTimes(1)
    expect(h.push).toHaveBeenCalledWith({ name: 'Login' })
  })

  it('退出登录：用户取消则不动登录态', async () => {
    h.confirm.mockRejectedValue(new Error('cancel'))
    wrapper = mountBar()
    await wrapper.findComponent(ElDropdown).vm.$emit('command', 'logout')
    await flushPromises()
    expect(h.signOut).not.toHaveBeenCalled()
    expect(h.push).not.toHaveBeenCalled()
  })
})
