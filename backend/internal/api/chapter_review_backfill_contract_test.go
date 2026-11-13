// 契约测试 #2498 C12：存量一次性豁免回填（pg 双跑）。
//
// 沿 000032 backfill_mock_exam_credential 先例：回填做成可重放函数
// （backfill_chapter_review_state，迁移内调用一次），本测试把样本打回「迁移前形状」
// （review_status='draft'、无槽行）后重放，验证三源文件清单冻结语义与幂等：
//
//	chapter_file 行 / tutor_file 挂接行 / legacy 折叠（无 chapter_file 行且 file_url 非空）；
//	所有存量章节置 review_status='published'（视为已过审、不入队列）。
package api

import (
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestChapterReviewBackfillOnPostgres(t *testing.T) {
	testutil.SetTestGinMode()
	db := testutil.NewPostgresDB(t)
	if db == nil {
		t.Skip("DATABASE_URL 未设置")
	}

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "backfill-rev", Name: "回填方向", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "backfill-rev-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("建等级失败: %v", err)
	}
	c := model.Course{Name: "回填课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("建课程失败: %v", err)
	}

	// 迁移前形状的四个章节：带 chapter_file 行 / 带 tutor_file 挂接行 / legacy 折叠 / 无文件。
	mkChapter := func(title, content, fileURL string) *model.Chapter {
		ch := model.Chapter{CourseID: c.CourseID, Title: title, Content: content,
			ContentType: "text", FileURL: fileURL, OrderNum: 1, CreatedAt: testutil.Now()}
		if err := db.Create(&ch).Error; err != nil {
			t.Fatalf("建章节失败: %v", err)
		}
		return &ch
	}
	chCf := mkChapter("带章节文件", "cf 正文", "")
	chTf := mkChapter("带挂接文件", "tf 正文", "")
	chLegacy := mkChapter("legacy 折叠", "legacy 正文", "/static/uploads/legacy/旧课件.pdf")
	chPlain := mkChapter("无文件", "plain 正文", "")

	cf := model.ChapterFile{ChapterID: &chCf.ChapterID, FileURL: "/static/uploads/a.pdf",
		FileName: "a.pdf", ContentType: "document", FileSize: 10, CreatedAt: testutil.Now()}
	if err := db.Create(&cf).Error; err != nil {
		t.Fatalf("建 chapter_file 失败: %v", err)
	}
	tf := model.TutorFile{TutorID: 1, FileURL: "/static/uploads/b.mp4",
		FileName: "b.mp4", ContentType: "video", FileSize: 20, CreatedAt: testutil.Now()}
	if err := db.Create(&tf).Error; err != nil {
		t.Fatalf("建 tutor_file 失败: %v", err)
	}
	if err := db.Model(&model.TutorFile{}).Where("file_id = ?", tf.FileID).
		Update("chapter_id", chTf.ChapterID).Error; err != nil {
		t.Fatalf("挂 tutor_file 失败: %v", err)
	}

	// 打回迁移前形状：审核列回 draft、删槽行（迁移跑在空库上，函数只在迁移内调用过一次）。
	if err := db.Exec("UPDATE chapter SET review_status = 'draft'").Error; err != nil {
		t.Fatalf("重置 review_status 失败: %v", err)
	}
	if err := db.Exec("DELETE FROM chapter_content_slot").Error; err != nil {
		t.Fatalf("清内容槽失败: %v", err)
	}
	if err := db.Exec("DELETE FROM chapter_file_slot").Error; err != nil {
		t.Fatalf("清文件槽失败: %v", err)
	}

	replay := func() {
		t.Helper()
		if err := db.Exec("SELECT backfill_chapter_review_state()").Error; err != nil {
			t.Fatalf("重放回填函数失败: %v", err)
		}
	}
	slotCount := func(sql string, args ...any) int {
		t.Helper()
		var n int64
		if err := db.Raw(sql, args...).Scan(&n).Error; err != nil {
			t.Fatalf("查槽行失败: %v", err)
		}
		return int(n)
	}

	replay()

	// 全部存量章节视为已过审（C12：不入队列）。
	for _, ch := range []*model.Chapter{chCf, chTf, chLegacy, chPlain} {
		var got model.Chapter
		if err := db.First(&got, ch.ChapterID).Error; err != nil {
			t.Fatalf("查章节 %d 失败: %v", ch.ChapterID, err)
		}
		if got.ReviewStatus != model.ChapterReviewPublished {
			t.Fatalf("章节 %q 应豁免为 published, got %q", ch.Title, got.ReviewStatus)
		}
	}
	// 内容槽：每章一行，正文冻结 working 值。
	for _, ch := range []*model.Chapter{chCf, chTf, chLegacy, chPlain} {
		var content string
		if err := db.Raw(`SELECT content FROM chapter_content_slot WHERE slot='published' AND chapter_id=?`,
			ch.ChapterID).Scan(&content).Error; err != nil {
			t.Fatalf("查内容槽失败: %v", err)
		}
		if content != ch.Content {
			t.Fatalf("章节 %q 内容槽应冻结 %q, got %q", ch.Title, ch.Content, content)
		}
	}

	// 文件清单三源：chapter_file / tutor_file 挂接 / legacy 折叠各落对，互不串源。
	if n := slotCount(`SELECT count(*) FROM chapter_file_slot WHERE chapter_id=? AND source='chapter_file' AND file_id=?`,
		chCf.ChapterID, cf.FileID); n != 1 {
		t.Fatalf("chapter_file 源应 1 行, got %d", n)
	}
	if n := slotCount(`SELECT count(*) FROM chapter_file_slot WHERE chapter_id=? AND source='tutor_file' AND file_id=?`,
		chTf.ChapterID, tf.FileID); n != 1 {
		t.Fatalf("tutor_file 源应 1 行, got %d", n)
	}
	var legacyName string
	if err := db.Raw(`SELECT file_name FROM chapter_file_slot WHERE chapter_id=? AND source='legacy'`,
		chLegacy.ChapterID).Scan(&legacyName).Error; err != nil {
		t.Fatalf("查 legacy 槽失败: %v", err)
	}
	if legacyName != "旧课件.pdf" {
		t.Fatalf("legacy 折叠文件名应取 URL 尾段「旧课件.pdf」, got %q", legacyName)
	}
	// legacy 折叠只在无 chapter_file 行时成立：chCf/chTf 不得混入 legacy 源。
	// 只在本测试造的四章范围内数——迁移库自带存量章节可能有自己的 legacy 槽。
	mineIDs := []int{chCf.ChapterID, chTf.ChapterID, chLegacy.ChapterID, chPlain.ChapterID}
	if n := slotCount(`SELECT count(*) FROM chapter_file_slot WHERE chapter_id IN (?) AND source='legacy'`, mineIDs); n != 1 {
		t.Fatalf("legacy 源应只有折叠章节 1 行, got %d", n)
	}
	// 无文件章节不落文件槽。
	if n := slotCount(`SELECT count(*) FROM chapter_file_slot WHERE chapter_id=?`, chPlain.ChapterID); n != 0 {
		t.Fatalf("无文件章节不应有文件槽, got %d", n)
	}

	// 幂等：重放不重复落行（函数保留不删的意义即在此）。
	// 只数本次造的四章——PG 迁移库里还有 baseline/seed 迁移带进来的存量章节，
	// 重放会给它们也建槽，全局计数不是本测试的判据。
	replay()
	if n := slotCount(`SELECT count(*) FROM chapter_file_slot WHERE chapter_id IN (?)`, mineIDs); n != 3 {
		t.Fatalf("重放后文件槽应仍 3 行（幂等）, got %d", n)
	}
	if n := slotCount(`SELECT count(*) FROM chapter_content_slot WHERE chapter_id IN (?)`, mineIDs); n != 4 {
		t.Fatalf("重放后内容槽应仍 4 行（幂等）, got %d", n)
	}
}
