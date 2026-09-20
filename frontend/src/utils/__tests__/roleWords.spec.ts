// 角色称谓单点锁（ADR-0060 票7 / spec #1201 场景 5、6）。
//
// 两件事：
//   1. 词表本身——canonical 是「讲师」，且 `Record<AuthzRole, string>` 穷尽（加角色不补称谓即编译报错）；
//   2. 消费面不得内联称谓裸串——沿用 #1103 的模板编译扫描形态（ADR-0056 §8），
//      把白名单从 status 维度扩到 role 维度。
//
// ⚠️ SCAN_PAGES 随收敛进度增长；全量收敛完成后应改为「扫 src 下所有 SFC」并删掉本清单
// （清单式扫描的漏扫风险由「文件不存在即读取失败」兜住，但漏列文件兜不住）。
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { parse, compileTemplate } from 'vue/compiler-sfc'
import { describeRole, RETIRED_ROLE_WORDS } from '../roleWords'
import type { AuthzRole } from '@/config/authz'

const SRC = resolve(__dirname, '../..')

/** 已收敛到词表的消费面（模板里出现称谓裸串即红）。 */
const SCAN_PAGES = [
  'components/layout/AppSidebar.vue',
  'pages/auth/Login.vue',
  'pages/tutor/Dashboard.vue',
  'pages/admin/AuditLogs.vue',
  'pages/admin/TutorManage.vue'
]

const ROLE_WORDS: string[] = [...new Set((['hrwai_user', 'tutor', 'admin', 'recruiter'] as AuthzRole[]).map(describeRole))]

function stringLiterals(src: string): string[] {
  const { descriptor } = parse(src)
  const { code } = compileTemplate({ source: descriptor.template?.content ?? '', id: 'role-word-scan', filename: 'role-word-scan.vue' })
  return [...code.matchAll(/"([^"\\]*)"|'([^'\\]*)'/g)].map(m => (m[1] ?? m[2]) as string)
}

describe('角色称谓单点（票7）', () => {
  it('canonical：tutor 的称谓是「讲师」，不是漂移别名', () => {
    expect(describeRole('tutor')).toBe('讲师')
  })

  it('词表覆盖 AuthzRole 的每一个取值（穷尽由 Record 在编译期保证，这里兜运行期回落）', () => {
    for (const role of ['hrwai_user', 'tutor', 'admin', 'recruiter'] as AuthzRole[]) {
      expect(typeof describeRole(role)).toBe('string')
      expect(describeRole(role)).not.toHaveLength(0)
    }
    expect(describeRole('no-such-role')).toBe('用户')
    expect(describeRole(undefined)).toBe('用户')
  })

  for (const page of SCAN_PAGES) {
    it(`${page} 的模板不内联称谓裸串（含退役别名）`, () => {
      const literals = stringLiterals(readFileSync(resolve(SRC, page), 'utf8'))
      expect(literals.filter(l => ROLE_WORDS.includes(l))).toEqual([])
      expect(literals.filter(l => [...RETIRED_ROLE_WORDS].includes(l))).toEqual([])
    })
  }

  it('负向探针：模板里注入「讲师」裸串或退役别名「导师」，扫描必红', () => {
    const hit = stringLiterals('<template><span>导师</span><b>讲师</b></template>')
    expect(hit).toContain('导师')
    expect(hit).toContain('讲师')
  })
})
