#!/usr/bin/env node
/**
 * api seam 守卫（ADR-0053 §7 / spec #1049）。
 *
 * 背景：前端有一层请求层（frontend/src/api/*.ts，薄薄一层 typed 转发）。但页面层曾出现
 * 「绕过请求层直接打端点」的写法——6 个页面 18 个调用点，每个都是手拼 URL + `any` 类型，
 * 同一个端点在很多文件里各打一遍。这次把它们收回 api 模块后，本脚本把「页面/组件不得
 * 直接引用请求层」变成 CI 可核验的事实。
 *
 * 用法（runner 面单点在 `scripts/lib/guard.mjs`，ADR-0056 §5 / #1094；本文件只有判定面）：
 *   node scripts/check-api-seam.mjs --all  [目录]   全量扫描（默认 frontend/src；有违规则退出 1）
 *   node scripts/check-api-seam.mjs --diff [base]   只查相对 base 的新增行（base 默认 origin/master）
 *     —— 例外文件（ALLOWLIST）在增量门里也是**逐行**判定：只有基线即违规的行号放行，新增行上的
 *        违规照报；基线取不到即非零退出（行号级口径见 scripts/lib/guard.mjs）。
 *
 * 判定面：只扫**页面与业务组件**（`/pages/` 与 `/components/` 下的 .vue / .ts）的
 * import / export-from / 动态 import 语句里的模块说明符。
 *   —— 只认「引用了请求层」这一条可精确判定的规则；不扫 URL 字符串字面量（那会误伤
 *      文档、注释与合法的外链常量）。
 *
 * 放行面（有意不进守卫集）：
 *   1. frontend/src/api/** —— 请求层自身与它的 api 模块；
 *   2. __tests__ / *.spec.* / *.test.* —— 测试要对请求层做 mock，按定义就得 import 它；
 *   3. 其余目录（utils / composables / stores …）—— 本守卫的适用面是页面与业务组件。
 * 白名单（ALLOWLIST）初始为空：确有例外时逐条登记并写明理由，不要放宽规则本身。
 */
import { isDirectRun, runGuardCli } from './lib/guard.mjs'

/** 请求层模块（页面不得直接引用）。含 `@/api/request` 与相对写法 `../api/request`。 */
export const GUARDED_MODULES = ['api/request', 'api/client']

/** 守卫面：路径含这些片段才算判定对象。 */
export const GUARDED_PATH_SEGMENTS = ['/pages/', '/components/']

/** 扫描面后缀。 */
export const SCAN_EXTENSIONS = ['.vue', '.ts']

/**
 * 逐条登记的有意例外（路径 → 理由）。收口本身把 18 处调用点全部收回 api 模块，
 * 没有为它们预留豁免；本表只有「跑守卫时实际发现、且经判定不属本次规则射程」的一条。
 *
 * 口径（#1123）：`--all` 整体放行；`--diff` 只放行**基线即违规的行号** —— 往例外文件里新增
 * 一条直接引用、或把违规挪到别的行号，增量门照报。
 * @type {Record<string, string>}
 */
export const ALLOWLIST = {
  'frontend/src/components/recruit/OnlineResumePdf.vue':
    '只用 getValidAccessToken()（取访问令牌的只读辅助）给原生 fetch 的 PDF 端点加鉴权头，' +
    '不经请求层发请求；待该组件改用 api 层的 blob 方法后删除本条。'
}

/** 测试文件不进判定面（按定义要 mock 请求层）。 */
export function isTestFile(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  return p.includes('/__tests__/') || /\.(spec|test)\.[jt]s$/.test(p)
}

/**
 * 路径是否在守卫面（页面 / 业务组件，且不是测试）。
 * **ALLOWLIST 不在这里判**（#1123）：豁免是 runner 的事（`--all` 整体放行 / `--diff` 只放行基线
 * 违规行号）。判定面若先把例外文件吞掉，`scanSource` 对它恒返回空，行号级放行就无从谈起。
 */
export function isGuardedPath(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  if (isTestFile(p)) return false
  return GUARDED_PATH_SEGMENTS.some((seg) => p.includes(seg))
}

