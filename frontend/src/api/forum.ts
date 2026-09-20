import { unwrappedRequest } from './request'
// 票 6（ADR-0060 决策 6）：管理端两条列表队列在出口处归一为中立容器 Page<T>。
// 学员端 forumApi 的读取面（listTopics / my-topics / …）**不改**：那些走 useAsyncPage 的
// append 档，需要响应上的 pages 字段（ADR-0060 决策 4 的判据），容不得容器收窄。
import { toPage, type Page } from './page'
import type {
  ForumImageUploadResultDTO,
  ForumLikeResultDTO,
  ForumReplyDTO,
  ForumReportDTO,
  ForumReportPageResult,
  ForumTopicDTO,
  ForumTopicDetailDTO,
  ForumTopicPageResult,
  MyReplyDTO,
  MyReplyPageResult
} from './generated/forum'

/**
 * 论坛帖子意图（#364）：判"学员想干什么"的唯一依据，与判区域的 chapter_id 正交。
 * ADR-0040 起值域收窄为两值；'experience' 只为**旧客户端兼容**留在类型与筛选白名单里
 * （后端接受但恒不出任何行——存量行已由迁移降级为 discussion）。
 * 「备考经验」是管理端**认定**，判据是 is_experience（见 ForumTopicItem.is_experience），
 * **不是类别**——两者不可互相替代。
 */
export type ForumCategory = 'discussion' | 'question' | 'experience'

/**
 * 学员可自述的发帖意图（ADR-0040）：'备考经验'已改为管理端认定，
 * 学员只能声明"想讨论什么"（讨论 / 提问），不能声明"产出了什么"。
 * 发帖与编辑的 category 入参只接受这两个值。
 */
export type ForumPublishCategory = 'discussion' | 'question'

/**
 * 正文格式声明（ADR-0044）：text=纯文本 / markdown=受限 Markdown 子集。
 *
 * 这是**作者自述的声明位**，不是系统猜测——同一段文字按两种格式渲染结果不同
 * （例如「1. 检查电瓶」在 markdown 下会变成有序列表），故渲染方式必须由作者决定。
 * 缺省（字段缺失或空串）按 `text` 处理，与后端 normalizeContentFormat 的归一一致。
 */
export type ForumContentFormat = 'text' | 'markdown'

/**
 * 生成物的 content_format / category 是 string —— 注解层还没有枚举词汇（ADR-0048 片一已知限制）。
 * 渲染前用它收窄回 UI 联合：只认 text / markdown，其余（空串 / 未来新值）返回 undefined，
 * 交给 ForumContent 的缺省（text）处理，与后端 normalizeContentFormat 的归一一致。
 */
export function toForumContentFormat(value?: string): ForumContentFormat | undefined {
  return value === 'markdown' ? 'markdown' : value === 'text' ? 'text' : undefined
}

/**
 * 论坛列表 Tab（#364）。学员端的「讨论 / 问答」与管理端的
 * 「全部帖子 / 综合讨论区 / 问答区」本质是同一片内容的三种切法，
 * 因此两端共用这一份映射，避免"讨论 Tab 必须带 category"这条规则各写一遍。
 */
export type ForumTab = 'all' | ForumCategory

/**
 * 帖子对象 —— 响应形状来自生成物（ADR-0048）。
 *
 * 注解层 category / content_format 是 string（枚举词汇尚未进注解，片一已知限制），
 * UI 侧窄化联合（ForumCategory / ForumContentFormat）保留在消费处按值断言。
 */
export type ForumTopicItem = ForumTopicDTO

/** 回复对象 —— 响应形状来自生成物（ADR-0048）。 */
export type ForumReplyItem = ForumReplyDTO

/** 帖子详情响应（ADR-0042）：`topic` 与 `replies` 平级 + 分页信封 */
/** 帖子详情响应（ADR-0042）：topic 与 replies 平级 + 分页信封；响应形状来自生成物。 */
export type ForumTopicDetailData = ForumTopicDetailDTO

