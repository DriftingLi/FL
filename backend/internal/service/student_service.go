// Package service 学员信息与学习记录。
package service

import (
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

// StudentService 学员服务。
type StudentService struct {
	db *gorm.DB

	logger *zap.Logger
}

// NewStudentService 创建学员服务实例。
func NewStudentService(db *gorm.DB, logger *zap.Logger) *StudentService {
	return &StudentService{db: db, logger: logger}
}

// ===== DTO（JSON 契约与 B8 前的 map key 逐字一致，前端零改动约束）=====

// StudentDTO 学员基本信息。
type StudentDTO struct {
	StudentID int    `json:"student_id"`
	UID       int64  `json:"uid,string"`
	Account   string `json:"account"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	Status    int16  `json:"status"`
	CreatedAt string `json:"created_at"`
}

// StudyStatsDTO 学习统计概览。
type StudyStatsDTO struct {
	TotalCourses       int64  `json:"total_courses"`
	TotalStudyDuration int64  `json:"total_study_duration"`
	CompletedCourses   int64  `json:"completed_courses"`
	LearningCourses    int64  `json:"learning_courses"`
	LatestStudyTime    string `json:"latest_study_time"`
}

// CourseProgressDTO 课程进度条目。
type CourseProgressDTO struct {
	CourseID      int     `json:"course_id"`
	CourseName    string  `json:"course_name"`
	Progress      float64 `json:"progress"`
	StudyDuration int64   `json:"study_duration"`
	TotalChapters int64   `json:"total_chapters"`
	StudyDate     string  `json:"study_date"`
}

// StudentProfileDTO 学员档案信封（信息 + 统计 + 课程进度）。
type StudentProfileDTO struct {
	StudentInfo    StudentDTO          `json:"student_info"`
	StudyStats     StudyStatsDTO       `json:"study_stats"`
	CourseProgress []CourseProgressDTO `json:"course_progress"`
}

// StudyDailyStatsDTO 按天学习统计（学员仪表盘图表）。
type StudyDailyStatsDTO struct {
	Days         int      `json:"days"`
	Labels       []string `json:"labels"`
	Data         []int64  `json:"data"`
	TotalMinutes int64    `json:"total_minutes"`
	ActiveDays   int      `json:"active_days"`
}

// StudyRecordDTO 学习记录条目（含课程名回填；chapter_title 未匹配章节时 null）。
type StudyRecordDTO struct {
	RecordID      int     `json:"record_id"`
	StudentID     int     `json:"student_id"`
	CourseID      int     `json:"course_id"`
	ChapterID     *int    `json:"chapter_id"`
	StudyDuration int     `json:"study_duration"`
	Progress      float64 `json:"progress"`
	StudyDate     string  `json:"study_date"`
	CourseName    string  `json:"course_name"`
	ChapterTitle  *string `json:"chapter_title"`
}

// GetProfile 学员档案。
func (s *StudentService) GetProfile(studentID int) (*StudentProfileDTO, error) {
	return s.queryProfile(studentID)
}

// queryProfile 执行实际的学员档案查询。
func (s *StudentService) queryProfile(studentID int) (*StudentProfileDTO, error) {
	var student model.HrwaiUser
	if err := s.db.First(&student, studentID).Error; err != nil {
		return nil, errors.New("学员不存在")
	}

	// 总学习时长
	var totalStudyDuration int64
	s.db.Model(&model.StudyRecord{}).Where("student_id = ?", studentID).
		Select("COALESCE(SUM(study_duration), 0)").Scan(&totalStudyDuration)

	// 课程总数（系统中所有已发布课程的数量，status=1 表示已发布）
	var totalCourses int64
	s.db.Model(&model.Course{}).Where("status = ?", 1).Count(&totalCourses)

	// 已完成课程数
	var completedCourses int64
	s.db.Model(&model.StudyRecord{}).Where("student_id = ? AND chapter_id IS NULL AND progress >= 100", studentID).
		Distinct("course_id").Count(&completedCourses)

	// 学习中课程数
	var learningCourses int64
	s.db.Model(&model.StudyRecord{}).Where("student_id = ? AND chapter_id IS NULL AND progress > 0 AND progress < 100", studentID).
		Distinct("course_id").Count(&learningCourses)

	// 最近学习时间（学员可能无任何学习记录，使用 Limit(1).Find() 避免 First() 在空结果时打印 record not found 日志）
	var latestRecord model.StudyRecord
	s.db.Where("student_id = ?", studentID).Order("study_date DESC").Limit(1).Find(&latestRecord)
	latestStudyTime := ""
	if !latestRecord.StudyDate.IsZero() {
		latestStudyTime = formatISO(latestRecord.StudyDate)
	}

	// 各课程进度
	type courseProgressRow struct {
		CourseID      int
		MaxProgress   float64
		TotalDuration int64
		LatestDate    time.Time
	}
	var rows []courseProgressRow
	s.db.Model(&model.StudyRecord{}).
		// 进度只看课程级记录（chapter_id IS NULL）；学习时长汇总该课程全部记录
		Select("course_id, MAX(progress) FILTER (WHERE chapter_id IS NULL) AS max_progress, "+
			"SUM(study_duration) AS total_duration, MAX(study_date) AS latest_date").
		Where("student_id = ?", studentID).
		Group("course_id").
		Scan(&rows)

	courseProgressList := make([]CourseProgressDTO, 0, len(rows))
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
		courseProgressList = append(courseProgressList, CourseProgressDTO{
			CourseID:      course.CourseID,
			CourseName:    course.Name,
			Progress:      r.MaxProgress,
			StudyDuration: r.TotalDuration,
			TotalChapters: totalChapters,
			StudyDate:     studyDate,
		})
	}

	return &StudentProfileDTO{
		StudentInfo: studentToDTO(&student),
		StudyStats: StudyStatsDTO{
			TotalCourses:       totalCourses,
			TotalStudyDuration: totalStudyDuration,
			CompletedCourses:   completedCourses,
			LearningCourses:    learningCourses,
			LatestStudyTime:    latestStudyTime,
		},
		CourseProgress: courseProgressList,
	}, nil
}

// GetStudyStats 学习统计（按天分组），用于学员仪表盘图表。
// days 仅允许 7 或 30，其他值统一回退为 7。
func (s *StudentService) GetStudyStats(studentID, days int) *StudyDailyStatsDTO {
	return s.queryStudyStats(studentID, days)
}

// queryStudyStats 执行实际的学习统计查询。
func (s *StudentService) queryStudyStats(studentID, days int) *StudyDailyStatsDTO {
	// 按天聚合学习时长（study_date 为 timestamp without time zone，按存储值即北京时间分组）
	// 起点由 BuildDailySeries 内部的 days 钳制 + startOfDay 归零 + 起点统一计算，此处仅 SQL 聚合出 day→分钟 map。
	start := dailySeriesStart(days)
	type dailyRow struct {
		Day     string
		Minutes int64
	}
	var rows []dailyRow
	s.db.Model(&model.StudyRecord{}).
		Select("TO_CHAR(study_date, 'YYYY-MM-DD') as day, COALESCE(SUM(study_duration), 0) as minutes").
		Where("student_id = ? AND study_date >= ?", studentID, start).
		Group("day").
		Order("day ASC").
		Scan(&rows)

	minutesByDay := make(map[string]int64, len(rows))
	for _, r := range rows {
		minutesByDay[r.Day] = r.Minutes
	}

	series := BuildDailySeries(days, minutesByDay)
	return &StudyDailyStatsDTO{
		Days:         series.Days,
		Labels:       series.Labels,
		Data:         series.Data,
		TotalMinutes: series.Total,
		ActiveDays:   series.ActiveDays,
	}
}

// StudyRecordPageResult 学习记录分页结果。
type StudyRecordPageResult struct {
	Page    int              `json:"page"`
	Pages   int              `json:"pages"`
	Records []StudyRecordDTO `json:"records"`
	Total   int64            `json:"total"`
}

// GetRecords 学习记录列表。
func (s *StudentService) GetRecords(studentID, page, pageSize int, startDate, endDate string) StudyRecordPageResult {
	records, total, page, pageSize := paging.Query[model.StudyRecord](s.db, page, pageSize, 10, "study_date DESC", func(q *gorm.DB) *gorm.DB {
		q = q.Where("student_id = ?", studentID)
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
		return q
	})

	items := make([]StudyRecordDTO, 0, len(records))
	for i := range records {
		r := &records[i]
		item := studyRecordToDTO(r)
		var course model.Course
		if err := s.db.First(&course, r.CourseID).Error; err == nil {
			item.CourseName = course.Name
		} else {
			item.CourseName = "未知课程"
		}
		if r.ChapterID != nil {
			var chapter model.Chapter
			if err := s.db.First(&chapter, *r.ChapterID).Error; err == nil {
				title := chapter.Title
				item.ChapterTitle = &title
			}
		}
		items = append(items, item)
	}
	return StudyRecordPageResult{
		Page:    page,
		Pages:   response.PageCount(total, pageSize),
		Records: items,
		Total:   total,
	}
}

// ===== DTO 构造（原 studentToDict/studyRecordToDict 折叠入内）=====

func studentToDTO(s *model.HrwaiUser) StudentDTO {
	return StudentDTO{
		StudentID: s.ID,
		UID:       s.UID,
		Account:   s.Account,
		Username:  s.Username,
		AvatarURL: s.AvatarURL,
		Status:    s.Status,
		CreatedAt: formatISO(s.CreatedAt),
	}
}

func studyRecordToDTO(r *model.StudyRecord) StudyRecordDTO {
	return StudyRecordDTO{
		RecordID:      r.RecordID,
		StudentID:     r.StudentID,
		CourseID:      r.CourseID,
		ChapterID:     r.ChapterID,
		StudyDuration: r.StudyDuration,
		Progress:      r.Progress,
		StudyDate:     formatISO(r.StudyDate),
	}
}
