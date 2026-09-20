// useFavorite：收藏开关的唯一状态机（第十三波票3，ADR-0060 决策 3）的接口级测试。
// seam：一个 module 的两个具名入口——useFavorite(key, id)（自带 favoriteApi.check 的单对象形态，
// 服务详情页/抽屉/面板）与 useFavoriteList(key)（不 check、态由列表行携带的列表形态）。
// favoriteApi 与 ElMessage 以 vi.mock 替身在 API 边界观测：状态机与 DOM 无关，故不挂载组件。
// 覆盖四段共享实现：查询 → 切换 → 提示 → 失败保持原态；另锁「哪种可收藏」的编译期收窄。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { favoriteApi } from '@/api/favorite'
import { CONTENT_OBJECTS, type ContentObjectKey, type FavoritableContentObjectKey } from '@/config/contentObjects'
import { useFavorite, useFavoriteList } from '../useFavorite'

vi.mock('element-plus', () => ({
  ElMessage: { success: vi.fn(), error: vi.fn(), warning: vi.fn() }
}))

vi.mock('@/api/favorite', () => ({
  favoriteApi: { check: vi.fn(), add: vi.fn(), remove: vi.fn(), list: vi.fn() }
}))

/** 收藏目标的响应替身（FavoriteDTO 其余字段与本流程无关） */
function added(favorite_id: number) {
  return { favorite_id } as never
}

function unchecked() {
  return { favorited: false, favorite_id: 0 }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(favoriteApi.check).mockResolvedValue(unchecked())
  vi.mocked(favoriteApi.add).mockResolvedValue(added(9))
  vi.mocked(favoriteApi.remove).mockResolvedValue(null)
})

