import { unwrappedRequest } from './request'

export interface ForumTopicItem {
  id: number
  chapter_id?: number | null
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
}

export interface ForumListParams {
  scope?: 'all' | 'general' | 'chapter'
  chapter_id?: number
  page?: number
  page_size?: number
  keyword?: string
  sort?: 'latest' | 'hot'
  order?: 'asc' | 'desc'
}

export const forumApi = {
  listTopics(params: ForumListParams) {
    return unwrappedRequest.get<{ topics: ForumTopicItem[]; total: number }>('/forum/topics', { params })
  },

  createTopic(data: { chapter_id?: number | null; title: string; content: string; images?: string[] }) {
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

  // ===== 打卡（spec #268）=====

  /** 打卡（幂等） */
  checkIn() {
    return unwrappedRequest.post<{ checked: boolean; streak: number; total: number; today_checked: boolean }>('/forum/check-in')
  },

  /** 日历（按月） */
  getCheckInCalendar(params: { year: number; month: number }) {
    return unwrappedRequest.get<{ dates: string[]; streak: number; total: number; today_checked: boolean }>('/forum/check-in/calendar', { params })
  },

  /** 排行榜（累计总榜） */
  getCheckInRank(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<{ items: CheckInRankItem[]; total: number; page: number; pages: number; me: CheckInRankMe | null }>('/forum/check-in/rank', { params })
  },

  // ===== 评论点赞（spec #268）=====

  /** 点赞评论（幂等） */
  likeReply(id: number) {
    return unwrappedRequest.post<{ likes_count: number; liked: boolean }>(`/forum/replies/${id}/like`)
  },

  /** 取消点赞评论（幂等） */
  unlikeReply(id: number) {
    return unwrappedRequest.delete<{ likes_count: number; liked: boolean }>(`/forum/replies/${id}/like`)
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

/** 打卡排行榜条目 */
export interface CheckInRankItem {
  rank: number
  user: { user_id: number; username: string; avatar_url: string }
  total: number
  streak: number
  today_checked: boolean
}

/** 当前用户排名（不在前 N 页时置顶） */
export interface CheckInRankMe {
  rank: number
  total: number
  streak: number
  today_checked: boolean
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
