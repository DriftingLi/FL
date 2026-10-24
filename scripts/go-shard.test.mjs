// backend-test 分片器的抽取式单测（issue #1862）。
//
// 分片器是「分片这件事的判据」：它自己坏了，CI 会**看起来**照常跑完两片、实际却有包
// 没被测到（漏测）或某片空转（假绿）。所以判据本身要钉住：划分完整性、LPT 的隔离效果、
// 未知包的兜底权重、以及空片 fail-closed 的退出码。
//
// 运行：node --test scripts/go-shard.test.mjs
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { mkdtempSync, writeFileSync, readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { DEFAULT_SECONDS, EXIT_BAD_ARGS, EXIT_EMPTY_SHARD, packShards, parseShardArgs } from './go-shard.mjs'

const HERE = dirname(fileURLToPath(import.meta.url))
const SCRIPT = resolve(HERE, 'go-shard.mjs')
const REAL_TABLE = resolve(HERE, 'go-package-timings.json')

/** 跑一次 CLI，返回 { status, stdout, stderr }（非零退出不抛）。 */
function runCli(args) {
  try {
    const stdout = execFileSync(process.execPath, [SCRIPT, ...args], { encoding: 'utf8', stdio: 'pipe' })
    return { status: 0, stdout, stderr: '' }
  } catch (e) {
    return { status: e.status ?? -1, stdout: String(e.stdout ?? ''), stderr: String(e.stderr ?? '') }
  }
}

/** 把包清单与权重表落到临时目录，返回两个路径。 */
function fixture(packages, weights) {
  const dir = mkdtempSync(join(tmpdir(), 'go-shard-'))
  const list = join(dir, 'packages.txt')
  const table = join(dir, 'weights.json')
  writeFileSync(list, packages.join('\n') + '\n', 'utf8')
  writeFileSync(table, JSON.stringify({ defaultSeconds: DEFAULT_SECONDS, packages: weights }), 'utf8')
  return { list, table }
}

test('划分完整性：每片互斥、并集等于输入（不丢包、不重复）', () => {
  const packages = Array.from({ length: 37 }, (_, i) => 'fl/pkg' + String(i).padStart(2, '0'))
  const weights = Object.fromEntries(packages.map((p, i) => [p, (i % 7) + 1]))
  for (const shards of [1, 2, 3, 4]) {
    const bins = packShards(packages, weights, shards)
    assert.equal(bins.length, shards, '片数 ' + shards)
    const flat = bins.flat()
    assert.equal(flat.length, packages.length, 'shards=' + shards + ' 总包数')
    assert.deepEqual([...flat].sort(), [...packages].sort(), 'shards=' + shards + ' 并集')
    assert.equal(new Set(flat).size, flat.length, '不允许重复')
  }
})

test('LPT：最慢的包独占一片，其余按负载摊平', () => {
  const weights = { 'fl/api': 400, 'fl/a': 30, 'fl/b': 20, 'fl/c': 10 }
  const bins = packShards(Object.keys(weights), weights, 2)
  assert.deepEqual(bins[0], ['fl/api'])
  assert.deepEqual(bins[1], ['fl/a', 'fl/b', 'fl/c'])
})

test('负载上界：任一片的负载不超过 total/k + 最重单包（LPT 的性质）', () => {
  const weights = { p1: 400, p2: 120, p3: 39, p4: 26, p5: 22, p6: 22, p7: 21, p8: 19 }
  const packages = Object.keys(weights)
  const total = Object.values(weights).reduce((a, b) => a + b, 0)
  const heaviest = Math.max(...Object.values(weights))
  for (const shards of [2, 3, 4]) {
    const bins = packShards(packages, weights, shards)
    const loads = bins.map((bin) => bin.reduce((sum, p) => sum + weights[p], 0))
    for (const load of loads) {
      assert.ok(load <= total / shards + heaviest, 'shards=' + shards + ' 有片超界: ' + load)
    }
  }
})

test('未知包按 defaultSeconds 计权，但绝不能丢', () => {
  const weights = { known: 100 }
  const bins = packShards(['known', 'mystery-one', 'mystery-two'], weights, 2, 10)
  assert.deepEqual(bins[0], ['known'])
  assert.deepEqual([...bins[1]].sort(), ['mystery-one', 'mystery-two'])
})

test('输入顺序无关：洗牌后结果一致（判据不能依赖 go list 的输出顺序）', () => {
  const weights = { a: 50, b: 40, c: 30, d: 20, e: 10, f: 10 }
  const straight = packShards(Object.keys(weights), weights, 3)
  const shuffled = packShards(['f', 'c', 'a', 'e', 'b', 'd'], weights, 3)
  assert.deepEqual(straight, shuffled)
})

test('重复的包名只出现一次（go list 输出重复行也不该让某片多跑）', () => {
  const bins = packShards(['a', 'a', 'b'], { a: 10, b: 10 }, 2)
  assert.deepEqual(bins.flat().sort(), ['a', 'b'])
})

test('片数多于包数：多出来的片是空数组（由 CLI 层 fail-closed）', () => {
  const bins = packShards(['a', 'b'], { a: 1, b: 1 }, 4)
  assert.equal(bins.length, 4)
  assert.deepEqual(bins.slice(2), [[], []])
})

test('参数解析：缺项 / 未知项 / 越界都抛错', () => {
  assert.throws(() => parseShardArgs(['--shard', '1']), /缺少参数/)
  assert.throws(
    () => parseShardArgs(['--shard', '1', '--shards', '2', '--list', 'x', '--weights', 'y', '--nope', 'z']),
    /未知参数/
  )
  assert.throws(
    () => parseShardArgs(['--shard', '3', '--shards', '2', '--list', 'x', '--weights', 'y']),
    /--shard 必须在/
  )
  assert.throws(
    () => parseShardArgs(['--shard', '0', '--shards', '2', '--list', 'x', '--weights', 'y']),
    /--shard 必须在/
  )
})

test('CLI 正常：本片输出一行一个包，全部片拼接后等于输入', () => {
  const packages = ['fl/a', 'fl/b', 'fl/c', 'fl/d']
  const { list, table } = fixture(packages, { 'fl/a': 40, 'fl/b': 30, 'fl/c': 20, 'fl/d': 10 })
  const collected = []
  for (const shard of [1, 2]) {
    const { status, stdout } = runCli(['--shard', String(shard), '--shards', '2', '--list', list, '--weights', table])
    assert.equal(status, 0, 'shard ' + shard + ' 退出码')
    collected.push(...stdout.split('\n').filter(Boolean))
  }
  assert.deepEqual(collected.sort(), [...packages].sort())
})

test('CLI fail-closed：本片为空时退出码 3（不许当成绿）', () => {
  const { list, table } = fixture(['fl/a', 'fl/b'], { 'fl/a': 10, 'fl/b': 10 })
  const { status, stderr } = runCli(['--shard', '5', '--shards', '5', '--list', list, '--weights', table])
  assert.equal(status, EXIT_EMPTY_SHARD)
  assert.match(stderr, /取不到任何包/)
})

test('CLI fail-closed：包清单为空 / 参数错时退出码 2', () => {
  const empty = fixture([], {})
  assert.equal(runCli(['--shard', '1', '--shards', '2', '--list', empty.list, '--weights', empty.table]).status, EXIT_BAD_ARGS)
  const ok = fixture(['fl/a'], { 'fl/a': 10 })
  assert.equal(runCli(['--shard', '9', '--shards', '2', '--list', ok.list, '--weights', ok.table]).status, EXIT_BAD_ARGS)
})

test('真实权重表可解析：defaultSeconds 是正数、包数够多、权重非负（表坏了会静默均分）', () => {
  const table = JSON.parse(readFileSync(REAL_TABLE, 'utf8'))
  assert.ok(table.defaultSeconds > 0, 'defaultSeconds 必须是正数')
  const entries = Object.entries(table.packages)
  assert.ok(entries.length > 60, '权重表条目太少: ' + entries.length)
  for (const [pkg, sec] of entries) {
    assert.equal(typeof sec, 'number', pkg + ' 的权重不是数字')
    assert.ok(sec >= 0, pkg + ' 的权重是负数')
  }
  // 表里最长的那条决定了分片下限 ⇒ 它必须真的是 internal/api（换成别的包说明表取错了）
  const slowest = entries.sort((a, b) => b[1] - a[1])[0]
  assert.equal(slowest[0], 'forklift-training/internal/api')
  assert.ok(slowest[1] > 300, '最慢包的权重应 > 300s，实际 ' + slowest[1])
})
