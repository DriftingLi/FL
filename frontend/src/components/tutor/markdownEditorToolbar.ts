/**
 * MarkdownEditor 的**内容排版工具栏**（颜色 / 字号 / 对齐 / 图片居中）。
 *
 * 为什么单独成文件而不是写进组件：Vditor 的自定义工具栏项是纯数据（`IMenuItem`），
 * 与 Vue 组件生命周期无关；把它们抽出来，命令表与图标一处定义、组件只负责把实例递进来。
 *
 * 三个关键约束（都来自 Vditor 源码实测，不是猜的）：
 *
 * 1. **`click(event, vditor)` 里的第二个参数是内部的 `IVditor`，不是 `Vditor` 实例** ——
 *    它没有 `insertMD`/`getValue`/`getSelection` 这些方法。所以要由组件用闭包把外层实例
 *    传进来（`getVditor()`），命令只认外层实例。
 * 2. **只有带 `prefix` 的菜单项才会被自动绑定插入事件**（`MenuItem` 构造器里
 *    `if (!menuItem.prefix) return;`）。我们的命令都要先读选区再做判断，所以一律用
 *    `click`，**不填 prefix** —— 填了会多绑一个插入监听，命令被执行两次。
 * 3. **图标必须是 16 视角的 SVG 字符串**：Vditor 用 `innerHTML` 直接塞进工具栏按钮，
 *    24 网格图标会撑到 24px 破坏工具栏对齐（图标尺寸由 `.vditor-toolbar__item svg` 定）。
 *
 * 命令体是纯函数（`utils/markdownHtmlCommands`），本文件只做「读选区 → 调纯函数 → 写回」。
 */
import type Vditor from 'vditor'
import {
  alignSelection,
  centerImageSelection,
  colorSelection,
  fontSizeSelection,
  hasMarkdownImage,
  CONTENT_ALIGNS,
  CONTENT_COLORS,
  CONTENT_FONT_SIZES,
  PLACEHOLDER_TEXT,
  type ContentAlign
} from '@/utils/markdownHtmlCommands'
import {
  cellFromRange,
  deleteColumn,
  deleteRow,
  insertColumn,
  insertRowBelow,
  type TableCell,
  type TableCommandResult
} from '@/utils/markdownTableCommands'

/** Vditor 的工具栏项形状（`IMenuItem` 的本地镜像：那个类型没从包入口导出） */
export interface ContentToolbarItem {
  name: string
  tip: string
  icon: string
  /**
   * 悬浮提示的方向。**必须设置**：`MenuItem` 会拼出 `vditor-tooltipped__${tipPosition}` 这个
   * class，不设就是 `vditor-tooltipped__undefined` —— 没有对应 CSS，气泡退化成无定位的
   * `absolute`，直接压在相邻按钮上（2026-11 实测：提示盖住了右边的按钮，本意要看的东西被它挡掉了）。
   * Vditor 内置项一律写 `"ne"`，这里跟随。
   */
  tipPosition?: string
  /** 子菜单 */
  toolbar?: ContentToolbarItem[]
  click?: (event: Event, vditor: unknown) => void
}

/**
 * 图标集（16 视角 / 描边 1.5 / `currentColor`）。
 *
 * 与 `components/markdown/MarkdownToolbarIcon.vue` 同一套画法：字形类（A、字）直接用
 * `<text>`，结构类用描边路径。颜色一律 `currentColor` —— Vditor 工具栏在深浅主题下
 * 靠 `fill: currentColor` 自动跟随，图标里写死色值会在深色下变成黑块。
 */
/**
 * 图标画笔。
 *
 * ⚠️ **必须用内联 `style` 而不是 presentation attribute**（2026-11 实测踩到）：
 * Vditor 有一条 `.vditor-toolbar__item svg { fill: currentColor; stroke-width: 0 }`，
 * **CSS 属性会盖掉 SVG 的 `fill="none"` / `stroke-width="1.5"`** —— 结果是所有描边图标
 * 的描边归零、又被填成实心色块，九个按钮长得一模一样（computedStrokeWidth 实测 0px）。
 * 内联样式优先于类选择器，所以这里把 paint 属性写进 `style`。
 */
const PAINT_STROKE = 'fill:none;stroke:currentColor;stroke-width:1.5;stroke-linecap:round;stroke-linejoin:round'
const PAINT_FILL = 'fill:currentColor;stroke:none'

