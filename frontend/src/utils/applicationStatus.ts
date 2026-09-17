// 投递状态词表单点（ADR-0056 §8，issue #1103）。
//
// 取值域是后端 service.ApplicationStatus* 常量表（backend/internal/service/job_application_service.go）；
// 对账锁在 __tests__/statusWords.spec.ts——后端新增状态而这里忘了加，测试报红。
//
// label / tone 只在这里判定一份：学员侧「我的投递」（MyApplications）、企业侧投递列表
// （ApplicationList，含详情抽屉）都消费它；模板里不得再内联状态文案裸串
// （扫描见 __tests__/statusWordsTemplate.spec.ts）。
import type { UiTagTone } from '@/components/ui/UiTag.vue'

/** job_applications.status 的取值域（与后端 ApplicationStatus* 三态逐值对齐）。 */
export const APPLICATION_STATUSES = ['applied', 'rejected', 'withdrawn'] as const

export type ApplicationStatus = (typeof APPLICATION_STATUSES)[number]

/** 状态描述：label 是给用户看的**状态事实**，tone 是 UiTag 的语义色。 */
export interface StatusDescriptor {
  label: string
  tone: UiTagTone
}

/** 唯一判定表：`Record<ApplicationStatus, …>` 穷尽——漏一个取值编译报错。 */
const DESCRIPTORS: Record<ApplicationStatus, StatusDescriptor> = {
  applied: { label: '投递中', tone: 'warning' },
  rejected: { label: '不合适', tone: 'danger' },
  withdrawn: { label: '已撤回', tone: 'info' }
}

/** 输入状态 → `{ label, tone }`；未知值回落为原文 + neutral（同联络授权侧口径）。 */
export function describeApplication(status: ApplicationStatus): StatusDescriptor {
  return DESCRIPTORS[status] ?? { label: String(status), tone: 'neutral' }
}