describe('useFavorite（单对象形态：查询 → 切换 → 提示）', () => {
  it('load 先查态；未收藏时 toggle 走 add、回填态与 favorite_id 并提示「已收藏」', async () => {
    const fav = useFavorite('chapter', 7)

    await fav.load()
    expect(favoriteApi.check).toHaveBeenCalledWith({ target_type: 'chapter', target_id: 7 })
    expect(fav.favorited.value).toBe(false)

    expect(await fav.toggle()).toEqual({ favorited: true, favorite_id: 9 })
    expect(favoriteApi.add).toHaveBeenCalledWith({ target_type: 'chapter', target_id: 7 })
    expect(favoriteApi.remove).not.toHaveBeenCalled()
    expect(fav.favorited.value).toBe(true)
    expect(fav.favoriteId.value).toBe(9)
    expect(ElMessage.success).toHaveBeenCalledWith('已收藏')
  })

  it('查得已收藏时 toggle 走 remove、清态清 id 并提示「已取消收藏」', async () => {
    vi.mocked(favoriteApi.check).mockResolvedValue({ favorited: true, favorite_id: 7 })
    const fav = useFavorite('topic', () => 21)

    await fav.load()
    expect(fav.favorited.value).toBe(true)

    expect(await fav.toggle()).toEqual({ favorited: false, favorite_id: 0 })
    expect(favoriteApi.remove).toHaveBeenCalledWith(7)
    expect(favoriteApi.add).not.toHaveBeenCalled()
    expect(fav.favorited.value).toBe(false)
    expect(fav.favoriteId.value).toBe(0)
    expect(ElMessage.success).toHaveBeenCalledWith('已取消收藏')
  })

  it('target_type 逐次向内容对象表取：表里每个可收藏种类都能开关，且契约键与表一致', async () => {
    for (const entry of CONTENT_OBJECTS.filter(o => o.favoritable)) {
      vi.clearAllMocks()
      const fav = useFavorite(entry.key as FavoritableContentObjectKey, 3)
      await fav.load()
      await fav.toggle()
      expect(favoriteApi.check).toHaveBeenCalledWith({ target_type: entry.favoriteTargetType, target_id: 3 })
      expect(favoriteApi.add).toHaveBeenCalledWith({ target_type: entry.favoriteTargetType, target_id: 3 })
    }
  })

  it('切换前清掉上一目标的态：换 id 再 load 不残留上一章节的收藏态', async () => {
    vi.mocked(favoriteApi.check).mockResolvedValueOnce({ favorited: true, favorite_id: 7 })
    const id = ref(1)
    const fav = useFavorite('chapter', id)

    await fav.load()
    expect(fav.favorited.value).toBe(true)

    id.value = 2
    const pending = fav.load()
    // 查询在途期间即已是未收藏（四处迁移前均是「先清后查」的形状）
    expect(fav.favorited.value).toBe(false)
    await pending
    expect(favoriteApi.check).toHaveBeenLastCalledWith({ target_type: 'chapter', target_id: 2 })
    expect(fav.favorited.value).toBe(false)
  })

  it('load(targetId) 可显式指定查询目标：课程抽屉「打开时才知道 id」的形状', async () => {
    vi.mocked(favoriteApi.check).mockResolvedValue({ favorited: true, favorite_id: 5 })
    const fav = useFavorite('course', () => 0)

    await fav.load(88)
    expect(favoriteApi.check).toHaveBeenCalledWith({ target_type: 'course', target_id: 88 })
    expect(fav.favorited.value).toBe(true)
    expect(fav.favoriteId.value).toBe(5)
  })

  it('route params 那类字符串 id 照样能用；空串 / NaN / 0 一律按「无目标」', async () => {
    const fav = useFavorite('course', () => '42')
    await fav.load()
    expect(favoriteApi.check).toHaveBeenCalledWith({ target_type: 'course', target_id: 42 })

    for (const empty of [0, '', 'abc', null, undefined]) {
      vi.clearAllMocks()
      const blank = useFavorite('course', empty)
      await blank.load()
      await blank.toggle()
      expect(favoriteApi.check, `空 id ${String(empty)} 不应查询`).not.toHaveBeenCalled()
      expect(favoriteApi.add).not.toHaveBeenCalled()
    }
  })

  it('无当前目标时 toggle 返回 null 且不打任何接口（题目会话退出后的形状）', async () => {
    const fav = useFavorite('question', () => 0)
    expect(await fav.toggle()).toBeNull()
    expect(favoriteApi.add).not.toHaveBeenCalled()
    expect(favoriteApi.remove).not.toHaveBeenCalled()
    expect(ElMessage.success).not.toHaveBeenCalled()
  })

  it('查询收藏状态失败：降级为未收藏、不提示（页面不被阻断）', async () => {
    vi.mocked(favoriteApi.check).mockRejectedValue(new Error('network'))
    const fav = useFavorite('chapter', 7)

    await fav.load()
    expect(fav.favorited.value).toBe(false)
    expect(fav.favoriteId.value).toBe(0)
    expect(ElMessage.success).not.toHaveBeenCalled()
  })

  it('add 失败：保持原态且提示不说谎', async () => {
    vi.mocked(favoriteApi.add).mockRejectedValue(new Error('boom'))
    const fav = useFavorite('chapter', 7)
    await fav.load()

    expect(await fav.toggle()).toBeNull()
    expect(fav.favorited.value).toBe(false)
    expect(fav.favoriteId.value).toBe(0)
    expect(ElMessage.success).not.toHaveBeenCalled()
  })

  it('remove 失败：保持已收藏态与原 id 且提示不说谎', async () => {
    vi.mocked(favoriteApi.check).mockResolvedValue({ favorited: true, favorite_id: 7 })
    vi.mocked(favoriteApi.remove).mockRejectedValue(new Error('boom'))
    const fav = useFavorite('topic', 21)
    await fav.load()
    vi.mocked(ElMessage.success).mockClear()

    expect(await fav.toggle()).toBeNull()
    expect(fav.favorited.value).toBe(true)
    expect(fav.favoriteId.value).toBe(7)
    expect(ElMessage.success).not.toHaveBeenCalled()
  })

  it('幂等：favorited=true 但没有 favorite_id 时重发 add，而不是 DELETE /favorites/0', async () => {
    vi.mocked(favoriteApi.check).mockResolvedValue({ favorited: true, favorite_id: 0 })
    const fav = useFavorite('question', 12)
    await fav.load()
    expect(fav.favorited.value).toBe(true)

    expect(await fav.toggle()).toEqual({ favorited: true, favorite_id: 9 })
    expect(favoriteApi.remove).not.toHaveBeenCalled()
    expect(favoriteApi.add).toHaveBeenCalledWith({ target_type: 'question', target_id: 12 })
  })

  it('notify:false：切换成功不提示，仍回新态（题目面板那类由页面自决文案的形状）', async () => {
    const fav = useFavorite('question', () => 4, { notify: false })
    await fav.load()

    expect(await fav.toggle()).toEqual({ favorited: true, favorite_id: 9 })
    expect(ElMessage.success).not.toHaveBeenCalled()
    expect(fav.favorited.value).toBe(true)
  })
})

