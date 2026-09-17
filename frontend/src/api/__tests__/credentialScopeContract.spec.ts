/**
 * #1106（客户端半边）：证件作用域契约锁 —— 失效注释清零 + 意图拆名 + 显式传参清单。
 *
 * 锁四件事（都用源码扫描，运行期观察不到「文件里不该有某种写法」）：
 *   1. 失效口径清零：api 层不得再出现「拦截器默认注入 credential_id」或「客户端维护豁免表」的说法
 *      —— `api/client.ts` 的请求拦截器只挂 Authorization（ADR-0047 §4 之后客户端不再注入证件）。
 *   2. 显式传参点 = 冻结清单：每个 credential_id 传参点都有「跟随当前 / 浏览指定 / 归属声明」归属
 *      与依据（见下面表）；新增传参点必须回来登记，否则判红。
 *   3. 「跟随当前」不再逐点显式传：student/tutor 页面不得再把 `credentialStore.current?.id`
 *      直接塞进请求参数（白名单 = ContributionTab 一处，见下）。
 *   4. 浏览指定的页面传参点必须用自解释名 `browseCredentialId`。
 *
 * 例外登记落在 `docs/adr/ADR-0056-第十一波架构深化.md` §2：无 JWT 的公开端点（/api/search、/api/courses）
 * 服务端 CredentialScoped 兜底永不生效（兜底分支要求 roleOf(c) == student），客户端必须显式下发。
 */
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { isCommentLine, scanCredentialIdPassSites, type CredentialIdPassSites } from '../credentialScopeScan'

const SRC = resolve(__dirname, '../..')
const read = (rel: string) => readFileSync(resolve(SRC, rel), 'utf8')

/** 扫描面：api 层 + 学员端/导师端页面（本期票面涉及的面；扫描器自身不计入）。 */
const SCAN_DIRS = ['api', 'pages/student', 'pages/tutor']
const SELF = 'api/credentialScopeScan.ts'

/**
 * 显式传参点冻结清单（扫描器口径：字段位 prop + 赋值位 assign；行号只作提示，不参与断言）。
 *
 * | 文件 | 意图 | 依据 |
 * | --- | --- | --- |
 * | api/course.ts prop1 | 入参声明，可选 | /courses 公开端点：服务端兜底不覆盖匿名/非学员，要分区只能显式传 |
 * | api/search.ts prop1 | 入参声明，可选 | /search 公开端点：同上 |
 * | api/favorite.ts prop1 | 入参声明，可选 | /favorites JWT 面：不传 = 服务端兜底，显式 = 浏览指定 |
 * | api/practiceMode.ts prop4 | 入参声明 ×3 + 保存 body ×1 | 练习域 JWT 面（兜底可用）；saveProgress 的 body 是游标分桶必带 |
 * | api/questionBank.ts prop2 | 入参声明，可选 | /question-bank JWT 面：导师端筛选即浏览指定 |
 * | api/training.ts prop2 | 显式传参 | /catalog/tree、/tags 不在 CredentialScoped 里，handler 自读；不传 = 不分区 |
 * | api/contribution.ts prop2 | 入参声明 + 归属声明 body | listPublic 不传即服务端兜底；create 的 body 字段是投稿归属（必填） |
 * | api/credential.ts prop1 + assign1 | 「当前证件」自身写入 | PATCH /me/credential —— 这是切证件本身，与分区语义无关 |
 * | api/admin.ts prop2 / api/recruit.ts prop1 | 管理/招聘筛选入参 | 各自管理面筛选轴，与学员证件分区无关 |
 * | pages/student/SearchPage.vue prop2 | **浏览指定** | 公开端点必须显式传；未选证件时值为 undefined |
 * | pages/student/CourseList.vue assign1 | **浏览指定** | 公开端点必须显式传；未选证件时不传（不分区） |
 * | pages/student/Materials.vue prop1 | **浏览指定** | 公开端点必须显式传（课程筛选选项） |
 * | pages/tutor/QuestionManage.vue assign1 | **浏览指定** | 导师端筛选栏显式挑选证件 |
 * | pages/tutor/TutorCourses.vue assign1 | **浏览指定** | 导师端「我的课程」按证件筛选 |
 * | pages/student/ContributionTab.vue prop2 | **跟随当前** + 归属声明 | prop 是 store.current.id 投影；上传 body 的 credential_id 是投稿归属 |
 * | pages/student/QuestionDetail.vue prop1 | 归属声明 body | 建题时把读到的题目证件带回去 |
 * | pages/tutor/QuestionCreate.vue prop1 | 归属声明 body | 建题选择题目所属证件（表单字段，非分区参数） |
 */
const EXPECTED_SITES: Record<string, { prop: number; assign: number }> = {
  'api/admin.ts': { prop: 2, assign: 0 },
  'api/contribution.ts': { prop: 2, assign: 0 },
  'api/course.ts': { prop: 1, assign: 0 },
  'api/credential.ts': { prop: 1, assign: 1 },
  'api/favorite.ts': { prop: 1, assign: 0 },
  'api/practiceMode.ts': { prop: 4, assign: 0 },
  'api/questionBank.ts': { prop: 2, assign: 0 },
  'api/recruit.ts': { prop: 1, assign: 0 },
  'api/search.ts': { prop: 1, assign: 0 },
  'api/training.ts': { prop: 2, assign: 0 },
  'pages/student/ContributionTab.vue': { prop: 2, assign: 0 },
  'pages/student/CourseList.vue': { prop: 0, assign: 1 },
  'pages/student/Materials.vue': { prop: 1, assign: 0 },
  'pages/student/QuestionDetail.vue': { prop: 1, assign: 0 },
  'pages/student/SearchPage.vue': { prop: 2, assign: 0 },
  'pages/tutor/QuestionCreate.vue': { prop: 1, assign: 0 },
  'pages/tutor/QuestionManage.vue': { prop: 0, assign: 1 },
  'pages/tutor/TutorCourses.vue': { prop: 0, assign: 1 }
}

