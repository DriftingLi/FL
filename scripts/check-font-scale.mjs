#!/usr/bin/env node
/**
 * 字号阶梯守卫（#1894 / ADR-0080）：字号只走 token 阶梯，禁 `text-[<数字><单位>]` 直写。
 *
 * 背景：`design-tokens.css` 定义了 12/14/16/18/20/24/30 七档，但 2026-10-26 实测前端有 **116 处**
 * 直写尺寸（`text-[9px]` … `text-[22px]`，11 种取值），其中 13px 一种就 64 处，`--text-sm` 几乎没人用。
 * 直写值不参与 token 阶梯：改 `--text-sm` 不会带着它们变，等于两套字号体系各自为政。
 *
 * 规则只有一条：`frontend/src/**` 的 `.vue` / `.ts` 里不得出现工具类 `text-[<数字><单位>]`。
 * 映射表见 `FONT_STEP`：13px → `text-sm`、15px → `text-base`、22px → `text-2xl`。
 *
 * 边界（写清，免得被当成漏网）：
 *   - scoped `<style>` 里的 `font-size: 13px` **不在本期射程**：那是 R2 冻结区页面的既有样式，
 *     按 ADR-0080 决策 3 逐页改版时顺手收。本守卫只认工具类。
 *   - `leading-[1.8]` 这类行高直写值不判：本仓行高没有阶梯。
 *   - `text-[var(--wechat-green)]` 是**颜色**不是字号，不判（正则要求数字 + 单位）。
 *   - `em` / `1em` 不判：相对单位不算「一档字号」，且本仓拿 `text-[1em]` 表达「图标跟随父字号」
 *     （`UiActionChip` 的 UiIcon）。判的是绝对尺寸 —— px 与 rem。
 *
 * 用法：
 *   node scripts/check-font-scale.mjs --all  [目录]   全量扫描（默认 frontend/src；有违规则退出 1）
 *   node scripts/check-font-scale.mjs --diff [base]   只查相对 base 的新增行（base 默认 origin/master）
 */

// runner 面走守卫单点（ADR-0056 §5 / #1094）：argv / 走查 / --diff 取 diff / allowlist / 报告与退出码。
import { isDirectRun, runGuardCli } from './lib/guard.mjs'

/** 扫描面后缀：模板类名在 .vue，类名常量在 .ts（如 config / composable 里的字符串）。 */
export const SCAN_EXTENSIONS = ['.vue', '.ts']

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
 * 逐条登记的有意例外（路径 → 理由）。收口后为空：全树 0 处直写，规则可以绝对化执行。
 * @type {Record<string, string>}
 */
export const ALLOWLIST = {}

/** 任意值字号：`text-[13px]` / `text-[0.875rem]`。要求数字 + 绝对单位，故不误伤颜色与 `1em`。 */
const ARBITRARY_SIZE_RE = /text-\[(\d+(?:\.\d+)?)(px|rem)\]/g

/** 整行注释（含 HTML 注释与块注释续行）不算判定对象。 */
export function isCommentLine(line) {
  const t = String(line).trim()
  return t.startsWith('//') || t.startsWith('*') || t.startsWith('/*') || t.startsWith('<!--')
}

/** 路径是否在守卫面（frontend/src 下的 .vue / .ts）。 */
export function isGuardedPath(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  return p.includes(GUARDED_PATH_SEGMENT) && SCAN_EXTENSIONS.some((ext) => p.endsWith(ext))
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
    // 防空转：当前树 479 个 .vue + .ts，留足增长余量取 400。
    minChecked: 400,
    stream: 'stdout',
    header: (ctx) => [
      '===== 字号阶梯守卫：禁直写字号（#1894 / ADR-0080）=====',
      '扫描目录: ' + ctx.scanDirRel,
      '守卫面: ' + GUARDED_PATH_SEGMENT + '**/*.{vue,ts}',
      '禁入写法: text-[<数字>px] / text-[<数字>rem]（em 是相对单位，允许）',
      '---'
    ],
    ok: (ctx) => '无违规。' + ctx.checked + ' 个文件均只用 token 阶梯字号（scoped font-size 不在射程，见 ADR-0080 决策 3）。',
    violation: format,
    footer: (ctx) => [
      '---',
      '共 ' + ctx.count + ' 处直写字号。阶梯：text-xs 12 / text-sm 14 / text-base 16 / text-lg 18 / text-xl 20 / text-2xl 24 / text-3xl 30。',
      '  13px → text-sm；15px → text-base；22px → text-2xl（与 UiPageHeader 的页标题同档）。'
    ]
  },
  diff: {
    pathspec: ['*.vue', '*.ts'],
    defaultBase: 'origin/master',
    stream: 'stderr',
    empty: (base) => '[check-font-scale] 相对 ' + base + ' 无 .vue / .ts 新增行，跳过。',
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
