// Package service 学员侧课程与章节。
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// CourseService 学员课程服务。
type CourseService struct {
	db          *gorm.DB
	fileService *FileService
}

// NewCourseService 创建课程服务实例。
func NewCourseService(db *gorm.DB, fileService *FileService) *CourseService {
	return &CourseService{db: db, fileService: fileService}
}

// GetCourses 课程列表。
func (s *CourseService) GetCourses(page, pageSize int, category string) map[string]any {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 12
	}
	q := s.db.Model(&model.Course{}).Where("status = ?", 1)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	var total int64
	q.Count(&total)
	var courses []model.Course
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&courses)
	items := make([]map[string]any, 0, len(courses))
	for i := range courses {
		items = append(items, courseToDict(&courses[i]))
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return map[string]any{
		"total":   total,
		"page":    page,
		"pages":   pages,
		"courses": items,
	}
}

// GetCourseDetail 课程详情。
func (s *CourseService) GetCourseDetail(courseID, studentID int) (map[string]any, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	var chapters []model.Chapter
	s.db.Where("course_id = ?", courseID).Order("order_num").Find(&chapters)
	chapterList := make([]map[string]any, 0, len(chapters))
	for i := range chapters {
		chapterList = append(chapterList, chapterToDict(&chapters[i]))
	}
	progress := 0.0
	if studentID > 0 {
		var record model.StudyRecord
		// 使用 Limit(1).Find() 而非 First()：学员首次访问课程时无学习记录是正常情况，
		// Find() 在无记录时不返回 ErrRecordNotFound，避免 GORM logger 误报 WARN 日志。
		if err := s.db.Where("student_id = ? AND course_id = ?", studentID, courseID).Limit(1).Find(&record).Error; err == nil && record.RecordID > 0 {
			progress = record.Progress
		}
	}
	return map[string]any{
		"course_info": courseToDict(&course),
		"chapters":    chapterList,
		"progress":    progress,
	}, nil
}

// GetChapterDetail 章节详情。
func (s *CourseService) GetChapterDetail(courseID, chapterID, studentID int) (map[string]any, error) {
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

	result := chapterToDict(&chapter)
	if prevID != 0 {
		result["previous_chapter_id"] = prevID
	} else {
		result["previous_chapter_id"] = nil
	}
	if nextID != 0 {
		result["next_chapter_id"] = nextID
	} else {
		result["next_chapter_id"] = nil
	}
	result["study_status"] = studyStatus

	var files []model.ChapterFile
	s.db.Where("chapter_id = ?", chapterID).Order("created_at").Find(&files)
	fileList := make([]map[string]any, 0, len(files))
	if len(files) == 0 && chapter.FileURL != "" {
		fileList = append(fileList, legacyFileEntry(&chapter))
	} else {
		for i := range files {
			fileList = append(fileList, chapterFileToDict(&files[i]))
		}
	}
	result["files"] = fileList
	return result, nil
}

// GetChapterSlides 章节幻灯片。
// 优先读取 DB 中持久化的 slide_urls；为空则从 PPT 文件下载并触发转图，
// 转图成功后把 URL 列表回写 chapter.slide_urls。
func (s *CourseService) GetChapterSlides(chapterID int) (map[string]any, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}

	// 1. 优先读 DB 持久化的 slide_urls
	if chapter.SlideUrls != "" {
		var urls []string
		if err := json.Unmarshal([]byte(chapter.SlideUrls), &urls); err == nil && len(urls) > 0 {
			return map[string]any{"chapter_id": chapterID, "slides": urls}, nil
		}
	}

	// 2. 查找 PPT 文件 URL
	pptURL := resolveChapterPPTURL(s.db, &chapter, chapterID)
	if pptURL == "" {
		return map[string]any{"chapter_id": chapterID, "slides": []string{}}, nil
	}

	// 3. 下载 PPT 并转图
	slideURLs := s.generateSlides(chapterID, pptURL)
	return map[string]any{"chapter_id": chapterID, "slides": slideURLs}, nil
}

// RegenerateChapterSlides 重新生成幻灯片。
// 总是重新下载 PPT 并转图，覆盖 chapter.slide_urls。
func (s *CourseService) RegenerateChapterSlides(chapterID int) (map[string]any, error) {
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
	return map[string]any{"chapter_id": chapterID, "slides": slideURLs}, nil
}

