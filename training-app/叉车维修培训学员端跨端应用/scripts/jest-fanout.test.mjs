/**
 * jest-fanout.mjs 的可执行单测（#1715）
 *
 * 为什么要有：分片器坏掉有两种 silent 形态，都必须钉住 ——
 *   ① **漏套件 / 重复套件**：某片少跑几个文件，门照绿但没测（比红更糟）；
 *   ② **切到 0 个套件**：要么静默跑 0 个（假绿），要么看不出分片口径已坏（血账 #1144）。
 * 另加一条正面判据：LPT 的**意义**是让慢片靠近均值，若实现退化成轮转，这条要红。
 *
 * 运行：node scripts/jest-fanout.test.mjs（CI 见 .github/workflows/ci.yml 的 mobile-test job，仅第 1 片）
 */
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { loadTimings, parseTimings, planShards, relKeyOf, weightOf, DEFAULT_TIMINGS_PATH } from './jest-fanout.mjs'

const HERE = dirname(fileURLToPath(import.meta.url))
const SCRIPT = join(HERE, 'jest-fanout.mjs')
const APP_ROOT = resolve(HERE, '..')
const TIMINGS = join(APP_ROOT, DEFAULT_TIMINGS_PATH)

let failures = 0
let passed = 0
function test(name, fn) {
  try {
    fn()
    passed += 1
    console.log(`ok   ${name}`)
  } catch (e) {
    failures += 1
    console.error(`FAIL ${name}\n     ${e.message}`)
  }
}

/** 合成清单：189 个套件（对齐真实全量），并把真实慢套件混进去，让配平吃到真权重 */
function syntheticFiles(n = 189) {
  const timings = loadTimings(TIMINGS)
  const heavy = [...timings.suites.keys()]
  const files = []
  for (let i = 0; i < n; i += 1) {
    files.push(join(APP_ROOT, `utils/suite${String(i).padStart(3, '0')}.test.js`))
  }
  // 把前 heavy.length 个换成真实慢套件的绝对路径（它们才是权重主导项）
  heavy.forEach((key, i) => {
    if (i < files.length) files[i] = join(APP_ROOT, key)
  })
  return files
}

test('并集 = 全量、交集 = 空（漏套件 / 重复套件都算坏）', () => {
  const files = syntheticFiles()
  const timings = loadTimings(TIMINGS)
  const { shards } = planShards(files, timings, 4, APP_ROOT)
  const flat = shards.flat()
  assert.equal(flat.length, files.length, `片内总数 ${flat.length} != 全量 ${files.length}`)
  assert.equal(new Set(flat).size, files.length, '出现了重复套件')
  for (const f of files) assert.ok(flat.includes(f), `套件 ${f} 没被分到任何片`)
})

test('每片非空（4 片 / 189 套件）', () => {
  const { shards } = planShards(syntheticFiles(), loadTimings(TIMINGS), 4, APP_ROOT)
  assert.equal(shards.length, 4)
  for (let i = 0; i < 4; i += 1) assert.ok(shards[i].length > 0, `第 ${i + 1} 片是空的`)
})

test('确定性：同输入两次结果逐字相同', () => {
  const files = syntheticFiles()
  const timings = loadTimings(TIMINGS)
  const a = planShards(files, timings, 4, APP_ROOT).shards
  const b = planShards(files, timings, 4, APP_ROOT).shards
  assert.deepEqual(a, b)
})

test('未知套件按 defaultSeconds 计权（新增套件不报错、不归零）', () => {
  const t = parseTimings(JSON.stringify({ defaultSeconds: 2, suites: { 'utils/a.test.js': 30 } }))
  assert.equal(weightOf('utils/a.test.js', t), 30)
  assert.equal(weightOf('utils/brand-new.test.js', t), 2)
  const files = [join(APP_ROOT, 'utils/a.test.js'), join(APP_ROOT, 'utils/brand-new.test.js')]
  const plan = planShards(files, t, 2, APP_ROOT)
  assert.equal(plan.total, 32)
  assert.deepEqual(plan.loads.slice().sort((x, y) => x - y), [2, 30])
})

