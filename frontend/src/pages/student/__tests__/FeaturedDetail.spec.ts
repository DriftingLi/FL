// 内容精选详情页的正文分叉（2026-11）：markdown 走发布端渲染，docx 走 docx-preview。
//
// 守的是「作者选了 Word，读者就一定看到 Word」——判据只有 content_type + docx_url 两列，
// 少判一个就会出现「选 docx 却渲染空 content」的黑屏页。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { epLite } from '@/test/element-lite'

const push = vi.fn()
vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '7' } }),
  useRouter: () => ({ push, back: vi.fn() })
}))
vi.mock('@/api/featured', () => ({ featuredApi: { getDetail: vi.fn() } }))

import { featuredApi } from '@/api/featured'
import FeaturedDetail from '../FeaturedDetail.vue'

const DocxStub = defineComponent({
  name: 'DocxDocument',
  props: { src: { type: String, default: '' } },
  setup(props) {
    return () => h('div', { class: 'docx-stub', 'data-src': props.src })
  }
})

const MarkdownStub = defineComponent({
  name: 'PublishMarkdown',
  props: { content: { type: String, default: '' }, subset: { type: String, default: '' } },
  setup(props) {
    return () => h('div', { class: 'md-stub', 'data-subset': props.subset }, props.content)
  }
})

function detail(over: Record<string, unknown> = {}) {
  return {
    content_id: 7,
    title: '内容精选标题',
    category: 'news',
    category_label: '行业资讯',
    source: '',
    cover_image: '',
    content: '# 正文',
    content_type: 'markdown',
    docx_url: '',
    published_at: '2026-11-01T10:00:00+08:00',
    created_at: '2026-11-01T10:00:00+08:00',
    related: [],
    prev: { content_id: 0, title: '' },
    next: { content_id: 0, title: '' },
    ...over
  }
}

async function mountWith(over: Record<string, unknown>) {
  vi.mocked(featuredApi.getDetail).mockResolvedValue(detail(over) as never)
  const w = mount(FeaturedDetail, {
    global: { plugins: [epLite()], stubs: { DocxDocument: DocxStub, PublishMarkdown: MarkdownStub } }
  })
  await flushPromises()
  return w
}

beforeEach(() => {
  vi.mocked(featuredApi.getDetail).mockReset()
})

describe('FeaturedDetail 正文形态分叉（2026-11）', () => {
  it('markdown 正文：渲染 PublishMarkdown，不挂 docx 组件', async () => {
    const w = await mountWith({ content: '# 正文' })
    expect(w.find('.md-stub').exists()).toBe(true)
    expect(w.find('.docx-stub').exists()).toBe(false)
    expect(w.find('.md-stub').attributes('data-subset')).toBe('featured')
    w.unmount()
  })

  it('docx 正文：渲染 DocxDocument 并把 docx_url 传给它，正文不再走 Markdown', async () => {
    const w = await mountWith({
      content_type: 'docx',
      content: '',
      docx_url: '/static/uploads/featured/docx/plan_1700000000000.docx'
    })
    const docx = w.find('.docx-stub')
    expect(docx.exists()).toBe(true)
    expect(docx.attributes('data-src')).toBe('/static/uploads/featured/docx/plan_1700000000000.docx')
    expect(w.find('.md-stub').exists()).toBe(false)
    w.unmount()
  })

  it('content_type=docx 但 docx_url 为空：退回 Markdown 渲染（不留空白页）', async () => {
    const w = await mountWith({ content_type: 'docx', content: '# 兜底正文', docx_url: '' })
    expect(w.find('.docx-stub').exists()).toBe(false)
    expect(w.find('.md-stub').exists()).toBe(true)
    w.unmount()
  })

  it('老数据没有 content_type 字段：按 markdown 渲染', async () => {
    const payload = detail() as Record<string, unknown>
    delete payload.content_type
    delete payload.docx_url
    vi.mocked(featuredApi.getDetail).mockResolvedValue(payload as never)
    const w = mount(FeaturedDetail, {
      global: { plugins: [epLite()], stubs: { DocxDocument: DocxStub, PublishMarkdown: MarkdownStub } }
    })
    await flushPromises()
    expect(w.find('.md-stub').exists()).toBe(true)
    expect(w.find('.docx-stub').exists()).toBe(false)
    w.unmount()
  })
})
