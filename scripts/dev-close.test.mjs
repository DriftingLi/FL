#!/usr/bin/env node
/**
 * dev-close 行为守护（#2174）。两条腿。
 *
 * 全 ASCII 源码（承 #2139 的编码自律）：机检行与断言字面量一旦依赖非 ASCII 字面量，会被写入链
 * 整字吞掉、当场得到空字符串 —— 那正是假绿的形状，所以这里连移动端目录名都不硬写，运行时枚举。
 *
 * L1 幻影引用腿：执行单里写的每一条 scripts/*.ps1 必须在两个承载面之一真实存在（口径同 #2053）。
 *   本票的红样品就是这条腿本身：dev-close.ps1 未落地时 AGENTS.md:20 / :60 教的是不存在的命令。
 * L2 决策腿：三件危险的事由纯函数判定 —— 未合并不得拆、dry-run 不得透传 -Execute、
 *   树已 absent 必须幂等放行、读不到 PR 状态必须 fail-closed。变异腿靠现场翻规则取证（见票评论）。
 */
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readdirSync, readFileSync, existsSync } from 'node:fs'
import { spawnSync } from 'node:child_process'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { decide, judgeVerdict, parseRemoveJson, findTargetTree, resolveTree, shortRef, SCRIPT_REF_RE } from './lib/dev-close.mjs'

const here = dirname(fileURLToPath(import.meta.url))
const repoRoot = resolve(here, '..')
const LIST_PRESENT = ['worktree D:/FL', 'branch refs/heads/master', '', 'worktree D:/FL/wt-2174', 'branch refs/heads/feat/2174', ''].join('\n')

// 移动端根按目录枚举得出（不写死中文名：写死=第二份口径，且中文名进源码正是被咬过的形态）
function mobileRoot() {
  const base = join(repoRoot, 'training-app')
  const dir = readdirSync(base, { withFileTypes: true }).filter((e) => e.isDirectory()).map((e) => e.name)[0]
  assert.ok(dir, 'training-app 下没有子目录，L1 腿无从判起')
  return join(base, dir)
}

function scriptRefs(md) {
  return [...new Set(md.match(SCRIPT_REF_RE) || [])].map((s) => s.trim())
}