/** 模块说明符是否指向请求层（支持 `@/api/request`、`../api/request`、`./api/client` 写法）。 */
export function guardedSpecifier(spec) {
  const s = String(spec).replace(/\\/g, '/')
  return GUARDED_MODULES.find((m) => s === '@/' + m || s === m || s.endsWith('/' + m)) || null
}

const SPECIFIER_RE = /(?:^|\s)(?:import|export)\b[^'"`]*?['"]([^'"]+)['"]|import\s*\(\s*['"]([^'"]+)['"]\s*\)|require\s*\(\s*['"]([^'"]+)['"]\s*\)/g

/**
 * 扫一份源码，返回违规列表（1-based 行号 + 命中的模块说明符 + 原文）。
 * 只认 import / export-from / 动态 import / require 的说明符。
 *
 * **路径不在守卫面即整体放行**（api 层自身、测试、白名单、非页面/组件目录）——
 * 判定收敛在这里，调用方不必自己记得先过滤（少一个「忘了过滤就假红」的面）。
 */
export function scanSource(source, file) {
  if (!isGuardedPath(file)) return []
  const violations = []
  const lines = String(source).split('\n')
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    // 廉价预筛：三种语句形态都覆盖（export-from 行里没有 import 关键字）
    if (!/import|require|export/.test(line)) continue
    SPECIFIER_RE.lastIndex = 0
    let m
    while ((m = SPECIFIER_RE.exec(line)) !== null) {
      const spec = m[1] ?? m[2] ?? m[3]
      if (!spec) continue
      const guarded = guardedSpecifier(spec)
      if (guarded) {
        violations.push({ file, line: i + 1, spec, module: guarded, text: line.trim() })
      }
    }
  }
  return violations
}

/**
 * 本守卫的声明：判定面 + 报告措辞。runner（argv / 走查 / --diff / allowlist / 退出码）
 * 在 scripts/lib/guard.mjs —— 新增守卫只需实现 scanSource 并声明这一份。
 */
export const GUARD_SPEC = {
  name: 'check-api-seam',
  usage: '用法: node scripts/check-api-seam.mjs --all [目录] | --diff [base]',
  cli: { noArgs: 'all', helpFlag: false, scanDirArg: true, usageOnUnknown: true, usageStream: 'stderr' },
  all: {
    scanDir: 'frontend/src',
    extensions: SCAN_EXTENSIONS,
    skipNodeModules: true,
    tolerateWalkErrors: true,
    stream: 'stdout',
    header: (ctx) => [
      '===== api seam 守卫：全量扫描（页面与业务组件不得直接引用请求层）=====',
      '扫描目录: ' + ctx.scanDirRel,
      '守卫面: ' + GUARDED_PATH_SEGMENTS.join(' / ') + '（跳过测试与白名单）',
      '请求层: ' + GUARDED_MODULES.map((m) => '@/' + m).join(' / '),
      '---'
    ],
    ok: (ctx) => '无违规。' + ctx.checked + ' 个文件均未直接引用请求层。',
    violation: (v) => v.file + ':' + v.line + ': ' + v.spec + '  ' + v.text,
    footer: (ctx) => [
      '---',
      '共 ' + ctx.count + ' 处。请在 frontend/src/api/ 下补具名方法，页面只调它：',
      '  —— 响应类型取 `@/api/generated/*`（后端注解 → swagger → go run ./cmd/gen-apitypes），不手写。'
    ]
  },
  diff: {
    pathspec: ['*.vue', '*.ts'],
    defaultBase: 'origin/master',
    stream: 'stderr',
    empty: (base) => '[check-api-seam] 相对 ' + base + ' 无 .vue/.ts 新增行，跳过。',
    ok: () => '[check-api-seam] 新增行未直接引用请求层，通过。',
    header: () => ['===== 新增行直接引用了请求层（页面与业务组件一律走 api/ 具名方法）====='],
    violation: (v) => v.file + ':' + v.line + ': ' + v.spec + '  ' + v.text,
    footer: (ctx) => ['---', '共 ' + ctx.count + ' 处。见 ADR-0053 §7 与 docs/agents/ui-conventions.md。']
  },
  isGuardedPath,
  scanSource,
  allowlist: ALLOWLIST
}

if (isDirectRun(import.meta.url)) runGuardCli(GUARD_SPEC)
