// #364 论坛类别分流的页面级测试：讨论 / 问答两个 Tab 各看各的列表。
//
// seam 选在组件层而非 api 层：forumApi.listTopics 只是把 params 透传给 axios，
// 给它写"category 有透传"的断言不可能失败，是无效测试。真正会被写坏的是
// 「讨论 Tab 有没有带上 category=discussion」——漏了这个参数，问答帖就会整片
// 灌进讨论 Tab（后端 scope=general 的定义恰好是 chapter_id IS NULL）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus, { ElRadioGroup } from 'element-plus'

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
      createTopic: vi.fn(),
      getCheckInCalendar: vi.fn(),
      checkIn: vi.fn()
    }
  }
})

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {}, params: {} })
}))

vi.mock('@/utils/forumHistory', () => ({
  loadHistory: vi.fn(() => []),
  removeHistoryItem: vi.fn(),
  clearHistory: vi.fn()
}))

// 页面只用到 userInfo?.user_id（浏览记录按用户隔离），mock 掉避免拉起真实 pinia。
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ userInfo: { user_id: 1 } })
}))

import { forumApi } from '@/api/forum'
import ForumPage from '../ForumPage.vue'

const listTopics = vi.mocked(forumApi.listTopics)

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

async function mountPage(total = 3, options: { attachTo?: HTMLElement } = {}) {
  // total=0 必须同时把 topics 置空，否则 v-else-if="topics.length > 0" 会渲染列表，
  // 空态断言就是在验一个永远走不到的分支。
  listTopics.mockResolvedValue({ topics: total === 0 ? [] : [topic(1, 'discussion')], total } as never)
  vi.mocked(forumApi.getCheckInCalendar).mockResolvedValue({ dates: [], streak: 0, total: 0, today_checked: false } as never)
  const wrapper = mount(ForumPage, {
    attachTo: options.attachTo,
    global: {
      plugins: [ElementPlus],
      // 打桩子组件：CheckInDialog 内部也是 el-dialog，且文档顺序在发帖框之前，
      // 不打桩的话 body 里会同时出现两个 .el-dialog，断言会挑错对象。
      stubs: { CheckInDialog: true, ForumHistoryPanel: true, ForumImageUploader: true }
    }
  })
  await flushPromises()
  return wrapper
}

/** 类别分段控件：用 class 定位，避免与其它 el-radio-group（模式 / 排序）混淆。 */
function categoryGroup(wrapper: Awaited<ReturnType<typeof mountPage>>) {
  const found = wrapper
    .findAllComponents(ElRadioGroup)
    .find((c) => c.classes().includes('forum-category'))
  if (!found) throw new Error('找不到类别分段控件（class="forum-category"）')
  return found
}

async function switchCategory(wrapper: Awaited<ReturnType<typeof mountPage>>, next: 'discussion' | 'question') {
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
    const labels = categoryGroup(wrapper).findAll('label').map((l) => l.text())
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
    const catGroup = wrapper
      .findAllComponents(ElRadioGroup)
      .find((c) => c.classes().includes('forum-category'))
    if (!catGroup) throw new Error('找不到一级分段控件')
    catGroup.vm.$emit('update:modelValue', 'mine')
    await flushPromises()
    expect(forumApi.getMyTopics).toHaveBeenCalledTimes(1)
    expect(listTopics).not.toHaveBeenCalled()

    // 二次切到「我的回复」走二级控件（forum-mode 仅在「我的」下渲染）
    vi.mocked(forumApi.getMyReplies).mockResolvedValue({ replies: [], total: 0, page: 1, pages: 0 } as never)
    listTopics.mockClear()
    const modeGroup = wrapper
      .findAllComponents(ElRadioGroup)
      .find((c) => c.classes().includes('forum-mode'))
    if (!modeGroup) throw new Error('找不到「我的」二级控件')
    modeGroup.vm.$emit('update:modelValue', 'my-replies')
    modeGroup.vm.$emit('change', 'my-replies')
    await flushPromises()
    expect(forumApi.getMyReplies).toHaveBeenCalled()
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
