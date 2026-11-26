// 能力中文词表（#1630）：角色权限页此前把能力键原样印给超管看（一屏英文）。
// 穷尽性由 `Record<AuthzCapability, string>` 保证；本用例锁中文标签、真实消费面与未知键降级。
import { describe, expect, it } from 'vitest'
import { CAPABILITY_LABELS, describeCapability } from '@/utils/capabilityWords'
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

  it('页面描述符声明过的能力都有中文名（词表与真实用面同步）', () => {
    const used = [...new Set(pages.map(p => p.capability).filter(Boolean))] as string[]
    expect(used.length).toBeGreaterThan(20)
    for (const key of used) {
      expect(describeCapability(key), key).toMatch(CJK)
      expect(describeCapability(key), key).not.toBe(key)
    }
  })

  it('admin.access 的措辞覆盖管理端入口与工作台', () => {
    // 锁的是 #1645 订正的那处漂移：#1639 之后用户管理已是独立的 hrwai_user.manage，
    // 这句再写「用户管理」就会骗超管 —— 他以为勾了入口键就等于给了用户管理。
    expect(describeCapability('admin.access')).toBe('管理端入口与工作台')
  })

  it('未登记的键回落为原值（后端加了能力而前端产物未再生成时降级可见，不静默留空）', () => {
    expect(describeCapability('future.thing')).toBe('future.thing')
  })
})
