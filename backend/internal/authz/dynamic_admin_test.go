package authz

import "testing"

// 本文件锁「管理端能力动态化」的形状（#1618 段1）：静态表与受保护角色能力全集必须互为补集，
// 且没有能力键在两侧都失联。这是「删掉 admin 行」这件事唯一可机检的判据。

// TestDynamicRolesOnlyAdmin 动态角色只有 admin 一个：多一个都意味着静态表漏答了某个角色的可达面。
func TestDynamicRolesOnlyAdmin(t *testing.T) {
	roles := DynamicRoles()
	if len(roles) != 1 || roles[0] != RoleAdmin {
		t.Fatalf("动态角色应只有 admin，实际 = %v", roles)
	}
	if !IsDynamicRole(RoleAdmin) || IsDynamicRole(RoleTutor) || IsDynamicRole(RoleStudent) || IsDynamicRole(RoleRecruiter) {
		t.Fatalf("IsDynamicRole 判定与 DynamicRoles 不一致")
	}
}

// TestNoStaticRowListsDynamicRole 静态表任何一行都不得再列动态角色 ——
// 列了就等于「静态表也回答 admin」，与动态化自相矛盾（且会让前端生成物重新出现 admin 行）。
func TestNoStaticRowListsDynamicRole(t *testing.T) {
	for _, c := range AllCapabilities() {
		for _, r := range RolesFor(c) {
			if IsDynamicRole(r) {
				t.Fatalf("能力 %q 的静态行仍列了动态角色 %q", c, r)
			}
		}
	}
}

// TestEmptyRowsAreExactlyProtectedSet 空行 ⇔ 受保护角色能力全集（双向）。
func TestEmptyRowsAreExactlyProtectedSet(t *testing.T) {
	protected := map[Capability]bool{}
	for _, c := range ProtectedAdminCapabilities() {
		if protected[c] {
			t.Fatalf("受保护能力清单里 %q 重复", c)
		}
		protected[c] = true
	}
	empty := map[Capability]bool{}
	for _, c := range AllCapabilities() {
		if len(RolesFor(c)) == 0 {
			empty[c] = true
		}
	}
	for c := range empty {
		if !protected[c] {
			t.Fatalf("能力 %q 没有任何静态角色，却不在受保护角色能力全集里（该能力谁都拿不到）", c)
		}
	}
	// 反向不要求「受保护清单里的都必须是空行」：contribution.review / catalog.author / question.author
	// 是讲师与超管**共有**的能力，它们的静态行（tutor）必须保留 —— 讲师侧不能因为管理端动态化而失权。
	// 受保护清单里的每一项都不得含动态角色，由 TestNoStaticRowListsDynamicRole 覆盖。
}

// TestEveryCapabilityReachable 每个能力键都至少有一条可达路径（静态行或受保护角色），
// 否则该键是死键：端点挂了守卫却永远 403。
func TestEveryCapabilityReachable(t *testing.T) {
	protected := map[Capability]bool{}
	for _, c := range ProtectedAdminCapabilities() {
		protected[c] = true
	}
	for _, c := range AllCapabilities() {
		if len(RolesFor(c)) == 0 && !protected[c] {
			t.Fatalf("能力 %q 既无静态角色也不在受保护清单 —— 死键", c)
		}
	}
}

// TestHasFailsClosedForDynamicRole 静态判据对动态角色一律 false（fail closed）：
// admin 的实际可达面只能由数据层回答，静态表不得给出「默认放行」。
func TestHasFailsClosedForDynamicRole(t *testing.T) {
	for _, c := range AllCapabilities() {
		if Has(RoleAdmin, c) {
			t.Fatalf("静态表不应回答 admin 的能力，但 Has(admin, %q) = true", c)
		}
	}
	if caps := Capabilities(RoleAdmin); len(caps) != 0 {
		t.Fatalf("Capabilities(admin) 应为空（由数据层回答），实际 %v", caps)
	}
}
