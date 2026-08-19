// ADR-0017 service 单测：进度上报换算与学习位置刷新。
//   - duration_seconds 优先并按 ceil 换算分钟
//   - completed 显式完成收敛 progress=100
//   - video_position 落库；带章节上报刷新课程级 last_chapter_id / last_studied_at
package service

import (
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newProgressTestEnv(t *testing.T) (*CourseService, *model.Course, *model.Chapter) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	svc := NewCourseService(db, nil, zap.NewNop())
	course := model.Course{Name: "换算课程", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: course.CourseID, Title: "第一章", Duration: 10, OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("创建章节失败: %v", err)
	}
	return svc, &course, &ch
}

func TestUpdateStudyProgressSecondsPreferredAndCeil(t *testing.T) {
	svc, course, ch := newProgressTestEnv(t)
	db := svc.db

	// 95 秒 → ceil 2 分钟。
	if _, err := svc.UpdateStudyProgress(1, course.CourseID, StudyProgressInput{ChapterID: ch.ChapterID, DurationSecs: 95}); err != nil {
		t.Fatalf("上报失败: %v", err)
	}
	// duration_seconds 优先于 duration：Duration=5 被忽略，1 秒 → 1 分钟。
	if _, err := svc.UpdateStudyProgress(1, course.CourseID, StudyProgressInput{ChapterID: ch.ChapterID, Duration: 5, DurationSecs: 1}); err != nil {
		t.Fatalf("上报失败: %v", err)
	}
	var chRow model.StudyRecord
	if err := db.Where("student_id = ? AND chapter_id = ?", 1, ch.ChapterID).First(&chRow).Error; err != nil {
		t.Fatalf("查章节记录失败: %v", err)
	}
	if chRow.StudyDuration != 3 {
		t.Fatalf("章节累计应为 2+1=3 分钟, got %d", chRow.StudyDuration)
	}
	if chRow.Progress == 100 {
		t.Fatal("未达阈值/未显式完成不应 progress=100")
	}
}

func TestUpdateStudyProgressCompletedAndPosition(t *testing.T) {
	svc, course, ch := newProgressTestEnv(t)
	db := svc.db
	pos := 823

	if _, err := svc.UpdateStudyProgress(1, course.CourseID, StudyProgressInput{ChapterID: ch.ChapterID, Completed: true, VideoPosition: &pos}); err != nil {
		t.Fatalf("上报失败: %v", err)
	}
	var chRow model.StudyRecord
	if err := db.Where("student_id = ? AND chapter_id = ?", 1, ch.ChapterID).First(&chRow).Error; err != nil {
		t.Fatalf("查章节记录失败: %v", err)
	}
	if chRow.Progress != 100 {
		t.Fatalf("显式完成应置 progress=100, got %v", chRow.Progress)
	}
	if chRow.VideoPosition != pos {
		t.Fatalf("播放位置应落库 %d, got %d", pos, chRow.VideoPosition)
	}

	// 课程级记录：last_chapter_id / last_studied_at 已刷新。
	var record model.StudyRecord
	if err := db.Where("student_id = ? AND course_id = ? AND chapter_id IS NULL", 1, course.CourseID).First(&record).Error; err != nil {
		t.Fatalf("查课程级记录失败: %v", err)
	}
	if record.LastChapterID == nil || *record.LastChapterID != ch.ChapterID {
		t.Fatalf("last_chapter_id 应为 %d, got %v", ch.ChapterID, record.LastChapterID)
	}
	if record.LastStudiedAt == nil {
		t.Fatal("last_studied_at 不应为 nil")
	}

	// 不带章节的上报不改变 last_*。
	if _, err := svc.UpdateStudyProgress(1, course.CourseID, StudyProgressInput{Duration: 3}); err != nil {
		t.Fatalf("上报失败: %v", err)
	}
	var after model.StudyRecord
	if err := db.Where("student_id = ? AND course_id = ? AND chapter_id IS NULL", 1, course.CourseID).First(&after).Error; err != nil {
		t.Fatalf("查课程级记录失败: %v", err)
	}
	if after.LastChapterID == nil || *after.LastChapterID != ch.ChapterID {
		t.Fatalf("无章节上报不应改变 last_chapter_id, got %v", after.LastChapterID)
	}
}

func TestLoadLearningPositionLegacyFallback(t *testing.T) {
	svc, course, _ := newProgressTestEnv(t)
	db := svc.db
	chID := 11

	// 历史数据：仅章节级记录（无 chapter_id IS NULL 的课程级记录）。
	legacy := model.StudyRecord{StudentID: 2, CourseID: course.CourseID, ChapterID: &chID,
		StudyDuration: 7, Progress: 100, StudyDate: testutil.Now()}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("创建历史记录失败: %v", err)
	}
	lp := loadLearningPosition(db, 2, course.CourseID)
	if lp.RecordID == 0 {
		t.Fatal("历史数据回退应命中记录")
	}
	if lp.CompletedChapters != 1 {
		t.Fatalf("完成章节数应为 1, got %d", lp.CompletedChapters)
	}

	// 未学学员零值。
	lp = loadLearningPosition(db, 3, course.CourseID)
	if lp.RecordID != 0 || lp.CompletedChapters != 0 {
		t.Fatalf("未学学员应为零值, got %+v", lp)
	}
	_ = gorm.ErrRecordNotFound
}
