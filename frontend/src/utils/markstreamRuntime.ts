/**
 * markstream 运行时开启单点（ADR-0046 / #900）。
 *
 * 全仓**唯一**允许调用 `enableKatex` / `enableMermaid` / `setDefaultI18nMap` 的地方。
 * 这几个开关写的是 markstream 的**进程级全局状态**，散在各渲染组件里会变成
 * 「谁后挂载谁说了算」——同一个内容面在不同入口下能力不同，正是本批要消灭的东西。
 * 启用只在应用启动处发生一次（`main.ts`），且幂等。
 *
 * 为什么三个可选 peer 一律**动态 import**（#900 硬约束）：
 *   - katex（约 270KB）/ mermaid（约 1MB）是首包体积的主要成本，而「有没有公式」对
 *     绝大多数页面是未知的——必须等到真的出现公式/图表那一帧才拉取；
 *   - `stream-diffs`（增强代码块）由库内部对 `stream-diffs/markstream` 做动态 import，
 *     装包即生效、没有显式开关，因此本文件**不** import 它——静态引入它会把
 *     这块运行时拖进首包，那正是要避免的。
 * 这里显式写 loader 而不是依赖库的默认值：让「动态 import」成为本仓库里**可见的契约**，
 * 哪天有人把它换成静态 import，构建产物的 chunk 清单会立刻暴露。
 *
 * katex 的**样式表**在这里全局引入：markstream 的公式面（AI 助手 / 论坛）与章节的
 * marked 面都要用同一份 katex CSS，放进任一懒加载面都会让另一面缺样式、或让同一份
 * CSS 在多个 chunk 里重复。它是纯 CSS（约 23KB）且字体按需加载，不含 JS 依赖，
 * 因此不改变「重型 peer 不进首屏」这条约束。
 */
import 'katex/dist/katex.min.css'

/**
 * markstream 内置文案的中文映射（#900）。
 *
 * 用 `setDefaultI18nMap` 而不是装 `vue-i18n`：后者是完整 i18n 框架（含复数、插值、
 * 语言切换），而这里只需要把十几个写死的英文按钮/提示换成中文，代价不对等。
 * 键名取自库的实际调用点（`common.*` / `image.*` / `artifacts.*`），
 * 库未提供的键（如 `common.fontSmaller`）也一并给出，否则会回落到按驼峰拆词的英文。
 *
 * 已知覆盖不到的：代码块的 `aria-label="Code block: js"` 与图表的「Mermaid」标题
 * 是库内**静态字面量**，不走 i18n 表——本批不改（要改只能提上游）。
 */
export const MARKSTREAM_I18N_ZH: Readonly<Record<string, string>> = {
  'common.copy': '复制',
  'common.selectCopy': '全选（复制）',
  'common.copied': '已复制',
  'common.more': '更多',
  'common.decrease': '减小',
  'common.reset': '重置',
  'common.increase': '增大',
  'common.expand': '展开',
  'common.collapse': '收起',
  'common.preview': '预览',
  'common.source': '源码',
  'common.export': '导出',
  'common.open': '打开',
  'common.minimize': '最小化',
  'common.zoomIn': '放大',
  'common.zoomOut': '缩小',
  'common.resetZoom': '重置缩放',
  'common.fontSmaller': '减小字号',
  'common.fontReset': '重置字号',
  'common.fontLarger': '增大字号',
  'image.preview': '预览图片',
  'image.loadError': '图片加载失败',
  'image.loading': '图片加载中…',
  'artifacts.htmlPreviewTitle': 'HTML 预览',
  'artifacts.svgPreviewTitle': 'SVG 预览'
}

/**
 * mermaid 的安全级**在本仓显式固定**（ADR-0046 安全口径）。
 *
 * markstream 的 MermaidBlockNode 默认就是 isStrict: true（strict 下禁用脚本、禁事件属性、
 * 不开 htmlLabels），但「默认值」不是契约：上游翻一次默认值，学员 UGC 与模型输出里的图表
 * 就会静默变成可执行内容。这里把它写成本仓的显式输入，各内容面引用同一个常量——
 * 它属于安全旋钮，属于本文档说的「唯一开启点」。
 */
export const MARKSTREAM_MERMAID_PROPS = { isStrict: true } as const

let applied: Promise<void> | null = null

/**
 * 应用启动时调用一次（`main.ts`，`void setupMarkstreamRuntime()`）。重复调用返回同一个 Promise。
 *
 * 为什么连 markstream 本体也走动态 import：它是 937KB 的 chunk，而**入口 chunk 的静态依赖
 * 会被每一个路由 chunk 继承**（Rollup 的 hoistTransitiveImports，实测：把它静态挂到
 * `main.ts` 之后，连登录页都多出 `markstream/katex/mermaid/markdown` 四条静态 import）。
 * 这与「重型 peer 不拖首屏」是同一件事，只是漏在上一层——**开启动作发生在启动处，
 * 不等于开启动作的代码必须进首包**。
 *
 * 代价与兜底：开启是异步的（差一个 chunk 的加载时间）。首屏渲染时若内容面已经挂载完，
 * 极早期渲染的公式/图表会先按降级形态（源码 / 纯文本）出现——按 ADR-0046 属
 * 「降级仍可读」。实际时序上内容来自接口，开启总是先完成。
 */
export function setupMarkstreamRuntime(): Promise<void> {
  applied ??= import('markstream-vue').then(({ enableKatex, enableMermaid, setDefaultI18nMap }) => {
    enableKatex(() => import('katex'))
    enableMermaid(() => import('mermaid'))
    setDefaultI18nMap({ ...MARKSTREAM_I18N_ZH })
  })
  return applied
}
