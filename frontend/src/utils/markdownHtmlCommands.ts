/**
 * 内容精选编辑器的**富文本样式命令**（纯函数，ADR-0046 内容面扩展）。
 *
 * 与 `utils/markdownToolbar.ts`（论坛工具栏，ADR-0052）同一形态：只做「文本片段 → 文本片段」
 * 的变换，不碰 DOM、不知道选区在哪，于是每条命令的输出都能用单测钉死。DOM 侧的选区读取留在
 * `components/tutor/MarkdownEditor.vue` 的自定义工具栏按钮里。
 *
 * 为什么统一用 `<span style="display:block;…">` 而不是 `<div>`：
 * Vditor / marked 都按 CommonMark 处理 HTML 块——**块级标签内部的 markdown 不再解析**，
 * 于是「把这段加粗文字居中」会当场把 `**粗体**` 变成字面量，`![图](url)` 更是直接不渲染。
 * 行内 span 包住的内容仍走行内解析（marked 与 Vditor 的 `getValue` 往返都保真），
 * 加粗、链接、图片都不会因此丢格式；`display:block` 只是把它变成独占一行的盒子，
 * 让 `text-align` 对整段生效。
 *
 * 三件事都只影响**读者看到的排版**，不改内容语义：移动端解析器不认 HTML，会退回纯文本
 * （ADR-0046 的降级口径），所以这里的能力必须与「门户渲染、移动端降级」的既定范围一致。
 */

/** 文本对齐方式（与 CSS `text-align` 同名，省掉一层映射） */
export type ContentAlign = 'center' | 'left'

export interface ContentAlignItem {
  key: ContentAlign
  /** 工具栏提示与无障碍名（唯一来源，组件里不另抄） */
  label: string
}

/** 对齐档：左对齐是显式写法——作者把它当「把这段从居中改回来」用 */
export const CONTENT_ALIGNS: readonly ContentAlignItem[] = [
  { key: 'center', label: '文本居中' },
  { key: 'left', label: '左对齐' }
]

export interface ContentColorItem {
  key: string
  label: string
  /** 写进 span 的色值。取 Tailwind 600 档：浅底与深底都还有足够对比度 */
  value: string
}

/**
 * 文字颜色档。
 *
 * 刻意**不接设计 token**：这段样式最终进的是读者端 HTML（门户 Nuxt / 移动端 WebView），
 * 那边没有本仓的 CSS 变量；写 `var(--color-danger)` 只会渲染成浏览器默认色。
 * 五个色值互不相近，作者在工具栏里点错也看得出来。
 * （`scripts/check-bare-hex.sh` 只扫 .vue 的 template/style，本文件的常量不在其判定面；
 * 这里也不是 UI 配色，是**读者端内容**的取色盘。）
 */
export const CONTENT_COLORS: readonly ContentColorItem[] = [
  { key: 'red', label: '红色', value: '#DC2626' },
  { key: 'orange', label: '橙色', value: '#EA580C' },
  { key: 'green', label: '绿色', value: '#16A34A' },
  { key: 'blue', label: '蓝色', value: '#2563EB' },
  { key: 'gray', label: '灰色', value: '#6B7280' }
]

export interface ContentFontSizeItem {
  key: string
  label: string
  /** 写进 span 的完整声明。见 `fontSizeTag` 对「为什么带 var 兜底」的说明 */
  declaration: string
}

/**
 * 字号档：以正文 16px 为基准，两小两大，够拉开层次又不至于让文章变海报。
 *
 * 每档都写成 `var(--text-*, 兜底 rem)`：本仓自己渲染时（学员端 Web）跟着 UI 字号 token 走，
 * 门户那边的 CSS 里没有这些变量，就落到 rem 兜底上——两边都是设计想要的字号，且
 * **不直写 px**（字号阶梯守卫 #1894 / ADR-0080 明令 frontend/src 不许出现裸 px 字号）。
 *
 * 声明末尾的分号是**实打实的 CSS 语法**，不是为凑守卫：内联 style 里多条声明本来就要分号分隔，
 * 顺带也让守卫能按 `;` 正确切出值本身（它扫的是 `frontend/src/**\/*.ts`，看不到这只是个字符串）。
 */
export const CONTENT_FONT_SIZES: readonly ContentFontSizeItem[] = [
  { key: 'sm', label: '小号字', declaration: 'font-size:var(--text-sm, 0.875rem);' },
  { key: 'base', label: '正常字', declaration: 'font-size:var(--text-base, 1rem);' },
  { key: 'lg', label: '大号字', declaration: 'font-size:var(--text-xl, 1.25rem);' },
  { key: 'xl', label: '特大字', declaration: 'font-size:var(--text-2xl, 1.5rem);' }
]

/** 选区为空时的占位文字：插入即选中，作者直接打字即可覆盖 */
export const PLACEHOLDER_TEXT = '在此输入文字'

const HTML_CLOSE = '</span>'

/**
 * 对齐外壳：`display:block` 让 text-align 对整段生效，同时保住内部 markdown 的行内解析。
 */
function alignTag(align: ContentAlign): string {
  return `<span style="display:block;text-align:${align}">`
}

/**
 * 字号外壳。`declaration` 来自 `CONTENT_FONT_SIZES`，已是完整的 `font-size:…` 声明，
 * 这里只负责拼标签——不在本处出现字号字面量，免得踩字号阶梯守卫（#1894 / ADR-0080）。
 */
function fontSizeTag(declaration: string): string {
  return `<span style="${declaration}">`
}

/**
 * 已包过同一种外壳则去壳，否则包上。
 * 只认「选区自身就是完整外壳」这一种形态——Vditor 没有读取选区的 API，
 * 选区内侧/外侧相邻的标签都看不见，所以不做更聪明的相邻判断。
 */
function wrapOrUnwrap(selected: string, open: string, placeholder: string): string {
  const body = selected || placeholder
  if (body.startsWith(open) && body.endsWith(HTML_CLOSE)) {
    return body.slice(open.length, body.length - HTML_CLOSE.length)
  }
  return open + body + HTML_CLOSE
}

/** 文本对齐：居中 / 左对齐 */
export function alignSelection(selected: string, align: ContentAlign): string {
  return wrapOrUnwrap(selected, alignTag(align), PLACEHOLDER_TEXT)
}

/** 文字颜色 */
export function colorSelection(selected: string, color: string): string {
  return wrapOrUnwrap(selected, `<span style="color:${color}">`, PLACEHOLDER_TEXT)
}

/** 字号 */
export function fontSizeSelection(selected: string, declaration: string): string {
  return wrapOrUnwrap(selected, fontSizeTag(declaration), PLACEHOLDER_TEXT)
}

/**
 * 图片居中：与文本居中是同一条命令。
 *
 * 图片在 markdown 里是行内元素（`![alt](src)`），包上 `display:block` 后 `text-align:center`
 * 就作用在它的位置上——**不需要**把 markdown 图片改写成 HTML `<img>` 标签，
 * 于是图片的中文 alt、后续换图、以及门户端的 `loading="lazy"` 补全（门户 renderMarkdown
 * 会给每个 img 补属性）都照旧有效。
 */
export function centerImageSelection(selected: string): string {
  return alignSelection(selected, 'center')
}
