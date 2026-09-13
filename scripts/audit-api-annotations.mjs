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
 * 输出三段：
 *   A. 未在 @Success 里指认 data DTO 的端点（codegen 的输入缺口：只有指认了类型才生成得出来）
 *   B. swagger 有、API.md 无的端点（文档滞后面）
 *   C. API.md 有、swagger 无的端点（注解缺口：可能是没注解，也可能是本文档写了不存在的端点）
 *
 * 只读脚本：不改任何文件。审计结论写进 PR 证据；缺口补齐按域另立片（spec #940 片五决策 20）。
 */
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..')
const BT = String.fromCharCode(96)

/** 规范化路径：去查询串、补 /api 前缀、{id} → :id。 */
export function normalizePath(p) {
  let out = String(p).split('?')[0].trim().replace(/\\+$/, '')
  if (!out.startsWith('/api')) out = out.startsWith('/') ? '/api' + out : out
  out = out.replace(/\{([a-zA-Z_]+)\}/g, ':$1')
  return out.replace(/\/+$/, '') || '/'
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
  const sw = new Map()
  for (const [p, ops] of Object.entries(swagger.paths || {})) {
    for (const [m, op] of Object.entries(ops)) {
      if (!['get', 'post', 'put', 'delete', 'patch'].includes(m)) continue
      sw.set(m.toUpperCase() + ' ' + normalizePath(p), {
        path: normalizePath(p),
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
      if (['GET', 'POST', 'PUT', 'DELETE', 'PATCH'].includes(m)) md.set(m + ' ' + normalizePath(path), true)
    }
  }
  const noData = [...sw.values()].filter((e) => !e.data).map((e) => e.method + ' ' + e.path).sort()
  const onlySwagger = [...sw.keys()].filter((k) => !md.has(k)).sort()
  const onlyApiMd = [...md.keys()].filter((k) => !sw.has(k)).sort()
  return { total: sw.size, withData: sw.size - noData.length, noData, onlySwagger, onlyApiMd }
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
}

if (process.argv[1] && import.meta.url === new URL('file://' + process.argv[1]).href) main()
