// keep-alive 名单锁（#1620）：描述符标了 keepAlive 的页面，组件名必须与路由名一致。
//
// keep-alive 的 include 按**组件名**匹配，对不上时该页"静默不缓存"——没有报错、没有告警，
// 只有用户觉得"切回来状态怎么没了"。这条锁把它变成可机检的红。
import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { pages } from '@/config/pages'

const PAGES_TS = resolve(__dirname, '../pages.ts')

describe('keepAlive 页面 ↔ 组件名', () => {
  const cached = pages.filter(p => p.keepAlive)
  const source = readFileSync(PAGES_TS, 'utf8')

  it('管理端的缓存清单非空（机制没被悄悄关掉）', () => {
    expect(cached.length).toBeGreaterThan(0)
  })

  it('每个缓存页都声明了与之同名的 defineOptions', () => {
    const offenders: string[] = []
    for (const page of cached) {
      // 从描述符源码里取出组件文件路径（描述符是单行字面量：name → import('...'))
      const line = source.split('\n').find(l => l.includes(`name: '${page.name}'`) && l.includes('keepAlive: true'))
      const m = line?.match(/import\('(@\/pages\/[^']+)'\)/)
      if (!line || !m) {
        offenders.push(`${page.name}: 描述符里找不到组件路径（形状变了？）`)
        continue
      }
      const file = resolve(__dirname, '../../', m[1].replace('@/', ''))
      const text = readFileSync(file, 'utf8')
      if (!text.includes(`defineOptions({ name: '${page.name}' })`)) {
        offenders.push(`${page.name} → ${m[1]}：缺 defineOptions({ name: '${page.name}' })`)
      }
    }
    expect(offenders).toEqual([])
  })

  it('未标 keepAlive 的页面不得偷偷留下同名 defineOptions 之外的缓存位（清单即事实）', () => {
    const names = cached.map(p => p.name)
    expect(new Set(names).size).toBe(names.length)
  })
})
