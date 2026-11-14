// 本文件：收件人角色映射与 core 身份词表的一致性判据。
//
// 为什么单独一个**外部**测试包（package notification_test）：service.go 里的
// RecipientForRole 用字面量 "hrwai_user"/"tutor" 而非 core.HrwaiRole/core.TutorRole，
// 因为 core → aiassistant → points → notification 已间接依赖 notification 包，反向
// import 会成环。字面量一旦与 core 漂移，生产上学员/讲师会静默拿 403——所以必须有一条
// 真能读 core 常量的判据。包内测试做不到（会成环），外部测试包可以。
package notification_test

import (
	"errors"
	"testing"

	"forklift-training/internal/core"
	"forklift-training/internal/notification"
)

// TestRecipientForRoleMatchesCoreVocab 角色字面量与 core 的身份常量真的一致。
// 这不是同义反复：断言的两侧分别是「生产 switch 里写的字面量」与「core 包导出的常量」，
// core 改名会让本用例判红（而包内那份用测试内自造字面量的写法会照绿）。
func TestRecipientForRoleMatchesCoreVocab(t *testing.T) {
	// 学员角色：core.HrwaiRole 必须被接受且分派到学员列。
	r, err := notification.RecipientForRole(core.HrwaiRole, 7)
	if err != nil {
		t.Fatalf("core.HrwaiRole=%q 应被接受（生产字面量与 core 漂移？）: %v", core.HrwaiRole, err)
	}
	if r.Role != notification.RecipientStudent {
		t.Fatalf("core.HrwaiRole 应分派到学员收件人, got %q", r.Role)
	}

	// 讲师角色：core.TutorRole 必须被接受且分派到讲师列。
	r, err = notification.RecipientForRole(core.TutorRole, 7)
	if err != nil {
		t.Fatalf("core.TutorRole=%q 应被接受（生产字面量与 core 漂移？）: %v", core.TutorRole, err)
	}
	if r.Role != notification.RecipientTutor {
		t.Fatalf("core.TutorRole 应分派到讲师收件人, got %q", r.Role)
	}

	// 其余身份（管理员、招聘者）没有信箱——ADR-0095 决策三。
	for _, role := range []string{core.AdminRole, core.RecruiterRole} {
		if _, err := notification.RecipientForRole(role, 1); !errors.Is(err, notification.ErrNoMailbox) {
			t.Fatalf("角色 %q 应无信箱（ErrNoMailbox）, got err=%v", role, err)
		}
	}
}
