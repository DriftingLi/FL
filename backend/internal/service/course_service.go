// Package service 学员侧课程与章节。
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/response"
)

// CoursePageResult 课程分页结果（学员端/管理端/导师端共用）。
type CoursePageResult struct {
	Courses []CourseDTO `json:"courses"`
	Page    int         `json:"page"`
	Pages   int         `json:"pages"`
	Total   int64       `json:"total"`
}

// ===== DTO（JSON 契约与 B5 前的 map key 逐字一致，前端零改动约束）=====

// CourseDTO 课程。可选字段（chapter_count 等）在未填充路径省略，与旧 map 行为一致。
// 字段声明按 key 字母序，保持与旧 map 序列化的字节序一致。
type CourseDTO struct {
	CertificateName       string                  `json:"certificate_name,omitempty"`
	CertificateTemplate   *CertificateTemplateDTO `json:"certificate_template,omitempty"`
	CertificateTemplateID *int                    `json:"certificate_template_id"`
	ChapterCount          *int64                  `json:"chapter_count,omitempty"`
	CoverImage            string                  `json:"cover_image"`
	CourseID              int                     `json:"course_id"`
	CreatedAt             string                  `json:"created_at"`
	Description           string                  `json:"description"`
	Duration              int                     `json:"duration"`
	Level                 *LevelBriefDTO          `json:"level,omitempty"`
	LevelID               *int                    `json:"level_id"`
	Name                  string                  `json:"name"`
	PracticeHours         int                     `json:"practice_hours"`
	PrerequisiteCourseIDs *[]int                  `json:"prerequisite_course_ids,omitempty"`
	Prerequisites         *[]CourseBriefDTO       `json:"prerequisites,omitempty"`
	SortOrder             int                     `json:"sort_order"`
	Specialty             *SpecialtyBriefDTO      `json:"specialty,omitempty"`
	SpecialtyID           *int                    `json:"specialty_id"`
	Status                int16                   `json:"status"`
	StudentCount          *int64                  `json:"student_count,omitempty"`
	TheoryHours           int                     `json:"theory_hours"`
	Chapters              *[]ChapterDTO           `json:"chapters,omitempty"`
}

// SpecialtyBriefDTO 专业方向简述（详情元数据）。
type SpecialtyBriefDTO struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	SpecialtyID int    `json:"specialty_id"`
}

// LevelBriefDTO 课程等级简述（详情元数据）。
type LevelBriefDTO struct {
	Code    string `json:"code"`
	LevelID int    `json:"level_id"`
	Name    string `json:"name"`
}

// CertificateTemplateDTO 证书模板（详情元数据）。
type CertificateTemplateDTO struct {
	Code         string `json:"code"`
	Description  string `json:"description"`
	ID           int    `json:"id"`
	Name         string `json:"name"`
	TemplateURL  string `json:"template_url"`
	ValidityDays int    `json:"validity_days"`
}

// CourseBriefDTO 前置课程简述（仅 id/name，避免嵌套过深）。
type CourseBriefDTO struct {
	CourseID int    `json:"course_id"`
	Name     string `json:"name"`
}

// ChapterDTO 章节。
type ChapterDTO struct {
	ChapterID   int    `json:"chapter_id"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`

	CourseID    int               `json:"course_id"`
	CreatedAt   string            `json:"created_at"`
	Description string            `json:"description"`
	Duration    int               `json:"duration"`
	FileURL     string            `json:"file_url"`
	OrderNum    int               `json:"order_num"`
	Title       string            `json:"title"`
	Files       *[]ChapterFileDTO `json:"files,omitempty"`
}

