/**
 * check-contract-read.mjs 的可执行单测（#1177 / ADR-0019 票 B）
 *
 * 为什么要有：判据的机检形状必须能被钉住 —— 尤其两条容易写错的：
 *   ① **假阴性边界**：解不出目标的裸读**有意放行**（免 allowlist 的代价），
 *      这条要被显式钉住，免得日后有人「顺手收紧」把合法例外判红；
 *   ② **fail-closed**：base 不可解析 / diff 失败必须**非零退出**，不得静默放行。
 *
 * 运行：node scripts/check-contract-read.test.mjs（CI 见 .github/workflows/ci.yml 的 mobile-test job）
 */
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import {
  decodeGitQuotedPath,
  stripComments,
  matchParen,
  collectConsts,
  resolveTargets,
  parseAddedLines,
  scanSource,
} from './check-contract-read.mjs'

const HERE = dirname(fileURLToPath(import.meta.url))
const SCRIPT = join(HERE, 'check-contract-read.mjs')
const REPO_ROOT = resolve(HERE, '..', '..', '..')

/** 把「仓内已跟踪」判定固定成我们喂进去的集合，测试与真实仓库解耦 */
const trackedOf = (...rels) => {
  const s = new Set(rels)
  return (rel) => s.has(rel)
}
const MOBILE = 'training-app/叉车维修培训学员端跨端应用'

let failures = 0
let passed = 0
function test(name, fn) {
  try {
    fn()
    passed++
    console.log(`  √ ${name}`)
  } catch (e) {
    failures++
    console.log(`  × ${name}\n      ${e.message}`)
  }
}

console.log('check-contract-read 单测')

// ---------------------------------------------------------------- 基础件

test('stripComments：块注释与行注释里的 readFileSync 不算调用点', () => {
  const src = ['/* fs.readFileSync(a) */', 'const x = 1; // fs.readFileSync(b)', 'const y = fs.readFileSync(c);'].join('\n')
  const code = stripComments(src)
  assert.equal((code.match(/readFileSync/g) ?? []).length, 1, '应只剩真实调用点')
})

test('matchParen：跳过字符串里的括号', () => {
  const src = "f(a, 'x)y', b)"
  const open = src.indexOf('(')
  assert.equal(src[matchParen(src, open)], ')')
  assert.equal(matchParen(src, open), src.length - 1)
})

test('collectConsts：字面量与 path.join 常量都被收；根被换成哨兵', () => {
  const src = [
    "const A = 'api/request.uts';",
    "const B = path.join(ROOT, 'scripts', 'x.ps1');",
    "const C = path.join(__dirname, 'pointsDisplay.uts');",
  ].join('\n')
  const c = collectConsts(src)
  assert.equal(c.get('A'), 'api/request.uts')
  assert.match(c.get('B'), /@ROOT@/)
  assert.match(c.get('C'), /@DIR@/)
})

test('resolveTargets：__dirname 根解析到本包 utils（不过度上溯）', () => {
  const out = resolveTargets("path.join(__dirname, 'pointsDisplay.uts')", new Map(), [])
  assert.deepEqual(out, [`${MOBILE}/utils/pointsDisplay.uts`])
})

test('resolveTargets：ROOT 根解析到本包根', () => {
  const out = resolveTargets("path.join(ROOT, 'api', 'request.uts')", new Map(), [])
  assert.deepEqual(out, [`${MOBILE}/api/request.uts`])
})

test('resolveTargets：常量经路径拼接的 `..` 能正确归一（上溯到仓根）', () => {
  const consts = collectConsts("const P = path.join(__dirname, '..', '..', '..', 'frontend', 'src', 'a.ts');")
  const out = resolveTargets('P', consts, [])
  assert.deepEqual(out, ['frontend/src/a.ts'])
})

// ---------------------------------------------------------------- 判据：违规面

test('违规①：裸读仓内源码（字面量路径）', () => {
  const src = "const s = fs.readFileSync(path.join(ROOT, 'api/request.uts'), 'utf8');"
  const v = scanSource(src, trackedOf(`${MOBILE}/api/request.uts`))
  assert.equal(v.length, 1)
  assert.equal(v[0].target, `${MOBILE}/api/request.uts`)
})

