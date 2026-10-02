// Package service 测试：章节详情的导师端读面（P2 波 3b-1 实施中拆回本包）。
// 这几条用例的接缝是 TutorService，而域包测试不能 import internal/service 的测试助手（会成环），
// 故导师端一节留在本包；学员端一节随课程域走 internal/course/chapter_detail_test.go。
// seam：service 层（testutil.NewMemoryDB 内存 sqlite）。
// 锁定行为：导师端详情 prev/next/文件/legacy 与学员端 shape 零漂移；GetCourseChapters 文件批量装载（无 N+1）。
package service

import (
	"encoding/json"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/course"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// seedChapterWithMeta 建一门课程并返回其中 3 个章节（order_num 1/2/3）。
// 第一个章节挂一个 chapter_file 表条目；第三个章节带 legacy file_url（无 chapter_file 行）。
// 逐字复制 internal/course/chapter_detail_test.go 的同一助手：两个包各持一份，
// 域包测试与 service 测试互不 import。
func seedChapterWithMeta(t *testing.T, db *gorm.DB) (*model.Course, []model.Chapter) {
	t.Helper()
	// 课程必须是「已发布 + 已挂载」：按 id 取章节详情的读路径已纳入学员可见性谓词
	// （ADR-0058），否则本夹具下的详情读取会按「不存在」返回。不可见面另由
	// TestCourseReadVisibilityContract 覆盖（api 层）。
	spec := model.Specialty{Code: "chapter-detail", Name: "章节详情", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "chapter-detail-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	crs := model.Course{Name: "章节详情课程", Status: 1,
		SpecialtyID: &spec.SpecialtyID, LevelID: &lv.LevelID, CreatedAt: testutil.Now()}
	if err := db.Create(&crs).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	chapters := make([]model.Chapter, 0, 3)
	for i := 1; i <= 3; i++ {
		ch := model.Chapter{CourseID: crs.CourseID, Title: "章节", OrderNum: i, CreatedAt: testutil.Now()}
		if err := db.Create(&ch).Error; err != nil {
			t.Fatalf("创建章节失败: %v", err)
		}
		chapters = append(chapters, ch)
	}
	// 第一章：chapter_file 表条目
	if err := db.Create(&model.ChapterFile{
		ChapterID: &chapters[0].ChapterID, FileName: "a.pdf",
		FileURL: "/static/uploads/chapters/a.pdf", ContentType: "document",
		FileSize: 100, CreatedAt: testutil.Now(),
	}).Error; err != nil {
		t.Fatalf("创建 chapter_file 失败: %v", err)
	}
	// 第三章：legacy file_url（无 chapter_file 行）
	if err := db.Model(&chapters[2]).Updates(map[string]any{
		"file_url": "/static/uploads/chapters/c.pdf", "content_type": "ppt",
	}).Error; err != nil {
		t.Fatalf("更新 legacy file_url 失败: %v", err)
	}
	return &crs, chapters
}

// cloneDetailToMap 将 course.ChapterDetailDTO 序列化为 map，便于比对两端 shape 零漂移。
func cloneDetailToMap(t *testing.T, d *course.ChapterDetailDTO) map[string]any {
	t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	return m
}

// ===== 导师端 TutorService.GetChapterDetail =====

// TestTutorChapterDetailPrevNextAndFiles 导师端详情 prev/next、文件、legacy 与学员端一致。
func TestTutorChapterDetailPrevNextAndFiles(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newTutorServiceForTest(t, db)
	_, chapters := seedChapterWithMeta(t, db)

	first, err := svc.GetChapterDetail(chapters[0].ChapterID)
	if err != nil {
		t.Fatalf("导师首章节详情失败: %v", err)
	}
	if first.PreviousChapterID != nil || first.NextChapterID == nil ||
		*first.NextChapterID != chapters[1].ChapterID {
		t.Fatalf("导师首章节 prev/next 不一致")
	}
	if len(first.Files) != 1 || first.Files[0].FileName != "a.pdf" {
		t.Fatalf("导师首章节文件列表不符: %#v", first.Files)
	}

	last, err := svc.GetChapterDetail(chapters[2].ChapterID)
	if err != nil {
		t.Fatalf("导师末章节详情失败: %v", err)
	}
	if len(last.Files) != 1 || last.Files[0].FileID != 0 ||
		last.Files[0].ChapterID == nil || *last.Files[0].ChapterID != chapters[2].ChapterID ||
		last.Files[0].ContentType != "ppt" {
		t.Fatalf("导师端 legacy 折叠不一致: %#v", last.Files)
	}

	mid, err := svc.GetChapterDetail(chapters[1].ChapterID)
	if err != nil {
		t.Fatalf("导师中间章节详情失败: %v", err)
	}
	if mid.StudyStatus != "" {
		t.Fatalf("导师端不应回填 study_status, got %q", mid.StudyStatus)
	}
	m := cloneDetailToMap(t, mid)
	if _, ok := m["study_status"]; ok {
		t.Fatal("导师端详情不应包含 study_status key")
	}
}

// TestTutorChapterDetailShapeZeroDrift 导师端详情与学员端详情（studentID=0 关闭回填）shape 零漂移。
func TestTutorChapterDetailShapeZeroDrift(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	tutorSvc := newTutorServiceForTest(t, db)
	studentSvc := course.NewService(db, nil, zap.NewNop())
	crs, chapters := seedChapterWithMeta(t, db)

	for _, ch := range chapters {
		tutorD, err := tutorSvc.GetChapterDetail(ch.ChapterID)
		if err != nil {
			t.Fatalf("导师详情失败: %v", err)
		}
		studentD, err := studentSvc.GetChapterDetail(crs.CourseID, ch.ChapterID, 0)
		if err != nil {
			t.Fatalf("学员详情失败: %v", err)
		}
		tm := cloneDetailToMap(t, tutorD)
		sm := cloneDetailToMap(t, studentD)
		for _, k := range []string{"chapter_id", "course_id", "title", "content", "content_type",
			"file_url", "description", "duration", "order_num", "created_at",
			"files", "previous_chapter_id", "next_chapter_id"} {
			tv, tok := tm[k]
			sv, sok := sm[k]
			if tok != sok {
				t.Fatalf("key %q 存在性漂移: 导师=%v 学员=%v", k, tok, sok)
			}
			if !tok {
				continue
			}
			// files 为嵌套 []interface{}，不可直接用 != 比较；统一经 JSON 归一化比较字节。
			tb, _ := json.Marshal(tv)
			sb, _ := json.Marshal(sv)
			if string(tb) != string(sb) {
				t.Fatalf("key %q 值漂移: 导师=%s 学员=%s", k, tb, sb)
			}
		}
	}
}

// ===== 导师端 GetCourseChapters 批量加载（N+1 消除）=====

// TestTutorGetCourseChaptersFilesPopulated 批量章节列表的文件列表正确（chapter_file + legacy）。
func TestTutorGetCourseChaptersFilesPopulated(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newTutorServiceForTest(t, db)
	crs, chapters := seedChapterWithMeta(t, db)

	res, err := svc.GetCourseChapters(crs.CourseID)
	if err != nil {
		t.Fatalf("GetCourseChapters 失败: %v", err)
	}
	if len(res.Chapters) != 3 {
		t.Fatalf("章节数 = %d, want 3", len(res.Chapters))
	}
	byID := map[int]*course.ChapterDTO{}
	for i := range res.Chapters {
		byID[res.Chapters[i].ChapterID] = &res.Chapters[i]
	}
	getFiles := func(cid int) []course.ChapterFileDTO {
		ch, ok := byID[cid]
		if !ok || ch.Files == nil {
			return nil
		}
		return *ch.Files
	}
	if f := getFiles(chapters[0].ChapterID); len(f) != 1 || f[0].FileName != "a.pdf" {
		t.Fatalf("第一章批量文件不符: %#v", byID[chapters[0].ChapterID].Files)
	}
	if f := getFiles(chapters[2].ChapterID); len(f) != 1 || f[0].FileID != 0 || f[0].ContentType != "ppt" {
		t.Fatalf("第三章批量 legacy 折叠不符: %#v", byID[chapters[2].ChapterID].Files)
	}
	if f := getFiles(chapters[1].ChapterID); len(f) != 0 {
		t.Fatalf("第二章批量文件应为空数组, got %#v", byID[chapters[1].ChapterID].Files)
	}
}

// TestTutorGetCourseChaptersNoNPlusOne 批量章节列表：多章节时文件装载为常数级查询（无 N+1）。
func TestTutorGetCourseChaptersNoNPlusOne(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newTutorServiceForTest(t, db)
	crs, _ := seedChapterWithMeta(t, db)

	var queryCount int
	cb := db.Callback().Query().Before("gorm:query")
	if err := cb.Register("c4:nplus1", func(*gorm.DB) { queryCount++ }); err != nil {
		t.Fatalf("注册查询计数回调失败: %v", err)
	}
	defer func() {
		_ = db.Callback().Query().Before("gorm:query").Remove("c4:nplus1")
	}()

	if _, err := svc.GetCourseChapters(crs.CourseID); err != nil {
		t.Fatalf("GetCourseChapters 失败: %v", err)
	}
	// 1（课程 Find）+ 1（章节列表 Find）+ 1（文件批量 Find）= 3
	if queryCount > 3 {
		t.Fatalf("批量章节查询数为 %d, 超过常数级上限 3（存在 N+1）", queryCount)
	}
}
