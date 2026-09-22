// 助手正文图片渲染面的前提事实（ADR-0063 的 E2 现测，已改为机检）：
// 后端 diagnosisAssistantAdapter 把 20260921 的 `[IMG:image_id]` 令牌归一成
// `![...](本站代理 URL)`，**成立的前提是 markstream 在 chat 模式 + html-policy=escape 下
// 真会把 markdown 图片渲染成带 src 的 <img>**。库升级改默认策略时这里先红，而不是等学员
// 看到裸 markdown 或内部标识。断言真实渲染产物、不 stub 库（先例 ForumContent.spec）。
import { describe, it, expect, vi, beforeAll } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import MarkdownRender from 'markstream-vue'
import { MARKSTREAM_MERMAID_PROPS } from '@/utils/markstreamRuntime'

vi.mock('markstream-vue/index.css', () => ({}))

// 库把图片节点做成**视口门控**（容器带 data-markstream-viewport-pending="true"，进视口才写
// src）。jsdom 没有真实视口 ⇒ 必须喂一个「立即命中」的 IntersectionObserver，否则断言的是
// 门控占位而不是渲染结果（首轮实测就是这样误判成「不渲染图片」）。
beforeAll(() => {
  const FakeIO = class {
    private readonly cb: (entries: IntersectionObserverEntry[]) => void
    constructor(cb: (entries: IntersectionObserverEntry[]) => void) {
      this.cb = cb
    }
    observe(el: Element) {
      queueMicrotask(() =>
        this.cb([{ isIntersecting: true, target: el } as unknown as IntersectionObserverEntry])
      )
    }
    unobserve() {}
    disconnect() {}
    takeRecords(): IntersectionObserverEntry[] {
      return []
    }
  }
  ;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = FakeIO
})

async function renderAssistant(content: string) {
  const w = mount(MarkdownRender, {
    props: {
      mode: 'chat',
      content,
      final: true,
      htmlPolicy: 'escape',
      mermaidProps: MARKSTREAM_MERMAID_PROPS,
      fade: false
    }
  })
  await nextTick()
  await flushPromises()
  await nextTick()
  return w
}

const PROXY_H5_IMG = '/api/ai-assistant/diagnosis/manual/fault_images/%E5%88%B6%E5%8A%A8%E7%B3%BB%E7%BB%9F/1.png'

describe('助手正文图片渲染前提（ChatPageShell 用的同一套 props）', () => {
  it('归一后的 markdown 图片渲染成 <img>，src 逐字保留（含中文百分号段）', async () => {
    const w = await renderAssistant(`## 一、前置准备\n1. 停机。\n\n![诊断配图](${PROXY_H5_IMG})\n\n2. 断电。`)
    const img = w.find('img')
    expect(img.attributes('src')).toBe(PROXY_H5_IMG)
    expect(img.attributes('alt')).toBe('诊断配图')
    // 归一后的正文不该再以文本形式露出 markdown 源码
    expect(w.text()).not.toContain('![')
  })

  it('未归一的裸令牌会以**字面文本**可见（这正是必须先在后端归一的理由）', async () => {
    const w = await renderAssistant('检查蓄能器。\n\n[IMG:img_ab12cd34ef56]\n\n断电。')
    expect(w.find('img').exists()).toBe(false)
    expect(w.text()).toContain('[IMG:img_ab12cd34ef56]')
  })

  it('html-policy=escape 仍生效：正文里的裸 <img onerror> 不生成可执行元素', async () => {
    const w = await renderAssistant('<img src=x onerror="window.__pwned_img = 1">')
    expect(w.element.querySelector('img[onerror]')).toBeNull()
    expect(w.element.querySelector('[onerror]')).toBeNull()
    expect((window as unknown as Record<string, unknown>).__pwned_img).toBeUndefined()
  })
})
