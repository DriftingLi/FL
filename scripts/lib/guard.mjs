#!/usr/bin/env node
/**
 * 守卫 runner 单点（ADR-0056 §5 / issue #1094）。
 *
 * 背景：三个守卫（`check-api-seam` / `check-el-controls` / `check-async-section`）此前各写一遍
 * runner 骨架（argv 解析 / `--all` 走查 / `--diff` 拉 diff / allowlist / 退出码 / isDirectRun）。
 * 第三份抄的时候把 `lib/added-lines.mjs` 的 interface 用错 —— `parseAddedLines(diffText)` 被写成
 * `parseAddedLines(base, '*.vue')`，`--diff` 恒拿到空 Map、恒 0 违规；CI 只跑 `--all`、自检只测
 * 判定函数，假绿两侧无人接住。本模块把 runner 面收成一份，守卫只留判定面与措辞。
 *
 * 守卫模块的契约（新增守卫只需写这些，见三个 `check-*.mjs` 的 `GUARD_SPEC`）：
 *
 *   export const GUARD_SPEC = {
 *     name,            // 诊断前缀，如 'check-api-seam'
 *     usage,           // argv 不合法时打印的用法
 *     cli,             // 入口形态（无参数 / --help / --all 是否收目录 / 未知参数，见 parseArgs）
 *     all,             // --all 的走查面与报告措辞
 *     diff,            // --diff 的 pathspec、默认 base 与报告措辞
 *     isGuardedPath,   // (仓库相对路径) => 是否进判定面
 *     scanSource,      // (源码, 仓库相对路径) => 违规项（至少含 line）
 *     allowlist        // { 仓库相对路径: 理由 } 逐条登记的例外（整体放行，两种模式一致）
 *   }
 *
 * `--diff` 一律 **fail-closed**：base 解析不了 / `git diff` 失败 / 新增文件读不出来 —— 任一情形都
 * 报错并非零退出，绝不落到「✓ 通过」。只有「diff 取成功且新增行面为空」才是合法的绿。
 */
import { execFileSync } from 'node:child_process'
import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

// 新增行解析的单点实现（ADR-0053 §10）：含 core.quotepath 转义解码与「浅克隆无 merge base →
// 退双点」的兜底。守卫不再各自 import 它，runner 是唯一消费面（gate-predicates.test.mjs 盯着）。
import { gitDiff, parseAddedLines } from './added-lines.mjs'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..')

/** 仓库相对 POSIX 路径（`--all` 走查给绝对路径，`--diff` 给的就是相对路径）。 */
function relTo(root, file) {
  const p = path.isAbsolute(file) ? path.relative(root, file) : file
  return p.split(path.sep).join('/')
}

function errorText(e) {
  return e && e.message ? e.message : String(e)
}

function isAllowlisted(spec, file) {
  return Boolean(spec.allowlist) && Object.prototype.hasOwnProperty.call(spec.allowlist, file)
}

/**
 * argv → 运行计划。三个守卫原有的 CLI 面逐字保留（差异写在各自的 `cli` 里）：
 *   `--all`           全量走查（api-seam / el-controls 可再跟一个目录）
 *   `--diff [base]`   只查相对 base 的新增行；base 缺省 origin/master
 */
export function parseArgs(spec, argv) {
  const cli = spec.cli ?? {}
  const usage = { mode: 'usage', stream: cli.usageStream ?? 'stdout', code: 0 }
  const mode = argv[0]
  if (mode === undefined) {
    return (cli.noArgs ?? 'all') === 'all' ? { mode: 'all', scanDir: null } : usage
  }
  if (mode === '--help' && cli.helpFlag) return usage
  if (mode === '--all') return { mode: 'all', scanDir: cli.scanDirArg && argv[1] ? argv[1] : null }
  if (mode === '--diff') {
    return { mode: 'diff', base: argv[1] || spec.diff.defaultBase || 'origin/master' }
  }
  return { mode: 'invalid', arg: mode }
}

/**
 * 递归收集待扫文件（绝对路径；readdir 顺序即报告顺序，与三个守卫原先的走查一致）。
 * `extensions` / `skipNodeModules` / `tolerateWalkErrors` 保留各守卫原有的走查语义。
 */
export function walkFiles(dir, { extensions, skipNodeModules = false, tolerateWalkErrors = false }) {
  const out = []
  let entries
  try {
    entries = readdirSync(dir, { withFileTypes: true })
  } catch (e) {
    if (tolerateWalkErrors) return out
    throw e
  }
  for (const entry of entries) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      if (skipNodeModules && entry.name === 'node_modules') continue
      out.push(...walkFiles(full, { extensions, skipNodeModules, tolerateWalkErrors }))
      continue
    }
    if (extensions.some((ext) => entry.name.endsWith(ext))) out.push(full)
  }
  return out
}

/** 取「新增行」集合：逐 pathspec 取 diff 文本（经 added-lines 的 gitDiff），再合并解析结果。 */
function addedLinesOf(base, pathspecs, io) {
  const added = new Map()
  for (const pathspec of pathspecs) {
    for (const [file, lines] of parseAddedLines(io.readDiff(base, pathspec))) {
      const set = added.get(file) ?? new Set()
      for (const line of lines) set.add(line)
      added.set(file, set)
    }
  }
  return added
}