test('违规②：经常量拼接的仓内源码裸读', () => {
  const src = [
    "const SCRIPT_REL = 'scripts/lib/x.ps1';",
    "const src = fs.readFileSync(path.join(ROOT, SCRIPT_REL), 'utf8');",
  ].join('\n')
  const v = scanSource(src, trackedOf(`${MOBILE}/scripts/lib/x.ps1`))
  assert.equal(v.length, 1)
})

test('违规③：读者助手参数绑定的仓内源码裸读（p 被实参解出）', () => {
  const src = [
    "const read = (p) => fs.readFileSync(path.join(ROOT, p), 'utf8');",
    "const s = read('api/request.uts');",
  ].join('\n')
  const v = scanSource(src, trackedOf(`${MOBILE}/api/request.uts`))
  assert.equal(v.length, 1, '应经助手参数绑定解出目标')
})

// ---------------------------------------------------------------- 判据：放行面（假阴性边界，显式钉住）

test('放行①：链式归一的裸读', () => {
  const src = "const s = fs.readFileSync(path.join(ROOT, 'api/request.uts'), 'utf8').replace(/\\r\\n/g, '\\n');"
  assert.equal(scanSource(src, trackedOf(`${MOBILE}/api/request.uts`)).length, 0)
})

test('放行②：本地读者已归一时，用它的调用点放行', () => {
  const src = [
    "const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8').replace(/\\r\\n/g, '\\n');",
    "const s = read('api/request.uts');",
  ].join('\n')
  assert.equal(scanSource(src, trackedOf(`${MOBILE}/api/request.uts`)).length, 0)
})

test('放行③：共享读者 readText 的调用点放行', () => {
  const src = [
    "const { readText } = require('./utsHarness');",
    "const s = readText(path.join(ROOT, 'api/request.uts'));",
  ].join('\n')
  assert.equal(scanSource(src, trackedOf(`${MOBILE}/api/request.uts`)).length, 0)
})

test('放行④（免 allowlist 的代价）：目标解不出 / 非已跟踪 ⇒ 放行', () => {
  const src = [
    "const capFile = path.join(os.tmpdir(), 'harness.capture.txt');",
    "const out = fs.readFileSync(capFile, 'utf8');",
    "const f = fs.readFileSync(file, 'utf8');",
  ].join('\n')
  // 即便把这两个路径都当作「已跟踪」也不该命中 —— 因为实参里根本没有它们
  assert.equal(scanSource(src, () => true).length, 0)
})

test('放行⑤：注释里的裸读不算', () => {
  const src = [
    '/*',
    " * const s = fs.readFileSync(path.join(ROOT, 'api/request.uts'), 'utf8');",
    ' */',
  ].join('\n')
  assert.equal(scanSource(src, trackedOf(`${MOBILE}/api/request.uts`)).length, 0)
})

// ---------------------------------------------------------------- 判据：读者助手体（#1178 追加裁定）

test('助手体①：`const read = (p) => fs.readFileSync(p, "utf8")` 的助手体未归一 ⇒ 违规（报在声明行）', () => {
  const src = "const read = (p) => fs.readFileSync(p, 'utf8');"
  const v = scanSource(src, trackedOf(`${MOBILE}/api/request.uts`))
  assert.equal(v.length, 1, '助手体未归一必须报红 —— 与调用点目标能否解析无关')
  assert.equal(v[0].line, 1, '违规点报在**助手声明行**（--diff 按新增行号筛）')
  assert.match(v[0].target, /助手体/)
})

test('助手体②：带 `path.join(ROOT, p)` 实参的助手体未归一 ⇒ 违规', () => {
  const src = "const read = (p) => fs.readFileSync(path.join(ROOT, p), 'utf8');"
  const v = scanSource(src, trackedOf())
  assert.equal(v.length, 1)
  assert.equal(v[0].line, 1)
})

