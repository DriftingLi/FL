// Package service 测试：学员端 DTO shape-lock（B8 决策 D6）。
// 断言 JSON key 集合与 B8 前的 map 契约逐字一致——前端契约零改动是最高优先级约束。
package service

import "testing"

func TestStudentDTOShapeLock(t *testing.T) {
	student := StudentDTO{
		StudentID: 42, Username: "13800000000", Name: "张三", Nickname: "小张",
		AvatarURL: "/static/uploads/avatars/a.webp", Status: 1, CreatedAt: "2026-08-01T10:00:00",
	}
	assertShapeLock(t, student,
		"student_id", "username", "name", "nickname", "avatar_url", "status", "created_at",
	)

	stats := StudyStatsDTO{
		TotalCourses: 5, TotalStudyDuration: 360, CompletedCourses: 2, LearningCourses: 1,
		LatestStudyTime: "2026-08-01T09:00:00", ExamCount: 3, AvgScore: 78.5,
	}
	assertShapeLock(t, stats,
		"total_courses", "total_study_duration", "completed_courses", "learning_courses",
		"latest_study_time", "exam_count", "avg_score",
	)

	progress := CourseProgressDTO{
		CourseID: 1, CourseName: "叉车基础", Progress: 66.66, StudyDuration: 120,
		TotalChapters: 6, StudyDate: "2026-08-01T09:00:00",
	}
	assertShapeLock(t, progress,
		"course_id", "course_name", "progress", "study_duration", "total_chapters", "study_date",
	)

	profile := StudentProfileDTO{
		StudentInfo:    student,
		StudyStats:     stats,
		CourseProgress: []CourseProgressDTO{progress},
	}
	assertShapeLock(t, profile, "student_info", "study_stats", "course_progress")

	daily := StudyDailyStatsDTO{
		Days: 7, Labels: []string{"1/1"}, Data: []int64{30}, TotalMinutes: 30, ActiveDays: 1,
	}
	assertShapeLock(t, daily, "days", "labels", "data", "total_minutes", "active_days")

	// 学习记录：chapter_id / chapter_title 为 null 时 key 仍在（课程级记录语义）
	rec := StudyRecordDTO{
		RecordID: 1, StudentID: 42, CourseID: 1, ChapterID: nil,
		StudyDuration: 30, Progress: 100, StudyDate: "2026-08-01T09:00:00",
		CourseName: "叉车基础", ChapterTitle: nil,
	}
	assertShapeLock(t, rec,
		"record_id", "student_id", "course_id", "chapter_id", "study_duration",
		"progress", "study_date", "course_name", "chapter_title",
	)
	if b, _ := marshalJSON(t, rec); string(b) != `{"record_id":1,"student_id":42,"course_id":1,"chapter_id":null,"study_duration":30,"progress":100,"study_date":"2026-08-01T09:00:00","course_name":"叉车基础","chapter_title":null}` {
		t.Fatalf("study_record 序列化与旧 map 契约不符（chapter_id/chapter_title 为 null）: %s", b)
	}

	page := StudyRecordPageResult{Page: 1, Pages: 1, Records: []StudyRecordDTO{rec}, Total: 1}
	assertShapeLock(t, page, "page", "pages", "records", "total")
}
