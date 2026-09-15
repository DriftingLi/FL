/**
 * 论坛 Markdown 工具栏的**纯编辑核心**（ADR-0052）。
 *
 * 只做「一段文本 + 选区 → 新文本 + 新选区」的纯函数变换：不碰 DOM，也就不知道
 * textarea 在哪、光标是谁——于是每条命令的边界行为都能用单测钉死。
 * DOM 层的插入（选区读写、撤销栈）留在 `components/markdown/MarkdownToolbar.vue`。
 *
 * 语义约定：
 * - 所有命令都是**切换**：命中已应用的标记再点一次即撤销，按钮不会越点越乱；
 * - 多行命令（三级标题 / 引用 / 三种列表）按「涉及行」整行处理，不做局部字符串拼接；
 * - 命令集合与论坛**已声明子集**一一对应（ADR-0044 子集补记）：表格不在子集内，
 *   所以这里没有表格命令；任务列表在 #1014 之后的子集内，故有 `task`。
 */

/** 工具栏命令键。新增命令必须同时更新 `MARKDOWN_TOOLBAR_ITEMS`（提示文案单点） */
export type MarkdownCommandKey =
  | 'heading3'
  | 'bold'
  | 'italic'
  | 'quote'
  | 'code'
  | 'link'
  | 'ul'
  | 'ol'
  | 'task'

export interface MarkdownToolbarItem {
  key: MarkdownCommandKey
  /** 中文标签：**同时**是悬停提示与 aria-label 的唯一来源，组件里不许再抄一份 */
  label: string
  /** `MarkdownToolbarIcon` 的图标名 */
  icon: string
}

/**
 * 工具栏按钮表（图一 markdown 全集）。顺序即渲染顺序，`|` 分隔由组件按 key 决定。
 *
 * 标题只给**三级标题**一个按钮（不做 H1–H6 浮层）：正文层级由页面标题占用，
 * 论坛回复里再多一级选择只增加点击成本，要高一级低一级手打 `##`/`####` 更快。
 */
export const MARKDOWN_TOOLBAR_ITEMS: readonly MarkdownToolbarItem[] = [
  { key: 'heading3', label: '三级标题', icon: 'heading3' },
  { key: 'bold', label: '加粗', icon: 'bold' },
  { key: 'italic', label: '斜体', icon: 'italic' },
  { key: 'quote', label: '引用', icon: 'quote' },
  { key: 'code', label: '代码', icon: 'code' },
  { key: 'link', label: '链接', icon: 'link' },
  { key: 'ul', label: '无序列表', icon: 'ul' },
  { key: 'ol', label: '有序列表', icon: 'ol' },
  { key: 'task', label: '任务列表', icon: 'task' }
]

/** 分隔符：在这些按钮之前插一条竖线（对齐图一的分组） */
export const MARKDOWN_TOOLBAR_DIVIDERS: readonly MarkdownCommandKey[] = ['link', 'ul']

export interface MarkdownEditResult {
  text: string
  /** 新选区起点（无选区时为光标位置） */
  start: number
  /** 新选区终点 */
  end: number
}

const TICK = '`'
const FENCE = TICK + TICK + TICK

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max)
}

/**
 * 选区覆盖的整行范围：无选区时取光标所在行；有选区时若末端正好落在行首，
 * 该行不算「涉及行」（否则在行尾拖选到下一行行首会多改一行）。
 */
function lineSpan(text: string, start: number, end: number): { from: number; to: number } {
  const from = text.lastIndexOf('\n', start - 1) + 1
  let effectiveEnd = end
  if (end > start && text[effectiveEnd - 1] === '\n') effectiveEnd -= 1
  const newline = text.indexOf('\n', effectiveEnd)
  return { from, to: newline === -1 ? text.length : newline }
}

/**
 * 把整块行替换掉，并给出新选区。
 * - 有选区：选中整块（改完还能接着改）；
 * - 无选区：光标按**首行**的增删量平移，落在行内不跳位。
 */
function spliceBlock(
  text: string,
  from: number,
  to: number,
  block: string,
  start: number,
  end: number,
  firstLineDelta: number
): MarkdownEditResult {
  const next = text.slice(0, from) + block + text.slice(to)
  const blockEnd = from + block.length
  if (start !== end) return { text: next, start: from, end: blockEnd }
  const caret = clamp(start + firstLineDelta, from, blockEnd)
  return { text: next, start: caret, end: caret }
}

/** 行首前缀切换（三级标题 / 引用）：所有涉及行都有前缀 → 去掉；否则补上 */
function toggleLinePrefix(text: string, start: number, end: number, prefix: string): MarkdownEditResult {
  const { from, to } = lineSpan(text, start, end)
  const lines = text.slice(from, to).split('\n')
  const applied = lines.every(line => line.startsWith(prefix))
  let firstLineDelta = 0
  const next = lines.map((line, index) => {
    if (applied) {
      const stripped = line.startsWith(prefix) ? line.slice(prefix.length) : line
      if (index === 0) firstLineDelta = stripped.length - line.length
      return stripped
    }
    if (line.startsWith(prefix)) return line
    if (index === 0) firstLineDelta = prefix.length
    return prefix + line
  })
  return spliceBlock(text, from, to, next.join('\n'), start, end, firstLineDelta)
}

/** 任意列表标记（普通项 / 任务项 / 有序项），用于换标记前先清干净，避免 `- 1. x` 这类叠加 */
const ANY_LIST_MARKER = /^(?:[-*+] \[[ xX]\] |[-*+] |\d+\. )/

type ListKind = 'ul' | 'ol' | 'task'

