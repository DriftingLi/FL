// 内容对象声明表（第十二波票 2，#1168；词条见 CONTEXT.md「内容对象」）：
// 搜索页与收藏页共用的「种类 → 称谓 / 标签色 / 落点 / 可检索 / 可收藏 / 进收藏 tab」单一事实源。
// 三行为槽互相独立、不由称谓反推（#1132 裁定「收藏 tab 刻意不含内容精选」不得被派生顺手合并）。
// 落点出「路由名 + 参数」，path 事实源仍在页面描述符表（ADR-0047 §2）；to 返回 null = 无落点，
// 该条目不得进可点面（ADR-0049 决策 2「搜到但打不开」的结构性防呆）。
import type { FavoriteTargetType } from '@/api/favorite'
import type { SearchType } from '@/api/search'
import type { UiTagTone } from '@/components/ui/UiTag.vue'
import { routeNames, type RouteName } from './routeNames'

export type ContentObjectKey = 'course' | 'chapter' | 'question' | 'featured' | 'topic'

// 标签色用 UiTag 导出的 UiTagTone 收窄（ui-conventions「剩余控件」条）；route name 用 RouteName union（ADR-0047 §2）。
export type ContentObjectTagTone = UiTagTone

export interface ContentObjectIds {
  id: number
  /** 章节的所属课程（搜索结果 = parent_id，收藏条目 = course_id）；其余种类忽略 */
  parentId?: number
}

export interface ContentObjectTarget {
  name: RouteName
  params?: Record<string, string>
  query?: Record<string, string>
}

export interface ContentObject {
  key: ContentObjectKey
  label: string
  tone: ContentObjectTagTone
  searchable: boolean
  /** 搜索契约键（内容精选在搜索域叫 content）；searchable=false 时缺省 */
  searchType?: SearchType
  /** SearchAllResult 里该种类的分组字段 */
  searchField?: 'courses' | 'chapters' | 'questions' | 'contents' | 'topics'
  favoritable: boolean
  /** 收藏契约键（favorite target_type） */
  favoriteTargetType: FavoriteTargetType
  inFavoriteTab: boolean
  to: (ids: ContentObjectIds) => ContentObjectTarget | null
}

// 表序 = 搜索分区序（课程/章节/题目/内容精选/帖子）；收藏 tab 由其上剔除 featured 后同序。
export const CONTENT_OBJECTS: readonly ContentObject[] = [
  {
    key: 'course',
    label: '课程',
    tone: 'primary',
    searchable: true,
    searchType: 'course',
    searchField: 'courses',
    favoritable: true,
    favoriteTargetType: 'course',
    inFavoriteTab: true,
    to: ({ id }) => ({ name: routeNames.CourseList, query: { course_id: String(id) } })
  },
  {
    key: 'chapter',
    label: '章节',
    tone: 'success',
    searchable: true,
    searchType: 'chapter',
    searchField: 'chapters',
    favoritable: true,
    favoriteTargetType: 'chapter',
    inFavoriteTab: true,
    // 缺所属课程即无落点（不猜、不乱跳，#1089）
    to: ({ id, parentId }) =>
      parentId && parentId > 0
        ? { name: routeNames.ChapterView, params: { courseId: String(parentId), chapterId: String(id) } }
        : null
  },
  {
    key: 'question',
    label: '题目',
    tone: 'warning',
    searchable: true,
    searchType: 'question',
    searchField: 'questions',
    favoritable: true,
    favoriteTargetType: 'question',
    inFavoriteTab: true,
    to: ({ id }) => ({ name: routeNames.StudentQuestionDetail, params: { id: String(id) } })
  },
  {
    key: 'featured',
    label: '内容精选',
    tone: 'info',
    searchable: true,
    searchType: 'content',
    searchField: 'contents',
    favoritable: true,
    favoriteTargetType: 'featured',
    // 2026-09-18 裁定（#1132）：精选阅读面在门户、Web 无创建点，进 tab 只会长期为空——刻意不补。
    inFavoriteTab: false,
    to: ({ id }) => ({ name: routeNames.StudentFeaturedDetail, params: { id: String(id) } })
  },
  {
    key: 'topic',
    label: '帖子',
    tone: 'danger',
    searchable: true,
    searchType: 'topic',
    searchField: 'topics',
    favoritable: true,
    favoriteTargetType: 'topic',
    inFavoriteTab: true,
    to: ({ id }) => ({ name: routeNames.ForumDetail, params: { topicId: String(id) } })
  }
]

const BY_KEY = new Map(CONTENT_OBJECTS.map(o => [o.key, o]))
const BY_SEARCH_TYPE = new Map(CONTENT_OBJECTS.filter(o => o.searchType).map(o => [o.searchType as SearchType, o]))

/**
 * 可收藏种类的**类型化投影** = 表 favoritable=true 那一档（ADR-0060 决策 3：收藏开关的 key
 * 参数只接受这里的种类，不可收藏种类编译期即挡）。
 *
 * 为什么是投影而不是派生：表按 `readonly ContentObject[]` 标注，favoritable 的字面量在类型层
 * 已擦除（全为 boolean），Extract 无从分档；本列表以 `as const satisfies readonly
 * ContentObjectKey[]` 定型——写出不存在的种类由 satisfies 在编译期挡，与表 favoritable 槽的
 * **互等**由 __tests__/contentObjects.spec.ts 那条锁在运行期挡（把某种类改成不可收藏而忘从本
 * 列表剔除、或漏登记，都判红）。收藏运行期（composables/useFavorite.ts）的 target_type 仍逐次
 * 向表取，本列表只承载类型，不复制契约键。
 */
export const FAVORITABLE_CONTENT_OBJECT_KEYS = ['course', 'chapter', 'question', 'featured', 'topic'] as const satisfies readonly ContentObjectKey[]

/** 可收藏种类（内容对象表 favoritable=true 那一档；判据唯一出处见上一条注释） */
export type FavoritableContentObjectKey = (typeof FAVORITABLE_CONTENT_OBJECT_KEYS)[number]

export function contentObjectByKey(key: ContentObjectKey | string): ContentObject | undefined {
  return BY_KEY.get(key as ContentObjectKey)
}

export function contentObjectBySearchType(type: SearchType): ContentObject | undefined {
  return BY_SEARCH_TYPE.get(type)
}

/** 搜索页分区：按表序取可检索种类 */
export function searchableContentObjects(): ContentObject[] {
  return CONTENT_OBJECTS.filter(o => o.searchable)
}

/** 收藏页 tab：可收藏且进 tab 的种类（表序） */
export function favoriteTabContentObjects(): ContentObject[] {
  return CONTENT_OBJECTS.filter(o => o.favoritable && o.inFavoriteTab)
}
