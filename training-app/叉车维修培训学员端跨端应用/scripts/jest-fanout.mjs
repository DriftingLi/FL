#!/usr/bin/env node
/**
 * ③ 门分片器：把 jest --listTests 的套件清单按**耗时**装箱切成 n 片（LPT，最长优先 + 最空优先）
 *
 * ## 为什么不是序号轮转
 *
 * #1711 的四片用的是 `NR % 4`：按**清单顺序**轮转，完全不看单套件耗时。实测 run 39113010218
 * 四片墙钟 **1:48 / 1:03 / 2:26 / 3:36** —— 慢并不是因为总量，而是因为单个慢套件
 * （`utils/wirelessDebugBehavior.test.js` 独占 108.4 s）连同它前后一段被塞进同一片。
 * 装箱后每片目标 = `max(总耗时/n, 最慢套件)`，四片都靠近总耗时/4，慢片才有得压。
 *
 * ## 判据（谁都不新造清单）
 *
 * 输入**仍然只有** `jest --listTests` 的输出（判据是 jest 自己的 testMatch），本脚本只决定
 * 「哪个文件进哪片」；每片内部继续 `-i` 串行执行。切到 0 个套件即 **exit 3**（fail-closed），
 * 血账 #1144 的「静默匹配 0 个套件」不许再犯。
 *
 * ## 权重从哪来
 *
 * `utils/jestSuiteTimings.json`：慢套件只列实测值（jest 只打印 ≥5s 的套件耗时，故 CI 日志
 * 天然只能提供这一档），其余套件按 `defaultSeconds` 计权。**表里没有的套件不报错**、按默认权重
 * 计入 —— 新增套件会让分片略微不均，但绝不会让门变红或退化成长尾（这是有意的取舍：宁可不均匀，
 * 也不要求每个 PR 都回去重采耗时）。
 *
 * 重采：`node scripts/gen-jest-timings.mjs --log <gh run view --log 的输出>`（CI 口径）
 * 或 `--json .ci-verify/jest-timings-raw.json`（本机口径，数字与 CI 不可混用，见该脚本注释）。
 *
 * 运行（CI 里由 ci.yml 的 mobile-test job 调用）：
 *   node scripts/jest-fanout.mjs --shard 2 --shards 4 --list /tmp/all-files.txt
 */
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

export const DEFAULT_TIMINGS_PATH = 'utils/jestSuiteTimings.json'

const toPosix = (p) => p.replace(/\\/g, '/')

/** 把绝对路径（或任意相对路径）折算成 timings 表里的键：相对项目根、正斜杠、小写盘符无关 */
export function relKeyOf(file, root) {
  const f = toPosix(resolve(file))
  const r = toPosix(resolve(root)).replace(/\/$/, '')
  if (f.startsWith(r + '/')) {
    const rest = f.slice(r.length + 1)
    // 反斜杠形态在 POSIX 上会被 path.resolve 当成一个普通相对段拼到 cwd 后面 ⇒ 前缀是
    // 「cwd + '/' + '\home\…'」，剥掉前缀后仍以 '/' 开头。这种痕迹不算已折键，落兜底。
    if (rest && !rest.startsWith('/')) return rest
  }
  // 兜底：取「utils/xxx.test.js」这样的尾段（--listTests 在个别 jest 版本会打印相对路径）
  const m = f.match(/(?:^|\/)((?:pages|utils|api|components|composables|scripts)\/.*)$/)
  return m ? m[1] : f
}

export function parseTimings(raw, from = DEFAULT_TIMINGS_PATH) {
  let data
  try {
    data = JSON.parse(raw)
  } catch (e) {
    throw new Error(`${from}: 不是合法 JSON（${e.message}）`)
  }
  const d = data.defaultSeconds
  if (typeof d !== 'number' || !(d > 0)) throw new Error(`${from}: defaultSeconds 必须是正数`)
  const suites = data.suites
  if (!suites || typeof suites !== 'object' || Array.isArray(suites)) throw new Error(`${from}: suites 必须是对象`)
  const out = new Map()
  for (const [k, v] of Object.entries(suites)) {
    if (typeof v !== 'number' || !(v >= 0)) throw new Error(`${from}: suites["${k}"] 必须是 >=0 的数`)
    out.set(toPosix(k), v)
  }
  return { defaultSeconds: d, suites: out, note: String(data.note ?? '') }
}

