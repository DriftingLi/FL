#!/usr/bin/env node
/**
 * dev-close 的判定层（#2174）——「收尾」这件事里**唯一该被测试**的部分住在这里。
 *
 * 为什么拆成 node 判定 + pwsh 执行：CI 全线 ubuntu（现测 ci.yml 所有 runs-on），守护不可能去 spawn
 * PowerShell 真跑删树；而形态先例是 preflight-pr.ps1（薄桥）+ .github/scripts/lib/gate-validator.mjs
 * （判据）——本文件照这个分工：**判据在这、副作用在 ps1**，ps1 只按本文件给的旗标走。
 *
 * 判据权威不在这里重写：删树顺序与「目录/注册/分支」三查仍属 remove-worktree.ps1，
 * 远端分支残留判定仍属 check-branch-residue.ps1。本文件只回答三个问题：
 *   ① 这棵树现在准不准拆（未合并 / 读不到 PR 状态 ⇒ 不拆，fail-closed）；
 *   ② 这次要不要把 -Execute 透传下去（只有 -Yes 才透传）；
 *   ③ 跑完之后这次收尾算不算干净（把两侧读数汇成一行 DEV_CLOSE_RESULT）。
 *
 * 机检行纯 ASCII（承 #2139 编码自律）：中文进结论行会被 OEM 码页采集链烤成 U+FFFD，
 * 而 #2162 的教训是结论行必须**在行首**且带 key=value，否则等于没读数。
 */

/** 文档里「scripts/*.ps1」这类命令引用的捕获面（L1 幻影引用守护用同一把尺）。 */
import { pathToFileURL } from 'node:url'

export const SCRIPT_REF_RE = /\/?scripts\/[A-Za-z0-9_.-]+\.ps1/g

/** 目标树名由票号唯一决定：一票一树一分支，不给通配留口子。 */
export function worktreeName(task) {
  const t = String(task ?? '').trim()
  if (!/^[0-9]+$/.test(t)) throw new Error(`dev-close 只接票号（纯数字），收到:${JSON.stringify(task)}`)
  return `wt-${t}`
}

/** git worktree list --porcelain → 该树是否在册 + 绝对路径 */
export function parseWorktreeList(text) {
  const out = new Map()
  let cur = null
  for (const raw of String(text ?? '').split(/\r?\n/)) {
    const line = raw.trimEnd()
    if (line.startsWith('worktree ')) cur = line.slice(9).trim()
    else if (line.startsWith('branch ') && cur) { out.set(cur, line.slice(7).trim()); cur = null }
    else if (line === '') cur = null
  }
  return out
}

/** 找到本票那棵树：按分支 refs/heads/feat/<task> 认，不按目录名猜（目录可被改名，分支不会） */
export function findTargetTree(worktreeListText, task) {
  const ref = `refs/heads/feat/${String(task).trim()}`
  for (const [path, branch] of parseWorktreeList(worktreeListText)) {
    if (branch === ref) return { path, branch, registered: true }
  }
  return { path: '', branch: ref, registered: false }
}

/** 未合并活：origin/master..HEAD 有提交就算有活，绝不能拆（squash 仓里这条只能问 git，不能问 patch-id） */
export function hasUnmergedCommits(logText) {
  return String(logText ?? '').split(/\r?\n/).some((l) => l.trim() !== '')
}

/** gh pr list --json state 的读数；读不到 = unknown（不是「没有 PR」） */
export function parsePrState(prJsonText) {
  try {
    const arr = JSON.parse(String(prJsonText ?? '').trim() || '[]')
    if (!Array.isArray(arr) || arr.length === 0) return 'NONE'
    const s = String(arr[0].state || '').toUpperCase()
    return ['MERGED', 'OPEN', 'CLOSED'].includes(s) ? s : 'unknown'
  } catch {
    return 'unknown'
  }
}

/**
 * ① + ②：准不准拆 / 要不要 -Execute。
 * fail-closed 三处：unknown、NONE、OPEN/CLOSED —— 一律不拆。读不到 PR 状态**不是**放行理由。
 */
export function decide(f) {
  const task = String(f.task ?? '').trim()
  const target = worktreeName(task)
  // 存在性与「站不站在要删的树里」都在这里算，ps1 不参与判断（判据一处，不在两侧各写一遍）
  const found = findTargetTree(f.worktreeList ?? '', task)
  const treeExists = found.registered
  const normPath = (s) => String(s).replace(/[\\/]+$/, '').replace(/\\/g, '/').toLowerCase()
  const cwd = normPath(f.cwd ?? '')
  const targetPath = normPath(found.path || '')
  const execute = Boolean(f.execute)
  const prState = f.prState || parsePrState(f.prJson)
  const base = { task, target, treePath: found.path, prState, execute, passExecute: false, skipRemove: true, allow: false, reason: '' }

  if (targetPath && cwd && (cwd === targetPath || cwd.startsWith(targetPath + '/'))) {
    // 站在要删的树里删自己的树：删完 cwd 失效，后面每一步都读不到东西 —— 先让人离开
    return { ...base, reason: 'cwd-inside-target', skipRemove: true, passExecute: false }
  }

  if (!treeExists) {
    // 幂等腿：树已经没了（重复跑 / 已手工拆过）⇒ 不报错、不发起删除，但分支侧照核
    return { ...base, allow: true, reason: 'tree-absent', skipRemove: true, passExecute: false }
  }
  if (prState !== 'MERGED') {
    return { ...base, reason: prState === 'unknown' ? 'pr-state-unreadable' : 'pr-not-merged', skipRemove: true, passExecute: false }
  }
  if (hasUnmergedCommits(f.unmergedLog ?? '')) {
    return { ...base, reason: 'unmerged-commits', skipRemove: true, passExecute: false }
  }
  return { ...base, allow: true, reason: 'ok', skipRemove: false, passExecute: execute }
}

