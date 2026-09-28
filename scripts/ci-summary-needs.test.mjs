// ci-summary 门禁的两条锁（真实缺陷 #2 / spec #1345）：
//
//  ① 依赖闭包：**任何被别的 job 依赖的 job，都必须进 ci-summary 的 needs**。
//     根因：needs 里少一个上游（本次是 changes），上游失败会把下游全变成 skipped，
//     而 skipped 不计入 FAILED ⇒ 门禁在没有跑过任何检查时被判 success。
//  ② 清单相等：ci-summary 的 needs 集合 == check() 调用集合。
//     既登记过的风险：把 job 加进 needs 却漏加 check()，同样会让失败静默变绿。
//
// 只做文本解析（不引入 YAML 依赖），判据面向 YAML 的 job/needs 形状。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const CI = readFileSync(resolve(ROOT, '.github/workflows/ci.yml'), 'utf8')

/** 切出某个顶层 job 的文本块（从 `  name:` 到下一个顶层 job 或文件尾）。 */
function jobBlock(name) {
  const lines = CI.split('\n')
  const start = lines.findIndex(l => l === '  ' + name + ':')
  if (start < 0) throw new Error('ci.yml 里找不到 job: ' + name)
  let end = lines.length
  for (let i = start + 1; i < lines.length; i++) {
    if (/^  [A-Za-z0-9_-]+:\s*$/.test(lines[i])) { end = i; break }
  }
  return lines.slice(start, end).join('\n')
}

/** 列出 ci.yml 的全部顶层 job 名。 */
function allJobs() {
  return CI.split('\n')
    .filter(l => /^  [A-Za-z0-9_-]+:\s*$/.test(l))
    .map(l => l.trim().replace(':', ''))
}

/** 解析一个 job 块里的 needs（支持行内 [a, b] 与块状 - a）。 */
function parseNeeds(block) {
  const inline = block.match(/^\s*needs:\s*\[([^\]]*)\]/m)
  if (inline) {
    return inline[1].split(',').map(s => s.trim()).filter(Boolean)
  }
  const scalar = block.match(/^\s*needs:\s*([A-Za-z0-9_-]+)\s*$/m) // needs: changes
  if (scalar) return [scalar[1]]
  const lines = block.split('\n')
  const idx = lines.findIndex(l => /^\s*needs:\s*$/.test(l))
  if (idx < 0) return []
  const out = []
  for (let i = idx + 1; i < lines.length; i++) {
    const m = lines[i].match(/^\s*-\s*([A-Za-z0-9_-]+)\s*$/)
    if (!m) break
    out.push(m[1])
  }
  return out
}

const summaryBlock = jobBlock('ci-summary')
const summaryNeeds = new Set(parseNeeds(summaryBlock))
const checked = new Set([...summaryBlock.matchAll(/check\s+"[^"]*"\s+"([^"]+)"/g)].map(m => m[1]))

test('ci-summary 依赖闭包：任何被依赖的 job 都必须在汇总 needs 里', () => {
  const dependents = new Map() // 被依赖的 job -> 依赖它的 job 列表
  for (const job of allJobs()) {
    for (const dep of parseNeeds(jobBlock(job))) {
      if (!dependents.has(dep)) dependents.set(dep, [])
      dependents.get(dep).push(job)
    }
  }
  const missing = [...dependents.keys()].filter(dep => !summaryNeeds.has(dep) && dep !== 'ci-summary')
  assert.deepEqual(
    missing, [],
    '这些 job 被别的 job 依赖，却不在 ci-summary 的 needs 里 ⇒ 它们失败会把下游变成 skipped，而汇总仍判绿：' +
      missing.map(m => m + '（被 ' + dependents.get(m).join(', ') + ' 依赖）').join('；')
  )
})

test('ci-summary 清单相等：needs 集合 == check() 调用集合', () => {
  const needsOnly = [...summaryNeeds].filter(n => !checked.has(n)).sort()
  const checkOnly = [...checked].filter(n => !summaryNeeds.has(n)).sort()
  assert.deepEqual(needsOnly, [], '在 needs 里但没有 check()：' + needsOnly.join(', '))
  assert.deepEqual(checkOnly, [], '有 check() 但不在 needs 里：' + checkOnly.join(', '))
})

test('ci-summary 的 skipped 语义：changes 必须被判红（不是只打印跳过）', () => {
  // changes 是「上游」：它失败 ⇒ 下游全 skipped。若它不在汇总清单里，门禁就没有判红入口。
  assert.ok(summaryNeeds.has('changes'), 'changes 必须在 ci-summary.needs 里')
  assert.ok(checked.has('changes'), 'changes 必须被 check() 调用，否则 skipped/cancelled 仍会被当成通过')
})
