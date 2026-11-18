// 编辑器「打开已发布内容时空白」缺陷的复现（TASK: 红 → 修 → 绿）。
//
// 用户症状：打开已有内容的编辑页，编辑器里什么都没加载。
//
// 真实时序（生产 / 本地全栈都成立）：
//   1. 父组件 onMounted 里发请求取详情（FeaturedContentEdit.vue:410 → loadDetail）；
//   2. Vditor 的模块（lute 等）要从 /vditor/dist/js/... 异步拉，after 回调比请求晚；
//   3. 请求先回来 ⇒ 父组件把 form.content 从 '' 改成正文 ⇒ 走到本组件的
//      watch(() => props.modelValue)；
//   4. 那时 isReady 还是 false ⇒ 第 286 行 `if (!vditor || !isReady.value) return`
//      把这次更新整个丢掉；after 回调里只恢复 valueToRestore（切模式才传），
//      首屏不传 ⇒ 内容永久丢失，编辑器停在创建时的空串。
//
// 这里的替身刻意让 after **延迟**触发（默认替身是 queueMicrotask，永远抢在
// 父组件赋值之前，所以现有用例全绿、掩盖了这个缺陷）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import MarkdownEditor from '../MarkdownEditor.vue'

vi.mock('vditor/dist/index.css', () => ({}))
vi.mock('vditor/dist/js/i18n/zh_CN.js', () => ({}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ token: 'test-token' }) }))
vi.mock('@/stores/theme', () => ({ useThemeStore: () => ({ resolved: 'light' }) }))

const vditorCtor = vi.hoisted(() => vi.fn())
vi.mock('vditor', () => ({ default: vditorCtor }))

interface Instance {
  opts: { value?: string; after?: () => void }
  setValue: ReturnType<typeof vi.fn>
  getValue: () => string
}

/**
 * 安装替身并**不自动 ready** —— 返回一个 ready() 让用例自己决定 after 何时触发，
 * 从而模拟「详情请求比 Vditor 模块先回来」这个真实顺序。
 */
function installDeferredMock(): { instances: Instance[]; ready: () => Promise<void> } {
  const instances: Instance[] = []
  vditorCtor.mockImplementation(function (this: Record<string, unknown>, _el: HTMLElement, opts: Instance['opts']) {
    const inst: Instance = {
      opts,
      setValue: vi.fn((v: string) => {
        inst.opts.value = v
      }),
      getValue: () => inst.opts.value ?? ''
    }
    instances.push(inst)
    this.destroy = vi.fn()
    this.getValue = inst.getValue
    this.setValue = inst.setValue
  })
  const ready = async () => {
    instances.forEach((i) => i.opts.after?.())
    await flushPromises()
  }
  return { instances, ready }
}

describe('MarkdownEditor 首屏加载已有内容', () => {
  beforeEach(() => vi.clearAllMocks())

  it('内容在 Vditor ready 之前到达时，不能被丢掉（用户症状：编辑器空白）', async () => {
    const { instances, ready } = installDeferredMock()
    const w = mount(MarkdownEditor, {
      props: { modelValue: '', height: 400 },
      global: { plugins: [epLite()] }
    })
    // 创建时是空内容
    expect(instances).toHaveLength(1)
    expect(instances[0].opts.value).toBe('')

    // 详情请求先回来：父组件把正文写进 modelValue（此时 Vditor 还没 ready）
    await w.setProps({ modelValue: '第一段\n\n第二段\n' })
    await flushPromises()

    // Vditor 模块这才就绪
    await ready()

    // 断言用户看到的东西：编辑器里的值必须是正文，而不是创建时的空串
    expect(instances[0].getValue()).toBe('第一段\n\n第二段\n')
    w.unmount()
  })

  it('ready 之后到达的内容照常同步（不能为了修首屏而破坏正常回填）', async () => {
    const { instances, ready } = installDeferredMock()
    const w = mount(MarkdownEditor, {
      props: { modelValue: '', height: 400 },
      global: { plugins: [epLite()] }
    })
    await ready()

    await w.setProps({ modelValue: '后来改的正文' })
    await flushPromises()

    expect(instances[0].getValue()).toBe('后来改的正文')
    w.unmount()
  })
})
