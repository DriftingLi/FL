// 本文件：章节文件读面合并挂接文件库条目（#2497，spec B2/C13）——
// tutor_file.chapter_id 指向章节的行随章节出现在文件清单；来源判别 Source 分流动作面。
// seam：testutil.NewMemoryDB 内存 sqlite（与 chapter_detail_test.go 同）。
package course

import (
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestLoadChapterFilesMergesAttachedTutorFiles(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	ch := model.Chapter{CourseID: 1, Title: "第一章", ContentType: "text", CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("造章节失败: %v", err)
	}
	other := model.Chapter{CourseID: 1, Title: "第二章", ContentType: "text", CreatedAt: testutil.Now()}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("造章节失败: %v", err)
	}
	// 一个 chapter_file 条目
	cf := model.ChapterFile{ChapterID: &ch.ChapterID, FileURL: "/static/uploads/chapter/x.pdf",
		FileName: "x.pdf", ContentType: "document", FileSize: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&cf).Error; err != nil {
		t.Fatalf("造 chapter_file 失败: %v", err)
	}
	// 两个挂接条目 + 一个挂别章 + 一个未挂
	attached1 := model.TutorFile{TutorID: 1, FileURL: "/static/uploads/tutor-files/1/a.mp4",
		FileName: "a.mp4", ContentType: "video", FileSize: 10, ChapterID: &ch.ChapterID, CreatedAt: testutil.Now()}
	attached2 := model.TutorFile{TutorID: 1, FileURL: "/static/uploads/tutor-files/1/b.pdf",
		FileName: "b.pdf", ContentType: "document", FileSize: 20, ChapterID: &ch.ChapterID, CreatedAt: testutil.Now()}
	elsewhere := model.TutorFile{TutorID: 1, FileURL: "/static/uploads/tutor-files/1/c.mp4",
		FileName: "c.mp4", ContentType: "video", FileSize: 30, ChapterID: &other.ChapterID, CreatedAt: testutil.Now()}
	unattached := model.TutorFile{TutorID: 1, FileURL: "/static/uploads/tutor-files/1/d.mp4",
		FileName: "d.mp4", ContentType: "video", FileSize: 40, CreatedAt: testutil.Now()}
	for _, f := range []*model.TutorFile{&attached1, &attached2, &elsewhere, &unattached} {
		if err := db.Create(f).Error; err != nil {
			t.Fatalf("造 tutor_file 失败: %v", err)
		}
	}

	got := loadChapterFiles(db, &ch)
	if len(got) != 3 {
		t.Fatalf("章内条目数 = %d, want 3（1 chapter_file + 2 挂接）: %+v", len(got), got)
	}
	if got[0].Source != FileSourceChapter || got[0].FileID != cf.FileID {
		t.Fatalf("首条应为 chapter_file 条目: %+v", got[0])
	}
	if got[1].Source != FileSourceTutor || got[1].FileName != "a.mp4" {
		t.Fatalf("第二条应为挂接条目 a: %+v", got[1])
	}
	if got[2].Source != FileSourceTutor || got[2].FileName != "b.pdf" {
		t.Fatalf("第三条应为挂接条目 b: %+v", got[2])
	}
	// 挂接条目的 FileID 是 tutor_file 行号、ChapterID 指向本章
	if got[1].FileID != attached1.FileID || got[1].ChapterID == nil || *got[1].ChapterID != ch.ChapterID {
		t.Fatalf("挂接条目字段不符: %+v", got[1])
	}
}

func TestLoadChapterFilesBulkMergesAttachedTutorFiles(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	chA := model.Chapter{CourseID: 1, Title: "A", ContentType: "text", CreatedAt: testutil.Now()}
	chB := model.Chapter{CourseID: 1, Title: "B", ContentType: "text", CreatedAt: testutil.Now()}
	db.Create(&chA)
	db.Create(&chB)
	f := model.TutorFile{TutorID: 1, FileURL: "/static/uploads/tutor-files/1/a.mp4",
		FileName: "a.mp4", ContentType: "video", FileSize: 10, ChapterID: &chB.ChapterID, CreatedAt: testutil.Now()}
	db.Create(&f)

	got := LoadChapterFilesBulk(db, []model.Chapter{chA, chB})
	if len(got[chA.ChapterID]) != 0 {
		t.Fatalf("A 章不应有条目: %+v", got[chA.ChapterID])
	}
	list := got[chB.ChapterID]
	if len(list) != 1 || list[0].Source != FileSourceTutor || list[0].FileID != f.FileID {
		t.Fatalf("B 章应恰含挂接条目: %+v", list)
	}
}

func TestLoadChapterFilesLegacyFoldWithAttachment(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	ch := model.Chapter{CourseID: 1, Title: "第一章", ContentType: "video",
		FileURL: "/static/uploads/chapter/legacy.mp4", CreatedAt: testutil.Now()}
	db.Create(&ch)
	f := model.TutorFile{TutorID: 1, FileURL: "/static/uploads/tutor-files/1/a.mp4",
		FileName: "a.mp4", ContentType: "video", FileSize: 10, ChapterID: &ch.ChapterID, CreatedAt: testutil.Now()}
	db.Create(&f)

	got := loadChapterFiles(db, &ch)
	if len(got) != 2 {
		t.Fatalf("应为 legacy 折叠 + 挂接 两条: %+v", got)
	}
	if got[0].Source != FileSourceLegacy || got[0].FileID != 0 {
		t.Fatalf("首条应为 legacy 折叠: %+v", got[0])
	}
	if got[1].Source != FileSourceTutor {
		t.Fatalf("次条应为挂接条目: %+v", got[1])
	}
}
