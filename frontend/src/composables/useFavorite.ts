// useFavorite：收藏开关的唯一状态机（第十三波票3，ADR-0060 决策 3；种类判据见 CONTEXT.md「内容对象」）。
// 迁移前「查询收藏状态 → 切换 → 提示 → 失败保持原态」在四处逐字复制（章节页 / 课程详情抽屉 /
// 题目面板 / 帖子详情），列表形态（错题本行内）另抄一份。本 module 收成一个实现、两个具名入口：
// - useFavorite(key, id)：**单对象形态**，自带 favoriteApi.check 的状态查询，服务「行上不携带
//   收藏态」的详情页 / 抽屉 / 面板开关。何时查询仍由调用方决定（章节页随 id 变、抽屉随打开、
//   题目面板随进题），本 module 只提供 load()，不猜触发时机。
// - useFavoriteList(key)：**列表形态**，**不查询** —— 列表行由服务端实时携带 favorited /
//   favorite_id（ADR-0059 既有口径），toggle 返回新态供调用方回填行。
// 两个入口各自对应真实存在的调用形（一个查态、一个态已知），即两个 adapter = 真 seam；
// 「一个 useFavorite(key,id,{initial}) 兼表『态已知/待查』」是 ADR-0060 明确否掉的形态（同一参数
// 兼表两义，ADR-0056 §2）。也不立收藏 store：收藏态是页面级瞬态，无跨面共享事实。
// 共享的是同一段 add / remove / 幂等 / 提示 / 失败保持原态实现；提示文案因此只有一处。
// 「哪种内容对象可收藏」的唯一判据 = 内容对象表：key 的类型收窄到 FavoritableContentObjectKey
// （表 favoritable=true 那一档的投影），运行期 target_type 也逐次向表取。表管声明槽、本 module
// 管运行期状态机，二者正交（ADR-0060 决策 3）。
import { ref, toValue, type MaybeRefOrGetter, type Ref } from 'vue'
import { ElMessage } from 'element-plus'
import { favoriteApi, type FavoriteTargetType } from '@/api/favorite'
import { contentObjectByKey, type FavoritableContentObjectKey } from '@/config/contentObjects'

/** 收藏态（单对象状态与列表回填共用同一形状；favorite_id=0 = 无收藏记录） */
export interface FavoriteState {
  favorited: boolean
  favorite_id: number
}

/**
 * 列表行的收藏槽。targetId 显式命名而不写 id，是因为行主键与收藏目标 id 常常不同名同号易混
 * （错题行的主键是 id、收藏目标是 question_id）——叫 id 会让 `toggle(item)` 整行直传也能
 * 过类型检查，然后静默收藏错对象。favorited / favorite_id 与契约字段同名，便于调用方逐字段回填。
 */
export interface FavoriteListRow {
  targetId: number
  favorited?: boolean | null
  favorite_id?: number | null
}

/** id 源：数字 / 字符串 / ref / getter；0 · null · undefined · '' · NaN 一律视为「当前无目标」 */
export type FavoriteIdSource = MaybeRefOrGetter<number | string | null | undefined>

function resolveId(raw: number | string | null | undefined): number {
  const n = typeof raw === 'string' ? Number(raw) : (raw ?? 0)
  return Number.isFinite(n) && n > 0 ? n : 0
}

/**
 * 「哪种可收藏」的运行期出处：向内容对象表要 favoriteTargetType。
 * 类型层已只放行 favoritable 种类，这里的兜底管的是「表把某种类改成不可收藏、而投影列表漏跟改」
 * 那一步漂移（互等锁在 contentObjects.spec.ts）：宁可整个开关停摆，也不拿一个平台不收的
 * target_type 去打接口。
 */
function favoriteTargetTypeOf(key: FavoritableContentObjectKey): FavoriteTargetType | null {
  const entry = contentObjectByKey(key)
  if (!entry?.favoritable) {
    console.error(`[useFavorite] 内容对象「${key}」不可收藏，该处收藏开关已停用`)
    return null
  }
  return entry.favoriteTargetType
}

/**
 * 共享状态机：查询 + 切换。两个入口的唯一区别是是否查询与态存在哪里，故 notify 之外无差异。
 *
 * 幂等：favorited=true 但 favorite_id 缺失（列表行刚被面板置真、check 响应没带 id 等）时不再
 * DELETE /favorites/0 打一个必然 404 的请求，而是重发 add（服务端收藏本就幂等）。
 */
