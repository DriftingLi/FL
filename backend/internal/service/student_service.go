// Package service 学员信息与学习记录。
package service

import (
	"errors"
	"sort"
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

	// 批量回填课程名与章节数（batch_backfill module），消除逐课程 N+1；
	// 未知课程跳过整行（courseNameFound 缺省 false，与旧逐行 First 失败 continue 同语义）。
	courseIDs := make([]int, 0, len(rows))
	for _, r := range rows {
		courseIDs = append(courseIDs, r.CourseID)
	}
	courseNames := batchCourseNames(s.db, courseIDs)
	chapterCounts := batchChapterCounts(s.db, courseIDs)

	courseProgressList := make([]CourseProgressDTO, 0, len(rows))
	for _, r := range rows {
		name, ok := courseNameFound(courseNames, r.CourseID)
		if !ok {
			continue
		}
		studyDate := ""
		if !r.LatestDate.IsZero() {
			studyDate = formatISO(r.LatestDate)
		}
		courseProgressList = append(courseProgressList, CourseProgressDTO{
			CourseID:      r.CourseID,
			CourseName:    name,
			Progress:      r.MaxProgress,
			StudyDuration: r.TotalDuration,
			TotalChapters: chapterCounts[r.CourseID],
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

	// 批量回填课程名与章节标题（batch_backfill module），消除逐记录 N+1。
	// 未知课程缺省文案由 courseName 统一解析为 UnknownCourseName；
	// chapter_title 未匹配章节时保持 null（与旧逐行 First 失败不赋值同语义）。
	courseIDs := make([]int, 0, len(records))
	chapterIDs := make([]int, 0, len(records))
	for i := range records {
		r := &records[i]
		courseIDs = append(courseIDs, r.CourseID)
		if r.ChapterID != nil {
			chapterIDs = append(chapterIDs, *r.ChapterID)
		}
	}
	courseNames := batchCourseNames(s.db, courseIDs)
	chapterTitles := batchChapterTitles(s.db, chapterIDs)

	items := make([]StudyRecordDTO, 0, len(records))
	for i := range records {
		r := &records[i]
		item := studyRecordToDTO(r)
		item.CourseName = courseName(courseNames, r.CourseID)
		if r.ChapterID != nil {
			if title, ok := chapterTitles[*r.ChapterID]; ok {
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

// ===== 我的课程（ADR-0017：学习位置与 continue-learning）=====

// StudentCourseDTO 我的课程条目：course_progress 的超集（补 cover/方向/等级/
// 完成章节数/最后学习位置）。
type StudentCourseDTO struct {
	CourseID          int     `json:"course_id"`
	CourseName        string  `json:"course_name"`
	Cover             string  `json:"cover"`
	SpecialtyID       *int    `json:"specialty_id"`
	LevelID           *int    `json:"level_id"`
	Progress          float64 `json:"progress"`
	CompletedChapters int64   `json:"completed_chapters"`
	TotalChapters     int64   `json:"total_chapters"`
	StudyDuration     int64   `json:"study_duration"`
	LastChapterID     *int    `json:"last_chapter_id"`
	LastChapterTitle  string  `json:"last_chapter_title"`
	LastPosition      int     `json:"last_position"`
	LastStudiedAt     string  `json:"last_studied_at"`
}

// StudentCoursesDTO 我的课程信封（continue_learning 为最后学习时间最新的课程）。
type StudentCoursesDTO struct {
	Courses          []StudentCourseDTO `json:"courses"`
	ContinueLearning *StudentCourseDTO  `json:"continue_learning"`
}

// StudentCourseChapterDTO 单课程章节学习状态。
type StudentCourseChapterDTO struct {
	ChapterID     int     `json:"chapter_id"`
	Title         string  `json:"title"`
	Progress      float64 `json:"progress"`
	VideoPosition int     `json:"video_position"`
	Completed     bool    `json:"completed"`
}

// StudentCourseDetailDTO 单课程学习详情（我的课程条目 + 每章状态）。
type StudentCourseDetailDTO struct {
	StudentCourseDTO
	Chapters []StudentCourseChapterDTO `json:"chapters"`
}

// GetStudentCourses 我的课程列表（按最后学习时间倒序）+ 继续学习 top1。
// 课程级记录驱动（chapter_id IS NULL 一行一课程），课程元信息/章节计数/完成计数/
// 最后章节标题与播放位置全部 batch 回填（batch_backfill 模式，无逐课程 N+1）。
func (s *StudentService) GetStudentCourses(studentID int) (*StudentCoursesDTO, error) {
	type courseRow struct {
		CourseID      int        `gorm:"column:course_id"`
		Progress      float64    `gorm:"column:progress"`
		LastChapterID *int       `gorm:"column:last_chapter_id"`
		LastStudiedAt *time.Time `gorm:"column:last_studied_at"`
	}
	var rows []courseRow
	s.db.Model(&model.StudyRecord{}).
		Select("course_id, progress, last_chapter_id, last_studied_at").
		Where("student_id = ? AND chapter_id IS NULL", studentID).
		Scan(&rows)
	result := &StudentCoursesDTO{Courses: make([]StudentCourseDTO, 0, len(rows))}
	if len(rows) == 0 {
		return result, nil
	}

	courseIDs := make([]int, 0, len(rows))
	for _, r := range rows {
		courseIDs = append(courseIDs, r.CourseID)
	}

	// 课程元信息（名称/封面/方向/等级）；课程已删则跳过（与 queryProfile 同语义）。
	metas := make(map[int]model.Course, len(rows))
	{
		var ms []model.Course
		s.db.Select("course_id, name, cover_image, specialty_id, level_id").
			Where("course_id IN ?", courseIDs).Find(&ms)
		for _, m := range ms {
			metas[m.CourseID] = m
		}
	}
	chapterCounts := batchChapterCounts(s.db, courseIDs)

	// 学习时长（该课程全部记录求和，与 profile 口径一致）。
	durationByCourse := make(map[int]int64, len(rows))
	{
		var ds []struct {
			CourseID int   `gorm:"column:course_id"`
			Total    int64 `gorm:"column:total"`
		}
		s.db.Model(&model.StudyRecord{}).
			Select("course_id, COALESCE(SUM(study_duration), 0) AS total").
			Where("student_id = ? AND course_id IN ?", studentID, courseIDs).
			Group("course_id").Scan(&ds)
		for _, d := range ds {
			durationByCourse[d.CourseID] = d.Total
		}
	}

	// 完成章节数（progress >= 100 单一事实源）。
	completedByCourse := make(map[int]int64, len(rows))
	{
		var ns []struct {
			CourseID int   `gorm:"column:course_id"`
			N        int64 `gorm:"column:n"`
		}
		s.db.Model(&model.StudyRecord{}).
			Select("course_id, COUNT(DISTINCT chapter_id) AS n").
			Where("student_id = ? AND course_id IN ? AND chapter_id IS NOT NULL AND progress >= 100", studentID, courseIDs).
			Group("course_id").Scan(&ns)
		for _, n := range ns {
			completedByCourse[n.CourseID] = n.N
		}
	}

	// 最后章节标题与播放位置（章节级记录，每 student+chapter 唯一）。
	lastChapterIDs := make([]int, 0, len(rows))
	for _, r := range rows {
		if r.LastChapterID != nil {
			lastChapterIDs = append(lastChapterIDs, *r.LastChapterID)
		}
	}
	titleByChapter := make(map[int]string, len(lastChapterIDs))
	posByChapter := make(map[int]int, len(lastChapterIDs))
	if len(lastChapterIDs) > 0 {
		var chs []model.Chapter
		s.db.Select("chapter_id, title").Where("chapter_id IN ?", lastChapterIDs).Find(&chs)
		for _, ch := range chs {
			titleByChapter[ch.ChapterID] = ch.Title
		}
		var prs []struct {
			ChapterID     int `gorm:"column:chapter_id"`
			VideoPosition int `gorm:"column:video_position"`
		}
		s.db.Model(&model.StudyRecord{}).
			Select("chapter_id, video_position").
			Where("student_id = ? AND chapter_id IN ?", studentID, lastChapterIDs).
			Scan(&prs)
		for _, pr := range prs {
			posByChapter[pr.ChapterID] = pr.VideoPosition
		}
	}

	for _, r := range rows {
		meta, ok := metas[r.CourseID]
		if !ok {
			continue
		}
		dto := StudentCourseDTO{
			CourseID:          r.CourseID,
			CourseName:        meta.Name,
			Cover:             meta.CoverImage,
			SpecialtyID:       meta.SpecialtyID,
			LevelID:           meta.LevelID,
			Progress:          r.Progress,
			CompletedChapters: completedByCourse[r.CourseID],
			TotalChapters:     chapterCounts[r.CourseID],
			StudyDuration:     durationByCourse[r.CourseID],
		}
		if r.LastChapterID != nil {
			dto.LastChapterID = r.LastChapterID
			dto.LastChapterTitle = titleByChapter[*r.LastChapterID]
			dto.LastPosition = posByChapter[*r.LastChapterID]
		}
		if r.LastStudiedAt != nil {
			dto.LastStudiedAt = formatISO(*r.LastStudiedAt)
		}
		result.Courses = append(result.Courses, dto)
	}

	// 最后学习时间倒序（formatISO 为 UTC 定长格式，字典序即时间序；无值排后）。
	sort.SliceStable(result.Courses, func(i, j int) bool {
		return result.Courses[i].LastStudiedAt > result.Courses[j].LastStudiedAt
	})
	if len(result.Courses) > 0 {
		top := result.Courses[0]
		result.ContinueLearning = &top
	}
	return result, nil
}

// GetStudentCourseDetail 单课程学习详情（含每章进度/播放位置/完成状态）。
// 共享 loadLearningPosition（课程详情增强同一数据源）。
func (s *StudentService) GetStudentCourseDetail(studentID, courseID int) (*StudentCourseDetailDTO, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	lp := loadLearningPosition(s.db, studentID, courseID)

	var chapters []model.Chapter
	s.db.Where("course_id = ?", courseID).Order("order_num ASC").Find(&chapters)

	type chRow struct {
		ChapterID     int     `gorm:"column:chapter_id"`
		Progress      float64 `gorm:"column:progress"`
		VideoPosition int     `gorm:"column:video_position"`
	}
	var chRows []chRow
	s.db.Model(&model.StudyRecord{}).
		Select("chapter_id, progress, video_position").
		Where("student_id = ? AND course_id = ? AND chapter_id IS NOT NULL", studentID, courseID).
		Scan(&chRows)
	chBy := make(map[int]chRow, len(chRows))
	for _, r := range chRows {
		chBy[r.ChapterID] = r
	}

	var totalDuration int64
	s.db.Model(&model.StudyRecord{}).
		Where("student_id = ? AND course_id = ?", studentID, courseID).
		Select("COALESCE(SUM(study_duration), 0)").Scan(&totalDuration)

	lastTitle := ""
	if lp.LastChapterID != nil {
		var ch model.Chapter
		if err := s.db.Select("title").First(&ch, *lp.LastChapterID).Error; err == nil {
			lastTitle = ch.Title
		}
	}
	lastStudiedAt := ""
	if lp.LastStudiedAt != nil {
		lastStudiedAt = formatISO(*lp.LastStudiedAt)
	}

	detail := &StudentCourseDetailDTO{
		StudentCourseDTO: StudentCourseDTO{
			CourseID:          course.CourseID,
			CourseName:        course.Name,
			Cover:             course.CoverImage,
			SpecialtyID:       course.SpecialtyID,
			LevelID:           course.LevelID,
			Progress:          lp.Progress,
			CompletedChapters: lp.CompletedChapters,
			TotalChapters:     int64(len(chapters)),
			StudyDuration:     totalDuration,
			LastChapterID:     lp.LastChapterID,
			LastChapterTitle:  lastTitle,
			LastPosition:      lp.LastPosition,
			LastStudiedAt:     lastStudiedAt,
		},
		Chapters: make([]StudentCourseChapterDTO, 0, len(chapters)),
	}
	for _, ch := range chapters {
		r, ok := chBy[ch.ChapterID]
		progress := 0.0
		videoPosition := 0
		if ok {
			progress = r.Progress
			videoPosition = r.VideoPosition
		}
		detail.Chapters = append(detail.Chapters, StudentCourseChapterDTO{
			ChapterID:     ch.ChapterID,
			Title:         ch.Title,
			Progress:      progress,
			VideoPosition: videoPosition,
			Completed:     progress >= 100,
		})
	}
	return detail, nil
}
