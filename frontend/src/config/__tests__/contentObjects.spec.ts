// 内容对象表一致性锁（第十二波票 2，#1168；互等锁形态照页面描述符 spec 先例）。
// 锁五件事：
//   1. 表与后端契约枚举互等（搜索 type / 收藏 target_type 双向）——新增种类漏登记即红；
//   2. 收藏 tab 集合钉到 2026-09-18 裁定（featured 刻意排除，理由在表注释）——派生不得顺手合并；
//   3. 每个种类必须有落点装配（to 必填槽），章节缺父课程时降级为 null（不可点，不猜落点）；
//   4. 称谓与标签色逐项锁定（搜索页/收藏页共用后，文案漂移只会红一次）；
//   5. 可收藏种类投影与表 favoritable 槽互等（ADR-0060 决策 3：收藏开关的编译期收窄唯一来自表）。
import { describe, it, expect } from 'vitest'
import router from '@/router'
import type { FavoriteTargetType } from '@/api/favorite'
import type { SearchType } from '@/api/search'
import { CONTENT_OBJECTS, FAVORITABLE_CONTENT_OBJECT_KEYS, contentObjectByKey, contentObjectBySearchType, favoriteTabContentObjects, searchableContentObjects, type ContentObjectKey } from '../contentObjects'

// 契约枚举的前端真源是 api/search.ts / api/favorite.ts 的 union；这里以穷尽 Record 保证
// union 增员时本测试编译期失败（而不是悄悄少测一种）。
const SEARCH_TYPE_EXHAUSTIVE: Record<SearchType, true> = {
  course: true,
  chapter: true,
  question: true,
  content: true,
  topic: true
}
const FAVORITE_TYPE_EXHAUSTIVE: Record<FavoriteTargetType, true> = {
  course: true,
  chapter: true,
  question: true,
  featured: true,
  topic: true
}
const KEY_EXHAUSTIVE: Record<ContentObjectKey, true> = {
  course: true,
  chapter: true,
  question: true,
  featured: true,
  topic: true
}

describe('内容对象表 ↔ 契约枚举', () => {
  it('可检索种类的 searchType 与搜索契约枚举互等', () => {
    const declared = searchableContentObjects().map(o => o.searchType)
    expect([...declared].sort()).toEqual(Object.keys(SEARCH_TYPE_EXHAUSTIVE).sort())
    for (const t of Object.keys(SEARCH_TYPE_EXHAUSTIVE) as SearchType[]) {
      expect(contentObjectBySearchType(t), '搜索种类 ' + t + ' 不在表里').toBeTruthy()
    }
  })

  it('可收藏种类的 favoriteTargetType 与收藏契约枚举互等', () => {
    const declared = CONTENT_OBJECTS.filter(o => o.favoritable).map(o => o.favoriteTargetType)
    expect([...declared].sort()).toEqual(Object.keys(FAVORITE_TYPE_EXHAUSTIVE).sort())
  })

  // 收藏开关（composables/useFavorite.ts）的 key 参数在类型层收窄到 FAVORITABLE_CONTENT_OBJECT_KEYS，
  // 那个投影列表与表 favoritable 槽的互等由本条钉：表把某种类改成不可收藏而忘从投影剔除、
  // 或新增可收藏种类漏进投影，都判红（编译期的 as const satisfies 只保证投影里的种类真实存在于表）。
  it('可收藏种类投影 = 表 favoritable=true 那一档（收藏入口类型收窄的唯一来源）', () => {
    const fromTable = CONTENT_OBJECTS.filter(o => o.favoritable).map(o => o.key)
    expect([...FAVORITABLE_CONTENT_OBJECT_KEYS].sort()).toEqual([...fromTable].sort())
    for (const key of FAVORITABLE_CONTENT_OBJECT_KEYS) {
      const entry = contentObjectByKey(key)
      expect(entry, key + ' 不在内容对象表里').toBeTruthy()
      expect(entry!.favoritable, key + ' 被投影成可收藏，但表里 favoritable=false').toBe(true)
    }
  })

  it('表 key 集合穷尽 ContentObjectKey', () => {
    const keys = CONTENT_OBJECTS.map(o => o.key)
    expect(keys.length).toBe(new Set(keys).size)
    expect([...keys].sort()).toEqual(Object.keys(KEY_EXHAUSTIVE).sort())
  })

  it('收藏 tab 集合 = 四类（featured 刻意排除，#1132 裁定）', () => {
    expect(favoriteTabContentObjects().map(o => o.key)).toEqual(['course', 'chapter', 'question', 'topic'])
  })
})

describe('内容对象表槽位', () => {
  it('称谓与标签色逐项锁定（两页共用后的唯一出处）', () => {
    expect(CONTENT_OBJECTS.map(o => [o.label, o.tone])).toEqual([
      ['课程', 'primary'],
      ['章节', 'success'],
      ['题目', 'warning'],
      ['内容精选', 'info'],
      ['帖子', 'danger']
    ])
  })

  it('每个种类都给全参数时必有落点；章节缺父课程降级为 null', () => {
    for (const o of CONTENT_OBJECTS) {
      expect(o.to({ id: 7, parentId: 3 }), o.key + ' 无落点装配').toBeTruthy()
    }
    expect(contentObjectByKey('chapter')!.to({ id: 7 })).toBeNull()
    expect(contentObjectByKey('chapter')!.to({ id: 7, parentId: 0 })).toBeNull()
  })

  // 形态照 pages.spec「导航 routeName 都能在路由表找到」：落点写错路由名不再靠肉眼——编译期 RouteName union + 这里的路由表实锁
  it('落点的路由名都能在路由表解析出非空 path（断链落点不可能上线）', () => {
    for (const o of CONTENT_OBJECTS) {
      const target = o.to({ id: 7, parentId: 3 })!
      const resolved = router.resolve({ name: target.name, params: target.params, query: target.query })
      expect(resolved.path, o.key + ' 落点路由 ' + target.name + ' 不在路由表').not.toBe('/')
      expect(resolved.name).toBe(target.name)
    }
  })

  it('表序 = 搜索分区序（课程/章节/题目/内容精选/帖子）', () => {
    expect(searchableContentObjects().map(o => o.label)).toEqual(['课程', '章节', '题目', '内容精选', '帖子'])
  })
})
