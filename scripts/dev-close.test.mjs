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
import { decide, judgeVerdict, parseRemoveJson, findTargetTree, SCRIPT_REF_RE } from './lib/dev-close.mjs'

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

test('L5 认树认分支不认目录名，且站在要删的树里必须先离开', () => {
  const list = ['worktree D:/FL', 'branch refs/heads/master', '', 'worktree D:/FL/wt-2174', 'branch refs/heads/feat/2174', ''].join('\n')
  const found = findTargetTree(list, '2174')
  assert.equal(found.registered, true, '按 refs/heads/feat/<票> 认，目录可被改名而分支不会')
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
  assert.match(v.stdout, /^DEV_CLOSE_RESULT task=2174 .*verdict=planned$/m)
})