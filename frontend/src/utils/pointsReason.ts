// 积分流水 reason → 中文事由/图标色 映射（#512 单点）。
// reason 枚举（与后端簿记对齐）：任务 code（task_*）、ai_tokens、redeem_course、
// redeem_real_paper、redeem_<sku>、accepted_bonus、accept_action、admin_penalty、rollback。
// 未匹配 reason 显示原文不崩（明细页口径：后端是契约，前端文案是展示层）。

export interface LedgerReasonMeta {
  label: string
  /** 收支方向：in 收入 / out 支出 */
  kind: 'in' | 'out'
}

const REASON_LABELS: Record<string, LedgerReasonMeta> = {
  ai_tokens: { label: 'AI 学习助手对话', kind: 'out' },
  redeem_course: { label: '兑换课程', kind: 'out' },
  redeem_real_paper: { label: '兑换真题卷', kind: 'out' },
  accepted_bonus: { label: '问答被采纳奖励', kind: 'in' },
  accept_action: { label: '采纳回答奖励', kind: 'in' },
  admin_penalty: { label: '违规扣减', kind: 'out' },
  rollback: { label: '系统退回', kind: 'in' }
}

/** 商城兑换类 reason 前缀（redeem_<sku> 动态） */
const SHOP_REDEEM_PREFIX = 'redeem_'

export function ledgerReasonMeta(reason: string): LedgerReasonMeta {
  if (!reason) return { label: '积分变动', kind: 'in' }
  if (REASON_LABELS[reason]) return REASON_LABELS[reason]
  // redeem_<sku> 动态键：前缀识别为商城兑换支出
  if (reason.startsWith(SHOP_REDEEM_PREFIX)) {
    return { label: '商城兑换', kind: 'out' }
  }
  // 任务 code（task 前缀或其它未收录）：按 delta 方向兜底，label 显原文
  return { label: reason, kind: 'unknown' as never }
}

/** 方向归一：delta 正负为准（rollback 等 + 方向对冲不受 reason 影响） */
export function deltaKind(delta: number): 'in' | 'out' {
  return delta >= 0 ? 'in' : 'out'
}
