// Package service 测试：管理端 DTO shape-lock（B7 决策 D6）。
// 断言 JSON key 集合与 B7 前的 map 契约逐字一致——前端契约零改动是最高优先级约束。
package service

import "testing"

func TestAdminDTOShapeLock(t *testing.T) {
	tutor := TutorDTO{TutorID: 1, Username: "tutor01", Name: "王老师", Status: 1, CreatedAt: "2026-08-01T10:00:00"}
	assertShapeLock(t, tutor, "tutor_id", "username", "name", "status", "created_at")

	list := TutorListDTO{Total: 3, Page: 1, Tutors: []TutorDTO{tutor}}
	assertShapeLock(t, list, "total", "page", "tutors")

	deleted := TutorDeletedDTO{TutorID: 1}
	assertShapeLock(t, deleted, "tutor_id")

	stats := AdminStatisticsDTO{
		Overview: AdminOverviewDTO{
			TotalStudents: 100, ActiveToday: 5, TotalCourses: 10, TotalStudyDuration: 3600,
		},
		CourseStats: []CourseStatDTO{{
			CourseID: 1, Name: "叉车基础", StudyCount: 20, TotalDuration: 1200, AvgProgress: 56.5,
		}},
	}
	assertShapeLock(t, stats, "overview", "course_stats")
	// 嵌套 key 逐字断言
	raw := topLevelKeys(t, stats.Overview)
	for _, k := range []string{"total_students", "active_today", "total_courses", "total_study_duration"} {
		if !raw[k] {
			t.Errorf("overview 缺少 key: %s", k)
		}
	}
	if len(raw) != 4 {
		t.Errorf("overview key 数量 = %d, 期望 4: %v", len(raw), raw)
	}
	statKeys := topLevelKeys(t, stats.CourseStats[0])
	for _, k := range []string{"course_id", "name", "study_count", "total_duration", "avg_progress"} {
		if !statKeys[k] {
			t.Errorf("course_stats 条目缺少 key: %s", k)
		}
	}
	if len(statKeys) != 5 {
		t.Errorf("course_stats 条目 key 数量 = %d, 期望 5: %v", len(statKeys), statKeys)
	}
}
