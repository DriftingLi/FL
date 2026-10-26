#!/usr/bin/env node
/**
 * 字号阶梯守卫（#1894 / ADR-0080）：字号只走 token 阶梯，工具类与 scoped 样式都算。
 *
 * 背景（两遍收口）：`design-tokens.css` 定义了 12/14/16/18/20/24/30 七档，但前端长期在用的是另一套 ——
 * 2026-10-26 实测 **116 处** `text-[<n>px]` 直写（#1929 收口）与 **152 处** scoped `font-size: <n>px`
 * （本轮收口）。直写值不参与 token 阶梯：改 `--text-sm` 不会带着它们变，等于两套字号体系各自为政；
 * 其中 9/10/11px 还有可读性问题（不是整齐问题）。
 *
 * 两条规则：
 *   1. `arbitrary-font-size` —— `frontend/src/**` 里不得出现工具类 `text-[<数字><单位>]`
 *      （`text-[var(…)]` 是颜色、`text-[1em]` 是相对单位，都不判）。
 *   2. `literal-font-size` —— `font-size: <字面量>` 必须写成 `var(--text-*)`；
 *      残值域自有的 `var(--fs-*)`（可带 px 兜底）同样放行。只有两类例外：
 *      · **展示级** 28 / 40 / 44 / 64px —— 大号统计数字与占位字形，不进七档（见 DISPLAY_TIER）；
 *      · **rem 基准** —— `html { font-size: … }` 那三档（16/14/13），声明前一行带
 *        `font-scale-root` 标记（INHERIT_BASE_MARKER）才放行：它派生整个 rem 尺度，
 *        跟字号阶梯无关，改成 `var(--text-*)` 会让「改 token」变成「整体缩放页面」。
 *
 * 映射口径（等距**向上取**，宁可大 1px 也不把正文压小）：9–12→`xs`、13–14→`sm`、15–16→`base`、
 * 17–18→`lg`、20→`xl`、21–24→`2xl`。
 *
 * 用法：
 *   node scripts/check-font-scale.mjs --all  [目录]   全量扫描（默认 frontend/src；有违规则退出 1）
 *   node scripts/check-font-scale.mjs --diff [base]   只查相对 base 的新增行（base 默认 origin/master）
 */

// runner 面走守卫单点（ADR-0056 §5 / #1094）：argv / 走查 / --diff 取 diff / allowlist / 报告与退出码。
import { isDirectRun, runGuardCli } from './lib/guard.mjs'

/** 扫描面后缀：模板类名在 .vue，类名常量在 .ts，scoped 样式在 .vue / .css。 */
export const SCAN_EXTENSIONS = ['.vue', '.ts', '.css']

/** 守卫面：前端源码树（runner 传的是仓库相对路径，不带前导斜杠）。 */
export const GUARDED_PATH_SEGMENT = 'frontend/src/'

/** 字号阶梯（px → 工具类）。取值源是 `assets/styles/design-tokens.css` 的 `--text-*`。 */
export const FONT_STEP = {
  12: 'text-xs',
  14: 'text-sm',
  16: 'text-base',
  18: 'text-lg',
  20: 'text-xl',
  24: 'text-2xl',
  30: 'text-3xl'
}

/**
 * 展示级字号：大号统计数字 / 占位字形，**不进七档**。
 * 现状 5 处（`ProfileEditDialog` 头像字 28、`Statistics` 统计值 28、`CourseCard` 封面占位字 40、
 * 残值 `.num-hero` 44 / 64）。新增展示级取值要改这里并在 ADR-0080 登记，不许就地写别的 px。
 */
export const DISPLAY_TIER = new Set([28, 40, 44, 64])

/** rem 基准标记：`html { font-size: … }` 那三档，声明前一行的注释里必须带这个词。 */
export const INHERIT_BASE_MARKER = 'font-scale-root'

/** 关键字值与 `inherit` 不判：它们不是「一档字号」。 */
const KEYWORD_VALUES = new Set(['inherit', 'initial', 'unset', 'revert', 'revert-layer'])