export interface ForumListParams {
  scope?: 'all' | 'general' | 'chapter'
  /** 类别分流；省略或 'all' 表示两类都看（移动端旧契约即如此） */
  category?: ForumCategory
  /** 解决状态（#367，仅问答有意义） */
  solved?: 'all' | 'solved' | 'unsolved'
  /** 精选过滤（#742）：'true' 仅精选 / 'false' 仅非精选，省略不过滤 */
  featured?: 'true' | 'false'
  /** 经验认定过滤（ADR-0040）：'true' 仅管理端认定的经验帖，省略不过滤 */
  is_experience?: 'true' | 'false'
  chapter_id?: number
  page?: number
  page_size?: number
  keyword?: string
  sort?: 'latest' | 'hot'
  order?: 'asc' | 'desc'
}

/**
 * 论坛列表 Tab → 查询参数（#364）。
 *
 * 学员端与管理端共用这一份映射：学员端的「讨论」与管理端的「综合讨论区」本就是同一片内容
 * （非章节 + 讨论类别），规则写两处迟早会各改各的。
 *
 * ⚠️ 两个轴不可互相替代：scope 判**区域**（chapter_id IS NULL），category 判**意图**。
 * 讨论 Tab 必须同时带 category —— 问答帖的 chapter_id 也是 NULL，漏掉它问答帖会整片灌进讨论列表。
 */
export function forumTabQuery(tab: ForumTab): Pick<ForumListParams, 'scope' | 'category' | 'is_experience'> {
  switch (tab) {
    case 'discussion':
      // 讨论 Tab 沿用既有口径：只看综合区、不合并章节讨论。
      return { scope: 'general', category: 'discussion' }
    case 'question':
      // 问答帖按设计无章节归属，没有区域维度可筛，故不带 scope。
      return { category: 'question' }
    case 'experience':
      // 备考经验（#722 / ADR-0040）：判据是管理端**认定** is_experience，**不是** category ——
      // category='experience' 的存量行已被迁移降级为 discussion，发它必然空。
      // scope=all：经验帖可挂章节也可不挂，列表看全量认定帖。
      return { scope: 'all', is_experience: 'true' }
    default:
      // 全部：两个参数都不带，与改动前逐条一致（服务端默认 scope=all、不过滤类别）。
      return {}
  }
}

