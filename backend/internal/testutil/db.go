// Package testutil 提供测试用内存数据库与工厂方法，避免每个测试重复搭建环境。
// 使用纯 Go 的 glebarez/sqlite 驱动，无需 CGO，适合在 Windows/Linux/macOS 本地与 CI 运行。
package testutil

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"forklift-training/internal/migrate"
	"forklift-training/internal/model"
)

// Now 返回当前时间，测试数据使用。
func Now() time.Time {
	return time.Now()
}

// applyCriticalUniqueIndexes 在 AutoMigrate 之后补跑 migrations 里的关键唯一索引 DDL（#1362）。
//
// 为什么必须在建库后补同句 DDL：偏唯一索引的 WHERE 谓词 GORM tag 表达不了，AutoMigrate 建不出
// 它们 ⇒ 「并发建号应被唯一索引兜底」这类用例过去靠「测试库没有约束」通过（真实缺陷 #14 的假绿）。
// DDL 的宿主与生产同一处（internal/migrate 的登记表，与 migrations 逐字相等到有锁），
// 这里不抄第二份；某条建不出来就直接 t.Fatalf（fail-closed：静默少一条约束正是本票要防的）。
func applyCriticalUniqueIndexes(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, idx := range migrate.CriticalUniqueIndexes() {
		if err := db.Exec(idx.DDL).Error; err != nil {
			t.Fatalf("测试库补建关键唯一索引 %s（表 %s）失败: %v\nDDL: %s", idx.Name, idx.Table, err, idx.DDL)
		}
	}
}

// NewMemoryDB 返回一个内存中的 sqlite 数据库，已 AutoMigrate 全部表 + 补建关键唯一索引。
// 每个测试用例应独立调用以获得隔离的数据库实例。
func NewMemoryDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	applyCriticalUniqueIndexes(t, db)
	return db
}

// NewFileDB 返回一个临时文件 SQLite 数据库（AutoMigrate 全部表 + 补建关键唯一索引）。
// 与 NewMemoryDB 的差异：:memory: 每连接独立库、无法多连接并发，文件库支持
// goroutine 并发读写（busy_timeout 串行化写冲突），供并发场景测试使用。
// 测试结束自动关闭连接池并随 TempDir 清理。
func NewFileDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db") + "?_pragma=busy_timeout(10000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开文件数据库失败: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	applyCriticalUniqueIndexes(t, db)
	return db
}

// seedUIDCounter 为测试用户生成递增 uid（雪花语义仅需唯一即可）。
var seedUIDCounter int64 = 1000000000000000000

// SeedStudent 插入一个测试学员，返回其 ID。
// username 作为昵称；account 自动派生以保证唯一性；uid 取自递增计数器。
func SeedStudent(t *testing.T, db *gorm.DB, username, hashedPassword string) *model.HrwaiUser {
	t.Helper()
	seedUIDCounter++
	s := &model.HrwaiUser{
		UID:       seedUIDCounter,
		Account:   "acct_" + username,
		Username:  username,
		Password:  hashedPassword,
		Phone:     "test_" + username,
		Status:    1,
		CreatedAt: Now(),
	}
	if err := db.Create(s).Error; err != nil {
		t.Fatalf("插入测试学员失败: %v", err)
	}
	return s
}

// SeedAdmin 插入一个测试管理员，返回其 ID。
//
// 自 #1618 段1 起，管理员的能力由「所挂角色」回答（静态表不再回答 admin），故这里必须同时建出
// 受保护（超级管理员）角色并挂上 —— 否则每个管理端契约测试都会因能力为空而 403。
// 生产侧同一件事由迁移 000041 的 seed 完成（测试面走 SQLite AutoMigrate，不跑迁移）。
func SeedAdmin(t *testing.T, db *gorm.DB, username, hashedPassword string) *model.Admin {
	t.Helper()
	roleID, err := migrate.EnsureProtectedAdminRole(db)
	if err != nil {
		t.Fatalf("确保受保护管理角色失败: %v", err)
	}
	a := &model.Admin{
		Username:  username,
		Password:  hashedPassword,
		Name:      username,
		RoleID:    &roleID,
		CreatedAt: Now(),
	}
	if err := db.Create(a).Error; err != nil {
		t.Fatalf("插入测试管理员失败: %v", err)
	}
	return a
}

// SeedTutor 插入一个测试导师。
func SeedTutor(t *testing.T, db *gorm.DB, username, hashedPassword string) *model.Tutor {
	t.Helper()
	tu := &model.Tutor{
		Username:  username,
		Password:  hashedPassword,
		Name:      username,
		Status:    1,
		CreatedAt: Now(),
	}
	if err := db.Create(tu).Error; err != nil {
		t.Fatalf("插入测试导师失败: %v", err)
	}
	return tu
}

// SeedQuestion 插入一道测试题目。
func SeedQuestion(t *testing.T, db *gorm.DB, qType, content, answer string) *model.Question {
	t.Helper()
	q := &model.Question{
		Type:          qType,
		Content:       content,
		Answer:        answer,
		Status:        "published",
		CreatedByType: "tutor",
		CreatedAt:     Now(),
		UpdatedAt:     Now(),
	}
	if err := db.Create(q).Error; err != nil {
		t.Fatalf("插入测试题目失败: %v", err)
	}
	return q
}

// SeedCourse 插入一门测试课程。
func SeedCourse(t *testing.T, db *gorm.DB, name string) *model.Course {
	t.Helper()
	c := &model.Course{
		Name:      name,
		Status:    1,
		CreatedAt: Now(),
	}
	if err := db.Create(c).Error; err != nil {
		t.Fatalf("插入测试课程失败: %v", err)
	}
	return c
}

// SeedRecruiter 插入一个测试企业招聘者（邀约制，独立表）。
func SeedRecruiter(t *testing.T, db *gorm.DB, username, hashedPassword string) *model.RecruiterUser {
	t.Helper()
	r := &model.RecruiterUser{
		Username:      username,
		Password:      hashedPassword,
		CompanyName:   "测试企业-" + username,
		CreditCode:    "91310000MA" + username,
		BusinessScope: "叉车租赁与维修",
		ContactName:   "联系人-" + username,
		ContactPhone:  "1380000" + "1234",
		ContactEmail:  username + "@example.com",
		Status:        1,
		CreatedAt:     Now(),
		UpdatedAt:     Now(),
	}
	if err := db.Create(r).Error; err != nil {
		t.Fatalf("插入测试招聘者失败: %v", err)
	}
	return r
}
