// 管理端「一页一键」锁（#1639）。
//
// 判据：自成标签的管理页（侧栏叶子；被列表页 activeRouteNames 收编的详情页不算）**两两能力位不同**。
// 收敛前 admin.access 一个键盖住五页、catalog.manage 盖住三页、content.manage 盖住两页，
// 「只给用户管理、不给讲师管理」在授权界面上根本表达不出来。
//
// 这些能力位是否存在于后端能力表**靠类型就够**：PageDescriptor.capability 的类型是
// AuthzCapability（由 config/authz.ts 生成，事实源是 backend/internal/authz/authz.go），
// 键写错或键被删掉时 npx vue-tsc --noEmit 直接报错——这里不再另写一份键清单。
import { describe, expect, it } from 'vitest'
import { pages } from '../pages'
import { tabOwnerOf } from '../navigation'

describe('管理端一页一键', () => {
  it('自成标签的管理页能力位两两不同', () => {
    const leaves = pages.filter(p => p.workspace === 'manage' && tabOwnerOf(p.name) === null)
    expect(leaves.length).toBeGreaterThan(1)

    const byCapability = new Map<string, string[]>()
    for (const page of leaves) {
      // 无能力位的页（「无管理权限」页刻意不声明）不参与撞键判定,单独归一档
      const key = page.capability ?? '(无能力位)'
      byCapability.set(key, [...(byCapability.get(key) ?? []), page.name])
    }
    const shared = [...byCapability.entries()].filter(([, names]) => names.length > 1)
    expect(shared).toEqual([])
  })

  it('被列表页收编的详情页与列表页共享能力位（不该各自占一个键）', () => {
    const edit = pages.find(p => p.name === 'AdminFeaturedContentEdit')
    expect(edit?.capability).toBe('featured.manage')
    expect(tabOwnerOf('AdminFeaturedContentEdit')).toBe('AdminFeaturedContentList')
  })
})