function hasListMarker(line: string, kind: ListKind): boolean {
  if (kind === 'task') return /^[-*+] \[[ xX]\] /.test(line)
  if (kind === 'ol') return /^\d+\. /.test(line)
  // 无序列表不认识任务项：任务项在 ul 语义下算「未应用」，点无序列表即把它转成普通列表项
  return /^[-*+] (?!\[[ xX]\] )/.test(line)
}

/** 列表切换：三种列表共用一个入口，换标记时先清掉原有标记（含任务项），再逐行编号 */
function toggleListPrefix(text: string, start: number, end: number, kind: ListKind): MarkdownEditResult {
  const { from, to } = lineSpan(text, start, end)
  const lines = text.slice(from, to).split('\n')
  const applied = lines.every(line => hasListMarker(line, kind))
  let firstLineDelta = 0
  const next = lines.map((line, index) => {
    const stripped = line.replace(ANY_LIST_MARKER, '')
    let out: string
    if (applied) out = stripped
    else if (kind === 'ol') out = `${index + 1}. ${stripped}`
    else if (kind === 'task') out = `- [ ] ${stripped}`
    else out = `- ${stripped}`
    if (index === 0) firstLineDelta = out.length - line.length
    return out
  })
  return spliceBlock(text, from, to, next.join('\n'), start, end, firstLineDelta)
}

/** 行内包裹类（加粗 / 斜体）：已包住则去壳，否则包上 */
function wrapInline(text: string, start: number, end: number, marker: string): MarkdownEditResult {
  const width = marker.length
  const selected = text.slice(start, end)
  // 选区自己就带着标记（用户把 **x** 整个选中了）→ 去内层壳
  if (selected.length >= width * 2 && selected.startsWith(marker) && selected.endsWith(marker)) {
    const inner = selected.slice(width, selected.length - width)
    return { text: text.slice(0, start) + inner + text.slice(end), start, end: start + inner.length }
  }
  // 选区外侧正好是标记 → 去外层壳（斜体要排除 ** 的情形，否则会把加粗拆成斜体）
  const outerWrapped =
    text.slice(start - width, start) === marker &&
    text.slice(end, end + width) === marker &&
    !(marker === '*' && (text[start - 2] === '*' || text[end + 1] === '*'))
  if (outerWrapped) {
    const next = text.slice(0, start - width) + selected + text.slice(end + width)
    return { text: next, start: start - width, end: start - width + selected.length }
  }
  const next = text.slice(0, start) + marker + selected + marker + text.slice(end)
  return { text: next, start: start + width, end: end + width }
}

/** 代码：单行 → 行内代码；跨行 → 围栏代码块（GitHub 语义）；两侧单反引号 → 去壳 */
function applyCode(text: string, start: number, end: number): MarkdownEditResult {
  if (start === end) {
    // 空选区插入一对反引号，光标落在中间
    return { text: text.slice(0, start) + TICK + TICK + text.slice(end), start: start + 1, end: start + 1 }
  }
  const selected = text.slice(start, end)
  if (text[start - 1] === TICK && text[end] === TICK) {
    const next = text.slice(0, start - 1) + selected + text.slice(end + 1)
    return { text: next, start: start - 1, end: end - 1 }
  }
  if (!selected.includes('\n')) {
    const next = text.slice(0, start) + TICK + selected + TICK + text.slice(end)
    return { text: next, start: start + 1, end: end + 1 }
  }
  // 跨行：围栏必须独占一行，所以按需在前后补换行（避免把围栏拼在正文行尾）
  const lead = start > 0 && text[start - 1] !== '\n' ? '\n' : ''
  const tail = end < text.length && text[end] !== '\n' ? '\n' : ''
  const block = lead + FENCE + '\n' + selected + '\n' + FENCE + tail
  const contentStart = start + lead.length + FENCE.length + 1
  return {
    text: text.slice(0, start) + block + text.slice(end),
    start: contentStart,
    end: contentStart + selected.length
  }
}

/** 链接：选中文字包成链接，并把 `url` 占位选中，作者直接打字即可覆写 */
function applyLink(text: string, start: number, end: number): MarkdownEditResult {
  const label = start === end ? '链接文字' : text.slice(start, end)
  const inserted = `[${label}](url)`
  const urlStart = start + 1 + label.length + 2
  return {
    text: text.slice(0, start) + inserted + text.slice(end),
    start: urlStart,
    end: urlStart + 3
  }
}

/**
 * 工具栏命令主入口。`start`/`end` 越界会被夹回文本范围（textarea 在外部值变化后
 * 选区可能一时对不上，宁可夹回也不要抛错把输入框卡死）。
 */
export function applyMarkdownCommand(
  text: string,
  start: number,
  end: number,
  command: MarkdownCommandKey
): MarkdownEditResult {
  const safeStart = clamp(start, 0, text.length)
  const safeEnd = clamp(Math.max(end, safeStart), safeStart, text.length)
  switch (command) {
    case 'bold':
      return wrapInline(text, safeStart, safeEnd, '**')
    case 'italic':
      return wrapInline(text, safeStart, safeEnd, '*')
    case 'code':
      return applyCode(text, safeStart, safeEnd)
    case 'link':
      return applyLink(text, safeStart, safeEnd)
    case 'heading3':
      return toggleLinePrefix(text, safeStart, safeEnd, '### ')
    case 'quote':
      return toggleLinePrefix(text, safeStart, safeEnd, '> ')
    case 'ul':
      return toggleListPrefix(text, safeStart, safeEnd, 'ul')
    case 'ol':
      return toggleListPrefix(text, safeStart, safeEnd, 'ol')
    case 'task':
      return toggleListPrefix(text, safeStart, safeEnd, 'task')
  }
}