function icon(inner: string, stroke = true): string {
  return `<svg viewBox="0 0 16 16" width="16" height="16" style="${stroke ? PAINT_STROKE : PAINT_FILL}">${inner}</svg>`
}

/**
 * 带文字的子菜单图标。
 *
 * 子菜单项走 Vditor 的 `Custom` 构造器（`dist/index.js:13173`），它执行
 * `element.children[0].innerHTML = menuItem.icon` —— **会把 `MenuItem` 按 `tip` 写进去的文字
 * 整个覆盖掉**。所以子菜单想显示「红色 / 大号字」这类文字，只能把文字拼进 `icon` 里。
 * （这也是为什么颜色子菜单原本是五个没有文字的圆点、字号是四个一样的图标。）
 */
function iconWithLabel(svg: string, label: string): string {
  return `<svg viewBox="0 0 16 16" width="14" height="14" style="${PAINT_FILL};float:left;margin:3px 6px 0 0">${svg}</svg>${label}`
}

/** 对齐类图标：三条横线，长短表示对齐侧 */
const ALIGN_ICONS: Record<ContentAlign, string> = {
  center: icon('<path d="M2.5 4h11M4 8h8M2.5 12h11"/>'),
  left: icon('<path d="M2.5 4h11M2.5 8h8M2.5 12h11"/>')
}

/**
 * 颜色图标：一个实心圆点。
 *
 * 同上，`fill` 必须走内联 `style` —— 否则 `.vditor-toolbar__item svg { fill: currentColor }`
 * 会把每个色档都染成同一个前景色（2026-11 实测：五个圆点全是一片深灰，看不出是红/橙/绿）。
 */
function colorIcon(value: string): string {
  return `<svg viewBox="0 0 16 16" width="16" height="16" style="stroke:none"><circle cx="8" cy="8" r="5.5" style="fill:${value};stroke:none"/></svg>`
}

/** 字号图标：大写 A 加一个升降号，四档共用同一形状（子菜单文案区分大小） */
const FONT_SIZE_GLYPH = '<path d="M2.5 12 6 4l3.5 8M3.8 9.5h4.4"/><path d="M11 5v6M9.5 9.5 11 11l1.5-1.5"/>'
const FONT_SIZE_ICON = icon(FONT_SIZE_GLYPH)

/** 图片居中图标：方框 + 居中的竖线标记 */
const IMAGE_CENTER_ICON = icon('<rect x="2.5" y="3" width="11" height="10" rx="1.2"/><path d="M5 10.5 7 8l1.7 1.7L10 9l1.5 1.5"/><circle cx="10.3" cy="6" r="0.9" fill="currentColor" stroke="none"/>')

/** 表格增删图标：表格框 + 指明「往哪边加 / 删哪条」的箭头或减号 */
const TABLE_INSERT_ROW_ICON = icon('<rect x="2.5" y="2.5" width="11" height="4" rx="0.8"/><path d="M2.5 9.5h11M6 12.5h4"/><path d="M8 9.2v3.6"/><path d="M6.6 11.2 8 12.6l1.4-1.4"/>')
const TABLE_INSERT_COLUMN_ICON = icon('<rect x="2.5" y="2.5" width="4" height="11" rx="0.8"/><path d="M9.5 2.5v11M12.5 6v4"/><path d="M9.2 8h3.6"/><path d="M11.2 6.6 12.6 8l-1.4 1.4"/>')
const TABLE_DELETE_ROW_ICON = icon('<rect x="2.5" y="2.5" width="11" height="4" rx="0.8"/><path d="M2.5 9.5h11"/><path d="M6 12h4" stroke-dasharray="1.6 1.4"/>')
const TABLE_DELETE_COLUMN_ICON = icon('<rect x="2.5" y="2.5" width="4" height="11" rx="0.8"/><path d="M9.5 2.5v11"/><path d="M12 7.5v2" stroke-dasharray="1.6 1.4"/>')

/** 选区可能落在一个块级 HTML 节点里 —— 那时真实源码在节点的 `code` 上，不在选区文本里 */
interface SelectionSource {
  /** 交给纯函数命令的 Markdown 文本 */
  text: string
  /** 选区整体落在某个 html-block 里时，指向那个块（「再点一次去壳」要做 DOM 级替换） */
  block: HTMLElement | null
}

/** 从选区节点向上找块级 HTML 容器（Vditor 给它的标记是 `data-type="html-block"`） */
function closestHtmlBlock(node: Node | null): HTMLElement | null {
  let current: Node | null = node
  while (current && current !== document.body) {
    if (current.nodeType === 1) {
      const element = current as HTMLElement
      if (element.getAttribute('data-type') === 'html-block') return element
    }
    current = current.parentNode
  }
  return null
}