test('LPT 把最慢套件单独隔开（不是按序号轮转）', () => {
  const t = parseTimings(JSON.stringify({ defaultSeconds: 1, suites: { 'utils/heavy.test.js': 100 } }))
  const files = [join(APP_ROOT, 'utils/heavy.test.js')]
  for (let i = 0; i < 8; i += 1) files.push(join(APP_ROOT, `utils/n${i}.test.js`))
  const { shards } = planShards(files, t, 4, APP_ROOT)
  const heavyShard = shards.findIndex((s) => s.some((f) => f.endsWith('heavy.test.js')))
  assert.equal(shards[heavyShard].length, 1, `最慢套件所在片还塞了 ${shards[heavyShard].length - 1} 个别的套件`)
})

test('配平判据：最慢片 <= max(总/4, 最慢套件) 的 1.15 倍', () => {
  const files = syntheticFiles()
  const timings = loadTimings(TIMINGS)
  const plan = planShards(files, timings, 4, APP_ROOT)
  const heaviest = Math.max(...plan.weights)
  const floor = Math.max(plan.total / 4, heaviest)
  const worst = Math.max(...plan.loads)
  assert.ok(
    worst <= floor * 1.15,
    `最慢片 ${worst.toFixed(1)}s > 下界 ${floor.toFixed(1)}s × 1.15`,
  )
})

test('relKeyOf：绝对路径 / 相对路径 / 反斜杠 / 别的根，都能折成表里的键', () => {
  assert.equal(relKeyOf(join(APP_ROOT, 'utils', 'x.test.js'), APP_ROOT), 'utils/x.test.js')
  assert.equal(relKeyOf('utils/x.test.js', APP_ROOT), 'utils/x.test.js')
  // 反斜杠形态要由本机真实绝对路径派生 —— 写死 'D:\FL\wt-1\...' 只在 Windows 成立，
  // 上 CI（/home/runner/work/...）会红（#1717 首跑实测）。
  const winStyle = join(APP_ROOT, 'utils', 'x.test.js').replace(/\//g, '\\')
  assert.equal(relKeyOf(winStyle, APP_ROOT), 'utils/x.test.js')
  // 尾段兜底：不在本根下面的绝对路径（别的机器 / 别的 clone 根）也按 utils/... 折键。
  assert.equal(relKeyOf(join(APP_ROOT, '..', 'another-root', 'utils', 'x.test.js'), APP_ROOT), 'utils/x.test.js')
})

test('parseTimings fail-closed：defaultSeconds 缺失 / 非数 / suites 非对象 ⇒ throw', () => {
  assert.throws(() => parseTimings('{}'), /defaultSeconds/)
  assert.throws(() => parseTimings(JSON.stringify({ defaultSeconds: 0, suites: {} })), /defaultSeconds/)
  assert.throws(() => parseTimings(JSON.stringify({ defaultSeconds: 1, suites: [] })), /suites/)
  assert.throws(() => parseTimings('not json'), /JSON/)
})

function runCli(args, stdin) {
  try {
    const out = execFileSync(process.execPath, [SCRIPT, ...args], { input: stdin, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'] })
    return { code: 0, out }
  } catch (e) {
    return { code: e.status ?? -1, out: String(e.stdout ?? '') + String(e.stderr ?? '') }
  }
}

test('CLI：--shard 越界 / 未知参数 ⇒ exit 2', () => {
  assert.equal(runCli(['--shard', '5', '--shards', '4'], 'x\n').code, 2)
  assert.equal(runCli(['--nonsense'], 'x\n').code, 2)
})

test('CLI：清单为空 ⇒ exit 3（fail-closed，不当绿）', () => {
  assert.equal(runCli(['--shard', '1', '--shards', '4', '--timings', TIMINGS], '').code, 3)
})

test('CLI：正常切片 ⇒ exit 0 且本片文件数 = 计划里那一片', () => {
  const files = syntheticFiles()
  const timings = loadTimings(TIMINGS)
  const expected = planShards(files, timings, 4, APP_ROOT).shards[0]
  const r = runCli(['--shard', '1', '--shards', '4', '--timings', TIMINGS], files.join('\n') + '\n')
  assert.equal(r.code, 0, `输出：${r.out.slice(0, 300)}`)
  const got = r.out.split('\n').map((l) => l.trim()).filter(Boolean)
  assert.deepEqual(got, expected)
})

// ---------------------------------------------------------------- 汇总

console.log('')
if (failures > 0) {
  console.error(`jest-fanout 单测失败：${failures} 条（通过 ${passed} 条）`)
  process.exitCode = 1
} else {
  console.log(`jest-fanout 单测全绿：${passed} 条`)
}
