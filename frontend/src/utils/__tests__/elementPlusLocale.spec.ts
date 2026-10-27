// EP 语言包（#1971）：守住「中文包装上了」的可见后果，而不是只守住有一个常量。
// 背景：不传 locale 时 EP 组件内置文案是英文出厂值 —— 分页 `Total 26`、行内确认
// `Yes`/`No`、下拉 `No data` —— 与全站中文业务文案并存。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createApp, h, nextTick } from 'vue'
import type { App } from 'vue'
import { provideGlobalConfig } from 'element-plus/es/components/config-provider/index.mjs'
import ElMessageBox from 'element-plus/es/components/message-box/index.mjs'
import { epLite } from '@/test/element-lite'
import UiPagination from '@/components/ui/UiPagination.vue'
import { EP_INSTALL_OPTIONS, EP_LOCALE } from '../elementPlusLocale'

/**
 * 复刻 `app.use(ElementPlus, EP_INSTALL_OPTIONS)` 的那一步。
 * EP 的 make-installer 收到 options 时调的正是 `provideGlobalConfig(options, app, true)`，
 * 所以这里把同一份 options 交给同一个函数 —— 生产与测试共用入口，语言包一丢就红。
 * 真去 `import ElementPlus from 'element-plus'` 会把 barrel 整个拉进测试进程（epLite 存在的理由见
 * test/element-lite.ts），故只借这一步而不装全量插件。
 */
const installLocale = {
  install(app: App) {
    provideGlobalConfig(EP_INSTALL_OPTIONS, app, true)
  }
}

function mountPaged(props: { total: number; showSizes?: boolean }) {
  return mount(UiPagination, { props, global: { plugins: [epLite(), installLocale] } })
}

describe('EP 语言包（#1971）', () => {
  it('分页 total 渲染中文文案，不再是 EP 出厂的 `Total N`', () => {
    const w = mountPaged({ total: 26 })
    expect(w.text()).toContain('共 26 条')
    expect(w.text()).not.toContain('Total')
  })

  // 这条是「挂在 app.use 上、而不是根节点写 <el-config-provider>」的唯一理由：
  // 命令式创建的 MessageBox 不在组件树里，只有 provideGlobalConfig 的 global 分支
  // （模块级 globalConfig）够得到它。换成 el-config-provider 这条会红。
  it('命令式 ElMessageBox 的默认按钮也是中文（global 分支覆盖组件树之外）', async () => {
    const host = document.createElement('div')
    document.body.appendChild(host)
    const app = createApp({ render: () => h('div') })
    installLocale.install(app)
    app.mount(host)

    const pending = ElMessageBox.confirm('测试内容')
    pending.catch(() => {})
    await nextTick()
    await nextTick()
    await nextTick()
    const buttons = Array.from(document.querySelectorAll('.el-message-box__btns button')).map((b) =>
      b.textContent?.trim()
    )

    ElMessageBox.close()
    app.unmount()
    host.remove()
    expect(buttons).toEqual(['取消', '确定'])
  })

  // 值断言：锁住导出的是 zh-cn 那份包（导入路径写错会拿到别的语言，上面的渲染断言
  // 也可能因为某些字段恰好同形而放过）。行内确认的两个按钮是票面点名的第二处。
  it('导出的是 zh-cn 语言包，行内确认按钮为「确定 / 取消」', () => {
    expect(EP_LOCALE.name).toBe('zh-cn')
    expect(EP_LOCALE.el.popconfirm.confirmButtonText).toBe('确定')
    expect(EP_LOCALE.el.popconfirm.cancelButtonText).toBe('取消')
    expect(EP_INSTALL_OPTIONS.locale).toBe(EP_LOCALE)
  })
})
