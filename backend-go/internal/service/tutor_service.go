// Package service 导师端课程与文件管理，对应 Python tutor_service。
package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
