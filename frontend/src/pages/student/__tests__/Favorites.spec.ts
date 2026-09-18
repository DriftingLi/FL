// Favorites（#1089）：多态收藏的**落点表**契约 —— 五个 target_type 各有落点，
// 章节落点消费后端新给的 course_id（缺失时不可点，避免"点了没反应"）。
// seam：页面组件层，mock '@/api/favorite'（不依赖真实后端）。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

const h = vi.hoisted(() => ({ push: vi.fn() }))

vi.mock('vue-router', () => ({ useRouter: () => ({ push: h.push }) }))
vi.mock('@/api/favorite', () => ({ favoriteApi: { list: vi.fn(), remove: vi.fn() } }))

import { favoriteApi } from '@/api/favorite'
import Favorites from '../Favorites.vue'

/** 收藏条目（形状照生成物 FavoriteDTO；course_id 仅章节有意义，其余为 0） */
function fav(over: Record<string, unknown> = {}) {
  return {
    favorite_id: 1,
    target_type: 'course',
    target_id: 7,
    title: '标题',
    cover: '',
    created_at: '2026-09-01T10:00:00+08:00',
    course_id: 0,
    ...over
  }
}

async function mountWith(items: Record<string, unknown>[]) {
  vi.mocked(favoriteApi.list).mockResolvedValue({
    favorites: items,
    page: 1,
    pages: 1,
    total: items.length
  } as never)
  const w = mount(Favorites, { global: { plugins: [epLite()] } })
  await flushPromises()
  return w
}

beforeEach(() => {
  h.push.mockClear()
  vi.mocked(favoriteApi.list).mockReset()
})

describe('我的收藏落点表（#1089）', () => {
  it.each([
    { type: 'course', id: 7, courseId: 0, path: '/training/courses?course_id=7' },
    { type: 'topic', id: 8, courseId: 0, path: '/training/forum/8' },
    { type: 'featured', id: 9, courseId: 0, path: '/training/featured/9' },
    { type: 'question', id: 10, courseId: 0, path: '/training/questions/10' },
    { type: 'chapter', id: 11, courseId: 3, path: '/training/course/3/chapter/11' }
  ])('$type 收藏点开落到 $path', async ({ type, id, courseId, path }) => {
    const w = await mountWith([fav({ target_type: type, target_id: id, course_id: courseId })])
    await w.find('.stagger-in').trigger('click')
    expect(h.push).toHaveBeenCalledWith(path)
  })

  it('章节落点消费后端下发的 course_id，不猜、不复用 target_id', async () => {
    const w = await mountWith([fav({ target_type: 'chapter', target_id: 4, course_id: 3 })])
    await w.find('.stagger-in').trigger('click')
    // 反例守卫：写成 target_id 会把章节 id 当课程 id（旧缺陷的形态）
    expect(h.push).not.toHaveBeenCalledWith('/training/course/4/chapter/4')
    expect(h.push).toHaveBeenCalledWith('/training/course/3/chapter/4')
  })

  it('章节缺 course_id 时是**不可点态**（无 pointer 样式、点击不跳转）', async () => {
    const w = await mountWith([fav({ target_type: 'chapter', target_id: 4, course_id: 0 })])
    const row = w.find('.stagger-in')
    expect(row.classes()).not.toContain('cursor-pointer')
    await row.trigger('click')
    expect(h.push).not.toHaveBeenCalled()
  })

  it('可点行仍带 pointer 样式（防"不可点"被写成恒真）', async () => {
    const w = await mountWith([fav({ target_type: 'chapter', target_id: 4, course_id: 3 })])
    expect(w.find('.stagger-in').classes()).toContain('cursor-pointer')
  })
})
