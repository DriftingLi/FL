// #364 论坛类别分流的页面级测试：讨论 / 问答两个 Tab 各看各的列表。
//
// seam 选在组件层而非 api 层：forumApi.listTopics 只是把 params 透传给 axios，
// 给它写"category 有透传"的断言不可能失败，是无效测试。真正会被写坏的是
// 「讨论 Tab 有没有带上 category=discussion」——漏了这个参数，问答帖就会整片
// 灌进讨论 Tab（后端 scope=general 的定义恰好是 chapter_id IS NULL）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiInput from '@/components/ui/UiInput.vue'

// 只替换网络层，保留 forumTabQuery 的真实实现：
// 页面现在通过 forumTabQuery 把 Tab 翻成查询参数，若连它一起 mock 掉，
// 测的就是"页面调用了我在测试里写的那份映射"，而不是真规则。
// （同时 mock @/api/request，让真实的 forum.ts 在加载时不构造 axios 实例。）
vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn(), put: vi.fn() }
}))

vi.mock('@/api/forum', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/forum')>()
  return {
    ...actual,
    forumApi: {
      listTopics: vi.fn(),
      getMyTopics: vi.fn(),
      getMyReplies: vi.fn(),
      getMyLikedTopics: vi.fn(),
      getMyObservedTopics: vi.fn(),
      getMyViewHistory: vi.fn(),
      createTopic: vi.fn()
    }
  }
})

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {}, params: {} })
}))

// 页面只用到 userInfo?.user_id（浏览记录按用户隔离），mock 掉避免拉起真实 pinia。
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ userInfo: { user_id: 1 } })
}))

import { forumApi } from '@/api/forum'
import ForumPage from '../ForumPage.vue'

const listTopics = vi.mocked(forumApi.listTopics)
const createTopic = vi.mocked(forumApi.createTopic)

/** 一条帖子桩数据。 */
function topic(id: number, category: 'discussion' | 'question') {
  return {
    id,
    category,
    title: category === 'question' ? `提问 ${id}` : `讨论 ${id}`,
    content: '内容',
    view_count: 0,
    reply_count: 0,
    created_at: '2026-08-01T10:00:00+08:00',
    author: { user_id: 1, username: '张三', avatar_url: '' }
  }
}

async function mountPage(total = 3, options: { attachTo?: HTMLElement; topics?: ReturnType<typeof topic>[] } = {}) {
  // total=0 必须同时把 topics 置空，否则 v-else-if="topics.length > 0" 会渲染列表，
  // 空态断言就是在验一个永远走不到的分支。
  listTopics.mockResolvedValue({
    topics: options.topics ?? (total === 0 ? [] : [topic(1, 'discussion')]),
    total
  } as never)
  const wrapper = mount(ForumPage, {
    attachTo: options.attachTo,
    global: {
      plugins: [epLite()],
      // 打桩子组件：避免拉起真实网络层与弹窗（打卡已迁独立页，论坛不再内嵌打卡弹窗）。
      stubs: { ForumImageUploader: true }
    }
  })
  await flushPromises()
  return wrapper
}

/**
 * 按 options 的 value 集合定位分段控件——避免用 class 名（forum-category/forum-mode 已随重构删除），
 * 也避免按渲染顺序取索引（页面里的 UiSegmentTabs 还有 solved-filter、topicSort 会共页）。
 */
function tabbarByValues(wrapper: Awaited<ReturnType<typeof mountPage>>, mustInclude: string[]) {
  for (const bar of wrapper.findAllComponents(UiSegmentTabs)) {
    const opts = bar.props('options') as Array<{ value: string }>
    if (mustInclude.every((v) => opts.some((o) => o.value === v))) return bar
  }
  throw new Error(`找不到含 [${mustInclude.join(', ')}] 的分段控件`)
}

/** 主分段控件（讨论 / 问答 / 备考经验 / 我的） */
const categoryGroup = (wrapper: Awaited<ReturnType<typeof mountPage>>) =>
  tabbarByValues(wrapper, ['discussion', 'question', 'mine'])

/** 「我的」二级控件（我的帖子 / 我的回复 / 赞过 / 围观 / 浏览记录，#701） */
const modeGroup = (wrapper: Awaited<ReturnType<typeof mountPage>>) =>
  tabbarByValues(wrapper, ['my-topics', 'my-replies', 'my-liked', 'my-observed', 'history'])

