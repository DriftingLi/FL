#!/usr/bin/env node
/**
 * 契约测试「读取层归一」守卫（#1177 / 移动端 ADR-0019 票 B）
 *
 * ## 为什么需要它
 *
 * ③ 门跑在 ubuntu，而一批**接线守护**是「读源文本 + 多行锚点匹配」的。Windows 上
 * `core.autocrlf=true` 会把源文件检出成 **CRLF** ⇒ 多行锚点里 `.` **不匹配 `\r`**、
 * `\n` 又必须紧跟其后 ⇒ 在 CRLF 行上**匹配 0 次**：变异根本没生效、断言却照跑 ⇒
 * 守护**静默失效**（以为在守，其实没守）。CI 默认关 autocrlf，故这一类**在 CI 里结构性不可见**。
 *
 * 修法是**读取层归一**（唯一真源 `utils/utsHarness.js` 的 `normalizeEol` / `readText`）——
 * 归一之后**扩展名钉没钉完全不重要**，对任何原因导致的 CRLF 一律免疫。本守卫是它的**结构半**：
 * 防止新写的契约测试又去裸读源码。
 *
 * ## 判据（两条，取或）
 *
 * **规则一 —— 调用点**：在 `utils/*.test.js` 里遍历每个 `readFileSync(` 调用点，**同时**满足
 * 下列两条即违规：
 *   ① 该取值**未**经归一（同一表达式上不接 `.replace(/\r\n/g, '\n')`）；
 *   ② 其读取目标**静态可解析**为**仓内已跟踪文件**
 *      （含 `path.join(ROOT|__dirname, …)`，含经该文件内读者助手参数绑定的字面量）。
 *
 * **为什么把 ② 当必要条件（本守卫的取舍，照实说）**：归一化的必要性只对**仓内源码**成立。
 * 读临时目录 / 运行期产物（`mkdtempSync` 下的 `.txt`、`.ci-verify/*.log`）与 EOL 无关 ⇒
 * 解不出目标的调用点**天然放行**，因此本守卫**不需要 allowlist**（ADR-0019 §②⑤ 的「免 allowlist」）。
 * 代价是**有假阴性**：目标是动态变量的裸读解析不出来就放行。这是有意偏向「不误报」的一侧 ——
 * 门若对现有合法代码判红，它就会被绕过。
 *
 * **规则二 —— 读者助手体（#1178 追加裁定，2026-09-19）**：`const NAME = (params) => …` 形态的
 * 读者助手，**其体内出现 `readFileSync` 而未归一 ⇒ 违规**，**不要求**该调用点的目标能静态解析。
 * 违规点报在**助手声明行**（不是体内那行）——`--diff` 按新增行号筛，改助手就是把声明行改掉。
 *
 * **为什么规则二不需要「目标可解析」这个前置**：票 B 收口后实测枚举了「有读者助手且未归一」的
 * 全部 26 个文件，**每一个助手的实参都指向仓内源码**（23 个 `fs.readFileSync(path.join(ROOT, rel), 'utf8')`
 * 形态、3 个 `path.join(ROOT, p)` 形态、2 个裸变量 `p` 形态——后两类的调用点传的也都是
 * `path.join(__dirname, …)` 的仓内源码），**没有一个只读临时产物** ⇒ 这条规则**结构性不会引入误报**。
 * 反过来说，规则一那 26 个文件的缺陷面正是靠规则二兜住的：助手体把「目标解析」和「是否归一」
 * 两件事解耦了，而解耦后仍然只有仓内源码这一种被读物。
 *
 * **仍未覆盖（写实，勿当全量）**：`function NAME(params) { return fs.readFileSync(…) }`
 * 形态的读者**不在规则二内**（规则二只认 `const NAME = (…) => …`），它们仍靠规则一在**调用点**
 * 兜；读的是循环变量（`fs.readFileSync(f, 'utf8')`）这类解不出的裸读也仍只能靠人。这是
 * 「不误报优先」的同一取舍，不是遗漏。
 *
 * ## 用法
 *
 *   node scripts/check-contract-read.mjs --all             全量扫（票 C 的收敛判据）
 *   node scripts/check-contract-read.mjs --diff [base]     只看相对 base 的新增行（base 默认 origin/master）
 *
 * ## 退出码
 *
 *   0 = 通过；1 = 有违规；2 = 用法错 / **fail-closed**（base 不可解析、git diff 失败、新增文件读不出来）
 *
 * ## 挂载与证据边界
 *
 * 挂在 `.github/workflows/ci.yml` 的 `mobile-test` job（`--diff`）。按本仓定义它属**接线守护**
 * ⇒ **不构成 ③ 门证据**（ADR-0008 的裁定）。
 */
