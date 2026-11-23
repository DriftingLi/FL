// 课程主类型的迁移与不变式契约测试（#2760）。
//
// 两条判据分开测，因为它们坏掉的方式完全不同：
//   - 迁移预填（本文件 TestCourseTypeMigration*）：一次性数据动作，坏了只在真实库上出现
//     （内存 SQLite 跑不了 UPDATE ... FROM 与 DO $$ 块），故这条走真实 PostgreSQL。
//   - 类型不变式（course_type_invariant_test.go）：每次写入都成立的业务规则，坏了在任何
//     库上都出现，故那条走内存库。
package course

import (
	"os"
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestCourseTypeMigrationBackfillsFromPublishedVideo 迁移 74 的预填判据：
// 任一**已发布**视频章节 ⇒ video；其余 ⇒ illustrated。
//
// 逐类建课是刻意的：这五类正是票里「课程迁移覆盖纯视频、纯图文或 PPT、混合、无已发布章节
// 和仅有待审视频」逐条对应的情形，合成一个用例就只能测到最松的那条。
func TestCourseTypeMigrationBackfillsFromPublishedVideo(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	migrationSQL := func(migration, direction string) string {
		t.Helper()
		path := filepath.Join("testdata", migration+"."+direction+".sql")
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(sql)
	}

	apply := func(migration, direction string) {
		t.Helper()
		if err := db.Exec(migrationSQL(migration, direction)).Error; err != nil {
			t.Fatalf("migration %s %s: %v", migration, direction, err)
		}
	}

	// 先回滚，再从零重建：预填判据只有在列尚不存在时才有意义。
	apply("000074_course_type", "down")

	categoryID := insertReturningID(t, db,
		`INSERT INTO training_category (name, sort_order, status) VALUES ('类型迁移分类', 900, 1) RETURNING id`)
	specialtyID := insertReturningID(t, db,
		`INSERT INTO specialty (code, name, sort_order, status) VALUES ('ctype_mig', '类型迁移方向', 900, 1) RETURNING specialty_id`)
	levelID := insertReturningID(t, db,
		`INSERT INTO course_level (code, name, sort_order, status) VALUES ('ctype_mig', '类型迁移等级', 900, 1) RETURNING level_id`)

	newCourse := func(name string) int {
		t.Helper()
		return insertReturningID(t, db, `INSERT INTO course (name, category_id, specialty_id, level_id, status)
			VALUES (?, ?, ?, ?, 1) RETURNING course_id`, name, categoryID, specialtyID, levelID)
	}
	// addChapter 造一章 + 其内容槽。slot 为空表示不建槽（无已发布内容）。
	addChapter := func(courseID int, workingType, slot, slotType string) int {
		t.Helper()
		chapterID := insertReturningID(t, db, `INSERT INTO chapter (course_id, title, content, content_type, order_num)
			VALUES (?, ?, 'body', ?, 1) RETURNING chapter_id`, courseID, "章-"+workingType, workingType)
		if slot != "" {
			if err := db.Exec(`INSERT INTO chapter_content_slot (slot, chapter_id, content, content_type, file_url, submitted_by, frozen_at)
				VALUES (?, ?, 'body', ?, '', 1, now())`, slot, chapterID, slotType).Error; err != nil {
				t.Fatal(err)
			}
		}
		return chapterID
	}

	// ① 纯视频：全部已发布章节都是 video
	pureVideo := newCourse("纯视频课")
	addChapter(pureVideo, "video", model.ChapterReviewPublished, "video")
	addChapter(pureVideo, "video", model.ChapterReviewPublished, "video")

	// ② 纯图文
	pureText := newCourse("纯图文课")
	addChapter(pureText, "text", model.ChapterReviewPublished, "text")

	// ③ 纯 PPT（illustrated 一侧：PPT 归图文）
	purePPT := newCourse("纯PPT课")
	addChapter(purePPT, "ppt", model.ChapterReviewPublished, "ppt")

	// ④ 混合：一章视频 + 一章图文（→ video，但进复核清单）
	mixed := newCourse("混合课")
	addChapter(mixed, "video", model.ChapterReviewPublished, "video")
	addChapter(mixed, "text", model.ChapterReviewPublished, "text")

	// ⑤ 无已发布章节：只有 working 草稿
	noPublished := newCourse("无已发布章节课")
	addChapter(noPublished, "video", "", "")

	// ⑥ 仅有待审视频：在审槽里的视频**不算**公开内容
	pendingVideoOnly := newCourse("仅有待审视频课")
	ch := addChapter(pendingVideoOnly, "video", "", "")
	if err := db.Exec(`INSERT INTO chapter_content_slot (slot, chapter_id, content, content_type, file_url, submitted_by, frozen_at)
		VALUES (?, ?, 'body', 'video', '', 1, now())`, model.ChapterReviewPending, ch).Error; err != nil {
		t.Fatal(err)
	}

	apply("000074_course_type", "up")

	readType := func(courseID int) string {
		t.Helper()
		var got string
		if err := db.Raw("SELECT course_type FROM course WHERE course_id = ?", courseID).Row().Scan(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	for _, tc := range []struct {
		name     string
		courseID int
		want     string
	}{
		{"纯视频", pureVideo, model.CourseTypeVideo},
		{"纯图文", pureText, model.CourseTypeIllustrated},
		{"纯PPT", purePPT, model.CourseTypeIllustrated},
		{"混合（任一视频⇒video）", mixed, model.CourseTypeVideo},
		// 无已发布章节落 illustrated：没有公开内容可依据，默认侧是「非视频」——
		// 猜成 video 会让一门可能全是图文的课出现在视频目录里。
		{"无已发布章节", noPublished, model.CourseTypeIllustrated},
		// 在审视频还没对外承诺，不得把课带进视频目录。
		{"仅有待审视频", pendingVideoOnly, model.CourseTypeIllustrated},
	} {
		if got := readType(tc.courseID); got != tc.want {
			t.Errorf("%s（课程 %d）course_type = %q, want %q", tc.name, tc.courseID, got, tc.want)
		}
	}

	// 库侧硬约束：两态以外的写法必须被拒（空串 / 拼错 / 大小写不符）
	if err := db.Exec("UPDATE course SET course_type = 'mixed' WHERE course_id = ?", mixed).Error; err == nil {
		t.Fatal("course_type='mixed' 被接受了：值域约束没生效")
	}
	if err := db.Exec("UPDATE course SET course_type = '' WHERE course_id = ?", mixed).Error; err == nil {
		t.Fatal("course_type='' 被接受了：NOT NULL/值域约束没生效")
	}
	if err := db.Exec("UPDATE course SET course_type = NULL WHERE course_id = ?", mixed).Error; err == nil {
		t.Fatal("course_type=NULL 被接受了：NOT NULL 没生效")
	}

	// 幂等：重复跑迁移不改已定的类型（管理员手改过的必须保住）。
	if err := db.Exec("UPDATE course SET course_type = 'illustrated' WHERE course_id = ?", pureVideo).Error; err != nil {
		t.Fatal(err)
	}
	apply("000074_course_type", "up")
	if got := readType(pureVideo); got != model.CourseTypeIllustrated {
		t.Errorf("重跑迁移覆盖了管理员的类型选择：got %q", got)
	}
}

// TestCourseTypeMigrationLeavesExistingRowsClassifiable 迁移后全库每一行都可归类：
// 没有 course_type 为空的行（上线前不会出现「类型未知」的课）。
func TestCourseTypeMigrationLeavesExistingRowsClassifiable(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	sql, err := os.ReadFile(filepath.Join("testdata", "000074_course_type.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(sql)).Error; err != nil {
		t.Fatalf("migration 000074 up: %v", err)
	}
	var bad int64
	if err := db.Raw("SELECT count(*) FROM course WHERE course_type IS NULL OR course_type NOT IN ('video','illustrated')").
		Row().Scan(&bad); err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Fatalf("%d 门课的 course_type 不在值域内", bad)
	}
}

// insertReturningID 跑一条 RETURNING 单列并取回 int（本文件的夹具都是「建一行拿 id」）。
func insertReturningID(t *testing.T, db *gorm.DB, sql string, args ...any) int {
	t.Helper()
	var id int
	if err := db.Raw(sql, args...).Row().Scan(&id); err != nil {
		t.Fatalf("insert returning id（%s）: %v", sql, err)
	}
	return id
}
