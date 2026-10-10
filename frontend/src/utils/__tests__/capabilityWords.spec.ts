// 能力中文词表（#1630）：角色权限页此前把能力键与资源域原样印给超管看（一屏英文）。
//
// 穷尽性由类型系统保证（`Record<AuthzCapability, string>` + `Record<CapabilityDomain, string>`，
// 而后者的域集合又是从键的模板字面量推断出来的）—— 本用例锁的是**形状**：
// 词表是不是中文、域表与键表出现的域是否一一对应、未登记键的降级是否可见。
import { describe, expect, it } from 'vitest'
import {
  CAPABILITY_DOMAIN_LABELS,
  CAPABILITY_LABELS,
  describeCapability,
  describeCapabilityDomain
} from '@/utils/capabilityWords'
import { pages } from '@/config/pages'
import type { AuthzCapability } from '@/config/authz'

const keys = Object.keys(CAPABILITY_LABELS) as AuthzCapability[]
const CJK = /[\u4e00-\u9fa5]/

describe('能力中文词表', () => {
  it('每个能力都有中文名，且不是把键原样印上去', () => {
    // 下限只是防「表被清空」的兜底；真正的完整性判据是 Record 的穷尽类型（type-check 面）
    expect(keys.length).toBeGreaterThan(40)
    for (const key of keys) {
      const label = describeCapability(key)
      expect(label, key).toMatch(CJK)
      expect(label, key).not.toBe(key)
    }
  })

  it('域表覆盖键表里出现过的每一个域（域由键派生，不另立清单）', () => {
    const domains = [...new Set(keys.map(key => key.split('.')[0]))].sort()
    expect(Object.keys(CAPABILITY_DOMAIN_LABELS).sort()).toEqual(domains)
    for (const domain of domains) {
      expect(describeCapabilityDomain(domain), domain).toMatch(CJK)
    }
  })

  it('页面描述符声明过的能力都有中文名（词表与真实用面同步）', () => {
    const used = [...new Set(pages.map(p => p.capability).filter(Boolean))] as string[]
    expect(used.length).toBeGreaterThan(20)
    for (const key of used) {
      expect(describeCapability(key), key).toMatch(CJK)
      expect(describeCapability(key), key).not.toBe(key)
    }
  })

  it('未登记的键/域回落为原值（后端加了能力而前端产物未再生成时降级可见，不静默留空）', () => {
    expect(describeCapability('future.thing')).toBe('future.thing')
    expect(describeCapabilityDomain('future')).toBe('future')
  })
})
