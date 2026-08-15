// Package service 测试：章节详情共享实现（学员端/导师端同源，Ticket #214 C4）
// seam：service 层（testutil.NewMemoryDB 内存 sqlite）。
// 锁定行为：
//   - 两端 GetChapterDetail 的 prev/next 边界、文件列表、legacy 兼容字段一致；
//   - study_status 仅在学员端路径回填（not_started/completed/studying），导师端省略；
//   - 导师端 GetCourseChapters 消除 N+1（文件列表批量装载）且与详情 shape 零漂移。
package service

import (
	"encoding/json"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// seedChapterWithMeta 建一门课程并返回其中 3 个章节（order_num 1/2/3）。
// 第一个章节挂一个 chapter_file 表条目；第三个章节带 legacy file_url（无 chapter_file 行）。
func seedChapterWithMeta(t *testing.T, db *gorm.DB) (*model.Course, []model.Chapter) {
	t.Helper()
	course := model.Course{Name: "章节详情课程", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	chapters := make([]model.Chapter, 0, 3)
	for i := 1; i <= 3; i++ {
		ch := model.Chapter{CourseID: course.CourseID, Title: "章节", OrderNum: i, CreatedAt: testutil.Now()}
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
	return &course, chapters
}

// cloneDetailToMap 将 ChapterDetailDTO 序列化为 map，便于比对两端 shape 零漂移。
func cloneDetailToMap(t *testing.T, d *ChapterDetailDTO) map[string]any {
	t.Helper()
	b, err := marshalJSON(t, d)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	return m
}

func newCourseServiceForTest(t *testing.T, db *gorm.DB) *CourseService {
	t.Helper()
	return NewCourseService(db, nil, zap.NewNop())
}

// ===== 学员端 CourseService.GetChapterDetail =====

// TestStudentChapterDetailPrevNextBoundaries 学员端详情 prev/next 边界。
func TestStudentChapterDetailPrevNextBoundaries(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newCourseServiceForTest(t, db)
	course, chapters := seedChapterWithMeta(t, db)

	first, err := svc.GetChapterDetail(course.CourseID, chapters[0].ChapterID, 0)
	if err != nil {
		t.Fatalf("首个章节详情失败: %v", err)
	}
	if first.PreviousChapterID != nil {
		t.Fatalf("首章节 prev 应 nil")
	}
	if first.NextChapterID == nil || *first.NextChapterID != chapters[1].ChapterID {
		t.Fatalf("首章节 next 应为 %d", chapters[1].ChapterID)
	}

	mid, err := svc.GetChapterDetail(course.CourseID, chapters[1].ChapterID, 0)
	if err != nil {
		t.Fatalf("中间章节详情失败: %v", err)
	}
	if mid.PreviousChapterID == nil || *mid.PreviousChapterID != chapters[0].ChapterID {
		t.Fatalf("中间章节 prev 应为 %d", chapters[0].ChapterID)
	}
	if mid.NextChapterID == nil || *mid.NextChapterID != chapters[2].ChapterID {
		t.Fatalf("中间章节 next 应为 %d", chapters[2].ChapterID)
	}

	last, err := svc.GetChapterDetail(course.CourseID, chapters[2].ChapterID, 0)
	if err != nil {
		t.Fatalf("末章节详情失败: %v", err)
	}
	if last.PreviousChapterID == nil || *last.PreviousChapterID != chapters[1].ChapterID {
		t.Fatalf("末章节 prev 应为 %d", chapters[1].ChapterID)
	}
	if last.NextChapterID != nil {
		t.Fatalf("末章节 next 应 nil")
	}
}

// TestStudentChapterDetailFilesAndLegacy 学员端详情文件列表 + legacy 兼容。
func TestStudentChapterDetailFilesAndLegacy(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newCourseServiceForTest(t, db)
	course, chapters := seedChapterWithMeta(t, db)

	first, err := svc.GetChapterDetail(course.CourseID, chapters[0].ChapterID, 0)
	if err != nil {
		t.Fatalf("第一章详情失败: %v", err)
	}
	if len(first.Files) != 1 || first.Files[0].FileName != "a.pdf" {
		t.Fatalf("第一章 files 不符: %#v", first.Files)
	}

	last, err := svc.GetChapterDetail(course.CourseID, chapters[2].ChapterID, 0)
	if err != nil {
		t.Fatalf("第三章详情失败: %v", err)
	}
	if len(last.Files) != 1 || last.Files[0].FileID != 0 ||
		last.Files[0].ChapterID == nil || *last.Files[0].ChapterID != chapters[2].ChapterID ||
		last.Files[0].FileName != "c.pdf" || last.Files[0].ContentType != "ppt" {
		t.Fatalf("第三章 legacy 折叠不符: %#v", last.Files)
	}

	mid, err := svc.GetChapterDetail(course.CourseID, chapters[1].ChapterID, 0)
	if err != nil {
		t.Fatalf("第二章详情失败: %v", err)
	}
	if mid.Files == nil || len(mid.Files) != 0 {
		t.Fatalf("第二章 files 应为空数组, got %#v", mid.Files)
	}
}

// TestStudentChapterDetailStudyStatus 学员端 study_status 回填（studentID>0 时）。
func TestStudentChapterDetailStudyStatus(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newCourseServiceForTest(t, db)
	course, chapters := seedChapterWithMeta(t, db)

	d, err := svc.GetChapterDetail(course.CourseID, chapters[0].ChapterID, 1)
	if err != nil {
		t.Fatalf("详情失败: %v", err)
	}
	if d.StudyStatus != "not_started" {
		t.Fatalf("无记录 study_status 应为 not_started, got %q", d.StudyStatus)
	}

	if err := db.Create(&model.StudyRecord{
		StudentID: 1, CourseID: course.CourseID, ChapterID: &chapters[1].ChapterID,
		Progress: 50, StudyDate: testutil.Now(),
	}).Error; err != nil {
		t.Fatalf("创建学习记录失败: %v", err)
	}
	d, err = svc.GetChapterDetail(course.CourseID, chapters[1].ChapterID, 1)
	if err != nil {
		t.Fatalf("详情失败: %v", err)
	}
	if d.StudyStatus != "studying" {
		t.Fatalf("progress<100 study_status 应为 studying, got %q", d.StudyStatus)
	}

	if err := db.Model(&model.StudyRecord{}).
		Where("student_id = ? AND course_id = ? AND chapter_id = ?", 1, course.CourseID, chapters[1].ChapterID).
		Update("progress", 100).Error; err != nil {
		t.Fatalf("更新 progress 失败: %v", err)
	}
	d, err = svc.GetChapterDetail(course.CourseID, chapters[1].ChapterID, 1)
	if err != nil {
		t.Fatalf("详情失败: %v", err)
	}
	if d.StudyStatus != "completed" {
		t.Fatalf("progress=100 study_status 应为 completed, got %q", d.StudyStatus)
	}
}

// TestStudentChapterDetailNoStudentNoStatus studentID<=0 时 study_status 省略（omitempty）。
func TestStudentChapterDetailNoStudentNoStatus(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newCourseServiceForTest(t, db)
	course, chapters := seedChapterWithMeta(t, db)

	d, err := svc.GetChapterDetail(course.CourseID, chapters[0].ChapterID, 0)
	if err != nil {
		t.Fatalf("详情失败: %v", err)
	}
	if d.StudyStatus != "" {
		t.Fatalf("studentID<=0 时 study_status 应为空, got %q", d.StudyStatus)
	}
	m := cloneDetailToMap(t, d)
	if _, ok := m["study_status"]; ok {
		t.Fatal("studentID<=0 时 study_status 不应出现在 JSON 中")
	}
}

// TestStudentChapterDetailErrors 章节不存在/不属于该课程报错。
func TestStudentChapterDetailErrors(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newCourseServiceForTest(t, db)
	course, chapters := seedChapterWithMeta(t, db)

	if _, err := svc.GetChapterDetail(course.CourseID, 99999, 0); err == nil {
		t.Fatal("不存在的章节应报错")
	}
	other := model.Course{Name: "其他课程", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("创建其他课程失败: %v", err)
	}
	if _, err := svc.GetChapterDetail(other.CourseID, chapters[0].ChapterID, 0); err == nil ||
		err.Error() != "章节不属于该课程" {
		t.Fatalf("期望 '章节不属于该课程', got %v", err)
	}
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
	studentSvc := newCourseServiceForTest(t, db)
	course, chapters := seedChapterWithMeta(t, db)

	for _, ch := range chapters {
		tutorD, err := tutorSvc.GetChapterDetail(ch.ChapterID)
		if err != nil {
			t.Fatalf("导师详情失败: %v", err)
		}
		studentD, err := studentSvc.GetChapterDetail(course.CourseID, ch.ChapterID, 0)
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
	course, chapters := seedChapterWithMeta(t, db)

	res, err := svc.GetCourseChapters(course.CourseID)
	if err != nil {
		t.Fatalf("GetCourseChapters 失败: %v", err)
	}
	if len(res.Chapters) != 3 {
		t.Fatalf("章节数 = %d, want 3", len(res.Chapters))
	}
	byID := map[int]*ChapterDTO{}
	for i := range res.Chapters {
		byID[res.Chapters[i].ChapterID] = &res.Chapters[i]
	}
	getFiles := func(cid int) []ChapterFileDTO {
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
	course, _ := seedChapterWithMeta(t, db)

	var queryCount int
	cb := db.Callback().Query().Before("gorm:query")
	if err := cb.Register("c4:nplus1", func(*gorm.DB) { queryCount++ }); err != nil {
		t.Fatalf("注册查询计数回调失败: %v", err)
	}
	defer func() {
		_ = db.Callback().Query().Before("gorm:query").Remove("c4:nplus1")
	}()

	if _, err := svc.GetCourseChapters(course.CourseID); err != nil {
		t.Fatalf("GetCourseChapters 失败: %v", err)
	}
	// 1（课程 Find）+ 1（章节列表 Find）+ 1（文件批量 Find）= 3
	if queryCount > 3 {
		t.Fatalf("批量章节查询数为 %d, 超过常数级上限 3（存在 N+1）", queryCount)
	}
}
