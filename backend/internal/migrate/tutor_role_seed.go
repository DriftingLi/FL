package migrate

import (
	"errors"

	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/model"
)

// 本文件：讲师角色链在**测试侧**的那一半（#2599 / 规格 E1）。
//
// 生产库由迁移 000061 建表 + seed 四模板 + 存量讲师挂全权角色；而测试面走 SQLite
// AutoMigrate（不跑迁移）—— 两侧表达的是同一件事，测试装配要补建。本包正是
// 「迁移在测试侧的那一半」的既有宿主（先例：CriticalUniqueIndexes、EnsureProtectedAdminRole）。
//
// 与迁移 SQL 的重复是**刻意**的：SQL 那份是生产事实，这份是测试事实，两者由 PG 契约测试
// （backend/internal/api/tutor_role_contract_test.go）逐键对账 —— 只有一边改了，对账就红。

// EnsureProtectedTutorRole 幂等确保「受保护（全权讲师）角色」存在，返回其 role_id。
//
// 幂等判据是 protected = TRUE 而不是角色名：受保护角色在系统里有且仅有一个（它是「讲师端能力
// 全集」的载体）。角色名字面量只有 authz.TutorRoleNameFull 一个来源。
func EnsureProtectedTutorRole(db *gorm.DB) (int, error) {
	var role model.TutorRole
	err := db.Where("protected = ?", true).First(&role).Error
	if err == nil {
		return role.RoleID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	role = model.TutorRole{
		Name:      authz.TutorRoleNameFull,
		Protected: true,
		Remark:    "受保护角色：拥有全部讲师能力，不可编辑/删除",
	}
	if err := db.Create(&role).Error; err != nil {
		return 0, err
	}
	return role.RoleID, nil
}

// EnsureTutorTemplateRoles 幂等确保四个预置讲师角色及其能力行存在（对应迁移 000061 的 seed 段）。
//
// 语义与迁移逐条相同：
//   - protected 角色（全权讲师）**不落能力行**（能力由 authz.ProtectedTutorCapabilities 回答）；
//   - 三个子角色的能力行按 authz.TutorTemplateRoles() 落库；
//   - 角色名已存在时不动它（迁移里是 ON CONFLICT (name) DO NOTHING）。
func EnsureTutorTemplateRoles(db *gorm.DB) error {
	for _, tpl := range authz.TutorTemplateRoles() {
		var role model.TutorRole
		err := db.Where("name = ?", tpl.Name).First(&role).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			role = model.TutorRole{Name: tpl.Name, Protected: tpl.Protected}
			if tpl.Protected {
				role.Remark = "受保护角色：拥有全部讲师能力，不可编辑/删除"
			}
			if err := db.Create(&role).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if tpl.Protected {
			continue
		}
		for _, cap := range tpl.Capabilities {
			row := model.TutorRoleCapability{RoleID: role.RoleID, Capability: string(cap)}
			// 幂等：已存在即跳过（迁移侧是 ON CONFLICT DO NOTHING）。
			var count int64
			if err := db.Model(&model.TutorRoleCapability{}).
				Where("role_id = ? AND capability = ?", row.RoleID, row.Capability).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			if err := db.Create(&row).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsureTutorRoleChain 幂等建起整条讲师角色链（四个模板 + 受保护角色存在性），供测试装配一行调用。
func EnsureTutorRoleChain(db *gorm.DB) error {
	if _, err := EnsureProtectedTutorRole(db); err != nil {
		return err
	}
	return EnsureTutorTemplateRoles(db)
}
