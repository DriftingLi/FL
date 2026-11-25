package admin

import (
	"errors"
	"slices"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestNormalizeCapabilityKeys(t *testing.T) {
	t.Parallel()
	allowed := []authz.Capability{
		authz.CapTutorAccess,
		authz.CapQuestionAuthor,
		authz.CapNotificationUse,
	}

	got, err := normalizeCapabilityKeys([]string{
		" tutor.access ", "question.author", "", "notification.use", "question.author", "\t",
	}, allowed)
	if err != nil {
		t.Fatalf("能力键归一失败: %v", err)
	}
	want := []string{"notification.use", "question.author", "tutor.access"}
	if !slices.Equal(got, want) {
		t.Fatalf("能力键应 trim、去重并排序，got %v want %v", got, want)
	}

	got, err = normalizeCapabilityKeys(nil, allowed)
	if err != nil {
		t.Fatalf("空能力集归一失败: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("空能力集应返回非 nil 空切片，got %#v", got)
	}

	got, err = normalizeCapabilityKeys([]string{"tutor.access", "unknown.cap"}, allowed)
	if got != nil || !errors.Is(err, ErrUnknownCapability) {
		t.Fatalf("未知能力键应返回 nil 与 ErrUnknownCapability，got %v err=%v", got, err)
	}
}

func TestRoleCapabilityNormalizationKeepsAllowedSets(t *testing.T) {
	t.Parallel()

	adminCaps, err := normalizeCapabilities([]string{
		"student.access", " admin_role.manage ", "student.access",
	})
	if err != nil {
		t.Fatalf("管理角色应接受 authz 全能力表中的键: %v", err)
	}
	adminWant := []string{"admin_role.manage", "student.access"}
	if !slices.Equal(adminCaps, adminWant) {
		t.Fatalf("管理角色能力集不对，got %v want %v", adminCaps, adminWant)
	}

	tutorCaps, err := normalizeTutorCapabilities([]string{
		" tutorfile.manage ", "tutor.access", "catalog.author", "question.author",
		"notification.use", "course.author", "contribution.review", "tutor.access",
	})
	if err != nil {
		t.Fatalf("讲师角色能力集归一失败: %v", err)
	}
	tutorWant := []string{
		"catalog.author", "contribution.review", "course.author", "notification.use",
		"question.author", "tutor.access", "tutorfile.manage",
	}
	if !slices.Equal(tutorCaps, tutorWant) {
		t.Fatalf("讲师角色能力集不对，got %v want %v", tutorCaps, tutorWant)
	}

	for _, forbidden := range []string{"admin_role.manage", "student.access"} {
		got, err := normalizeTutorCapabilities([]string{forbidden})
		if got != nil || !errors.Is(err, ErrUnknownCapability) {
			t.Fatalf("讲师角色应拒绝 %q，got %v err=%v", forbidden, got, err)
		}
	}
	if got, err := normalizeCapabilities([]string{"unknown.cap"}); got != nil || !errors.Is(err, ErrUnknownCapability) {
		t.Fatalf("管理角色应拒绝未知能力键，got %v err=%v", got, err)
	}
}

func TestRoleUpdatesKeepNameAndRemarkSemantics(t *testing.T) {
	t.Parallel()
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())

	adminRole, err := svc.CreateAdminRole("  管理员角色  ", "  管理员备注  ", []string{" audit.read "})
	if err != nil {
		t.Fatalf("新建管理角色失败: %v", err)
	}
	if adminRole.Name != "管理员角色" || adminRole.Remark != "管理员备注" {
		t.Fatalf("管理角色名称与备注应 trim，got %+v", adminRole)
	}
	if _, err := svc.UpdateAdminRole(adminRole.RoleID, " \t ", "不应落库", nil); err == nil {
		t.Fatal("管理角色更新空名称应被拒绝")
	}
	var storedAdmin model.AdminRole
	if err := db.First(&storedAdmin, adminRole.RoleID).Error; err != nil {
		t.Fatalf("读取管理角色失败: %v", err)
	}
	if storedAdmin.Name != "管理员角色" || storedAdmin.Remark != "管理员备注" {
		t.Fatalf("被拒绝的更新不应改动管理角色，got %+v", storedAdmin)
	}
	updatedAdmin, err := svc.UpdateAdminRole(adminRole.RoleID, "  新管理角色  ", "  新管理备注  ", nil)
	if err != nil {
		t.Fatalf("更新管理角色失败: %v", err)
	}
	if updatedAdmin.Name != "新管理角色" || updatedAdmin.Remark != "新管理备注" {
		t.Fatalf("管理角色更新后的名称与备注应 trim，got %+v", updatedAdmin)
	}

	tutorRole, err := svc.CreateTutorRole("  讲师角色  ", "  讲师备注  ", []string{" tutor.access "})
	if err != nil {
		t.Fatalf("新建讲师角色失败: %v", err)
	}
	if tutorRole.Name != "讲师角色" || tutorRole.Remark != "  讲师备注  " {
		t.Fatalf("讲师角色名称应 trim、备注应原样保留，got %+v", tutorRole)
	}
	updatedTutor, err := svc.UpdateTutorRole(tutorRole.RoleID, " \t ", "  新讲师备注  ", nil)
	if err != nil {
		t.Fatalf("讲师角色空名称更新应回退旧名: %v", err)
	}
	if updatedTutor.Name != "讲师角色" || updatedTutor.Remark != "  新讲师备注  " {
		t.Fatalf("讲师角色更新应保留回退旧名与原样备注，got %+v", updatedTutor)
	}
	if updatedTutor.Capabilities == nil || len(updatedTutor.Capabilities) != 0 {
		t.Fatalf("讲师角色清空能力后应返回非 nil 空集，got %#v", updatedTutor.Capabilities)
	}
}

func TestReplaceTutorRoleCapabilities(t *testing.T) {
	t.Parallel()
	db := testutil.NewMemoryDB(t)
	role := model.TutorRole{Name: "批量能力角色"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("新建讲师角色失败: %v", err)
	}
	oldRows := []model.TutorRoleCapability{
		{RoleID: role.RoleID, Capability: "tutor.access"},
		{RoleID: role.RoleID, Capability: "notification.use"},
	}
	if err := db.Create(&oldRows).Error; err != nil {
		t.Fatalf("播种旧能力行失败: %v", err)
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		return replaceTutorRoleCapabilities(tx, role.RoleID, []string{
			"question.author", "tutor.access", "notification.use",
		})
	})
	if err != nil {
		t.Fatalf("批量替换讲师角色能力失败: %v", err)
	}
	var rows []model.TutorRoleCapability
	if err := db.Where("role_id = ?", role.RoleID).Order("capability").Find(&rows).Error; err != nil {
		t.Fatalf("读取批量写入结果失败: %v", err)
	}
	got := make([]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.Capability)
	}
	want := []string{"notification.use", "question.author", "tutor.access"}
	if !slices.Equal(got, want) {
		t.Fatalf("批量替换后的能力行不对，got %v want %v", got, want)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return replaceTutorRoleCapabilities(tx, role.RoleID, nil)
	}); err != nil {
		t.Fatalf("清空讲师角色能力失败: %v", err)
	}
	var count int64
	if err := db.Model(&model.TutorRoleCapability{}).Where("role_id = ?", role.RoleID).Count(&count).Error; err != nil {
		t.Fatalf("核对清空结果失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("空能力集应删除全部旧行，仍有 %d 行", count)
	}

	original := model.TutorRoleCapability{RoleID: role.RoleID, Capability: "tutor.access"}
	if err := db.Create(&original).Error; err != nil {
		t.Fatalf("播种回滚基线失败: %v", err)
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		return replaceTutorRoleCapabilities(tx, role.RoleID, []string{"question.author", "question.author"})
	})
	if err == nil {
		t.Fatal("批量写入重复能力行应触发唯一约束失败")
	}
	rows = nil
	if err := db.Where("role_id = ?", role.RoleID).Find(&rows).Error; err != nil {
		t.Fatalf("读取回滚结果失败: %v", err)
	}
	if len(rows) != 1 || rows[0].Capability != original.Capability {
		t.Fatalf("批量写入失败应回滚删除，保留旧能力行，got %+v", rows)
	}
}
