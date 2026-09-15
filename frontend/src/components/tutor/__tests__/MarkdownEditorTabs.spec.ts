// Vditor 编辑器的模式 tab（#1017）：形态换成共享的 UiUnderlineTabs 之后，
// 守两件事：① 两档选项与文案没变（讲师/管理员的操作记忆）；
// ② Vditor 真正 ready 之前 tab 不可点 —— 早期切换会触发 VditorIRDOM2Md undefined，
//    这是历史上修过的坑，换控件不许把它带回来。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import MarkdownEditor from '../MarkdownEditor.vue'
import UiUnderlineTabs from '@/components/ui/UiUnderlineTabs.vue'

vi.mock('vditor/dist/index.css', () => ({}))
vi.mock('vditor/dist/js/i18n/zh_CN.js', () => ({}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ token: 'test-token' }) }))

const vditorCtor = vi.hoisted(() => vi.fn())
vi.mock('vditor', () => ({ default: vditorCtor }))

interface VditorOptions {
  mode?: string
  value?: string
  after?: () => void
}

interface VditorInstance {
  options: VditorOptions
  destroy: ReturnType<typeof vi.fn>
}

/** 安装 Vditor 替身；autoReady=false 时永不触发 after（模拟模块未就绪） */
function installVditorMock(autoReady = true): VditorInstance[] {
  const instances: VditorInstance[] = []
  vditorCtor.mockImplementation(function (this: Record<string, unknown>, _el: HTMLElement, opts: VditorOptions) {
    const destroy = vi.fn()
    instances.push({ options: opts, destroy })
    this.destroy = destroy
    this.getValue = () => opts.value ?? ''
    this.setValue = vi.fn()
    if (autoReady && opts.after) queueMicrotask(() => opts.after?.())
  })
  return instances
}

function mountEditor() {
  return mount(MarkdownEditor, {
    props: { modelValue: '正文', height: 400 },
    global: { plugins: [epLite()] }
  })
}

describe('MarkdownEditor 模式 tab（预览编辑 / 源码）', () => {
  beforeEach(() => vi.clearAllMocks())

  it('两档文案与顺序不变，默认停在「预览编辑」', () => {
    installVditorMock()
    const w = mountEditor()
    const tabs = w.findComponent(UiUnderlineTabs)
    expect(tabs.exists()).toBe(true)
    expect(tabs.findAll('button').map(b => b.text())).toEqual(['预览编辑', '源码'])
    expect(tabs.findAll('button')[0].attributes('aria-selected')).toBe('true')
  })

  it('Vditor 未 ready 前 tab 置灰（避免 VditorIRDOM2Md undefined）', () => {
    installVditorMock(false)
    const w = mountEditor()
    for (const btn of w.findComponent(UiUnderlineTabs).findAll('button')) {
      expect(btn.attributes('disabled')).toBeDefined()
    }
  })

  it('ready 后可切到源码：Vditor 以 mode=sv 重建，旧实例被销毁', async () => {
    const instances = installVditorMock()
    const w = mountEditor()
    await flushPromises()
    expect(instances).toHaveLength(1)
    expect(instances[0].options.mode).toBe('ir')

    const sourceTab = w.findComponent(UiUnderlineTabs).findAll('button')[1]
    expect(sourceTab.attributes('disabled')).toBeUndefined()
    await sourceTab.trigger('click')
    await flushPromises()

    expect(instances).toHaveLength(2)
    expect(instances[1].options.mode).toBe('sv')
    expect(instances[0].destroy).toHaveBeenCalled()
    // 切档后把当前值带进新实例，内容不丢
    expect(instances[1].options.value).toBe('正文')
  })
})
