package authz

import "testing"

// 角色常量与能力表的形状锁：能力命名必须是「资源域.动作」，且每个能力的角色集合非空、
// 角色取值必须是已知角色。
func TestCapabilityTableShape(t *testing.T) {
	for _, c := range AllCapabilities() {
		if c == "" {
			t.Fatal("能力键不得为空")
		}
		roles := RolesFor(c)
		if len(roles) == 0 {
			t.Fatalf("能力 %q 没有任何角色，能力表不完整", c)
		}
		for _, r := range roles {
			if !r.Valid() {
				t.Fatalf("能力 %q 含未知角色 %q", c, r)
			}
		}
	}
}

// Has：按「角色 → 能力」判定，未知角色一律不通过（fail closed）。
func TestHas(t *testing.T) {
	cases := []struct {
		role Role
		cap  Capability
		want bool
	}{
		{RoleStudent, CapForumParticipate, true},
		{RoleStudent, CapForumModerate, false},
		{RoleAdmin, CapForumModerate, true},
		{RoleAdmin, CapForumParticipate, false},
		{RoleTutor, CapContributionReview, true},
		{RoleAdmin, CapContributionReview, true},
		{RoleStudent, CapContributionReview, false},
		{RoleRecruiter, CapRecruitAccess, true},
		{RoleRecruiter, CapStudentAccess, false},
		{RoleStudent, CapValuationUse, true},
		{RoleStudent, CapValuationConfig, false},
		{Role(""), CapStudentAccess, false},
		{Role("nobody"), CapStudentAccess, false},
	}
	for _, c := range cases {
		if got := Has(c.role, c.cap); got != c.want {
			t.Fatalf("Has(%q, %q) = %v, want %v", c.role, c.cap, got, c.want)
		}
	}
}

// Capabilities：返回该角色的全部能力，顺序稳定（用于 codegen 与断言）。
func TestCapabilitiesStableAndComplete(t *testing.T) {
	got := Capabilities(RoleAdmin)
	if len(got) == 0 {
		t.Fatal("管理员能力集合不得为空")
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("能力集合必须按字典序稳定输出: %v", got)
		}
	}
	if !contains(got, CapForumModerate) {
		t.Fatalf("管理员应拥有 forum.moderate: %v", got)
	}
	if contains(got, CapForumParticipate) {
		t.Fatalf("管理员不应拥有学员侧 forum.participate（保持现状口径）: %v", got)
	}
	if len(Capabilities(Role(""))) != 0 {
		t.Fatal("未知角色的能力集合必须为空（fail closed）")
	}
}

func contains(list []Capability, want Capability) bool {
	for _, c := range list {
		if c == want {
			return true
		}
	}
	return false
}
