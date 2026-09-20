// 角色称谓词表单点（ADR-0060 票7 / spec #1201 场景 5、6）。
//
// 一屏里同一个角色出现过两个名字：侧栏叫「导师」、审计页叫「讲师」、后端提示语也叫「导师」，
// 而 `CONTEXT.md` 认定的是「讲师」。本文件是**唯一**的称谓声明处——词表是仲裁者，
// 运行期文案向它收敛，**不按 caller 视角分档**（学员侧与管理侧同名，见 CONTEXT.md「讲师」词条：
// 「导师」是历史漂移别名）。
//
// 角色集合不自立一份清单，直接吃 `config/authz` 的 AuthzRole；
// `Record<AuthzRole, string>` 穷尽——authz 加一个角色而这里没跟上，编译报错。
// 模板/串里不得再内联称谓裸串（扫描见 __tests__/statusWordsTemplate.spec.ts 的角色维度）。
import type { AuthzRole } from '@/config/authz'

/** 已退役的漂移别名：出现即视为回归（供扫描与 RETIRED 表引用）。 */
export const RETIRED_ROLE_WORDS: readonly string[] = ['导师']

const DESCRIPTORS: Record<AuthzRole, string> = {
  hrwai_user: '学员',
  tutor: '讲师',
  admin: '管理员',
  recruiter: '企业'
}

/** 输入角色 → 用户看到的称谓；未知取值回落为「用户」（与收敛前的侧栏徽章同口径）。 */
export function describeRole(role: AuthzRole | string | undefined | null): string {
  return DESCRIPTORS[role as AuthzRole] ?? '用户'
}