import { readFileSync, readdirSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { dirname, basename, join, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

/** 本脚本 = <repo>/training-app/叉车维修培训学员端跨端应用/scripts/check-contract-read.mjs */
const MOBILE_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const REPO_ROOT = resolve(MOBILE_ROOT, '..', '..')
const UTILS_DIR = join(MOBILE_ROOT, 'utils')
/** 仓库相对前缀（git 输出用 `/`，且相对仓库根） */
const MOBILE_REL_PREFIX = 'training-app/叉车维修培训学员端跨端应用'
const UTILS_REL_PREFIX = MOBILE_REL_PREFIX + '/utils'

/** 已跟踪文件清单与 basename 索引（`main` 之前由 `trackedSet()` / `buildBasenameIndex` 填好；
 *  放在这里而不是文件末尾：`scanSource` → `resolveTargets` 会读它们，声明在使用之前可避免
 *  「先被 import 调用、后初始化」的时序歧义）。 */
let tracked = new Set()
/** basename → 已跟踪路径列表（兜底解析用；同名多命中视为不可判定 ⇒ 放行） */
let byBasename = new Map()

/** 归一判据：`\r\n` → `\n`（与 utils/utsHarness.js 的 normalizeEol 同一语义） */
const EOL_NORMALIZE_RE = /\.replace\(\s*\/\\r\\n\/g\s*,\s*['"]\\n['"]\s*\)/

/** 读者助手形态：`const NAME = (params) => …`（规则二只认这一种；`function NAME()` 不在内） */
const READER_HELPER_RE = /(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*\(([^)]*)\)\s*=>/g

/** 形如 `fs.readFileSync` / `require('fs').readFileSync` 的取值（裸 `readFileSync` 也算 —— 本仓
 *  有 `const read = (abs) => readFileSync(abs, 'utf8')` 这种先解构 `fs` 的写法）。
 *  ⚠️ 这里**不能**写「前缀不是 `.`」：那样会把最常见的 `fs.readFileSync` 一起排除掉 ——
 *  实测踩过（规则二在改造前应当报 26 个助手，写成排除 `.` 后报 0 个，`--all` 假绿）。 */
const RAW_READ_PREFIX = String.raw`(?:fs|require\(['"]fs['"]\))\.readFileSync|\breadFileSync`

/** 助手体里出现裸读 —— 按实参形状分四条（每条都要求实参是 `…, 'utf8'` 收尾）：
 *  ① 单个标识符 / 点号链   ② 字符串字面量   ③ 括号配平的调用（`path.join(ROOT, p)`）
 *  ④ 含 `${…}` 的模板串。
 *  刻意**不写**「贪婪到最后一个 `,'utf8'`」的兜底正则：多行写法下它会跨调用点吞掉中间代码，
 *  把后面本已合规的读也判红 —— 那是**误报**，比漏报更坏。 */
const RAW_READ_IN_BODY_RES = [
  new RegExp(`${RAW_READ_PREFIX}\\s*\\(\\s*[A-Za-z_$][\\w$.]*\\s*,\\s*['"]utf8['"]\\s*\\)`),
  new RegExp(`${RAW_READ_PREFIX}\\s*\\(\\s*['"][^'\\n]*['"][^)\\n]*,\\s*['"]utf8['"]\\s*\\)`),
  new RegExp(`${RAW_READ_PREFIX}\\s*\\(\\s*(?:[A-Za-z_$][\\w$]*\\.)*[A-Za-z_$][\\w$]*\\s*\\((?:[^()]|\\([^()]*\\))*\\)\\s*,\\s*['"]utf8['"]\\s*\\)`),
  new RegExp(`${RAW_READ_PREFIX}\\s*\\(\\s*\`[^\`\\n]*\`\\s*,\\s*['"]utf8['"]\\s*\\)`),
]

/** 指明「这段文本里出现了仓内源码文件的字面量路径」 */
const SOURCE_EXT_RE = /\.(?:uts|uvue|ps1|mjs|json|md|ts|tsx|vue|js|go|sh|css)\b/i

/** 去注释（块注释 + 行注释）——与 scripts/classify-guards.mjs 同一做法：
 *  注释里出现 `readFileSync` 不算调用点（本仓多处注释在讲这个坑）。
 *  ⚠️ 必须**保留换行**：后续按 `\n` 切行算行号，若把整行连同换行一起删掉，
 *  行号会整体错位（`--diff` 的新增行比对就会失准）。 */
export function stripComments(src) {
  return String(src)
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/\/\/[^\n]*/g, '')
}

/**
 * 把注释**内容换成等长空白**（换行原样保留）——用于**需要行号准确**的扫描。
 *
 * 为什么不能直接用 `stripComments`：它把整段注释删掉 ⇒ 注释之后的行**整体前移**。
 * 实测血账：第 675 行的注入违规被判成第 **457** 行，而 `--diff` 是拿「新增行号集合」
 * 去比 `v.line` ⇒ **永远命不中**、打印「通过」并 exit 0 —— **门形同虚设**。
 * 而 `--all` 因为不看行号照样报红 ⇒ **只测 `--all` 发现不了这个 bug**。
 */
export function blankComments(src) {
  const blank = (m) => m.replace(/[^\n]/g, ' ')
  return String(src)
    .replace(/\/\*[\s\S]*?\*\//g, blank)
    .replace(/\/\/[^\n]*/g, blank)
}

/** 从 `(` 起做括号配对（跳过字符串字面量），返回匹配的 `)` 索引；失败返回 -1 */
export function matchParen(src, openIdx) {
  let depth = 0
  for (let i = openIdx; i < src.length; i++) {
    const ch = src[i]
    if (ch === '(') depth++
    else if (ch === ')') {
      depth--
      if (depth === 0) return i
    } else if (ch === "'" || ch === '"' || ch === '`') {
      const quote = ch
      i++
      while (i < src.length) {
        if (src[i] === '\\') { i += 2; continue }
        if (src[i] === quote) break
        i++
      }
    }
  }
  return -1
}

/** 收集字面量常量：`const X = 'literal'` 与 `const X = path.join(...)`。
 *  `path.join` 里的根标记（`ROOT` / `__dirname` / `REPO`）**立刻**换成哨兵，
 *  因为后续会做常量替换——那时原文里的 `__dirname` 已被替换掉、看不见了。 */
export function collectConsts(src) {
  const out = new Map()
  for (const m of src.matchAll(/(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*'([^'\n]*)'/g)) {
    out.set(m[1], m[2])
  }
  for (const m of src.matchAll(/(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*([^;\n]*path\.join\([^;]*?\))\s*;/gs)) {
    out.set(m[1], withRootSentinels(m[2]))
  }
  return out
}

/** 把路径根表达式换成哨兵：`__dirname`→`@DIR@`、`ROOT`→`@ROOT@`、`REPO`→`@REPO@`
 *  ⚠️ 三处都用**函数式**替换：替换串里的 `@ROOT@` 本身含 `$`，用字符串形式会被
 *  `String.replace` 当成 `$&`/`$1` 之类模式解析而**吞掉**（实测踩过：哨兵变成 `@ROOT`）。 */
function withRootSentinels(text) {
  return String(text)
    .replace(/__dirname/g, () => '@DIR@')
    .replace(/\bROOT\b/g, () => '@ROOT@')
    .replace(/\bREPO\b/g, () => '@REPO@')
}

/** 调用点若落在 `const NAME = (params) => BODY` 里，返回该助手信息（含 BODY 自身是否已归一）。
 *  用途一（规则一）：把助手的**实参字面量**按位置绑到形参，从而解析 `read(p)` 里的 `p`。
 *  用途二（规则二）：**助手体本身**就是判据面 —— 体内裸读即违规，与实参能否解析无关。
 *
 *  实现要点：BODY 用**括号配对**界定（不是正则懒匹配 `;`）——否则 `const read = (p) => fs.readFileSync(path.join(ROOT, p), 'utf8');`
 *  这种「函数体里有分号前的 `)`」的写法会截断在第一个 `)` 上。 */
export function enclosingHelper(src, idx) {
  const re = new RegExp(READER_HELPER_RE.source, 'g')
  let m
  let last = null
  while ((m = re.exec(src)) !== null) if (m.index < idx) last = m
  if (!last) return null
  const bodyStart = last.index + last[0].length
  if (idx < bodyStart) return null
  const bodyEnd = helperBodyEnd(src, bodyStart)
  if (bodyEnd < 0 || idx > bodyEnd) return null
  const body = src.slice(bodyStart, bodyEnd + 1)
  return {
    name: last[1],
    params: last[2].split(',').map((s) => s.trim()).filter(Boolean),
    /** 声明行行号（1-based）——规则二的违规点报在这里 */
    line: src.slice(0, last.index).split('\n').length,
    normalized: EOL_NORMALIZE_RE.test(body),
    rawRead: RAW_READ_IN_BODY_RES.some((r) => r.test(body)),
  }
}

/** 助手体结束位置：以 `{` 开头则按花括号配对，否则到第一个分号 */
function helperBodyEnd(src, bodyStart) {
  let i = bodyStart
  while (i < src.length && /\s/.test(src[i])) i++
  if (src[i] === '{') {
    let depth = 0
    for (; i < src.length; i++) {
      if (src[i] === '{') depth++
      else if (src[i] === '}') { depth--; if (depth === 0) return i }
    }
    return -1
  }
  const semi = src.indexOf(';', i)
  return semi < 0 ? -1 : semi
}

/** 去 `..` / `.` / 重复斜杠，得到仓库相对路径 */
export function normalizeRel(p) {
  const segs = []
  for (const s of String(p).split('/')) {
    if (s === '.' || s === '') continue
    if (s === '..') segs.pop()
    else segs.push(s)
  }
  return segs.join('/')
}

/** 把实参文本展开为仓库相对路径候选（常量 + 参数绑定，迭代到不动点） */
export function resolveTargets(argText, consts, paramBindings) {
  let expanded = withRootSentinels(argText)
  for (let i = 0; i < 6; i++) {
    const before = expanded
    for (const [k, v] of consts) {
      // ⚠️ 必须用**函数式**替换：字符串替换里的 `$` 有特殊含义（`$1`/`$&`…），
      //    而常量值里恰恰含 `$`（哨兵 `@DIR@` 经 `$1` 前缀拼接时会被吞掉）——实测踩过。
      expanded = expanded.replace(new RegExp(`(^|[^\\w$.])${k}\\b`, 'g'), (_, p1) => p1 + v)
    }
    for (const [k, v] of paramBindings) {
      expanded = expanded.replace(new RegExp(`(^|[^\\w$.])${k}\\b`, 'g'), (_, p1) => p1 + v)
    }
    if (expanded === before) break
  }
  const cands = new Set()
  // 只认「带**根哨兵**」的 join：`path.join(ROOT|__dirname|REPO, …)` 才是「仓内路径」的
  // 确证。`path.join(ctx.tmp, 'x.txt')` / `path.join(logDir, f)` 这类**临时目录 / 运行期产物**
  // 的 join 不在此列 —— 这正是「读产物与 EOL 无关 ⇒ 天然放行」的落点（免 allowlist）。
  //
  // 段提取：**必须**先按逗号切、再去引号 —— 常量展开过后段会变成**未加引号**的普通文本
  // （`path.join(@ROOT@, scripts/lib/x.ps1)`），只认引号会把整条 join 丢掉（实测踩过）。
  for (const m of expanded.matchAll(/path\.join\(([^)]*)\)/gs)) {
    const argsText = m[1]
    const base = /@DIR@/.test(argsText) ? UTILS_REL_PREFIX : /@ROOT@/.test(argsText) ? MOBILE_REL_PREFIX : null
    if (!base) continue
    const parts = argsText
      .split(',')
      .map((s) => s.trim().replace(/^['"`]|['"`]$/g, ''))
      .filter((s) => s.length > 0 && !/^@(?:DIR|ROOT|REPO)@$/.test(s))
    if (parts.length === 0) continue
    cands.add(normalizeRel([base, ...parts].join('/')))
  }
  // 独立字面量路径（含 `/`）：只有看起来像仓内相对路径的才收（避免把 `a/b.txt` 这类
  // 一次性夹具名当成仓内文件）。
  for (const m of expanded.matchAll(/'([^'\n]*\/[^'\n]*)'/g)) {
    const p = normalizeRel(m[1])
    if (!/^(?:[a-z][\w.-]*\/){1,}/i.test(p)) continue
    cands.add(p)
  }
  // 兜底：形如 `'request.uts'` 的**纯文件名**字面量。用途是那些**嵌套写法**
  // （`require('path').join(__dirname, '..', 'api', 'request.uts')`）—— 上面的 `path.join`
  // 提取在这种写法下匹配不到（`require('path').join` 不是 `path.join`）。
  // 判据仍是「该名字能**唯一**命中一个**已跟踪**文件」，故临时夹具名不会被误判成仓内源码。
  for (const m of expanded.matchAll(/'([^'\n\/]+\.(?:uts|uvue|ps1|mjs|json|md|ts|tsx|vue|go|sh|css))'/gi)) {
    const b = byBasename.get(m[1])
    if (b && b.length === 1) cands.add(b[0])
  }
  return [...cands]
}

/**
 * 扫一个测试文件的源码，返回违规点。
 * @param {string} src 源码
 * @param {(rel:string)=>boolean} isTracked 该仓库相对路径是否为已跟踪文件
 */
export function scanSource(src, isTracked) {
  // 用 blankComments：**行号必须与原文件一致**（--diff 要按新增行号筛）。
  const code = blankComments(src)
  const consts = collectConsts(code)
  const violations = []
  /** 已按规则一报过的**助手声明行** —— 规则二按它去重，一个助手只报一次 */
  const reportedHelpers = new Set()
  const re = /readFileSync\s*\(/g
  let m
  while ((m = re.exec(code)) !== null) {
    const openIdx = code.indexOf('(', m.index)
    const closeIdx = matchParen(code, openIdx)
    if (closeIdx < 0) continue
    const lineNo = code.slice(0, m.index).split('\n').length
    // ① 链式归一
    if (EOL_NORMALIZE_RE.test(code.slice(closeIdx + 1, closeIdx + 90))) continue
    // ② 目标是否为仓内已跟踪文件
    const argText = code.slice(openIdx + 1, closeIdx)
    const helper = enclosingHelper(code, m.index)
    const paramBindings = []
    if (helper) {
      const callRe = new RegExp(`\\b${helper.name}\\s*\\(([^)]*)\\)`, 'g')
      let c
      while ((c = callRe.exec(code)) !== null) {
        const args = [...c[1].matchAll(/'([^'\n]*)'/g)].map((x) => x[1])
        helper.params.forEach((p, i) => { if (args[i]) paramBindings.push([p, args[i]]) })
      }
    }
    const targets = resolveTargets(argText, consts, paramBindings)
    const target = targets.find((t) => isTracked(t))
    if (!target) continue
    // ③ 只对「读源码文本」的面报红：目标是源码类文件，或实参里出现了源码扩展名。
    //    （读 .json 但只做 JSON.parse 的也走读者，因为 parse 对 CRLF 不敏感——但归一无害，
    //      且统一走读者更简单；此处不额外放宽，避免规则出现第二套判据。）
    if (helper && !helper.normalized) {
      reportedHelpers.add(`${helper.name}@${helper.line}`)
    }
    violations.push({
      line: lineNo,
      target,
      text: (code.split('\n')[lineNo - 1] ?? '').trim(),
    })
  }
  // 规则二：读者助手体出现裸读而未归一 ⇒ 违规（**不要求**调用点的目标能静态解析）。
  // 报在**助手声明行**：--diff 按新增行号筛，而改助手就是把那一行改掉。
  for (const h of helpersIn(code)) {
    if (h.normalized || !h.rawRead) continue
    if (reportedHelpers.has(`${h.name}@${h.line}`)) continue
    violations.push({
      line: h.line,
      target: `${h.name}() 助手体内裸读（未归一）`,
      text: (code.split('\n')[h.line - 1] ?? '').trim(),
    })
  }
  return violations
}

/** 枚举源码里全部 `const NAME = (…) => …` 读者助手（体里有裸读的才算，避免把工具函数当成读者） */
export function helpersIn(src) {
  const re = new RegExp(READER_HELPER_RE.source, 'g')
  const out = []
  let m
  while ((m = re.exec(src)) !== null) {
    const bodyStart = m.index + m[0].length
    const bodyEnd = helperBodyEnd(src, bodyStart)
    if (bodyEnd < 0) continue
    const body = src.slice(bodyStart, bodyEnd + 1)
    if (!new RegExp(`${RAW_READ_PREFIX}\\s*\\(`).test(body)) continue
    out.push({
      name: m[1],
      line: src.slice(0, m.index).split('\n').length,
      normalized: EOL_NORMALIZE_RE.test(body),
      rawRead: RAW_READ_IN_BODY_RES.some((r) => r.test(body)),
    })
  }
  return out
}

/** 解码 git 引号路径里的八进制转义（`\345\217\211` → 中文）。本包目录名是中文，必然命中。 */
export function decodeGitQuotedPath(p) {
  const bytes = []
  const s = String(p)
  for (let i = 0; i < s.length; i++) {
    if (s[i] === '\\' && /[0-7]/.test(s[i + 1] ?? '')) {
      const oct = s.slice(i + 1, i + 4)
      if (/^[0-7]{3}$/.test(oct)) { bytes.push(parseInt(oct, 8)); i += 3; continue }
    }
    bytes.push(...Buffer.from(s[i], 'utf8'))
  }
  return Buffer.from(bytes).toString('utf8')
}

/** 解析 git diff 的新增行 → Map<仓库相对路径, Set<行号>>（与 scripts/check-el-controls.mjs 同款）
 *
 *  ⚠️ **必须兼容被引号包起来、且八进制转义的路径**：本包目录名是中文，git 在 `+++` 行会写成
 *  `"b/\345\217\211…"`（即便 `core.quotepath=false` 也会 —— 实测）。只认 `^\+\+\+ b/` 会
 *  **完全匹配不到** ⇒ `--diff` 静默变空、门形同虚设（实测踩过）。 */
export function parseAddedLines(diffText) {
  const added = new Map()
  let file = null
  let lineNo = 0
  let inHunk = false
  for (const line of String(diffText).split('\n')) {
    const f = line.match(/^\+\+\+\s+(?:"b\/(.+?)"|b\/(.+?))\s*$/)
    if (f) {
      file = decodeGitQuotedPath(f[1] ?? f[2])
      if (!added.has(file)) added.set(file, new Set())
      inHunk = false
      continue
    }
    const h = line.match(/^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]+)(?:,[0-9]+)? @@/)
    if (h) { lineNo = Number(h[1]); inHunk = true; continue }
    if (!inHunk || !file) continue
    if (line.startsWith('+')) {
      if (line.startsWith('+++')) continue
      added.get(file).add(lineNo)
      lineNo++
      continue
    }
    if (line.startsWith('-')) continue
    if (line.startsWith('\\')) continue
    lineNo++
  }
  return added
}

/** 仓库已跟踪文件清单（供 isTracked；git 不可用 ⇒ 抛错，由 main 转成 fail-closed 的 exit 2） */
function trackedSet() {
  const out = execFileSync('git', ['-c', 'core.quotepath=false', 'ls-tree', '-r', '--name-only', 'HEAD'], {
    cwd: REPO_ROOT,
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
  })
  return new Set(out.split(/\r?\n/).filter(Boolean))
}

/** 以 basename 建索引（兜底解析用；同名多命中视为不可判定 ⇒ 放行） */
function buildBasenameIndex(trackedFiles) {
  const map = new Map()
  for (const p of trackedFiles) {
    const b = p.slice(p.lastIndexOf('/') + 1)
    if (!map.has(b)) map.set(b, [])
    map.get(b).push(p)
  }
  return map
}

/** 把仓库相对路径（git 输出）映射到本包内的相对路径；不属于本包 ⇒ null */
function toMobileRel(repoRel) {
  const p = repoRel.replace(/\\/g, '/')
  if (!p.startsWith(MOBILE_REL_PREFIX + '/')) return null
  return p.slice(MOBILE_REL_PREFIX.length + 1)
}

const ACTIONABLE_HELP = [
  '== 契约测试读源码必须经「读取层归一」==',
  '改法：把裸读换成共享读者 —— utils/utsHarness.js 的 readText(abs)（或本地读者上加 .replace(/\\r\\n/g, \'\\n\')）：',
  '',
  '    const { readText } = require(\'./utsHarness\');',
  '    const src = readText(path.join(ROOT, \'api/request.uts\'));',
  '',
  '为什么：Windows 检出（core.autocrlf=true）把源文件变成 CRLF，多行锚点里 `.` 不匹配 `\\r`',
  '⇒ 锚点在 CRLF 行上匹配 0 次，变异没生效而断言照跑 ⇒ 守护**静默失效**。CI 跑 ubuntu、默认关',
  'autocrlf，所以这一类在 CI 里**结构性不可见**。',
  '详见移动端 docs/adr/0019-契约测试换行符盲区与读取层归一.md（票 B / #1177）。',
].join('\n')

function reportList(violations) {
  console.error('===== 契约测试裸读仓内源码（未归一）=====')
  for (const v of violations) console.error(`${v.file}:${v.line}: 读 ${v.target}\n    ${v.text}`)
  console.error('---')
  console.error(`共 ${violations.length} 处。`)
  console.error(ACTIONABLE_HELP)
}

function reportAll() {
  const isTracked = (rel) => tracked.has(rel)
  const files = readdirSync(UTILS_DIR).filter((f) => f.endsWith('.test.js')).sort()
  const violations = []
  for (const name of files) {
    const abs = join(UTILS_DIR, name)
    let src
    try {
      src = readFileSync(abs, 'utf8')
    } catch (e) {
      console.error(`[check-contract-read] 读不出 ${name}（fail-closed）：${e.message}`)
      return 2
    }
    for (const v of scanSource(src, isTracked)) {
      violations.push({ file: `${UTILS_REL_PREFIX}/${name}`, ...v })
    }
  }
  if (violations.length === 0) {
    console.log(`[check-contract-read] --all 通过：${files.length} 个测试文件无「未归一的仓内源码裸读」。`)
    return 0
  }
  reportList(violations)
  console.error('\n（注：--all 是**存量收敛判据**（票 C）。日常 PR 走 --diff，只看新增行。）')
  return 1
}

/**
 * 取相对 base 的 diff（`-U0`，路径限定本包）。
 *
 * **三点优先、浅历史退双点**（与 frontend-check「守卫增量门」注释里的 CI 口径一致）：
 * CI 的 checkout 是单分支浅克隆，那条 `git fetch --depth=1` 只带回默认分支那**一个**提交，
 * 于是 `base...HEAD` 因**没有共同祖先**直接报 `fatal: … no merge base`（实测，#1177 CI 首跑即红）。
 * 浅历史下取不到 merge base，就退成 `git diff base HEAD`（两点）——它比的是两棵树，
 * 不依赖祖先关系。两种都失败才 fail-closed。
 */
function diffAgainst(base) {
  const baseArgs = ['-c', 'core.quotepath=false', 'diff', '-U0']
  const tail = ['--', MOBILE_REL_PREFIX]
  const opts = { cwd: REPO_ROOT, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 }
  try {
    // 三点：一个 range 参数
    return execFileSync('git', [...baseArgs, `${base}...HEAD`, ...tail], opts)
  } catch (e) {
    const msg = String(e.stderr ?? e.message ?? '')
    if (!/no merge base/i.test(msg)) throw e
    console.error(`[check-contract-read] ${base}...HEAD 无共同祖先（浅历史），退成两点 diff`)
    // ⚠️ 两点必须是**两个**参数：`git diff origin/master HEAD`。写成单个 'origin/master HEAD'
    //    git 会当成一个 revision 解析 ⇒ `fatal: bad revision 'origin/master HEAD'`（实测踩过）。
    return execFileSync('git', [...baseArgs, base, 'HEAD', ...tail], opts)
  }
}

function reportDiff(base) {
  try {
    execFileSync('git', ['rev-parse', '--verify', base], { cwd: REPO_ROOT, stdio: 'ignore' })
  } catch {
    console.error(`[check-contract-read] 无法解析 base ref: ${base}（fail-closed；CI 上请先 git fetch 目标分支）`)
    return 2
  }
  let diff
  try {
    diff = diffAgainst(base)
  } catch (e) {
    console.error(`[check-contract-read] git diff 失败（fail-closed）：${e.message}`)
    return 2
  }
  const added = parseAddedLines(diff)
  if (added.size === 0) {
    console.log(`[check-contract-read] 相对 ${base} 本包无新增行，跳过。`)
    return 0
  }
  const isTracked = (rel) => tracked.has(rel)
  const violations = []
  for (const [repoRel, lines] of added) {
    const mobileRel = toMobileRel(repoRel)
    if (!mobileRel || !mobileRel.startsWith('utils/') || !mobileRel.endsWith('.test.js')) continue
    let src
    try {
      src = readFileSync(join(MOBILE_ROOT, mobileRel), 'utf8')
    } catch {
      continue // 删除的文件
    }
    for (const v of scanSource(src, isTracked)) {
      if (lines.has(v.line)) violations.push({ file: repoRel, ...v })
    }
  }
  if (violations.length === 0) {
    console.log(`[check-contract-read] 相对 ${base} 的新增行未裸读仓内源码，通过。`)
    return 0
  }
  reportList(violations)
  console.error('\n（本步只判**新增行**；存量的收敛见 #1178。）')
  return 1
}

function main(argv) {
  const mode = argv[0] ?? '--all'
  if (mode !== '--all' && mode !== '--diff') {
    console.error('用法: node scripts/check-contract-read.mjs --all | --diff [base]')
    return 2
  }
  try {
    tracked = trackedSet()
    byBasename = buildBasenameIndex(tracked)
  } catch (e) {
    console.error(`[check-contract-read] 取不到已跟踪文件清单（fail-closed）：${e.message}`)
    return 2
  }
  return mode === '--all' ? reportAll() : reportDiff(argv[1] ?? 'origin/master')
}

// 仅作为 CLI 直接运行时才执行：被 import（自检里喂源码文本）时不产生副作用。
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = main(process.argv.slice(2))
}
