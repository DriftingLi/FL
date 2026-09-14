// SearchPage（ADR-0049 / #983）：URL 状态同步、结果落点（死链修复）、分区计数、两级空态、
// 命中片段投影与高亮、本地历史入口。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

const h = vi.hoisted(() => ({
  route: { query: {} as Record<string, unknown>, params: {} as Record<string, unknown>, path: '/training/search' },
  replace: vi.fn(),
  push: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRoute: () => h.route,
  useRouter: () => ({ replace: h.replace, push: h.push })
}))
vi.mock('@/api/search', () => ({ searchApi: { search: vi.fn() } }))
vi.mock('@/stores/credential', () => ({ useCredentialStore: () => ({ current: { id: 1 } }) }))

import { searchApi } from '@/api/search'
import SearchPage from '../SearchPage.vue'
import SearchResultRow from '@/components/student/SearchResultRow.vue'

function item(type: string, id: number, over: Record<string, unknown> = {}) {
  return {
    type,
    id,
    title: '标题' + id,
    cover: '',
    summary: '摘要' + id,
    snippet: '片段里带关键词液压' + id,
    hit_field: 'title',
    parent_id: type === 'chapter' ? 3 : 0,
    ...over
  }
}

function allResult() {
  return {
    keyword: '液压',
    courses: { items: [item('course', 5)], total: 1 },
    chapters: { items: [item('chapter', 4)], total: 2 },
    questions: { items: [item('question', 7)], total: 1 },
    contents: { items: [item('content', 9)], total: 1 },
    topics: { items: [item('topic', 11, { hit_field: 'reply' })], total: 1 }
  }
}

function mountPage() {
  return mount(SearchPage, { global: { plugins: [epLite()] } })
}

beforeEach(() => {
  h.route.query = {}
  h.replace.mockClear()
  h.push.mockClear()
  window.localStorage.clear()
  vi.mocked(searchApi.search).mockReset()
  // 同一端点两种形状：带 type = 分页结果，不带 = 分区聚合（ADR-0018 的联合形状）
  vi.mocked(searchApi.search).mockImplementation((async (params: { keyword: string; type?: string }) => {
    if (params?.type) {
      return { keyword: params.keyword, type: params.type, total: 1, page: 1, pages: 1, items: [item(params.type, 4)] }
    }
    return allResult()
  }) as never)
})

describe('SearchPage 全局搜索页', () => {
  it('带 keyword 打开即复现搜索态，并渲染五个分区与分区计数', async () => {
    h.route.query = { keyword: '液压' }
    const w = mountPage()
    await flushPromises()
    expect(searchApi.search).toHaveBeenCalledWith({ keyword: '液压', credential_id: 1 })
    const text = w.text()
    for (const label of ['课程', '章节', '题目', '内容精选', '帖子']) expect(text).toContain(label)
    expect(text).toContain('课程(1)')
    expect(text).toContain('章节(2)')
    expect(text).toContain('命中在回复')
  })

  it('每条结果都有落点：题目 / 内容精选 / 章节点得开（死链修复）', async () => {
    h.route.query = { keyword: '液压' }
    const w = mountPage()
    await flushPromises()
    const rows = w.findAllComponents(SearchResultRow)
    expect(rows.length).toBe(5)
    const titles = rows.map((r) => r.props('item').title)
    await rows[titles.indexOf('标题7')].trigger('click')
    expect(h.push).toHaveBeenCalledWith('/training/questions/7')
    await rows[titles.indexOf('标题9')].trigger('click')
    expect(h.push).toHaveBeenCalledWith('/training/featured/9')
    await rows[titles.indexOf('标题4')].trigger('click')
    expect(h.push).toHaveBeenCalledWith('/training/course/3/chapter/4')
  })

  it('提交搜索把状态写进 URL，切分区同样同步', async () => {
    const w = mountPage()
    await flushPromises()
    const input = w.find('input')
    await input.setValue('液压')
    const searchBtn = w.findAll('button').find((b) => b.text() === '搜索')!
    await searchBtn.trigger('click')
    await flushPromises()
    expect(h.replace).toHaveBeenCalledWith({ query: { keyword: '液压' } })
    const chapterTab = w.findAll('button').find((b) => b.text().startsWith('章节'))!
    await chapterTab.trigger('click')
    await flushPromises()
    expect(h.replace).toHaveBeenCalledWith({ query: { keyword: '液压', type: 'chapter' } })
    expect(searchApi.search).toHaveBeenLastCalledWith(expect.objectContaining({ type: 'chapter', page: 1 }))
  })

  it('两级空态：未搜索给提示 + 本地历史；无匹配给替代路径', async () => {
    window.localStorage.setItem('search_history', JSON.stringify(['液压']))
    const w = mountPage()
    await flushPromises()
    expect(w.text()).toContain('搜索历史')
    expect(w.text()).toContain('液压')

    vi.mocked(searchApi.search).mockResolvedValue({
      keyword: '查无',
      courses: { items: [], total: 0 },
      chapters: { items: [], total: 0 },
      questions: { items: [], total: 0 },
      contents: { items: [], total: 0 },
      topics: { items: [], total: 0 }
    } as never)
    const input = w.find('input')
    await input.setValue('查无')
    await w.findAll('button').find((b) => b.text() === '搜索')!.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('没有找到相关内容')
    expect(w.text()).toContain('去题库练习')
  })

  it('点历史词直接发起搜索并记录到本地', async () => {
    window.localStorage.setItem('search_history', JSON.stringify(['液压']))
    const w = mountPage()
    await flushPromises()
    const chip = w.findAll('span.cursor-pointer').find((s) => s.text() === '液压')!
    await chip.trigger('click')
    await flushPromises()
    expect(searchApi.search).toHaveBeenCalledWith(expect.objectContaining({ keyword: '液压' }))
    expect(JSON.parse(window.localStorage.getItem('search_history') || '[]')).toEqual(['液压'])
  })
})