/**
 * 取当前选区的「源码」。
 *
 * 三种情况，都是在真实环境里踩出来的（2026-11，ADR-0046 决定 9）：
 *
 * 1. **选区落在块级 HTML 里**：`getSelection()` 只给渲染后的文字（`第二段`），看不到外面那层
 *    `<div align="center">`，于是「再点一次去壳」永远判断不出来，越点越套。真实源码在块节点的
 *    `code[data-type="html-block"]` 上，这里直接取它。
 * 2. **选区框住了图片**：IR 的图片节点里全是 `<img>` 与源码标记，**没有可选中的文字** ⇒
 *    `getSelection()` 返回空串；而 `insertMD` 尊重 DOM 选区，会把整张图片替换成占位文字，
 *    作者的内容当场丢失。这里按 `![alt](src)` 拼回真实语法。
 * 3. 光标折叠（没选中任何内容）不是选区：一律当空处理，让「必须有内容」的命令自己拒绝。
 */
function selectionSource(vditor: Vditor): SelectionSource {
  const readVditorSelection = (): string => {
    try {
      return vditor.getSelection() || ''
    } catch {
      // 模板尚未渲染完时 getSelection 会抛；按空选区处理
      return ''
    }
  }
  if (typeof window === 'undefined') return { text: readVditorSelection(), block: null }
  const selection = window.getSelection()
  if (!selection || selection.rangeCount === 0 || selection.isCollapsed) {
    // 浏览器侧选不到东西时（含折叠光标）仍问一下 Vditor：它自带选区判定，
    // 某些环境（测试替身、非浏览器宿主）下只有它有答案。
    return { text: readVditorSelection(), block: null }
  }
  const range = selection.getRangeAt(0)
  const block = closestHtmlBlock(range.commonAncestorContainer)
  if (block) {
    const code = block.querySelector('code[data-type="html-block"]')
    if (code) return { text: code.textContent || '', block }
  }

  const text = readVditorSelection()
  if (text) return { text, block: null }

  const images = Array.from(range.cloneContents().querySelectorAll('img'))
  if (images.length === 0) return { text: '', block: null }
  return {
    text: images
      .map((img) => {
        const alt = img.getAttribute('alt') || ''
        const src = img.getAttribute('src') || ''
        const title = img.getAttribute('title')
        return title ? `![${alt}](${src} "${title}")` : `![${alt}](${src})`
      })
      .join('\n\n'),
    block: null
  }
}

/**
 * 把块级 HTML 节点换成一个普通段落（「再点一次去壳」的落地动作）。
 *
 * 为什么要走 DOM 而不是 `insertMD`：实测 `selectNode(block)` + `insertMD` 会**追加重叠**，
 * 得到两段重复文字，不是替换。DOM 级替换 + undo 栈 + input 回调是唯一干净的路子，
 * 与表格命令同一套三件套（见 `createTableToolbarItems` 的说明）。
 */
function replaceBlockWithParagraph(block: HTMLElement, text: string): void {
  const paragraph = document.createElement('p')
  paragraph.setAttribute('data-block', '0')
  paragraph.textContent = text
  block.replaceWith(paragraph)
}

/** DOM 级改动后必须显式同步：进 undo 栈 + 通知父组件（否则 v-model 还是旧值） */
function syncAfterDomEdit(vditor: Vditor): void {
  vditor.vditor.undo?.addToUndoStack?.(vditor.vditor)
  vditor.vditor.options.input?.(vditor.getValue())
}

/**
 * 清掉块级 HTML 前面紧贴着的空段落。
 *
 * `insertMD` 插入块级 HTML 时会多留一个 `<p><br></p>`（实测：`第一段\n\n第二段` 变成
 * `第一段\n\n\n\n<div …>`），每次点按钮就多两行空行。只在「空段落 + 紧跟 html-block」这个
 * 组合下清，作者自己敲出来的空行不会被误删。
 */
function dropBlankParagraphBeforeBlock(block: Element): void {
  const previous = block.previousElementSibling
  if (!previous || previous.tagName !== 'P') return
  if ((previous.textContent ?? '').trim() !== '') return
  const html = previous.innerHTML.trim().toLowerCase()
  if (html === '' || html === '<br>' || html === '<br/>') previous.remove()
}

