// Package service 导师端课程与文件管理。
package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// TutorService 导师服务。
type TutorService struct {
	db            *gorm.DB
	uploadFolder  string
	fileStore     *FileStore
	slideRenderer *SlideRenderer

	logger *zap.Logger
}

// NewTutorService 创建导师服务实例。
func NewTutorService(db *gorm.DB, uploadFolder string, fileStore *FileStore, slideRenderer *SlideRenderer, logger *zap.Logger) *TutorService {
	return &TutorService{db: db, uploadFolder: uploadFolder, fileStore: fileStore, slideRenderer: slideRenderer, logger: logger}
}

// GetCourses 导师课程列表（与学员端同口径：已上架 + 已挂载方向/等级/证件，ADR-0012 §2），
// 附学习学员数；实现收敛到课程列表 module（ListCourses）。
func (s *TutorService) GetCourses(page, pageSize int, credentialID, specialtyID, levelID *int) CoursePageResult {
	return ListCourses(s.db, page, pageSize, CourseListOptions{
		OnlyMounted: true, CredentialID: credentialID, SpecialtyID: specialtyID, LevelID: levelID,
		WithStudentCount: true, DefaultPageSize: 10,
	})
}

// GetCourseChapters 导师章节列表（含文件）。
// 文件列表批量装载（一次 IN 查询）消除逐章节 N+1。
func (s *TutorService) GetCourseChapters(courseID int) (*TutorCourseChaptersDTO, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	var chapters []model.Chapter
	s.db.Where("course_id = ?", courseID).Order("order_num").Find(&chapters)

	filesByChapter := loadChapterFilesBulk(s.db, chapters)
	resultChapters := make([]ChapterDTO, 0, len(chapters))
	for i := range chapters {
		ch := &chapters[i]
		fileList := filesByChapter[ch.ChapterID]
		// legacy 兼容：无 chapter_file 条目且 chapter.file_url 非空时折叠 legacy 条目
		if len(fileList) == 0 && ch.FileURL != "" {
			legacy := legacyFileEntry(ch)
			fileList = []ChapterFileDTO{legacy}
		}
		if fileList == nil {
			fileList = []ChapterFileDTO{}
		}
		chDTO := chapterToDTO(ch)
		chDTO.Files = &fileList
		resultChapters = append(resultChapters, chDTO)
	}
	cd := courseToDTO(&course)
	return &TutorCourseChaptersDTO{
		Course:   cd,
		Chapters: resultChapters,
	}, nil
}

// GetChapterDetail 章节详情（含上下章ID + 文件列表，供导师端编辑页使用）。
// 实现收敛到共享章节详情 module；导师端不回填 study_status（fillStudyStatus=false）。
func (s *TutorService) GetChapterDetail(chapterID int) (*ChapterDetailDTO, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	return chapterDetailShared(s.db, &chapter, false, 0), nil
}

// UploadChapterFile 上传章节文件。
func (s *TutorService) UploadChapterFile(chapterID int, filename string, fileContent []byte) (*ChapterFileDTO, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	if filename == "" {
		return nil, errors.New("文件名不能为空")
	}
	if s.fileStore == nil {
		return nil, errors.New("文件服务不可用")
	}
	if !allowedFile(filename) {
		return nil, errors.New("不支持的文件格式")
	}
	if !validateFileSize(int64(len(fileContent)), filename) {
		return nil, errors.New("文件大小超出限制")
	}

	contentType := fileContentType(filename)
	fileURL, err := s.fileStore.Save(fileContent, filename, "chapters")
	if err != nil {
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}

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

	// PPT 自动转图片并持久化 slide URL 列表到 chapter.slide_urls
	if contentType == "ppt" && s.slideRenderer != nil {
		slideURLs := s.slideRenderer.Render(fileContent, chapterID)
		if len(slideURLs) > 0 {
			slideURLsJSON, _ := json.Marshal(slideURLs)
			s.db.Model(&model.Chapter{}).Where("chapter_id = ?", chapterID).Update("slide_urls", string(slideURLsJSON))
		}
	}

	d := chapterFileToDTO(&chapterFile)
	return &d, nil
}

// UpdateChapterInfo 更新章节信息。
func (s *TutorService) UpdateChapterInfo(chapterID int, in *ChapterInput) (*ChapterDTO, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	if in == nil {
		in = &ChapterInput{}
	}
	if in.Title != nil && *in.Title != "" {
		chapter.Title = *in.Title
	}
	if in.Content != nil {
		chapter.Content = *in.Content
	}
	if in.Duration != nil {
		chapter.Duration = *in.Duration
	}
	if in.OrderNum != nil {
		chapter.OrderNum = *in.OrderNum
	}
	if in.Description != nil {
		chapter.Description = *in.Description
	}
	if err := s.db.Save(&chapter).Error; err != nil {
		return nil, err
	}
	d := chapterToDTO(&chapter)
	return &d, nil
}

// DeleteChapterFileByID 删除章节文件。
func (s *TutorService) DeleteChapterFileByID(fileID int) (*DeleteFileResult, error) {
	var chapterFile model.ChapterFile
	if err := s.db.First(&chapterFile, fileID).Error; err != nil {
		return nil, errors.New("文件不存在")
	}
	if s.fileStore != nil {
		_ = s.fileStore.Delete(chapterFile.FileURL)
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
	return &DeleteFileResult{FileID: fileID, Deleted: true}, nil
}

// BatchDeleteChapterFiles 批量删除文件。
func (s *TutorService) BatchDeleteChapterFiles(fileIDs []int) *BatchDeleteFilesResult {
	successCount := 0
	failedIDs := make([]int, 0)
	for _, fid := range fileIDs {
		var chapterFile model.ChapterFile
		if err := s.db.First(&chapterFile, fid).Error; err != nil {
			failedIDs = append(failedIDs, fid)
			continue
		}
		if s.fileStore != nil {
			_ = s.fileStore.Delete(chapterFile.FileURL)
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
	return &BatchDeleteFilesResult{
		SuccessCount: successCount,
		FailedCount:  len(failedIDs),
		FailedIDs:    failedIDs,
	}
}
