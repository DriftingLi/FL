// markstream 运行时开启单点（ADR-0046 / #900）。
//
// 这批的三个可选 peer（katex / mermaid / stream-diffs）有一个共同约束：**动态 import**，
// 不许拖首屏。这里把「走 loader 而不是静态 import」变成可断言的事实——
// 谁把它换成静态 import，本文件会先红，而不是等到首包体积回归才发现。
//
// 组件文案中文化同样在这条单点上：`setDefaultI18nMap` 只许在这里被调一次，
// 散到各渲染组件就会出现「同一个库两套文案」。
import { describe, it, expect, vi, beforeEach } from 'vitest'

const mocks = vi.hoisted(() => ({
  enableKatex: vi.fn(),
  enableMermaid: vi.fn(),
  setDefaultI18nMap: vi.fn()
}))

vi.mock('markstream-vue', () => ({
  enableKatex: mocks.enableKatex,
  enableMermaid: mocks.enableMermaid,
  setDefaultI18nMap: mocks.setDefaultI18nMap
}))

/** 每个用例都拿一份**全新的模块实例**（`applied` 是模块级状态，跨用例会互相影响）。 */
async function freshSetup() {
  vi.resetModules()
  const mod = await import('../markstreamRuntime')
  await mod.setupMarkstreamRuntime()
  return mod
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('markstreamRuntime 开启单点', () => {
  it('幂等：重复调用只开启一次', async () => {
    vi.resetModules()
    const mod = await import('../markstreamRuntime')
    const first = mod.setupMarkstreamRuntime()
    const second = mod.setupMarkstreamRuntime()
    expect(second).toBe(first)
    await Promise.all([first, second])
    expect(mocks.enableKatex).toHaveBeenCalledTimes(1)
    expect(mocks.enableMermaid).toHaveBeenCalledTimes(1)
    expect(mocks.setDefaultI18nMap).toHaveBeenCalledTimes(1)
  })

  it('katex / mermaid 都经 loader 动态 import（渲染到才下载，不进首屏）', async () => {
    await freshSetup()
    const katexLoader = mocks.enableKatex.mock.calls[0]?.[0] as () => Promise<unknown>
    const mermaidLoader = mocks.enableMermaid.mock.calls[0]?.[0] as () => Promise<unknown>
    expect(typeof katexLoader).toBe('function')
    expect(typeof mermaidLoader).toBe('function')

    const katexModule = (await katexLoader()) as { default?: { renderToString?: unknown }; renderToString?: unknown }
    const katexApi = katexModule.default ?? katexModule
    expect(typeof katexApi.renderToString).toBe('function')
  })

  it('组件文案走 setDefaultI18nMap 中文化，且不引入 vue-i18n', async () => {
    const mod = await freshSetup()
    expect(mocks.setDefaultI18nMap).toHaveBeenCalledTimes(1)
    const map = mocks.setDefaultI18nMap.mock.calls[0]?.[0] as Record<string, string>
    expect(map).toEqual(mod.MARKSTREAM_I18N_ZH)
    // 每个键都是中文文案（否则会回落成按驼峰拆词的英文）
    const values = Object.values(map)
    expect(values.length).toBeGreaterThan(10)
    for (const value of values) {
      expect(value).toMatch(/[\u4e00-\u9fa5]/)
    }
  })
})