/** remove-worktree.ps1 的出口 JSON（后面还跟着 ✅/持有者行）；读不回来显式标 unknown，不当成已删除 */
export function parseRemoveJson(stdoutText) {
  const s = String(stdoutText ?? '')
  const m = s.match(/\{[\s\S]*\}/)
  if (!m) return { unknown: true }
  try {
    const o = JSON.parse(m[0])
    return {
      dirRemoved: Boolean(o.dirRemoved),
      regRemoved: Boolean(o.regRemoved),
      branchRemoved: Boolean(o.branchRemoved),
      worktree: o.worktree || '',
    }
  } catch {
    return { unknown: true }
  }
}

function ternary(flag, unknown) {
  if (unknown) return 'unknown'
  if (flag === true) return 'ok'
  if (flag === false) return 'no'
  return 'na'
}

/** ③：两侧读数汇成一行机检。dry-run 永远只报 planned，绝不声称已删除。 */
export function judgeVerdict(f) {
  const r = f.remove ?? {}
  const res = f.residue ?? {}
  const mode = f.mode === 'execute' ? 'execute' : 'dryrun'

  let verdict = 'ok'
  let reason = ''
  if (f.blockedReason) { verdict = 'blocked'; reason = f.blockedReason }
  else if (mode === 'dryrun') { verdict = 'planned' }
  else if (r.unknown || r.exitCode !== 0 || r.dirRemoved === false || r.regRemoved === false) { verdict = 'red'; reason = 'tree-leftover' }
  else if (res.ran && res.exitCode !== 0) { verdict = 'red'; reason = 'branch-residue' }

  const tree = f.treeBefore === 'absent' ? 'absent' : (verdict === 'ok' ? 'removed' : mode === 'dryrun' ? 'todo' : 'kept')
  const parts = [
    'DEV_CLOSE_RESULT',
    `task=${f.task}`,
    `pr=${f.prState || 'unknown'}`,
    `mode=${mode}`,
    `fetch=${f.fetch || 'skipped'}`,
    `tree=${tree}`,
    `dir=${ternary(r.dirRemoved, r.unknown)}`,
    `reg=${ternary(r.regRemoved, r.unknown)}`,
    `local_branch=${r.branchRemoved ? 'removed' : (r.unknown ? 'unknown' : 'kept')}`,
    `remote_branch=${res.state || 'na'}`, ...(reason ? [`reason=${reason}`] : []),
    `verdict=${verdict}`,
  ]
  return { line: parts.join(' '), ok: verdict === 'ok' || verdict === 'planned', verdict, reason }
}

/**
 * stdin 里的 facts 解析。剥 BOM 不是洁癖：#2174 实测 PowerShell 把
 * `$OutputEncoding = [Text.Encoding]::UTF8` 设上之后，往**子进程 stdin** 写字符串会带 U+FEFF，
 * JSON.parse 当场 `Unexpected token '\uFEFF'` —— 于是整条链判红。修法两侧都做：ps1 侧改投无 BOM 的
 * UTF8Encoding，这里也照样剥掉（读 stdin 的 CLI 不该指望上游不带 BOM）。
 */
export function parseFacts(text) {
  const s = String(text ?? '').replace(/^\uFEFF/, '').trim()
  return JSON.parse(s || '{}')
}

async function readStdin() {
  const chunks = []
  for await (const c of process.stdin) chunks.push(c)
  return Buffer.concat(chunks).toString('utf8')
}

const invokedDirectly = process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href
if (invokedDirectly) {
  const modeArg = process.argv[2]
  const raw = await readStdin()
  let facts
  try { facts = parseFacts(raw) } catch {
    console.log('DEV_CLOSE_RESULT task=? verdict=red reason=facts-unreadable')
    process.exit(2)
  }
  if (modeArg === '--decide') {
    const d = decide(facts)
    console.log(`DECIDE allow=${d.allow} pr=${d.prState} skip_remove=${d.skipRemove} pass_execute=${d.passExecute} reason=${d.reason} target=${d.target} tree_path=${d.treePath}`)
    process.exit(d.allow ? 0 : 1)
  }
  if (modeArg === '--verdict') {
    const v = judgeVerdict(facts)
    console.log(v.line)
    process.exit(v.ok ? 0 : 1)
  }
  console.log('用法: node dev-close.mjs --decide|--verdict < facts.json')
  process.exit(2)
}