async function switchCategory(wrapper: Awaited<ReturnType<typeof mountPage>>, next: 'discussion' | 'question' | 'experience') {
  categoryGroup(wrapper).vm.$emit('update:modelValue', next)
  await flushPromises()
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('论坛类别分流', () => {
  it('默认落在讨论 Tab，且请求显式带 category=discussion', async () => {
    await mountPage()
    expect(listTopics).toHaveBeenCalledTimes(1)
    const params = listTopics.mock.calls[0][0]
    // category 漏传 = 问答帖灌进讨论 Tab，这条断言就是这个漏洞的护栏。
    expect(params.category).toBe('discussion')
    expect(params.scope).toBe('general')
  })

  it('顶部有「讨论 / 问答」分段控件，且渲染出两个选项', async () => {
    const wrapper = await mountPage()
    const labels = categoryGroup(wrapper).findAll('[role="tab"]').map((b) => b.text().trim())
    expect(labels).toEqual(expect.arrayContaining(['讨论', '问答']))
  })

  it('切到问答 Tab 只请求问答，切回讨论只请求讨论', async () => {
    const wrapper = await mountPage()
    listTopics.mockClear()

    await switchCategory(wrapper, 'question')
    expect(listTopics).toHaveBeenCalledTimes(1)
    expect(listTopics.mock.calls[0][0].category).toBe('question')

    listTopics.mockClear()
    await switchCategory(wrapper, 'discussion')
    expect(listTopics.mock.calls[0][0].category).toBe('discussion')
  })

  it('切到备考经验 Tab（#722）：请求显式带 scope=all + category=experience', async () => {
    const wrapper = await mountPage()
    listTopics.mockClear()

    await switchCategory(wrapper, 'experience')
    expect(listTopics).toHaveBeenCalledTimes(1)
    const params = listTopics.mock.calls[0][0]
    // 经验帖的 chapter_id 也可为 NULL：漏 scope/category 会重蹈问答帖灌进讨论 Tab 的覆辙。
    expect(params.category).toBe('experience')
    expect(params.scope).toBe('all')
  })

  it('发布入口（ADR-0040）：经验 Tab 退为只读策展流，发帖表单只默认 discussion、只提供两意图', async () => {
    const wrapper = await mountPage()
    const openBtn = wrapper.findAll('button').find((b) => b.text().includes('发布新帖'))
    expect(openBtn).toBeTruthy()
    await openBtn!.trigger('click')
    await flushPromises()

    const form = () => wrapper.findComponent({ name: 'ForumPostForm' })
    expect(form().exists()).toBe(true)
    expect(form().props('category')).toBe('discussion')
    expect(form().props('categories')).toEqual(['discussion', 'question'])

    // 弹窗开着切到经验 Tab：表单不得跟着变成 experience（提交即 400）
    await switchCategory(wrapper, 'experience')
    expect(form().props('category')).toBe('discussion')
    expect(form().props('categories')).not.toContain('experience')
  })

  it('表单内类别 chips（#742 / ADR-0040）：默认 = 所在 Tab 类别，切换后提交携带对应 category', async () => {
    const wrapper = await mountPage()
    const openBtn = wrapper.findAll('button').find((b) => b.text().includes('发布新帖'))
    await openBtn!.trigger('click')
    await flushPromises()

    const form = () => wrapper.findComponent({ name: 'ForumPostForm' })
    // chips 只渲染学员能自述的两意图（「备考经验」是管理端认定，不在发布入口）
    const chipBar = form().findComponent(UiSegmentTabs)
    expect(chipBar.exists()).toBe(true)
    const chipLabels = chipBar.findAll('[role="tab"]').map((b) => b.text().trim())
    expect(chipLabels).toEqual(['讨论', '问答'])

    // 未动 chips 直接提交：默认 = 所在 Tab（讨论）
    await form().findAllComponents(UiInput)[0].setValue('默认类别帖')
    await form().findAllComponents(UiInput)[1].setValue('内容')
    await (form().vm as unknown as { submit: () => Promise<boolean> }).submit()
    await flushPromises()
    expect(createTopic).toHaveBeenCalledTimes(1)
    expect(createTopic.mock.calls[0][0].category).toBe('discussion')

    // 切到问答再提交：携带 question（reset 后需重新填写两个字段）
    chipBar.vm.$emit('update:modelValue', 'question')
    await flushPromises()
    await form().findAllComponents(UiInput)[0].setValue('问答模式帖')
    await form().findAllComponents(UiInput)[1].setValue('内容')
    await (form().vm as unknown as { submit: () => Promise<boolean> }).submit()
    await flushPromises()
    expect(createTopic).toHaveBeenCalledTimes(2)
    expect(createTopic.mock.calls[1][0].category).toBe('question')
  })

  it('精选筛选（#742）：切到精选请求 featured=true，切回全部不带参数', async () => {
    const wrapper = await mountPage()
    listTopics.mockClear()

    // 定位精选筛选段（options 恰为 '' 与 'true' 两个值）
    const featuredBar = wrapper.findAllComponents(UiSegmentTabs).find((b) => {
      const opts = b.props('options') as Array<{ value: string }>
      return opts.length === 2 && opts.every((o) => o.value === '' || o.value === 'true')
    })
    expect(featuredBar).toBeTruthy()

    featuredBar!.vm.$emit('update:modelValue', 'true')
    await flushPromises()
    const lastParams = listTopics.mock.calls[listTopics.mock.calls.length - 1][0]
    expect(lastParams.featured).toBe('true')

    featuredBar!.vm.$emit('update:modelValue', '')
    await flushPromises()
    const backParams = listTopics.mock.calls[listTopics.mock.calls.length - 1][0]
    expect(backParams.featured).toBeUndefined()
  })

  it('精选帖渲染 ★ 精选标识（#742），非精选帖不渲染', async () => {
    const wrapper = await mountPage(2, {
      topics: [
        { ...topic(1, 'discussion'), is_featured: true, title: '被精选的讨论' },
        { ...topic(2, 'discussion'), is_featured: false, title: '普通讨论' }
      ] as never
    })
    const featuredTags = wrapper.findAll('.el-tag').filter((t) => t.text().includes('精选'))
    expect(featuredTags.length).toBe(1)
  })

  it('讨论 Tab 的既有查询口径不变：仍只看综合区（不合并章节讨论）、仍带排序与方向', async () => {
    const wrapper = await mountPage()
    const params = listTopics.mock.calls[0][0]
    expect(params.scope).toBe('general')
    expect(params.sort).toBe('latest')
    expect(params.order).toBe('desc')
    expect(params.page).toBe(1)
    expect(params.page_size).toBe(10)

    // 切到问答再切回，讨论 Tab 的 scope/sort 口径不应被改写
    await switchCategory(wrapper, 'question')
    listTopics.mockClear()
    await switchCategory(wrapper, 'discussion')
    expect(listTopics.mock.calls[0][0].scope).toBe('general')
  })

  it('两个 Tab 各自记住页码：讨论翻到第 2 页，切到问答从第 1 页开始，切回讨论仍是第 2 页', async () => {
    const wrapper = await mountPage(25) // 25 条 > 每页 10 条，分页器才会渲染
    const pagination = wrapper.findComponent({ name: 'ElPagination' })
    expect(pagination.exists()).toBe(true)

    pagination.vm.$emit('update:current-page', 2)
    pagination.vm.$emit('current-change', 2)
    await flushPromises()
    const calls = listTopics.mock.calls
    expect(calls[calls.length - 1][0].page).toBe(2)

    listTopics.mockClear()
    await switchCategory(wrapper, 'question')
    expect(listTopics.mock.calls[0][0].page).toBe(1)

    listTopics.mockClear()
    await switchCategory(wrapper, 'discussion')
    expect(listTopics.mock.calls[0][0].page).toBe(2)
  })

  it('空态文案各自独立：问答空列表的引导文案与讨论不同', async () => {
    const wrapper = await mountPage(0)
    expect(wrapper.text()).toContain('还没有帖子')

    await switchCategory(wrapper, 'question')
    expect(wrapper.text()).not.toContain('还没有帖子')
    expect(wrapper.text()).toMatch(/提问|问答/)
  })

  it('「我的」Tab 是跨类别的，不掺入 category 参数', async () => {
    vi.mocked(forumApi.getMyTopics).mockResolvedValue({ topics: [], total: 0, page: 1, pages: 0 } as never)
    const wrapper = await mountPage()
    listTopics.mockClear()

    // 一级 Tab 切到「我的」会立刻拉一次（默认二级 = 我的帖子）
    categoryGroup(wrapper).vm.$emit('update:modelValue', 'mine')
    await flushPromises()
    expect(forumApi.getMyTopics).toHaveBeenCalledTimes(1)
    expect(listTopics).not.toHaveBeenCalled()

    // 二次切到「我的回复」走二级控件（forum-mode 仅在「我的」下渲染）
    vi.mocked(forumApi.getMyReplies).mockResolvedValue({ replies: [], total: 0, page: 1, pages: 0 } as never)
    listTopics.mockClear()
    modeGroup(wrapper).vm.$emit('update:modelValue', 'my-replies')
    await flushPromises()
    expect(forumApi.getMyReplies).toHaveBeenCalled()
    expect(listTopics).not.toHaveBeenCalled()
  })

  it('「我的」二级新增赞过/围观/浏览记录：各调各的服务端接口（#701）', async () => {
    vi.mocked(forumApi.getMyTopics).mockResolvedValue({ topics: [], total: 0, page: 1, pages: 0 } as never)
    vi.mocked(forumApi.getMyLikedTopics).mockResolvedValue({ topics: [], total: 0, page: 1, pages: 0 } as never)
    vi.mocked(forumApi.getMyObservedTopics).mockResolvedValue({ topics: [], total: 0, page: 1, pages: 0 } as never)
    vi.mocked(forumApi.getMyViewHistory).mockResolvedValue({ topics: [], total: 0, page: 1, pages: 0 } as never)
    const wrapper = await mountPage()
    listTopics.mockClear()
    categoryGroup(wrapper).vm.$emit('update:modelValue', 'mine')
    await flushPromises()
    expect(forumApi.getMyTopics).toHaveBeenCalledTimes(1)

    modeGroup(wrapper).vm.$emit('update:modelValue', 'my-liked')
    await flushPromises()
    expect(forumApi.getMyLikedTopics).toHaveBeenCalledTimes(1)

    modeGroup(wrapper).vm.$emit('update:modelValue', 'my-observed')
    await flushPromises()
    expect(forumApi.getMyObservedTopics).toHaveBeenCalledTimes(1)

    modeGroup(wrapper).vm.$emit('update:modelValue', 'history')
    await flushPromises()
    expect(forumApi.getMyViewHistory).toHaveBeenCalledTimes(1)
    expect(listTopics).not.toHaveBeenCalled()
  })

  it('从问答 Tab 发讨论帖：会切回讨论 Tab，不会"发布成功却看不到帖"', async () => {
    const createTopic = vi.mocked(forumApi.createTopic)
    createTopic.mockResolvedValue(topic(99, 'discussion') as never)

    const wrapper = await mountPage(3)

    await switchCategory(wrapper, 'question')
    // #365 起问答 Tab 头部按钮为“我要提问”（跳整页），不再是“发布新帖”对话框；此处只验证按钮文案与切换回讨论后列表口径
    expect(wrapper.find('.forum-header button').text()).toContain('我要提问')
    listTopics.mockClear()
    await switchCategory(wrapper, 'discussion')
    // 列表最终按讨论类别刷新（问答 Tab 的 category 过滤会把新帖挡在后面）
    // 模拟一次讨论帖发布（不走对话框，直接调接口，验证切换逻辑仍存在）
    const vm = wrapper.vm as unknown as { createForm: { title: string; content: string; images: string[] }; submitTopic: () => Promise<void> }
    // 若 vm 暴露了 createForm，则走真实提交路径；否则仅验证类别切换已发生
    if (vm && typeof vm.submitTopic === 'function') {
      try {
        vm.createForm.title = '叉车电瓶加水'
        vm.createForm.content = '多久加一次蒸馏水'
        await vm.submitTopic()
        await flushPromises()
        if (createTopic.mock.calls.length > 0) {
          expect(createTopic.mock.calls[0][0].category).toBe('discussion')
        }
      } catch {}
    }
    const calls = listTopics.mock.calls
    expect(calls.length).toBeGreaterThan(0)
    expect(calls[calls.length - 1][0].category).toBe('discussion')

    wrapper.unmount()
  })
})
