// ForumComposer 正文格式（#879 / ADR-0044）。
//
// 守三件事：① 首次默认纯文本；② 切换改变提交载荷（父级据此调 replyTopic）；
// ③ **与发帖共用同一个格式偏好**——同一个人对正文格式的偏好与他写的是主题还是回复无关，
// 分两个键只会产生「发帖选了 Markdown、回复却还是纯文本」这种莫名其妙的不一致。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import ForumPostForm from '../ForumPostForm.vue'
import ForumContent from '../ForumContent.vue'
import ForumComposer from '../ForumComposer.vue'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn(), put: vi.fn() }
}))
vi.mock('markstream-vue/index.css', () => ({}))
vi.mock('@/composables/useForumImageUpload', () => ({
  useForumImageUpload: () => ({
    uploading: false,
    uploadFiles: vi.fn(),
    removeImage: vi.fn(),
    handlePaste: vi.fn()
  })
}))

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

  it('选 Markdown 才出现预览；预览复用 UGC 渲染单点', async () => {
    const w = mountComposer();
    expect(w.findAll('button').find((b) => b.text().includes('预览'))).toBeUndefined();
    w.findAllComponents({ name: 'UiSegmentTabs' })[0].vm.$emit('update:modelValue', 'markdown');
    await flushPromises();
    const btn = w.findAll('button').find((b) => b.text().includes('预览'));
    expect(btn).toBeTruthy();
    expect(w.findComponent(ForumContent).exists()).toBe(false);
    await btn!.trigger('click');
    await flushPromises();
    const fc = w.findComponent(ForumContent);
    expect(fc.exists()).toBe(true);
    expect(fc.props('format')).toBe('markdown');
    expect(fc.props('content')).toContain('## 回复内容');
  })

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
