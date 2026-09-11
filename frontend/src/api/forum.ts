import { unwrappedRequest } from './request'

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
 * 论坛列表 Tab（#364）。学员端的「讨论 / 问答」与管理端的
 * 「全部帖子 / 综合讨论区 / 问答区」本质是同一片内容的三种切法，
 * 因此两端共用这一份映射，避免"讨论 Tab 必须带 category"这条规则各写一遍。
 */
export type ForumTab = 'all' | ForumCategory

export interface ForumTopicItem {
  id: number
  chapter_id?: number | null
  category?: ForumCategory
  chapter_title?: string
  title: string
  content: string
  images?: string[]
  view_count: number
  reply_count: number
  last_reply_at?: string | null
  created_at: string
  author: {
    user_id: number
    username: string
    avatar_url: string
  }
  can_delete?: boolean
  likes_count?: number
  liked_by_me?: boolean
  accepted_reply_id?: number | null
  solved_at?: string | null
  /** 精选位（#742）：管理端全类别可精/可撤，三 Tab 筛选与标识展示 */
  is_featured?: boolean
  /**
   * 备考经验认定（ADR-0040）：管理端认定列，与 is_featured 同轴且**蕴含精选**
   * （is_experience ⇒ is_featured，不存在"经验但非精选"）。
   * 经验 Tab 的唯一判据就是这个字段；category 是学员自述的意图，别拿它当本字段的别名。
   */
  is_experience?: boolean
  reward_issued?: boolean
}

export interface ForumReplyItem {
  id: number
  topic_id: number
  parent_id?: number | null
  parent_name?: string
  /** 被回复人的头像（ADR-0042「昵称 › 被回复人」行内形态）；顶层回复为空 */
  parent_avatar_url?: string
  content: string
  images?: string[]
  created_at: string
  author: {
    user_id: number
    username: string
    avatar_url: string
  }
  can_delete?: boolean
  likes_count?: number
  liked_by_me?: boolean
  is_accepted?: boolean
}

/** 帖子详情响应（ADR-0042）：`topic` 与 `replies` 平级 + 分页信封 */
export interface ForumTopicDetailData {
  topic: ForumTopicItem
  replies: ForumReplyItem[]
  page: number
  pages: number
  /** 回复总数（含置顶条），与 topic.reply_count 同源 */
  total: number
}

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
    return unwrappedRequest.get<{ topics: ForumTopicItem[]; total: number }>('/forum/topics', { params })
  },

  createTopic(data: { chapter_id?: number | null; category?: ForumPublishCategory; title: string; content: string; images?: string[] }) {
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

  replyTopic(id: number, content: string, parentReplyId?: number | null, images?: string[]) {
    return unwrappedRequest.post<ForumReplyItem>(`/forum/topics/${id}/replies`, {
      content,
      parent_reply_id: parentReplyId || null,
      images: images || []
    })
  },

  // 上传论坛图片（图文分离：先传图拿 URL，随发帖/回复提交 images 数组）
  uploadImage(formData: FormData) {
    return unwrappedRequest.post<{ url: string }>('/forum/upload-image', formData, {
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
    return unwrappedRequest.post<{ likes_count: number; liked: boolean }>(`/forum/topics/${id}/like`)
  },

  /** 取消点赞（幂等） */
  unlikeTopic(id: number) {
    return unwrappedRequest.delete<{ likes_count: number; liked: boolean }>(`/forum/topics/${id}/like`)
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
    return unwrappedRequest.get<{ topics: ForumTopicItem[]; total: number; page: number; pages: number }>(
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
    return unwrappedRequest.get<{ topics: ForumTopicItem[]; total: number; page: number; pages: number }>(
      '/forum/my-liked-topics',
      { params }
    )
  },

  /** 围观（#701：浏览减去四项直接互动，按最近浏览倒序） */
  getMyObservedTopics(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<{ topics: ForumTopicItem[]; total: number; page: number; pages: number }>(
      '/forum/my-observed',
      { params }
    )
  },

  /** 浏览记录（#701：服务端替换本地 localStorage，按主题去重、最近浏览倒序） */
  getMyViewHistory(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<{ topics: ForumTopicItem[]; total: number; page: number; pages: number }>(
      '/forum/my-view-history',
      { params }
    )
  },

  // ===== 评论点赞（spec #268）=====

  /** 点赞评论（幂等） */
  likeReply(id: number) {
    return unwrappedRequest.post<{ likes_count: number; liked: boolean }>(`/forum/replies/${id}/like`)
  },

  /** 取消点赞评论（幂等） */
  unlikeReply(id: number) {
    return unwrappedRequest.delete<{ likes_count: number; liked: boolean }>(`/forum/replies/${id}/like`)
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

/** 我的回复条目（主题被删时 topic_title 为空串，条目保留） */
export interface MyReplyItem {
  id: number
  topic_id: number
  topic_title?: string
  parent_id?: number | null
  content: string
  images?: string[]
  created_at: string
  author: {
    user_id: number
    username: string
    avatar_url: string
  }
}

/** 我的回复分页响应 */
export interface MyRepliesData {
  replies: MyReplyItem[]
  total: number
  page: number
  pages: number
}

// ===== 论坛管理（管理端）=====

export interface AdminForumTopic {
  id: number
  chapter_id?: number | null
  category?: ForumCategory
  chapter_title?: string
  title: string
  content: string
  images?: string[]
  view_count: number
  reply_count: number
  last_reply_at?: string | null
  created_at: string
  /** 精选位（#742） */
  is_featured?: boolean
  /** 备考经验认定（ADR-0040）：管理端授予的归类，蕴含精选；经验 Tab 的筛选判据 */
  is_experience?: boolean
  author: {
    user_id: number
    username: string
    avatar_url: string
  }
}

export interface AdminForumReply {
  id: number
  topic_id: number
  parent_id?: number | null
  parent_name?: string
  content: string
  images?: string[]
  created_at: string
  author: {
    user_id: number
    username: string
    avatar_url: string
  }
}

/** 管理端帖子详情响应（ADR-0042：`topic` 与 `replies` 平级 + 分页信封） */
export interface AdminForumTopicDetailData {
  topic?: AdminForumTopic
  replies?: AdminForumReply[]
  page?: number
  pages?: number
  total?: number
}

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
  listTopics(params: AdminForumListParams) {
    return unwrappedRequest.get<{ topics: AdminForumTopic[]; total: number }>('/admin/forum/topics', { params })
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

  /** 举报列表（status 缺省全部；0 待处理 / 1 已处理） */
  listReports(params: { status?: number; page?: number; page_size?: number }) {
    return unwrappedRequest.get<AdminForumReportsData>('/admin/forum/reports', { params })
  },

  /** 处理举报（status: 0 待处理 / 1 已处理） */
  handleReport(id: number, status: number) {
    return unwrappedRequest.put<null>(`/admin/forum/reports/${id}`, { status })
  }
}

/** 管理端举报条目（与后端 ForumReportDTO 对齐） */
export interface AdminForumReportItem {
  id: number
  reporter_id: number
  reporter?: string
  topic_id?: number | null
  topic_title?: string
  reply_id?: number | null
  reason: string
  status: number
  created_at: string
}

/** 管理端举报列表响应 */
export interface AdminForumReportsData {
  reports: AdminForumReportItem[]
  total: number
  page: number
  pages: number
}
