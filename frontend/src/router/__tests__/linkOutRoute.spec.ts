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

  it('中转页是独立页面：不挂布局外壳（无侧栏/主题入口），工作区与登录要求不丢', () => {
    const resolved = router.resolve({ name: routeNames.LinkOut })
    expect(resolved.meta.workspace).toBe('training')
    expect(resolved.meta.requiresAuth).toBe(true)
    // 顶层记录：matched 只有自身一条 —— 没有布局外壳，也就没有侧栏与主题切换入口
    expect(resolved.matched.length).toBe(1)
    expect(resolved.matched[0].path).toBe(FORUM_LINK_OUT_PATH)
    // 角色约束不因脱离布局而丢失（原由布局 meta.role: hrwai_user 继承）
    expect(resolved.meta.roles).toEqual(['hrwai_user'])
  })

  it('反查：LinkOut 路由的完整路径就是常量本身（改名漏改即红）', () => {
    const byName = router.resolve({ name: routeNames.LinkOut })
    expect(byName.path).toBe(FORUM_LINK_OUT_PATH)
  })
})
