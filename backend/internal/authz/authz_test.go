package authz

import "testing"

// 角色常量与能力表的形状锁：每个能力的角色集合要么非空且角色已知，要么为空。
//
// 空集不是「能力表不完整」而是**声明**（#1618 段1）：该能力的可达面由数据层回答（受保护角色持有，
// 见 protectedAdminCapabilities）。空行的完整性（空行 ⇔ 受保护清单）由 dynamic_admin_test.go 锁，
// 本测试只管形状：键非空、角色合法、且不得列动态角色。
func TestCapabilityTableShape(t *testing.T) {
	for _, c := range AllCapabilities() {
		if c == "" {
			t.Fatal("能力键不得为空")
		}
		for _, r := range RolesFor(c) {
			if !r.Valid() {
				t.Fatalf("能力 %q 含未知角色 %q", c, r)
			}
			if IsDynamicRole(r) {
				t.Fatalf("能力 %q 的静态行含动态角色 %q（其可达面应由数据层回答）", c, r)
			}
		}
	}
}

// Has：按「角色 → 能力」判定，未知角色与动态角色一律不通过（fail closed）。
func TestHas(t *testing.T) {
	cases := []struct {
		role Role
		cap  Capability
		want bool
	}{
		{RoleStudent, CapForumParticipate, true},
		{RoleStudent, CapForumModerate, false},
		// 管理端能力自 #1618 段1 起由数据层回答、讲师侧自 #2600 起同理：静态判据对二者一律
		// false（fail closed），实际可达面见 middleware.HasCapability 与两个 resolver。
		{RoleAdmin, CapForumModerate, false},
		{RoleAdmin, CapForumParticipate, false},
		{RoleTutor, CapContributionReview, false},
		{RoleTutor, CapTutorAccess, false},
		{RoleAdmin, CapContributionReview, false},
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
//
// 静态角色的集合由本测试锁；两个动态角色（admin / tutor）的静态集合**必须为空** ——
// 它们的可达面由数据层回答，前端生成物的对应行因此有意为空。
func TestCapabilitiesStableAndComplete(t *testing.T) {
	// 学员与招聘者是**静态角色**，他们的集合仍由本表回答（翻转只动讲师与管理端）。
	got := Capabilities(RoleStudent)
	if len(got) == 0 {
		t.Fatal("学员能力集合不得为空")
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("能力集合必须按字典序稳定输出: %v", got)
		}
	}
	if !contains(got, CapCourseLearn) {
		t.Fatalf("学员应拥有 course.learn: %v", got)
	}
	if contains(got, CapForumModerate) {
		t.Fatalf("学员不应拥有管理端 forum.moderate: %v", got)
	}
	rec := Capabilities(RoleRecruiter)
	if !contains(rec, CapRecruitAccess) || contains(rec, CapStudentAccess) {
		t.Fatalf("招聘者能力集合异常: %v", rec)
	}
	// #2600：讲师成为动态角色 ⇒ 静态集合必须为空（能力由 tutor_role 回答）。
	// 讲师侧那 7 键的可达性由 ProtectedTutorCapabilities() 与 resolver 保证，
	// 不再由本表回答 —— 这正是「授权链上线」这件事在能力表上的表现。
	if caps := Capabilities(RoleTutor); len(caps) != 0 {
		t.Fatalf("动态角色 tutor 的静态能力集合必须为空（由数据层回答），实际 %v", caps)
	}
	if len(Capabilities(RoleAdmin)) != 0 {
		t.Fatal("动态角色 admin 的静态能力集合必须为空（由数据层回答）")
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
