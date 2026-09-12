// 「即将离开本站」中转页的路径一致性（#881 / ADR-0044）。
//
// ForumContent 在 AST 层改写 href 时只能写路径字面量，而路由表是别名 + 相对子路径。
// 两处不同步 → 站外链接静默 404（无报错、无类型提示）。本文件就是那道闸门。
import { describe, it, expect } from 'vitest'
import router from '../index'
import { routeNames } from '@/config/routeNames'
import { FORUM_LINK_OUT_PATH } from '@/config/forumLinks'

describe('外链中转页路由（#881）', () => {
  it('config 里的中转路径确实指向 LinkOut 路由', () => {
    const resolved = router.resolve(FORUM_LINK_OUT_PATH)
    expect(resolved.name).toBe(routeNames.LinkOut)
  })

  it('中转页在学员工作区下（继承布局与主题入口，且带上工作区声明）', () => {
    const resolved = router.resolve({ name: routeNames.LinkOut })
    expect(resolved.meta.workspace).toBe('training')
    expect(resolved.meta.requiresAuth).toBe(true)
  })

  it('反查：LinkOut 路由的完整路径就是常量本身（改名漏改即红）', () => {
    const byName = router.resolve({ name: routeNames.LinkOut })
    expect(byName.path).toBe(FORUM_LINK_OUT_PATH)
  })
})
