// Package service 导师端课程与文件管理，对应 Python tutor_service。
package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// TutorService 导师服务。
type TutorService struct {
	db           *gorm.DB
	uploadFolder string
	fileService  *FileService
}

// NewTutorService 创建导师服务实例。
func NewTutorService(db *gorm.DB, uploadFolder string, fileService *FileService) *TutorService {
	return &TutorService{db: db, uploadFolder: uploadFolder, fileService: fileService}
}

// GetCourses 导师课程列表，对应 Python get_tutor_courses。
func (s *TutorService) GetCourses(tutorID *int, page, pageSize int) map[string]interface{} {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := s.db.Model(&model.Course{}).Where("status = ?", 1)
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
	}
}

// GetGradingStats 阅卷统计（按天分组），用于导师仪表盘图表。
// 统计当前导师 grader_id 命中的 exam_answer 行数（即导师本人批阅题数）。
// days 仅允许 7 或 30，其他值统一回退为 7。
func (s *TutorService) GetGradingStats(tutorID, days int) map[string]interface{} {
	if days != 7 && days != 30 {
		days = 7
	}

	// 计算最近 days 天的起止时间（北京时间）
	end := beijingNow()
	startOfDay := end.Add(-time.Duration(end.Hour()) * time.Hour).
		Add(-time.Duration(end.Minute()) * time.Minute).
		Add(-time.Duration(end.Second()) * time.Second).
		Add(-time.Duration(end.Nanosecond()) * time.Nanosecond)
	start := startOfDay.AddDate(0, 0, -(days - 1))

	// 按天聚合当前导师已批阅题数
	type dailyRow struct {
		Day   string
		Count int64
	}
	var rows []dailyRow
	s.db.Model(&model.ExamAnswer{}).
		Select("TO_CHAR(graded_at, 'YYYY-MM-DD') as day, COUNT(*) as count").
		Where("grader_id = ? AND graded_at IS NOT NULL AND graded_at >= ?", tutorID, start).
		Group("day").
		Order("day ASC").
		Scan(&rows)

	// 构建日期 -> 题数映射
	countByDay := make(map[string]int64, len(rows))
	var totalCount int64
	for _, r := range rows {
		countByDay[r.Day] = r.Count
		totalCount += r.Count
	}

	// 生成最近 days 天的完整序列（含无批阅记录的天，补 0）
	// start 由 beijingNow() 派生，携带 Asia/Shanghai 时区，AddDate 保留时区
	labels := make([]string, 0, days)
	data := make([]int64, 0, days)
	activeDays := 0
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i)
		key := d.Format("2006-01-02")
		cnt := countByDay[key]
		if cnt > 0 {
			activeDays++
		}
		labels = append(labels, d.Format("1/2"))
		data = append(data, cnt)
	}

	return map[string]interface{}{
		"days":         days,
		"labels":       labels,
		"data":         data,
		"total_count":  totalCount,
		"active_days":  activeDays,
	}
}

// GetCourseChapters 导师章节列表（含文件），对应 Python get_tutor_course_chapters。
func (s *TutorService) GetCourseChapters(courseID int) (map[string]interface{}, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	var chapters []model.Chapter
	s.db.Where("course_id = ?", courseID).Order("order_num").Find(&chapters)
	resultChapters := make([]map[string]interface{}, 0, len(chapters))
	for i := range chapters {
		ch := &chapters[i]
		chDict := chapterToDict(ch)
		var files []model.ChapterFile
		s.db.Where("chapter_id = ?", ch.ChapterID).Order("created_at").Find(&files)
		fileList := make([]map[string]interface{}, 0, len(files))
		if len(files) == 0 && ch.FileURL != "" {
			fileList = append(fileList, legacyFileEntry(ch))
		} else {
			for j := range files {
				fileList = append(fileList, chapterFileToDict(&files[j]))
			}
		}
		chDict["files"] = fileList
		resultChapters = append(resultChapters, chDict)
	}
	return map[string]interface{}{
		"course":   courseToDict(&course),
		"chapters": resultChapters,
	}, nil
}

