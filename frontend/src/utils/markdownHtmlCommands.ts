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

/**
 * 对齐外壳：**块级** div，用 `align` 属性而不是 `text-align`。
 *
 * 为什么不是行内 `<span style="display:block;text-align:center">`（2026-11 改，ADR-0046 决定 9）：
 *
 *   1. **Vditor 的 IR 模式只给块级 HTML 做渲染预览**。行内 HTML 一律显示成灰色源码标签
 *      （`.vditor-ir__marker`），作者点完按钮看不出任何变化，以为「没生效」——这是关于这批
 *      按钮被反馈最多的一条。
 *   2. 行内 span 包在 `<p>` 里时，`display:block` 只能撑满**段落自身的宽度**，段落本身仍
 *      跟着文档左对齐，视觉上根本不居中。
 *   3. 块级 div 里的行内 span（颜色、字号）**照样能渲染**（实测），所以颜色/字号不受影响。
 *
 * 用 `align` 属性而非 `style="text-align:…"`：门户与移动端 WebView 对两者的支持等价，
 * 但 `align` 更短、且与本仓 `markdown.css` 里 `thead th:not([align])` 那套口径同源。
 */
function alignTag(align: ContentAlign): string {
  return `<div align="${align}">`
}

/**
 * 块级图片容器：`<div align="center"><img …></div>`。
 *
 * 图片必须写成 HTML `<img>`：块级容器内部不再解析 markdown，`![alt](url)` 会原样显示成
 * 字面量。由 `markdownImagesToHtml` 负责把选区里的图片语法转过来。
 */
function blockImageTag(align: ContentAlign): string {
  return `<div align="${align}">`
}

/**
 * 把选区里的 markdown 图片语法转成 HTML `<img>`。
 *
 * 只转 `![alt](url)` 这一种最常见的形态（含可选的 `"title"`）。链接地址里可能有括号，
 * 所以按「首个 `)` 收尾」取——足够覆盖本站上传与外部图床的实际写法，也不至于贪婪到
 * 吃掉后面的内容。
 *
 * 非图片的内容原样返回，调用方据此判断「这段里到底有没有图」。
 */
export function markdownImagesToHtml(text: string): string {
  return text.replace(
    /!\[([^\]]*)\]\(([^)\s]+)(?:\s+"([^"]*)")?\)/g,
    (_match, alt: string, src: string, title?: string) => {
      const titleAttr = title ? ` title="${escapeAttr(title)}"` : ''
      return `<img src="${escapeAttr(src)}" alt="${escapeAttr(alt)}"${titleAttr}>`
    }
  )
}

/** 属性值转义：alt 里出现的引号会把 img 标签撑破，进而破坏整段 HTML */
function escapeAttr(value: string): string {
  return value.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

/** 选区里是否有 markdown 图片语法——决定「图片居中」要不要走 HTML 分支 */
export function hasMarkdownImage(text: string): boolean {
  return /!\[[^\]]*\]\([^)\s]+/.test(text)
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
 *
 * `closeTag` 必须与 `open` 配对：对齐/图片用的是 `div`，颜色/字号用的是 `span`，
 * 写死成 `</span>` 会让「再点一次去壳」对 div 完全失效。
 */
function wrapOrUnwrap(selected: string, open: string, closeTag: string, placeholder: string): string {
  const body = selected || placeholder
  if (body.startsWith(open) && body.endsWith(closeTag)) {
    return body.slice(open.length, body.length - closeTag.length)
  }
  return open + body + closeTag
}

/**
 * 文本对齐：居中 / 左对齐。
 *
 * 选区里含 markdown 图片语法时，先把它转成 `<img>` ——块级容器内部不解析 markdown，
 * 不转的话图片会显示成字面量 `![图](url)`。
 */
export function alignSelection(selected: string, align: ContentAlign): string {
  return wrapOrUnwrap(withImagesAsHtml(selected), alignTag(align), '</div>', PLACEHOLDER_TEXT)
}

/** 把选区里的图片语法转成 HTML（块级容器内部不再解析 markdown，见 alignTag 的说明） */
function withImagesAsHtml(text: string): string {
  return hasMarkdownImage(text) ? markdownImagesToHtml(text) : text
}

/** 文字颜色 */
export function colorSelection(selected: string, color: string): string {
  return wrapOrUnwrap(selected, `<span style="color:${color}">`, '</span>', PLACEHOLDER_TEXT)
}

/** 字号 */
export function fontSizeSelection(selected: string, declaration: string): string {
  return wrapOrUnwrap(selected, fontSizeTag(declaration), '</span>', PLACEHOLDER_TEXT)
}

/**
 * 图片居中：把整段图片语法包进块级容器。
 *
 * ⚠️ 这里必须产出**块级 `<div align="center">`**，且图片要是 HTML `<img>` 标签 ——
 * 两个原因都是在真实环境里踩出来的（2026-11，见 ADR-0046 决定 9）：
 *
 *   1. 行内 `<span style="display:block;text-align:center">![图](url)</span>` 在 Vditor 的 IR
 *      模式里**只显示灰色源码标签、不渲染预览**，作者点完看不出任何变化；
 *   2. 更糟的是它会让 `getSelection()` 返回空串 —— 工具按钮随后按「空选区」处理，
 *      插入占位文字 `在此输入文字` **把图片本身覆盖掉**，内容直接丢失。
 *
 * 块级 HTML 才会被 Vditor 当作 html-block 渲染预览（实测 `.vditor-ir__preview` 里
 * `<div align="center"><img …></div>` 正常显示），所以图片必须转成 `img` 标签。
 *
 * 代价与取舍：块级 HTML **内部不再解析 markdown**，所以图片段里的中文 alt 只能靠
 * `alt` 属性、不能带加粗等行内语法。图片段落本来也不该有这些，可接受。
 */
export function centerImageSelection(selected: string): string {
  return wrapOrUnwrap(withImagesAsHtml(selected), blockImageTag('center'), '</div>', PLACEHOLDER_TEXT)
}
