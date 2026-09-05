import { unwrappedRequest } from './request'

/** 论坛帖子类别（#364）：判"帖子意图"的唯一依据，与判区域的 chapter_id 正交。 */
export type ForumCategory = 'discussion' | 'question'

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
  reward_issued?: boolean
}

export interface ForumReplyItem {
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
  can_delete?: boolean
  likes_count?: number
  liked_by_me?: boolean
  is_accepted?: boolean
}

export interface ForumListParams {
  scope?: 'all' | 'general' | 'chapter'
  /** 类别分流；省略或 'all' 表示两类都看（移动端旧契约即如此） */
  category?: ForumCategory
  /** 解决状态（#367，仅问答有意义） */
  solved?: 'all' | 'solved' | 'unsolved'
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
 * ⚠️ 两个轴不可互相替代：scope 判**区域**（chapter_id IS NULL），category 判**类别**。
 * 讨论 Tab 必须同时带 category —— 问答帖的 chapter_id 也是 NULL，漏掉它问答帖会整片灌进讨论列表。
 */
export function forumTabQuery(tab: ForumTab): Pick<ForumListParams, 'scope' | 'category'> {
  switch (tab) {
    case 'discussion':
      // 讨论 Tab 沿用既有口径：只看综合区、不合并章节讨论。
      return { scope: 'general', category: 'discussion' }
    case 'question':
      // 问答帖按设计无章节归属，没有区域维度可筛，故不带 scope。
      return { category: 'question' }
    default:
      // 全部：两个参数都不带，与改动前逐条一致（服务端默认 scope=all、不过滤类别）。
      return {}
  }
}

export const forumApi = {
  listTopics(params: ForumListParams) {
    return unwrappedRequest.get<{ topics: ForumTopicItem[]; total: number }>('/forum/topics', { params })
  },

  createTopic(data: { chapter_id?: number | null; category?: ForumCategory; title: string; content: string; images?: string[] }) {
    return unwrappedRequest.post<ForumTopicItem>('/forum/topics', data)
  },

  getTopic(id: number, sort?: 'latest' | 'hot' | 'time', order?: 'asc' | 'desc') {
    const params: Record<string, string> = {}
    if (sort) params.sort = sort
    if (order) params.order = order
    return unwrappedRequest.get<{ topic: ForumTopicItem; replies: ForumReplyItem[] }>(`/forum/topics/${id}`, { params: Object.keys(params).length ? params : undefined })
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

export interface AdminForumListParams {
  scope?: 'all' | 'general' | 'chapter'
  /** 类别维度（#364）；省略表示两类都看 */
  category?: ForumCategory
  solved?: 'all' | 'solved' | 'unsolved'
  chapter_id?: number
  page?: number
  page_size?: number
  keyword?: string
}

export const adminForumApi = {
  listTopics(params: AdminForumListParams) {
    return unwrappedRequest.get<{ topics: AdminForumTopic[]; total: number }>('/admin/forum/topics', { params })
  },

  getTopic(id: number) {
    return unwrappedRequest.get<{ topic?: AdminForumTopic; replies?: AdminForumReply[] }>(`/admin/forum/topics/${id}`)
  },

  deleteTopic(id: number) {
    return unwrappedRequest.delete<null>(`/admin/forum/topics/${id}`)
  },

  deleteReply(id: number) {
    return unwrappedRequest.delete<null>(`/admin/forum/replies/${id}`)
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