test('L1 执行单里引用的 scripts/*.ps1 全部真实存在（#2174 的缺口就是这条）', () => {
  const mroot = mobileRoot()
  const md = readFileSync(join(mroot, 'AGENTS.md'), 'utf8')
  const refs = scriptRefs(md)
  assert.ok(refs.length > 0, 'AGENTS.md 里一条 scripts/*.ps1 都没匹配到 = 判据空转，不是绿')
  const missing = refs.filter((r) => {
    const rel = r.replace(/^\//, '')
    return !existsSync(join(repoRoot, rel)) && !existsSync(join(mroot, rel))
  })
  assert.deepEqual(missing, [], '幻影命令（文档教了不存在的脚本）：' + missing.join(', '))
})

test('L2a 未合并 / 读不到 PR 状态一律拒绝拆树（fail-closed）', () => {
  for (const prState of ['OPEN', 'CLOSED', 'NONE', 'unknown']) {
    const d = decide({ task: '2174', prState, worktreeList: LIST_PRESENT, unmergedLog: '', execute: true })
    assert.equal(d.allow, false, prState + ' 不许拆树')
    assert.equal(d.passExecute, false, prState + ' 即便给了 -Yes 也不得透传 -Execute')
  }
  const d = decide({ task: '2174', prState: 'MERGED', worktreeList: LIST_PRESENT, unmergedLog: 'deadbeef 还有一条没进 master 的提交', execute: true })
  assert.equal(d.allow, false, '有未合并提交时不许拆（remove-worktree 自己也会保留分支）')
})

test('L2a2 squash 仓判据：只能问 PR 头，不能问 origin/master（#2174 dogfood 实测）', () => {
  const ps1 = readFileSync(join(here, 'dev-close.ps1'), 'utf8')
  assert.ok(ps1.includes('headRefOid'), '必须取 PR 的 head sha 当比对基准')
  assert.ok(!/origin\/master\.\.HEAD/.test(ps1), '不许再用 origin/master..HEAD：squash 合并后它恒非空，会挡住每一次正常收尾')
  assert.ok(ps1.includes('"$prHead..HEAD"'), '没合并的活 = PR 头之后的提交')
  const d = decide({ task: '2174', prState: 'MERGED', worktreeList: LIST_PRESENT, unmergedLog: 'pr-head-unreadable', execute: true })
  assert.equal(d.allow, false, '读不到 PR 头必须 fail-closed，不许当成「没活」放行')
})

// wt-2170 实测事实（#2253）：squash 合并后本地 HEAD（7b1a331f）在 origin/master 祖先链上，
// 与 PR headRefOid（1529863）tree 相同、无祖先关系 ⇒ prHead..HEAD 输出 1 行、旧判据恒拦。
// 逃生阀只认 yes；no/unreadable/缺失一律维持 fail-closed。
const LIST_2170 = ['worktree D:/FL', 'branch refs/heads/master', '', 'worktree D:/FL/wt-2170', 'branch refs/heads/feat/2170', ''].join('\n')
test('L2a3 squash 同内容异 sha 误红：HEAD 已是 origin/master 祖先时不拦（#2253 wt-2170 实测）', () => {
  const d = decide({ task: '2170', prState: 'MERGED', worktreeList: LIST_2170, cwd: 'D:\\FL', unmergedLog: '7b1a331f ci(gate): ADR-0052 wave2 (#2172)', headInMaster: 'yes', execute: true })
  assert.equal(d.allow, true, 'HEAD 全在 master 祖先链 ⇒ 树里没有会丢的活，该放行拆树')
  assert.equal(d.passExecute, true, '-Yes 照常透传')
  for (const fact of ['no', 'unreadable', undefined, '']) {
    const b = decide({ task: '2170', prState: 'MERGED', worktreeList: LIST_2170, cwd: 'D:\\FL', unmergedLog: '7b1a331f x', headInMaster: fact, execute: true })
    assert.equal(b.allow, false, 'headInMaster=' + fact + ' 时不许拆（fail-closed 不放松）')
    assert.equal(b.reason, 'unmerged-commits', '拦的理由仍是未合并对账，不许换成别的')
  }
  const ps1 = readFileSync(join(here, 'dev-close.ps1'), 'utf8')
  assert.ok(ps1.includes('merge-base --is-ancestor HEAD origin/master'), 'ps1 采集侧必须问祖先事实（三态：yes/no/unreadable）')
  assert.ok(/headInMaster\s*=/.test(ps1), 'ps1 必须把 headInMaster 事实递给 decide——判据在 node，采集在 ps1，两头都缺不了')
})

// #2289 实测事实（wt-2262）：GitHub squash 合并若走了「update branch」，PR headRefOid 被换成
// master merge + 原分支头的合成提交（tree 与 master 上 squash commit 完全一致），本地 HEAD 是它的
// **祖先** ⇒ log prHead..HEAD 把本地独有 sha 恒假红。headInPr=yes 与 headInMaster=yes 同一安全方向。
const LIST_2262 = ['worktree D:/FL', 'branch refs/heads/master', '', 'worktree D:/FL/wt-2262', 'branch refs/heads/feat/2262', ''].join('\n')
test('L2a4 squash update-branch 误红：HEAD 已是 PR 头祖先时不拦（#2289 wt-2262 实测）', () => {
  const d = decide({ task: '2262', prState: 'MERGED', worktreeList: LIST_2262, cwd: 'D:\\FL', unmergedLog: 'bbe28c01 fix(gates): gate-tee 收流窗口按仍在出数据重置（#2262）', headInMaster: 'no', headInPr: 'yes', execute: true })
  assert.equal(d.allow, true, 'HEAD 全在 PR 头祖先链 ⇒ 树里没有 PR 之外的活，该放行拆树')
  assert.equal(d.passExecute, true, '-Yes 照常透传')
  for (const fact of ['no', 'unreadable', undefined, '']) {
    const b = decide({ task: '2262', prState: 'MERGED', worktreeList: LIST_2262, cwd: 'D:\\FL', unmergedLog: 'bbe28c01 x', headInMaster: 'no', headInPr: fact, execute: true })
    assert.equal(b.allow, false, 'headInPr=' + fact + ' 时不许拆（fail-closed 不放松）')
    assert.equal(b.reason, 'unmerged-commits', '拦的理由仍是未合并对账，不许换成别的')
  }
  const ps1 = readFileSync(join(here, 'dev-close.ps1'), 'utf8')
  assert.ok(ps1.includes('merge-base --is-ancestor HEAD $prHead'), 'ps1 采集侧必须问 PR 头祖先事实（三态：yes/no/unreadable）')
  assert.ok(/headInPr\s*=/.test(ps1), 'ps1 必须把 headInPr 事实递给 decide——判据在 node，采集在 ps1，两头都缺不了')
})

test('L2b dry-run 是默认，-Yes 才透传 -Execute', () => {
  const dry = decide({ task: '2174', prState: 'MERGED', worktreeList: LIST_PRESENT, unmergedLog: '', execute: false })
  assert.equal(dry.allow, true, '已合并且无未提交活 => 允许')
  assert.equal(dry.passExecute, false, '不带 -Yes 绝不透传 -Execute')
  const yes = decide({ task: '2174', prState: 'MERGED', worktreeList: LIST_PRESENT, unmergedLog: '', execute: true })
  assert.equal(yes.passExecute, true, '-Yes 才真正执行')
})

test('L2c 树已不存在 = 幂等放行，且不再发起删除', () => {
  const d = decide({ task: '2174', prState: 'MERGED', worktreeList: '', unmergedLog: '', execute: true })
  assert.equal(d.allow, true, '重复跑/已手工拆过不该报错')
  assert.equal(d.skipRemove, true, '没有东西可删就不发起删除')
  assert.equal(d.passExecute, false, '幂等腿不带 -Execute')
})

test('L2d 只动本票那一棵树：目标目录由 task 唯一决定', () => {
  const d = decide({ task: '2174', prState: 'MERGED', worktreeList: LIST_PRESENT, unmergedLog: '', execute: true })
  assert.equal(d.target, 'wt-2174', '目标必须由票号唯一决定，不得是通配或别的会话的树')
})

test('L3 remove-worktree 的出口 JSON 解析（判据在那一侧，这里只读回来）', () => {
  const j = parseRemoveJson(['{"task":"2174","worktree":"D:/FL/wt-2174","dirRemoved":true,"regRemoved":true,"branchRemoved":true}', '[remove-worktree] done'].join('\n'))
  assert.equal(j.dirRemoved, true)
  assert.equal(j.branchRemoved, true)
  assert.equal(parseRemoveJson('not json at all').unknown, true, '读不回来必须显式标 unknown，不得当成已删除')
})

test('L4 机检读数行：执行干净才 ok，残留判红，dry-run 只报 planned', () => {
  const clean = judgeVerdict({ task: '2174', prState: 'MERGED', mode: 'execute', fetch: 'ok', treeBefore: 'present', remove: { ran: true, exitCode: 0, dirRemoved: true, regRemoved: true, branchRemoved: true }, residue: { ran: true, exitCode: 0, state: 'absent' } })
  assert.equal(clean.ok, true, clean.line)
  assert.match(clean.line, /^DEV_CLOSE_RESULT task=2174 /, '行首必须是机检 token（#2162 的教训：结论行不在行首就没人读得出）')
  assert.match(clean.line, /verdict=ok$/)
  const residue = judgeVerdict({ task: '2174', prState: 'MERGED', mode: 'execute', fetch: 'ok', treeBefore: 'present', remove: { ran: true, exitCode: 0, dirRemoved: true, regRemoved: true, branchRemoved: true }, residue: { ran: true, exitCode: 1, state: 'residue' } })
  assert.equal(residue.ok, false, '远端分支还在就必须红')
  assert.match(residue.line, /verdict=red$/)
  const dry = judgeVerdict({ task: '2174', prState: 'MERGED', mode: 'dryrun', fetch: 'ok', treeBefore: 'present', remove: { ran: true, exitCode: 0, unknown: true }, residue: { ran: false, state: 'na' } })
  assert.match(dry.line, /verdict=planned$/, 'dry-run 不得声称已删除')
  const blocked = judgeVerdict({ task: '2174', prState: 'OPEN', mode: 'execute', fetch: 'ok', treeBefore: 'present', remove: { ran: false, state: 'na' }, residue: { ran: false, state: 'na' }, blockedReason: 'pr-not-merged' })
  assert.equal(blocked.ok, false)
  assert.match(blocked.line, /reason=pr-not-merged/, '被谁挡的必须写在读数里，不能只说红')
  assert.match(blocked.line, /verdict=blocked$/)
})

test('L5 认树：目录名 wt-<票号> 是入口（#2247 翻的面），且站在要删的树里必须先离开', () => {
  // 旧口径是「按 refs/heads/feat/<票> 认」，它假设分支名由票号唯一决定 —— 血账见 #2247。
  // 新口径的主判据与分支叫什么无关：入口只有「注册表里路径名为 wt-<票号> 的条目」这一条。
  const renamed = ['worktree D:/FL', 'branch refs/heads/master', '', 'worktree D:/FL/wt-oldname', 'branch refs/heads/feat/2174', ''].join('\n')
  const rr = resolveTree({ task: '2174', worktreeList: renamed })
  assert.notEqual(rr.state, 'registered', '目录名不是 wt-2174 就不走主判据（主判据只认目录名）')
  assert.equal(rr.state, 'legacy', '但分支仍叫 feat/2174 ⇒ 回退腿兜住，宽一期，并留 legacy 痕')
  const d = decide({ task: '2174', worktreeList: LIST_PRESENT, prState: 'MERGED', unmergedLog: '', cwd: 'D:\\FL\\wt-2174', execute: true })
  assert.equal(d.allow, false, '站在要删的树里删自己的树 = 删完 cwd 失效，后面每步都读不到东西')
  assert.equal(d.reason, 'cwd-inside-target')
  const out = decide({ task: '2174', worktreeList: LIST_PRESENT, prState: 'MERGED', unmergedLog: '', cwd: 'D:\\FL', execute: true })
  assert.equal(out.allow, true, '主树里跑本票收尾是正常姿势')
  assert.equal(out.treePath, 'D:/FL/wt-2174')
})
test('L6 CLI 真执行：带 BOM 的 stdin 必须照读，坏 JSON 必须 fail-closed（#2174 实测坑）', () => {
  const core = join(here, 'lib', 'dev-close.mjs')
  // PowerShell 把 $OutputEncoding 设成 [Text.Encoding]::UTF8 后，往子进程 stdin 写字符串会冠 U+FEFF。
  // 这一腿就是那次的回归钉：不剥 BOM ⇒ 整条判定链读不回 facts ⇒ 恒红，而红得看不出根因。
  const bom = spawnSync(process.execPath, [core, '--decide'], { input: '\uFEFF{"task":"2174","prState":"MERGED","execute":false}', encoding: 'utf8' })
  assert.equal(bom.status, 0, bom.stdout + bom.stderr)
  assert.match(bom.stdout, /^DECIDE allow=true /, '结论行必须在行首（#2162 同族教训）')
  assert.match(bom.stdout, /reason=tree-absent/, '没给 worktreeList 时按幂等腿走，不谎报已删')
  const bad = spawnSync(process.execPath, [core, '--decide'], { input: 'not json', encoding: 'utf8' })
  assert.equal(bad.status, 2, '读不回 facts 必须非零退出，不许当成放行')
  assert.match(bad.stdout, /verdict=red reason=facts-unreadable/)
  const v = spawnSync(process.execPath, [core, '--verdict'], { input: '{"task":"2174","prState":"MERGED","mode":"dryrun","fetch":"ok","treeBefore":"absent","remove":{"ran":false},"residue":{"ran":false,"state":"na"}}', encoding: 'utf8' })
  assert.equal(v.status, 0, v.stdout + v.stderr)
  // #2247：verdict 之后可以跟 note=（无动作永不打 ok，absent+dry-run 必须自带标记）。
  // 断言 verdict 这个词与值成对出现、且是最后一个 token 组的第一项，不依赖行尾（行尾已属 note）。
  assert.match(v.stdout, /^DEV_CLOSE_RESULT task=2174 .*verdict=planned( |$)/m)
  assert.match(v.stdout, /note=no-pending-action/, '「本次无任何待执行动作」必须落在读数上，否则空转像达成')
})

// #2188 补的第三族：production wire 形态重放。L4/L6 的 fixture 都是「预解析字段直喂 judgeVerdict」，
// 而 ps1 生产链路只交 {ran,exitCode,raw}（dev-close.ps1:145-146）—— fixture 形态不等于生产形态，
// 就是这个缺口让 dir=na / local_branch=kept / exit-4 恒红三连漂到线上跑了几个月。
test('L7 wire 形态重放：raw 必须被判定层解析，residue 的 4/5/2/3 不许揉成一条 branch-residue', () => {
  // remove raw 的真实形状 = plan ConvertTo-Json 与 result ConvertTo-Json 两块相连（remove-worktree.ps1:391/461）。
  const planJson = [
    '{',
    '    "task":  "2184",',
    '    "worktree":  "D:/FL/wt-2184",',
    '    "branch":  "feat/2184",',
    '    "holders":  [],',
    '    "actions":  ["git-worktree-remove: D:/FL/wt-2184"]',
    '}',
  ].join('\n')
  const resultJson = [
    '{',
    '    "task":  "2184",',
    '    "worktree":  "D:/FL/wt-2184",',
    '    "dirRemoved":  true,',
    '    "regRemoved":  true,',
    '    "branchRemoved":  true',
    '}',
  ].join('\n')
  const removeRaw = [planJson, resultJson, '[remove-worktree] done'].join('\n')
  const ABSENT = 'ABSENT  feat/2184'
  const SUMMARY0 = 'SUMMARY branches=0 mergedResidue=0 needHuman=0 openPR=0 archived=0'
  const orphanRaw = [ABSENT, SUMMARY0, 'WT-DIRS registered=5 disk=6 orphans=1 linked=0 skeleton=1'].join('\n')
  const dirsCleanRaw = [ABSENT, SUMMARY0, 'WT-DIRS registered=4 disk=4 orphans=0 linked=0 skeleton=0'].join('\n')
  const auditDeadRaw = [ABSENT, SUMMARY0, 'WT-DIRS=unavailable'].join('\n')
  const residueRaw1 = ['RESIDUE   feat/2184  PR#2184 merged', 'SUMMARY branches=1 mergedResidue=1 needHuman=0 openPR=0 archived=0'].join('\n')
  const wire = (over) => judgeVerdict({
    task: '2184', prState: 'MERGED', mode: 'execute', fetch: 'ok', treeBefore: 'present',
    remove: { ran: true, exitCode: 0, raw: removeRaw },
    residue: { ran: true, exitCode: 0, raw: dirsCleanRaw },
    ...over,
  })

  // A) #2184 现场重放：真拆了树 + 分支侧绿（ABSENT），但仓级孤儿目录让 residue exit 4。
  //    旧代码当场 dir=na reg=na local_branch=kept verdict=red reason=branch-residue —— 三连漂的现场形状。
  const a = wire({ residue: { ran: true, exitCode: 4, raw: orphanRaw } })
  assert.match(a.line, / dir=ok/, 'raw 里的 result JSON 必须被读回（' + a.line + '）')
  assert.match(a.line, / reg=ok/)
  assert.match(a.line, /local_branch=removed/)
  assert.match(a.line, /remote_branch=absent/)
  assert.match(a.line, /wt_dirs=orphan:1/, '别人树的孤儿目录要可见，但进独立 token，不揉进本票残留')
  assert.match(a.line, /verdict=ok$/, '分支侧已绿 = 本次收尾干净，孤儿目录不是本票的债（' + a.line + '）')

  // B) 幂等腿重放：树已 absent、remove 未发起，local_branch 必须随 git branch --list 事实报 absent（验收第 3 条）。
  //    #2251 翻面：execute + tree=absent = 本次什么都没拆 ⇒ 不许再报 verdict=ok（ok 只留给真拆且对账通过的腿），
  //    改报 noop + note=no-pending-action —— 「已重跑过」与「刚干完」在读数上必须长得不一样。
  const b = wire({ treeBefore: 'absent', localBranch: 'absent', remove: { ran: false, exitCode: 0, raw: '' } })
  assert.match(b.line, /local_branch=absent/, '重跑不能谎报 kept（' + b.line + '）')
  assert.match(b.line, / dir=na/)
  assert.match(b.line, /verdict=noop( |$)/, '无动作不许报 ok（' + b.line + '）')
  assert.match(b.line, /note=no-pending-action/, '无动作必须自带「本次无任何待执行动作」（' + b.line + '）')
  assert.equal(b.ok, true, 'noop 不是红：已收完的票重跑一次不该失败')

  // C) 真残留（exit 1）照旧红，reason 不变 —— 老语义的回归钉。
  const c = wire({ residue: { ran: true, exitCode: 1, raw: residueRaw1 } })
  assert.match(c.line, /verdict=red$/)
  assert.match(c.line, /reason=branch-residue/)
  assert.match(c.line, /remote_branch=residue/)

  // D) 目录对账不可用（exit 5）：fail-closed 仍红，但 reason 必须与真残留分家（验收第 1 条的「区分」）。
  const d = wire({ residue: { ran: true, exitCode: 5, raw: auditDeadRaw } })
  assert.match(d.line, /verdict=red$/)
  assert.match(d.line, /reason=wt-audit-unavailable/, '（' + d.line + '）')
  assert.match(d.line, /remote_branch=absent/, '分支侧读数是绿的事实，红在目录审计（' + d.line + '）')
  assert.match(d.line, /wt_dirs=unavailable/)

  // E) API 读不到（exit 2）：红且 reason 独立。
  const e = wire({ residue: { ran: true, exitCode: 2, raw: '[check-branch-residue] gh api failed' } })
  assert.match(e.line, /verdict=red$/)
  assert.match(e.line, /reason=branch-check-failed/)

  // F) remove 跑了但 result JSON 读不回：fail-closed，不许 ok。
  const f2 = wire({ remove: { ran: true, exitCode: 0, raw: [planJson, '[remove-worktree] output truncated'].join('\n') } })
  assert.match(f2.line, / dir=unknown/)
  assert.match(f2.line, /reason=tree-leftover/, '删除结果读不回不许当成已删（' + f2.line + '）')
  assert.match(f2.line, /verdict=red$/)

  // G) CLI 全链路（ps1 stdin 形态直喂）：与 A 同读数。
  const core2 = join(here, 'lib', 'dev-close.mjs')
  const cli = spawnSync(process.execPath, [core2, '--verdict'], {
    input: JSON.stringify({
      task: '2184', prState: 'MERGED', mode: 'execute', fetch: 'ok', treeBefore: 'present', localBranch: 'absent',
      remove: { ran: true, exitCode: 0, raw: removeRaw }, residue: { ran: true, exitCode: 4, raw: orphanRaw },
    }),
    encoding: 'utf8',
  })
  assert.equal(cli.status, 0, cli.stdout + cli.stderr)
  assert.match(cli.stdout, / dir=ok reg=ok local_branch=removed remote_branch=absent wt_dirs=orphan:1 verdict=ok/)
})

// ---------------------------------------------------------------------------
// #2247（spec B / #2233）：树身份重裁定。认树 = 路径名 wt-<票号>，身份 = 该条目登记的分支。
// 这一族是血账的回归钉：wt-2230 挂 fix/2230-dead-subtab、wt-2189 挂 fix/2189-lint-brace-expansion，
// 旧判据（按 refs/heads/feat/<票号> 认树）一律判「无树」⇒ 静默跳过全部动作，dry-run 还出一行
// 「无待办」被读成已达成。下面每条都钉住「宽/严」的那一面，而不是只钉住 happy path。
// ---------------------------------------------------------------------------

// fix/* 形态的真机形状：票号的树挂着一条不以 feat/ 开头的分支。
const LIST_FIX_SHAPE = [
  'worktree D:/FL', 'branch refs/heads/master', '',
  'worktree D:/FL/wt-2230', 'branch refs/heads/fix/2230-dead-subtab', '',
  'worktree D:/FL/wt-2189', 'branch refs/heads/fix/2189-lint-brace-expansion', '',
].join('\n')

test('L8 认树看目录名、身份看登记分支：fix/* 形态的树必须被认出（#2247 血账）', () => {
  const r = resolveTree({ task: '2230', worktreeList: LIST_FIX_SHAPE })
  assert.equal(r.state, 'registered', '目录名 wt-2230 在册 ⇒ 是本票的树，与分支叫什么都不相干')
  assert.equal(r.path, 'D:/FL/wt-2230')
  assert.equal(r.identityBranch, 'refs/heads/fix/2230-dead-subtab', '身份 = 该条目登记的分支')
  assert.equal(r.mismatch, false)
  // 旧判据在这里判 absent —— 钉住它不再回来
  assert.equal(decide({ task: '2230', worktreeList: LIST_FIX_SHAPE, prState: 'MERGED', unmergedLog: '', cwd: 'D:\\FL', execute: true }).treeState, 'registered')
})

test('L9 身份分支贯穿四类消费方；拆掉贯穿必红（变异留痕）', () => {
  // 四类消费方 = PR 头查询 / 未合并对账 / 拆树调用 / 分支残留核对。它们都从同一个身份出发：
  // 判定层给 identityBranch 与 probeBranch，ps1 侧与 verdict 行只许用它，不许自己拼 feat/<票号>。
  const r = resolveTree({ task: '2189', worktreeList: LIST_FIX_SHAPE })
  assert.equal(shortRef(r.identityBranch), 'fix/2189-lint-brace-expansion')
  assert.equal(shortRef(r.probeBranch), 'fix/2189-lint-brace-expansion', '探测分支也必须是身份，不能是 feat/<票号>')

  // 读数行带身份分支：一行就能看出收的是哪条分支
  const v = judgeVerdict({
    task: '2189', prState: 'MERGED', mode: 'execute', fetch: 'ok', treeBefore: 'present',
    branch: r.identityBranch, localBranch: 'kept',
    remove: { ran: true, exitCode: 0, raw: '{"dirRemoved":true,"regRemoved":true,"branchRemoved":false}' },
    residue: { ran: true, exitCode: 0, raw: 'ABSENT  fix/2189-lint-brace-expansion' },
  })
  assert.match(v.line, / branch=fix\/2189-lint-brace-expansion /, '身份分支必须进读数行（' + v.line + '）')

  // 变异：把身份换回写死的 feat/<票号> ⇒ 同一条链当场判红，而不是静默「已清干净」
  const mutated = judgeVerdict({
    task: '2189', prState: 'MERGED', mode: 'execute', fetch: 'ok', treeBefore: 'present',
    branch: 'refs/heads/feat/2189', localBranch: 'kept',
    remove: { ran: true, exitCode: 0, raw: '{"dirRemoved":true,"regRemoved":true,"branchRemoved":false}' },
    residue: { ran: true, exitCode: 1, raw: 'RESIDUE   fix/2189-lint-brace-expansion  local' },
  })
  assert.equal(mutated.ok, false, '没删掉的分支不许被报成绿')
  assert.match(mutated.line, /reason=branch-residue/)

  // 贯穿的硬面在 ps1 源码：可执行行里不许再有自己拼的 feat/<Task>。
  // 注释里提到谱系是允许的（那是记录血账），所以只扫非注释行。
  const ps1 = readFileSync(join(here, 'dev-close.ps1'), 'utf8')
  // 先整块剥掉 <# ... #> 帮助注释（谱系记录住在那里），再逐行去掉 # 行注释 —— 剩下的才是可执行行
  const codeOnly = ps1.replace(/<#[\s\S]*?#>/g, '')
  const codeLines = codeOnly.split(/\r?\n/).filter((l) => !/^\s*#/.test(l) && !/^\s*$/.test(l))
  const hardcoded = codeLines.filter((l) => /feat\//.test(l))
  assert.deepEqual(hardcoded, [], 'ps1 可执行行里不许再写死 feat/<票号>：' + hardcoded.join(' | '))
  // 反证：ps1 必须真的把身份分支传下去（四处消费方都靠这一个变量）
  assert.match(ps1, /'--resolve'/, '认树必须问判定层，不许在自己这侧重写一遍')
  assert.match(ps1, /\$identBranch/, '身份分支必须是 ps1 里唯一的来源')
  assert.match(ps1, /'-Branch', \$identBranch/, '拆树与残留核对都要按身份分支调用')
})

test('L10 回退腿：目录不在册但 refs/heads/feat/<票号> 仍在 ⇒ 命中并打 legacy-branch-match（WARN，宽一期）', () => {
  // 老形态的树：目录名被改过（或历史上就叫别的），只剩分支还是 feat/<票号>。
  const legacyList = [
    'worktree D:/FL', 'branch refs/heads/master', '',
    'worktree D:/FL/wt-renamed-old', 'branch refs/heads/feat/2231', '',
  ].join('\n')
  const r = resolveTree({ task: '2231', worktreeList: legacyList })
  assert.equal(r.state, 'legacy', '主判据落空时才走回退腿')
  assert.equal(r.legacy, true)
  assert.equal(shortRef(r.identityBranch), 'feat/2231')

  const d = decide({ task: '2231', worktreeList: legacyList, prState: 'MERGED', unmergedLog: '', cwd: 'D:\\FL', execute: true })
  assert.equal(d.allow, true, '回退腿命中要能继续走，不是判红')
  assert.equal(d.legacy, true, '但必须留痕，好让下一期收掉这条宽腿')

  const v = judgeVerdict({
    task: '2231', prState: 'MERGED', mode: 'dryrun', fetch: 'ok', treeBefore: 'present', legacy: true,
    branch: r.identityBranch, remove: { ran: false }, residue: { ran: false, state: 'na' },
  })
  assert.match(v.line, /note=legacy-branch-match/, 'WARN 必须写在读数上，不能只活在调用栈里（' + v.line + '）')
  assert.equal(v.verdict, 'planned', 'WARN 不升级成红')
})

test('L11 -Branch 覆盖与登记分支不一致 ⇒ 判红 branch-mismatch（不猜）', () => {
  const d = decide({
    task: '2230', worktreeList: LIST_FIX_SHAPE, branch: 'feat/2230',
    prState: 'MERGED', unmergedLog: '', cwd: 'D:\\FL', execute: true,
  })
  assert.equal(d.allow, false, '覆盖参数与树身份冲突时不许二选一硬干')
  assert.equal(d.reason, 'branch-mismatch')
  assert.equal(d.skipRemove, true)
  // 一致时放行（覆盖参数本身不是问题）
  const ok = decide({
    task: '2230', worktreeList: LIST_FIX_SHAPE, branch: 'fix/2230-dead-subtab',
    prState: 'MERGED', unmergedLog: '', cwd: 'D:\\FL', execute: true,
  })
  assert.equal(ok.allow, true, '给了且与登记一致就该放行')
  assert.equal(ok.reason, 'ok')
})

test('L12 detached 条目不参与票号判定：落 tree-branch-mismatch，不是「树不存在」', () => {
  // 基座树 wtx-base 那种：注册表里有条目但没有 branch 行。
  const detachedList = [
    'worktree D:/FL', 'branch refs/heads/master', '',
    'worktree D:/FL/wt-2232', 'detached', '',
  ].join('\n')
  const r = resolveTree({ task: '2232', worktreeList: detachedList })
  assert.equal(r.state, 'tree-branch-mismatch', 'detached 不参与票号判定，但「目录在、判据没匹配上」必须可分辨')
  assert.equal(r.reason, 'tree-branch-mismatch')

  const d = decide({ task: '2232', worktreeList: detachedList, prState: 'MERGED', unmergedLog: '', cwd: 'D:\\FL', execute: true })
  assert.equal(d.allow, false, '这一态不许当成幂等腿放行 —— 那正是静默空转的形状')
  assert.equal(d.reason, 'tree-branch-mismatch')

  // 对照：真不存在 ⇒ 幂等放行、但树态是 absent
  const none = decide({ task: '2233', worktreeList: detachedList, prState: 'MERGED', unmergedLog: '', cwd: 'D:\\FL', execute: true })
  assert.equal(none.allow, true)
  assert.equal(none.reason, 'tree-absent')
})

test('L13 未注册野树点名 unregistered-tree，指回 -PruneOrphans（不接管）', () => {
  const cleanList = ['worktree D:/FL', 'branch refs/heads/master', ''].join('\n')
  const r = resolveTree({ task: '2234', worktreeList: cleanList, treeDirExists: true })
  assert.equal(r.state, 'unregistered-tree', '磁盘上有 wt-2234 但注册表里没有 = 野树')
  assert.equal(r.reason, 'unregistered-tree')
  const d = decide({ task: '2234', worktreeList: cleanList, treeDirExists: true, prState: 'MERGED', unmergedLog: '', cwd: 'D:\\FL', execute: true })
  assert.equal(d.allow, false, '野树不归单棵树这条链管，别替它做决定')
  assert.equal(d.reason, 'unregistered-tree')
})

test('L14 三态行内可 grep、机检可分流（#2247 Review 补丁 / spec AC3）', () => {
  // 三个 reason 必须是三个不同的字面量，且各自能被单独一行 Select-String 挑出来。
  // 揉成一态就是「静默空转」被读成「已达成」的老病 —— 这里钉死它们不许再合流。
  const detachedList = ['worktree D:/FL', 'branch refs/heads/master', '', 'worktree D:/FL/wt-2240', 'detached', ''].join('\n')
  const cleanList = ['worktree D:/FL', 'branch refs/heads/master', ''].join('\n')
  const three = {
    'tree-branch-mismatch': decide({ task: '2240', worktreeList: detachedList, prState: 'MERGED', unmergedLog: '', cwd: 'D:\\FL' }).reason,
    'unregistered-tree': decide({ task: '2241', worktreeList: cleanList, treeDirExists: true, prState: 'MERGED', unmergedLog: '', cwd: 'D:\\FL' }).reason,
    'branch-mismatch': decide({ task: '2230', worktreeList: LIST_FIX_SHAPE, branch: 'feat/2230', prState: 'MERGED', unmergedLog: '', cwd: 'D:\\FL' }).reason,
  }
  const got = Object.values(three)
  assert.deepEqual(got, ['tree-branch-mismatch', 'unregistered-tree', 'branch-mismatch'], '三态必须各归各位、互不吞并')
  for (const [want, reason] of Object.entries(three)) {
    const line = `DEV_CLOSE_RESULT task=2240 pr=MERGED mode=dryrun tree=absent reason=${reason} verdict=red`
    assert.match(line, new RegExp(`reason=${want}\\b`), `单判据 grep 必须能挑出 ${want}`)
  }
  // 三者与「真不存在」的 absent 也必须是四个不同字面量（absent 那一态不带 reason 进入红）
  assert.equal(new Set([...got, 'tree-absent']).size, 4)
})

test('L15 主树根问 git（--git-common-dir），不从脚本位置上溯；无身份即 branch=na', () => {
  // 从 linked worktree 调用时 $PSScriptRoot 指的是**那棵树自己** ⇒ 上溯得到 wt-2247，
  // 于是 wt-<票号> 目录对着自己找（unregistered-tree 永不触发）、-SyncMaster 也会合错树。
  const ps1 = readFileSync(join(here, 'dev-close.ps1'), 'utf8')
  assert.doesNotMatch(ps1, /\$root = Split-Path -Parent \$PSScriptRoot/, '不许再从脚本位置上溯推主树根')
  assert.match(ps1, /Get-WtMainRoot/, '主树根必须走唯一实现（移动端 lib/wt-bootstrap.ps1）')
  assert.match(ps1, /--git-common-dir|Get-WtMainRoot/, '判据来自 git，不是路径形状')

  // 没有认出树就没有身份分支：读数行不许拿探测分支（feat/<票号>）冒充身份。
  const v = judgeVerdict({
    task: '9999', prState: 'NONE', mode: 'dryrun', fetch: 'ok', treeBefore: 'absent',
    branch: '', remove: { ran: false }, residue: { ran: false, state: 'na' },
  })
  assert.match(v.line, / branch=na\b/, '无身份必须是 na，不能写成探测分支（' + v.line + '）')
  assert.doesNotMatch(v.line, /branch=feat\//)
})

test('L16 dry-run 三态分离：无待办动作 / 确不存在 / 目录在但不匹配 —— 三行互斥、各自可 grep（#2251）', () => {
  // #2251 的核心断言：这三种「没干活」必须长得不一样。B-1 之前 absent 一律 planned+no-pending-action，
  // 于是「真没树」与「没什么可做」共用同一行 —— 空转被读成达成的老病就是这么回来的。
  const cleanList = ['worktree D:/FL', 'branch refs/heads/master', ''].join('\n')
  const detachedList = ['worktree D:/FL', 'branch refs/heads/master', '', 'worktree D:/FL/wt-2250', 'detached', ''].join('\n')

  // ① no-pending-action：dry-run 且树不存在 —— 有标记、不报 ok
  const dryAbsent = judgeVerdict({
    task: '2250', prState: 'NONE', mode: 'dryrun', fetch: 'ok', treeBefore: 'absent',
    branch: '', remove: { ran: false }, residue: { ran: false, state: 'na' },
  })
  assert.equal(dryAbsent.verdict, 'planned')
  assert.ok(dryAbsent.ok, 'dry-run 永远不是红')
  assert.match(dryAbsent.line, /note=no-pending-action/, dryAbsent.line)
  assert.doesNotMatch(dryAbsent.line, /verdict=ok( |$)/, '无动作不许报 ok')

  // ② absent 已消费：execute 重跑同一票 —— 报 noop，与 ① 靠 verdict 分家
  const execAbsent = judgeVerdict({
    task: '2250', prState: 'MERGED', mode: 'execute', fetch: 'ok', treeBefore: 'absent',
    branch: '', remove: { ran: false, exitCode: 0, raw: '' }, residue: { ran: true, exitCode: 0, raw: 'ABSENT  x' },
    localBranch: 'absent',
  })
  assert.equal(execAbsent.verdict, 'noop', '树已不存在时，执行腿也没干活')
  assert.match(execAbsent.line, /note=no-pending-action/, execAbsent.line)

  // ③ 目录在但不匹配：判红点名 —— 与 ①② 靠 verdict + reason 双重分家
  const d = decide({ task: '2250', worktreeList: detachedList, prState: 'MERGED', unmergedLog: '', cwd: 'D:\\FL', execute: false })
  const execMismatch = judgeVerdict({
    task: '2250', prState: 'MERGED', mode: 'execute', fetch: 'ok', treeBefore: 'present',
    branch: 'refs/heads/feat/2250', blockedReason: d.reason,
    remove: { ran: false }, residue: { ran: false, state: 'na' },
  })
  assert.equal(execMismatch.verdict, 'blocked')
  assert.equal(execMismatch.ok, false, '目录在而判据没匹配上不许放行')
  assert.match(execMismatch.line, /reason=tree-branch-mismatch/, execMismatch.line)
  assert.doesNotMatch(execMismatch.line, /no-pending-action/, '这一态必须与「没什么可做」分开')

  // 互斥性：三行的 verdict 两两不同（任意两条都不许同形）
  const verdicts = [dryAbsent.verdict, execAbsent.verdict, execMismatch.verdict]
  assert.equal(new Set(verdicts).size, 3, '三态必须三个不同 verdict：' + verdicts.join(' / '))
  // 各自可 grep：每条都有唯一可挑的判据 token
  assert.match(dryAbsent.line, /verdict=planned .*note=no-pending-action/)
  assert.match(execAbsent.line, /verdict=noop .*note=no-pending-action/)
  assert.match(execMismatch.line, /reason=tree-branch-mismatch .*verdict=blocked/)

  // ④ 「无待办动作」的 note 只属于 absent 腿，不许漏到真干活的腿上
  const didWork = judgeVerdict({
    task: '2250', prState: 'MERGED', mode: 'execute', fetch: 'ok', treeBefore: 'present',
    branch: 'refs/heads/feat/2250', remove: { ran: true, exitCode: 0, dirRemoved: true, regRemoved: true, branchRemoved: true },
    residue: { ran: true, exitCode: 0, state: 'clean' },
  })
  assert.equal(didWork.verdict, 'ok', '真拆完才是 ok')
  assert.doesNotMatch(didWork.line, /no-pending-action/, '干过活的腿不许挂无动作标记')
})

test('L17 三态真走 CLI（真执行注入事实）：进程退出码与读数行同源，noop 不许变成失败退出', () => {
  // L16 走的是函数内直调；这一条走生产形态 —— 判定层作为子进程真跑一次，
  // 因为 ps1 只认 `exit $v.Code`：verdict 与退出码分家的话，人读到的行和链路的判断会不一致。
  const core = join(here, 'lib', 'dev-close.mjs')
  const cli = (facts) => spawnSync(process.execPath, [core, '--verdict'], { input: JSON.stringify(facts), encoding: 'utf8' })

  // ① dry-run + absent：planned，退出 0
  const dry = cli({ task: '2250', prState: 'NONE', mode: 'dryrun', fetch: 'ok', treeBefore: 'absent', branch: '', remove: { ran: false }, residue: { ran: false, state: 'na' } })
  assert.equal(dry.status, 0, dry.stdout + dry.stderr)
  assert.match(dry.stdout, /verdict=planned( |$)/)
  assert.match(dry.stdout, /note=no-pending-action/)

  // ② execute + absent：noop，退出 0（重跑已收完的票不该失败）
  const exec = cli({ task: '2250', prState: 'MERGED', mode: 'execute', fetch: 'ok', treeBefore: 'absent', localBranch: 'absent', branch: '', remove: { ran: false, exitCode: 0, raw: '' }, residue: { ran: true, exitCode: 0, raw: 'ABSENT  x' } })
  assert.equal(exec.status, 0, 'noop 不是红，退出码必须 0：' + exec.stdout + exec.stderr)
  assert.match(exec.stdout, /verdict=noop( |$)/)
  assert.match(exec.stdout, /note=no-pending-action/)

  // ③ 目录在但不匹配：blocked，退出非 0
  const blocked = cli({ task: '2250', prState: 'MERGED', mode: 'dryrun', fetch: 'ok', treeBefore: 'present', branch: 'refs/heads/feat/2250', blockedReason: 'tree-branch-mismatch', remove: { ran: false }, residue: { ran: false, state: 'na' } })
  assert.notEqual(blocked.status, 0, '判红就必须真退出非 0，不能只在读数行上写着')
  assert.match(blocked.stdout, /verdict=blocked/)
  assert.match(blocked.stdout, /reason=tree-branch-mismatch/)
  assert.doesNotMatch(blocked.stdout, /no-pending-action/)
})

test('L18 对账分支不许绕过 noop，也不许把「无动作」贴到红行上（#2251 code-review 两处反例）', () => {
  // 两个反例都来自同一类写法错误：把 noop 与 note 挂成「红分支链的 else / 只看 tree」。
  // 判据来源：spec B「「无动作」永不打 verdict=ok」+ 本票 AC③「planned + tree=absent 必带 note」——
  // 反过来说，note 只属于「确实没事可做」的那两态，红行挂着它等于自己打自己的脸。

  // ① 出口码 4：分支侧已绿、仓里躺着别的会话的孤儿目录（check-branch-residue.ps1 的并发边界）。
  //    它不进本票的红，但也绝不该因为树不存在就被算成「干完了」。
  const orphanOnly = judgeVerdict({
    task: '2250', prState: 'MERGED', mode: 'execute', fetch: 'ok', treeBefore: 'absent',
    localBranch: 'absent', branch: '',
    remove: { ran: false, exitCode: 0, raw: '' },
    residue: { ran: true, exitCode: 4, state: 'orphan', dirs: 'orphan', orphans: 1 },
  })
  assert.equal(orphanOnly.verdict, 'noop', 'exit 4 不是红，但树不在就没有达成可称：' + orphanOnly.line)
  assert.equal(orphanOnly.ok, true, 'noop 仍然是可重跑的非失败态')
  assert.doesNotMatch(orphanOnly.line, /verdict=ok( |$)/, '树不在的执行腿永不许报 ok：' + orphanOnly.line)
  assert.match(orphanOnly.line, /note=no-pending-action/)

  // ② 真有分支残留：红行必须干净利落地是红，不许同时声称「无任何待执行动作」。
  const residueRed = judgeVerdict({
    task: '2250', prState: 'MERGED', mode: 'execute', fetch: 'ok', treeBefore: 'absent',
    localBranch: 'kept', branch: 'refs/heads/feat/2250',
    remove: { ran: false, exitCode: 0, raw: '' },
    residue: { ran: true, exitCode: 1, state: 'dirty', dirs: 'clean' },
  })
  assert.equal(residueRed.verdict, 'red')
  assert.equal(residueRed.ok, false)
  assert.match(residueRed.line, /reason=branch-residue/)
  assert.match(residueRed.line, /tree=absent/, '树确实不在，这一格照实报')
  assert.doesNotMatch(residueRed.line, /no-pending-action/, '有残留就有关动作要做，红行不许贴无动作标记：' + residueRed.line)

  // ③ 出口码 5（目录对账读不到）：同样不许被「树不在」洗成 noop。
  const unreadable = judgeVerdict({
    task: '2250', prState: 'MERGED', mode: 'execute', fetch: 'ok', treeBefore: 'absent',
    localBranch: 'absent', branch: '',
    remove: { ran: false, exitCode: 0, raw: '' },
    residue: { ran: true, exitCode: 5, state: 'unknown' },
  })
  assert.equal(unreadable.verdict, 'red', '读不到就是 fail-closed 红')
  assert.equal(unreadable.ok, false)
  assert.doesNotMatch(unreadable.line, /no-pending-action/)
})
