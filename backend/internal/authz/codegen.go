package authz

import (
	"errors"
	"fmt"
	"strings"
)

// 前端授权配置窄域生成器（ADR-0047 §1 / spec #928 决策 5）：把「角色 → 能力」渲染为
// frontend/src/config/authz.ts（生成勿改）。渲染是纯函数（能力表快照 → 输出字符串），
// 输出确定性（无时间戳、无随机序）是「字节级全等」契约测试的前提；生成物过期由
// codegen_test.go 直接变红暴露，再生成入口在 cmd/gen-authz。
//
// 派生面只含**角色与能力**：页面 → 所需能力是前端域（页面描述符，候选 06），不进生成面。

// authzTSTemplate 生成物模板（%s₁ = 角色联合类型成员，%s₂ = 能力联合类型成员，
// %s₃ = ROLE_CAPABILITIES 字面量）。无时间戳：时间戳使再生成永不幂等。
const authzTSTemplate = `// 生成文件，勿手改（ADR-0047 §1 授权能力声明，spec #928）。
// 唯一事实源：后端能力表 backend/internal/authz/authz.go。
// 再生成：cd backend && go run ./cmd/gen-authz
// 同步契约：backend/internal/authz/codegen_test.go 将本文件与能力表渲染结果全等比对，
// 手改或能力表变更未再生成时后端测试即红。
//
// 用法：页面/路由声明「需要什么能力」（AuthzCapability），角色可达面由 ROLE_CAPABILITIES
// 回答。**不要**在前端另抄一份角色清单——那是本文件要消灭的东西。

export type AuthzRole =
%s

export type AuthzCapability =
%s

/** 角色 → 能力集合（按能力键字典序，生成序稳定）。 */
export const ROLE_CAPABILITIES: Readonly<Record<AuthzRole, readonly AuthzCapability[]>> = {
%s}

/** 判定角色是否拥有能力：未知角色或未登记能力一律 false（与后端 Has 同口径，fail closed）。 */
export function hasCapability(role: AuthzRole | '' | null | undefined, capability: AuthzCapability): boolean {
  if (!role) return false
  const caps = ROLE_CAPABILITIES[role]
  return !!caps && caps.includes(capability)
}
`

// orderedRoles 角色声明序（生成物中角色成员的顺序；与 authz.go 的常量声明一致）。
var orderedRoles = []Role{RoleStudent, RoleTutor, RoleAdmin, RoleRecruiter}

// RenderFrontendAuthzTS 渲染前端授权配置内容（纯函数，确定性输出）。
// 能力表为空、或某角色没有任何能力时拒绝生成——那属能力表事故，不应静默产出空配置。
func RenderFrontendAuthzTS() (string, error) {
	if len(roleCapabilities) == 0 {
		return "", errors.New("能力表为空，拒绝生成前端授权配置")
	}
	var roles, caps, table strings.Builder
	for _, r := range orderedRoles {
		fmt.Fprintf(&roles, "  | '%s'\n", r)
	}
	for _, c := range AllCapabilities() {
		fmt.Fprintf(&caps, "  | '%s'\n", c)
	}
	if caps.Len() == 0 {
		return "", errors.New("能力表筛出 0 个能力，拒绝生成空配置")
	}
	for _, r := range orderedRoles {
		list := Capabilities(r)
		if len(list) == 0 {
			return "", fmt.Errorf("角色 %q 没有任何能力，拒绝生成（能力表不完整）", r)
		}
		parts := make([]string, 0, len(list))
		for _, c := range list {
			parts = append(parts, "'"+string(c)+"'")
		}
		fmt.Fprintf(&table, "  %s: [%s],\n", r, strings.Join(parts, ", "))
	}
	return fmt.Sprintf(authzTSTemplate, roles.String(), caps.String(), table.String()), nil
}