/** 「跟随当前」显式传参的白名单（值为 store.current.id 的纯投影 + 已有注释说明）。 */
const FOLLOW_CURRENT_ALLOWLIST = ['pages/student/ContributionTab.vue']

/** 浏览指定语义的页面传参点必须用自解释名。 */
const BROWSE_MARKERS: Record<string, string[]> = {
  'pages/student/SearchPage.vue': ['const browseCredentialId'],
  'pages/student/CourseList.vue': ['const browseCredentialId'],
  'pages/student/Materials.vue': ['const browseCredentialId'],
  'pages/tutor/QuestionManage.vue': ['const browseCredentialId'],
}

/** 声称拦截器会注入 credential_id、且同处没有否定词的行（「不再注入」「不注入」是合法口径）。 */
function unnegatedInjectionClaims(text: string): string[] {
  return text
    .split('\n')
    .filter((line) => line.includes('credential_id'))
    .filter((line) => /拦截器/.test(line))
    .filter((line) => !/不/.test(line))
}

const countByFile = (sites: CredentialIdPassSites[]) => {
  const out: Record<string, { prop: number; assign: number }> = {}
  for (const s of sites) out[s.file] = { prop: s.propertySites.length, assign: s.assignmentSites.length }
  return out
}

describe('#1106 证件作用域：客户端不再假装拦截器注入', () => {
  it('api 层无「拦截器默认注入 credential_id」的失效口径，旧注入票引注清零', () => {
    // 旧注入票的引注（详见「客户端半边」票面）全面清零（含 spec），且不得再声称拦截器会注入证件参数。
    // 断言里用拼接构造该字面量：守卫自身不该在源码里留下这个失效引注。
    const staleTicketRef = '#' + '387'
    for (const rel of [
      'api/client.ts',
      'api/search.ts',
      'api/course.ts',
      'api/favorite.ts',
      'api/questionBank.ts',
      'api/practiceMode.ts',
      'api/realExam.ts',
      'api/__tests__/realExam.spec.ts',
      'api/__tests__/practiceMode.spec.ts'
    ]) {
      const text = read(rel)
      expect(text, rel).not.toContain(staleTicketRef)
      expect(unnegatedInjectionClaims(text), rel).toEqual([])
    }
    // 正向锁：唯一提到豁免表的地方必须仍写「不再维护」
    expect(read('api/client.ts')).toContain('豁免表')
  })

  it('显式传参点 = 冻结清单（新增/删除都要回来登记意图与依据）', () => {
    const sites = scanCredentialIdPassSites(SRC, SCAN_DIRS).filter((s) => s.file !== SELF && !s.file.endsWith('.spec.ts'))
    expect(countByFile(sites)).toEqual(EXPECTED_SITES)
  })

  it('浏览指定的页面传参点用自解释名 browseCredentialId', () => {
    for (const [rel, markers] of Object.entries(BROWSE_MARKERS)) {
      for (const marker of markers) expect(read(rel), rel).toContain(marker)
    }
    // 跟随当前的那一处（投稿广场）保留「跟随当前」注释，不得被改写成浏览语义
    expect(read('pages/student/ContributionTab.vue')).toContain('「跟随当前证件」语义')
  })

  it('客户端不得把 credential_id 静默塞进公共请求面（headers / 拦截器 / 全局默认）', () => {
    const sites = scanCredentialIdPassSites(SRC, SCAN_DIRS).filter((s) => s.file !== SELF && !s.file.endsWith('.spec.ts'))
    for (const s of sites) {
      for (const site of [...s.propertySites, ...s.assignmentSites]) {
        expect(site.text, s.file + ':' + site.line).not.toMatch(/headers|interceptor|axios\.defaults/)
      }
    }
  })

  it('「跟随当前」不再逐点显式传：store 投影不得与 credential_id 同现于一行', () => {
    const offenders: string[] = []
    for (const s of scanCredentialIdPassSites(SRC, SCAN_DIRS).filter((x) => x.file !== SELF && !x.file.endsWith('.spec.ts'))) {
      const passLines = [...s.propertySites, ...s.assignmentSites].map((x) => x.line)
      const lines = read(s.file).split('\n')
      for (const line of passLines) {
        if (!/credentialStore\.current|current\?\.id/.test(lines[line - 1] ?? '')) continue
        if (!FOLLOW_CURRENT_ALLOWLIST.includes(s.file)) offenders.push(s.file + ':' + line)
      }
    }
    expect(offenders).toEqual([])
  })

  it('注释行不参与扫描（说明文字不算传参点）', () => {
    expect(isCommentLine('    // credential_id 是说明文字，不算传参点')).toBe(true)
    expect(isCommentLine('  const x = 1')).toBe(false)
  })
})