/**
 * 写回选区。
 *
 * `insertMD` 在 IR 模式下走 `lute.Md2VditorIRDOM` + `insertHTML`：选中的内容会被替换，
 * 未选中的部分原样保留，`getValue()` 往返保真。SV 模式下它走 `processPaste`，同样正确。
 *
 * `options.requireContent` 给「必须有选区才有意义」的命令用（如图片居中）：拿不到内容时
 * 只弹提示、不动文档。**不要**改成「没内容就填占位文字」—— 那正是图片被吃掉的原因。
 */
function wrapSelection(
  vditor: Vditor | null,
  build: (selected: string) => string,
  options: { requireContent?: string; requireImage?: string } = {}
): void {
  if (!vditor) return
  const { text: selected, block } = selectionSource(vditor)
  if (!selected && options.requireContent) {
    vditor.tip(options.requireContent, 2500)
    return
  }
  // 「图片居中」还要确认选中的确实是图片：只框住一段文字时，把它包进居中容器
  // 不是作者的意图（那是「文本居中」按钮的活），提示比默默包上更有用。
  if (selected && options.requireImage && !hasMarkdownImage(selected)) {
    vditor.tip(options.requireImage, 2500)
    return
  }
  const next = build(selected)
  // 包壳的结果一定包含原选区源码；不再包含它，说明纯函数选的是「去壳」
  if (block && next.indexOf(selected) === -1) {
    replaceBlockWithParagraph(block, next)
    syncAfterDomEdit(vditor)
    vditor.focus()
    return
  }
  vditor.insertMD(next)
  // insertMD 刚插进去的块级 HTML 前面会多一个空段落，顺手清掉（值已变，需再同步一次）
  const inserted = vditor.vditor?.ir?.element?.querySelector('[data-type="html-block"]')
  if (inserted && typeof window !== 'undefined') {
    const before = vditor.getValue()
    dropBlankParagraphBeforeBlock(inserted)
    if (vditor.getValue() !== before) syncAfterDomEdit(vditor)
  }
  vditor.focus()
}

/**
 * 生成内容排版工具栏项。
 *
 * @param getVditor 取当前 Vditor 实例（组件切换模式 / 主题时会重建实例，故传入 getter 而非实例）
 */
export function createContentToolbarItems(getVditor: () => Vditor | null): ContentToolbarItem[] {
  const alignItems: ContentToolbarItem[] = CONTENT_ALIGNS.map((item) => ({
    name: `align-${item.key}`,
    tip: item.label,
    icon: ALIGN_ICONS[item.key],
    tipPosition: 'ne',
    click: () => wrapSelection(getVditor(), (selected) => alignSelection(selected, item.key))
  }))

  const colorItems: ContentToolbarItem[] = CONTENT_COLORS.map((item) => ({
    name: `color-${item.key}`,
    tip: item.label,
    icon: `<svg viewBox="0 0 16 16" width="14" height="14" style="stroke:none;float:left;margin:3px 6px 0 0"><circle cx="8" cy="8" r="5.5" style="fill:${item.value};stroke:none"/></svg>${item.label}`,
    tipPosition: 'ne',
    click: () => wrapSelection(getVditor(), (selected) => colorSelection(selected, item.value))
  }))

  const fontSizeItems: ContentToolbarItem[] = CONTENT_FONT_SIZES.map((item) => ({
    name: `font-size-${item.key}`,
    tip: item.label,
    // 子菜单的文字只能拼进 icon：`Custom` 会用 icon 覆盖掉 tip（见 iconWithLabel 的说明）
    icon: iconWithLabel(FONT_SIZE_GLYPH, item.label),
    tipPosition: 'ne',
    click: () => wrapSelection(getVditor(), (selected) => fontSizeSelection(selected, item.declaration))
  }))

  return [
    // 文本居中与左对齐是两个平铺按钮：作者最高频的动作，塞进子菜单要多点一次
    ...alignItems,
    {
      name: 'image-center',
      tip: '图片居中',
      icon: IMAGE_CENTER_ICON,
      tipPosition: 'ne',
      // 两条前置条件都必要：空选区下占位文字会覆盖内容；选中文字时包壳又不是作者意图
      click: () =>
        wrapSelection(getVditor(), centerImageSelection, {
          requireContent: '请先选中整张图片（含 ![]() 那段），再点这个按钮',
          requireImage: '选中的内容里没有图片 —— 要让文字居中请用左边的「文本居中」'
        })
    },
    {
      name: 'color',
      tip: '文字颜色',
      icon: colorIcon('currentColor'),
      tipPosition: 'ne',
      toolbar: colorItems
    },
    {
      name: 'font-size',
      tip: '字号',
      icon: FONT_SIZE_ICON,
      tipPosition: 'ne',
      toolbar: fontSizeItems
    }
  ]
}

