// Package service 学员侧课程与章节，对应 Python course_service。
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"forklift-training/internal/cache"
	"forklift-training/internal/model"
)

// CourseService 学员课程服务。
type CourseService struct {
	db           *gorm.DB
	uploadFolder string
	fileService  *FileService
}

// NewCourseService 创建课程服务实例。
func NewCourseService(db *gorm.DB, uploadFolder string, fileService *FileService) *CourseService {
	return &CourseService{db: db, uploadFolder: uploadFolder, fileService: fileService}
}

// GetCourses 课程列表，对应 Python get_courses。
func (s *CourseService) GetCourses(page, pageSize int, category string) map[string]interface{} {
	cacheKey := cache.SafeKey("course", "list", category, fmt.Sprintf("%d", page), fmt.Sprintf("%d", pageSize))
	var result map[string]interface{}
	err := cache.GetOrSetJSON(context.Background(), cacheKey, cache.TTLStats, &result, func() (interface{}, error) {
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
		items := make([]map[string]interface{}, 0, len(courses))
		for i := range courses {
			items = append(items, courseToDict(&courses[i]))
		}
		pages := int((total + int64(pageSize) - 1) / int64(pageSize))
		return map[string]interface{}{
			"total":   total,
			"page":    page,
			"pages":   pages,
			"courses": items,
		}, nil
	})
	if err != nil {
		// 降级：缓存失败时返回空结果
		return map[string]interface{}{"courses": []interface{}{}, "total": 0}
	}
	return result
}

// GetCourseDetail 课程详情，对应 Python get_course_detail。
func (s *CourseService) GetCourseDetail(courseID, studentID int) (map[string]interface{}, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	var chapters []model.Chapter
	s.db.Where("course_id = ?", courseID).Order("order_num").Find(&chapters)
	chapterList := make([]map[string]interface{}, 0, len(chapters))
	for i := range chapters {
		chapterList = append(chapterList, chapterToDict(&chapters[i]))
	}
	progress := 0.0
	if studentID > 0 {
		var record model.StudyRecord
		if err := s.db.Where("student_id = ? AND course_id = ?", studentID, courseID).First(&record).Error; err == nil {
			progress = record.Progress
		}
	}
	return map[string]interface{}{
		"course_info": courseToDict(&course),
		"chapters":    chapterList,
		"progress":    progress,
	}, nil
}