/**
 * 逐条登记的有意例外（路径 → 理由）。收口后为空：全树只剩展示级与 rem 基准，
 * 两者都由**值/标记**判定，不需要按文件开例外。
 * @type {Record<string, string>}
 */
export const ALLOWLIST = {}

/** 任意值字号：`text-[13px]` / `text-[0.875rem]`。要求数字 + 绝对单位，故不误伤颜色与 `1em`。 */
const ARBITRARY_SIZE_RE = /text-\[(\d+(?:\.\d+)?)(px|rem)\]/g

/** scoped 声明：`font-size: 13px` / `font-size:var(--text-sm)`。 */
const FONT_SIZE_DECL_RE = /\bfont-size\s*:\s*([^;}]+)/g

/** 合法值：`var(--text-…)` / `var(--fs-…[, 兜底])`，允许 `!important` 后缀。 */
const TOKEN_VALUE_RE = /^var\(\s*--(?:text|fs)-[a-z0-9-]+\s*(?:,[^()]*)?\)$/

/** 裸 px 值：`13px` / `0.875px`。 */
const PX_VALUE_RE = /^(\d+(?:\.\d+)?)px$/

/** 整行注释（含 HTML 注释与块注释续行）不算判定对象。 */
export function isCommentLine(line) {
  const t = String(line).trim()
  return t.startsWith('//') || t.startsWith('*') || t.startsWith('/*') || t.startsWith('<!--')
}

/** 路径是否在守卫面（frontend/src 下的 .vue / .ts / .css）。 */
export function isGuardedPath(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  return p.includes(GUARDED_PATH_SEGMENT) && SCAN_EXTENSIONS.some((ext) => p.endsWith(ext))
}

/** 往上找最近一条非空行（跳过空行；不看注释与否——标记本身就在注释里）。 */
function previousNonEmpty(lines, index) {
  for (let i = index - 1; i >= 0; i--) {
    if (lines[i].trim() !== '') return lines[i]
  }
  return ''
}

/**
 * 给直写值找一个最近的阶梯档（px 才换算；rem 只提示「换 token」）。
 * 等距时**向上取** —— 与本次收口的实际映射一致（13→14、15→16、17→18、22→24），
 * 口径是「宁可大 1px，也不把正文压小」。
 */
export function suggestStep(value, unit) {
  if (unit !== 'px') return '阶梯里的一档（12/14/16/18/20/24/30）'
  const px = Number(value)
  const steps = Object.keys(FONT_STEP).map(Number)
  let best = steps[0]
  for (const step of steps) {
    const dist = Math.abs(step - px)
    const bestDist = Math.abs(best - px)
    if (dist < bestDist || (dist === bestDist && step > best)) best = step
  }
  return FONT_STEP[best] + '（' + best + 'px）'
}

/**
 * 单个 `font-size` 声明是否合法。返回 null = 合法，否则返回一句为什么。
 * 拆出来是为了自检能单测判定面，而不用整段源码跑。
 */
export function judgeFontSizeValue(rawValue, lines, lineIndex) {
  const value = String(rawValue).trim().replace(/\s*!important$/i, '').trim()
  if (KEYWORD_VALUES.has(value)) return null
  if (TOKEN_VALUE_RE.test(value)) return null
  const px = PX_VALUE_RE.exec(value)
  if (px) {
    const n = Number(px[1])
    if (DISPLAY_TIER.has(n)) return null
    // 标记只对紧邻的这条声明生效：上一行必须是带 font-scale-root 的注释
    if (previousNonEmpty(lines, lineIndex).includes(INHERIT_BASE_MARKER)) return null
    return (
      '机检能读出的 px 字号必须写 var(--text-*)（残值域 var(--fs-*, Npx)）；' +
      '这个值最近的一档是 ' +
      suggestStep(px[1], 'px') +
      '（展示级只有 28/40/44/64px，rem 基准要带 ' +
      INHERIT_BASE_MARKER +
      ' 标记）'
    )
  }
  return '字号请写 var(--text-*)（残值域 var(--fs-*, Npx)）或关键字值；不要用 rem / em / calc 之类的直写'
}

