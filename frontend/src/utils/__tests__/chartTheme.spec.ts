// 图表主题锁（#1893）：系列色两两可辨 + 与卡片底有足够反差 + 字面色与 design-tokens 对账。
//
// 为什么这些数字要写死在测试里：chartTheme.ts 里的颜色是 canvas 用的字面量，读不到 CSS 变量，
// 也没法被视觉回归覆盖。没有锁的话，下一次「调个好看点的蓝」就会悄悄把两条曲线调成同色。
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { chartTheme, resolveChartThemeName } from '../chartTheme'

// ---------- 颜色数学 ----------
const hexToRgb = (h: string) => h.replace('#', '').match(/../g)!.map((x) => parseInt(x, 16) / 255)
const srgbToLinear = (c: number) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
const linearToSrgb = (c: number) => (c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055)
const luminance = (h: string) => {
  const [r, g, b] = hexToRgb(h).map(srgbToLinear)
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}
const contrast = (a: string, b: string) => {
  const l1 = luminance(a)
  const l2 = luminance(b)
  const [hi, lo] = l1 > l2 ? [l1, l2] : [l2, l1]
  return (hi + 0.05) / (lo + 0.05)
}

/** Machado 2009，deuteranopia（红绿色盲）severity 1.0，作用在线性 RGB 上 */
const DEUTAN = [
  [0.367322, 0.860646, -0.227968],
  [0.280085, 0.672501, 0.047413],
  [-0.01182, 0.04294, 0.968881]
]
function simulateDeutan(h: string): [number, number, number] {
  const c = hexToRgb(h).map(srgbToLinear)
  return DEUTAN.map((row) => {
    const v = row[0] * c[0] + row[1] * c[1] + row[2] * c[2]
    return Math.round(linearToSrgb(Math.max(0, Math.min(1, v))) * 255)
  }) as [number, number, number]
}
function toLab(rgb255: number[]): [number, number, number] {
  const [r, g, b] = rgb255.map((v) => {
    const s = v / 255
    return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
  })
  const X = (r * 0.4124 + g * 0.3576 + b * 0.1805) / 0.95047
  const Y = r * 0.2126 + g * 0.7152 + b * 0.0722
  const Z = (r * 0.0193 + g * 0.1192 + b * 0.9505) / 1.08883
  const f = (t: number) => (t > 0.008856 ? Math.cbrt(t) : 7.787 * t + 16 / 116)
  const [fx, fy, fz] = [f(X), f(Y), f(Z)]
  return [116 * fy - 16, 500 * (fx - fy), 200 * (fy - fz)]
}
const lab = (h: string) => toLab(hexToRgb(h).map((v) => Math.round(v * 255)))
const deltaE = (a: [number, number, number], b: [number, number, number]) =>
  Math.hypot(a[0] - b[0], a[1] - b[1], a[2] - b[2])

/** 两两最小色差（正常视觉 / 红绿色盲模拟） */
function minPairwise(colors: readonly string[], simulated: boolean) {
  let min = Infinity
  let pair = ''
  for (let i = 0; i < colors.length; i++) {
    for (let j = i + 1; j < colors.length; j++) {
      const d = simulated
        ? deltaE(toLab(simulateDeutan(colors[i])), toLab(simulateDeutan(colors[j])))
        : deltaE(lab(colors[i]), lab(colors[j]))
      if (d < min) {
        min = d
        pair = `${colors[i]} ~ ${colors[j]}`
      }
    }
  }
  return { min, pair }
}

// ---------- 与 design-tokens 对账 ----------
const css = readFileSync(resolve(__dirname, '../../assets/styles/design-tokens.css'), 'utf8')
function blockOf(source: string, selector: RegExp): string {
  const m = selector.exec(source)
  if (!m) throw new Error('找不到选择器: ' + selector)
  const open = source.indexOf('{', m.index)
  let depth = 0
  for (let i = open; i < source.length; i++) {
    if (source[i] === '{') depth++
    else if (source[i] === '}') {
      depth--
      if (depth === 0) return source.slice(open + 1, i)
    }
  }
  throw new Error('花括号不配对: ' + selector)
}
function decls(block: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const m of block.matchAll(/--([\w-]+)\s*:\s*([^;]+);/g)) out[m[1]] = m[2].trim()
  return out
}
const lightTokens = decls(blockOf(css, /\n:root\s*\{/))
const darkTokens = { ...lightTokens, ...decls(blockOf(css, /\[data-theme="dark"\]\s*\{/)) }

const CASES = [
  { name: 'light' as const, bg: '#FFFFFF', tokens: lightTokens, minDeutan: 12 },
  // 深色底要求所有系列色比 #1E293B 亮 3:1 ⇒ 亮度区间被压窄，8 色能到的盲辨 ΔE 上限约 7
  { name: 'dark' as const, bg: '#1E293B', tokens: darkTokens, minDeutan: 7 }
]

describe.each(CASES)('图表主题 $name', ({ name, bg, tokens, minDeutan }) => {
  const theme = chartTheme(name)

  it('8 个系列色，每一色与图表底色 ≥ 3:1（非文本图形对比度）', () => {
    expect(theme.series).toHaveLength(8)
    for (const c of theme.series) {
      expect(contrast(c, bg), `${c} 压在 ${bg}`).toBeGreaterThanOrEqual(3)
    }
  })

  it('系列色两两可辨：正常视觉 ΔE ≥ 20，红绿色盲模拟 ΔE ≥ 阈值', () => {
    const normal = minPairwise(theme.series, false)
    expect(normal.min, `最近的一对：${normal.pair}`).toBeGreaterThanOrEqual(20)
    const deutan = minPairwise(theme.series, true)
    expect(deutan.min, `红绿色盲下最近的一对：${deutan.pair}`).toBeGreaterThanOrEqual(minDeutan)
  })

  it('轴/图例/提示框的字面色与 design-tokens 同值（canvas 读不到变量，只能对账）', () => {
    expect(theme.text).toBe(tokens['color-text-secondary'])
    expect(theme.textStrong).toBe(tokens['color-text-primary'])
    expect(theme.axisLine).toBe(tokens['color-border'])
    expect(theme.splitLine).toBe(tokens['color-border-light'])
    expect(theme.tooltip.backgroundColor).toBe(tokens['color-bg-card'])
    expect(theme.tooltip.borderColor).toBe(tokens['color-border'])
    expect(theme.tooltip.textStyle.color).toBe(tokens['color-text-primary'])
    expect(theme.pieBorderColor).toBe(tokens['color-bg-card'])
    // 状态色与语义填充档同值（#1895）
    expect(theme.status.ok).toBe(tokens['fill-ok'])
    expect(theme.status.warn).toBe(tokens['fill-warn'])
    expect(theme.status.bad).toBe(tokens['fill-bad'])
  })
})

describe('主题解析与防静默失灵', () => {
  it('默认跟随 <html data-theme>，缺省按浅色', () => {
    expect(resolveChartThemeName()).toBe('light')
    document.documentElement.dataset.theme = 'dark'
    expect(resolveChartThemeName()).toBe('dark')
    expect(chartTheme().series).toEqual(chartTheme('dark').series)
    delete document.documentElement.dataset.theme
  })

  it('两套主题的系列色不是同一份（深色必须提亮，否则压在深底上看不出）', () => {
    expect(chartTheme('light').series).not.toEqual(chartTheme('dark').series)
    expect(chartTheme('light').pieBorderColor).not.toBe(chartTheme('dark').pieBorderColor)
  })
})
