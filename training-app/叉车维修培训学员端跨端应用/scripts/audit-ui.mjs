#!/usr/bin/env node
/**
 * audit:ui —— 十项视觉审计的只读聚合入口（#2864；契约 #2836 / T7）
 *
 * 为什么有它：十项视觉审计的判据散在 25 个 jest 套件与 1 个诊断脚本里，
 * 「哪些项有判据在看、哪些没有」只能靠人翻套件。本工具把同一批事实按 10 项归位，
 * 输出「已扫描 / 未扫描」两栏，未扫描项必须带闭合原因码。
 *
 * 边界（#2715，不可越；逐条见契约 §10）：
 *   1. 不进 CI 必检 —— 本文件不出现在任何 workflow 的 run: 里；
 *   2. 不进 pre-commit；
 *   3. 不进 pr-evidence，不改 .github/scripts/lib/gate-validator.mjs；
 *   4. 不改 ruleset，不新增必检 job；
 *   5. --strict 是本地自选，与 audit-spacing 同义。
 * 一句话：**只读聚合，零判红面**。它读的是既有契约套件，那些套件红了 ③ 门本来就红。
 *
 * 用法：
 *   npm run audit:ui              # 两张表
 *   npm run audit:ui -- --json    # 机读（同一棵树两次跑逐字节相同）
 *   npm run audit:ui -- --strict  # 已扫描项有断言失败时 exit 1
 *
 * 退出码：0 = 完成；1 = --strict 下已扫描项有失败；2 = 来源读失败（fail-closed）
 *
 * 与契约 §5 的两处差异（均为「契约未定义 / 不可执行」的收敛，不是放宽）：
 *   ① 不设 gateway 字段：契约里它只出现一次且恒为 null，无任何语义；
 *   ② note / reasonCode 不写进 items.json，由 coverage 与 requiredSources 推导
 *      （point→point-only、none→no-criteria、含 spacing→ruled-no-gate），
 *      避免同一事实在数据文件与代码里各存一份。
 *
 * 一处对契约 §4 的收紧说明：evidence.assertions 在「已扫描」行通常是数字；
 * 唯一例外是 jest 来源整体不可用时仍靠 spacing 读数的项（项 1 / 5）——此时填 null
 * 而不是 0，因为 0 会被读成「扫了且全绿」，正是 §4 明令禁止的。
 */
import { existsSync, readFileSync } from 'node:fs'
import { spawnSync } from 'node:child_process'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const APP_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const ITEMS_PATH = join(APP_ROOT, 'scripts', 'audit-ui-items.json')
const SPACING_SCRIPT = join(APP_ROOT, 'scripts', 'audit-spacing.mjs')
const JEST_BIN = join(APP_ROOT, 'node_modules', 'jest', 'bin', 'jest.js')
const JEST_CONFIG = 'jest.config.unit.js'
const TIMEOUT_MS = 120_000

const COVERAGES = ['full', 'narrow', 'point', 'none']
const SOURCES = ['jest', 'spacing']
const NOTE_RULED_NO_GATE = 'ruled-no-gate'

/** coverage → 两栏归属。point 算未扫描是刻意的：点状断言 ≠ 全仓扫描。 */
export function statusForCoverage(coverage) {
  if (coverage === 'full' || coverage === 'narrow') return 'scanned'
  if (coverage === 'point' || coverage === 'none') return 'not-scanned'
  throw new Error('未知 coverage：' + coverage)
}

/** 未扫描项的闭合原因码；已扫描项恒 null。 */
export function reasonCodeForCoverage(coverage) {
  if (coverage === 'point') return 'point-only'
  if (coverage === 'none') return 'no-criteria'
  return null
}

/** 已扫描项的 note：间距类项由 #2715 裁定「机检不立门」，其余无话可说。 */
export function noteForItem(item) {
  return item.requiredSources.includes('spacing') ? NOTE_RULED_NO_GATE : null
}

