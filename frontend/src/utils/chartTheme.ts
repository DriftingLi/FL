/**
 * 图表主题（#1893）：ECharts 在 canvas 绘制，**读不到 CSS 变量**，所以颜色必须以字面量给出。
 *
 * 两条硬约束（判据锁在 `__tests__/chartTheme.spec.ts`）：
 * 1. **两套主题各一份**：深色页面上的浅色轴标签 / 分隔线不能用浅色主题那套（浅色下
 *    `#64748B` 压 `#1E293B` 只有 3.07:1）；同理浅色下的饼图描边不能是深色。
 * 2. **系列色两两可辨**：除了正常视觉的色差，还要过红绿色盲模拟（deuteranopia）。
 *    深色那一套受「必须比 #1E293B 亮 3:1」限制，亮度区间被压窄，8 色能做到的盲辨 ΔE
 *    上限约 7 —— 因此**颜色从不是唯一通道**：图例文字、tooltip、饼图分片描边都在。
 *
 * 用法：`const t = chartTheme()`（跟随当前 `<html data-theme>`）或 `chartTheme('dark')`。
 * 页面切换主题时要重绘（`watch(themeStore.resolved)`），因为 option 里的颜色是快照。
 */

export type ChartThemeName = 'light' | 'dark'

export interface ChartTheme {
  /** 分类系列色，按序取用 */
  series: readonly string[]
  /** 轴标签 / 图例文字 */
  text: string
  /** 标题 / 强调文字 */
  textStrong: string
  /** 轴线 */
  axisLine: string
  /** 分隔线（网格） */
  splitLine: string
  tooltip: {
    backgroundColor: string
    borderColor: string
    textStyle: { color: string }
  }
  /** 饼图 / 环图分片之间的描边（等于卡片底色，形成视觉间隙） */
  pieBorderColor: string
  /** 状态色（进度阈值等）：与 design-tokens 的 fill 档同值 */
  status: { ok: string; warn: string; bad: string }
}

/**
 * 浅色：8 色，两两 ΔE ≥ 23.1（正常）/ ≥ 18.2（红绿色盲）。首个是品牌石墨青。
 * 深色：同一批色相族的提亮版（主题切换时同一分片的色相不变），两两 ΔE ≥ 22.1 / ≥ 7.4。
 */
const SERIES_LIGHT = [
  '#0F766E', // teal（品牌）
  '#1E40AF', // blue
  '#B45309', // amber
  '#8B5CF6', // violet
  '#BE185D', // pink
  '#3F6212', // lime
  '#0E7490', // cyan
  '#334155' // slate
] as const

const SERIES_DARK = [
  '#5EEAD4', // teal
  '#93C5FD', // blue
  '#FBBF24', // amber
  '#A78BFA', // violet
  '#F472B6', // pink
  '#BEF264', // lime
  '#67E8F9', // cyan
  '#94A3B8' // slate
] as const

const LIGHT: ChartTheme = {
  series: SERIES_LIGHT,
  // 以下字面量与 design-tokens.css 同值（canvas 读不到变量），spec 会逐条对账
  text: '#475569', // --color-text-secondary
  textStrong: '#0F172A', // --color-text-primary
  axisLine: '#E2E8F0', // --color-border
  splitLine: '#F1F5F9', // --color-border-light
  tooltip: {
    backgroundColor: '#FFFFFF', // --color-bg-card
    borderColor: '#E2E8F0', // --color-border
    textStyle: { color: '#0F172A' } // --color-text-primary
  },
  pieBorderColor: '#FFFFFF', // --color-bg-card
  status: { ok: '#047857', warn: '#B45309', bad: '#B91C1C' } // --fill-ok / --fill-warn / --fill-bad
}

const DARK: ChartTheme = {
  series: SERIES_DARK,
  text: '#CBD5E1', // --color-text-secondary
  textStrong: '#F1F5F9', // --color-text-primary
  axisLine: '#334155', // --color-border
  splitLine: '#273449', // --color-border-light
  tooltip: {
    backgroundColor: '#1E293B', // --color-bg-card
    borderColor: '#334155', // --color-border
    textStyle: { color: '#F1F5F9' } // --color-text-primary
  },
  pieBorderColor: '#1E293B', // --color-bg-card
  status: { ok: '#34D399', warn: '#FBBF24', bad: '#F87171' } // --fill-ok / --fill-warn / --fill-bad
}

/** 当前生效主题：以 `<html data-theme>` 为准（theme store 与 index.html 内联脚本都写它） */
export function resolveChartThemeName(): ChartThemeName {
  if (typeof document === 'undefined') return 'light'
  return document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light'
}

export function chartTheme(name: ChartThemeName = resolveChartThemeName()): ChartTheme {
  return name === 'dark' ? DARK : LIGHT
}
