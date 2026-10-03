#!/usr/bin/env node
/**
 * 注释三态守卫（issue #1445 的 P4；判据来源 docs/agents/comments.md 第 3 节「删除」类）。
 *
 * 锁三类**可机检且无歧义**的注释欠账，基线为 0 —— 只减不增：
 *   1. 失真包文档：文件头写 Package <名>，而该文件的 package 子句不是那个名。
 *      实证：P3 把 internal/service 更名为 internal/core 之后，42 个文件的头注释仍写 service/api。
 *      CI 的 golangci-lint 只开 errcheck/gosimple/govet/ineffassign/staticcheck/unused，
 *      stylecheck 一族不在列表，这一类没人抓。
 *   2. 装饰分隔线：整行是 8 个及以上同一个符号（= - # * ~ _ +）的注释行。它只做分区排版，
 *      不携带任何事实；标题行本身（文字）保留。
 *   3. 相邻重复注释行：连续两行去掉行首空白后逐字相同。
 *
 * 明确**不锁**的两类（理由见 docs/agents/comments.md 第 5 节）：
 *   - 注释率（注释行 / 非空行）：会给新增解释性注释判红，与「保留类」判据冲突。
 *   - 「注释掉的代码」语法启发式：实测 24 处命中全是公式 / 路由形态 / //go:embed 指令 /
 *     文档示例，精度约 0。
 *
 * 用法（runner 面单点在 scripts/lib/guard.mjs）：
 *   node scripts/check-comment-cleanliness.mjs --all            全量扫 backend 下的 .go
 *   node scripts/check-comment-cleanliness.mjs --diff [base]    只查相对 base 的新增行
 */
import { isDirectRun, runGuardCli } from './lib/guard.mjs'

/** 只在文件头这几行里找 Package 文档（包文档按 Go 惯例写在 package 子句之前）。 */
export const PACKAGE_DOC_HEAD_LINES = 6

/** 装饰分隔线允许的符号集（每个符号连续出现 SEPARATOR_MIN_LEN 次以上才算）。 */
export const SEPARATOR_SYMBOLS = '=-#*~_+'
export const SEPARATOR_MIN_LEN = 8

/** 去掉行首的 // 与空白；不是注释行返回 null。 */
export function commentBody(text) {
  let t = text.trim()
  if (!t.startsWith('//')) return null
  while (t.startsWith('/')) t = t.slice(1)
  return t.trim()
}

/** 形如 // Package foo 的行返回 foo，否则返回空串。 */
export function packageDocName(text) {
  const body = commentBody(text)
  if (body === null || !body.startsWith('Package ')) return ''
  return body.slice('Package '.length).split(' ')[0].trim()
}

/** 源码里的 package 子句名（找不到返回空串）。 */
export function packageClause(src) {
  for (const raw of src.split('\n')) {
    const t = raw.trim()
    if (!t.startsWith('package ')) continue
    return t.slice('package '.length).split(' ')[0].trim()
  }
  return ''
}

/** 整行由同一个符号（8 个及以上）组成的注释行。 */
export function isDecorativeSeparator(text) {
  const body = commentBody(text)
  if (body === null || body.length < SEPARATOR_MIN_LEN) return false
  const first = body[0]
  if (!SEPARATOR_SYMBOLS.includes(first)) return false
  for (const ch of body) if (ch !== first) return false
  return true
}

/** 判定面：一份 Go 源码里的三类违规（line 从 1 开始）。 */
export function scanSource(src) {
  const out = []
  const lines = src.split('\n')
  const actual = packageClause(src)
  for (let i = 0; i < Math.min(lines.length, PACKAGE_DOC_HEAD_LINES); i++) {
    // 只看**注释块的第一行**：Go 的包文档写在块首；把「块中间某行恰好以 Package 开头」也算上，
    // 会把「本文件说明：/ // Package service 的旧形态见 ADR-0070。」这类散文误判成包文档。
    if (commentBody(lines[i]) === null) continue
    if (i > 0 && commentBody(lines[i - 1]) !== null) continue
    const declared = packageDocName(lines[i])
    if (declared === '') continue
    if (actual !== '' && declared !== actual) {
      out.push({
        line: i + 1,
        kind: 'package-doc-mismatch',
        message: '文件头写 Package ' + declared + '，但本文件的 package 子句是 ' + actual + '（失真包文档）'
      })
    }
    break
  }
  for (let i = 0; i < lines.length; i++) {
    const body = commentBody(lines[i])
    if (body === null) continue
    if (isDecorativeSeparator(lines[i])) {
      out.push({
        line: i + 1,
        kind: 'decorative-separator',
        message: '装饰性分隔线（只有分区排版作用，不携带事实；标题行保留、两条符号行删掉）'
      })
    }
    if (i > 0 && body !== '' && lines[i].trim() === lines[i - 1].trim()) {
      out.push({
        line: i + 1,
        kind: 'duplicate-comment',
        message: '与上一行注释逐字重复'
      })
    }
  }
  return out
}

export const GUARD_SPEC = {
  name: 'check-comment-cleanliness',
  usage: [
    '用法:',
    '  node scripts/check-comment-cleanliness.mjs --all           全量扫 backend/**/*.go（有违规则退出 1）',
    '  node scripts/check-comment-cleanliness.mjs --diff [base]   只查相对 base 的新增行（base 默认 origin/master）'
  ].join('\n'),
  cli: { noArgs: 'all', helpFlag: true, usageStream: 'stdout' },
  all: {
    scanDir: 'backend',
    extensions: ['.go'],
    skipNodeModules: true,
    minChecked: 600,
    header: (ctx) => [
      '[check-comment-cleanliness] 全量扫描 ' + ctx.scanDirRel + ' 下的 .go：失真包文档 / 装饰分隔线 / 相邻重复注释行'
    ],
    ok: (ctx) => '[check-comment-cleanliness] ✓ 通过：' + ctx.checked + ' 个 .go 文件，三类欠账 0 处',
    violation: (v) => '  ' + v.file + ':' + v.line + '  [' + v.kind + '] ' + v.message,
    footer: (ctx) => [
      '共 ' + ctx.count + ' 处。判据见 docs/agents/comments.md 第 3 节；基线为 0，只减不增——确有例外须逐条登记在 allowlist 并写明理由。'
    ]
  },
  diff: {
    pathspec: ['backend'],
    defaultBase: 'origin/master',
    empty: (base) => '[check-comment-cleanliness] ✓ 相对 ' + base + ' 无新增行（跳过）',
    ok: (base) => '[check-comment-cleanliness] ✓ 相对 ' + base + ' 的新增行里无违规',
    violation: (v) => '  ' + v.file + ':' + v.line + '  [' + v.kind + '] ' + v.message,
    footer: (ctx) => ['共 ' + ctx.count + ' 处新增违规（判据与 --all 同一份 scanSource）。']
  },
  isGuardedPath: (file) => file.startsWith('backend/') && file.endsWith('.go'),
  scanSource,
  allowlist: {}
}

if (isDirectRun(import.meta.url)) runGuardCli(GUARD_SPEC)