function runAll(spec, plan, io) {
  const cfg = spec.all
  const scanDir = plan.scanDir ? path.resolve(plan.scanDir) : path.resolve(io.root, cfg.scanDir)
  const files = walkFiles(scanDir, cfg)
  const ctx = { root: io.root, scanDir, scanDirRel: relTo(io.root, scanDir), files, checked: 0, count: 0 }
  for (const line of cfg.header ? cfg.header(ctx) : []) emit(io, cfg.stream, line)
  const violations = []
  for (const abs of files) {
    const file = relTo(io.root, abs)
    if (!spec.isGuardedPath(file)) continue
    ctx.checked++
    if (isAllowlisted(spec, file)) continue // 逐条登记的例外整体放行
    for (const v of spec.scanSource(io.readSource(abs), file)) violations.push({ ...v, file })
  }
  ctx.count = violations.length
  if (violations.length === 0) {
    io.out(cfg.ok(ctx))
    return 0
  }
  for (const v of violations) emit(io, cfg.stream, cfg.violation(v, ctx))
  for (const line of cfg.footer ? cfg.footer(ctx) : []) emit(io, cfg.stream, line)
  return 1
}

function runDiff(spec, plan, io) {
  const cfg = spec.diff
  const base = plan.base
  // 1) base 必须可解析：拿不到就红（「静默跳过」正是 #1094 的病根）
  let resolvable = false
  try {
    resolvable = io.resolveBase(base)
  } catch {
    resolvable = false
  }
  if (!resolvable) {
    io.err('[' + spec.name + '] 无法解析 base ref: ' + base + '（CI 上请先 git fetch）')
    return 2
  }
  // 2) diff 取文本：命令失败即红（三点失败退双点由 added-lines 负责）
  let added
  try {
    added = addedLinesOf(base, cfg.pathspec, io)
  } catch (e) {
    io.err('[' + spec.name + '] ' + errorText(e))
    return 2
  }
  // 3) 「diff 成功且新增行面为空」才是合法的绿
  if (added.size === 0) {
    io.out(cfg.empty(base))
    return 0
  }
  const violations = []
  for (const [file, lines] of added) {
    if (!spec.isGuardedPath(file)) continue
    if (isAllowlisted(spec, file)) continue
    let source
    try {
      source = io.readSource(file)
    } catch (e) {
      if (e && e.code === 'ENOENT') continue // 删除的文件（新增行面不含它）
      io.err('[' + spec.name + '] 读取新增文件失败: ' + file + '（' + errorText(e) + '）')
      return 2
    }
    for (const v of spec.scanSource(source, file)) {
      if (lines.has(v.line)) violations.push({ ...v, file })
    }
  }
  if (violations.length === 0) {
    io.out(cfg.ok(base))
    return 0
  }
  const ctx = { root: io.root, base, added, count: violations.length }
  for (const line of cfg.header ? cfg.header(ctx) : []) io.err(line)
  for (const v of violations) io.err(cfg.violation(v, ctx))
  for (const line of cfg.footer ? cfg.footer(ctx) : []) io.err(line)
  return 1
}

function emit(io, stream, line) {
  if (stream === 'stderr') io.err(line)
  else io.out(line)
}

/**
 * 跑一个守卫，返回退出码：0 绿 / 1 有违规 / 2 判据坏了（fail-closed）。
 * `options` 只给自检注入用（合成 diff、假源码、捕获输出）；CLI 直接跑用默认值。
 */
export function runGuard(spec, options = {}) {
  const root = options.root ?? ROOT
  const io = {
    root,
    out: options.stdout ?? ((line) => process.stdout.write(line + '\n')),
    err: options.stderr ?? ((line) => process.stderr.write(line + '\n')),
    readSource: options.readSource ?? ((file) => readFileSync(path.resolve(root, file), 'utf8')),
    resolveBase:
      options.resolveBase ??
      ((base) => {
        try {
          execFileSync('git', ['rev-parse', '--verify', base], { cwd: root, stdio: 'ignore' })
          return true
        } catch {
          return false
        }
      }),
    readDiff: options.readDiff ?? ((base, pathspec) => gitDiff(base, pathspec, root))
  }
  const plan = parseArgs(spec, options.argv ?? process.argv.slice(2))
  if (plan.mode === 'usage') {
    emit(io, plan.stream, spec.usage)
    return plan.code
  }
  if (plan.mode === 'invalid') {
    if (spec.cli && spec.cli.usageOnUnknown) emit(io, (spec.cli.usageStream ?? 'stderr'), spec.usage)
    else io.err('未知参数: ' + plan.arg)
    return 2
  }
  return plan.mode === 'all' ? runAll(spec, plan, io) : runDiff(spec, plan, io)
}

/**
 * CLI 入口：退出码写进 `process.exitCode`（不用 `process.exit` —— 它会截断还没刷出的 stdout）。
 */
export function runGuardCli(spec, argv = process.argv.slice(2)) {
  try {
    process.exitCode = runGuard(spec, { argv })
  } catch (e) {
    console.error(e)
    process.exitCode = 2
  }
}

/** 仅当守卫脚本被直接 `node scripts/check-x.mjs` 调用时才跑（被 import 时无副作用）。 */
export function isDirectRun(metaUrl) {
  return Boolean(process.argv[1]) && pathToFileURL(process.argv[1]).href === metaUrl
}
