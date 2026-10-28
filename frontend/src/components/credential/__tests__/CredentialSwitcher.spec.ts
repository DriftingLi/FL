// 证件切换器（顶栏中区）的紧凑档（2026-10-28）。
//
// 守的是**一个曾经坏掉的事实**：紧凑档必须把当前证件名渲染出来。
// 改之前控件靠 `width:100%` 在 flex 项里按 min-content 结算，实测只剩 34×24、
// 名字被裁到 11px —— 顶栏上只有一个悬空的箭头（用户反馈「图标歪了」）。
// 这条用例不看宽度（宽度归 CSS），只要求名字在 DOM 里可见。
import { describe, it, expect, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

const CURRENT = { id: 7, name: '叉车司机N1证', category: 'special_operation' }

vi.mock('@/stores/credential', () => ({
  useCredentialStore: () => ({
    grouped: {
      special_operation: [CURRENT],
      skill_level: [{ id: 9, name: '五级/初级工', category: 'skill_level', level: 5 }]
    },
    current: CURRENT,
    loading: false,
    initialized: true,
    loadGrouped: vi.fn().mockResolvedValue(undefined),
    loadCurrent: vi.fn().mockResolvedValue(undefined),
    switchTo: vi.fn().mockResolvedValue(undefined)
  })
}))

import CredentialSwitcher from '../CredentialSwitcher.vue'

describe('证件切换器紧凑档', () => {
  it('紧凑档渲染当前证件名（不是只有一个箭头）', async () => {
    const w = mount(CredentialSwitcher, { props: { compact: true }, global: { plugins: [epLite()] } })
    await flushPromises()
    expect(w.find('.credential-switcher').classes()).toContain('is-compact')
    // EP 把选中项渲染在 .el-select__placeholder 里（挂载后一拍才落地）
    expect(w.find('.credential-select').text()).toContain('叉车司机N1证')
    // 紧凑档不显示「当前证件」标签与 meta 行（顶栏放不下三行）
    expect(w.text()).not.toContain('当前证件')
  })

  it('紧凑档在窄屏有一个图标钮后备（按钮常驻 DOM，显示与否交给媒体查询）', async () => {
    const w = mount(CredentialSwitcher, { props: { compact: true }, global: { plugins: [epLite()] } })
    const btn = w.find('.compact-icon-btn')
    expect(btn.exists()).toBe(true)
    expect(btn.attributes('aria-label')).toContain('叉车司机N1证')
  })
})