/** 读判据表并做结构校验；任何不合规都抛错（调用方 fail-closed 成 exit 2）。 */
export function loadItems(path = ITEMS_PATH) {
  const raw = JSON.parse(readFileSync(path, 'utf8'))
  if (raw.schemaVersion !== 1) throw new Error('audit-ui-items.json：schemaVersion 必须为 1')
  if (!Array.isArray(raw.items)) throw new Error('audit-ui-items.json：缺 items 数组')
  const ids = new Set()
  const seqs = new Set()
  for (const it of raw.items) {
    if (typeof it.id !== 'string' || it.id === '') throw new Error('audit-ui-items.json：id 必须是非空字符串')
    if (ids.has(it.id)) throw new Error('audit-ui-items.json：id 重复 ' + it.id)
    ids.add(it.id)
    if (!Number.isInteger(it.seq) || it.seq < 1 || it.seq > 10) throw new Error('audit-ui-items.json：seq 必须是 1–10 的整数（' + it.id + '）')
    if (seqs.has(it.seq)) throw new Error('audit-ui-items.json：seq 重复 ' + it.seq)
    seqs.add(it.seq)
    if (typeof it.label !== 'string' || it.label === '') throw new Error('audit-ui-items.json：label 必须是非空字符串（' + it.id + '）')
    if (!COVERAGES.includes(it.coverage)) throw new Error('audit-ui-items.json：coverage 必须是 ' + COVERAGES.join('/') + '（' + it.id + '）')
    if (!Array.isArray(it.requiredSources) || it.requiredSources.length === 0) throw new Error('audit-ui-items.json：requiredSources 必须是非空数组（' + it.id + '）')
    for (const s of it.requiredSources) {
      if (!SOURCES.includes(s)) throw new Error('audit-ui-items.json：requiredSources 只认 ' + SOURCES.join('/') + '（' + it.id + '）')
    }
    if (new Set(it.requiredSources).size !== it.requiredSources.length) throw new Error('audit-ui-items.json：requiredSources 有重复（' + it.id + '）')
    if (!Array.isArray(it.suites)) throw new Error('audit-ui-items.json：suites 必须是数组（' + it.id + '）')
    if (new Set(it.suites).size !== it.suites.length) throw new Error('audit-ui-items.json：suites 有重复（' + it.id + '）')
    for (const s of it.suites) {
      if (typeof s !== 'string' || s === '' || s.includes('\\')) throw new Error('audit-ui-items.json：suites 必须是正斜杠相对路径（' + it.id + '）')
      if (!existsSync(join(APP_ROOT, s))) throw new Error('audit-ui-items.json：套件不存在 ' + s)
    }
    const scanned = statusForCoverage(it.coverage) === 'scanned'
    if (scanned && it.reason != null) throw new Error('audit-ui-items.json：已扫描项的 reason 必须为 null（' + it.id + '）')
    if (!scanned && (typeof it.reason !== 'string' || it.reason.trim() === '')) throw new Error('audit-ui-items.json：未扫描项必须给非空 reason（' + it.id + '）')
    if (it.coverage === 'point' && it.suites.length === 0) throw new Error('audit-ui-items.json：point 项必须至少有一个点状断言套件（' + it.id + '）')
  }
  return raw.items
}

/** jest 的绝对路径 → 相对 APP_ROOT 的正斜杠路径（--json 里不许出现绝对路径）。 */
export function relSuitePath(abs, appRoot = APP_ROOT) {
  const norm = String(abs).split('\\').join('/')
  const root = appRoot.split('\\').join('/').replace(/\/+$/, '')
  if (norm.startsWith(root + '/')) return norm.slice(root.length + 1)
  const m = norm.match(/((?:utils|scripts|pages|components)\/.*)$/)
  return m ? m[1] : norm
}

/** 解析 jest --json 的 stdout；任何读不成的情形都返回 ok:false（fail-closed）。 */
export function parseJestJson(stdout, appRoot = APP_ROOT) {
  const text = String(stdout == null ? '' : stdout).trim()
  if (text === '') return { ok: false, reason: 'jest stdout 为空（零套件匹配或进程崩溃）' }
  let raw
  try {
    raw = JSON.parse(text)
  } catch {
    return { ok: false, reason: 'jest stdout 不是合法 JSON' }
  }
  if (!raw || !Array.isArray(raw.testResults)) return { ok: false, reason: 'jest JSON 缺 testResults' }
  if (raw.testResults.length === 0) return { ok: false, reason: 'jest 零套件匹配' }
  const suites = new Map()
  for (const tr of raw.testResults) {
    const results = Array.isArray(tr.assertionResults) ? tr.assertionResults : []
    suites.set(relSuitePath(tr.name, appRoot), {
      assertions: results.length,
      failed: results.filter((a) => a.status === 'failed').length,
      execError: Boolean(tr.testExecError),
    })
  }
  return { ok: true, suites }
}

