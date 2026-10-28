// ForumComposer 正文格式（#879 / ADR-0044）。
//
// 守三件事：① 首次默认纯文本；② 切换改变提交载荷（父级据此调 replyTopic）；
// ③ **与发帖共用同一个格式偏好**——同一个人对正文格式的偏好与他写的是主题还是回复无关，
// 分两个键只会产生「发帖选了 Markdown、回复却还是纯文本」这种莫名其妙的不一致。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import UiInput from '@/components/ui/UiInput.vue'
import { epLite } from '@/test/element-lite'
import ForumPostForm from '../ForumPostForm.vue'
import ForumContent from '../ForumContent.vue'
import ForumComposer from '../ForumComposer.vue'
import { FORUM_REGION_NOTICE } from '@/utils/forumDisplay'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn(), put: vi.fn() }
}))
vi.mock('markstream-vue/index.css', () => ({}))
vi.mock('@/composables/useForumImageUpload', () => ({
  useForumImageUpload: () => ({
    uploading: false,
    // #1017：拖拽三件套 + 高亮状态也在单点里（缺字段会让虚线区渲染直接炸）
    dragging: { value: false },
    uploadFiles: vi.fn(),
    removeImage: vi.fn(),
    handlePaste: vi.fn(),
    handleDragOver: vi.fn(),
    handleDragLeave: vi.fn(),
    handleDrop: vi.fn()
  })
}))

/** 预览入口 = 下划线 tab 里的「预览」项（#1017 起取代原来的文字按钮） */
function previewTab(w: ReturnType<typeof mountComposer>) {
  return w
    .findAllComponents({ name: 'UiUnderlineTabs' })[0]
    .findAll('button')
    .find((b) => b.text().includes('预览'))
}

function mountComposer() {
  return mount(ForumComposer, {
    props: { modelValue: '## 回复内容', images: [], replyingTo: null, rows: 2 },
    global: { plugins: [epLite()] }
  })
}

describe('回复框正文格式（#879 / ADR-0044）', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('首次默认纯文本，提交载荷 contentFormat=text', async () => {
    const w = mountComposer();
    expect(w.text()).not.toContain('预览');
    await w.find('.forum-composer button[title*="发表回复"]').trigger('click');
    await flushPromises();
    expect(w.emitted('submit')?.[0]?.[0]).toEqual({ contentFormat: 'text' });
  })

  it('切到 Markdown 后提交载荷 contentFormat=markdown', async () => {
    const w = mountComposer();
    const tabs = w.findAllComponents({ name: 'UiSegmentTabs' })[0];
    tabs.vm.$emit('update:modelValue', 'markdown');
    await flushPromises();
    await w.find('.forum-composer button[title*="发表回复"]').trigger('click');
    await flushPromises();
    expect(w.emitted('submit')?.[0]?.[0]).toEqual({ contentFormat: 'markdown' });
  })

  it('选 Markdown 才出现编写/预览 tab；预览复用 UGC 渲染单点', async () => {
    const w = mountComposer();
    // 纯文本档：连 tab 都不存在（没有可预览的渲染结果）
    expect(w.findAllComponents({ name: 'UiUnderlineTabs' })).toHaveLength(0);
    w.findAllComponents({ name: 'UiSegmentTabs' })[0].vm.$emit('update:modelValue', 'markdown');
    await flushPromises();
    const btn = previewTab(w);
    expect(btn).toBeTruthy();
    expect(w.findComponent(ForumContent).exists()).toBe(false);
    await btn!.trigger('click');
    await flushPromises();
    const fc = w.findComponent(ForumContent);
    expect(fc.exists()).toBe(true);
    expect(fc.props('format')).toBe('markdown');
    expect(fc.props('content')).toContain('## 回复内容');
  })

  it('提交后退出预览态（父级清空正文，停在空预览会让人以为发丢了）', async () => {
    const w = mountComposer();
    w.findAllComponents({ name: 'UiSegmentTabs' })[0].vm.$emit('update:modelValue', 'markdown');
    await flushPromises();
    const btn = previewTab(w);
    await btn!.trigger('click');
    await flushPromises();
    expect(w.findComponent(ForumContent).exists()).toBe(true);

    await w.find('.forum-composer button[title*="发表回复"]').trigger('click');
    await flushPromises();
    // 回到编辑态：渲染单点不再占位，输入框回来了
    expect(w.findComponent(ForumContent).exists()).toBe(false);
    expect(w.findAllComponents(UiInput).length).toBeGreaterThan(0);
  });

  it('与发帖共用同一个格式偏好：发帖选了 Markdown，回复框也是 Markdown', async () => {
    const post = mount(ForumPostForm, {
      props: { category: 'discussion' },
      global: { plugins: [epLite()] }
    });
    post.findAllComponents({ name: 'UiSegmentTabs' })[0].vm.$emit('update:modelValue', 'markdown');
    await flushPromises();
    post.unmount();

    const composer = mountComposer();
    expect(composer.findAllComponents({ name: 'UiSegmentTabs' })[0].props('modelValue')).toBe('markdown');
  })
})
// 行内小回复框（inline 档，2026-10-28）：帖子详情里点某条回复的「回复」键就是挂它。
// 与主回复区的差异只有三处：没有格式档、披露与发送挤同一行、挂载即聚焦。
describe('行内小回复框（inline 档）', () => {
  function mountInline() {
    return mount(ForumComposer, {
      props: { modelValue: '', images: [], inline: true, rows: 2 },
      attachTo: document.body,
      global: { plugins: [epLite()] }
    })
  }

  it('不放正文格式分段控件（格式偏好与主回复区共用一份）', () => {
    const w = mountInline()
    expect(w.findAllComponents({ name: 'UiSegmentTabs' })).toHaveLength(0)
  })

  it('披露提示与发送键在同一行（行内档不能再长出一行）', () => {
    const w = mountInline()
    const send = w.find('.forum-composer button[title*="发表回复"]')
    expect(send.exists()).toBe(true)
    const row = send.element.parentElement!
    expect(row.textContent).toContain(FORUM_REGION_NOTICE)
    // 主回复区那一份披露在**下一行**，行内档不该同时存在两份
    expect(w.findAll('.forum-region-notice')).toHaveLength(1)
  })

  it('挂载即聚焦输入框（点「回复」的下一步就是打字）', async () => {
    const w = mountInline()
    await flushPromises()
    expect(document.activeElement).toBe(w.find('textarea').element)
    w.unmount()
  })

  it('主回复区：发送键与格式档同一行，不再独占最后一行', () => {
    const w = mountComposer()
    const send = w.find('.forum-composer button[title*="发表回复"]')
    const row = send.element.parentElement!
    // 同一行里既有格式档（.seg-bar）也有发送键
    expect(row.querySelector('.seg-bar')).toBeTruthy()
    // 披露提示仍在下方一行（ADR-0045 的披露位置不变）
    expect(w.findAll('.forum-region-notice')).toHaveLength(1)
    expect(row.textContent).not.toContain(FORUM_REGION_NOTICE)
  })
})
