/**
 * Element Plus 的语言包与安装参数（全站唯一挂载点，#1971）。
 *
 * 不传 `locale` 时，EP 所有**组件内置文案**走英文出厂值 —— 分页的 `Total 26`、
 * 行内确认的 `Yes` / `No`、下拉的 `No data`、日期选择器的英文月份 —— 而我们的业务
 * 文案全是中文，屏幕上于是长期两种语言并存。`UiPagination` 的 `showTotal` 注释写的
 * 是「是否显示『共 X 条』」，修之前它渲染的正是 EP 的 `Total {total}`。
 *
 * 为什么挂在 `app.use(ElementPlus, …)` 而不是根节点写 `<el-config-provider>`：
 * `useConfirm` 的三个方法都通过 `ElMessageBox` **命令式**创建，不在组件树里。
 * `el-config-provider` 只对子树生效、够不到它们；而 `app.use` 的 options 会被
 * `make-installer` 转成 `provideGlobalConfig(options, app, global = true)`，
 * 既写 app 级 provide（组件树内），又写模块级 `globalConfig`（命令式创建读它），
 * 两条路径一并覆盖。
 *
 * 回归面：分页 total / 每页条数、`el-popconfirm` 按钮、`el-select` 空态、
 * `el-date-picker` 月份、`el-table` 默认空文案、`ElMessageBox` 按钮。
 */
import zhCn from 'element-plus/es/locale/lang/zh-cn'

/** 中文语言包（EP 2.x 子路径导入；`name` 字段即 'zh-cn'） */
export const EP_LOCALE = zhCn

/** `app.use(ElementPlus, EP_INSTALL_OPTIONS)` 的参数 */
export const EP_INSTALL_OPTIONS = { locale: EP_LOCALE }
