import request from './request'

export interface ForumTopicItem {
  id: number
  chapter_id?: number | null
  chapter_title?: string
  title: string
  content: string
  view_count: number
  reply_count: number
  last_reply_at?: string | null
  created_at: string
  author: {
    user_id: number
    username: string
    name: string
    nickname: string
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
  created_at: string
  author: {
    user_id: number
    username: string
    name: string
    nickname: string
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
    return request.get('/forum/topics', { params })
  },

  createTopic(data: { chapter_id?: number | null; title: string; content: string }) {
    return request.post('/forum/topics', data)
  },

  getTopic(id: number) {
    return request.get(`/forum/topics/${id}`)
  },

  replyTopic(id: number, content: string, parentReplyId?: number | null) {
    return request.post(`/forum/topics/${id}/replies`, { content, parent_reply_id: parentReplyId || null })
  },

  deleteTopic(id: number) {
    return request.delete(`/forum/topics/${id}`)
  },

  deleteReply(id: number) {
    return request.delete(`/forum/replies/${id}`)
  }
}
