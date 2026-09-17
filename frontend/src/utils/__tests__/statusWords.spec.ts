// 状态词表单点（#1103 / ADR-0056 §8）：
// ① 取值集合与后端 Go 常量表对账——后端新增状态而前端 union 忘了加，本文件必红；
// ② descriptor 的 label / tone 单点（漂移文案已按「状态事实」统一，这里锁住统一结果）。
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { CONTACT_REQUEST_STATUSES, contactBadge, describeContactRequest } from '../contactRequestStatus'
import { APPLICATION_STATUSES, describeApplication } from '../applicationStatus'

/** 车道根（frontend/src/utils/__tests__ → frontend → 仓库根）。 */
const ROOT = resolve(__dirname, '../../../..')

const GO_CONTACT = 'backend/internal/service/contact_authz.go'
const GO_APPLICATION = 'backend/internal/service/job_application_service.go'

// 命名约定（Go 侧注释冻结）：状态常量名为 `ContactGrantXxx`（类型 ContactGrantState）/
// `ApplicationStatusXxx`。改名会让下面的正则一个也匹配不到，`not.toHaveLength(0)` 即报红。

/** 联络授权常量表：`ContactGrantXxx ContactGrantState = "值"`（ContactGrantSource 那组类型不同，不会误入）。 */
const CONTACT_CONST_RE = /^\s*ContactGrant\w+\s+ContactGrantState\s*=\s*"([^"]+)"/gm
/** 投递常量表：`ApplicationStatusXxx = "值"`。 */
const APPLICATION_CONST_RE = /^\s*ApplicationStatus\w+\s*=\s*"([^"]+)"/gm

/** 抠出常量值；正则失配返回空数组——对账会红，不会假绿。 */
function goConstantValues(src: string, constRe: RegExp): string[] {
  return [...src.matchAll(constRe)].map((m) => m[1] as string)
}

const sorted = (values: readonly string[]) => [...values].sort()

describe('取值集合与后端常量表一致（#1103 对账锁）', () => {
  it('contact：TS union == service.ContactGrantState', () => {
    const values = goConstantValues(readFileSync(resolve(ROOT, GO_CONTACT), 'utf8'), CONTACT_CONST_RE)
    expect(values).not.toHaveLength(0)
    expect(sorted(values)).toEqual(sorted(CONTACT_REQUEST_STATUSES))
  })

  it('application：TS union == service.ApplicationStatus*', () => {
    const values = goConstantValues(readFileSync(resolve(ROOT, GO_APPLICATION), 'utf8'), APPLICATION_CONST_RE)
    expect(values).not.toHaveLength(0)
    expect(sorted(values)).toEqual(sorted(APPLICATION_STATUSES))
  })

  it('负样本：Go 表少一态 → 对账必红（自检正则确实在看这张表）', () => {
    const src = readFileSync(resolve(ROOT, GO_CONTACT), 'utf8')
      .replace('ContactGrantRevoked ContactGrantState = "revoked"', '')
    expect(sorted(goConstantValues(src, CONTACT_CONST_RE))).not.toEqual(sorted(CONTACT_REQUEST_STATUSES))
  })
})

describe('descriptor 的 label / tone 单点（#1103）', () => {
  it('contact 五态：pending = 「待同意」（原「待处理」按状态事实统一）', () => {
    expect(CONTACT_REQUEST_STATUSES.map((s) => describeContactRequest(s).label))
      .toEqual(['待同意', '已同意', '已拒绝', '已过期', '已撤回'])
  })

  it('contact 五态：tone 单点（原 MyRequests primary / Resumes warning 两处已统一）', () => {
    expect(CONTACT_REQUEST_STATUSES.map((s) => describeContactRequest(s).tone))
      .toEqual(['warning', 'success', 'danger', 'info', 'danger'])
  })

  it('application 三态：applied = 「投递中」（原「待处理」按状态事实统一）', () => {
    expect(APPLICATION_STATUSES.map((s) => describeApplication(s).label))
      .toEqual(['投递中', '不合适', '已撤回'])
  })

  it('企业侧角标：只有 pending / approved 出角标，未授权（空 / 缺省）为 null', () => {
    expect(contactBadge(undefined)).toBeNull()
    expect(contactBadge('')).toBeNull()
    expect(contactBadge('rejected')).toBeNull()
    expect(contactBadge('pending')?.label).toBe('待同意')
    expect(contactBadge('approved')).toEqual({ label: '已同意', tone: 'success' })
  })
})