// UploadChapterFile 上传章节文件，对应 Python upload_chapter_file。
func (s *TutorService) UploadChapterFile(chapterID int, filename string, fileContent []byte) (map[string]interface{}, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	if filename == "" {
		return nil, errors.New("文件名不能为空")
	}
	if s.fileService == nil {
		return nil, errors.New("文件服务不可用")
	}
	if !s.fileService.AllowedFile(filename) {
		return nil, errors.New("不支持的文件格式")
	}
	if !s.fileService.ValidateFileSize(int64(len(fileContent)), filename) {
		return nil, errors.New("文件大小超出限制")
	}

	contentType := s.fileService.GetContentType(filename)
	fileURL, filePath := s.fileService.SaveFile(fileContent, filename, "chapters")

	chapterFile := model.ChapterFile{
		ChapterID:   &chapterID,
		FileURL:     fileURL,
		FileName:    filename,
		ContentType: contentType,
		FileSize:    int64(len(fileContent)),
		CreatedAt:   beijingNow(),
	}
	if err := s.db.Create(&chapterFile).Error; err != nil {
		return nil, err
	}

	if chapter.ContentType == "" || chapter.ContentType == "text" {
		chapter.ContentType = contentType
		chapter.FileURL = fileURL
		s.db.Save(&chapter)
	}

	// PPT 自动转图片
	if contentType == "ppt" {
		slidesDir := filepath.Join(s.uploadFolder, "slides", fmt.Sprintf("%d", chapterID))
		_ = os.RemoveAll(slidesDir)
		_ = s.fileService.ConvertPPTToImages(filePath, slidesDir)
	}

	return chapterFileToDict(&chapterFile), nil
}

// UpdateChapterInfo 更新章节信息，对应 Python update_chapter_info。
func (s *TutorService) UpdateChapterInfo(chapterID int, data map[string]interface{}) (map[string]interface{}, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	if v, ok := data["title"].(string); ok && v != "" {
		chapter.Title = v
	}
	if v, ok := data["content"]; ok {
		chapter.Content, _ = v.(string)
	}
	if v, ok := data["duration"]; ok {
		chapter.Duration = toIntDefault(v, chapter.Duration)
	}
	if v, ok := data["order_num"]; ok {
		chapter.OrderNum = toIntDefault(v, chapter.OrderNum)
	}
	if v, ok := data["description"]; ok {
		chapter.Description, _ = v.(string)
	}
	if err := s.db.Save(&chapter).Error; err != nil {
		return nil, err
	}
	return chapterToDict(&chapter), nil
}

// DeleteChapterFileByID 删除章节文件，对应 Python delete_chapter_file_by_id。
func (s *TutorService) DeleteChapterFileByID(fileID int) (map[string]interface{}, error) {
	var chapterFile model.ChapterFile
	if err := s.db.First(&chapterFile, fileID).Error; err != nil {
		return nil, errors.New("文件不存在")
	}
	if s.fileService != nil {
		s.fileService.DeleteFile(chapterFile.FileURL)
	}
	chapterID := chapterFile.ChapterID
	s.db.Delete(&chapterFile)

	var remaining []model.ChapterFile
	s.db.Where("chapter_id = ?", chapterID).Find(&remaining)
	if chapterID != nil {
		var chapter model.Chapter
		if err := s.db.First(&chapter, *chapterID).Error; err == nil {
			if len(remaining) == 0 {
				chapter.FileURL = ""
				chapter.ContentType = "text"
			} else {
				chapter.FileURL = remaining[0].FileURL
				chapter.ContentType = remaining[0].ContentType
			}
			s.db.Save(&chapter)
		}
	}
	return map[string]interface{}{"file_id": fileID, "deleted": true}, nil
}

// BatchDeleteChapterFiles 批量删除文件，对应 Python batch_delete_chapter_files。
func (s *TutorService) BatchDeleteChapterFiles(fileIDs []int) map[string]interface{} {
	successCount := 0
	failedIDs := make([]int, 0)
	for _, fid := range fileIDs {
		var chapterFile model.ChapterFile
		if err := s.db.First(&chapterFile, fid).Error; err != nil {
			failedIDs = append(failedIDs, fid)
			continue
		}
		if s.fileService != nil {
			s.fileService.DeleteFile(chapterFile.FileURL)
		}
		chapterID := chapterFile.ChapterID
		s.db.Delete(&chapterFile)
		var remaining []model.ChapterFile
		s.db.Where("chapter_id = ?", chapterID).Find(&remaining)
		if len(remaining) == 0 && chapterID != nil {
			var chapter model.Chapter
			if err := s.db.First(&chapter, *chapterID).Error; err == nil {
				chapter.FileURL = ""
				chapter.ContentType = "text"
				s.db.Save(&chapter)
			}
		}
		successCount++
	}
	return map[string]interface{}{
		"success_count": successCount,
		"failed_count":  len(failedIDs),
		"failed_ids":    failedIDs,
	}
}
