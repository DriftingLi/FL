// 联络授权状态词表单点（ADR-0056 §8，issue #1103）。
//
// 取值域是后端 service.ContactGrantState 常量表（backend/internal/service/contact_authz.go 五态）；
// 对账锁在 __tests__/statusWords.spec.ts——后端新增状态而这里忘了加，测试报红。
//
// label / tone 只在这里判定一份：学员侧列表（ResumePage）、企业侧列表（MyRequests）、
// 企业侧简历卡角标（Resumes）、admin 留痕（Inspection）都消费它；
// 模板里不得再内联状态文案裸串（扫描见 __tests__/statusWordsTemplate.spec.ts）。
import type { UiTagTone } from '@/components/ui/UiTag.vue'

/** contact_requests.status 的取值域（与后端 ContactGrantState 五态逐值对齐）。 */
export const CONTACT_REQUEST_STATUSES = ['pending', 'approved', 'rejected', 'expired', 'revoked'] as const

export type ContactRequestStatus = (typeof CONTACT_REQUEST_STATUSES)[number]

/**
 * 企业侧看到的授权展示态（后端 contactGrant.State 的**三值投影**，不是另一套状态）：
 * 五态之一，或空串 = 未授权（拒绝 / 过期 / 已撤回 / 无记录）。
 */
export type ContactState = ContactRequestStatus | ''

/** 状态描述：label 是给用户看的**状态事实**，tone 是 UiTag 的语义色。 */
export interface StatusDescriptor {
  label: string
  tone: UiTagTone
}

/** 唯一判定表：`Record<ContactRequestStatus, …>` 穷尽——漏一个取值编译报错。 */
const DESCRIPTORS: Record<ContactRequestStatus, StatusDescriptor> = {
  pending: { label: '待同意', tone: 'warning' },
  approved: { label: '已同意', tone: 'success' },
  rejected: { label: '已拒绝', tone: 'danger' },
  expired: { label: '已过期', tone: 'info' },
  revoked: { label: '已撤回', tone: 'danger' }
}

/**
 * 输入状态 → `{ label, tone }`。
 * 未知值（后端先上线、前端 union 尚未跟上）回落为原文 + neutral，不留空白标签；
 * 这类漂移由对账锁在测试期拦截，不走运行期。
 */
export function describeContactRequest(status: ContactRequestStatus): StatusDescriptor {
  return DESCRIPTORS[status] ?? { label: String(status), tone: 'neutral' }
}

/**
 * 企业侧简历卡角标：只有「待同意 / 已同意」出角标（与后端三值投影一致），
 * 文案与 tone 仍来自上面唯一表，页面不得自写。
 */
export function contactBadge(state: ContactState | undefined): StatusDescriptor | null {
  return state === 'pending' || state === 'approved' ? DESCRIPTORS[state] : null
}

/**
 * 「授权在、明文取不到」那一维的展示（ADR-0064 决策 5 / 移动端 #1267）。
 *
 * 徽章（contactBadge）与它是**两格正交事实**：企业被禁用不改写授权事实，所以徽章仍按
 * approved 投影；可用性另用同 key 的 `company_disabled` 说。二者不得互相顶替。
 *
 * 判据只写 `=== true`：`company_disabled` 是可选槽，缺席即「无此态」；
 * 若补一个 `=== false` 分支，就把「后端还没这个字段」读成了「企业可用」——那是把未知当结论。
 * 措辞在此单点判定（模板不得内联裸串，扫描见 __tests__/statusWordsTemplate.spec.ts）。
 */
export function companyAvailability(disabled: boolean | undefined): StatusDescriptor | null {
  return disabled === true ? { label: '企业已停用，联系方式已收回', tone: 'warning' } : null
}
