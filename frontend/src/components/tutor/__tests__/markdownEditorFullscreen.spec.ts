// 全屏退不出（#2631 需求 ④）。
//
// 两处都得测，因为它们坏掉的表现完全不同：
//   ① `fullscreen.index` 低于应用顶栏的 z-index（--z-fixed=1030）⇒ 退出按钮被顶栏盖住，
//      用户点不到、表现为「全屏后退不出」。这一条只能靠断言传进去的值来守 ——
//      探针 fs4.html 已实测：index=90 时 elementFromPoint 命中顶栏，抬高后命中按钮。
//   ② Vditor 的 destroy 不清 `document.body.style.overflow` ⇒ 在全屏态切模式/切主题
//      （本组件 destroy + 重建）会把整页永久锁死滚动。这一条测我们把它还回去。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import MarkdownEditor from '../MarkdownEditor.vue'
import UiUnderlineTabs from '@/components/ui/UiUnderlineTabs.vue'

vi.mock('vditor/dist/index.css', () => ({}))
vi.mock('vditor/dist/js/i18n/zh_CN.js', () => ({}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ token: 'test-token' }) }))
vi.mock('@/stores/theme', () => ({ useThemeStore: () => ({ resolved: 'light' }) }))

const vditorCtor = vi.hoisted(() => vi.fn())
vi.mock('vditor', () => ({ default: vditorCtor }))

interface FakeInstance {
  options: { fullscreen?: { index: number }; after?: () => void }
  element: HTMLElement
  destroy: ReturnType<typeof vi.fn>
}

/**
 * 装 Vditor 替身。`element` 是实例自己的根节点：真组件用 classList 判全屏态，
 * 这里也照做，好让 releaseBodyScrollLock 的判据能被真实走到。
 */
function installMock(): FakeInstance[] {
  const instances: FakeInstance[] = []
  vditorCtor.mockImplementation(function (this: Record<string, unknown>, _el: HTMLElement, opts: FakeInstance['options']) {
    const element = document.createElement('div')
    element.className = 'vditor'
    const destroy = vi.fn()
    this.destroy = destroy
    this.getValue = () => ''
    this.setValue = vi.fn()
    this.focus = vi.fn()
    this.tip = vi.fn()
    this.vditor = { element, currentMode: 'ir', options: {}, ir: {}, undo: {} }
    instances.push({ options: opts, element, destroy })
    queueMicrotask(() => opts.after?.())
  })
  return instances
}

function mountEditor() {
  return mount(MarkdownEditor, {
    props: { modelValue: '正文', height: 400 },
    global: { plugins: [epLite()] }
  })
}

describe('全屏层级与滚动锁', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    document.body.style.overflow = ''
    // 剥掉测试环境里可能被别的用例写进来的令牌值
    document.documentElement.style.removeProperty('--z-fixed')
  })

  it('fullscreen.index 高于顶栏层级（--z-fixed），否则退出按钮被顶栏盖住', async () => {
    document.documentElement.style.setProperty('--z-fixed', '1030')
    const instances = installMock()
    mountEditor()
    await flushPromises()
    expect(instances[0].options.fullscreen?.index).toBe(1031)
  })

  it('令牌读不到时也要高于 1030（写死兜底，不能退回 Vditor 默认的 90）', async () => {
    const instances = installMock()
    mountEditor()
    await flushPromises()
    const index = instances[0].options.fullscreen?.index ?? 0
    expect(index).toBeGreaterThan(1030)
  })

  it('令牌调整后跟着走：--z-fixed 变 2000 时 index 变 2001', async () => {
    document.documentElement.style.setProperty('--z-fixed', '2000')
    const instances = installMock()
    mountEditor()
    await flushPromises()
    expect(instances[0].options.fullscreen?.index).toBe(2001)
  })

  it('全屏态切模式：销毁前把 body 的滚动锁还回去（Vditor destroy 不管这个）', async () => {
    const instances = installMock()
    const w = mountEditor()
    await flushPromises()
    // 模拟进了全屏：Vditor 会加 class + 锁 body
    instances[0].element.classList.add('vditor--fullscreen')
    document.body.style.overflow = 'hidden'

    await w.findComponent(UiUnderlineTabs).findAll('button')[1].trigger('click')
    await flushPromises()

    expect(instances[0].destroy).toHaveBeenCalled()
    expect(document.body.style.overflow).toBe('')
  })

  it('全屏态卸载组件：同样还回滚动锁', async () => {
    const instances = installMock()
    const w = mountEditor()
    await flushPromises()
    instances[0].element.classList.add('vditor--fullscreen')
    document.body.style.overflow = 'hidden'

    w.unmount()

    expect(document.body.style.overflow).toBe('')
  })

  it('不在全屏态时不动 body：别把别处设的 overflow 顺手清了', async () => {
    installMock()
    const w = mountEditor()
    await flushPromises()
    document.body.style.overflow = 'scroll'

    w.unmount()

    expect(document.body.style.overflow).toBe('scroll')
  })
})