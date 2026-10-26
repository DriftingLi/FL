// 无障碍锁：设计 token 的对比度矩阵（WCAG 2.x AA）。#1892
//
// 为什么锁在 vitest 而不是守卫脚本里：这里的判据是「全局 token 之间的关系」，不是逐行源码模式；
// 仓库对这类不变量的既有做法就是 vitest 锁（statusWords.spec.ts 直接读 Go 常量表，同理）。
// 用法用了错的「组合」由 scripts/check-a11y.mjs 那层管。
//
// 判据来自 2026-10-24 的实测：主按钮渐变原为 #0D9488 → #0EA5E9（白字 3.74 / 2.77），
// color-text-muted 当正文用是 2.45 / 3.07 —— 都低于 4.5。本锁把「两端都 ≥4.5」钉成事实。
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const css = readFileSync(resolve(__dirname, '../design-tokens.css'), 'utf8')

type Tokens = Record<string, string>

/** 按选择器定位一个块，按花括号配对截取内容（不引第三方 CSS parser，够用且无依赖） */
function blockOf(source: string, selector: RegExp): string {
  const m = selector.exec(source)
  if (!m) throw new Error('design-tokens.css 里找不到选择器: ' + selector)
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

function decls(block: string): Tokens {
  const out: Tokens = {}
  for (const m of block.matchAll(/--([\w-]+)\s*:\s*([^;]+);/g)) out[m[1]] = m[2].trim()
  return out
}

const lightTokens = decls(blockOf(css, /\n:root\s*\{/))
const darkTokens: Tokens = { ...lightTokens, ...decls(blockOf(css, /\[data-theme="dark"\]\s*\{/)) }

const HEX_RE = /^#[0-9a-f]{6}$/i

/** token 名（不带 --）或裸 hex → 统一解析成 #rrggbb */
function resolveValue(raw: string, tokens: Tokens, depth = 0): string {
  if (depth > 8) throw new Error('token 引用链过深（疑似自引用）: ' + raw)
  const v = raw.trim()
  if (HEX_RE.test(v)) return v
  const direct = v.match(/^var\(--([\w-]+)\)$/)
  if (direct) {
    const next = tokens[direct[1]]
    if (next === undefined) throw new Error('未定义的 token: --' + direct[1])
    return resolveValue(next, tokens, depth + 1)
  }
  const mix = v.match(/^color-mix\(in srgb,\s*var\(--([\w-]+)\)\s*([\d.]+)%,\s*var\(--([\w-]+)\)\s*\)$/)
  if (mix) {
    const a = resolveToken(mix[1], tokens, depth + 1)
    const b = resolveToken(mix[3], tokens, depth + 1)
    return mixSrgb(a, b, Number(mix[2]) / 100)
  }
  throw new Error('无法解析的 token 值: ' + v)
}

function resolveToken(name: string, tokens: Tokens, depth = 0): string {
  const raw = tokens[name]
  if (raw === undefined) throw new Error('未定义的 token: --' + name)
  return resolveValue(raw, tokens, depth)
}

function mixSrgb(a: string, b: string, weightA: number): string {
  const pa = hexToRgb(a)
  const pb = hexToRgb(b)
  const out = pa.map((v, i) => Math.round(v * weightA + pb[i] * (1 - weightA)))
  return '#' + out.map((v) => v.toString(16).padStart(2, '0')).join('').toUpperCase()
}

function hexToRgb(hex: string): number[] {
  return hex.replace('#', '').match(/../g)!.map((x) => parseInt(x, 16))
}

function relativeLuminance(hex: string): number {
  const [r, g, b] = hexToRgb(hex).map((c) => {
    const s = c / 255
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function contrast(fg: string, bg: string): number {
  const l1 = relativeLuminance(fg)
  const l2 = relativeLuminance(bg)
  const [hi, lo] = l1 > l2 ? [l1, l2] : [l2, l1]
  return (hi + 0.05) / (lo + 0.05)
}

/** 矩阵里既可以写裸 hex、也可以写 token 名（不带 --）或完整 var(...) */
function pick(spec: string, tokens: Tokens): string {
  if (HEX_RE.test(spec)) return spec
  return spec.startsWith('var(') ? resolveValue(spec, tokens) : resolveToken(spec, tokens)
}

/** [说明, 前景, 背景, 下限]；前景/背景可写 token 名或裸 hex */
const PAIRS: Array<[string, string, string, number]> = [
  ['正文 ink / 页面底', 'color-text-primary', 'color-bg-page', 4.5],
  ['正文 ink / 卡片底', 'color-text-primary', 'color-bg-card', 4.5],
  ['次要 ink2 / 页面底', 'color-text-secondary', 'color-bg-page', 4.5],
  ['次要 ink2 / 卡片底', 'color-text-secondary', 'color-bg-card', 4.5],
  ['三级 ink3 / 页面底', 'color-text-tertiary', 'color-bg-page', 4.5],
  ['三级 ink3 / 卡片底', 'color-text-tertiary', 'color-bg-card', 4.5],
  // 占位符独立成档（#1934）：此前借 disabled，浅色只有 1.42:1
  ['占位符 / 页面底', 'color-text-placeholder', 'color-bg-page', 4.5],
  ['占位符 / 卡片底', 'color-text-placeholder', 'color-bg-card', 4.5],
  ['主按钮白字 / 品牌 600 填充', '#FFFFFF', 'color-primary-600', 4.5],
  ['品牌浅底深字 / 品牌浅底', 'color-primary-soft-ink', 'color-primary-soft', 4.5],
  ['成功深字 / 成功浅底', 'color-success-strong', 'color-success-light', 4.5],
  ['警告深字 / 警告浅底', 'color-warning-strong', 'color-warning-light', 4.5],
  ['危险深字 / 危险浅底', 'color-danger-strong', 'color-danger-light', 4.5],
  // 实心填充成对档（#1895）：浅色深底配白字、深色亮底配深字
  ['成功填充文字 / 成功填充', 'on-fill-ok', 'fill-ok', 4.5],
  ['警告填充文字 / 警告填充', 'on-fill-warn', 'fill-warn', 4.5],
  ['危险填充文字 / 危险填充', 'on-fill-bad', 'fill-bad', 4.5]
]

for (const [theme, tokens] of [
  ['浅色', lightTokens],
  ['深色', darkTokens]
] as const) {
  describe(`${theme} token 对比度矩阵`, () => {
    for (const [label, fg, bg, min] of PAIRS) {
      it(`${label} ≥ ${min}:1`, () => {
        const ratio = contrast(pick(fg, tokens), pick(bg, tokens))
        expect(ratio, `${label} 实测 ${ratio.toFixed(2)}:1（${theme}）`).toBeGreaterThanOrEqual(min)
      })
    }

    it('主按钮渐变两端白字都 ≥ 4.5:1', () => {
      const stops = (tokens['gradient-brand'] ?? '').match(/#[0-9a-f]{6}/gi) ?? []
      expect(stops.length, '--gradient-brand 必须有两个色标，否则本锁形同虚设').toBe(2)
      for (const stop of stops) {
        const ratio = contrast('#FFFFFF', stop)
        expect(ratio, `渐变端点 ${stop} 白字 ${ratio.toFixed(2)}:1（${theme}）`).toBeGreaterThanOrEqual(4.5)
      }
    })
  })
}

// ---- Element Plus 变量面（#1934）：EP 自己的语义色阶，判据在 element-overrides.css ----
// 为什么单开一段：上面的矩阵只读 design-tokens.css，EP 的变量映射一直不在判据里 ——
// 「error 家族漏覆盖」这个 bug 就是这么活下来的（message 文字回落出厂红，矩阵一无所知）。
const overridesCss = readFileSync(resolve(__dirname, '../element-overrides.css'), 'utf8')
/** element-overrides.css 的 :root 在文件首行、前面没有换行，故选择器要带 ^ 分支 */
const elOverridesLight = decls(blockOf(overridesCss, /(?:^|\n):root\s*\{/))
const elOverridesDark = decls(blockOf(overridesCss, /(?:^|\n)\[data-theme="dark"\]\s*\{/))
const elLight = { ...lightTokens, ...elOverridesLight }
const elDark = { ...darkTokens, ...elOverridesDark }

/** 错误提示文字色走 danger-strong（element-overrides.css 末段已把 message/alert 指过去） */
const EP_ERROR_TEXT = 'color-danger-strong'
const EP_ERROR_SURFACE = 'el-color-error-light-9'

for (const [theme, tokens] of [
  ['浅色', elLight],
  ['深色', elDark]
] as const) {
  describe(`Element 语义色阶（${theme}）`, () => {
    it('error 家族六档与 danger 同值（EP 的 message / alert 走 error，漏覆盖会静默回落出厂红）', () => {
      for (const suffix of ['', '-light-3', '-light-5', '-light-7', '-light-9', '-dark-2']) {
        expect(tokens[`el-color-error${suffix}`], `--el-color-error${suffix} 与 danger 不一致（${theme}）`)
          .toBe(tokens[`el-color-danger${suffix}`])
      }
    })

    it('错误提示文字压在错误浅底上 ≥ 4.5:1', () => {
      const ratio = contrast(resolveToken(EP_ERROR_TEXT, tokens), resolveToken(EP_ERROR_SURFACE, tokens))
      expect(ratio, `错误提示实测 ${ratio.toFixed(2)}:1（${theme}）`).toBeGreaterThanOrEqual(4.5)
    })
  })
}

describe('矩阵本身没有静默失灵', () => {
  it('两个块都解析出了足量 token，且关键底色与源文件一致', () => {
    expect(Object.keys(lightTokens).length).toBeGreaterThan(20)
    expect(Object.keys(darkTokens).length).toBeGreaterThan(40)
    expect(lightTokens['color-bg-card']).toBe('#FFFFFF')
    expect(darkTokens['color-bg-card']).toBe('#1E293B')
    // 深色块只覆盖「表面 / 文字 / 边框 / 语义浅底」，品牌色阶不翻
    expect(darkTokens['color-primary-500']).toBe(lightTokens['color-primary-500'])
  })

  it('color-mix 解析（品牌浅底）不是把原值当 hex 放过', () => {
    const soft = resolveValue('var(--color-primary-soft)', lightTokens)
    expect(soft).toMatch(/^#[0-9A-F]{6}$/)
    expect(soft).not.toBe(lightTokens['color-primary-500'].toUpperCase())
  })

  it('Element 覆盖面解析成功，且占位符不再借 disabled（#1934）', () => {
    expect(Object.keys(elOverridesLight).length).toBeGreaterThan(40)
    expect(Object.keys(elOverridesDark).length).toBeGreaterThan(10)
    expect(resolveToken('color-text-placeholder', lightTokens)).toBe(resolveToken('color-text-tertiary', lightTokens))
    expect(resolveToken('color-text-placeholder', darkTokens)).toBe('#94A3B8')
  })
})
