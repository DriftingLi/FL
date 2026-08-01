import request from './request'

export interface ForumTopicItem {
  id: number
  course_id?: number | null
  course_name?: string
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
  scope?: 'all' | 'general' | 'course'
  course_id?: number
  page?: number
  page_size?: number
  keyword?: string
}

export const forumApi = {
  listTopics(params: ForumListParams) {
    return request.get('/forum/topics', { params })
  },

  createTopic(data: { course_id?: number | null; title: string; content: string }) {
    return request.post('/forum/topics', data)
  },

  getTopic(id: number) {
    return request.get(`/forum/topics/${id}`)
  },

  replyTopic(id: number, content: string) {
    return request.post(`/forum/topics/${id}/replies`, { content })
  },

  deleteTopic(id: number) {
    return request.delete(`/forum/topics/${id}`)
  },

  deleteReply(id: number) {
    return request.delete(`/forum/replies/${id}`)
  }
}