function createFavoriteFlow(key: FavoritableContentObjectKey, notify: boolean) {
  const targetType = favoriteTargetTypeOf(key)

  /** 查询收藏状态；失败降级为未收藏（不阻断页面，也不留下可点的假态） */
  async function query(id: number): Promise<FavoriteState> {
    if (!targetType) return { favorited: false, favorite_id: 0 }
    try {
      const res = await favoriteApi.check({ target_type: targetType, target_id: id })
      return { favorited: !!res?.favorited, favorite_id: res?.favorite_id || 0 }
    } catch (e) {
      console.error('查询收藏状态失败:', e)
      return { favorited: false, favorite_id: 0 }
    }
  }

  /** 切换：成功返回新态并按 notify 提示；失败返回 null 且**保持原态**（错误已由拦截器提示） */
  async function apply(id: number, current: FavoriteState): Promise<FavoriteState | null> {
    if (!targetType || !id) return null
    try {
      if (current.favorited && current.favorite_id) {
        await favoriteApi.remove(current.favorite_id)
        if (notify) ElMessage.success('已取消收藏')
        return { favorited: false, favorite_id: 0 }
      }
      const res = await favoriteApi.add({ target_type: targetType, target_id: id })
      if (notify) ElMessage.success('已收藏')
      return { favorited: true, favorite_id: res?.favorite_id || 0 }
    } catch (e) {
      console.error('收藏操作失败:', e)
      return null
    }
  }

  return { query, apply }
}

export interface UseFavoriteOptions {
  /**
   * 切换成功后是否 ElMessage 提示（默认 true：详情页/抽屉/帖子那四处一直是弹提示的形状）。
   * 传 false 由页面按 toggle() 返回值自行提示——题目面板的形状（练习页与真题页的提示口径不同，
   * 收敛前既已如此，见 useQuestionPeripherals）。
   */
  notify?: boolean
}

/**
 * 单对象形态的收藏开关：自带状态查询（favoriteApi.check）+ 切换 + 提示 + 失败保持原态。
 *
 * @param key 内容对象种类（仅 favoritable 那一档可传，其余编译期即挡）
 * @param id 收藏目标 id 源（ref / getter / 定值皆可；切换时按**当时**的值取，故章节切页、
 *           抽屉换课程无需重建开关）
 * @param options.notify 提示开关，见 UseFavoriteOptions
 *
 * 触发时机留给调用方：load() 查当前 id，load(explicitId) 查指定 id（抽屉「打开时才知道 id」的形状）。
 */
export function useFavorite(
  key: FavoritableContentObjectKey,
  id: FavoriteIdSource,
  options: UseFavoriteOptions = {}
): {
  favorited: Ref<boolean>
  favoriteId: Ref<number>
  load: (targetId?: number) => Promise<void>
  toggle: () => Promise<FavoriteState | null>
} {
  const flow = createFavoriteFlow(key, options.notify ?? true)
  const favorited = ref(false)
  const favoriteId = ref(0)

  function currentId(): number {
    return resolveId(toValue(id))
  }

  /** 先清态再查：切换目标后不残留上一个目标的收藏态（四处迁移前均是此序）。id 为 0 ⇒ 只清态不查 */
  async function load(targetId?: number): Promise<void> {
    favorited.value = false
    favoriteId.value = 0
    const resolved = resolveId(targetId) || currentId()
    if (!resolved) return
    const state = await flow.query(resolved)
    favorited.value = state.favorited
    favoriteId.value = state.favorite_id
  }

  /** 切换当前目标的收藏；成功返回新态（调用方可忽略，态已自持），失败/无目标返回 null 且态不变 */
  async function toggle(): Promise<FavoriteState | null> {
    const next = await flow.apply(currentId(), { favorited: favorited.value, favorite_id: favoriteId.value })
    if (next) {
      favorited.value = next.favorited
      favoriteId.value = next.favorite_id
    }
    return next
  }

  return { favorited, favoriteId, load, toggle }
}

export interface FavoriteListOptions {
  /** 「只看收藏」筛选当前是否生效 */
  favoritedOnly?: () => boolean
  /**
   * 重载列表。**只在**「只看收藏」生效且本次是取消收藏时被本 module 调用——该筛选下被取消的
   * 那条必须离开列表，否则赖在原位（#1168 之后的既有口径）。生效与否由调用方声明（它才知道
   * 筛选状态），「什么时候需要重载」这条判据归本 module。
   */
  reload?: () => unknown | Promise<unknown>
}

/**
 * 列表形态的收藏开关：**不查询**（态由列表行携带，服务端实时下发 favorited / favorite_id 是
 * ADR-0059 既有口径），也不提示（行内星标本身就是反馈，逐项弹 toast 是噪声）。
 *
 * toggle(row) 返回新态供调用方回填行；失败返回 null，调用方据此不写回（行保持原态）。
 */
export function useFavoriteList(
  key: FavoritableContentObjectKey,
  options: FavoriteListOptions = {}
): {
  toggle: (row: FavoriteListRow) => Promise<FavoriteState | null>
} {
  const flow = createFavoriteFlow(key, false)

  async function toggle(row: FavoriteListRow): Promise<FavoriteState | null> {
    const next = await flow.apply(resolveId(row.targetId), {
      favorited: !!row.favorited,
      favorite_id: row.favorite_id || 0
    })
    if (next && !next.favorited && options.favoritedOnly?.()) await options.reload?.()
    return next
  }

  return { toggle }
}
