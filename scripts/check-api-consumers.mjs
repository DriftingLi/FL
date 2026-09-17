#!/usr/bin/env node
/**
 * 契约消费面覆盖锁（ADR-0056 §11 / issue #1100）。
 *
 * 背景：前端请求层（frontend/src/api/**）打出的每个 method+path 都必须落在后端域声明表的端点集内
 * （backend/internal/apitypes/domains.go 的 Domains.Endpoints）—— 域声明表是「后端注解 → swagger →
 * 前端生成物」这条管道的事实源。此前没有任何守卫会因为「前端消费了一个未登记端点」变红：
 * inspection.ts 的 8 个巡检端点里 4 个在 swagger 但不在任何域声明表、另 4 个连 swagger 都没有，
 * 全靠人工清点（issue #1100 的证据段）。
 *
 * 用法（runner 面单点在 scripts/lib/guard.mjs，ADR-0056 §5 / #1094；本文件只有判定面）：
 *   node scripts/check-api-consumers.mjs --all            全量扫描（有未登记消费即退出 1）
 *   node scripts/check-api-consumers.mjs --diff [base]     只查相对 base 的新增行（base 默认 origin/master）
 *
 * 判定面：请求层调用（.get / .post / .put / .delete / .patch，含嵌套泛型 get<PagedResult<T>>）的
 * **第一个实参**路径字面量。模板字符串插值与字符串拼接的表达式统一归一为一个路径段占位 {}。
 * 归一后的模式必须命中域声明表里的某个端点：
 *   - 尾段逐段相等，声明参数名与消费参数名允许不同（{id} vs {course_id}），{} 是单段通配；
 *   - 允许消费侧少掉 baseURL 前缀（估值模块 client 的 baseURL 是 /api/valuation，模块里写 /dictionaries/...）；
 *   - 全段都是 {}、或首参是变量（路径由变量给出）→ **不可静态判定**，判违规（放过去就是假绿逃生口）。
 *
 * 放行面（有意不进判定集）：
 *   1. __tests__ / *.spec.* / *.test.* —— 测试按定义要断言路径字面量；
 *   2. frontend/src/api/generated/** —— 生成物，不含调用；
 *   3. ALLOWLIST 逐条登记的存量欠条（键 = "<仓库相对文件>::<METHOD> <归一模式>"，每条带理由）。
 *      欠条是**逐条销账**的：端点进了域声明表而条目还挂着 → 本守卫判红（补一个销一个），
 *      条目凭空挂着（文件里已经没有这个消费）→ 自测
 *      scripts/check-api-consumers.test.mjs 的「ALLOWLIST 不许有死条目」用例判红。
 */
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { isDirectRun, runGuardCli } from './lib/guard.mjs'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const BT = String.fromCharCode(96) // 反引号：本文件自身要解析模板字符串
const DOLLAR = String.fromCharCode(36)
const HOLE = String.fromCharCode(0) // 插值/表达式的占位符

/** 消费面：请求层模块自身（页面直连请求层由 check-api-seam 管）。 */
export const CONSUMER_DIR = 'frontend/src/api'

/** 端点事实源：apitypes 域声明表。 */
export const DECLARATION_FILE = 'backend/internal/apitypes/domains.go'

/** 扫描面后缀。 */
export const SCAN_EXTENSIONS = ['.ts']

/**
 * 存量欠条（键 = "<仓库相对文件>::<METHOD> <归一模式>" → 理由，一行一条）。
 * 现状 8 条：4 条巡检端点（等 #1097 补注解 + 登记域声明表）+ 4 条 createCrud 动态拼装。补一个销一个。
 *
 * 销账操作单（#1100 编排者裁决：4 个巡检端点由 INSP 车道补注解并登记进 apitypes.Domains）：
 *   INSP 合并后跑一次 node scripts/check-api-consumers.mjs --all —— 这 4 条会被判红，
 *   报「ALLOWLIST 欠条已可销账：<METHOD> <path> 已在域声明表（域 X）」，此时**删掉下面这 4 行**：
 *     'frontend/src/api/inspection.ts::GET /admin/points/ledger'
 *     'frontend/src/api/inspection.ts::GET /admin/inspection/deleted-after-accepted'
 *     'frontend/src/api/inspection.ts::GET /admin/recruit/views'
 *     'frontend/src/api/inspection.ts::GET /admin/recruit/requests'
 *   （自测 scripts/check-api-consumers.test.mjs 的「ALLOWLIST 逐条可解释且不许有死条目」用例
 *     同时会拦住删错/漏删：条目必须在文件里真的还有对应消费。）
 * 另 4 条（createCrud 动态拼装）的销账是另一票：把 valuation/admin.ts 的封装从「resource 形参拼路径」
 * 改成「显式资源 → 显式路径」的具名方法表。
 * @type {Record<string, string>}
 */