export const forumApi = {
  listTopics(params: ForumListParams) {
    return unwrappedRequest.get<ForumTopicPageResult>('/forum/topics', { params })
  },

  createTopic(data: {
    chapter_id?: number | null
    category?: ForumPublishCategory
    title: string
    content: string
    images?: string[]
    /** 正文格式声明（ADR-0044）。与 replyTopic 同口径：缺省按 text。 */
    content_format?: ForumContentFormat
  }) {
    return unwrappedRequest.post<ForumTopicItem>('/forum/topics', data)
  },

  /**
   * 帖子详情（ADR-0042）：回复**分页读取**，`topic` 与 `replies` 平级，附带分页信封。
   *
   * ⚠️ 被采纳回复由**后端**保证占首页第一条并从排序结果剔除（不再是前端派生置顶），
   * 故首页条数 = page_size（含置顶条）；翻页时页与页之间无重叠、无遗漏。
   */
  getTopic(id: number, sort?: 'latest' | 'hot' | 'time', order?: 'asc' | 'desc', page?: number, pageSize?: number) {
    const params: Record<string, string | number> = {}
    if (sort) params.sort = sort
    if (order) params.order = order
    if (page) params.page = page
    if (pageSize) params.page_size = pageSize
    return unwrappedRequest.get<ForumTopicDetailData>(`/forum/topics/${id}`, { params: Object.keys(params).length ? params : undefined })
  },

  /**
   * 回复（ADR-0044 起带正文格式）。
   *
   * contentFormat 缺省按 text 提交；Web 侧一律显式传（用户的选择在
   * useForumContentFormat 里），只有尚未适配的旧客户端才会省略该字段。
   */
  replyTopic(
    id: number,
    content: string,
    parentReplyId?: number | null,
    images?: string[],
    contentFormat?: ForumContentFormat
  ) {
    return unwrappedRequest.post<ForumReplyItem>(`/forum/topics/${id}/replies`, {
      content,
      parent_reply_id: parentReplyId || null,
      images: images || [],
      content_format: contentFormat ?? 'text'
    })
  },

  // 上传论坛图片（图文分离：先传图拿 URL，随发帖/回复提交 images 数组）
  uploadImage(formData: FormData) {
    return unwrappedRequest.post<ForumImageUploadResultDTO>('/forum/upload-image', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000
    })
  },

  deleteTopic(id: number) {
    return unwrappedRequest.delete<null>(`/forum/topics/${id}`)
  },

  deleteReply(id: number) {
    return unwrappedRequest.delete<null>(`/forum/replies/${id}`)
  },

  // ===== 互动（ADR-0018）=====

  /** 点赞主题（幂等，返回当前计数与状态） */
  likeTopic(id: number) {
    return unwrappedRequest.post<ForumLikeResultDTO>(`/forum/topics/${id}/like`)
  },

  /** 取消点赞（幂等） */
  unlikeTopic(id: number) {
    return unwrappedRequest.delete<ForumLikeResultDTO>(`/forum/topics/${id}/like`)
  },

  /** 举报主题（reason 1-500 字） */
  reportTopic(id: number, reason: string) {
    return unwrappedRequest.post<null>(`/forum/topics/${id}/report`, { reason })
  },

  /** 举报回复 */
  reportReply(id: number, reason: string) {
    return unwrappedRequest.post<null>(`/forum/replies/${id}/report`, { reason })
  },

  /** 我的帖子（复用主题列表结构） */
  getMyTopics(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<ForumTopicPageResult>(
      '/forum/my-topics',
      { params }
    )
  },

  /** 我的回复（带主题标题回填） */
  getMyReplies(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<MyRepliesData>('/forum/my-replies', { params })
  },

  /** 赞过（#701：响应逐字沿用 my-topics 形态，按点赞时间倒序） */
  getMyLikedTopics(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<ForumTopicPageResult>(
      '/forum/my-liked-topics',
      { params }
    )
  },

  /** 围观（#701：浏览减去四项直接互动，按最近浏览倒序） */
  getMyObservedTopics(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<ForumTopicPageResult>(
      '/forum/my-observed',
      { params }
    )
  },

  /** 浏览记录（#701：服务端替换本地 localStorage，按主题去重、最近浏览倒序） */
  getMyViewHistory(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<ForumTopicPageResult>(
      '/forum/my-view-history',
      { params }
    )
  },

  // ===== 回复点赞（spec #268）=====

  /** 点赞回复（幂等） */
  likeReply(id: number) {
    return unwrappedRequest.post<ForumLikeResultDTO>(`/forum/replies/${id}/like`)
  },

  /** 取消点赞回复（幂等） */
  unlikeReply(id: number) {
    return unwrappedRequest.delete<ForumLikeResultDTO>(`/forum/replies/${id}/like`)
  },

  // ===== 采纳（#366/#367）=====

  /** 采纳回答（仅楼主，幂等，首次加分） */
  acceptReply(topicId: number, replyId: number) {
    return unwrappedRequest.post<ForumTopicItem>(`/forum/topics/${topicId}/accept`, { reply_id: replyId })
  },

  /** 取消采纳（仅楼主，状态回到未解决，已发分不回滚） */
  cancelAccept(topicId: number) {
    return unwrappedRequest.delete<ForumTopicItem>(`/forum/topics/${topicId}/accept`)
  }
}

/** 我的回复条目（主题被删时 topic_title 为空串，条目保留）——响应形状来自生成物。 */
export type MyReplyItem = MyReplyDTO

/** 我的回复分页响应 */
/** 我的回复分页响应 */
export type MyRepliesData = MyReplyPageResult

// ===== 论坛管理（管理端）=====

/** 管理端帖子（与学员端同一 DTO，字段全量）——响应形状来自生成物。 */
export type AdminForumTopic = ForumTopicDTO

/** 管理端回复（与学员端同一 DTO）——响应形状来自生成物。 */
export type AdminForumReply = ForumReplyDTO

/** 管理端帖子详情响应（ADR-0042：`topic` 与 `replies` 平级 + 分页信封） */
/** 管理端帖子详情响应（ADR-0042：topic 与 replies 平级 + 分页信封）——响应形状来自生成物。 */
export type AdminForumTopicDetailData = ForumTopicDetailDTO

