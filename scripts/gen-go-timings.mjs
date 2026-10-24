#!/usr/bin/env node
/**
 * 重新生成 scripts/go-package-timings.json（issue #1862）。
 *
 * 权重表是 backend-test 分片的**门禁参数**：它不决定「测不测」，但决定「谁和谁同片」，
 * 而分片墙钟的下限就是最慢的那个包（internal/api）。所以数字只能从 CI 日志来，别手改。
 *
 * 用法：
 *   # 1) 找一次 backend-test 的 job id（两片各一个）
 *   gh run view <run> --json jobs --jq '.jobs[] | select(.name|startswith("backend-test")) | .databaseId'
 *   # 2) 把日志取下来（两片的日志**都要**，见下方警告）
 *   gh run view --job <id1> --log > /tmp/bt.log
 *   gh run view --job <id2> --log >> /tmp/bt.log
 *   # 3) 生成（默认只打印，--write 直接写回权重表）
 *   node scripts/gen-go-timings.mjs /tmp/bt.log [--write] [--from "run x / job y"]
 *
 * 解析三种行（同一份日志里三种都会出现）：
 *   ok  \tpkg\t6.832s\tcoverage: 36.2% of statements     ← 正常跑过的包
 *   ?   \tpkg\t[no test files]                            ← 没有测试文件的包
 *   \tpkg\t\tcoverage: 0.0% of statements                  ← -coverprofile 下没有测试文件时
 *
 * ⚠️ **只喂一片的日志会让另一片的包全按 defaultSeconds 计权**（每个分片只跑自己的那部分
 * 包），生成出来的表会把分片算歪。两片日志拼起来；包数明显偏少时脚本会告警（不失败，
 * 拼日志是人负责的事）。
 */
import { readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const HERE = dirname(fileURLToPath(import.meta.url))
const TARGET = resolve(HERE, 'go-package-timings.json')
const DEFAULT_SECONDS = 10
/** 无测试文件的包：只编译 + 报覆盖率，给个象征性权重，别和真跑的包抢位。 */
const NO_TEST_SECONDS = 0.2

/** 从 go test 的日志里抽出「包 → 秒」。返回 { timed, noTest }。 */
export function parseTimings(log) {
  const timed = new Map()
  const noTest = new Set()
  for (const line of log.split('\n')) {
    const ok = line.match(/\sok\s+(\S+)\s+([0-9.]+)s/)
    if (ok) {
      timed.set(ok[1], Math.round(parseFloat(ok[2]) * 10) / 10)
      continue
    }
    const skipped = line.match(/[?\s]\s+(\S+)\s+\[no test files\]/)
    if (skipped) {
      noTest.add(skipped[1])
      continue
    }
    const bare = line.match(/\t(\S+)\t\tcoverage: 0\.0% of statements/)
    if (bare) noTest.add(bare[1])
  }
  return { timed, noTest }
}

/** 组装权重表对象（键序 = 权重降序 + 包名升序，便于人读、也让 diff 稳定）。 */
export function buildTable({ timed, noTest }, { measuredAt, measuredFrom } = {}) {
  const entries = [...timed.entries()]
  for (const pkg of [...noTest].sort()) if (!timed.has(pkg)) entries.push([pkg, NO_TEST_SECONDS])
  entries.sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
  const packages = Object.fromEntries(entries)
  const totalTimed = [...timed.values()].reduce((a, b) => a + b, 0)
  return {
    _comment:
      'backend-test 分片的包级权重（go test 的 ok 行墙钟，单位秒）。判据面在 scripts/go-shard.mjs。'
      + '权重是经验值不是判据：表里没有的包按 defaultSeconds 计权，不会丢；跑得比它慢的新包记得回来补表。'
      + '重新生成用 scripts/gen-go-timings.mjs（别手改数字）。',
    measuredAt: measuredAt ?? new Date().toISOString().slice(0, 10),
    measuredFrom:
      measuredFrom ??
      'go test 日志 ' + timed.size + ' 个含测试的包合计 ' + totalTimed.toFixed(1) + 's；'
        + '0.2 = 无测试文件的包（只编译 + 报覆盖率）。',
    defaultSeconds: DEFAULT_SECONDS,
    packages
  }
}

function main(argv) {
  const write = argv.includes('--write')
  const fromIdx = argv.indexOf('--from')
  const measuredFrom = fromIdx >= 0 ? argv[fromIdx + 1] : undefined
  const logPath = argv.find((a, i) => !a.startsWith('--') && argv[i - 1] !== '--from')
  if (!logPath) {
    console.error('用法: node scripts/gen-go-timings.mjs <go-test 日志> [--write] [--from "run x / job y"]')
    return 2
  }
  const parsed = parseTimings(readFileSync(logPath, 'utf8'))
  if (parsed.timed.size === 0) {
    console.error('错误: ' + logPath + ' 里没有解析到任何 ok 行（不是 go test 的日志？）')
    return 1
  }
  if (parsed.timed.size < 60) {
    console.error(
      '警告: 只解析到 ' + parsed.timed.size + ' 个含测试的包（历史全量是 65 个）。'
        + ' 只喂了一片 backend-test 的日志？两片都要拼进来，否则别的片会按 defaultSeconds 计权。'
    )
  }
  const table = buildTable(parsed, { measuredFrom })
  const json = JSON.stringify(table, null, 2) + '\n'
  if (write) {
    writeFileSync(TARGET, json, 'utf8')
    console.error('已写入 ' + TARGET + '（' + Object.keys(table.packages).length + ' 条）')
  } else {
    process.stdout.write(json)
  }
  return 0
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exitCode = main(process.argv.slice(2))
}
