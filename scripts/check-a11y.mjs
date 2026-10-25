#!/usr/bin/env node
/**
 * 无障碍档位守卫（#1892）：把「压不住 4.5:1 的文字色」挡在源码层。
 *
 * 背景：前端 8 个守卫（el-controls / api-seam / async-section / api-consumers /
 * ai-assistant-send / render-error-face / catalog-sort / comment-cleanliness）没有一个看颜色，
 * 对比度只能靠人眼与临时审计。token 矩阵锁在 `frontend/src/assets/styles/__tests__/contrast.spec.ts`
 * （判据是全局 token 之间的关系，不是逐行源码模式）；本守卫只管**用法**：
 *
 *   1. `text-ink-muted` —— 该档在浅色 2.45:1、深色 3.07:1，当正文用读不清；正文用 `text-ink-3`（≥4.5）。
 *   2. `var(--color-text-muted)` —— 同上，只是写法在 `<style>` 里；正文用 `--color-text-tertiary`。
 *   3. 同一行既有 `bg-ui-500` 又有 `text-white` / `text-panel` —— 白字压在 brand-500 上只有 3.74:1；
 *      填充抬到 `bg-ui-600`（5.47:1）。`bg-ui-500` 单独出现（圆点、指示条这类无文字图形）不判。
 *   4. 同一行既有 `bg-ok` / `bg-warn` / `bg-bad` 又有浅字 —— 白字压在语义主色上是 2.54 / 1.87 / 3.75；
 *      填充与文字必须成对用 `bg-*-fill` + `text-*-on`（#1895，两主题各自 ≥4.5）。`-soft` 与单独出现的
 *      状态点不判。
 *
 * 为什么白名单为空：这几类在 2026-10-24 / 10-25 的收口里已全部清零（45 处 muted + 10 处 brand-500 +
 * 5 处语义填充），规则可以绝对化执行。确有例外时逐条登记并写明理由，不要放宽规则本身。
 *
 * 用法：
 *   node scripts/check-a11y.mjs --all  [目录]   全量扫描（默认 frontend/src；有违规则退出 1）
 *   node scripts/check-a11y.mjs --diff [base]   只查相对 base 的新增行（base 默认 origin/master）
 */

// runner 面走守卫单点（ADR-0056 §5 / #1094）：argv / 走查 / --diff 取 diff / allowlist / 报告与退出码。
import { isDirectRun, runGuardCli } from './lib/guard.mjs'

/** 扫描面后缀：只认单文件组件（模板类名与 <style> 都在里面）。 */
export const SCAN_EXTENSIONS = ['.vue']

/** 守卫面：前端源码树（runner 传的是仓库相对路径，不带前导斜杠）。 */
export const GUARDED_PATH_SEGMENT = 'frontend/src/'

/**
 * 逐条登记的有意例外（路径 → 理由）。收口后为空：本表保持为空即「规则绝对化」。
 * @type {Record<string, string>}
 */
export const ALLOWLIST = {}

/** 整行注释（含 HTML 注释与块注释续行）不算判定对象。 */
export function isCommentLine(line) {
  const t = String(line).trim()
  return t.startsWith('//') || t.startsWith('*') || t.startsWith('/*') || t.startsWith('<!--')
}

/** 路径是否在守卫面（frontend/src 下的 .vue）。 */
export function isGuardedPath(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  return p.includes(GUARDED_PATH_SEGMENT) && p.endsWith('.vue')
}

/**
 * 逐行规则。`why` 会进报告，说明「为什么这条读不清、该用什么」。
 */
export const RULES = [
  {
    id: 'muted-text',
    re: /\btext-ink-muted\b/,
    why: 'ink-muted 浅色 2.45:1 / 深色 3.07:1，正文请用 text-ink-3（≥4.5）'
  },
  {
    id: 'muted-var',
    re: /var\(--color-text-muted\)/,
    why: '--color-text-muted 同上，正文请用 var(--color-text-tertiary)'
  }
]

/** 浅色填充 + 反白字：白字在 brand-500 上 3.74:1，填充抬到 brand-600。 */
const BRAND_FILL_RE = /\bbg-ui-500\b/
const LIGHT_TEXT_RE = /\btext-(?:white|panel)\b/

