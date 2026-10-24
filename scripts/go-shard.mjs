#!/usr/bin/env node
/**
 * backend-test 的按包分片器（LPT 装箱，issue #1862）。
 *
 * 为什么要分片：`go test ./...` 在 4 核 runner 上把 948s 的包级工压到 525s 墙钟，而
 * `internal/api` 一个包就占 400s —— 分片的收益下限就是这 400s，切分的意义只在「别让
 * 这 400s 之外的时间还排队」：一片跑 api（400s），另一片跑其余 64 个包（合计 548s，
 * 4 核并行约 125s 墙钟）。所以片数不是越多越好 —— k=2/3/4 的墙钟都卡在 400s。
 *
 * 为什么是 LPT 而不是 `NR % k` 轮转：轮转不看单包耗时，最慢包会和一堆中包叠在同一片
 * （移动端 #1715 的同款教训，实测四片 1:29 / 1:15 / 2:36 / 3:39）。按实测权重降序装箱后
 * 两片负载 ≈ 475s / 474s。
 *
 * 权重表 `scripts/go-package-timings.json` 是**权重不是判据**：表里没有的包按
 * defaultSeconds 计权（照样会被排进某一片，不会丢，只是可能分得不均）；新增的慢包
 * 记得回来补一行。
 *
 * 用法：
 *   go list ./... > /tmp/all-packages.txt
 *   node scripts/go-shard.mjs --shard 1 --shards 2 \
 *     --list /tmp/all-packages.txt --weights ../scripts/go-package-timings.json
 * 输出：本片的包路径，一行一个（CI 里交给 `mapfile -t` 再 `go test "${PKGS[@]}"`）。
 * 退出码：0 正常；2 参数错；3 本片取不到任何包（fail-closed —— 空片绝不许当绿）。
 */
import { readFileSync } from 'node:fs'
import { isDirectRun } from './lib/guard.mjs'

export const EXIT_BAD_ARGS = 2
export const EXIT_EMPTY_SHARD = 3
export const DEFAULT_SECONDS = 10

/**
 * LPT（Longest Processing Time first）装箱：按权重降序，逐个放进当前负载最小的片。
 * 并列时取下标小的片 ⇒ 结果与输入顺序无关（自检里钉住）。
 *
 * @param {string[]} packages 包路径（重复项先去重）
 * @param {Record<string, number>} weights 包路径 → 秒
 * @param {number} shards 片数
 * @param {number} defaultSeconds 表里没有的包的权重
 * @returns {string[][]} 长度恒为 shards（包比片少时后面的片就是空数组，由调用方 fail-closed）
 */
export function packShards(packages, weights, shards, defaultSeconds = DEFAULT_SECONDS) {
  if (!Number.isInteger(shards) || shards < 1) {
    throw new Error(`片数必须是正整数，收到 ${shards}`)
  }
  const table = weights && typeof weights === 'object' ? weights : {}
  const uniq = [...new Set(packages.map((p) => String(p).trim()).filter(Boolean))]
  const weighted = uniq.map((pkg) => {
    const sec = Number(table[pkg])
    return { pkg, sec: Number.isFinite(sec) && sec >= 0 ? sec : defaultSeconds }
  })
  weighted.sort((a, b) => b.sec - a.sec || a.pkg.localeCompare(b.pkg))

  const bins = Array.from({ length: shards }, () => [])
  const load = new Array(shards).fill(0)
  for (const { pkg, sec } of weighted) {
    let target = 0
    for (let i = 1; i < shards; i++) {
      if (load[i] < load[target]) target = i
    }
    bins[target].push(pkg)
    load[target] += sec
  }
  return bins
}

/** 解析 `--shard N --shards K --list PATH --weights PATH`；缺项/未知项一律抛错（调用方转 exit 2）。 */
export function parseShardArgs(argv) {
  const spec = { shard: '--shard', shards: '--shards', list: '--list', weights: '--weights' }
  const out = {}
  for (let i = 0; i < argv.length; i++) {
    const flag = argv[i]
    const key = Object.keys(spec).find((k) => spec[k] === flag)
    if (!key) throw new Error(`未知参数 ${flag}`)
    const value = argv[++i]
    if (value === undefined) throw new Error(`${flag} 缺少取值`)
    out[key] = value
  }
  for (const key of Object.keys(spec)) {
    if (out[key] === undefined) throw new Error(`缺少参数 ${spec[key]}`)
  }
  const shard = Number(out.shard)
  const shards = Number(out.shards)
  if (!Number.isInteger(shards) || shards < 1) throw new Error(`--shards 必须是正整数，收到 ${out.shards}`)
  if (!Number.isInteger(shard) || shard < 1 || shard > shards) {
    throw new Error(`--shard 必须在 1..${shards} 之间，收到 ${out.shard}`)
  }
  return { shard, shards, list: out.list, weights: out.weights }
}

function main(argv) {
  let args
  try {
    args = parseShardArgs(argv)
  } catch (e) {
    console.error(`错误: ${e.message}`)
    return EXIT_BAD_ARGS
  }
  const list = readFileSync(args.list, 'utf8')
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean)
  if (list.length === 0) {
    console.error(`错误: 包清单 ${args.list} 是空的（go list 没取到包 ⇒ 判据失效）`)
    return EXIT_BAD_ARGS
  }
  const table = JSON.parse(readFileSync(args.weights, 'utf8'))
  const bins = packShards(list, table.packages, args.shards, table.defaultSeconds ?? DEFAULT_SECONDS)
  const mine = bins[args.shard - 1]
  if (mine.length === 0) {
    console.error(`错误: 第 ${args.shard}/${args.shards} 片取不到任何包（${list.length} 个包都分到了别的片）`)
    return EXIT_EMPTY_SHARD
  }
  process.stdout.write(mine.join('\n') + '\n')
  return 0
}

if (isDirectRun(import.meta.url)) {
  process.exitCode = main(process.argv.slice(2))
}
