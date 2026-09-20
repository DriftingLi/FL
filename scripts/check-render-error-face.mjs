#!/usr/bin/env node
/**
 * 端点错误面守卫（ADR-0060 决策 1 / 第十三波票 1b）。
 *
 * 背景：`Endpoint` 的 `Render` 闭包过去「全权负责写响应」，签名上带着 `err`，于是「记得查本域
 * 错误表」只是一条注释约束（实测 179 处手写闭包里 ~98 处自己读 err）。票1b 把 `err` 从
 * `RenderFunc` 的签名上拿掉：错误面归骨架无条件渲染，Render 只写成功面。本守卫把剩下的那一半
 * 也变成可核验的事实——**Render 闭包里不得再出现任何错误信封调用**：
 *   - `response.ServerError` / `response.NotFound` / `response.BadRequest` /
 *     `response.Unauthorized` / `response.Forbidden`（五个错误信封，`pkg/response` 的错误面）；
 *   - `renderStatus(`（域表状态码的单一咽喉，端点侧手写它等于绕开 ErrStatus）；
 *   - `renderError(`（旧「Render 内自查域表」的形态，票1b 后由骨架代做）。
 * 成功面（`response.Success` / `SuccessWithMsg` / `Created`）不在禁列——Render 现在唯一职责就是它。
 *
 * 用法（runner 面单点在 `scripts/lib/guard.mjs`，ADR-0056 §5 / #1094；本文件只有判定面）：
 *   node scripts/check-render-error-face.mjs --all  [目录]   全量扫描（默认 backend；有违规则退出 1）
 *   node scripts/check-render-error-face.mjs --diff [base]   只查相对 base 的新增行（base 默认 origin/master）
 *     —— 例外文件（ALLOWLIST）在增量门里也是**逐行**判定：只有基线即违规的行号放行（口径见 runner）。
 *
 * 判定面（刻意窄，每条都有原因）：
 *   - 只扫 `backend/internal/api` 下的非测试 `.go`：`Endpoint` 骨架的管辖面就是这一层；
 *     `endpoint.go` 自身除外（它是错误面的**唯一合法作者**，`renderError`/`renderStatus` 写在它里面）。
 *   - 只判 `Render:` 字段上那个函数字面量的**词法体内**：用括号深度进出闭包，深度计数前先剥掉
 *     注释与字符串字面量——Swagger 注解里的 `example({"a":1})` 有大括号，不剥就会数歪闭包边界。
 *   - 命中的是 `Render:` 之外的代码（Parse/Invoke、raw handler、service 层）一律不管：
 *     错误状态码在那两处出现是各自层的职责，本守卫只保证「成功面里没有错误形状」。
 * 白名单（ALLOWLIST）初始为空：票1b 收口后 `--all` 实测 0 存量；确有例外逐条登记并写明理由。
 *
 * 一致性：下面的错误信封名单是 `pkg/response` 的**手抄一份**，它与 response.go 的真实导出函数由
 * `scripts/check-render-error-face.test.mjs` 的「错误信封名单与 response 包互等」用例锁住——
 * 新增错误信封而不更新本表，自检即红（同 `check-catalog-sort` 那条「判据不许改回双份」）。
 */
import { isDirectRun, runGuardCli } from './lib/guard.mjs'

/** 守卫面：api 层（端点骨架管辖）的非测试 .go。 */
export const GUARDED_DIR_PREFIX = 'backend/internal/api/'

/** 骨架自身：错误面的合法唯一作者，不进判定面。 */
export const SKELETON_FILE = 'backend/internal/api/endpoint.go'

/** 扫描面后缀（Go 侧只有 .go）。 */
export const SCAN_EXTENSIONS = ['.go']

/** `pkg/response` 的错误信封（写响应的五个错误出口）。 */
export const ERROR_ENVELOPE_FNS = ['ServerError', 'BadRequest', 'Unauthorized', 'Forbidden', 'NotFound']

/** 成功面：不在禁列（Render 的唯一职责就是写成功面）。 */
export const SUCCESS_ENVELOPE_FNS = ['Success', 'SuccessWithMsg', 'Created']

