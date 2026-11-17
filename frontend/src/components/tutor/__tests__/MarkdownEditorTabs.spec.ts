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
// 编辑器按主题选代码块高亮样式（github / github-dark）：store 的 resolved 就够了
vi.mock('@/stores/theme', () => ({ useThemeStore: () => ({ resolved: 'light' }) }))

const vditorCtor = vi.hoisted(() => vi.fn())
vi.mock('vditor', () => ({ default: vditorCtor }))

interface VditorOptions {
  mode?: string
  value?: string
  toolbar?: unknown[]
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

function mountEditor(extraProps: Record<string, unknown> = {}) {
  return mount(MarkdownEditor, {
    props: { modelValue: '正文', height: 400, ...extraProps },
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

describe('MarkdownEditor 内容排版按钮（contentStyles）', () => {
  beforeEach(() => vi.clearAllMocks())

  /** 从 Vditor 选项里挑出自定义项（字符串项是预置按钮名） */
  function customItems(options: VditorOptions) {
    return (options.toolbar ?? []).filter((item): item is { name: string; tip: string; click: unknown } =>
      typeof item === 'object' && item !== null && 'name' in item
    )
  }

  it('默认不给内容排版按钮：产出内嵌 HTML，移动端与论坛面不渲染', async () => {
    const instances = installVditorMock()
    mountEditor()
    await flushPromises()
    expect(customItems(instances[0].options)).toHaveLength(0)
  })

  it('contentStyles 打开后追加 5 个自定义项：两个对齐 + 图片居中 + 颜色 + 字号', async () => {
    const instances = installVditorMock()
    mountEditor({ contentStyles: true })
    await flushPromises()
    expect(customItems(instances[0].options).map((i) => i.name)).toEqual([
      'align-center',
      'align-left',
      'image-center',
      'color',
      'font-size'
    ])
  })

  it('预置按钮顺序不受影响：内容排版项只插在表格/代码之后、撤销之前', async () => {
    const instances = installVditorMock()
    mountEditor({ contentStyles: true })
    await flushPromises()
    const toolbar = (instances[0].options.toolbar ?? []) as Array<string | { name: string }>
    const names = toolbar.map((i) => (typeof i === 'string' ? i : i.name))
    expect(names.indexOf('table')).toBeLessThan(names.indexOf('align-center'))
    expect(names.indexOf('font-size')).toBeLessThan(names.indexOf('undo'))
    expect(names[names.length - 1]).toBe('preview')
  })

  it('颜色与字号是子菜单，各带五档 / 四档', async () => {
    const instances = installVditorMock()
    mountEditor({ contentStyles: true })
    await flushPromises()
    const items = customItems(instances[0].options) as Array<{ name: string; toolbar?: unknown[] }>
    const color = items.find((i) => i.name === 'color')
    const font = items.find((i) => i.name === 'font-size')
    expect(color?.toolbar).toHaveLength(5)
    expect(font?.toolbar).toHaveLength(4)
  })

  it('点击「文本居中」把选区包成 display:block 外壳（命令走真实实例，不碰内部 IVditor）', async () => {
    const instances = installVditorMock()
    const w = mountEditor({ contentStyles: true })
    await flushPromises()

    // 给替身补上命令用到的两个方法（真实实例上有，替身此前只做了 getValue/setValue）
    const instance = vditorCtor.mock.instances[0] as unknown as Record<string, unknown>
    const insertMD = vi.fn()
    instance.getSelection = () => '选中的文字'
    instance.insertMD = insertMD
    instance.focus = vi.fn()

    const items = customItems(instances[0].options) as Array<{ name: string; click: (e: Event, v: unknown) => void }>
    const center = items.find((i) => i.name === 'align-center')!
    // 第二个参数刻意传内部替身：命令必须忽略它、用闭包里的真实实例
    center.click(new Event('click'), { notAVditor: true })
    void w

    expect(insertMD).toHaveBeenCalledWith(
      '<span style="display:block;text-align:center">选中的文字</span>'
    )
  })
})