/**
 * 生成表格结构工具栏项（插入行 / 插入列 / 删除行 / 删除列）。
 *
 * 为什么需要：Vditor 自带的这四条只挂在 **wysiwyg 模式**的 popover 上，本项目用 IR，
 * IR 没有 popover ⇒ 原生按钮永不出现（见 `utils/markdownTableCommands` 的说明）。
 *
 * 只在 IR 模式下挂出来（源码模式没有表格 DOM，命令无处落脚）。
 *
 * 命令执行照实测顺序三件事，缺一件就出错（探针 probe5）：
 *   1. 改 DOM（`utils/markdownTableCommands`）；
 *   2. `undo.addToUndoStack` —— 否则这次改动不可撤销；
 *   3. `options.input(getValue())` —— 否则 `getValue()` 是对的、但父组件 v-model 还是旧值，
 *      保存时写回旧内容。**不要**改用手动派发 `new Event('input')`（实测不生效）。
 */
export function createTableToolbarItems(getVditor: () => Vditor | null): ContentToolbarItem[] {
  const run = (command: (cell: TableCell) => TableCommandResult) => {
    const vditor = getVditor()
    if (!vditor) return
    const cell = cellFromRange(currentRange(vditor))
    if (!cell) {
      vditor.tip(TABLE_FAILURE_TIPS['no-cell'], 2000)
      return
    }
    const result = command(cell)
    if (!result.ok) {
      vditor.tip(TABLE_FAILURE_TIPS[result.failure], 2500)
      return
    }
    // 命令已改完 DOM：进 undo 栈 + 通知父组件（顺序见上方注释）
    vditor.vditor.undo?.addToUndoStack?.(vditor.vditor)
    vditor.vditor.options.input?.(vditor.getValue())
    placeCaret(result.caret)
    vditor.focus()
  }

  return [
    {
      name: 'table-insert-row',
      tip: '在下方插入一行',
      icon: TABLE_INSERT_ROW_ICON,
      tipPosition: 'ne',
      click: () => run(insertRowBelow)
    },
    {
      name: 'table-insert-column',
      tip: '在右侧插入一列',
      icon: TABLE_INSERT_COLUMN_ICON,
      tipPosition: 'ne',
      click: () => run((cell) => insertColumn(cell))
    },
    {
      name: 'table-delete-row',
      tip: '删除当前行',
      icon: TABLE_DELETE_ROW_ICON,
      tipPosition: 'ne',
      click: () => run(deleteRow)
    },
    {
      name: 'table-delete-column',
      tip: '删除当前列',
      icon: TABLE_DELETE_COLUMN_ICON,
      tipPosition: 'ne',
      click: () => run(deleteColumn)
    }
  ]
}

/** 命令失败时的人话提示：静默不动比报错更让人困惑 */
const TABLE_FAILURE_TIPS: Record<string, string> = {
  'no-cell': '请先把光标放进表格的某个格子里，再点这个按钮',
  'header-row': '表头行不能删除（要减列请用「删除当前列」）',
  'last-column': '这是最后一列了 —— 要整张表格删掉，请选中它按 Delete'
}

/**
 * 取当前选区。
 *
 * 优先用浏览器实时选区（真实鼠标点工具栏不会夺焦，实测选区仍在）；拿不到再退回
 * Vditor 在 blur 时保存的 `vditor.ir.range`（用户点过别处再回来点按钮的情形）。
 */
function currentRange(vditor: Vditor): Range | null {
  const live = typeof window !== 'undefined' ? window.getSelection() : null
  if (live && live.rangeCount > 0) return live.getRangeAt(0)
  return vditor.vditor.ir?.range ?? null
}

/** 把光标放进命令指定的格子（没有就什么都不做，别抢走用户原本的位置） */
function placeCaret(cell: TableCell | undefined): void {
  if (!cell || typeof window === 'undefined') return
  const selection = window.getSelection()
  if (!selection) return
  const range = document.createRange()
  range.selectNodeContents(cell)
  range.collapse(true)
  selection.removeAllRanges()
  selection.addRange(range)
}

/** 选区为空时插入的占位文字（使用说明与组件共用一份） */
export { PLACEHOLDER_TEXT }
