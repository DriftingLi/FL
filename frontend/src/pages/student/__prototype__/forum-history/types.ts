// PROTOTYPE — throwaway types for 论坛浏览记录
export interface ForumHistoryItem {
  id: number
  title: string
  excerpt: string
  author: {
    user_id: number
    username: string
    avatar_url: string
  }
  images_count: number
  view_count: number
  reply_count: number
  viewedAt: string
  deleted?: boolean
}

export interface MockTopic {
  id: number
  title: string
  content: string
  author: {
    user_id: number
    username: string
    avatar_url: string
  }
  images_count: number
  view_count: number
  reply_count: number
}
