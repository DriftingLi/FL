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

/**
 * 属地展示文案（ADR-0045）：**市优先、市为空退到省**。
 *
 * 为什么只显示一级：署名行是窄的（时间 · 属地 · 楼主 / 已采纳标签），两级叠加会互相挤压；
 * 数据仍按两级存（ip_province / ip_city），日后想改成「省 · 市」不必回填。
 * 两者都空时返回空串——展示侧据此**整段不渲染**（不显示「未知」、不留悬空的分隔符）。
 */
export function regionLabel(item: { ip_province?: string; ip_city?: string }): string {
  return item.ip_city || item.ip_province || ''
}

/**
 * 发布前的属地披露文案（ADR-0045）。
 *
 * ⚠️ 这一行是 docs/agents/ui-conventions.md「不写说明性 hint 文本」的**明确例外**：
 * 属地是发布那一刻才产生的事实，事前告知比事后解释便宜——它是功能性提示，不是装饰。
 * 约定文件里已登记该例外，清理 hint 文案时不要连它一起删。
 */
export const FORUM_REGION_NOTICE = '发布内容会显示 IP 属地'