// GetChapterDetail 章节详情，对应 Python get_chapter_detail。
func (s *CourseService) GetChapterDetail(courseID, chapterID, studentID int) (map[string]interface{}, error) {
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
		if err := s.db.Where("student_id = ? AND course_id = ? AND chapter_id = ?", studentID, courseID, chapterID).First(&record).Error; err == nil {
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
	fileList := make([]map[string]interface{}, 0, len(files))
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

// GetChapterSlides 章节幻灯片，对应 Python get_chapter_slides。
func (s *CourseService) GetChapterSlides(chapterID int) (map[string]interface{}, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	hasPPT := chapter.ContentType == "ppt"
	if !hasPPT {
		var f model.ChapterFile
		if err := s.db.Where("chapter_id = ? AND content_type = ?", chapterID, "ppt").First(&f).Error; err == nil {
			hasPPT = true
		}
	}
	if !hasPPT {
		return map[string]interface{}{"chapter_id": chapterID, "slides": []string{}}, nil
	}

	slidesDir := filepath.Join(s.uploadFolder, "slides", strconv.Itoa(chapterID))
	existingImages := listSlideImages(slidesDir)
	if len(existingImages) == 0 {
		var pptFile model.ChapterFile
		pptURL := ""
		if err := s.db.Where("chapter_id = ? AND content_type = ?", chapterID, "ppt").First(&pptFile).Error; err == nil {
			pptURL = pptFile.FileURL
		}
		if pptURL == "" {
			pptURL = chapter.FileURL
		}
		if pptURL != "" {
			pptPath := s.resolvePPTPath(pptURL)
			if _, err := os.Stat(pptPath); err == nil {
				if s.fileService != nil {
					slideURLs := s.fileService.ConvertPPTToImages(pptPath, slidesDir)
					return map[string]interface{}{"chapter_id": chapterID, "slides": slideURLs}, nil
				}
			}
		}
	}
	urls := make([]string, 0, len(existingImages))
	for _, img := range existingImages {
		urls = append(urls, fmt.Sprintf("/static/uploads/slides/%d/%s", chapterID, img))
	}
	return map[string]interface{}{"chapter_id": chapterID, "slides": urls}, nil
}

// RegenerateChapterSlides 重新生成幻灯片，对应 Python regenerate_chapter_slides。
func (s *CourseService) RegenerateChapterSlides(chapterID int) (map[string]interface{}, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	var pptFile model.ChapterFile
	pptURL := ""
	if err := s.db.Where("chapter_id = ? AND content_type = ?", chapterID, "ppt").First(&pptFile).Error; err == nil {
		pptURL = pptFile.FileURL
	}
	if pptURL == "" {
		pptURL = chapter.FileURL
	}
	if pptURL == "" {
		return nil, errors.New("该章节没有PPT文件")
	}
	pptPath := s.resolvePPTPath(pptURL)
	if _, err := os.Stat(pptPath); err != nil {
		return nil, errors.New("PPT文件不存在，请重新上传")
	}
	slidesDir := filepath.Join(s.uploadFolder, "slides", strconv.Itoa(chapterID))
	_ = os.RemoveAll(slidesDir)
	if s.fileService == nil {
		return nil, errors.New("文件服务不可用")
	}
	slideURLs := s.fileService.ConvertPPTToImages(pptPath, slidesDir)
	return map[string]interface{}{"chapter_id": chapterID, "slides": slideURLs}, nil
}

// UpdateStudyProgress 更新学习进度，对应 Python update_study_progress。
func (s *CourseService) UpdateStudyProgress(studentID, courseID, chapterID, duration int) (map[string]interface{}, error) {
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
				newRecord := model.StudyRecord{
					StudentID:     studentID,
					CourseID:      courseID,
					ChapterID:     &chapterID,
					StudyDuration: duration,
					Progress:      0,
					StudyDate:     beijingNow(),
				}
				s.db.Create(&newRecord)
				completedChapters++
			}
		}
		record.Progress = roundFloat2(float64(completedChapters) / float64(totalChapters) * 100)
		s.db.Save(&record)
		_ = cache.InvalidatePattern(context.Background(), cache.SafeKey("student", "profile", fmt.Sprintf("%d", studentID)))
		_ = cache.InvalidatePattern(context.Background(), "student:stats:"+fmt.Sprintf("%d", studentID)+":*")
		_ = cache.InvalidatePattern(context.Background(), "admin:stats")
		return map[string]interface{}{
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
	_ = cache.InvalidatePattern(context.Background(), cache.SafeKey("student", "profile", fmt.Sprintf("%d", studentID)))
	_ = cache.InvalidatePattern(context.Background(), "student:stats:"+fmt.Sprintf("%d", studentID)+":*")
	_ = cache.InvalidatePattern(context.Background(), "admin:stats")
	return map[string]interface{}{
		"record_id":      newRecord.RecordID,
		"progress":       newRecord.Progress,
		"study_duration": newRecord.StudyDuration,
	}, nil
}

// ===== 辅助 =====

func (s *CourseService) resolvePPTPath(pptURL string) string {
	relative := strings.TrimPrefix(pptURL, "/static/uploads/")
	return filepath.Join(s.uploadFolder, relative)
}

func listSlideImages(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var images []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".png") {
			images = append(images, e.Name())
		}
	}
	sort.Slice(images, func(i, j int) bool {
		return naturalSortKey(images[i]) < naturalSortKey(images[j])
	})
	return images
}

var numberRe = regexp.MustCompile(`(\d+)`)

func naturalSortKey(s string) string {
	parts := numberRe.FindAllString(s, -1)
	for i := range parts {
		if _, err := strconv.Atoi(parts[i]); err == nil {
			parts[i] = fmt.Sprintf("%08d", atoiSafe(parts[i]))
		}
	}
	return strings.Join(parts, "")
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// roundFloat2 保留 2 位小数。
func roundFloat2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// roundFloat1 保留 1 位小数。
func roundFloat1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}

// ===== dict 辅助 =====

func courseToDict(c *model.Course) map[string]interface{} {
	return map[string]interface{}{
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

func chapterToDict(c *model.Chapter) map[string]interface{} {
	return map[string]interface{}{
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

func chapterFileToDict(f *model.ChapterFile) map[string]interface{} {
	d := map[string]interface{}{
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

func legacyFileEntry(ch *model.Chapter) map[string]interface{} {
	fileName := ""
	if ch.FileURL != "" {
		parts := strings.Split(ch.FileURL, "/")
		fileName = parts[len(parts)-1]
	}
	contentType := ch.ContentType
	if contentType == "" {
		contentType = "document"
	}
	return map[string]interface{}{
		"file_id":      0,
		"chapter_id":   ch.ChapterID,
		"file_url":     ch.FileURL,
		"file_name":    fileName,
		"content_type": contentType,
		"file_size":    0,
		"created_at":   formatISO(ch.CreatedAt),
	}
}
