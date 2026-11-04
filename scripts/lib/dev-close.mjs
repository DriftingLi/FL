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

/**
 * git worktree list --porcelain → 每个条目一条记录。
 * 条目里没有 branch 行就是 detached（基座树 wtx-base 那种）：它不是任何票号的树，
 * 但**必须留下痕迹**——否则「目录在、判据没匹配上」会退化成「不存在」，#2247 正是要分开这两态。
 */
export function parseWorktreeEntries(text) {
  const out = []
  let cur = null
  for (const raw of String(text ?? '').split(/\r?\n/)) {
    const line = raw.trimEnd()
    if (line.startsWith('worktree ')) { cur = { path: line.slice(9).trim(), branch: '', detached: false }; out.push(cur) }
    else if (line.startsWith('branch ') && cur) cur.branch = line.slice(7).trim()
    else if (line === 'detached' && cur) cur.detached = true
    else if (line === '') cur = null
  }
  return out
}

/** 兼容形：路径 → 登记分支（detached 条目记空串，不再整条消失） */
export function parseWorktreeList(text) {
  const out = new Map()
  for (const e of parseWorktreeEntries(text)) out.set(e.path, e.branch)
  return out
}

function baseName(p) {
  return String(p ?? '').replace(/[\\/]+$/, '').split(/[\\/]/).pop().toLowerCase()
}

/** `feat/2189` 与 `refs/heads/feat/2189` 都是同一分支的写法，用户与 git 各用各的 */
export function toRef(name) {
  const s = String(name ?? '').trim()
  if (!s) return ''
  return s.startsWith('refs/heads/') ? s : `refs/heads/${s}`
}