/** 扫一份源码，返回违规列表（1-based 行号 + 原文 + 建议）。路径不在守卫面即整体放行。 */
export function scanSource(source, file) {
  if (!isGuardedPath(file)) return []
  const violations = []
  const lines = String(source).split('\n')
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (isCommentLine(line)) continue

    ARBITRARY_SIZE_RE.lastIndex = 0
    let m
    while ((m = ARBITRARY_SIZE_RE.exec(line)) !== null) {
      violations.push({
        file,
        line: i + 1,
        rule: 'arbitrary-font-size',
        text: m[0],
        why:
          '字号只走 token 阶梯（12/14/16/18/20/24/30）；这个直写值请换成 ' +
          suggestStep(m[1], m[2])
      })
    }

    FONT_SIZE_DECL_RE.lastIndex = 0
    let d
    while ((d = FONT_SIZE_DECL_RE.exec(line)) !== null) {
      const why = judgeFontSizeValue(d[1], lines, i)
      if (why) {
        violations.push({
          file,
          line: i + 1,
          rule: 'literal-font-size',
          text: 'font-size: ' + d[1].trim(),
          why
        })
      }
    }
  }
  return violations
}

const format = (v) => `${v.file}:${v.line}: ${v.rule}  ${v.text}\n    → ${v.why}`

/** 本守卫的声明（形状对齐 scripts/lib/guard.mjs）。 */
export const GUARD_SPEC = {
  name: 'check-font-scale',
  usage: '用法: node scripts/check-font-scale.mjs --all [目录] | --diff [base]',
  cli: { noArgs: 'all', helpFlag: true, scanDirArg: true, usageOnUnknown: true, usageStream: 'stderr' },
  all: {
    scanDir: 'frontend/src',
    extensions: SCAN_EXTENSIONS,
    skipNodeModules: true,
    tolerateWalkErrors: true,
    // 防空转：当前树 482 个 .vue + .ts + .css，留足增长余量取 400。
    minChecked: 400,
    stream: 'stdout',
    header: (ctx) => [
      '===== 字号阶梯守卫：工具类与 scoped 样式都不许直写（#1894 / ADR-0080）=====',
      '扫描目录: ' + ctx.scanDirRel,
      '守卫面: ' + GUARDED_PATH_SEGMENT + '**/*.{vue,ts,css}',
      '禁入写法: text-[<数字>px|rem] · font-size: <数字>px|rem|em（要 var(--text-*)）',
      '放行面: text-[1em] / text-[var(…)] · font-size: var(--text-*) / var(--fs-*, Npx) · 展示级 ' +
        [...DISPLAY_TIER].join('/') + 'px · html 的 rem 基准（上一行带 ' + INHERIT_BASE_MARKER + ' 标记）',
      '---'
    ],
    ok: (ctx) =>
      '无违规。' + ctx.checked + ' 个文件：工具类无任意值字号，scoped font-size 全走 token（展示级与 rem 基准按值/标记放行）。',
    violation: format,
    footer: (ctx) => [
      '---',
      '共 ' + ctx.count + ' 处直写字号。阶梯：text-xs 12 / text-sm 14 / text-base 16 / text-lg 18 / text-xl 20 / text-2xl 24 / text-3xl 30。',
      '  13px → text-sm；15px → text-base；22px → text-2xl（与 UiPageHeader 的页标题同档）。'
    ]
  },
  diff: {
    pathspec: ['*.vue', '*.ts', '*.css'],
    defaultBase: 'origin/master',
    stream: 'stderr',
    empty: (base) => '[check-font-scale] 相对 ' + base + ' 无 .vue / .ts / .css 新增行，跳过。',
    ok: () => '[check-font-scale] 新增行无直写字号，通过。',
    header: () => ['===== 新增行出现直写字号 ====='],
    violation: format,
    footer: (ctx) => ['---', '共 ' + ctx.count + ' 处。见 #1894 与 docs/agents/ui-conventions.md「字号」。']
  },
  isGuardedPath,
  scanSource,
  allowlist: ALLOWLIST
}

if (isDirectRun(import.meta.url)) runGuardCli(GUARD_SPEC)
