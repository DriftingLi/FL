// Package service 管理员服务，对应 Python admin_service。
package service

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// AdminService 管理员服务。
type AdminService struct {
	db *gorm.DB
}

// NewAdminService 创建管理员服务实例。
func NewAdminService(db *gorm.DB) *AdminService {
	return &AdminService{db: db}
}

// GetStudents 学员列表，对应 Python get_students。
func (s *AdminService) GetStudents(page, pageSize int, keyword string) map[string]interface{} {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := s.db.Model(&model.Student{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR name LIKE ?", like, like)
	}
	var total int64
	q.Count(&total)
	var students []model.Student
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&students)
	items := make([]map[string]interface{}, 0, len(students))
	for i := range students {
		items = append(items, studentToDict(&students[i]))
	}
	return map[string]interface{}{
		"total":    total,
		"page":     page,
		"students": items,
	}
}

// DeleteStudent 删除学员，对应 Python delete_student。
func (s *AdminService) DeleteStudent(studentID int) (map[string]interface{}, error) {
	var student model.Student
	if err := s.db.First(&student, studentID).Error; err != nil {
		return nil, errors.New("学员不存在")
	}
	if err := s.db.Delete(&student).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"student_id": studentID}, nil
}

// GetTutors 导师列表，对应 Python get_tutors。
func (s *AdminService) GetTutors(page, pageSize int, keyword string) map[string]interface{} {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := s.db.Model(&model.Tutor{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR name LIKE ?", like, like)
	}
	var total int64
	q.Count(&total)
	var tutors []model.Tutor
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tutors)
	items := make([]map[string]interface{}, 0, len(tutors))
	for i := range tutors {
		items = append(items, tutorToDict(&tutors[i]))
	}
	return map[string]interface{}{
		"total":  total,
		"page":   page,
		"tutors": items,
	}
}

// DeleteTutor 删除导师，对应 Python delete_tutor。
func (s *AdminService) DeleteTutor(tutorID int) (map[string]interface{}, error) {
	var tutor model.Tutor
	if err := s.db.First(&tutor, tutorID).Error; err != nil {
		return nil, errors.New("导师不存在")
	}
	if err := s.db.Delete(&tutor).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"tutor_id": tutorID}, nil
}

// GetStatistics 统计看板，对应 Python get_statistics。
func (s *AdminService) GetStatistics() map[string]interface{} {
	var totalStudents, totalCourses, totalStudyDuration int64
	s.db.Model(&model.Student{}).Count(&totalStudents)
	s.db.Model(&model.Course{}).Count(&totalCourses)
	s.db.Model(&model.StudyRecord{}).Select("COALESCE(SUM(study_duration), 0)").Scan(&totalStudyDuration)

	todayStart := beijingNow()
	startOfDay := todayStart
	startOfDay = startOfDay.Add(-time.Duration(startOfDay.Hour()) * time.Hour)
	startOfDay = startOfDay.Add(-time.Duration(startOfDay.Minute()) * time.Minute)
	startOfDay = startOfDay.Add(-time.Duration(startOfDay.Second()) * time.Second)
	startOfDay = startOfDay.Add(-time.Duration(startOfDay.Nanosecond()) * time.Nanosecond)

	var activeToday int64
	s.db.Model(&model.StudyRecord{}).Where("study_date >= ?", startOfDay).
		Distinct("student_id").Count(&activeToday)

	// 课程统计
	type courseStatRow struct {
		CourseID      int
		Name          string
		StudyCount    int64
		AvgProgress   float64
		TotalDuration int64
	}
	var rows []courseStatRow
	s.db.Model(&model.Course{}).
		Select(`course.course_id, course.name,
			COUNT(DISTINCT study_record.student_id) as study_count,
			COALESCE(AVG(study_record.progress), 0) as avg_progress,
			COALESCE(SUM(study_record.study_duration), 0) as total_duration`).
		Joins("LEFT JOIN study_record ON study_record.course_id = course.course_id").
		Group("course.course_id").
		Scan(&rows)

	courseStats := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		courseStats = append(courseStats, map[string]interface{}{
			"course_id":      r.CourseID,
			"name":           r.Name,
			"study_count":    r.StudyCount,
			"total_duration": r.TotalDuration,
			"avg_progress":   roundFloat2(r.AvgProgress),
		})
	}

	return map[string]interface{}{
		"overview": map[string]interface{}{
			"total_students":       totalStudents,
			"active_today":         activeToday,
			"total_courses":        totalCourses,
			"total_study_duration": totalStudyDuration,
		},
		"course_stats": courseStats,
	}
}

// ===== dict 辅助 =====

func tutorToDict(t *model.Tutor) map[string]interface{} {
	return map[string]interface{}{
		"tutor_id":   t.TutorID,
		"username":   t.Username,
		"name":       t.Name,
		"status":     t.Status,
		"created_at": formatISO(t.CreatedAt),
	}
}
