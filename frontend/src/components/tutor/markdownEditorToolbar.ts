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
  CONTENT_ALIGNS,
  CONTENT_COLORS,
  CONTENT_FONT_SIZES,
  PLACEHOLDER_TEXT,
  type ContentAlign
} from '@/utils/markdownHtmlCommands'

/** Vditor 的工具栏项形状（`IMenuItem` 的本地镜像：那个类型没从包入口导出） */
export interface ContentToolbarItem {
  name: string
  tip: string
  icon: string
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
const STROKE = 'fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"'
const FILL = 'fill="currentColor" stroke="none"'

function icon(inner: string, stroke = true): string {
  return `<svg viewBox="0 0 16 16" width="16" height="16" ${stroke ? STROKE : FILL}>${inner}</svg>`
}

/** 对齐类图标：三条横线，长短表示对齐侧 */
const ALIGN_ICONS: Record<ContentAlign, string> = {
  center: icon('<path d="M2.5 4h11M4 8h8M2.5 12h11"/>'),
  left: icon('<path d="M2.5 4h11M2.5 8h8M2.5 12h11"/>')
}

/** 颜色图标：一个实心圆点（子菜单里每个色档再用同形状换 fill） */
function colorIcon(value: string): string {
  return `<svg viewBox="0 0 16 16" width="16" height="16" fill="${value}"><circle cx="8" cy="8" r="5"/></svg>`
}

/** 字号图标：大写 A 加一个升降号，四档共用同一形状（子菜单文案区分大小） */
const FONT_SIZE_ICON = icon('<path d="M2.5 12 6 4l3.5 8M3.8 9.5h4.4"/><path d="M11 5v6M9.5 9.5 11 11l1.5-1.5"/>')

/** 图片居中图标：方框 + 居中的竖线标记 */
const IMAGE_CENTER_ICON = icon('<rect x="2.5" y="3" width="11" height="10" rx="1.2"/><path d="M5 10.5 7 8l1.7 1.7L10 9l1.5 1.5"/><circle cx="10.3" cy="6" r="0.9" fill="currentColor" stroke="none"/>')

/**
 * 写回选区。
 *
 * `insertMD` 在 IR 模式下走 `lute.Md2VditorIRDOM` + `insertHTML`：选中的内容会被替换，
 * 未选中的部分原样保留，`getValue()` 往返保真（span 是行内节点，不像块级 div 那样
 * 在两侧多出空行）。SV 模式下它走 `processPaste`，同样正确。
 */
function wrapSelection(vditor: Vditor | null, build: (selected: string) => string): void {
  if (!vditor) return
  let selected = ''
  try {
    selected = vditor.getSelection() || ''
  } catch {
    // 模板尚未渲染完时 getSelection 会抛；空选区按占位文字插入，不阻断操作
    selected = ''
  }
  vditor.insertMD(build(selected))
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
    click: () => wrapSelection(getVditor(), (selected) => alignSelection(selected, item.key))
  }))

  const colorItems: ContentToolbarItem[] = CONTENT_COLORS.map((item) => ({
    name: `color-${item.key}`,
    tip: item.label,
    icon: colorIcon(item.value),
    click: () => wrapSelection(getVditor(), (selected) => colorSelection(selected, item.value))
  }))

  const fontSizeItems: ContentToolbarItem[] = CONTENT_FONT_SIZES.map((item) => ({
    name: `font-size-${item.key}`,
    tip: item.label,
    icon: FONT_SIZE_ICON,
    click: () => wrapSelection(getVditor(), (selected) => fontSizeSelection(selected, item.declaration))
  }))

  return [
    // 文本居中与左对齐是两个平铺按钮：作者最高频的动作，塞进子菜单要多点一次
    ...alignItems,
    {
      name: 'image-center',
      tip: '图片居中',
      icon: IMAGE_CENTER_ICON,
      click: () => wrapSelection(getVditor(), centerImageSelection)
    },
    {
      name: 'color',
      tip: '文字颜色',
      icon: colorIcon('currentColor'),
      toolbar: colorItems
    },
    {
      name: 'font-size',
      tip: '字号',
      icon: FONT_SIZE_ICON,
      toolbar: fontSizeItems
    }
  ]
}

/** 选区为空时插入的占位文字（使用说明与组件共用一份） */
export { PLACEHOLDER_TEXT }
