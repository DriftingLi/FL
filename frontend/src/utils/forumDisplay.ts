import type { ForumTopicItem } from '@/api/forum'

export type ForumAuthor = ForumTopicItem['author']

/** 论坛作者展示名（#389 收编：帖子列表 / 帖子详情 / 章节讨论三处重复） */
export function displayName(author: ForumAuthor): string {
  return author.username
}

/** 头像占位字母（展示名首字母大写；空名兜底「?」） */
export function authorLetter(author: ForumAuthor): string {
  return (displayName(author) || '?').charAt(0).toUpperCase()
}