test('助手体③：助手体**已**归一（.replace）⇒ 放行（同一个助手，只差那一层）', () => {
  const src = "const read = (p) => fs.readFileSync(p, 'utf8').replace(/\\r\\n/g, '\\n');"
  assert.equal(scanSource(src, trackedOf()).length, 0)
})

test('助手体④：助手体建在共享读者 readText 上 ⇒ 放行（ADR-0019 的首选改法）', () => {
  const src = [
    "const { readText } = require('./utsHarness');",
    'const read = (rel) => readText(path.join(ROOT, rel));',
  ].join('\n')
  assert.equal(scanSource(src, trackedOf()).length, 0)
})

test('助手体⑤：**多行**助手体里的裸读也被判红（形态不同、判据相同）', () => {
  const src = [
    'const read = (rel) =>',
    "  fs.readFileSync(path.join(ROOT, rel), 'utf8');",
  ].join('\n')
  const v = scanSource(src, trackedOf())
  assert.equal(v.length, 1)
  assert.equal(v[0].line, 1)
})

test('助手体⑥：同一助手体多处裸读只报一次（不刷屏）', () => {
  const src = [
    'const read = (rel) => {',
    "  const a = fs.readFileSync(path.join(ROOT, rel), 'utf8');",
    "  return a + fs.readFileSync(path.join(ROOT, rel), 'utf8');",
    '};',
  ].join('\n')
  const v = scanSource(src, trackedOf())
  assert.equal(v.length, 1, `一个助手只报一次，实得 ${v.length}`)
})

test('助手体⑦：`function NAME() { return fs.readFileSync(…) }` 形态**不在**规则二内（写实边界）', () => {
  const src = [
    'function readSource() {',
    "  return fs.readFileSync(path.join(ROOT, SCRIPT_REL), 'utf8');",
    '}',
  ].join('\n')
  assert.equal(scanSource(src, trackedOf()).length, 0, '规则二只认 `const NAME = (…) =>` —— 不假装覆盖 function 形态')
})

test('助手体⑧（**已知并接受的代价**，非回归）：读者助手体的实参指向非仓内目标时也判红', () => {
  // 这一条**故意**钉住规则二的假阳性面：判据只看「助手体未归一」，**不看**实参能否解析到仓根
  // （#1178 追加裁定的原话就是「不再要求该调用点的目标能静态解析」）。所以一个只读临时产物的
  // 读者助手也会被判红。为什么可以接受：全仓实测「体里有裸读的 `const … =>` 助手」共 26 个，
  // **无一**只读临时/运行期产物（全部是 `path.join(ROOT|__dirname, …)` 的仓内源码），故这条
  // 规则在**存量与现状**上零假阳性；真正的裸读临时产物走的是**非助手**形态（`function` 声明体、
  // `.map((f) => …)` 这类回调、以及赋值右侧的内联读），规则二不碰它们 —— 见 `放行④` 与 `助手体⑦⑨`。
  // 若日后真出现「只读临时产物的 `const … =>` 读者」，改法是**把它改成不做 `readFileSync` 的
  // 普通函数/内联读**，而不是放宽本判据（放宽会让 26 个缺陷面重新漏掉）。
  const src = [
    "const readCapture = (capFile) => fs.readFileSync(capFile, 'utf8');",
    'const out = readCapture(capFile);',
  ].join('\n')
  const v = scanSource(src, () => true)
  assert.equal(v.length, 1)
  assert.equal(v[0].line, 1)
})

test('助手体⑨：**非助手**的裸读临时产物仍放行（与上一条合起来才是完整的零误报口径）', () => {
  const src = [
    'function readLaunchOut(logDir) {',
    "  return fs.readdirSync(logDir).map((f) => fs.readFileSync(path.join(logDir, f), 'utf8')).join('\\n');",
    '}',
    "const stdout = fs.existsSync(capFile) ? fs.readFileSync(capFile, 'utf8') : '';",
    "const stderr = fs.readFileSync(capFile + '.err', 'utf8');",
  ].join('\n')
  assert.equal(scanSource(src, () => true).length, 0, '临时/运行期产物的裸读不得被判红（票 B 已实测的零误报面）')
})

