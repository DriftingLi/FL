import type { Eligibility, Reason } from '@/api/generated/course'
import type { UiTagTone } from '@/components/ui/UiTag.vue'

export interface PublicStateDescription {
  label: string
  tone: UiTagTone
}

const REASON_WORDS: Record<Reason, PublicStateDescription> = {
  manually_hidden: { label: '管理员已关闭公开', tone: 'info' },
  not_published: { label: '未发布', tone: 'info' },
  category_missing: { label: '未设置培训分类', tone: 'danger' },
  category_disabled: { label: '培训分类已停用', tone: 'warning' },
  category_unmapped: { label: '培训分类未映射证件', tone: 'warning' },
  credential_missing: { label: '映射证件不存在', tone: 'danger' },
  credential_disabled: { label: '映射证件已停用', tone: 'warning' },
  source_tag_excluded: { label: '来源标签题不计入公开题库', tone: 'info' }
}

export function describePublicReason(reason: Reason): PublicStateDescription {
  return REASON_WORDS[reason] ?? { label: '未知公开阻断原因', tone: 'neutral' }
}

export function describePublicEligibility(eligibility?: Eligibility): PublicStateDescription {
  if (!eligibility) return { label: '—', tone: 'neutral' }
  if (eligibility.eligible) return { label: '已公开', tone: 'success' }
  return { label: '未公开', tone: 'warning' }
}

export function allowsPublicVisibility(eligibility: Eligibility): boolean {
  return !eligibility.reasons.includes('manually_hidden')
}

export function hasLegacyCategoryReason(reasons?: Reason[]): boolean {
  return reasons?.some(reason => [
    'category_missing',
    'category_disabled',
    'category_unmapped',
    'credential_missing',
    'credential_disabled'
  ].includes(reason)) ?? false
}
