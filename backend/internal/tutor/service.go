// Package tutor 讲师域：HTTP 出口（handler.go）与导师端课程 / 章节 / 附件实现（service.go）。
package tutor

import (
	"encoding/json"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/course"
	"forklift-training/internal/filestore"
	"forklift-training/internal/model"
)

// Service 导师服务。
type Service struct {
	db            *gorm.DB
	uploadFolder  string
	fileStore     *filestore.FileStore
	slideRenderer *course.SlideRenderer

	logger *zap.Logger
}

// NewService 创建导师服务实例。
func NewService(db *gorm.DB, uploadFolder string, fileStore *filestore.FileStore, slideRenderer *course.SlideRenderer, logger *zap.Logger) *Service {
	return &Service{db: db, uploadFolder: uploadFolder, fileStore: fileStore, slideRenderer: slideRenderer, logger: logger}
}

// ErrChapterFileNotFound 章节文件行不存在（课程章节的附件，与「章节不存在」是两件事）。
// 本文件的「课程/章节不存在」直接用课程域的唯一载体 model.ErrCourseNotFound / course.ErrChapterNotFound。
// （声明必须留在函数文档块之外：它一度夹在下面那条注释与 func 之间，把 godoc 抢走了 ——
// 同形缺陷在 api 层会让整条 swagger 路由消失，见 4c488c3c。）
var ErrChapterFileNotFound = errors.New("文件不存在")

// GetCourses 导师课程列表（与学员端同口径：已上架 + 已挂载方向/等级/证件，ADR-0012 §2），
// 附学习学员数；实现收敛到课程列表 module（ListCourses）。
func (s *Service) GetCourses(page, pageSize int, credentialID, specialtyID, levelID *int) (course.CoursePageResult, error) {
	return course.ListCourses(s.db, page, pageSize, course.CourseListOptions{
		OnlyMounted: true, CredentialID: credentialID, SpecialtyID: specialtyID, LevelID: levelID,
		WithStudentCount: true, DefaultPageSize: 10,
	})
}

// GetCourseChapters 导师章节列表（含文件）。
// 文件列表批量装载（一次 IN 查询）消除逐章节 N+1。
func (s *Service) GetCourseChapters(courseID int) (*course.TutorCourseChaptersDTO, error) {
	// 局部变量不叫 course：包名 course 已被课程域包占用（P2 波 3b-1）。
	var c model.Course
	if err := s.db.First(&c, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrCourseNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0064 决策 1）
	}
	var chapters []model.Chapter
	// 「查不动」如实上抛（ADR-0062 票6）：原先不查 error ⇒ chapter 表读不动时讲师看到的是一个
	// 空章节列表（200 假绿），而不是「这次没读到」。与课程域 loadCourseWithChapters 同一判据。
	if err := s.db.Where("course_id = ?", courseID).Order("order_num").Find(&chapters).Error; err != nil {
		return nil, err
	}

	filesByChapter := course.LoadChapterFilesBulk(s.db, chapters)
	resultChapters := make([]course.ChapterDTO, 0, len(chapters))
	for i := range chapters {
		ch := &chapters[i]
		fileList := filesByChapter[ch.ChapterID]
		// legacy 兼容：无 chapter_file 条目且 chapter.file_url 非空时折叠 legacy 条目
		if len(fileList) == 0 && ch.FileURL != "" {
			legacy := course.LegacyFileEntry(ch)
			fileList = []course.ChapterFileDTO{legacy}
		}
		if fileList == nil {
			fileList = []course.ChapterFileDTO{}
		}
		chDTO := course.ChapterToDTO(ch)
		chDTO.Files = &fileList
		resultChapters = append(resultChapters, chDTO)
	}
	cd := course.CourseToDTO(&c)
	return &course.TutorCourseChaptersDTO{
		Course:   cd,
		Chapters: resultChapters,
	}, nil
}

// GetChapterDetail 章节详情（含上下章ID + 文件列表，供导师端编辑页使用）。
// 实现收敛到共享章节详情 module；导师端不回填 study_status（fillStudyStatus=false）。
func (s *Service) GetChapterDetail(chapterID int) (*course.ChapterDetailDTO, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, course.ErrChapterNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0064 决策 1）
	}
	return course.ChapterDetailShared(s.db, &chapter, false, 0)
}

// UploadChapterFile 上传章节文件。
func (s *Service) UploadChapterFile(chapterID int, filename string, fileContent []byte) (*course.ChapterFileDTO, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, course.ErrChapterNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0064 决策 1）
	}
	if filename == "" {
		return nil, errors.New("文件名不能为空")
	}
	if s.fileStore == nil {
		return nil, errors.New("文件服务不可用")
	}
	if !filestore.AllowedFile(filename) {
		return nil, errors.New("不支持的文件格式")
	}
	if !filestore.ValidateFileSize(int64(len(fileContent)), filename) {
		return nil, errors.New("文件大小超出限制")
	}

	contentType := filestore.FileContentType(filename)
	fileURL, err := s.fileStore.Save(fileContent, filename, filestore.ChapterFileDirPrefix)
	if err != nil {
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}

	chapterFile := model.ChapterFile{
		ChapterID:   &chapterID,
		FileURL:     fileURL,
		FileName:    filename,
		ContentType: contentType,
		FileSize:    int64(len(fileContent)),
		CreatedAt:   clock.Now(),
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

	d := course.ChapterFileToDTO(&chapterFile)
	return &d, nil
}

// UpdateChapterInfo 更新章节信息。
func (s *Service) UpdateChapterInfo(chapterID int, in *course.ChapterInput) (*course.ChapterDTO, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, course.ErrChapterNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0064 决策 1）
	}
	if in == nil {
		in = &course.ChapterInput{}
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
	d := course.ChapterToDTO(&chapter)
	return &d, nil
}

// DeleteChapterFileByID 删除章节文件。
func (s *Service) DeleteChapterFileByID(fileID int) (*DeleteFileResult, error) {
	var chapterFile model.ChapterFile
	if err := s.db.First(&chapterFile, fileID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChapterFileNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0064 决策 1）
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
func (s *Service) BatchDeleteChapterFiles(fileIDs []int) *BatchDeleteFilesResult {
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
