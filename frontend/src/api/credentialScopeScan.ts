/**
 * 证件作用域「显式传参点」源码扫描器（#1106 客户端半边）。
 *
 * 为什么需要它：同一个 wire key `credential_id` 在客户端承载两种相反意图——
 *   ① 「跟随当前」（跟随 store.current.id，或服务端 CredentialScoped 兜底）；
 *   ② 「浏览指定」（用户显式挑选的证件；公开端点要分区也只能走这条）。
 * 两种意图在请求形状上无法区分，只能靠注释与命名自解释 + 本扫描器的静态清单把它钉住。
 *
 * 扫描口径（逐行文本，**不做引号/模板字面量解析**——那种解析在本仓的中文弯引号模板串上不健壮）：
 *   - 字段位：`credential_id:` / `credential_id?:`（请求参数、入参声明、归属声明 body）；
 *   - 赋值位：`credential_id = <表达式>`、`params.credential_id = <表达式>` 与对象简写
 *     `credential_id,` / `credential_id}`；
 * 不计：属性读取（`row.credential_id` 后不接 `=`）、比较运算（`===`）、置空默认值
 * （`credential_id: null` —— 表单初值，不是传参）、纯注释行。
 *
 * 边界：`credential_id` 作为 body 字段（投稿创建、题库创建等「归属声明」）会被计入——它是同一
 * wire key 的第三种用法，冻结清单里单独标注。**行号只作提示，不参与测试断言**（改名/插行不该判红），
 * 断言只按「文件 → 计数」。
 *
 * 本 module 是测试（以及将来可能的 CI 守卫）共用的唯一扫描事实源：不要在两处各写一份扫描逻辑。
 */
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative, resolve, sep } from 'node:path'

/** 一个显式传参点：行号 + 该行原文（截断），便于人工核对冻结清单。 */
export interface CredentialIdSite {
  line: number
  text: string
}

/** 单文件扫描结果：字段位与赋值位分开计数。 */
export interface CredentialIdPassSites {
  file: string
  /** `credential_id:` / `credential_id?:` 字段位。 */
  propertySites: CredentialIdSite[]
  /** `credential_id = …`、`params.credential_id = …` 与对象简写。 */
  assignmentSites: CredentialIdSite[]
}

/** 纯注释行判定（行首 `//` 或块注释续行 `*`）。 */
export function isCommentLine(line: string): boolean {
  const t = line.trim()
  return t.startsWith('//') || t.startsWith('/*') || t.startsWith('*')
}

const isNullLiteral = (rest: string) => /^\s*null\b/.test(rest)

/** 扫描一段源码文本（rel 只用于回填 file 字段）。 */
export function scanCredentialIdPassSitesInText(rel: string, raw: string): CredentialIdPassSites {
  const propertySites: CredentialIdSite[] = []
  const assignmentSites: CredentialIdSite[] = []
  const lines = raw.split('\n')
  for (let idx = 0; idx < lines.length; idx++) {
    const line = lines[idx]
    if (!line.includes('credential_id') || isCommentLine(line)) continue
    const text = line.trim().slice(0, 120)
    for (let i = 0; i < line.length; i++) {
      if (!line.startsWith('credential_id', i)) continue
      const rest = line.slice(i + 'credential_id'.length)
      // 属性读取（row.credential_id）只在后接 = 时算「写进请求参数」，否则跳过
      if (line[i - 1] === '.' && !/^\s*=/.test(rest)) continue
      if (/^\s*[?:]/.test(rest)) {
        const afterColon = rest.replace(/^\s*\??\s*:/, '')
        if (!isNullLiteral(afterColon)) propertySites.push({ line: idx + 1, text })
      } else if (/^\s*=/.test(rest)) {
        assignmentSites.push({ line: idx + 1, text })
      } else if (/^\s*[,}]/.test(rest)) {
        // 对象简写：{ credential_id } / { index, credential_id }
        assignmentSites.push({ line: idx + 1, text })
      }
    }
  }
  return { file: rel, propertySites, assignmentSites }
}

/** 递归收集目录下的 .ts / .vue 源文件，跳过 __tests__ 与生成物/bundler 目录。 */
function collectSourceFiles(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir)) {
    if (entry === '__tests__' || entry === 'generated' || entry === 'node_modules') continue
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) out.push(...collectSourceFiles(full))
    else if (/\.(ts|vue)$/.test(entry)) out.push(full)
  }
  return out
}

/** 扫描给定相对目录（如 api / pages/student）下的全部源码，返回非空的单文件结果。 */
export function scanCredentialIdPassSites(root: string, relDirs: string[]): CredentialIdPassSites[] {
  const results: CredentialIdPassSites[] = []
  for (const relDir of relDirs) {
    for (const full of collectSourceFiles(resolve(root, relDir))) {
      const rel = relative(root, full).split(sep).join('/')
      const res = scanCredentialIdPassSitesInText(rel, readFileSync(full, 'utf8'))
      if (res.propertySites.length > 0 || res.assignmentSites.length > 0) results.push(res)
    }
  }
  return results
}
