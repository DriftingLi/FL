// Package testutil 提供测试用内存数据库与工厂方法，避免每个测试重复搭建环境。
// 使用纯 Go 的 glebarez/sqlite 驱动，无需 CGO，适合在 Windows/Linux/macOS 本地与 CI 运行。
package testutil

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"forklift-training/internal/model"
)

// Now 返回当前时间，测试数据使用。
func Now() time.Time {
	return time.Now()
}

// NewMemoryDB 返回一个内存中的 sqlite 数据库，已 AutoMigrate 全部 19 张表。
// 每个测试用例应独立调用以获得隔离的数据库实例。
func NewMemoryDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(allModels()...); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	return db
}

// allModels 返回全部模型，按外键依赖顺序排列。
func allModels() []interface{} {
	return []interface{}{
		&model.Student{},
		&model.Admin{},
		&model.Tutor{},
		&model.Course{},
		&model.KnowledgePoint{},
		&model.Chapter{},
		&model.ChapterFile{},
		&model.StudyRecord{},
		&model.ExamSession{},
		&model.Question{},
		&model.ExamParticipant{},
		&model.ExamAnswer{},
		&model.ExamRecord{},
		&model.QuestionPracticeRecord{},
		&model.PracticeRecord{},
		&model.WrongQuestion{},
		&model.MockExam{},
		&model.AIGenerationLog{},
		&model.AsyncTask{},
	}
}

// SeedStudent 插入一个测试学员，返回其 ID。
func SeedStudent(t *testing.T, db *gorm.DB, username, hashedPassword string) *model.Student {
	t.Helper()
	s := &model.Student{
		Username:  username,
		Password:  hashedPassword,
		Name:      username,
		Status:    1,
		Level:     "beginner",
		CreatedAt: Now(),
	}
	if err := db.Create(s).Error; err != nil {
		t.Fatalf("插入测试学员失败: %v", err)
	}
	return s
}

// SeedAdmin 插入一个测试管理员，返回其 ID。
func SeedAdmin(t *testing.T, db *gorm.DB, username, hashedPassword string) *model.Admin {
	t.Helper()
	a := &model.Admin{
		Username:  username,
		Password:  hashedPassword,
		Name:      username,
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
func SeedQuestion(t *testing.T, db *gorm.DB, qType, level, content, answer string) *model.Question {
	t.Helper()
	q := &model.Question{
		Type:         qType,
		Level:        level,
		Content:      content,
		Answer:       answer,
		Status:       "published",
		CreatedByType: "tutor",
		CreatedAt:    Now(),
		UpdatedAt:    Now(),
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
		Category:  "general",
		Status:    1,
		CreatedAt: Now(),
	}
	if err := db.Create(c).Error; err != nil {
		t.Fatalf("插入测试课程失败: %v", err)
	}
	return c
}

// SeedKnowledgePoint 插入一个测试知识点。
func SeedKnowledgePoint(t *testing.T, db *gorm.DB, name, level string) *model.KnowledgePoint {
	t.Helper()
	kp := &model.KnowledgePoint{
		Name:      name,
		Level:     level,
		CreatedAt: Now(),
	}
	if err := db.Create(kp).Error; err != nil {
		t.Fatalf("插入测试知识点失败: %v", err)
	}
	return kp
}
