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
import { readFileSync, readdirSync } from 'node:fs'
import { relative, resolve } from 'node:path'
import { parse, compileTemplate } from 'vue/compiler-sfc'
import { describeRole, RETIRED_ROLE_WORDS } from '../roleWords'
import type { AuthzRole } from '@/config/authz'

const SRC = resolve(__dirname, '../..')

/**
 * 扫描面 = src 下**全部** SFC。票7 的收敛完成后不留清单式白名单：
 * 「漏列一个文件」这种漏扫，清单本身兜不住（只有文件被删/改名才会读取失败而暴露）。
 * 测试目录不扫——用例里写裸称谓是正当的。
 */
function allSfcFiles(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const p = resolve(dir, entry.name)
    if (entry.isDirectory()) {
      if (entry.name !== '__tests__' && entry.name !== 'node_modules') out.push(...allSfcFiles(p))
    } else if (entry.name.endsWith('.vue')) {
      out.push(p)
    }
  }
  return out
}

const SCAN_FILES = allSfcFiles(SRC)

const ROLE_WORDS: string[] = [...new Set((['hrwai_user', 'tutor', 'admin', 'recruiter'] as AuthzRole[]).map(describeRole))]

/**
 * 全站扫的裸串集合只取**无歧义**的两个：本票裁定的那个词（讲师）与它的退役别名（导师）。
 * 「学员 / 企业 / 管理员」同时是日常名词（CompanyContactInfo 的「企业」是抬头、
 * CheckInPage 的「学员」是文案主语），全站禁裸串会把正当用法一起判红——
 * 与 #1103 当年「『待处理』是通用词，只在写错的消费面禁回流」同一取舍。
 */
const GLOBAL_SCAN_WORDS = [describeRole('tutor'), ...RETIRED_ROLE_WORDS]

function stringLiterals(src: string): string[] {
  const { descriptor } = parse(src)
  const { code } = compileTemplate({ source: descriptor.template?.content ?? '', id: 'role-word-scan', filename: 'role-word-scan.vue' })
  return [...code.matchAll(/"([^"\\]*)"|'([^'\\]*)'/g)].map(m => (m[1] ?? m[2]) as string)
}

describe('角色称谓单点（票7）', () => {
  it('词表四个角色的称谓互不相同（撞名就意味着某一处又拿称谓指别的东西）', () => {
    expect(new Set(ROLE_WORDS).size).toBe(4)
  })

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

  it('全站 SFC 模板都不内联称谓裸串（含退役别名「导师」）', () => {
    expect(SCAN_FILES.length, '扫描面为空即扫描器坏了').toBeGreaterThan(80)
    const offenders: string[] = []
    for (const file of SCAN_FILES) {
      const literals = stringLiterals(readFileSync(file, 'utf8'))
      const bad = literals.filter(l => GLOBAL_SCAN_WORDS.includes(l))
      if (bad.length) offenders.push(relative(SRC, file) + ' → ' + bad.join('/'))
    }
    expect(offenders, '这些模板把称谓写成了裸串：\n' + offenders.join('\n')).toEqual([])
  })

  it('负向探针：模板里注入「讲师」裸串或退役别名「导师」，扫描必红', () => {
    const hit = stringLiterals('<template><span>导师</span><b>讲师</b></template>')
    expect(hit).toContain('导师')
    expect(hit).toContain('讲师')
  })
})
