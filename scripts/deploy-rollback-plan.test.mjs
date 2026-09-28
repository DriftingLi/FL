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
import { execFileSync, spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
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

/** 跑一次 --image-match dry-run，返回 {status, out}：退出码就是自证判据。 */
function imageMatch(actual, expected) {
  const r = spawnSync('bash', ['scripts/deploy-remote.sh', '--image-match', actual, expected], {
    cwd: ROOT,
    encoding: 'utf8',
    env: { ...process.env },
    stdio: ['ignore', 'pipe', 'pipe']
  })
  return { status: r.status, out: (r.stdout || '').trim() }
}

test('镜像自证：一致 ⇒ 退出码 0；不一致 ⇒ 非零且打印 mismatch', () => {
  const same = imageMatch('ghcr.io/org/fl-backend:sha-a', 'ghcr.io/org/fl-backend:sha-a')
  assert.equal(same.status, 0, '一致时必须 0（否则回滚会被误判为失败）')
  const bad = imageMatch('ghcr.io/org/fl-backend:sha-new', 'ghcr.io/org/fl-backend:sha-old')
  assert.notEqual(bad.status, 0, '不一致必须非零（否则「回滚成功」会被假打印）')
  assert.equal(bad.out.split('\n').pop(), 'mismatch')
})

// 结构判据（照 check-async-section.mjs 的文本守卫手法）：回滚必须只改 tag 变量、必须带自证，
// 且不得再出现历史那份「export BACKEND_IMAGE=」—— write_env_file 不读它，正是空操作回滚的成因。
test('回滚接线：只改 IMAGE_TAG_* + write_env_file + assert_running_image，且不再 export BACKEND_IMAGE', () => {
  const src = readFileSync(resolve(ROOT, 'scripts/deploy-remote.sh'), 'utf8')
  const start = src.indexOf('do_rollback() {')
  assert.ok(start > 0, '找不到 do_rollback')
  const end = src.indexOf('\nmain\n', start)
  const body = src.slice(start, end > 0 ? end : src.length)
  for (const needle of ['export IMAGE_TAG_BACKEND=', 'export IMAGE_TAG_FRONTEND=', 'write_env_file', 'assert_running_image']) {
    assert.ok(body.includes(needle), 'do_rollback 缺少：' + needle)
  }
  for (const banned of ['export BACKEND_IMAGE=', 'export FRONTEND_IMAGE=']) {
    assert.ok(!body.includes(banned), 'do_rollback 又出现 ' + banned + '（write_env_file 不读它 ⇒ 空操作回滚回归）')
  }
})