// ChapterFileDTO 章节文件（chapter_file 表条目与旧版 chapter.file_url 兼容条目同构）。
type ChapterFileDTO struct {
	ChapterID   *int   `json:"chapter_id"`
	ContentType string `json:"content_type"`
	CreatedAt   string `json:"created_at"`
	FileID      int    `json:"file_id"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	FileURL     string `json:"file_url"`
}

// ChapterDetailDTO 章节详情（含上下章 ID 与文件列表；study_status 仅学员端路径填充）。
type ChapterDetailDTO struct {
	ChapterDTO
	Files             []ChapterFileDTO `json:"files"`
	NextChapterID     *int             `json:"next_chapter_id"`
	PreviousChapterID *int             `json:"previous_chapter_id"`
	StudyStatus       string           `json:"study_status,omitempty"`
}

// CourseDetailDTO 学员端课程详情信封。
type CourseDetailDTO struct {
	CourseInfo CourseDTO    `json:"course_info"`
	Chapters   []ChapterDTO `json:"chapters"`
	Progress   float64      `json:"progress"`
}

// AdminCourseDetailDTO 管理端课程详情（course 字段平铺 + chapters）。
type AdminCourseDetailDTO struct {
	CourseDTO
	Chapters []ChapterDTO `json:"chapters"`
}

// TutorCourseChaptersDTO 导师端课程章节列表信封。
type TutorCourseChaptersDTO struct {
	Course   CourseDTO    `json:"course"`
	Chapters []ChapterDTO `json:"chapters"`
}

// ChapterSlidesDTO 章节幻灯片。
type ChapterSlidesDTO struct {
	ChapterID int      `json:"chapter_id"`
	Slides    []string `json:"slides"`
}

// StudyProgressDTO 学习进度更新结果。
type StudyProgressDTO struct {
	Progress      float64 `json:"progress"`
	RecordID      int     `json:"record_id"`
	StudyDuration int64   `json:"study_duration"`
}

// ===== 课程挂载不变式（唯一事实源）=====
//
// 领域规则：课程必须同时挂专业方向与课程等级才「存在/可见」——
// 学员端列表与目录树只展示已挂载课程；创建/编辑/排序校验同源（ADR-0006 口径）。

// courseMounted 挂载判定。
func courseMounted(specialtyID, levelID *int) bool {
	return specialtyID != nil && levelID != nil
}

// mountedCourseScope 学员端可见课程查询范围：挂载不变式的 SQL 形态。
func mountedCourseScope(q *gorm.DB) *gorm.DB {
	return q.Where("specialty_id IS NOT NULL AND level_id IS NOT NULL")
}

// validateMountedCourseInput 挂载不变式的写入校验：创建必填；编辑携带时不允许清空。
func validateMountedCourseInput(data map[string]any, update bool) error {
	_, hasSpec := data["specialty_id"]
	_, hasLevel := data["level_id"]
	if !update || hasSpec {
		if toInt(data["specialty_id"]) <= 0 {
			return errors.New("专业方向不能为空")
		}
	}
	if !update || hasLevel {
		if toInt(data["level_id"]) <= 0 {
			return errors.New("课程等级不能为空")
		}
	}
	return nil
}

// ===== 课程列表/详情共享实现（学员端/管理端同源）=====

// courseListOptions 列表差异点：学员端仅已挂载+上架，管理端全量（可关键字搜索）。
type courseListOptions struct {
	onlyMounted     bool
	keyword         string
	specialtyID     *int
	levelID         *int
	defaultPageSize int
}

// listCourses 课程列表共享实现（分页归一化、章节数/前置课程回填、信封组装只此一份）。
func listCourses(db *gorm.DB, page, pageSize int, opts courseListOptions) CoursePageResult {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = opts.defaultPageSize
	}
	q := db.Model(&model.Course{})
	if opts.onlyMounted {
		// 挂载不变式 + 上架：学员端可见性口径
		q = q.Where("status = ?", 1)
		q = mountedCourseScope(q)
	}
	if opts.keyword != "" {
		q = q.Where("name LIKE ?", "%"+opts.keyword+"%")
	}
	if opts.specialtyID != nil {
		q = q.Where("specialty_id = ?", *opts.specialtyID)
	}
	if opts.levelID != nil {
		q = q.Where("level_id = ?", *opts.levelID)
	}
	var total int64
	q.Count(&total)
	var courses []model.Course
	q.Order("sort_order ASC, created_at DESC, course_id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&courses)

	// 证书模板数量少，一次加载映射（学员端路径附带证书名，避免逐课程 N+1）
	var certNameByID map[int]string
	if opts.onlyMounted {
		var certs []model.CertificateTemplate
		db.Find(&certs)
		certNameByID = make(map[int]string, len(certs))
		for i := range certs {
			certNameByID[certs[i].ID] = certs[i].Name
		}
	}
	items := make([]CourseDTO, 0, len(courses))
	for i := range courses {
		item := courseToDTO(&courses[i])
		fillChapterCount(db, courses[i].CourseID, &item)
		fillPrereqIDs(db, courses[i].CourseID, &item)
		if opts.onlyMounted {
			if id := courses[i].CertificateTemplateID; id != nil {
				item.CertificateName = certNameByID[*id]
			}
		}
		items = append(items, item)
	}
	return CoursePageResult{
		Courses: items,
		Page:    page,
		Pages:   response.PageCount(total, pageSize),
		Total:   total,
	}
}

// loadCourseWithChapters 课程 + 章节列表共享装载（学员端/管理端详情同源）。
func loadCourseWithChapters(db *gorm.DB, courseID int) (*model.Course, []ChapterDTO, error) {
	var course model.Course
	if err := db.First(&course, courseID).Error; err != nil {
		return nil, nil, errors.New("课程不存在")
	}
	var chapters []model.Chapter
	db.Where("course_id = ?", courseID).Order("order_num").Find(&chapters)
	chapterList := make([]ChapterDTO, 0, len(chapters))
	for i := range chapters {
		chapterList = append(chapterList, chapterToDTO(&chapters[i]))
	}
	return &course, chapterList, nil
}

// CourseService 学员课程服务。
type CourseService struct {
	db          *gorm.DB
	fileService *FileService
	logger      *zap.Logger
}

// NewCourseService 创建课程服务实例。
func NewCourseService(db *gorm.DB, fileService *FileService, logger *zap.Logger) *CourseService {
	return &CourseService{db: db, fileService: fileService, logger: logger}
}

// GetCourses 课程列表（可额外按专业方向/课程等级过滤）。
// 未挂专业方向/等级的课程不展示（与目录树口径统一，见挂载不变式）。
func (s *CourseService) GetCourses(page, pageSize int, specialtyID, levelID *int) CoursePageResult {
	return listCourses(s.db, page, pageSize, courseListOptions{
		onlyMounted: true, specialtyID: specialtyID, levelID: levelID, defaultPageSize: 12,
	})
}

// GetCourseDetail 课程详情。
func (s *CourseService) GetCourseDetail(courseID, studentID int) (*CourseDetailDTO, error) {
	course, chapterList, err := loadCourseWithChapters(s.db, courseID)
	if err != nil {
		return nil, err
	}
	progress := 0.0
	if studentID > 0 {
		var record model.StudyRecord
		// 优先取课程级记录（chapter_id IS NULL）；历史数据没有 NULL 记录时回退到任意一条记录。
		if err := s.db.Where("student_id = ? AND course_id = ? AND chapter_id IS NULL", studentID, courseID).
			Order("record_id ASC").Limit(1).Find(&record).Error; err == nil && record.RecordID == 0 {
			s.db.Where("student_id = ? AND course_id = ?", studentID, courseID).
				Order("record_id ASC").Limit(1).Find(&record)
		}
		if record.RecordID > 0 {
			progress = record.Progress
		}
	}
	detail := courseToDTO(course)
	fillChapterCount(s.db, course.CourseID, &detail)
	fillCourseMeta(s.db, course, &detail)
	return &CourseDetailDTO{
		CourseInfo: detail,
		Chapters:   chapterList,
		Progress:   progress,
	}, nil
}

// GetChapterDetail 章节详情。
func (s *CourseService) GetChapterDetail(courseID, chapterID, studentID int) (*ChapterDetailDTO, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	if chapter.CourseID != courseID {
		return nil, errors.New("章节不属于该课程")
	}
	var chapters []model.Chapter
	s.db.Where("course_id = ?", courseID).Order("order_num").Find(&chapters)

	prevID, nextID := 0, 0
	for i, ch := range chapters {
		if ch.ChapterID == chapterID {
			if i > 0 {
				prevID = chapters[i-1].ChapterID
			}
			if i < len(chapters)-1 {
				nextID = chapters[i+1].ChapterID
			}
			break
		}
	}

	studyStatus := "not_started"
	if studentID > 0 {
		var record model.StudyRecord
		// 同上：使用 Limit(1).Find() 避免首次访问章节时 GORM 误报 record not found。
		if err := s.db.Where("student_id = ? AND course_id = ? AND chapter_id = ?", studentID, courseID, chapterID).Limit(1).Find(&record).Error; err == nil && record.RecordID > 0 {
			if record.Progress >= 100 {
				studyStatus = "completed"
			} else {
				studyStatus = "studying"
			}
		}
	}

	result := chapterToDTO(&chapter)
	var prevIDPtr, nextIDPtr *int
	if prevID != 0 {
		prevIDPtr = &prevID
	}
	if nextID != 0 {
		nextIDPtr = &nextID
	}

	var files []model.ChapterFile
	s.db.Where("chapter_id = ?", chapterID).Order("created_at").Find(&files)
	fileList := make([]ChapterFileDTO, 0, len(files))
	if len(files) == 0 && chapter.FileURL != "" {
		fileList = append(fileList, legacyFileEntry(&chapter))
	} else {
		for i := range files {
			fileList = append(fileList, chapterFileToDTO(&files[i]))
		}
	}
	return &ChapterDetailDTO{
		ChapterDTO:        result,
		Files:             fileList,
		PreviousChapterID: prevIDPtr,
		NextChapterID:     nextIDPtr,
		StudyStatus:       studyStatus,
	}, nil
}

// GetChapterSlides 章节幻灯片。
// 优先读取 DB 中持久化的 slide_urls；为空则从 PPT 文件下载并触发转图，
// 转图成功后把 URL 列表回写 chapter.slide_urls。
func (s *CourseService) GetChapterSlides(chapterID int) (*ChapterSlidesDTO, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}

	// 1. 优先读 DB 持久化的 slide_urls
	if chapter.SlideUrls != "" {
		var urls []string
		if err := json.Unmarshal([]byte(chapter.SlideUrls), &urls); err == nil && len(urls) > 0 {
			return &ChapterSlidesDTO{ChapterID: chapterID, Slides: urls}, nil
		}
	}

	// 2. 查找 PPT 文件 URL
	pptURL := resolveChapterPPTURL(s.db, &chapter, chapterID)
	if pptURL == "" {
		return &ChapterSlidesDTO{ChapterID: chapterID, Slides: []string{}}, nil
	}

	// 3. 下载 PPT 并转图
	slideURLs := s.generateSlides(chapterID, pptURL)
	return &ChapterSlidesDTO{ChapterID: chapterID, Slides: slideURLs}, nil
}

// RegenerateChapterSlides 重新生成幻灯片。
// 总是重新下载 PPT 并转图，覆盖 chapter.slide_urls。
func (s *CourseService) RegenerateChapterSlides(chapterID int) (*ChapterSlidesDTO, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	pptURL := resolveChapterPPTURL(s.db, &chapter, chapterID)
	if pptURL == "" {
		return nil, errors.New("该章节没有PPT文件")
	}
	slideURLs := s.generateSlides(chapterID, pptURL)
	if len(slideURLs) == 0 {
		return nil, errors.New("PPT转图失败，请检查文件是否损坏")
	}
	return &ChapterSlidesDTO{ChapterID: chapterID, Slides: slideURLs}, nil
}

// generateSlides 下载 PPT bytes 并调 FileService 转图，把 URL 列表持久化到 chapter.slide_urls。
func (s *CourseService) generateSlides(chapterID int, pptURL string) []string {
	if s.fileService == nil {
		return nil
	}
	pptBytes, err := downloadFile(pptURL)
	if err != nil {
		s.logger.Error("下载 PPT 失败", zap.String("url", pptURL), zap.Error(err))
		return nil
	}
	slideURLs := s.fileService.ConvertPPTToImages(pptBytes, chapterID)
	if len(slideURLs) > 0 {
		slideURLsJSON, _ := json.Marshal(slideURLs)
		s.db.Model(&model.Chapter{}).Where("chapter_id = ?", chapterID).Update("slide_urls", string(slideURLsJSON))
	}
	return slideURLs
}

// resolveChapterPPTURL 查找章节的 PPT 文件 URL（优先 chapter_file 表，其次 chapter.file_url）。
func resolveChapterPPTURL(db *gorm.DB, chapter *model.Chapter, chapterID int) string {
	var pptFile model.ChapterFile
	if err := db.Where("chapter_id = ? AND content_type = ?", chapterID, "ppt").First(&pptFile).Error; err == nil {
		return pptFile.FileURL
	}
	if chapter.ContentType == "ppt" && chapter.FileURL != "" {
		return chapter.FileURL
	}
	return ""
}

// downloadFile 通过 HTTP GET 下载文件内容（R2 公开访问 URL 可直接下载）。
func downloadFile(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败,状态码: %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// UpdateStudyProgress 更新学习进度（按学习时长自动完成章节，无需手动点击"完成"）。
// 每次上报把 duration（分钟）累加到对应章节记录；当章节累计学习时长达到章节时长
// （chapter.duration 分钟；未设置时按 1 分钟）自动标记完成（progress=100）。
// 课程级进度 = 已完成章节数 / 总章节数。
func (s *CourseService) UpdateStudyProgress(studentID, courseID, chapterID, duration int) (*StudyProgressDTO, error) {
	var totalChapters int64
	s.db.Model(&model.Chapter{}).Where("course_id = ?", courseID).Count(&totalChapters)
	if totalChapters == 0 {
		totalChapters = 1
	}

	// 1. 课程级记录（chapter_id IS NULL）：承载课程学习时长与课程进度。
	// 历史数据中不存在 NULL 记录时，从旧主记录（首条任意章节记录）继承时长，避免学习时长丢失。
	var record model.StudyRecord
	if err := s.db.Where("student_id = ? AND course_id = ? AND chapter_id IS NULL", studentID, courseID).
		Order("record_id ASC").Limit(1).Find(&record).Error; err == nil && record.RecordID == 0 {
		var legacy model.StudyRecord
		if e := s.db.Where("student_id = ? AND course_id = ?", studentID, courseID).
			Order("record_id ASC").Limit(1).Find(&legacy).Error; e == nil && legacy.RecordID > 0 {
			record.StudyDuration = legacy.StudyDuration
		}
		record = model.StudyRecord{
			StudentID:     studentID,
			CourseID:      courseID,
			StudyDuration: record.StudyDuration,
			Progress:      0,
			StudyDate:     beijingNow(),
		}
		if err := s.db.Create(&record).Error; err != nil {
			return nil, err
		}
	}

	// 2. 章节级记录：累加学习时长，达到阈值自动标记完成。
	if chapterID > 0 {
		threshold := 1
		var chapter model.Chapter
		if err := s.db.Select("duration").First(&chapter, chapterID).Error; err == nil && chapter.Duration > 0 {
			threshold = chapter.Duration
		}
		var ch model.StudyRecord
		if e := s.db.Where("student_id = ? AND course_id = ? AND chapter_id = ?", studentID, courseID, chapterID).
			Order("record_id ASC").Limit(1).Find(&ch).Error; e == nil && ch.RecordID > 0 {
			ch.StudyDuration += duration
			ch.StudyDate = beijingNow()
			updates := map[string]any{
				"study_duration": ch.StudyDuration,
				"study_date":     ch.StudyDate,
			}
			if ch.StudyDuration >= threshold {
				updates["progress"] = 100
			}
			if err := s.db.Model(&model.StudyRecord{}).Where("record_id = ?", ch.RecordID).
				Updates(updates).Error; err != nil {
				return nil, err
			}
		} else {
			chProgress := 0.0
			if duration >= threshold {
				chProgress = 100
			}
			newChapter := model.StudyRecord{
				StudentID:     studentID,
				CourseID:      courseID,
				ChapterID:     &chapterID,
				StudyDuration: duration,
				Progress:      chProgress,
				StudyDate:     beijingNow(),
			}
			if err := s.db.Create(&newChapter).Error; err != nil {
				return nil, err
			}
		}
	}

	// 3. 重算课程进度：统计 progress >= 100 的不同章节数。
	var completedChapters int64
	s.db.Model(&model.StudyRecord{}).
		Where("student_id = ? AND course_id = ? AND chapter_id IS NOT NULL AND progress >= 100", studentID, courseID).
		Distinct("chapter_id").Count(&completedChapters)

	record.Progress = roundFloat2(float64(completedChapters) / float64(totalChapters) * 100)
	if err := s.db.Save(&record).Error; err != nil {
		return nil, err
	}
	// 总学习时长 = 课程级历史时长 + 各章节累计时长（避免重复统计）
	var totalDuration int64
	s.db.Model(&model.StudyRecord{}).
		Where("student_id = ? AND course_id = ?", studentID, courseID).
		Select("COALESCE(SUM(study_duration), 0)").Scan(&totalDuration)
	return &StudyProgressDTO{
		RecordID:      record.RecordID,
		Progress:      record.Progress,
		StudyDuration: totalDuration,
	}, nil
}

// ===== 辅助 =====

// roundFloat2 保留 2 位小数。
func roundFloat2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// roundFloat1 保留 1 位小数。
func roundFloat1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}

// ===== DTO 构造（原 courseToDict/chapterToDict/chapterFileToDict/legacyFileEntry 折叠入内）=====

func courseToDTO(c *model.Course) CourseDTO {
	return CourseDTO{
		CourseID:              c.CourseID,
		Name:                  c.Name,
		Description:           c.Description,
		CoverImage:            c.CoverImage,
		Duration:              c.Duration,
		SpecialtyID:           c.SpecialtyID,
		LevelID:               c.LevelID,
		TheoryHours:           c.TheoryHours,
		PracticeHours:         c.PracticeHours,
		CertificateTemplateID: c.CertificateTemplateID,
		SortOrder:             c.SortOrder,
		Status:                c.Status,
		CreatedAt:             formatISO(c.CreatedAt),
	}
}

// fillChapterCount 填充课程章节数。
func fillChapterCount(db *gorm.DB, courseID int, dto *CourseDTO) {
	var count int64
	db.Model(&model.Chapter{}).Where("course_id = ?", courseID).Count(&count)
	dto.ChapterCount = &count
}

// fillPrereqIDs 填充前置课程 ID 列表（供编辑表单回填，避免前端提交空数组清空关联）。
// 无前置课程时填空数组（与旧 map 行为一致：[] 而非 null；omitempty 对空切片也省略，
// 故用指针区分"未填充"与"空数组"两种状态）。
func fillPrereqIDs(db *gorm.DB, courseID int, dto *CourseDTO) {
	var ids []int
	db.Model(&model.CoursePrerequisite{}).Where("course_id = ?", courseID).
		Order("prerequisite_course_id ASC").Pluck("prerequisite_course_id", &ids)
	if ids == nil {
		ids = []int{}
	}
	dto.PrerequisiteCourseIDs = &ids
}

func chapterToDTO(c *model.Chapter) ChapterDTO {
	return ChapterDTO{
		ChapterID:   c.ChapterID,
		CourseID:    c.CourseID,
		Title:       c.Title,
		Content:     c.Content,
		ContentType: c.ContentType,
		FileURL:     c.FileURL,
		Description: c.Description,
		Duration:    c.Duration,
		OrderNum:    c.OrderNum,
		CreatedAt:   formatISO(c.CreatedAt),
	}
}

func chapterFileToDTO(f *model.ChapterFile) ChapterFileDTO {
	return ChapterFileDTO{
		FileID:      f.FileID,
		ChapterID:   f.ChapterID,
		FileURL:     f.FileURL,
		FileName:    f.FileName,
		ContentType: f.ContentType,
		FileSize:    f.FileSize,
		CreatedAt:   formatISO(f.CreatedAt),
	}
}

func legacyFileEntry(ch *model.Chapter) ChapterFileDTO {
	fileName := ""
	if ch.FileURL != "" {
		parts := strings.Split(ch.FileURL, "/")
		fileName = parts[len(parts)-1]
	}
	contentType := ch.ContentType
	if contentType == "" {
		contentType = "document"
	}
	return ChapterFileDTO{
		FileID:      0,
		ChapterID:   &ch.ChapterID,
		FileURL:     ch.FileURL,
		FileName:    fileName,
		ContentType: contentType,
		FileSize:    0,
		CreatedAt:   formatISO(ch.CreatedAt),
	}
}

// ===== 培训目录扩展辅助（课程等级/学时/前置课程/证书模板） =====

// applyCourseTrainingFields 应用课程培训扩展字段（专业方向/等级/学时/证书模板）。
// specialty_id / level_id / certificate_template_id 传 0 或空表示清空（置 NULL）。
func applyCourseTrainingFields(db *gorm.DB, course *model.Course, data map[string]any) error {
	if v, ok := data["specialty_id"]; ok {
		id := toInt(v)
		if id < 0 {
			return errors.New("专业方向ID无效")
		}
		if id == 0 {
			course.SpecialtyID = nil
		} else {
			var count int64
			if err := db.Model(&model.Specialty{}).Where("specialty_id = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errors.New("专业方向不存在")
			}
			course.SpecialtyID = ptrInt(id)
		}
	}
	if v, ok := data["level_id"]; ok {
		id := toInt(v)
		if id < 0 {
			return errors.New("课程等级ID无效")
		}
		if id == 0 {
			course.LevelID = nil
		} else {
			var count int64
			if err := db.Model(&model.CourseLevel{}).Where("level_id = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errors.New("课程等级不存在")
			}
			course.LevelID = ptrInt(id)
		}
	}
	if v, ok := data["certificate_template_id"]; ok {
		id := toInt(v)
		if id < 0 {
			return errors.New("证书模板ID无效")
		}
		if id == 0 {
			course.CertificateTemplateID = nil
		} else {
			var count int64
			if err := db.Model(&model.CertificateTemplate{}).Where("id = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errors.New("证书模板不存在")
			}
			course.CertificateTemplateID = ptrInt(id)
		}
	}
	if v, ok := data["theory_hours"]; ok {
		hours := toInt(v)
		if hours < 0 {
			return errors.New("理论学时不能为负数")
		}
		course.TheoryHours = hours
	}
	if v, ok := data["practice_hours"]; ok {
		hours := toInt(v)
		if hours < 0 {
			return errors.New("实操学时不能为负数")
		}
		course.PracticeHours = hours
	}
	if v, ok := data["sort_order"]; ok {
		order := toInt(v)
		if order < 0 {
			return errors.New("课程排序值不能为负数")
		}
		course.SortOrder = order
	}
	return nil
}

// prerequisiteIDsFromData 提取前置课程 ID 列表。
func prerequisiteIDsFromData(data map[string]any) []int {
	v, ok := data["prerequisite_course_ids"]
	if !ok {
		return nil
	}
	return toIntSlice(v)
}

// toIntSlice 将任意值转为 int 切片（支持 []any / []float64 等 JSON 解码结果）。
func toIntSlice(v any) []int {
	switch vals := v.(type) {
	case []any:
		out := make([]int, 0, len(vals))
		for _, item := range vals {
			out = append(out, toInt(item))
		}
		return out
	case []float64:
		out := make([]int, 0, len(vals))
		for _, item := range vals {
			out = append(out, int(item))
		}
		return out
	case []int:
		return vals
	}
	return nil
}

// replaceCoursePrerequisites 全量替换课程前置课程关联。
func replaceCoursePrerequisites(db *gorm.DB, courseID int, prereqIDs []int) error {
	prereqIDs = dedupeInts(prereqIDs)
	// 校验前置课程存在且不能指向自己
	for _, id := range prereqIDs {
		if id == courseID {
			return errors.New("课程不能设置为自己的前置课程")
		}
		var count int64
		if err := db.Model(&model.Course{}).Where("course_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.New("前置课程不存在")
		}
	}
	if err := checkPrerequisiteCycle(db, courseID, prereqIDs); err != nil {
		return err
	}
	if len(prereqIDs) == 0 {
		return db.Where("course_id = ?", courseID).Delete(&model.CoursePrerequisite{}).Error
	}
	rels := make([]model.CoursePrerequisite, 0, len(prereqIDs))
	for _, id := range prereqIDs {
		rels = append(rels, model.CoursePrerequisite{
			CourseID:             courseID,
			PrerequisiteCourseID: id,
			CreatedAt:            beijingNow(),
		})
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("course_id = ?", courseID).Delete(&model.CoursePrerequisite{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&rels).Error; err != nil {
			return err
		}
		return nil
	})
}

// checkPrerequisiteCycle 检测前置课程依赖是否成环（A→B→A 等）。
// 从每个新前置课程出发，沿"它的前置课程"向下游遍历，若能回到 courseID 则成环；
// visited 防止既有数据中已存在的环导致死循环。
func checkPrerequisiteCycle(db *gorm.DB, courseID int, prereqIDs []int) error {
	if len(prereqIDs) == 0 {
		return nil
	}
	visited := make(map[int]bool, len(prereqIDs))
	stack := make([]int, 0, len(prereqIDs))
	stack = append(stack, prereqIDs...)
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == courseID {
			return errors.New("前置课程关系存在循环依赖")
		}
		if visited[cur] {
			continue
		}
		visited[cur] = true
		var next []int
		if err := db.Model(&model.CoursePrerequisite{}).
			Where("course_id = ?", cur).
			Pluck("prerequisite_course_id", &next).Error; err != nil {
			return err
		}
		stack = append(stack, next...)
	}
	return nil
}

// fillCourseMeta 填充课程详情的扩展元数据：
// specialty / level / certificate_template / prerequisites（前置课程列表）。
func fillCourseMeta(db *gorm.DB, course *model.Course, dto *CourseDTO) {
	if course.SpecialtyID != nil {
		var spec model.Specialty
		if err := db.First(&spec, *course.SpecialtyID).Error; err == nil {
			dto.Specialty = &SpecialtyBriefDTO{
				SpecialtyID: spec.SpecialtyID,
				Code:        spec.Code,
				Name:        spec.Name,
			}
		}
	}
	if course.LevelID != nil {
		var level model.CourseLevel
		if err := db.First(&level, *course.LevelID).Error; err == nil {
			dto.Level = &LevelBriefDTO{
				LevelID: level.LevelID,
				Code:    level.Code,
				Name:    level.Name,
			}
		}
	}
	if course.CertificateTemplateID != nil {
		var tpl model.CertificateTemplate
		if err := db.First(&tpl, *course.CertificateTemplateID).Error; err == nil {
			dto.CertificateTemplate = &CertificateTemplateDTO{
				ID:           tpl.ID,
				Code:         tpl.Code,
				Name:         tpl.Name,
				Description:  tpl.Description,
				ValidityDays: tpl.ValidityDays,
				TemplateURL:  tpl.TemplateURL,
			}
		}
	}
	// 前置课程（仅返回 id/name，避免嵌套过深）
	type prereqRow struct {
		CourseID int
		Name     string
	}
	var prereqs []prereqRow
	db.Table("course_prerequisite AS cp").
		Select("cp.prerequisite_course_id AS course_id, c.name").
		Joins("JOIN course AS c ON c.course_id = cp.prerequisite_course_id").
		Where("cp.course_id = ?", course.CourseID).
		Order("c.created_at DESC, c.course_id ASC").
		Scan(&prereqs)
	prereqList := make([]CourseBriefDTO, 0, len(prereqs))
	prereqIDList := make([]int, 0, len(prereqs))
	for _, p := range prereqs {
		prereqList = append(prereqList, CourseBriefDTO(p))
		prereqIDList = append(prereqIDList, p.CourseID)
	}
	dto.Prerequisites = &prereqList
	dto.PrerequisiteCourseIDs = &prereqIDList
}