/** 解析 audit-spacing --json 的 stdout；同样 fail-closed。 */
export function parseSpacingJson(stdout) {
  const text = String(stdout == null ? '' : stdout).trim()
  if (text === '') return { ok: false, reason: 'audit-spacing stdout 为空' }
  let raw
  try {
    raw = JSON.parse(text)
  } catch {
    return { ok: false, reason: 'audit-spacing stdout 不是合法 JSON' }
  }
  const isCount = (v) => Number.isInteger(v) && v >= 0
  if (!raw || !isCount(raw.scanned) || !Array.isArray(raw.warn) || !Array.isArray(raw.info) || !Array.isArray(raw.skip)) {
    return { ok: false, reason: 'audit-spacing JSON 缺 scanned/warn/info/skip' }
  }
  return { ok: true, scanned: raw.scanned, warn: raw.warn.length, info: raw.info.length, skip: raw.skip.length }
}

/**
 * 纯函数：判据表 + 两个来源的读数 → 报告。
 * 不读盘、不起子进程，便于单测直接喂构造数据。
 */
export function buildReport({ items, jest, spacing }) {
  const out = []
  for (const it of items) {
    const scanned = statusForCoverage(it.coverage) === 'scanned'
    const suites = it.suites.slice().sort()
    const evidence = { suites, assertions: null, failed: null }
    const readings = []
    let status = scanned ? 'scanned' : 'not-scanned'
    let reasonCode = scanned ? null : reasonCodeForCoverage(it.coverage)
    let reason = scanned ? null : it.reason
    let note = scanned ? noteForItem(it) : null

    const jestRequired = scanned && it.requiredSources.includes('jest')
    const spacingRequired = scanned && it.requiredSources.includes('spacing')
    const missingSuite = jest.ok && suites.some((s) => !jest.suites.has(s))
    const brokenSuite = jest.ok && suites.some((s) => jest.suites.get(s).execError)
    const jestBroken = jestRequired && (!jest.ok || missingSuite || brokenSuite)
    const spacingBroken = spacingRequired && !spacing.ok

    if (jestBroken || spacingBroken) {
      status = 'not-scanned'
      reasonCode = 'run-failed'
      reason = jestBroken
        ? 'jest 来源不可用：' + (jest.reason || '套件未跑成')
        : 'audit-spacing 来源不可用：' + (spacing.reason || '读失败')
      note = null
      evidence.assertions = null
      evidence.failed = null
      readings.length = 0
    } else if (scanned) {
      if (jest.ok && suites.length > 0) {
        evidence.assertions = suites.reduce((n, s) => n + jest.suites.get(s).assertions, 0)
        evidence.failed = suites.reduce((n, s) => n + jest.suites.get(s).failed, 0)
      }
      if (spacingRequired) {
        readings.push({ source: 'audit-spacing', scanned: spacing.scanned, warn: spacing.warn, info: spacing.info, skip: spacing.skip })
      }
    }

    out.push({ id: it.id, seq: it.seq, label: it.label, status, coverage: it.coverage, note, reasonCode, reason, evidence, readings })
  }
  out.sort((a, b) => a.seq - b.seq)
  return {
    schemaVersion: 1,
    items: out,
    counts: {
      scanned: out.filter((i) => i.status === 'scanned').length,
      notScanned: out.filter((i) => i.status === 'not-scanned').length,
    },
  }
}

/** 退出码：来源读失败优先（2），其次 --strict 下的断言失败（1）。 */
export function exitCodeFor(report, strict = false) {
  if (report.items.some((i) => i.reasonCode === 'run-failed')) return 2
  if (strict && report.items.some((i) => i.status === 'scanned' && (i.evidence.failed || 0) > 0)) return 1
  return 0
}

function dispWidth(s) {
  let w = 0
  for (const ch of String(s)) {
    w += /[\u1100-\u115F\u2E80-\uA4CF\uAC00-\uD7A3\uF900-\uFAFF\uFE30-\uFE6F\uFF00-\uFF60\uFFE0-\uFFE6]/.test(ch) ? 2 : 1
  }
  return w
}

function padRight(s, w) {
  return String(s) + ' '.repeat(Math.max(0, w - dispWidth(s)))
}

