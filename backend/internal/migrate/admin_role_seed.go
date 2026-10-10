package migrate

import (
	"errors"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// EnsureProtectedAdminRole 幂等确保「受保护（超级管理员）角色」存在，返回其 role_id。
//
// 为什么它在 migrate 包（#1618 段1）：生产库由**迁移 000041 的 seed**建出这一行，而测试面走
// SQLite AutoMigrate（不跑迁移）—— 两侧表达的是同一件事，测试装配要补建。本包正是「迁移在
// 测试侧的那一半」的既有宿主（先例：CriticalUniqueIndexes 登记表），且测试基建（testutil）
// 已经依赖它 —— 放进 internal/core 会让 core 的测试 import testutil 而直接成环（实测）。
//
// 幂等判据是 protected = TRUE 而不是角色名：受保护角色在系统里有且仅有一个（它是「管理端能力
// 全集」的载体，多一个没有语义）。角色名的字面量只有 model.ProtectedAdminRoleName 一个来源，
// 迁移侧由 model 包的锁测试逐字对账。
func EnsureProtectedAdminRole(db *gorm.DB) (int, error) {
	var role model.AdminRole
	err := db.Where("protected = ?", true).First(&role).Error
	if err == nil {
		return role.RoleID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	role = model.AdminRole{
		Name:      model.ProtectedAdminRoleName,
		Protected: true,
		Remark:    "受保护角色：拥有全部管理能力，不可编辑/删除",
	}
	if err := db.Create(&role).Error; err != nil {
		return 0, err
	}
	return role.RoleID, nil
}
