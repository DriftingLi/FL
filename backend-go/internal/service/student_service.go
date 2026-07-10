// Package service 学员信息与学习记录，对应 Python student_service。
package service

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// StudentService 学员服务。
type StudentService struct {
	db *gorm.DB
}

// NewStudentService 创建学员服务实例。
func NewStudentService(db *gorm.DB) *StudentService {
	return &StudentService{db: db}
}

// GetProfile 学员档案，对应 Python get_student_profile。
func (s *StudentService) GetProfile(studentID int) (map[string]interface{}, error) {
	var student model.Student
	if err := s.db.First(&student, studentID).Error; err != nil {
		return nil, errors.New("学员不存在")
	}

	// 总学习时长
	var totalStudyDuration int64
	s.db.Model(&model.StudyRecord{}).Where("student_id = ?", studentID).
		Select("COALESCE(SUM(study_duration), 0)").Scan(&totalStudyDuration)

	// 已完成课程数
	var completedCourses int64
	s.db.Model(&model.StudyRecord{}).Where("student_id = ? AND progress >= 100").
		Distinct("course_id").Count(&completedCourses)

	// 学习中课程数
	var learningCourses int64
	s.db.Model(&model.StudyRecord{}).Where("student_id = ? AND progress > 0 AND progress < 100").
		Distinct("course_id").Count(&learningCourses)

	// 最近学习时间
	var latestRecord model.StudyRecord
	s.db.Where("student_id = ?", studentID).Order("study_date DESC").First(&latestRecord)
	latestStudyTime := ""
	if !latestRecord.StudyDate.IsZero() {
		latestStudyTime = formatISO(latestRecord.StudyDate)
	}

	// 考试次数与平均分
	var examCount int64
	s.db.Model(&model.ExamRecord{}).Where("student_id = ?", studentID).Count(&examCount)
	var avgScore float64
	s.db.Model(&model.ExamRecord{}).Where("student_id = ?", studentID).
		Select("COALESCE(AVG(score), 0)").Scan(&avgScore)

	// 各课程进度
	type courseProgressRow struct {
		CourseID      int
		MaxProgress   float64
		TotalDuration int64
		LatestDate    time.Time
	}
	var rows []courseProgressRow
	s.db.Model(&model.StudyRecord{}).
		Select("course_id, MAX(progress) as max_progress, SUM(study_duration) as total_duration, MAX(study_date) as latest_date").
		Where("student_id = ?", studentID).
		Group("course_id").
		Scan(&rows)

	courseProgressList := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		var course model.Course
		if err := s.db.First(&course, r.CourseID).Error; err != nil {
			continue
		}
		var totalChapters int64
		s.db.Model(&model.Chapter{}).Where("course_id = ?", r.CourseID).Count(&totalChapters)
		studyDate := ""
		if !r.LatestDate.IsZero() {
			studyDate = formatISO(r.LatestDate)
		}
		courseProgressList = append(courseProgressList, map[string]interface{}{
			"course_id":      course.CourseID,
			"course_name":    course.Name,
			"category":       course.Category,
			"progress":       r.MaxProgress,
			"study_duration": r.TotalDuration,
			"total_chapters": totalChapters,
			"study_date":     studyDate,
		})
	}

	return map[string]interface{}{
		"student_info": studentToDict(&student),
		"study_stats": map[string]interface{}{
			"total_study_duration": totalStudyDuration,
			"completed_courses":    completedCourses,
			"learning_courses":     learningCourses,
			"latest_study_time":    latestStudyTime,
			"exam_count":           examCount,
			"avg_score":            roundFloat1(avgScore),
		},
		"course_progress": courseProgressList,
	}, nil
}

// GetRecords 学习记录列表，对应 Python get_student_records。
func (s *StudentService) GetRecords(studentID, page, pageSize int, startDate, endDate string) map[string]interface{} {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := s.db.Model(&model.StudyRecord{}).Where("student_id = ?", studentID)
	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			q = q.Where("study_date >= ?", t)
		}
	}
	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			q = q.Where("study_date <= ?", t.Add(24*time.Hour-time.Nanosecond))
		}
	}
	var total int64
	q.Count(&total)
	var records []model.StudyRecord
	q.Order("study_date DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records)

	items := make([]map[string]interface{}, 0, len(records))
	for i := range records {
		r := &records[i]
		item := studyRecordToDict(r)
		var course model.Course
		if err := s.db.First(&course, r.CourseID).Error; err == nil {
			item["course_name"] = course.Name
		} else {
			item["course_name"] = "未知课程"
		}
		if r.ChapterID != nil {
			var chapter model.Chapter
			if err := s.db.First(&chapter, *r.ChapterID).Error; err == nil {
				item["chapter_title"] = chapter.Title
			} else {
				item["chapter_title"] = nil
			}
		} else {
			item["chapter_title"] = nil
		}
		items = append(items, item)
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return map[string]interface{}{
		"total":   total,
		"page":    page,
		"pages":   pages,
		"records": items,
	}
}

// ===== dict 辅助 =====

func studentToDict(s *model.Student) map[string]interface{} {
	d := map[string]interface{}{
		"student_id": s.StudentID,
		"username":   s.Username,
		"name":       s.Name,
		"status":     s.Status,
		"level":      s.Level,
		"created_at": formatISO(s.CreatedAt),
	}
	if s.LevelUpdatedAt != nil {
		d["level_updated_at"] = formatISO(*s.LevelUpdatedAt)
	} else {
		d["level_updated_at"] = nil
	}
	return d
}

func studyRecordToDict(r *model.StudyRecord) map[string]interface{} {
	d := map[string]interface{}{
		"record_id":      r.RecordID,
		"student_id":     r.StudentID,
		"course_id":      r.CourseID,
		"study_duration": r.StudyDuration,
		"progress":       r.Progress,
		"study_date":     formatISO(r.StudyDate),
	}
	if r.ChapterID != nil {
		d["chapter_id"] = *r.ChapterID
	} else {
		d["chapter_id"] = nil
	}
	return d
}