export const ALLOWLIST = {
  'frontend/src/api/inspection.ts::GET /admin/points/ledger':
    '巡检面端点：handler 是 internal/api/admin_inspection.go 的闭包，没有 swagger 注解（swagger 无此路径），' +
    '故暂时无法登记进 apitypes.Domains（TestDomainEndpointsExist 要求域内端点必须在 swagger paths 里）。' +
    '#1097 补注解后：登记进域声明表并删掉本条。',
  'frontend/src/api/inspection.ts::GET /admin/inspection/deleted-after-accepted':
    '同上（巡检计数端点，#1097 补注解后登记进域声明表并删掉本条）。',
  'frontend/src/api/inspection.ts::GET /admin/recruit/views':
    '同上（简历查看留痕巡检端点，#1097 补注解后登记进域声明表并删掉本条）。',
  'frontend/src/api/inspection.ts::GET /admin/recruit/requests':
    '同上（联系方式交换申请巡检端点，#1097 补注解后登记进域声明表并删掉本条）。',
  'frontend/src/api/valuation/admin.ts::GET {}':
    'createCrud 通用封装：路径由 resource 形参拼装（列表 = /dictionaries/<resource>），静态面只看到变量。' +
    '资源名在同文件的 adminResources 表逐个给出（brands/series/tonnages/...），全部是域内已登记端点，' +
    '但守卫读不出这个映射。销账要把封装改成「显式资源 → 显式路径」的具名方法（另开票）。',
  'frontend/src/api/valuation/admin.ts::POST {}':
    '同上（createCrud 的 create = /admin/<resource>）。',
  'frontend/src/api/valuation/admin.ts::PUT {}/{}':
    '同上（createCrud 的 update = /admin/<resource>/<id>）。注意：规格族 6 个资源' +
    '（tonnages/mast-types/mast-heights/battery-types/transmission-types/engine-types）的 update() 打的是' +
    '从未注册的幻影 PUT（见 backend/internal/valuation/handler/dictcrud_docs_lock_test.go 的 phantomAnnotations）；' +
    '前端目前无调用点（ValuationConfigManage.vue 只对 original-prices 用 CRUD）。',
  'frontend/src/api/valuation/admin.ts::DELETE {}/{}':
    '同上（createCrud 的 remove = /admin/<resource>/<id>）。'
}

/** 测试文件不进判定面（按定义要断言路径字面量）。 */
export function isTestFile(filePath) {
  const p = String(filePath).replace(/[\\]/g, '/')
  return p.includes('/__tests__/') || /\.(spec|test)\.[jt]s$/.test(p)
}

/** 路径是否在守卫面（请求层模块，且不是测试与生成物）。 */
export function isGuardedPath(filePath) {
  const p = String(filePath).replace(/[\\]/g, '/')
  if (isTestFile(p)) return false
  if (!p.startsWith(CONSUMER_DIR + '/')) return false
  if (p.startsWith(CONSUMER_DIR + '/generated/')) return false
  return true
}

/**
 * 解析域声明表源码 → Map("<METHOD> <归一路径>" → { method, path, domain })。
 * 只认 Domains 里的 Endpoint 字面量 {Method: "GET", Path: "/x"}；解析不出任何端点即抛错
 * （fail-closed：声明表格式变了必须红，不能静默放行所有消费）。
 */
export function parseDeclaredEndpoints(source) {
  const out = new Map()
  let domain = ''
  const lines = String(source).split('\n')
  for (const line of lines) {
    const dm = /^\s*Name:\s*"([^"]+)",\s*$/.exec(line)
    if (dm) domain = dm[1]
    const re = /\{Method:\s*"([A-Za-z]+)",\s*Path:\s*"([^"]+)"/g
    let m
    while ((m = re.exec(line)) !== null) {
      const method = m[1].toUpperCase()
      const endpoint = { method, path: m[2], domain }
      out.set(method + ' ' + normalizePattern(m[2]), endpoint)
    }
  }
  if (out.size === 0) {
    throw new Error(DECLARATION_FILE + ' 未解析出任何端点（域声明表格式变了？）')
  }
  return out
}

/** 路径参数归一：{id} / {course_id} → {}（参数名不是契约的一部分，位次是）。 */
export function normalizePattern(p) {
  return String(p).replace(/\{[^}]*\}/g, '{}')
}

/** 路径 → 段数组（丢掉空的首段）。 */
export function pathSegments(p) {
  const parts = normalizePattern(p).split('/')
  if (parts[0] === '') parts.shift()
  return parts
}