function renderTable(headers, rows) {
  const widths = headers.map((h, i) => Math.max(dispWidth(h), ...rows.map((r) => dispWidth(r[i]))))
  const fmt = (cells) => cells.map((c, i) => (i === cells.length - 1 ? String(c) : padRight(c, widths[i]))).join('  ')
  return [fmt(headers), ...rows.map(fmt)]
}

function readingText(item) {
  const parts = item.readings.map((r) => {
    const info = r.info > 0 ? ' / INFO ' + r.info : ''
    return r.scanned + ' 件 / WARN ' + r.warn + info + ' / SKIP ' + r.skip
  })
  const a = item.evidence.assertions
  const jestText = a === null ? '' : a + ' 断言' + (item.evidence.failed > 0 ? ' / ' + item.evidence.failed + ' 失败' : '')
  if (parts.length > 0 && jestText !== '') return parts.join('；') + '（+ ' + jestText + '）'
  return parts.join('；') || jestText || '—'
}

/** 终端两张表；耗时行只在这里出现，不进 --json。 */
export function renderTables(report, elapsedMs) {
  const scanned = report.items.filter((i) => i.status === 'scanned')
  const notScanned = report.items.filter((i) => i.status === 'not-scanned')
  const lines = []
  lines.push('已扫描（' + scanned.length + ' 项）')
  if (scanned.length > 0) {
    lines.push(...renderTable(['#', '审计项', '覆盖度', '读数'], scanned.map((i) => [String(i.seq), i.label, i.coverage, readingText(i)])))
  }
  lines.push('')
  lines.push('未扫描（' + notScanned.length + ' 项）')
  if (notScanned.length > 0) {
    lines.push(...renderTable(['#', '审计项', '原因码', '原因'], notScanned.map((i) => [String(i.seq), i.label, i.reasonCode, i.reason])))
  }
  lines.push('')
  lines.push('审计耗时 ' + (elapsedMs / 1000).toFixed(1) + 's（上限 ' + TIMEOUT_MS / 1000 + 's）')
  return lines.join('\n')
}

function runJest(items) {
  const suites = [...new Set(items.flatMap((i) => i.suites))].sort()
  if (suites.length === 0) return { ok: false, reason: 'items.json 未声明任何套件' }
  if (!existsSync(JEST_BIN)) return { ok: false, reason: '找不到 node_modules/jest/bin/jest.js（先 npm ci）' }
  const r = spawnSync(
    process.execPath,
    [JEST_BIN, '--config', JEST_CONFIG, '-i', '--silent', '--json', '--runTestsByPath', ...suites],
    { cwd: APP_ROOT, encoding: 'utf8', timeout: TIMEOUT_MS, maxBuffer: 64 * 1024 * 1024 },
  )
  if (r.error) return { ok: false, reason: 'jest 未能跑完：' + (r.error.code || r.error.message) }
  return parseJestJson(r.stdout)
}

function runSpacing() {
  const r = spawnSync(process.execPath, [SPACING_SCRIPT, '--json'], {
    cwd: APP_ROOT,
    encoding: 'utf8',
    timeout: TIMEOUT_MS,
    maxBuffer: 64 * 1024 * 1024,
  })
  if (r.error) return { ok: false, reason: 'audit-spacing 未能跑完：' + (r.error.code || r.error.message) }
  if (r.status !== 0) return { ok: false, reason: 'audit-spacing exit ' + r.status }
  return parseSpacingJson(r.stdout)
}

function main() {
  const argv = process.argv.slice(2)
  const unknown = argv.filter((a) => a !== '--json' && a !== '--strict')
  if (unknown.length > 0) {
    console.error('[audit-ui] 未知参数：' + unknown.join(' ') + '（只认 --json / --strict）')
    return 2
  }
  const strict = argv.includes('--strict')
  const jsonOut = argv.includes('--json')
  const startedAt = Date.now()

  let items
  try {
    items = loadItems()
  } catch (e) {
    console.error('[audit-ui] 判据表读不出（fail-closed）：' + e.message)
    return 2
  }

  const report = buildReport({ items, jest: runJest(items), spacing: runSpacing() })
  if (jsonOut) console.log(JSON.stringify(report, null, 2))
  else console.log(renderTables(report, Date.now() - startedAt))
  return exitCodeFor(report, strict)
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = main()
}
