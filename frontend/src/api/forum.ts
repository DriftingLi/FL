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
}

export interface ForumListParams {
  scope?: 'all' | 'general' | 'chapter'
  chapter_id?: number
  page?: number
  page_size?: number
  keyword?: string
}

export const forumApi = {
  listTopics(params: ForumListParams) {
    return unwrappedRequest.get<{ topics: ForumTopicItem[]; total: number }>('/forum/topics', { params })
  },

  createTopic(data: { chapter_id?: number | null; title: string; content: string; images?: string[] }) {
    return unwrappedRequest.post<ForumTopicItem>('/forum/topics', data)
  },

  getTopic(id: number) {
    return unwrappedRequest.get<{ topic: ForumTopicItem; replies: ForumReplyItem[] }>(`/forum/topics/${id}`)
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
  }
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
  }
}