export interface AdminForumListParams {
  scope?: 'all' | 'general' | 'chapter'
  /** 类别维度（#364）；省略表示两类都看 */
  category?: ForumCategory
  solved?: 'all' | 'solved' | 'unsolved'
  /** 精选过滤（#742）：'true' 仅精选 / 'false' 找待精候选，省略不过滤 */
  featured?: 'true' | 'false'
  /** 经验认定过滤（ADR-0040）：'true' 仅认定的经验帖（「备考经验」筛选轴），省略不过滤 */
  is_experience?: 'true' | 'false'
  chapter_id?: number
  page?: number
  page_size?: number
  keyword?: string
}

export const adminForumApi = {
  /** 帖子列表（后端行键 = `topics`）。 */
  async listTopics(params: AdminForumListParams): Promise<Page<AdminForumTopic>> {
    const res = await unwrappedRequest.get<ForumTopicPageResult>('/admin/forum/topics', { params })
    return toPage(res?.topics, res?.total)
  },

  /**
   * 管理端帖子详情（ADR-0042：回复同样分页读取）。
   *
   * ⚠️ 详情接口改为分页后，管理端展开面板若只取首页就会**静默少掉**后面的回复——
   * 治理面看不到全部回复等于看不见违规内容，故调用方必须接「加载更多」。
   */
  getTopic(id: number, page?: number, pageSize?: number) {
    const params: Record<string, number> = {}
    if (page) params.page = page
    if (pageSize) params.page_size = pageSize
    return unwrappedRequest.get<AdminForumTopicDetailData>(`/admin/forum/topics/${id}`, {
      params: Object.keys(params).length ? params : undefined
    })
  },

  deleteTopic(id: number) {
    return unwrappedRequest.delete<null>(`/admin/forum/topics/${id}`)
  },

  deleteReply(id: number) {
    return unwrappedRequest.delete<null>(`/admin/forum/replies/${id}`)
  },

  // ===== 精选位（#742，全类别可精/可撤）=====

  /** 加精（首次加精同事务给帖主 featured_bonus +30，幂等） */
  featureTopic(id: number) {
    return unwrappedRequest.post<AdminForumTopic>(`/admin/forum/topics/${id}/featured`)
  },

  /**
   * 取消精选（已发分不回滚，幂等）。
   * ⚠️ 经验帖撤精后端 400（经验蕴含精选位），须先取消经验认定——前端已在行内禁用该入口。
   */
  unfeatureTopic(id: number) {
    return unwrappedRequest.delete<AdminForumTopic>(`/admin/forum/topics/${id}/featured`)
  },

  // ===== 备考经验认定（ADR-0040，与精选位同轴）=====

  /** 认定备考经验：同时置 is_experience 与 is_featured，首次认定给帖主 +30（与加精共用一次，幂等） */
  designateExperience(id: number) {
    return unwrappedRequest.post<AdminForumTopic>(`/admin/forum/topics/${id}/experience`)
  },

  /** 取消经验认定：**保留精选位**，已发分不回滚（幂等） */
  revokeExperience(id: number) {
    return unwrappedRequest.delete<AdminForumTopic>(`/admin/forum/topics/${id}/experience`)
  },

  // ===== 举报管理（ADR-0018）=====

  /** 举报列表（status 缺省全部；0 待处理 / 1 已处理。后端行键 = `reports`）。 */
  async listReports(params: { status?: number; page?: number; page_size?: number }): Promise<Page<AdminForumReportItem>> {
    const res = await unwrappedRequest.get<AdminForumReportsData>('/admin/forum/reports', { params })
    return toPage(res?.reports, res?.total)
  },

  /** 处理举报（status: 0 待处理 / 1 已处理） */
  handleReport(id: number, status: number) {
    return unwrappedRequest.put<null>(`/admin/forum/reports/${id}`, { status })
  }
}

/** 管理端举报条目（与后端 ForumReportDTO 对齐） */
/** 管理端举报条目（与后端 ForumReportDTO 对齐）——响应形状来自生成物。 */
export type AdminForumReportItem = ForumReportDTO

/** 管理端举报列表响应 */
/** 管理端举报列表响应 */
export type AdminForumReportsData = ForumReportPageResult