// ---------------------------------------------------------------- diff 解析

test('parseAddedLines：新增行号按新侧推进（含上下文行与删除行）', () => {
  const diff = [
    'diff --git a/utils/x.test.js b/utils/x.test.js',
    '--- a/utils/x.test.js',
    '+++ b/utils/x.test.js',
    '@@ -1,2 +1,3 @@',
    ' context',
    '+added1',
    '+added2',
    '-removed',
    ' tail',
  ].join('\n')
  const map = parseAddedLines(diff)
  assert.deepEqual([...map.get('utils/x.test.js')].sort((a, b) => a - b), [2, 3])
})

test('parseAddedLines：非 ASCII 路径两种形态都能解析 —— 否则 --diff 静默变空', () => {
  // 实测：本包目录名是中文。git 在某些环境下把 +++ 行写成**带引号且八进制转义**的形式，
  // 直接 `^\+\+\+ b/` 匹配不到 ⇒ --diff 变空、门形同虚设。两种形态都钉住。
  const cn = '叉车维修培训学员端跨端应用'
  const rel = `training-app/${cn}/utils/coursesContract.test.js`
  const octal = [...Buffer.from(rel, 'utf8')]
    .map((b) => (b < 0x80 ? String.fromCharCode(b) : `\\${b.toString(8).padStart(3, '0')}`))
    .join('')
  const mk = (plusPath) => [
    `--- a/${rel}`,
    `+++ ${plusPath}`,
    '@@ -1,0 +2,2 @@',
    '+const a = 1;',
    '+const b = 2;',
  ].join('\n')

  for (const [name, plusPath] of [
    ['引号+八进制', `"b/${octal}"`],
    ['不带引号（UTF-8 原样）', `b/${rel}`],
  ]) {
    const map = parseAddedLines(mk(plusPath))
    const keys = [...map.keys()]
    assert.equal(keys.length, 1, `${name}：应解析出 1 个文件，实得 ${JSON.stringify(keys)}`)
    assert.equal(keys[0], rel, `${name}：路径应解码还原`)
    assert.deepEqual([...map.get(keys[0])].sort((a, b) => a - b), [2, 3], `${name}：行号`)
  }
})

test('decodeGitQuotedPath：纯 ASCII 路径原样返回', () => {
  assert.equal(decodeGitQuotedPath('utils/x.test.js'), 'utils/x.test.js')
})

// ---------------------------------------------------------------- CLI 行为（fail-closed）

function runCli(args) {
  try {
    const out = execFileSync(process.execPath, [SCRIPT, ...args], {
      cwd: REPO_ROOT,
      encoding: 'utf8',
      maxBuffer: 32 * 1024 * 1024,
    })
    return { code: 0, out }
  } catch (e) {
    return { code: e.status ?? -1, out: String(e.stdout ?? '') + String(e.stderr ?? '') }
  }
}

test('CLI fail-closed：base 不可解析 ⇒ exit 2（不得静默放行）', () => {
  const r = runCli(['--diff', 'origin/definitely-not-a-real-ref-xyz'])
  assert.equal(r.code, 2, `期望 2，实得 ${r.code}；输出：${r.out.slice(0, 200)}`)
})

test('CLI：用法错 ⇒ exit 2', () => {
  assert.equal(runCli(['--nonsense']).code, 2)
})

test('CLI --diff：base 可解析但无本包新增行 ⇒ exit 0', () => {
  const r = runCli(['--diff', 'HEAD'])
  assert.equal(r.code, 0, `期望 0，实得 ${r.code}；输出：${r.out.slice(0, 200)}`)
})

// ---------------------------------------------------------------- 汇总

console.log('')
if (failures > 0) {
  console.error(`check-contract-read 单测失败：${failures} 条（通过 ${passed} 条）`)
  process.exitCode = 1
} else {
  console.log(`check-contract-read 单测全绿：${passed} 条`)
}
