// 登录页渲染回归测试：主登录按钮初始为「登 录」而非「登录中...」。
// 锁定 useAuthFlow 的 loading 解包契约（曾因返回普通对象导致嵌套 ref 不解包、按钮永久 loading）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia } from 'pinia'

vi.mock('@/api/auth', () => ({
  authApi: {
    login: vi.fn(), emailLogin: vi.fn(), phoneLogin: vi.fn(),
    tutorLogin: vi.fn(), adminLogin: vi.fn(), getUserInfo: vi.fn(),
    getCaptcha: vi.fn(), sendPhoneCode: vi.fn(), sendEmailCode: vi.fn(),
    logout: vi.fn(), updateProfile: vi.fn(), uploadAvatar: vi.fn()
  }
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ userInfo: { role: 'hrwai_user' }, isLoggedIn: false, setAuthData: vi.fn(), clearAuthData: vi.fn(), refreshUserInfo: vi.fn() })
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {} }),
  RouterLink: { template: '<a><slot /></a>' }
}))
vi.mock('@/utils/subdomain', () => ({
  getSubdomain: () => 'training',
  getRoleForSubdomain: () => 'hrwai_user',
  getDefaultWorkspaceBySubdomain: () => '/training',
  getTargetSubdomainForPath: vi.fn()
}))

import { authApi } from '@/api/auth'
import Login from '../Login.vue'

describe('Login 按钮初始状态', () => {
  beforeEach(() => {
    vi.mocked(authApi.getCaptcha).mockResolvedValue({ id: 'c1', image: 'data:image/png;base64,AA==' })
  })

  it('挂载后主登录按钮显示「登 录」且不带 loading 态', async () => {
    const wrapper = mount(Login, { global: { plugins: [ElementPlus, createPinia()] } })
    await flushPromises()
    const btn = wrapper.find('button.auth-btn')
    expect(btn.exists()).toBe(true)
    expect(btn.text()).toContain('登 录')
    expect(btn.classes()).not.toContain('is-loading')
    expect(btn.attributes('disabled')).toBeUndefined()
  })

  it('点击登录后按钮不残留 loading 态', async () => {
    const wrapper = mount(Login, { global: { plugins: [ElementPlus, createPinia()] } })
    await flushPromises()
    await wrapper.find('button.auth-btn').trigger('click')
    await flushPromises()
    expect(wrapper.find('button.auth-btn').classes()).not.toContain('is-loading')
  })
})