export function loadTimings(path) {
  return parseTimings(readFileSync(path, 'utf8'), path)
}

export function weightOf(key, timings) {
  const hit = timings.suites.get(key)
  return typeof hit === 'number' ? hit : timings.defaultSeconds
}

/**
 * LPT 装箱：按权重降序依次放进「当前负载最小」的片（并列取片号小的）。
 * 返回保留输入顺序的片内容（jest 的参数顺序因此稳定）。纯函数，无 IO。
 */
export function planShards(files, timings, shardCount, root) {
  if (!Number.isInteger(shardCount) || shardCount < 1) throw new Error('shardCount 必须是正整数')
  const keys = files.map((f) => relKeyOf(f, root))
  const weights = keys.map((k) => weightOf(k, timings))
  const order = files.map((_, i) => i).sort((a, b) => weights[b] - weights[a] || keys[a].localeCompare(keys[b]))
  const loads = new Array(shardCount).fill(0)
  const buckets = Array.from({ length: shardCount }, () => [])
  for (const idx of order) {
    let target = 0
    for (let s = 1; s < shardCount; s += 1) if (loads[s] < loads[target]) target = s
    buckets[target].push(idx)
    loads[target] += weights[idx]
  }
  return {
    shards: buckets.map((b) => b.sort((x, y) => x - y).map((i) => files[i])),
    loads,
    weights,
    keys,
    total: loads.reduce((a, b) => a + b, 0),
  }
}

function parseArgs(argv) {
  const out = { shard: 0, shards: 0, list: '-', timings: DEFAULT_TIMINGS_PATH, root: process.cwd() }
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i]
    const next = () => {
      const v = argv[i + 1]
      if (v === undefined) throw new Error(`${a} 缺少取值`)
      i += 1
      return v
    }
    if (a === '--shard') out.shard = Number(next())
    else if (a === '--shards') out.shards = Number(next())
    else if (a === '--list') out.list = next()
    else if (a === '--timings') out.timings = next()
    else if (a === '--root') out.root = next()
    else throw new Error(`未知参数 ${a}`)
  }
  if (!Number.isInteger(out.shard) || out.shard < 1) throw new Error('--shard 必须是 >=1 的整数')
  if (!Number.isInteger(out.shards) || out.shards < 1) throw new Error('--shards 必须是 >=1 的整数')
  if (out.shard > out.shards) throw new Error(`--shard(${out.shard}) 不能大于 --shards(${out.shards})`)
  return out
}

function main(argv) {
  let args
  try {
    args = parseArgs(argv)
  } catch (e) {
    console.error(`jest-fanout: ${e.message}`)
    return 2
  }
  let files
  try {
    const raw = args.list === '-' ? readFileSync(0, 'utf8') : readFileSync(args.list, 'utf8')
    files = raw.split(/\r?\n/).map((l) => l.trim()).filter(Boolean)
  } catch (e) {
    console.error(`jest-fanout: 读清单失败（${args.list}）：${e.message}`)
    return 2
  }
  let timings
  try {
    timings = loadTimings(args.timings)
  } catch (e) {
    console.error(`jest-fanout: ${e.message}`)
    return 2
  }
  const plan = planShards(files, timings, args.shards, args.root)
  const mine = plan.shards[args.shard - 1]
  const load = plan.loads[args.shard - 1]
  const avg = plan.total / args.shards
  const heaviest = mine
    .map((f) => ({ f, w: weightOf(relKeyOf(f, args.root), timings) }))
    .sort((a, b) => b.w - a.w)[0]
  console.error(
    `[fanout] shard ${args.shard}/${args.shards} files=${mine.length}/${files.length} ` +
      `load=${load.toFixed(1)}s avg=${avg.toFixed(1)}s total=${plan.total.toFixed(1)}s ` +
      `maxSuite=${heaviest ? heaviest.w.toFixed(1) : 0}s`,
  )
  if (mine.length === 0) {
    console.error('jest-fanout: 本片切到 0 个套件（判据失效，fail-closed）')
    return 3
  }
  process.stdout.write(mine.join('\n') + '\n')
  return 0
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(fileURLToPath(import.meta.url))) {
  process.exitCode = main(process.argv.slice(2))
}
