// 回滚镜像标签解析的 dry-run 判据物（真实缺陷 #3 / spec #1345）。
//
// 缺陷形状：do_rollback 曾 export BACKEND_IMAGE=<历史镜像>，而 write_env_file 只读
// IMAGE_BACKEND:IMAGE_TAG_BACKEND ⇒ .env 写回的是**本次失败的 tag**，compose 判定无变更、
// 坏版本继续在线（回滚是空操作，健康检查偶然通过还会打印「回滚成功」）。
//
// 现在回滚只改 tag 变量（.env 仍是 IMAGE_*:TAG 的函数）；这里断言唯一判据点
// rollback_tag 的解析结果 —— 完整镜像坐标 ⇒ tag，无 tag / unknown / 缺省 ⇒ 空。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')

/** 跑一次回滚计划 dry-run，返回打印出的 KEY=VALUE 表（不碰 docker、不写文件）。 */
function plan(env) {
  const out = execFileSync('bash', ['scripts/deploy-remote.sh', '--rollback-plan'], {
    cwd: ROOT,
    encoding: 'utf8',
    env: { ...process.env, ...env },
    stdio: ['ignore', 'pipe', 'pipe']
  })
  const kv = {}
  for (const line of out.trim().split('\n')) {
    const m = line.match(/^([A-Z_]+)=(.*)$/)
    if (m) kv[m[1]] = m[2]
  }
  return kv
}

test('回滚计划：从完整镜像坐标解析出历史 tag', () => {
  const p = plan({
    PREVIOUS_BACKEND_IMAGE: 'ghcr.io/org/fl-backend:sha-old999',
    PREVIOUS_FRONTEND_IMAGE: 'ghcr.io/org/fl-frontend:sha-old888'
  })
  assert.equal(p.IMAGE_TAG_BACKEND, 'sha-old999')
  assert.equal(p.IMAGE_TAG_FRONTEND, 'sha-old888')
})

test('回滚计划：带端口的仓库地址取最后一个冒号之后（不能把端口当 tag）', () => {
  const p = plan({
    PREVIOUS_BACKEND_IMAGE: 'registry.local:5000/org/fl-backend:v1.2.3',
    PREVIOUS_FRONTEND_IMAGE: 'registry.local:5000/org/fl-frontend:v1'
  })
  assert.equal(p.IMAGE_TAG_BACKEND, 'v1.2.3')
  assert.equal(p.IMAGE_TAG_FRONTEND, 'v1')
})

test('回滚计划：无 tag / unknown / 缺省 ⇒ 空（调用方按「无历史版本」处理，不改写 .env）', () => {
  assert.equal(plan({ PREVIOUS_BACKEND_IMAGE: 'ghcr.io/org/fl-backend' }).IMAGE_TAG_BACKEND, '')
  assert.equal(plan({ PREVIOUS_BACKEND_IMAGE: 'unknown' }).IMAGE_TAG_BACKEND, '')
  assert.equal(plan({}).IMAGE_TAG_BACKEND, '')
  assert.equal(plan({}).IMAGE_TAG_FRONTEND, '')
})
