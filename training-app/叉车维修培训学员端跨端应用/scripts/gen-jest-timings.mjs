#!/usr/bin/env node
/**
 * 重新生成 utils/jestSuiteTimings.json（③ 门分片配平用的逐套件耗时表，#1715）
 *
 * ## 两种口径，别混用
 *
 * - `--log <file>`（**CI 口径，推荐**）：file = `gh run view <ci run id> --log` 的输出。
 *   jest 对**慢套件**才打印 `PASS utils/x.test.js (12.3 s)`，故这份表天然只含慢套件；
 *   其余套件由 `--default-seconds`（默认 1s）代表。表里没列到的套件在分片器里按默认权重计。
 * - `--json <file>`（**本机口径，慎用**）：file = 本机 `jest --json --outputFile` 的产物，含**全部**
 *   套件精确耗时。⚠️ 本机数字与 CI 不可混用：实测同一套件（`utils/mpWeixinGateResultLineBehavior.test.js`）
 *   本机 503s、CI 里根本进不了 ≥5s 名单（本机受会话并发 / 杀毒 / 磁盘影响），拿它当 CI 权重会把片切歪。
 *
 * ## 为什么要脚本而不是手改
 *
 * 权重是**判据输入**：手抄会漂且无来源，重算一次就能对齐口径。生成的 JSON 带 `note` 记采集来源。
 *
 * 运行：
 *   gh run view <id> --log > .ci-verify/ci.log
 *   node scripts/gen-jest-timings.mjs --log .ci-verify/ci.log
 */
import { readFileSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const DEFAULT_OUT = 'utils/jestSuiteTimings.json'

function parseArgs(argv) {
  const out = { log: '', json: '', out: DEFAULT_OUT, minSeconds: 5, defaultSeconds: 1, source: '' }
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i]
    const next = () => {
      const v = argv[i + 1]
      if (v === undefined) throw new Error(`${a} 缺少取值`)
      i += 1
      return v
    }
    if (a === '--log') out.log = next()
    else if (a === '--json') out.json = next()
    else if (a === '--out') out.out = next()
    else if (a === '--min-seconds') out.minSeconds = Number(next())
    else if (a === '--default-seconds') out.defaultSeconds = Number(next())
    else if (a === '--source') out.source = next()
    else throw new Error(`未知参数 ${a}`)
  }
  if (!out.log && !out.json) throw new Error('必须给 --log 或 --json')
  if (out.log && out.json) throw new Error('--log 与 --json 只能给一个')
  return out
}

/** 从 `gh run view --log` 里抽 `PASS <路径> (N s)` 的慢套件 */
export function parseCiLog(text, minSeconds) {
  const suites = {}
  const re = /(\S+\.test\.[jt]sx?)\)? \((\d+(?:\.\d+)?) s\)/
  for (const line of text.split(/\r?\n/)) {
    if (!/\bPASS\b/.test(line)) continue
    const m = line.match(re)
    if (!m) continue
    const key = m[1].replace(/^.*?((?:utils|pages|api|components|composables|scripts)\/)/, '$1')
    const sec = Number(m[2])
    if (!(sec >= minSeconds)) continue
    suites[key] = Math.max(suites[key] ?? 0, sec)
  }
  return suites
}

/** 从 jest --json 产物里抽每个套件的墙钟（秒） */
export function parseJestJson(text, minSeconds) {
  const data = JSON.parse(text)
  const suites = {}
  for (const r of data.testResults ?? []) {
    const key = String(r.name).replace(/\\/g, '/').replace(/^.*?((?:utils|pages|api|components|composables|scripts)\/)/, '$1')
    const sec = (Number(r.endTime) - Number(r.startTime)) / 1000
    if (!Number.isFinite(sec) || !(sec >= minSeconds)) continue
    suites[key] = Math.round(sec * 1000) / 1000
  }
  return suites
}

function main(argv) {
  let args
  try {
    args = parseArgs(argv)
  } catch (e) {
    console.error(`gen-jest-timings: ${e.message}`)
    return 2
  }
  let suites
  let source = args.source
  try {
    if (args.log) {
      suites = parseCiLog(readFileSync(args.log, 'utf8'), args.minSeconds)
      source = source || `CI 日志（gh run view --log，只含 ≥${args.minSeconds}s 的慢套件）`
    } else {
      suites = parseJestJson(readFileSync(args.json, 'utf8'), args.minSeconds)
      source = source || `本机 jest --json（⚠️ 与本机环境耦合，勿直接替代 CI 口径）`
    }
  } catch (e) {
    console.error(`gen-jest-timings: 解析失败：${e.message}`)
    return 2
  }
  const keys = Object.keys(suites).sort((a, b) => suites[b] - suites[a] || a.localeCompare(b))
  if (keys.length === 0) {
    console.error(`gen-jest-timings: 一个 ≥${args.minSeconds}s 的套件都没抽到（判据失效，不写文件）`)
    return 3
  }
  const ordered = {}
  for (const k of keys) ordered[k] = suites[k]
  const payload = {
    schema: 1,
    note: `逐套件耗时（秒），用于 ③ 门分片配平（#1715）。采集来源：${source}。改这里请同时改 scripts/jest-fanout.mjs 的注释与 scripts/jest-fanout.test.mjs 的自测用例。表里没列到的套件按 defaultSeconds 计权。`,
    defaultSeconds: args.defaultSeconds,
    suites: ordered,
  }
  writeFileSync(args.out, JSON.stringify(payload, null, 2) + '\n', 'utf8')
  const sum = keys.reduce((a, k) => a + ordered[k], 0)
  console.error(`[gen-jest-timings] 写入 ${args.out}：${keys.length} 个慢套件，合计 ${sum.toFixed(1)}s，最慢 ${keys[0]} ${ordered[keys[0]]}s`)
  return 0
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(fileURLToPath(import.meta.url))) {
  process.exitCode = main(process.argv.slice(2))
}