export function shortRef(ref) {
  return String(ref ?? '').replace(/^refs\/heads\//, '')
}

/**
 * 树身份解析（#2247，spec B / #2233）——本文件里**唯一**的认树判据。
 *
 * 判据与来路（为什么不再按 `feat/<票号>` 认、血账 `wt-2189`、回退腿退役条件）见
 * docs/adr/ADR-0092-树身份-目录为入口与登记分支为身份.md —— 这里只留一句：入口看注册表路径名
 * `wt-<票号>`，身份看该条目登记的分支，分支形态由闸门 new-worktree.ps1 -Branch 决定。
 *
 * 五态由下方返回值逐态给出（`registered` / `legacy` / `tree-branch-mismatch` / `unregistered-tree` /
 * `absent`），每态行内带可 grep 的 reason；用例见同目录 ../dev-close.test.mjs。
 */
export function resolveTree(f) {
  const task = String(f.task ?? '').trim()
  const target = worktreeName(task)
  const want = target.toLowerCase()
  const entries = parseWorktreeEntries(f.worktreeList ?? '')
  const argRef = toRef(f.branch)
  const dirExists = Boolean(f.treeDirExists)
  const wantLegacy = `refs/heads/feat/${task}`

  let hit = null
  let detachedHit = null
  for (const e of entries) {
    if (baseName(e.path) !== want) continue
    if (e.branch) { hit = e; break }
    if (!detachedHit) detachedHit = e
  }
  const shape = (state, path, identityBranch, mismatch) => ({
    state, path, identityBranch, mismatch, legacy: state === 'legacy',
    dirExists, target, reason: state === 'registered' || state === 'legacy' ? (mismatch ? 'branch-mismatch' : '') : (state === 'absent' ? '' : state),
    // 无树时没有身份可言，只剩「按惯例该叫什么」这一条可核：分支残留核对照旧核它（fail-closed 不放松），
    // 但这只是探测目标、不是判定依据 —— 认树判据仍然只有上面那两级。
    probeBranch: identityBranch || wantLegacy,
  })

  if (hit) return shape('registered', hit.path, hit.branch, Boolean(argRef) && argRef !== hit.branch)
  if (detachedHit) return shape('tree-branch-mismatch', detachedHit.path, '', false)

  // 野树先于回退腿：磁盘上立着 wt-<票号> 却没人注册它，它不是「老形态的树」，是没人管的目录
  if (dirExists) return shape('unregistered-tree', '', '', false)

  // 回退腿 = 旧判据原样保留（按 feat/<票号> 分支在注册表里认），只宽一期、只打 WARN
  for (const e of entries) {
    if (e.branch === wantLegacy) return shape('legacy', e.path, e.branch, Boolean(argRef) && argRef !== e.branch)
  }
  return shape('absent', '', '', false)
}

/** 找到本票那棵树：主判据（目录名）命中、否则回退腿；两者都没有就是没树 */
export function findTargetTree(worktreeListText, task) {
  const r = resolveTree({ task, worktreeList: worktreeListText })
  const matched = r.state === 'registered' || r.state === 'legacy'
  return { path: r.path, branch: r.identityBranch || `refs/heads/feat/${String(task).trim()}`, registered: matched }
}

/**
 * 未合并活：**PR 头之后**还有提交就算有活，绝不能拆。
 * 不许拿 origin/master..HEAD 当判据 —— 本仓走 squash 合并，功能分支的提交永远不是 master 祖先，
 * 那样判在每一条已合并的 PR 上都会是「有未合并提交」（#2174 dogfood 实测踩中）。
 */
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
 * 认树／身份一律走 resolveTree（判据只有一份）；`f.branch` 是调用方的 -Branch 覆盖，只与身份比对、绝不取代它。
 */
export function decide(f) {
  const task = String(f.task ?? '').trim()
  const target = worktreeName(task)
  const tree = resolveTree({ task, worktreeList: f.worktreeList ?? '', branch: f.branch, treeDirExists: f.treeDirExists })
  const treeExists = tree.state === 'registered' || tree.state === 'legacy'
  const normPath = (s) => String(s).replace(/[\\/]+$/, '').replace(/\\/g, '/').toLowerCase()
  const cwd = normPath(f.cwd ?? '')
  const targetPath = normPath(tree.path || '')
  const execute = Boolean(f.execute)
  const prState = f.prState || parsePrState(f.prJson)
  const base = {
    task, target, treePath: tree.path, treeState: tree.state, branch: tree.identityBranch,
    legacy: tree.legacy, prState, execute, passExecute: false, skipRemove: true, allow: false, reason: '',
  }

  if (targetPath && cwd && (cwd === targetPath || cwd.startsWith(targetPath + '/'))) {
    // 站在要删的树里删自己的树：删完 cwd 失效，后面每一步都读不到东西 —— 先让人离开
    return { ...base, reason: 'cwd-inside-target', skipRemove: true, passExecute: false }
  }

  if (!treeExists) {
    // 两种「没树」必须分家（#2247 的血账就是它们被揉成一态、静默空转还被读成达成）：
    //   目录在而判据没匹配上（detached / 野树）⇒ 判红点名，别让人以为已经收完了；
    //   真不存在 ⇒ 幂等腿放行，但不发起删除。
    if (tree.state === 'absent') return { ...base, allow: true, reason: 'tree-absent', skipRemove: true, passExecute: false }
    return { ...base, reason: tree.reason || tree.state, skipRemove: true, passExecute: false }
  }
  // -Branch 覆盖与树身份冲突：不猜、不二选一，直接判红让人看清楚（错配的收尾比没做更坏）
  if (tree.mismatch) return { ...base, reason: 'branch-mismatch', skipRemove: true, passExecute: false }
  if (prState !== 'MERGED') {
    return { ...base, reason: prState === 'unknown' ? 'pr-state-unreadable' : 'pr-not-merged', skipRemove: true, passExecute: false }
  }
  if (hasUnmergedCommits(f.unmergedLog ?? '')) {
    return { ...base, reason: 'unmerged-commits', skipRemove: true, passExecute: false }
  }
  return { ...base, allow: true, reason: 'ok', skipRemove: false, passExecute: execute }
}

/**
 * remove-worktree.ps1 的出口 JSON（后面还跟着 ✅/持有者行）；读不回来显式标 unknown，不当成已删除。
 * #2188 接线坑：execute 腿连着打 plan（remove-worktree.ps1:391）与 result（:461）两个 ConvertTo-Json 块，
 * 原先贪婪的 `\{[\s\S]*\}` 从 plan 的第一个 { 吞到 result 的最后一个 } —— 整块 parse 必失败、恒 unknown。
 * 改成按花括号配对切候选、从后往前扫：result 才是事实（含 dirRemoved），plan 只是意图，跳过。
 */
export function parseRemoveJson(stdoutText) {
  const s = String(stdoutText ?? '')
  const cands = s.match(/\{(?:[^{}]|\{(?:[^{}]|\{[^{}]*\})*\})*\}/g) || []
  for (let i = cands.length - 1; i >= 0; i--) {
    let o = null
    try { o = JSON.parse(cands[i]) } catch { continue }
    if (o && typeof o === 'object' && 'dirRemoved' in o) {
      return {
        dirRemoved: Boolean(o.dirRemoved),
        regRemoved: Boolean(o.regRemoved),
        branchRemoved: Boolean(o.branchRemoved),
        worktree: o.worktree || '',
      }
    }
  }
  return { unknown: true }
}

/**
 * check-branch-residue.ps1 在 -Branch 形态下的出口分类（#2188 主症：原先一个 `exitCode !== 0`
 * 把 1/2/3/4/5 揉成同一条 branch-residue —— 而 4 是「分支侧已绿、只是仓里躺着别的会话的孤儿目录」，
 * 于是真拆树那次必假红）。出口码归脚本管、这里是唯一读码处；raw 的固定锚行只供读数 token。
 * 读不到的一律 unknown，fail-closed 的教条不变：不可读永远不当成干净。
 */
export function parseResidueText(stdoutText, exitCode) {
  const s = String(stdoutText ?? '')
  const out = {}
  if (/^\s*ABSENT\s+\S+/m.test(s)) out.state = 'absent'
  else if (/^\s*RESIDUE\s+\S+/m.test(s)) out.state = 'residue'
  else if (/^\s*NEED-HUMAN\s+\S+/m.test(s)) out.state = 'need-human'
  else if (/^\s*OPEN\s+\S+/m.test(s)) out.state = 'open'
  else if (/^\s*ARCHIVED\s+\S+/m.test(s)) out.state = 'archived'
  else if (exitCode === 0 || exitCode === 4 || exitCode === 5) out.state = 'clean'
  else if (exitCode === 1) out.state = 'residue'
  else out.state = 'unknown'
  const d = s.match(/WT-DIRS registered=\d+ disk=\d+ orphans=(\d+)/)
  if (d) { out.orphans = Number(d[1]); out.dirs = out.orphans > 0 ? 'orphan' : 'clean' }
  else if (/WT-DIRS=(unavailable|unclaimed)/.test(s)) out.dirs = 'unavailable'
  else out.dirs = 'na'
  return out
}

function ternary(flag, unknown) {
  if (unknown) return 'unknown'
  if (flag === true) return 'ok'
  if (flag === false) return 'no'
  return 'na'
}

/** ③：两侧读数汇成一行机检。dry-run 永远只报 planned，绝不声称已删除。 */
export function judgeVerdict(f) {
  const mode = f.mode === 'execute' ? 'execute' : 'dryrun'
  // 生产接线只交 {ran,exitCode,raw}（dev-close.ps1 的 facts），预解析字段是测试与旧调用的便捷形：
  // raw 只补 facts 里缺的那几个字段，两种形都走同一条判定链（#2188 的教训：fixture 形不等于生产形）。
  const r = { ...(f.remove ?? {}) }
  if (mode === 'execute' && r.ran && r.raw != null && r.dirRemoved === undefined) Object.assign(r, parseRemoveJson(r.raw))
  const res = { ...(f.residue ?? {}) }
  if (res.ran && res.raw != null && res.state === undefined) Object.assign(res, parseResidueText(res.raw, res.exitCode))

  let verdict = 'ok'
  let reason = ''
  if (f.blockedReason) { verdict = 'blocked'; reason = f.blockedReason }
  else if (mode === 'dryrun') { verdict = 'planned' }
  else if (r.ran && (r.unknown || r.exitCode !== 0 || r.dirRemoved === false || r.regRemoved === false)) { verdict = 'red'; reason = 'tree-leftover' }
  else if (res.ran && res.exitCode !== 0) {
    // 出口码分家（码义在 check-branch-residue.ps1 的 Exit-Green / exit 定义处）：
    // 1=真有残留；4=分支侧已绿、只是仓里躺着别的会话的孤儿目录 —— 并发边界：本票只核本票的分支，
    // 孤儿目录不进本票的红、进 wt_dirs 读数供人处置；5=目录对账读不到，fail-closed 仍红但 reason 分家；2/3=读不到。
    if (res.exitCode === 1) { verdict = 'red'; reason = 'branch-residue' }
    else if (res.exitCode === 5) { verdict = 'red'; reason = 'wt-audit-unavailable' }
    else if (res.exitCode !== 4) { verdict = 'red'; reason = 'branch-check-failed' }
  }
  // 「无动作」与「干完了」必须长得不一样（#2251 / spec B）：执行腿上树本来就不存在 ⇒ 这趟什么都没拆，
  // 不许报 ok（ok 只留给真拆且对账通过的腿）。noop 不是红（重跑一张已收完的票是常态），但也不再是「达成」。
  // 写成「红分支链之后的独立判定」而不是链尾的 else if：出口码 4（分支侧已绿、仓里躺着别的会话的孤儿目录）
  // 会进上一个 else if 却三个子分支都不设 verdict，verdict 仍是 ok —— 链尾 else if 就此被短路，
  // 树不在的执行腿又报回 ok（#2251 code-review 实测）。条件收成 verdict === 'ok'，红的照样红。
  if (verdict === 'ok' && f.treeBefore === 'absent') { verdict = 'noop' }

  // 三个 token 一律从事实来，不再从 verdict 反推（#2188 的三连漂：tree=kept、dir=na、local_branch=kept 都是编的）。
  const treeDone = r.dirRemoved === true && r.regRemoved === true
  const tree = f.treeBefore === 'absent' ? 'absent' : mode === 'dryrun' ? 'todo'
    : treeDone ? 'removed' : r.unknown ? 'unknown' : verdict === 'ok' ? 'removed' : 'kept'
  const lb = r.branchRemoved === true ? 'removed'
    : r.ran && r.unknown ? 'unknown'
    : f.localBranch === 'kept' ? 'kept'
    : f.localBranch === 'absent' ? 'absent'
    : r.ran ? 'kept' : 'na'
  const dirs = !res.ran ? 'na' : res.dirs === 'orphan' ? `orphan:${res.orphans ?? 0}` : (res.dirs || 'na')
  // 树身份进读数行（#2247）：读过一行就知道这次收的是哪条分支，不必回头翻注册表。
  // 行是纯 ASCII（#2139）：分支名本身 ASCII，读不到就是 na —— 中文若进这一行会被采集链烤成 U+FFFD。
  const branch = shortRef(f.branch) || 'na'
  // 无动作永不打 ok（spec B）：absent + dry-run 的组合行必须自带一句「本次无任何待执行动作」，
  // 否则「什么都没干」与「干完了」在读数上长得一样 —— 旧判据静默空转被读成达成正是这个形状。
  // 中文不进机检行，用固定 ASCII token 承载，人读的中文由 ps1 侧照打。
  const notes = []
  if (f.legacy) notes.push('legacy-branch-match')
  // note 只属于「确实没事可做」的两态（dry-run 的 planned 与执行腿的 noop）。
  // 曾经按「凡 tree=absent」挂，结果 branch-residue 的红行也带上「无任何待执行动作」——
  // 有残留就是有动作要做，红行贴这句等于自认没活干（#2251 code-review 实测）。
  if (tree === 'absent' && (verdict === 'planned' || verdict === 'noop')) notes.push('no-pending-action')
  const parts = [
    'DEV_CLOSE_RESULT',
    `task=${f.task}`,
    `pr=${f.prState || 'unknown'}`,
    `mode=${mode}`,
    `fetch=${f.fetch || 'skipped'}`,
    `tree=${tree}`,
    `branch=${branch}`,
    `dir=${ternary(r.dirRemoved, r.unknown)}`,
    `reg=${ternary(r.regRemoved, r.unknown)}`,
    `local_branch=${lb}`,
    `remote_branch=${res.state || (res.ran ? 'unknown' : 'na')}`,
    `wt_dirs=${dirs}`, ...(reason ? [`reason=${reason}`] : []),
    `verdict=${verdict}`,
    ...(notes.length ? [`note=${notes.join(',')}`] : []),
  ]
  return { line: parts.join(' '), ok: verdict === 'ok' || verdict === 'planned' || verdict === 'noop', verdict, reason, notes }
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
  if (modeArg === '--resolve') {
    // ps1 只采事实、不判（#2247）：认树与身份都在这里算完，它拿结果去查 PR 头／对账／拆树／核残留。
    // 这一腿就是「三处 feat/<票号> 写死退场」的接线口——ps1 手里不再有自己拼的分支名。
    const r = resolveTree(facts)
    const env = [
      'TREE_RESOLVE',
      `state=${r.state}`,
      `path=${r.path || 'na'}`,
      `branch=${shortRef(r.identityBranch) || 'na'}`,
      `probe_branch=${shortRef(r.probeBranch) || 'na'}`,
      `legacy=${r.legacy}`,
      `mismatch=${r.mismatch}`,
      `reason=${r.reason || 'na'}`,
    ]
    console.log(env.join(' '))
    // 只有认出树（registered/legacy）才让 ps1 继续往下走；其余态由 decide 判红并给 reason
    process.exit(r.state === 'registered' || r.state === 'legacy' ? 0 : 1)
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
  console.log('用法: node dev-close.mjs --resolve|--decide|--verdict < facts.json')
  process.exit(2)
}
