// 迁移失败门的两条分支（ADR-0056 §6 / #1099 的「可 dry-run 断言」判据物）。
//
// 票面要求部署脚本的失败分支留下常驻判据物：这里不跑 docker、不连数据库，只调用脚本的 dry-run
// 入口 --migration-gate（判定函数 migration_failure_action 是唯一判据点），断言：
//   未声明 ALLOW_MIGRATION_FAILURE ⇒ abort；显式 =1 ⇒ continue；其他取值（0/true/...）⇒ 仍 abort。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..')

/** 解释器：CI 用 python3；Windows 本机常见 python。都找不到时给出明确提示。 */
function pythonBin() {
  const candidates = [process.env.PYTHON, process.platform === 'win32' ? 'python' : 'python3', 'python3', 'python'].filter(Boolean)
  for (const candidate of candidates) {
    const r = spawnSync(candidate, ['--version'], { stdio: 'ignore' })
    if (!r.error && r.status === 0) return candidate
  }
  throw new Error('找不到 Python 解释器（用 PYTHON 环境变量指定，例如 PYTHON=python3）')
}

const PY = pythonBin()

/** 跑一次 dry-run 门，返回最后一行（判定结果）。env 显式给 ALLOW_MIGRATION_FAILURE：空串 = 未声明。 */
function gate(env) {
  const out = execFileSync(PY, ['deploy/deploy.py', '--migration-gate'], {
    cwd: ROOT,
    encoding: 'utf8',
    env: { ...process.env, ...env },
    stdio: ['ignore', 'pipe', 'pipe']
  })
  return out.trim().split(/\r?\n/).pop()
}

test('未声明 ALLOW_MIGRATION_FAILURE ⇒ abort（默认硬失败，不靠改日志绕过）', () => {
  assert.equal(gate({ ALLOW_MIGRATION_FAILURE: '' }), 'abort')
})

test('ALLOW_MIGRATION_FAILURE=1 ⇒ continue（显式逃生开关）', () => {
  assert.equal(gate({ ALLOW_MIGRATION_FAILURE: '1' }), 'continue')
})

test('其他取值（0/true/yes）⇒ 仍 abort（只有显式 1 才放行）', () => {
  for (const v of ['0', 'true', 'yes']) {
    assert.equal(gate({ ALLOW_MIGRATION_FAILURE: v }), 'abort', '取值 ' + v)
  }
})
