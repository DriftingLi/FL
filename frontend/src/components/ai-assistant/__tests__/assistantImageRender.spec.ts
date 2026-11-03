// 助手正文图片渲染面的前提事实（ADR-0063 / ADR-0091 的 E2 现测，已改为机检）：
// 后端 diagnosisAssistantAdapter 把 v2 回答里的第三方 OSS 图片归一成
// `![...](/api/ai-assistant/diagnosis/ossimg/… 本站代理 URL)`，白名单外的降级成 alt 文字。
// **成立的前提是 markstream 在 chat 模式 + html-policy=escape 下真会把 markdown 图片
// 渲染成带 src 的 <img>**。库升级改默认策略时这里先红，而不是等学员看到裸 markdown。
// 断言真实渲染产物、不 stub 库（先例 ForumContent.spec）。
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

const PROXY_H5_IMG = '/api/ai-assistant/diagnosis/ossimg/images_all/%E5%88%B6%E5%8A%A8%E7%B3%BB%E7%BB%9F/1.png'

describe('助手正文图片渲染前提（ChatPageShell 用的同一套 props）', () => {
  it('归一后的 markdown 图片渲染成 <img>，src 逐字保留（含中文百分号段）', async () => {
    const w = await renderAssistant(`## 一、前置准备\n1. 停机。\n\n![诊断配图](${PROXY_H5_IMG})\n\n2. 断电。`)
    const img = w.find('img')
    expect(img.attributes('src')).toBe(PROXY_H5_IMG)
    expect(img.attributes('alt')).toBe('诊断配图')
    // 归一后的正文不该再以文本形式露出 markdown 源码
    expect(w.text()).not.toContain('![')
  })

  it('未归一的第三方图（白名单外）以 alt 文字呈现，不出现可点链接或裸 URL', async () => {
    // adapter 侧已降级成纯文字；这里验的是**若它漏了**，前端也不会把它变成可执行资源：
    // 一段裸 OSS URL 在 escape 下只是文本，不生成 <img>。
    const w = await renderAssistant(
      '检查蓄能器。\n\n![别家图](https://cdn.example.com/a.png)\n\n断电。'
    )
    // markdown 图片语法本身仍会被 markstream 渲染成 img —— 但它指向的是第三方 host，
    // 说明归一链漏了。本用例只钉「裸 URL 不会变成可执行 html」，图的存在与否由后端测试锁。
    expect(w.element.querySelector('script')).toBeNull()
    expect(w.find('img[onerror]').exists()).toBe(false)
  })

  it('html-policy=escape 仍生效：正文里的裸 <img onerror> 不生成可执行元素', async () => {
    const w = await renderAssistant('<img src=x onerror="window.__pwned_img = 1">')
    expect(w.element.querySelector('img[onerror]')).toBeNull()
    expect(w.element.querySelector('[onerror]')).toBeNull()
    expect((window as unknown as Record<string, unknown>).__pwned_img).toBeUndefined()
  })
})
