#!/usr/bin/env node
/**
 * UI 形态意图登记制守卫（ADR-0083 决策 6 / 票 #1982）。
 *
 * 取代 `check-el-controls` 的**黑名单制**：那边维护「已收敛的 10 类控件」清单，没进清单的裸
 * `el-*` 一律放行 —— 清单外新增一个 `el-xxx` 不会有人发现。这边反过来：**登记表就是白名单**，
 * 模板里出现登记表之外的 `el-*` 即红。
 *
 * 数据源只有一份：`docs/agents/ui-conventions.md` 的「意图登记表」小节（派生逻辑在
 * `lib/ui-intent-registry.mjs`）。这里**不落一份手维护的 JSON** —— 派生清单的唯一形态就是
 * 「从文档现算」，落盘就会有人手改，那就是本票要收掉的漂移源。要看派生结果用
 * `--emit-registry <file>`（只写不读，仅供排查）。
 *
 * 两条规则：
 *   ① **意图**：根 `<template>` 里的 `el-*` 必须是登记过的那几种之一 ——
 *      原生登记（`el-tabs` / `el-drawer` / `el-table` …）、按域分批（`el-input` / `el-select` …）、
 *      或某个已登记父件的子件（`el-option` ⊂ `el-select`）。封装层 `components/ui/**` 不进面。
 *   ② **档位**：容器外观的**纯字面量**（`border-radius` / `box-shadow` 里既不含 `var(`、也不在
 *      `0` / `50%` / `none` 之列）即红 —— 圆角与阴影只有档位表那几档，几何值必须走 token。
 *
 * 用法（runner 面在 `lib/guard.mjs`，本文件只有判定面）：
 *   node scripts/check-ui-intent.mjs --all  [目录]   全量扫描（默认 frontend/src）
 *   node scripts/check-ui-intent.mjs --diff [base]   只查相对 base 的新增行
 *   node scripts/check-ui-intent.mjs --emit-registry <file>   把派生结果写成 JSON（排查用）
 */
import { writeFileSync } from 'node:fs'
import { isDirectRun, runGuardCli } from './lib/guard.mjs'
import { visibleTemplateLines } from './lib/vue-template.mjs'
import { classify, loadRegistryFromFile } from './lib/ui-intent-registry.mjs'

/** 封装层内部本来就是 Element Plus 控件（ADR-0035）：规则①不进这一面，规则②照查。 */
export const WRAPPER_SEGMENT = '/components/ui/'

export function isWrapperPath(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  // 补前导斜杠：调用方可能给仓库相对路径（`frontend/src/components/…`），也可能给 src 相对路径
  // （`components/ui/…`），两种都得命中 `'/components/ui/'`。
  return ('/' + p).includes(WRAPPER_SEGMENT)
}

let cachedRegistry = null
/** 派生结果按进程缓存：一个 `--all` 会扫 170+ 个文件，重复解析文档没有意义。 */
export function getRegistry() {
  if (!cachedRegistry) cachedRegistry = loadRegistryFromFile()
  return cachedRegistry
}

/** 测试用：换一份登记面（也顺手清掉缓存，避免用例之间互相污染）。 */
export function setRegistry(next) {
  cachedRegistry = next
}

/** 规则①：模板里未登记的 `el-*`。 */
export function findIntentViolations(source, registry, filePath) {
  if (isWrapperPath(filePath)) return []
  const reg = registry ?? getRegistry();
  const violations = []
  for (const { line, text, raw } of visibleTemplateLines(source)) {
    const re = /<(el-[a-z0-9-]+)\b/g
    let m
    while ((m = re.exec(text))) {
      const tag = m[1]
      const verdict = classify(tag, reg)
      if (verdict.kind !== 'unknown') continue
      violations.push({ rule: 'intent', line, tag, text: raw.trim(), orphanParent: verdict.orphan ? verdict.parent : null })
    }
  }
  return violations
}

/** 纯字面量里允许出现的值：零档 / 真圆 / 无阴影。其余都必须走 token。 */
export const ALLOWED_LITERALS = new Set(['0', 'none', '50%'])

/** 常见越档值的最近档提示（只用于报告措辞，不参与判定）。 */
const NEAREST = {
  '2px': 'var(--radius-md)（8px，或零档 0）',
  '4px': 'var(--radius-md)',
  '6px': 'var(--radius-md)',
  '8px': 'var(--radius-md)',
  '10px': 'var(--radius-md)',
  '12px': 'var(--radius-lg)',
  '16px': 'var(--radius-lg)'
}