/**
 * 语义主色当填充 + 浅字（#1895）：白字压在 success/warning/danger 主色上是
 * 2.54 / 1.87 / 3.75，全部低于 4.5；填充请用成对的 `bg-*-fill` + `text-*-on`。
 * `(?![\w-])` 是为了不误伤 `bg-ok-fill` / `bg-ok-soft` 这类同前缀档。
 */
const SEMANTIC_FILL_RE = /\bbg-(?:ok|warn|bad)(?![\w-])/

/**
 * 扫一份源码，返回违规列表（1-based 行号 + 规则 id + 原文）。
 * 路径不在守卫面即整体放行。
 */
export function scanSource(source, file) {
  if (!isGuardedPath(file)) return []
  const violations = []
  const lines = String(source).split('\n')
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (isCommentLine(line)) continue
    for (const rule of RULES) {
      if (rule.re.test(line)) {
        violations.push({ file, line: i + 1, rule: rule.id, text: line.trim(), why: rule.why })
      }
    }
    if (BRAND_FILL_RE.test(line) && LIGHT_TEXT_RE.test(line)) {
      violations.push({
        file,
        line: i + 1,
        rule: 'brand-fill-500',
        text: line.trim(),
        why: '白字压在 bg-ui-500 上 3.74:1，填充请用 bg-ui-600（5.47）'
      })
    }
    if (SEMANTIC_FILL_RE.test(line) && LIGHT_TEXT_RE.test(line)) {
      violations.push({
        file,
        line: i + 1,
        rule: 'semantic-fill',
        text: line.trim(),
        why: '白字压在 bg-ok/warn/bad 上只有 2.54/1.87/3.75，请用成对的 bg-*-fill + text-*-on'
      })
    }
  }
  return violations
}

const format = (v) => `${v.file}:${v.line}: ${v.rule}  ${v.text}\n    → ${v.why}`

/** 本守卫的声明（形状对齐 scripts/lib/guard.mjs）。 */
export const GUARD_SPEC = {
  name: 'check-a11y',
  usage: '用法: node scripts/check-a11y.mjs --all [目录] | --diff [base]',
  cli: { noArgs: 'all', helpFlag: true, scanDirArg: true, usageOnUnknown: true, usageStream: 'stderr' },
  all: {
    scanDir: 'frontend/src',
    extensions: SCAN_EXTENSIONS,
    skipNodeModules: true,
    tolerateWalkErrors: true,
    // 防空转：判定面掉到个位数（前缀改名 / 目录搬走 / 判据写错）时必须红，
    // 否则报告读起来与「零违规」一模一样。当前树 169 个 .vue，留足增长余量取 120。
    minChecked: 120,
    stream: 'stdout',
    header: (ctx) => [
      '===== 无障碍档位守卫：低对比度文字档（#1892）=====',
      '扫描目录: ' + ctx.scanDirRel,
      '守卫面: ' + GUARDED_PATH_SEGMENT + '**/*.vue',
      '禁入写法: text-ink-muted / var(--color-text-muted) / bg-ui-500 或 bg-ok|warn|bad 配 text-white|text-panel',
      '---'
    ],
    ok: (ctx) =>
      '无违规。' + ctx.checked + ' 个 .vue 均未使用低对比度文字档（token 矩阵锁另见 contrast.spec.ts）。',
    violation: format,
    footer: (ctx) => [
      '---',
      '共 ' + ctx.count + ' 处。判据与实测：浅色 ink-muted 2.45:1、深色 3.07:1；白字 on brand-500 3.74:1。',
      '  正文 → text-ink-3 / var(--color-text-tertiary)；带字的品牌填充 → bg-ui-600。'
    ]
  },
  diff: {
    pathspec: ['*.vue'],
    defaultBase: 'origin/master',
    stream: 'stderr',
    empty: (base) => '[check-a11y] 相对 ' + base + ' 无 .vue 新增行，跳过。',
    ok: () => '[check-a11y] 新增行无低对比度文字档，通过。',
    header: () => ['===== 新增行出现低对比度文字档 ====='],
    violation: format,
    footer: (ctx) => ['---', '共 ' + ctx.count + ' 处。见 #1892 与 docs/agents/checks.md。']
  },
  isGuardedPath,
  scanSource,
  allowlist: ALLOWLIST
}

if (isDirectRun(import.meta.url)) runGuardCli(GUARD_SPEC)
