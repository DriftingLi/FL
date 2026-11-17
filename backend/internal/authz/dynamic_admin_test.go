package authz

import "testing"

// 本文件锁「管理端能力动态化」的形状（#1618 段1）：静态表与受保护角色能力全集必须互为补集，
// 且没有能力键在两侧都失联。这是「删掉 admin 行」这件事唯一可机检的判据。

// TestDynamicRolesOnlyAdmin 动态角色恰为 admin 与 tutor 两个（#2600 翻转后）：
// 多一个都意味着静态表漏答了某个角色的可达面，少一个意味着授权链没有真正上线。
func TestDynamicRolesOnlyAdmin(t *testing.T) {
	roles := DynamicRoles()
	want := []Role{RoleAdmin, RoleTutor}
	if len(roles) != len(want) || roles[0] != want[0] || roles[1] != want[1] {
		t.Fatalf("动态角色应为 %v，实际 = %v", want, roles)
	}
	if !IsDynamicRole(RoleAdmin) || !IsDynamicRole(RoleTutor) || IsDynamicRole(RoleStudent) || IsDynamicRole(RoleRecruiter) {
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

// TestEmptyRowsAreExactlyProtectedSet 空行 ⇔ 两张受保护清单的并集（单向：空行 ⊆ 并集）。
//
// #2600 起「受保护角色」有两个：超管（管理端能力面）与全权讲师（讲师端能力面）。
// 静态表里没有任何角色的键，必须落在这两张清单的并集里 —— 否则该键谁都拿不到（死键）。
//
// 两张清单**有意重叠**：question.author / contribution.review / catalog.author 是超管与
// 全权讲师**共有**的能力（题库作者与投稿审核两侧都要），重叠不是漂移而是事实。
func TestEmptyRowsAreExactlyProtectedSet(t *testing.T) {
	protected := map[Capability]bool{}
	for _, c := range ProtectedAdminCapabilities() {
		if protected[c] {
			t.Fatalf("受保护能力清单里 %q 重复", c)
		}
		protected[c] = true
	}
	for _, c := range ProtectedTutorCapabilities() {
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
			t.Fatalf("能力 %q 没有任何静态角色，也不在两张受保护角色能力全集里（该能力谁都拿不到）", c)
		}
	}
	// 反向不要求「受保护清单里的都必须是空行」——那是 #2600 之前的状态；翻转后讲师侧静态行
	// 也清空了，故本测试只做「空行 ⊆ 两张清单」这一个方向。
	//
	// #2595 追加一条：讲师侧新拆出的 course.author / tutorfile.manage **不得**进管理端受保护清单 ——
	// 它们不是「超管也持有的」能力，管理员今天打不进讲师面端点。
	adminProtected := map[Capability]bool{}
	for _, c := range ProtectedAdminCapabilities() {
		adminProtected[c] = true
	}
	for _, c := range []Capability{CapCourseAuthor, CapTutorFileManage} {
		if adminProtected[c] {
			t.Fatalf("能力 %q 只绑讲师，不应进受保护管理角色能力全集", c)
		}
	}
}

// TestEveryCapabilityReachable 每个能力键都至少有一条可达路径（静态行或两张受保护清单），
// 否则该键是死键：端点挂了守卫却永远 403。
func TestEveryCapabilityReachable(t *testing.T) {
	protected := map[Capability]bool{}
	for _, c := range ProtectedAdminCapabilities() {
		protected[c] = true
	}
	for _, c := range ProtectedTutorCapabilities() {
		protected[c] = true
	}
	for _, c := range AllCapabilities() {
		if len(RolesFor(c)) == 0 && !protected[c] {
			t.Fatalf("能力 %q 既无静态角色也不在受保护清单 —— 死键", c)
		}
	}
}

// TestHasFailsClosedForDynamicRole 静态判据对动态角色一律 false（fail closed）：
// admin / tutor 的实际可达面只能由数据层回答，静态表不得给出「默认放行」。
func TestHasFailsClosedForDynamicRole(t *testing.T) {
	for _, c := range AllCapabilities() {
		if Has(RoleAdmin, c) {
			t.Fatalf("静态表不应回答 admin 的能力，但 Has(admin, %q) = true", c)
		}
		if Has(RoleTutor, c) {
			t.Fatalf("静态表不应回答 tutor 的能力（#2600 翻转后），但 Has(tutor, %q) = true", c)
		}
	}
	if caps := Capabilities(RoleAdmin); len(caps) != 0 {
		t.Fatalf("Capabilities(admin) 应为空（由数据层回答），实际 %v", caps)
	}
	if caps := Capabilities(RoleTutor); len(caps) != 0 {
		t.Fatalf("Capabilities(tutor) 应为空（由数据层回答），实际 %v", caps)
	}
}
