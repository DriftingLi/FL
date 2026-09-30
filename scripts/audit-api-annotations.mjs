#!/usr/bin/env node
/**
 * 注解覆盖审计（spec #940 片五②）：把「注解产物」与「手写契约文档」的差集打出来。
 *
 * 用法：node scripts/audit-api-annotations.mjs [--json]
 *
 * 输入：
 *   backend/docs/swagger.json   —— 注解产物（唯一事实源；再生成：cd backend && make swagger）
 *   API.md                      —— 手写契约文档（人类可读叙述面）
 *
 * 输出四段：
 *   A. 未在 @Success 里指认 data DTO 的端点（codegen 的输入缺口：只有指认了类型才生成得出来）
 *   B. swagger 有、API.md 无的端点（文档滞后面）
 *   C. API.md 有、swagger 无的端点（注解缺口：可能是没注解，也可能是本文档写了不存在的端点）
 *   D. 落在 swagger `basePath` 之外的根级路由（**结构上不可能进这份文档**，既不是缺注解也不是幽灵）
 *
 * 为什么要有 D：`API.md` 写的是**对外 URL**，而 swagger 文档的 `basePath` 是 `/api`。有些路由注册在
 * gin 的**根引擎**上（先例：`internal/api/router.go` 的 `r.GET("/static/*filepath")`，nginx 侧也是
 * `location /static/`），它们对外就没有 `/api` 前缀。旧实现把**两侧**都过同一个 `normalizePath`，
 * 而它会「给一切非 `/api` 开头的路径无条件补 `/api`」⇒ 这类路由被永久报成 C 段的
 * `/api/static/*`：**一个无论补注解还是改文档都消不掉的假阳性**（补注解更糟 —— `basePath` 会把它
 * 渲染成一条对外宣称存在、实际 404 的假契约）。判据与取证见 issue #1425。
 *
 * 只读脚本：不改任何文件。审计结论写进 PR 证据；缺口补齐按域另立片（spec #940 片五决策 20）。
 */
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..')
const BT = String.fromCharCode(96)

/** 归一化**对外 URL**：去查询串、`{id}` → `:id`、折叠尾斜杠。不补任何前缀 —— API.md 写的就是对外路径。 */
export function publicPath(p) {
  const out = String(p).split('?')[0].trim().replace(/\\+$/, '')
  return out.replace(/\{([a-zA-Z_]+)\}/g, ':$1').replace(/\/+$/, '') || '/'
}

/** swagger 侧的对外 URL = 文档的 basePath + path（`/` 这条对应 `GET /api` 本身）。 */
export function swaggerPath(p, basePath) {
  const base = String(basePath || '').replace(/\/+$/, '')
  return publicPath(base + (p.startsWith('/') ? p : '/' + p))
}

/**
 * `basePath` 之外的根级路由前缀（D 段的判据源）。**故意保守**：只有确实注册在 gin 根引擎上的前缀才列。
 * 加进这张表 = 宣称「这条路由对外就长这样，swagger 文档表达不了它」⇒ 每条都必须能指到注册处，
 * 否则它就会从 C 段（看得见缺口）悄悄挪进 D 段（当成结构性豁免）——那是吞掉真缺口的逃生口。
 */
export const ROOT_LEVEL_PREFIXES = ['/static']

export function isRootLevel(p) {
  return ROOT_LEVEL_PREFIXES.some((pre) => p === pre || p.startsWith(pre + '/'))
}

/** swagger 响应里的 data DTO 引用（可能直接 $ref，也可能包在 allOf 里）。 */
export function dataRef(schema) {
  if (!schema) return ''
  for (const part of schema.allOf || []) {
    const d = (part && part.properties && part.properties.data) || null
    if (d && d.$ref) return d.$ref.split('/').pop()
  }
  return ''
}

export function audit(swagger, apiMd) {
  const base = swagger.basePath
  const sw = new Map()
  for (const [p, ops] of Object.entries(swagger.paths || {})) {
    for (const [m, op] of Object.entries(ops)) {
      if (!['get', 'post', 'put', 'delete', 'patch'].includes(m)) continue
      const full = swaggerPath(p, base)
      sw.set(m.toUpperCase() + ' ' + full, {
        path: full,
        method: m.toUpperCase(),
        data: dataRef((op.responses && op.responses['200'] && op.responses['200'].schema) || null)
      })
    }
  }
  const md = new Map()
  for (const line of String(apiMd).split('\n')) {
    if (!line.startsWith('|')) continue
    const cells = line.replace(/^\|/, '').replace(/\|\s*$/, '').split('|').map((c) => c.trim())
    if (cells.length < 2) continue
    const path = cells[1].split(BT).join('')
    if (!path.startsWith('/')) continue
    for (const m of cells[0].replace(/\s/g, '').split('/')) {
      if (['GET', 'POST', 'PUT', 'DELETE', 'PATCH'].includes(m)) md.set(m + ' ' + publicPath(path), true)
    }
  }
  const noData = [...sw.values()].filter((e) => !e.data).map((e) => e.method + ' ' + e.path).sort()
  const onlySwagger = [...sw.keys()].filter((k) => !md.has(k)).sort()
  // C 段先收全量「API.md 有、swagger 无」，再把落在 basePath 之外的那部分单列成 D：
  // 它们不是缺口，是这份文档结构上表达不了的对外根级路由（见文件头 D 的说明）。
  const missingFromSwagger = [...md.keys()].filter((k) => !sw.has(k))
  const rootLevel = missingFromSwagger.filter((k) => isRootLevel(k.slice(k.indexOf(' ') + 1))).sort()
  const onlyApiMd = missingFromSwagger.filter((k) => !isRootLevel(k.slice(k.indexOf(' ') + 1))).sort()
  return { total: sw.size, withData: sw.size - noData.length, noData, onlySwagger, onlyApiMd, rootLevel }
}

function main() {
  const swagger = JSON.parse(readFileSync(join(ROOT, 'backend', 'docs', 'swagger.json'), 'utf8'))
  const apiMd = readFileSync(join(ROOT, 'API.md'), 'utf8')
  const r = audit(swagger, apiMd)
  if (process.argv.includes('--json')) {
    console.log(JSON.stringify(r, null, 2))
    return
  }
  console.log('===== 注解覆盖审计（spec #940 片五②）=====')
  console.log('swagger 端点 ' + r.total + ' | 指认了 data DTO ' + r.withData + ' | 未指认 ' + r.noData.length)
  console.log('\n--- A. 未在 @Success 指认 data DTO（' + r.noData.length + '）---')
  for (const x of r.noData) console.log('  ' + x)
  console.log('\n--- B. swagger 有、API.md 无（' + r.onlySwagger.length + '）---')
  for (const x of r.onlySwagger) console.log('  ' + x)
  console.log('\n--- C. API.md 有、swagger 无（' + r.onlyApiMd.length + '）---')
  for (const x of r.onlyApiMd) console.log('  ' + x)
  console.log('\n--- D. basePath 之外的根级路由（不进 C：这份文档表达不了它）（' + r.rootLevel.length + '）---')
  for (const x of r.rootLevel) console.log('  ' + x)
}

if (process.argv[1] && import.meta.url === new URL('file://' + process.argv[1]).href) main()