/**
 * 禁列：错误信封调用 + 两个「自己渲染错误」的骨架内部口。
 * @type {{re: RegExp, why: string}[]}
 */
export const FORBIDDEN = [
  ...ERROR_ENVELOPE_FNS.map((fn) => ({
    re: new RegExp('(?:response\\.)?' + '\\b' + fn + '\\s*\\('),
    why: '错误信封 response.' + fn + ' 归骨架渲染，写进 ErrStatus 域表条目'
  })),
  { re: /\brenderStatus\s*\(/, why: 'renderStatus 是域表的单一咽喉，端点侧不得手写' },
  { re: /\.renderError\s*\(/, why: 'Render 不再自查域表（票1b）：把哨兵挂进 ErrStatus' }
]

/**
 * 逐条登记的有意例外（路径 → 理由）。规则绝对执行，本表保持为空。
 * @type {Record<string, string>}
 */
export const ALLOWLIST = {}

/** Go 测试文件不进判定面（骨架行为用例按定义要在 Render 里写错误形状测它）。 */
export function isTestFile(filePath) {
  return String(filePath).replace(/\\/g, '/').endsWith('_test.go')
}

/**
 * 路径是否在守卫面（api 层的非测试 .go，骨架自身除外）。
 * **ALLOWLIST 不在这里判**（#1123）：豁免由 runner 承载（`--all` 整体放行 / `--diff` 只放行基线
 * 违规行号）——判定面先吞掉例外文件的话 `scanSource` 恒返回空，行号级放行会静默失效。
 */
export function isGuardedPath(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  if (!p.endsWith('.go') || isTestFile(p)) return false
  if (!p.startsWith(GUARDED_DIR_PREFIX)) return false
  return p !== SKELETON_FILE
}

/**
 * 剥掉注释与字符串字面量（保留换行，行号不变），返回可安全数括号 / 匹配调用的代码文本。
 * 逐字符走一遍是因为 `"// 不是注释"` 与 `// 这是 "不是字符串"` 都要判对；块注释跨行由 state 承载。
 */
export function stripCommentsAndStrings(line, state) {
  const out = []
  let i = 0
  while (i < line.length) {
    if (state.inBlock) {
      const end = line.indexOf('*/', i)
      if (end < 0) {
        i = line.length
        continue
      }
      state.inBlock = false
      i = end + 2
      continue
    }
    const two = line.slice(i, i + 2)
    if (two === '//') {
      i = line.length
      continue
    }
    if (two === '/*') {
      state.inBlock = true
      i += 2
      continue
    }
    const ch = line[i]
    if (ch === '"' || ch === '`' || ch === "'") {
      const quote = ch
      i++
      while (i < line.length) {
        if (quote === '`') {
          if (line[i] === '`') {
            i++
            break
          }
          i++
          continue
        }
        if (line[i] === '\\') {
          i += 2
          continue
        }
        if (line[i] === quote) {
          i++
          break
        }
        // 反引号外的字符串跨行在 Go 里非法（原始串字面量才允许），按行截断交给下一行
        if (line[i] === '\n') break
        i++
      }
      out.push(quote === '"' ? '""' : "''")
      continue
    }
    out.push(ch)
    i++
  }
  return out.join('')
}

/** 一行是否开启 `Render:` 函数字面量（字段值写成函数名/构造器的不在词法射程内，见文件头）。 */
const RENDER_OPEN = /(?:^|[{,])\s*Render\s*:\s*func\s*\(/

/**
 * 扫一份源码，返回违规列表（1-based 行号 + 命中原因 + 原文）。
 * **路径不在守卫面即整体放行**（判定收敛在这里，调用方不必自己记得过滤）。
 *
 * 状态机：`Render:` 开一处闭包 → 按剥壳后的括号深度前进 → 深度回 0 即出闭包。
 * 嵌套的 `Endpoint{...}`（闭包里再构造端点，实测 0 处）会让内层 Render 复用外层区间——
 * 判定为「在任一 Render 体内」即禁，方向保守，不会漏报。
 */
export function scanSource(source, file) {
  if (!isGuardedPath(file)) return []
  const violations = []
  const state = { inBlock: false }
  const lines = String(source).split('\n')
  let depth = 0
  let inRender = false
  let renderLine = 0
  for (let i = 0; i < lines.length; i++) {
    const raw = lines[i]
    const code = stripCommentsAndStrings(raw, state)
    if (!inRender) {
      if (!RENDER_OPEN.test(code)) continue
      inRender = true
      depth = 0
      renderLine = i + 1
    }
    if (!code.trim()) {
      // 空行（或被剥空的纯注释行）：不影响深度，也不判定
      continue
    }
    for (const f of FORBIDDEN) {
      if (f.re.test(code)) {
        violations.push({
          file,
          line: i + 1,
          why: f.why,
          renderLine,
          text: raw.trim()
        })
      }
    }
    for (const ch of code) {
      if (ch === '{') depth++
      else if (ch === '}') depth--
    }
    if (depth <= 0) {
      inRender = false
      depth = 0
    }
  }
  return violations
}

/**
 * 从 `pkg/response` 源码里取真实导出的信封函数名（自检用它把两份名单锁成互等）。
 * 只认 `func Name(c *gin.Context, …)` 形态：写响应的出口都是这个签名。
 */
export function exportedEnvelopeFns(source) {
  const out = []
  const re = /^func\s+([A-Z]\w*)\s*\(\s*c\s+\*gin\.Context/gm
  let m
  while ((m = re.exec(String(source))) !== null) out.push(m[1])
  return out
}

/**
 * 本守卫的声明：判定面 + 报告措辞。runner（argv / 走查 / --diff / allowlist / 退出码）
 * 在 scripts/lib/guard.mjs —— 新增守卫只需实现 scanSource 并声明这一份。
 */
export const GUARD_SPEC = {
  name: 'check-render-error-face',
  usage: '用法: node scripts/check-render-error-face.mjs --all [目录] | --diff [base]',
  cli: { noArgs: 'all', helpFlag: true, scanDirArg: true, usageOnUnknown: true, usageStream: 'stderr' },
  all: {
    scanDir: 'backend',
    extensions: SCAN_EXTENSIONS,
    skipNodeModules: true,
    tolerateWalkErrors: true,
    stream: 'stdout',
    header: (ctx) => [
      '===== 端点错误面守卫：全量扫描（Render 闭包内不得渲染错误）=====',
      '扫描目录: ' + ctx.scanDirRel,
      '守卫面: ' + GUARDED_DIR_PREFIX + ' 的非测试 .go（骨架 ' + SKELETON_FILE + ' 是错误面的唯一作者，除外）',
      '禁列: response.' + ERROR_ENVELOPE_FNS.join(' / response.') + ' | renderStatus( | .renderError(',
      '---'
    ],
    ok: (ctx) => '无违规。' + ctx.checked + ' 个 api 文件的 Render 闭包内均未出现错误信封调用。',
    violation: (v) =>
      v.file + ':' + v.line + ': Render 闭包（起于 :' + v.renderLine + '）内' + v.why + '  ' + v.text,
    footer: (ctx) => [
      '---',
      '共 ' + ctx.count + ' 处。错误面归骨架：把状态码/固定文案写进本端点的 ErrStatus',
      '  （单码用 errStatusAll/errStatusAllMsg，哨兵用 errStatusEntry 条目），Render 只写成功面（ADR-0060 决策 1）。'
    ]
  },
  diff: {
    pathspec: ['*.go'],
    defaultBase: 'origin/master',
    stream: 'stderr',
    empty: (base) => '[check-render-error-face] 相对 ' + base + ' 无 .go 新增行，跳过。',
    ok: () => '[check-render-error-face] 新增行未在 Render 闭包内渲染错误，通过。',
    header: () => ['===== Render 闭包内又出现错误信封（错误面归骨架，见 ErrStatus）====='],
    violation: (v) =>
      v.file + ':' + v.line + ': Render 闭包（起于 :' + v.renderLine + '）内' + v.why + '  ' + v.text,
    footer: (ctx) => ['---', '共 ' + ctx.count + ' 处。见 ADR-0060 决策 1 与 docs/agents/checks.md。']
  },
  isGuardedPath,
  scanSource,
  allowlist: ALLOWLIST
}

if (isDirectRun(import.meta.url)) runGuardCli(GUARD_SPEC)