// generateSlides 下载 PPT bytes 并调 FileService 转图，把 URL 列表持久化到 chapter.slide_urls。
func (s *CourseService) generateSlides(chapterID int, pptURL string) []string {
	if s.fileService == nil {
		return nil
	}
	pptBytes, err := downloadFile(pptURL)
	if err != nil {
		fmt.Printf("[course_service] 下载 PPT 失败 url=%s: %v\n", pptURL, err)
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

// UpdateStudyProgress 更新学习进度。
func (s *CourseService) UpdateStudyProgress(studentID, courseID, chapterID, duration int) (map[string]any, error) {
	var record model.StudyRecord
	err := s.db.Where("student_id = ? AND course_id = ?", studentID, courseID).First(&record).Error

	var totalChapters int64
	s.db.Model(&model.Chapter{}).Where("course_id = ?", courseID).Count(&totalChapters)
	if totalChapters == 0 {
		totalChapters = 1
	}

	if err == nil {
		record.StudyDuration += duration
		var completedChapters int64
		s.db.Model(&model.StudyRecord{}).
			Where("student_id = ? AND course_id = ? AND chapter_id IS NOT NULL", studentID, courseID).
			Distinct("chapter_id").Count(&completedChapters)
		if chapterID > 0 {
			var existing model.StudyRecord
			if e := s.db.Where("student_id = ? AND course_id = ? AND chapter_id = ?", studentID, courseID, chapterID).First(&existing).Error; e != nil {
				// 新章节记录的 study_duration 置 0：duration 已累加到上方 First() 取出的主记录上，
				// 此处仅为统计 completedChapters 创建占位记录，避免 SUM(study_duration) 重复计算
				newRecord := model.StudyRecord{
					StudentID:     studentID,
					CourseID:      courseID,
					ChapterID:     &chapterID,
					StudyDuration: 0,
					Progress:      0,
					StudyDate:     beijingNow(),
				}
				s.db.Create(&newRecord)
				completedChapters++
			}
		}
		record.Progress = roundFloat2(float64(completedChapters) / float64(totalChapters) * 100)
		s.db.Save(&record)
		return map[string]any{
			"record_id":      record.RecordID,
			"progress":       record.Progress,
			"study_duration": record.StudyDuration,
		}, nil
	}

	progress := 0.0
	if chapterID > 0 {
		progress = roundFloat2(1.0 / float64(totalChapters) * 100)
	}
	newRecord := model.StudyRecord{
		StudentID:     studentID,
		CourseID:      courseID,
		ChapterID:     &chapterID,
		StudyDuration: duration,
		Progress:      progress,
		StudyDate:     beijingNow(),
	}
	if err := s.db.Create(&newRecord).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"record_id":      newRecord.RecordID,
		"progress":       newRecord.Progress,
		"study_duration": newRecord.StudyDuration,
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

// ===== dict 辅助 =====

func courseToDict(c *model.Course) map[string]any {
	return map[string]any{
		"course_id":   c.CourseID,
		"name":        c.Name,
		"category":    c.Category,
		"description": c.Description,
		"cover_image": c.CoverImage,
		"duration":    c.Duration,
		"status":      c.Status,
		"created_at":  formatISO(c.CreatedAt),
	}
}

func chapterToDict(c *model.Chapter) map[string]any {
	return map[string]any{
		"chapter_id":   c.ChapterID,
		"course_id":    c.CourseID,
		"title":        c.Title,
		"content":      c.Content,
		"content_url":  c.ContentURL,
		"content_type": c.ContentType,
		"file_url":     c.FileURL,
		"description":  c.Description,
		"duration":     c.Duration,
		"order_num":    c.OrderNum,
		"created_at":   formatISO(c.CreatedAt),
	}
}

func chapterFileToDict(f *model.ChapterFile) map[string]any {
	d := map[string]any{
		"file_id":      f.FileID,
		"file_url":     f.FileURL,
		"file_name":    f.FileName,
		"content_type": f.ContentType,
		"file_size":    f.FileSize,
		"created_at":   formatISO(f.CreatedAt),
	}
	if f.ChapterID != nil {
		d["chapter_id"] = *f.ChapterID
	} else {
		d["chapter_id"] = nil
	}
	return d
}

func legacyFileEntry(ch *model.Chapter) map[string]any {
	fileName := ""
	if ch.FileURL != "" {
		parts := strings.Split(ch.FileURL, "/")
		fileName = parts[len(parts)-1]
	}
	contentType := ch.ContentType
	if contentType == "" {
		contentType = "document"
	}
	return map[string]any{
		"file_id":      0,
		"chapter_id":   ch.ChapterID,
		"file_url":     ch.FileURL,
		"file_name":    fileName,
		"content_type": contentType,
		"file_size":    0,
		"created_at":   formatISO(ch.CreatedAt),
	}
}