describe('useFavoriteList（列表形态：不查询，返回新态供回填行）', () => {
  it('全程不调 check；未收藏行走 add 并返回新态供调用方回填', async () => {
    const list = useFavoriteList('question')

    const next = await list.toggle({ targetId: 12, favorited: false, favorite_id: 0 })
    expect(favoriteApi.check).not.toHaveBeenCalled()
    expect(favoriteApi.add).toHaveBeenCalledWith({ target_type: 'question', target_id: 12 })
    expect(next).toEqual({ favorited: true, favorite_id: 9 })
    expect(ElMessage.success).not.toHaveBeenCalled()
  })

  it('已收藏行走 remove 并返回清空后的新态（同样不查询）', async () => {
    const list = useFavoriteList('question')

    const next = await list.toggle({ targetId: 12, favorited: true, favorite_id: 7 })
    expect(favoriteApi.check).not.toHaveBeenCalled()
    expect(favoriteApi.remove).toHaveBeenCalledWith(7)
    expect(next).toEqual({ favorited: false, favorite_id: 0 })
  })

  it('失败返回 null：调用方据此不回填，行保持原态且不提示', async () => {
    vi.mocked(favoriteApi.add).mockRejectedValue(new Error('boom'))
    const list = useFavoriteList('question')
    const row = { targetId: 12, favorited: false, favorite_id: 0 }

    expect(await list.toggle(row)).toBeNull()
    expect(ElMessage.success).not.toHaveBeenCalled()
    // 行未被写过：这就是「失败保持原态」在列表形态下的落点
    expect(row).toEqual({ targetId: 12, favorited: false, favorite_id: 0 })
  })

  it('「只看收藏」筛选下取消收藏 → 调用方传入的 reload hook 被触发', async () => {
    const reload = vi.fn()
    const list = useFavoriteList('question', { favoritedOnly: () => true, reload })

    const next = await list.toggle({ targetId: 12, favorited: true, favorite_id: 7 })
    expect(next?.favorited).toBe(false)
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it('「只看收藏」下新增收藏不重载（该行本就该留在列表里）；筛选未生效时取消收藏也不重载', async () => {
    const reloadFiltered = vi.fn()
    const filtered = useFavoriteList('question', { favoritedOnly: () => true, reload: reloadFiltered })
    await filtered.toggle({ targetId: 12, favorited: false, favorite_id: 0 })
    expect(reloadFiltered).not.toHaveBeenCalled()

    const reloadUnfiltered = vi.fn()
    const unfiltered = useFavoriteList('question', { favoritedOnly: () => false, reload: reloadUnfiltered })
    await unfiltered.toggle({ targetId: 12, favorited: true, favorite_id: 7 })
    expect(reloadUnfiltered).not.toHaveBeenCalled()

    // 未声明筛选事实 ⇒ 永不重载（判据归 module，事实由调用方声明）
    const bare = useFavoriteList('question')
    await bare.toggle({ targetId: 12, favorited: true, favorite_id: 7 })
    expect(favoriteApi.remove).toHaveBeenCalled()
  })

  it('取消收藏失败 ⇒ 不重载（列表重载只在态真的翻转后）', async () => {
    vi.mocked(favoriteApi.remove).mockRejectedValue(new Error('boom'))
    const reload = vi.fn()
    const list = useFavoriteList('question', { favoritedOnly: () => true, reload })

    expect(await list.toggle({ targetId: 12, favorited: true, favorite_id: 7 })).toBeNull()
    expect(reload).not.toHaveBeenCalled()
  })

  it('行上没有 favorite_id 时同样幂等重发 add（与单对象形态同一判据）', async () => {
    const list = useFavoriteList('question')
    expect(await list.toggle({ targetId: 12, favorited: true })).toEqual({ favorited: true, favorite_id: 9 })
    expect(favoriteApi.remove).not.toHaveBeenCalled()
  })

  it('无 targetId 的行不打接口', async () => {
    const list = useFavoriteList('question')
    expect(await list.toggle({ targetId: 0, favorited: false })).toBeNull()
    expect(favoriteApi.add).not.toHaveBeenCalled()
  })
})

describe('类型收窄：不可收藏种类进不了两个入口（ADR-0060 决策 3）', () => {
  // 收窄的类型源是内容对象表的 favoritable 投影 FAVORITABLE_CONTENT_OBJECT_KEYS；
  // 投影与表槽位的互等锁在 config/__tests__/contentObjects.spec.ts。
  it('入口参数只接受可收藏种类：越界写法由 vue-tsc 判错（@ts-expect-error 失效即反向报错）', () => {
    // @ts-expect-error 不在可收藏种类那一档的种类不能作为 key（今天表里五种全可收藏，
    // 故「表外的种类」与「favoritable=false 的种类」在类型层同形；后者一旦进表即被同一行挡住）
    useFavorite('not-a-favoritable-kind', 1)
    // @ts-expect-error 列表入口共用同一收窄
    useFavoriteList('not-a-favoritable-kind')
    expect(true).toBe(true)
  })

  it('投影必须是表 key 的子集（编译期方向；反方向与 favoritable 一致性由 contentObjects.spec 锁）', () => {
    const projectionIsSubset: FavoritableContentObjectKey extends ContentObjectKey ? true : false = true
    expect(projectionIsSubset).toBe(true)
    // 表里不可收藏的种类（若有）必然落在入口参数的补集里：当前为空集，表新增即自动生效
    const excluded: Exclude<ContentObjectKey, FavoritableContentObjectKey>[] = []
    expect(excluded).toEqual([])
  })
})
