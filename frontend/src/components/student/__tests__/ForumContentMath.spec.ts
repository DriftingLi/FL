// 论坛 / AI 助手面的公式与图表契约（ADR-0046 / #900）。
//
// markstream 面的能力由**应用启动处的开启单点**决定（markstreamRuntime），
// 这里不 stub 库、断言真实行为：开关没生效时公式不会渲染成公式。
//
// mermaid 是例外：它的渲染要等「块进入视口」，而 happy-dom 没有 IntersectionObserver，
// 且真 mermaid 需要真实布局才能画图。所以对 mermaid 用**替身 + 立刻回调的 IO**，
// 断言的是**库交给 mermaid 的安全配置**（strict / 禁脚本），而不是 mermaid 的画图正确性
// ——后者属于上游，且在真实浏览器里验证。
import { describe, it, expect, vi, beforeAll, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import ForumContent from '../ForumContent.vue'
import { setupMarkstreamRuntime } from '@/utils/markstreamRuntime'

vi.mock('markstream-vue/index.css', () => ({}))

const mermaidCalls = vi.hoisted(() => [] as unknown[][])

vi.mock('mermaid', () => ({
  default: {
    initialize: (...args: unknown[]) => {
      mermaidCalls.push(['initialize', args])
    },
    render: async (...args: unknown[]) => {
      mermaidCalls.push(['render', args])
      return { svg: '<svg id="fake-mermaid"></svg>' }
    },
    parse: async () => true
  }
}))

beforeAll(() => {
  class ImmediateIntersectionObserver {
    private readonly callback: (entries: unknown[]) => void
    constructor(callback: (entries: unknown[]) => void) {
      this.callback = callback
    }
    observe(target: Element) {
      this.callback([{ target, isIntersecting: true, intersectionRatio: 1 }])
    }
    unobserve() {}
    disconnect() {}
    takeRecords() {
      return []
    }
  }
  ;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = ImmediateIntersectionObserver
  setupMarkstreamRuntime()
})

afterEach(() => {
  delete (window as unknown as Record<string, unknown>).__pwned
  mermaidCalls.length = 0
})

function mountContent(content: string) {
  return mount(ForumContent, { props: { content, format: 'markdown' as const }, global: { plugins: [epLite()] } })
}

describe('ForumContent 公式（#900）', () => {
  it('块级公式渲染成公式', async () => {
    const w = mountContent('$$E=mc^2$$')
    await vi.waitFor(() => {
      expect(w.element.querySelector('.katex')).not.toBeNull()
    })
    expect(w.element.querySelector('[data-markstream-math="block"]')?.getAttribute('data-markstream-mode')).toBe('katex')
  })

  it('公式里的 \`\\href\` 不产生可点击链接（KaTeX trust 默认关闭）', async () => {
    const w = mountContent('$$\\href{javascript:alert(1)}{点我}$$')
    await vi.waitFor(() => {
      expect(w.element.querySelector('[data-markstream-math]')).not.toBeNull()
    })
    expect(w.element.querySelector('a[href^="javascript"]')).toBeNull()
  })

  it('新能力不削弱 escape 闸门：同一条正文里的 raw HTML 仍以文本呈现', async () => {
    const w = mountContent('$$E=mc^2$$\n\n<script>window.__pwned = 1</script>')
    await vi.waitFor(() => {
      expect(w.element.querySelector('.katex')).not.toBeNull()
    })
    expect((window as unknown as Record<string, unknown>).__pwned).toBeUndefined()
    expect(w.element.querySelector('script')).toBeNull()
    expect(w.text()).toContain('<script>')
  })

  it('行内 \`$...$\` 目前不渲染（上游现状，登记在 ADR-0046）：以源码可见，不假装支持', async () => {
    const w = mountContent('扭矩 $T$ 时按此取值')
    await vi.waitFor(() => {
      expect(w.text()).toContain('$T$')
    })
    expect(w.element.querySelector('[data-markstream-math]')).toBeNull()
  })
})

describe('mermaid 安全级（#900）', () => {
  it('渲染不可信内容前固定 strict：禁脚本、禁事件属性、不开 htmlLabels', async () => {
    const w = mountContent('```mermaid\ngraph TD;\nA-->B;\n```')
    await vi.waitFor(() => {
      expect(mermaidCalls.length).toBeGreaterThan(0)
    })

    const initCall = mermaidCalls.find((call) => call[0] === 'initialize')
    expect(initCall).toBeTruthy()
    const config = (initCall?.[1] as Record<string, unknown>[])[0] as Record<string, unknown>
    expect(config.securityLevel).toBe('strict')
    expect(config.startOnLoad).toBe(false)
    expect(config.htmlLabels).toBe(false)
    const dompurify = config.dompurifyConfig as { FORBID_TAGS?: string[] } | undefined
    expect(dompurify?.FORBID_TAGS).toContain('script')

    // 图中的标记走 mermaid 自己的渲染器，不会落成可执行的 DOM
    expect((window as unknown as Record<string, unknown>).__pwned).toBeUndefined()
    expect(w.element.querySelector('script')).toBeNull()
  })

  it('mermaid 块旁边的 raw HTML 也不会被执行', async () => {
    const w = mountContent('```mermaid\ngraph TD;\nA-->B;\n```\n\n<img src=x onerror="window.__pwned = 2">')
    await vi.waitFor(() => {
      expect(w.element.querySelector('[data-markstream-mermaid]')).not.toBeNull()
    })
    expect((window as unknown as Record<string, unknown>).__pwned).toBeUndefined()
    expect(w.element.querySelector('[onerror]')).toBeNull()
  })
})