const declaredCache = new Map()

/** 按仓库根读取并缓存域声明表（scanSource 无 root 参数，事实源按仓库根定位）。 */
export function declaredEndpoints(root = ROOT) {
  if (!declaredCache.has(root)) {
    const source = readFileSync(path.resolve(root, DECLARATION_FILE), 'utf8')
    declaredCache.set(root, parseDeclaredEndpoints(source))
  }
  return declaredCache.get(root)
}

/**
 * 消费模式是否命中声明端点：返回命中的端点（含所属域），否则 null。
 * 尾段逐段相等；{} 是单段通配；全部段都是 {} → 视为不可静态判定（返回 null）。
 */
export function matchDeclared(method, pattern, declared) {
  let segments = pathSegments(pattern)
  // 前导 {} = 未知 baseURL 前缀（featured.ts 的 ${baseURL}/... 形态）：它不属于契约路径，跳过再比。
  while (segments.length > 0 && segments[0] === '{}') segments = segments.slice(1)
  if (segments.length === 0) return null
  if (segments.every((s) => s === '{}')) return null
  for (const endpoint of declared.values()) {
    if (endpoint.method !== method) continue
    const declSegments = pathSegments(endpoint.path)
    if (segments.length > declSegments.length) continue
    let ok = true
    for (let i = 0; i < segments.length; i++) {
      const consumed = segments[segments.length - 1 - i]
      const decl = declSegments[declSegments.length - 1 - i]
      if (consumed === '{}' || decl === '{}' || consumed === decl) continue
      ok = false
      break
    }
    if (ok) return endpoint
  }
  return null
}

/** 带泛型实参的方法调用前缀（含嵌套 get<PagedResult<T>>）。 */
const METHOD_RE = /\.(get|post|put|delete|patch)(?![A-Za-z0-9_$])/g

/** 跳过平衡的尖括号组（泛型实参）。返回跳过后的下标。 */
function skipGenerics(source, i) {
  const n = source.length
  while (i < n && /\s/.test(source[i])) i++
  if (source[i] !== '<') return i
  let depth = 0
  while (i < n) {
    if (source[i] === '<') depth++
    else if (source[i] === '>') {
      depth--
      if (depth === 0) return i + 1
    }
    i++
  }
  return i
}

/**
 * 解析第一个实参为路径模式：字面量 + 拼接（'a' + x + 'b'）与模板插值都归一为 {}。
 * 返回 { pattern, unresolved }：unresolved = 首参不是字面量（路径完全由变量给出）。
 */
export function parseFirstArg(source, i) {
  let out = ''
  let sawLiteral = false
  const n = source.length
  const skipSpace = () => {
    while (i < n && /\s/.test(source[i])) i++
  }
  skipSpace()
  for (;;) {
    skipSpace()
    const ch = source[i]
    if (ch === undefined) break
    if (ch === BT || ch === "'" || ch === '"') {
      const quote = ch
      i++
      while (i < n && source[i] !== quote) {
        if (source[i] === '\\') {
          out += source[i + 1]
          i += 2
          continue
        }
        if (quote === BT && source[i] === DOLLAR && source[i + 1] === '{') {
          let depth = 1
          i += 2
          while (i < n && depth > 0) {
            if (source[i] === '{') depth++
            else if (source[i] === '}') depth--
            i++
          }
          out += HOLE
          continue
        }
        out += source[i]
        i++
      }
      i++
      sawLiteral = true
    } else {
      let depth = 0
      const start = i
      while (i < n) {
        const c = source[i]
        if (c === '(' || c === '[' || c === '{') depth++
        else if (c === ')' || c === ']' || c === '}') {
          if (depth === 0) break
          depth--
        } else if (depth === 0 && (c === ',' || c === '+')) break
        i++
      }
      if (!source.slice(start, i).trim()) break
      out += HOLE
    }
    skipSpace()
    if (source[i] === '+') {
      i++
      continue
    }
    break
  }
  return { pattern: out, unresolved: !sawLiteral }
}

/** 归一一个原始模式：占位 → {}、折叠重复斜杠、去掉 query。 */
export function normalizeConsumed(raw) {
  return String(raw).split(HOLE).join('{}').replace(/\/{2,}/g, '/').split('?')[0]
}

/**
 * 扫一份源码，返回违规列表（1-based 行号 + method + 归一模式 + 理由）。
 * 路径不在守卫面即整体放行（测试 / 生成物 / 非请求层目录）。
 */