function normalizeValue(value) {
  return String(value)
    .replace(/!important/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}

/**
 * 规则②：容器外观的纯字面量。
 * 含 `var(` 即放行 —— 那是 token 引用（`0 var(--radius-full) var(--radius-full) 0`、
 * `0 0 0 1px var(--color-border) inset` 这类复合值都在这一格里）。
 */
export function findStyleViolations(source) {
  const violations = []
  const lines = String(source).split('\n')
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    const re = /(border-radius|box-shadow)\s*:\s*([^;}"']+)/g
    let m
    while ((m = re.exec(line))) {
      const prop = m[1]
      const rawValue = m[2].trim()
      if (rawValue.includes('var(')) continue
      const value = normalizeValue(rawValue)
      if (ALLOWED_LITERALS.has(value)) continue
      violations.push({ rule: 'tier', line: i + 1, prop, value, text: line.trim(), hint: NEAREST[value] ?? null })
    }
  }
  return violations
}

/** 源码 + 路径 → 违规项（两条规则合流）。 */
export function scanSource(source, filePath) {
  return [...findIntentViolations(source, null, filePath), ...findStyleViolations(source)]
}

function describe(v) {
  if (v.rule === 'intent') {
    const why = v.orphanParent
      ? '子件 <' + v.tag + '> 的父件 <' + v.orphanParent + '> 没有登记'
      : '登记表里没有 <' + v.tag + '>（原生登记 / 按域分批 / 子件 三处都没有）'
    return '[' + '①意图' + '] <' + v.tag + '>  ' + why
  }
  const near = v.hint ? '，最近档：' + v.hint : ''
  return '[' + '②档位' + '] ' + v.prop + ': ' + v.value + '  —— 容器外观只走档位 token' + near
}

export const GUARD_SPEC = {
  name: 'check-ui-intent',
  usage: '用法: node scripts/check-ui-intent.mjs --all [目录] | --diff [base] | --emit-registry <file>',
  cli: { noArgs: 'all', helpFlag: true, scanDirArg: true, usageOnUnknown: true, usageStream: 'stderr' },
  all: {
    scanDir: 'frontend/src',
    extensions: ['.vue'],
    skipNodeModules: false,
    tolerateWalkErrors: false,
    stream: 'stdout',
    minChecked: 150,
    header: (ctx) => [
      '===== UI 意图登记制守卫（登记表 = 白名单；容器外观只走档位 token）=====',
      '扫描目录: ' + ctx.scanDirRel,
      '---'
    ],
    ok: (ctx) => '无违规。' + ctx.checked + ' 个单文件组件：模板里的 el-* 全部在登记表内，' +
      '容器外观字面量全部走档位 token（`0` / `50%` / `none` 是档内值）。',
    violation: (v) => v.file + ':' + v.line + ': ' + describe(v) + '  ' + v.text,
    footer: (ctx) => [
      '---',
      '共 ' + ctx.count + ' 处。两件事二选一：改走封装层，或先在 docs/agents/ui-conventions.md 登记。',
      '「按域分批」的欠账（el-input / el-select 等）登记在案，不报红 —— 那是有意保留的迁移面。'
    ]
  },
  diff: {
    pathspec: ['*.vue'],
    defaultBase: 'origin/master',
    stream: 'stderr',
    empty: (base) => '[check-ui-intent] 相对 ' + base + ' 无 .vue 新增行，跳过。',
    ok: () => '[check-ui-intent] 新增行未裸用未登记控件、未写越档字面量，通过。',
    header: () => ['===== 新增行出现未登记的 el-* 或越档的容器外观字面量 ====='],
    violation: (v) => v.file + ':' + v.line + ': ' + describe(v) + '  ' + v.text,
    footer: (ctx) => ['---', '共 ' + ctx.count + ' 处。见 docs/agents/ui-conventions.md 的意图登记表与容器档位表。']
  },
  isGuardedPath: (filePath) => String(filePath).endsWith('.vue'),
  scanSource,
  allowlist: null
}

/** 排查面：把派生结果写成 JSON（只写不读 —— 没有任何代码会把这个文件当判据）。 */
function emitRegistry(file) {
  const reg = getRegistry()
  const json = {
    source: 'docs/agents/ui-conventions.md「意图登记表」派生',
    native: [...reg.native].sort(),
    pendingByDomain: [...reg.pending].sort(),
    children: Object.fromEntries([...reg.children].sort((a, b) => a[0].localeCompare(b[0]))),
    counts: reg.counts
  }
  writeFileSync(file, JSON.stringify(json, null, 2) + '\n', 'utf8')
  console.log('registry → ' + file)
  return 0
}

if (isDirectRun(import.meta.url)) {
  const argv = process.argv.slice(2)
  if (argv[0] === '--emit-registry') {
    if (!argv[1]) {
      console.error(GUARD_SPEC.usage)
      process.exit(1)
    }
    process.exit(emitRegistry(argv[1]))
  }
  runGuardCli(GUARD_SPEC)
}
