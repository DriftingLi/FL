// 反馈状态词表单点（ADR-0089 决策 1：只有两态）。
//
// 取值域是后端 model.FeedbackStatus* 常量表（backend/internal/model/feedback.go）；
// label / tone 只在这里判定一份——管理端列表、行状态标签都消费它，
// 模板里不得再内联状态文案裸串。
//
// 为什么「已处理」不叫「已回复」：标记不向学员发任何消息（ADR-0089 决策 8），
// 把状态读成「回复」会让运营以为学员收到了答复。
import type { UiTagTone } from '@/components/ui/UiTag.vue'

/** feedback.status 的取值域（与后端两态逐值对齐）。 */
export const FEEDBACK_STATUSES = ['pending', 'handled'] as const

export type FeedbackStatus = (typeof FEEDBACK_STATUSES)[number]

/** 状态描述：label 是给用户看的**状态事实**，tone 是 UiTag 的语义色。 */
export interface FeedbackStatusDescriptor {
  label: string
  tone: UiTagTone
}

/** 唯一判定表：穷尽映射——漏一个取值编译报错。 */
const DESCRIPTORS: Record<FeedbackStatus, FeedbackStatusDescriptor> = {
  pending: { label: '待处理', tone: 'warning' },
  handled: { label: '已处理', tone: 'success' }
}

/** 输入状态 → `{ label, tone }`；未知值回落为原文 + neutral。 */
export function describeFeedbackStatus(status: string): FeedbackStatusDescriptor {
  return DESCRIPTORS[status as FeedbackStatus] ?? { label: String(status), tone: 'neutral' }
}