export function scanSource(source, file) {
  if (!isGuardedPath(file)) return []
  const declared = declaredEndpoints()
  const text = String(source)
  const lines = text.split('\n')
  const violations = []
  let m
  METHOD_RE.lastIndex = 0
  while ((m = METHOD_RE.exec(text)) !== null) {
    let i = skipGenerics(text, m.index + m[0].length)
    while (i < text.length && /\s/.test(text[i])) i++
    if (text[i] !== '(') continue
    const arg = parseFirstArg(text, i + 1)
    if (!arg.pattern) continue
    const method = m[1].toUpperCase()
    const pattern = normalizeConsumed(arg.pattern)
    if (pattern.startsWith('http')) continue
    const line = text.slice(0, m.index).split('\n').length
    const site = { file, line, method, pattern, text: (lines[line - 1] || '').trim() }
    const key = file + '::' + method + ' ' + pattern
    const allowReason = ALLOWLIST[key]
    const matched = matchDeclared(method, pattern, declared)
    if (allowReason !== undefined) {
      if (matched) {
        violations.push({
          ...site,
          reason:
            'ALLOWLIST 欠条已可销账：' + matched.method + ' ' + matched.path + ' 已在域声明表（域 ' +
            matched.domain + '）—— 删掉脚本 ALLOWLIST 里的条目'
        })
      }
      continue
    }
    if (matched) continue
    violations.push({
      ...site,
      reason: arg.unresolved || pathSegments(pattern).every((s) => s === '{}')
        ? '路径由变量/表达式拼装，无法静态判定（拒绝假绿逃生口）'
        : '未落在 ' + DECLARATION_FILE + ' 的域端点集内'
    })
  }
  return violations
}

/** ALLOWLIST 条目的展示行（--all 报告用）。 */
export function allowlistLines() {
  return Object.keys(ALLOWLIST)
    .sort()
    .map((key) => '  - ' + key + '：' + ALLOWLIST[key])
}

/**
 * 本守卫的声明：判定面 + 报告措辞。runner（argv / 走查 / --diff / allowlist / 退出码）
 * 在 scripts/lib/guard.mjs。文件级 allowlist 空置：欠条是**逐端点**登记的（见 ALLOWLIST），
 * 整体放行一个文件会把该文件后续新增的未登记消费一起放掉。
 */
export const GUARD_SPEC = {
  name: 'check-api-consumers',
  usage: '用法: node scripts/check-api-consumers.mjs --all | --diff [base]',
  cli: { noArgs: 'all', helpFlag: true, scanDirArg: false, usageOnUnknown: true, usageStream: 'stderr' },
  all: {
    scanDir: CONSUMER_DIR,
    extensions: SCAN_EXTENSIONS,
    skipNodeModules: true,
    tolerateWalkErrors: false,
    stream: 'stdout',
    header: (ctx) => [
      '===== 契约消费面覆盖锁（请求层消费的端点必须已登记进域声明表）=====',
      '消费面: ' + ctx.scanDirRel,
      '端点事实源: ' + DECLARATION_FILE + '（' + declaredEndpoints().size + ' 个端点）',
      '---'
    ],
    ok: (ctx) =>
      '无违规。' + ctx.checked + ' 个请求层文件消费的端点全部在域声明表内；存量欠条 ' +
      Object.keys(ALLOWLIST).length + ' 条（见 ALLOWLIST，补一个销一个）。',
    violation: (v) => v.file + ':' + v.line + ': ' + v.method + ' ' + v.pattern + ' —— ' + v.reason + '\n      ' + v.text,
    footer: (ctx) => [
      '---',
      '共 ' + ctx.count + ' 处。端点要进生成面：后端注解 → cd backend && make swagger → ' +
        'go run ./cmd/gen-apitypes，并在 backend/internal/apitypes/domains.go 登记该端点。',
      '存量欠条（' + Object.keys(ALLOWLIST).length + ' 条，逐条登记理由）：',
      ...allowlistLines()
    ]
  },
  diff: {
    pathspec: ['*.ts'],
    defaultBase: 'origin/master',
    stream: 'stderr',
    empty: (base) => '[check-api-consumers] 相对 ' + base + ' 无 .ts 新增行，跳过。',
    ok: () => '[check-api-consumers] 新增行消费的端点均已在域声明表内，通过。',
    header: () => ['===== 新增行消费了未登记的端点（补注解 + 登记域声明表，或走 ALLOWLIST 欠条）====='],
    violation: (v) => v.file + ':' + v.line + ': ' + v.method + ' ' + v.pattern + ' —— ' + v.reason,
    footer: (ctx) => ['---', '共 ' + ctx.count + ' 处。见 ADR-0056 §11 与 docs/agents/checks.md。']
  },
  isGuardedPath,
  scanSource,
  allowlist: {}
}

if (isDirectRun(import.meta.url)) runGuardCli(GUARD_SPEC)
