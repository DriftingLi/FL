// 运行时能力判定（#1618 段1）：前端「谁能做什么」的**唯一入口**。
//
// 为什么需要它：生成物 config/authz.ts 的 ROLE_CAPABILITIES 只回答**静态角色**
// （学员 / 讲师 / 招聘者）。管理端角色（DYNAMIC_ROLES）的权限由超管按角色分配，
// 登录后经 GET /admin/me/capabilities 下发，静态表对它**有意为空**（fail closed）。
// 于是判定有两条路径，必须收在一个函数里 —— 守卫与侧栏各写一遍时，漂移的表现是
// 「菜单看得见、点进去 403」这种最难归因的一类缺陷。
import { DYNAMIC_ROLES, hasCapability, type AuthzCapability, type AuthzRole } from '@/config/authz'

/** 该角色是否为动态角色（能力由数据层回答，不来自静态表）。 */
export function isDynamicRole(role: string | null | undefined): boolean {
  return !!role && (DYNAMIC_ROLES as readonly string[]).includes(role)
}

/**
 * 判定「当前身份是否拥有某能力」。
 *
 * - 静态角色 → 生成的能力表；
 * - 动态角色 → **只**认运行时能力集：集合尚未加载时返回 false（fail closed）。
 *   这里刻意**不回落**到静态表 —— 该表对动态角色是有意为空的，回落等于放行。
 */
export function holdsCapability(
  role: string | null | undefined,
  runtimeCapabilities: readonly string[],
  capability: AuthzCapability
): boolean {
  if (isDynamicRole(role)) {
    return runtimeCapabilities.includes(capability)
  }
  return hasCapability(role as AuthzRole, capability)
}
