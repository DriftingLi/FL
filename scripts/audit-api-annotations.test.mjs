#!/usr/bin/env node
/**
 * 注解覆盖审计的判定逻辑自检（issue #1425）。
 *
 * 为什么这张自检存在：`audit-api-annotations.mjs` 旧版把 swagger 侧与 API.md 侧过**同一个**
 * `normalizePath`，而它会「给一切非 /api 开头的路径无条件补 /api」。于是注册在 gin 根引擎上的
 * `GET /static/*filepath`（对外就没有 /api 前缀）被永久报成 C 段的 `/api/static/*` ——
 * **一个无论补注解、无论改文档都消不掉的假阳性**；更糟的是它把人引向「给这条补 @Router」，
 * 那会经 `basePath=/api` 渲染出一条对外宣称存在、实际 404 的假契约。
 * 这个错误是真实代价：它被抄进 #1416 的票面当判据，直到动手改文档才被证伪。
 *
 * 自检用**夹具**而不是仓库真实文件（判据逻辑不该耦合文档演化），只用一条真实文件不变量兜住回归本身。
 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { audit, publicPath, swaggerPath, ROOT_LEVEL_PREFIXES } from './audit-api-annotations.mjs'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..')

const swagger = {
  basePath: '/api',
  paths: {
    '/': { get: { responses: { '200': { schema: {} } } } },
    '/auth/login': { post: { responses: { '200': { schema: { allOf: [{ properties: { data: { $ref: '#/definitions/service.LoginResult' } } }] } } } } },
    '/valuation/health': { get: { responses: { '200': { schema: {} } } } }
  }
}

const apiMd = [
  '| 方法 | 路径 | 说明 |',
  '|---|---|---|',
  '| GET | `/api` | 服务信息（根路由） |',
  '| POST | `/api/auth/login` | 登录 |',
  '| GET | `/api/valuation/health` | 子模块探活 |',
  '| GET/HEAD | `/static/*filepath` | 静态资源与上传文件 |',
  '| POST | `/api/valuation/auth/login` | 幽灵：注册面上根本没有这条 |',
  ''
].join('\n')

const r = audit(swagger, apiMd)

test('C 段只收真缺口：basePath 之外的根级路由不得混进来', () => {
  assert.deepEqual(r.onlyApiMd, ['POST /api/valuation/auth/login'])
})

test('根级路由单列 D 段：对外路径逐字保留、不被补成 /api/static', () => {
  assert.deepEqual(r.rootLevel, ['GET /static/*filepath'])
  assert.equal(
    [...r.onlyApiMd, ...r.rootLevel].some((k) => k.includes('/api/static')),
    false,
    '出现 /api/static 说明又给根级路由补了前缀（#1425 的原缺陷回归）'
  )
})

test('两侧口径：swagger 侧接 basePath，API.md 侧原样视为对外 URL', () => {
  assert.equal(swaggerPath('/', '/api'), '/api') // 根路由这条对应 GET /api 本身
  assert.equal(swaggerPath('/valuation/health', '/api'), '/api/valuation/health')
  assert.equal(publicPath('/static/*filepath'), '/static/*filepath')
  assert.equal(publicPath('/auth/{id}?x=1'), '/auth/:id')
  // 夹具里这三条都双向命中 ⇒ 既不落 B 也不落 C
  assert.deepEqual(r.onlySwagger, [])
})

test('D 段的桶不能吞掉普通缺口（逃生口检查）', () => {
  // ⚠️ 说实话版：这条交叉断言**与实现共用同一张 ROOT_LEVEL_PREFIXES**，所以单独加宽 allowlist 时它会
  // 自洽地跟着变绿 —— 真正承重的是上面那条 `onlyApiMd` 的 deepEqual（幽灵一旦被塞进 D，deepEqual 立刻红）。
  // 本条的作用是让 allowlist 为空/被清空这类「D 形同虚设」的退化显形，别把它当主判据。
  assert.ok(ROOT_LEVEL_PREFIXES.length > 0, 'allowlist 为空则 D 段形同虚设，夹具断言会假绿')
  for (const k of r.onlyApiMd) assert.ok(!isRoot(k), '根级路由被判成 C 段缺口：' + k)
  for (const k of r.rootLevel) assert.ok(isRoot(k), '非根级路由被塞进 D 段（吞掉真缺口的逃生口）：' + k)
})
function isRoot (k) {
  const p = k.slice(k.indexOf(' ') + 1)
  return ROOT_LEVEL_PREFIXES.some((pre) => p === pre || p.startsWith(pre + '/'))
}

test('真实文件不变量：仓库现状里 C 段不得再出现 /api/static', () => {
  const real = audit(
    JSON.parse(readFileSync(join(ROOT, 'backend', 'docs', 'swagger.json'), 'utf8')),
    readFileSync(join(ROOT, 'API.md'), 'utf8')
  )
  assert.equal(
    real.onlyApiMd.filter((k) => k.includes('/api/static')).length,
    0,
    'C 段又出现 /api/static：假阳性回归（#1425）'
  )
})
