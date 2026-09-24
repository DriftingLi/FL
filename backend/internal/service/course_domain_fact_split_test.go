// 第③批（课程/内容/精选/讲师/学员）的**事实→错误**档位锁（ADR-0064 决策 1/2/8）。
//
// 形状与第②批 B 段一致：每档断言两件事 ——
//
//	正向：行不在 ⇒ 抛出**该对象那一个**具名哨兵；
//	反向：删表（查不动）⇒ 不得抛该哨兵（必须是如实上抛的真实错误）。
//
// 反向那一半才是本波真正的新增保护：此前这些读路径的 `if err != nil { return Err…NotFound }`
// 把两件事写成一件事，光看正向永远发现不了。
//
// 建在 service 层而非 HTTP 码：这批端点分属管理端 / 讲师端 / 学员端三套能力位，HTTP 档要造
// 「角色×能力×证件×种子」的笛卡尔积；api 侧那 17 处 WithSentinel 只是把这里的错误身份映射成码，
// 映射机制本身已由第①批 HTTP 台账与 errstatus 测试覆盖。
package service

import (
	"errors"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

type readabilityRow struct {
	name    string
	sent    error
	table   string
	missing func(t *testing.T, db *gorm.DB) error // 取一个必然不存在的对象
	downed  func(t *testing.T, db *gorm.DB) error // 删表后再取一个「看起来存在」的 id
}

func TestCourseDomainFactSplit(t *testing.T) {
	db0 := testutil.NewMemoryDB(t)
	spec := model.Specialty{Code: "split", Name: "拆分", SortOrder: 1, Status: 1}
	lv := model.CourseLevel{Code: "split-lv", Name: "级", SortOrder: 1, Status: 1}
	if err := db0.Create(&spec).Error; err != nil {
		t.Fatalf("建方向失败: %v", err)
	}
	if err := db0.Create(&lv).Error; err != nil {
		t.Fatalf("建等级失败: %v", err)
	}
	crs := model.Course{Name: "载体课", Status: 1, SpecialtyID: &spec.SpecialtyID,
		LevelID: &lv.LevelID, CreatedAt: testutil.Now()}
	if err := db0.Create(&crs).Error; err != nil {
		t.Fatalf("建课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: crs.CourseID, Title: "章", OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db0.Create(&ch).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}

	rows := []readabilityRow{
		{"精选内容", ErrFeaturedContentNotFound, "featured_content",
			func(t *testing.T, db *gorm.DB) error {
				_, e := NewFeaturedService(db, nil, zap.NewNop()).AdminDetail(999999)
				return e
			},
			func(t *testing.T, db *gorm.DB) error {
				_, e := NewFeaturedService(db, nil, zap.NewNop()).AdminDetail(1)
				return e
			}},
		{"课程", ErrCourseNotFound, "course",
			func(t *testing.T, db *gorm.DB) error {
				_, e := NewAdminCourseService(db, nil, zap.NewNop()).UpdateCourse(999999, &CourseInput{})
				return e
			},
			func(t *testing.T, db *gorm.DB) error {
				_, e := NewAdminCourseService(db, nil, zap.NewNop()).UpdateCourse(1, &CourseInput{})
				return e
			}},
		{"章节", ErrChapterNotFound, "chapter",
			func(t *testing.T, db *gorm.DB) error {
				_, e := NewAdminCourseService(db, nil, zap.NewNop()).UpdateChapter(999999, &ChapterInput{})
				return e
			},
			func(t *testing.T, db *gorm.DB) error {
				_, e := NewAdminCourseService(db, nil, zap.NewNop()).UpdateChapter(1, &ChapterInput{})
				return e
			}},
	}

	for _, r := range rows {
		t.Run(r.name+" 真不存在", func(t *testing.T) {
			db := testutil.NewMemoryDB(t)
			if err := r.missing(t, db); !errors.Is(err, r.sent) {
				t.Fatalf("%s 不在时应报 %v，实际 %v", r.name, r.sent, err)
			}
		})
		t.Run(r.name+" 查不动不得冒充不存在", func(t *testing.T) {
			db := testutil.NewMemoryDB(t)
			dropOnly(t, db, r.table)
			err := r.downed(t, db)
			if err == nil {
				t.Fatalf("%s：表都读不到却返回成功", r.name)
			}
			if errors.Is(err, r.sent) {
				t.Fatalf("%s：查不动被读成「%s 不存在」（ADR-0064 决策 1 要消灭的那一型）", r.name, r.name)
			}
		})
	}
}

// dropOnly 只删表：「查不动」这一档不需要先播种 —— 表不在，取任何 id 都会失败。
func dropOnly(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	if err := db.Exec("DROP TABLE " + table).Error; err != nil {
		t.Fatalf("删 %s 表失败: %v", table, err)
	}
}
