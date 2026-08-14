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
	db           *gorm.DB
	uploadFolder string
	fileService  *FileService

	logger *zap.Logger
}

// NewTutorService 创建导师服务实例。
func NewTutorService(db *gorm.DB, uploadFolder string, fileService *FileService, logger *zap.Logger) *TutorService {
	return &TutorService{db: db, uploadFolder: uploadFolder, fileService: fileService, logger: logger}
}

// GetCourses 导师课程列表（与学员端同口径：已上架 + 已挂载方向/等级，ADR-0012 §2），
// 附学习学员数；实现收敛到课程列表 module（ListCourses）。
func (s *TutorService) GetCourses(page, pageSize int, specialtyID, levelID *int) CoursePageResult {
	return ListCourses(s.db, page, pageSize, CourseListOptions{
		OnlyMounted: true, SpecialtyID: specialtyID, LevelID: levelID,
		WithStudentCount: true, DefaultPageSize: 10,
	})
}

// GetGradingStats 阅卷统计（按天分组），用于导师仪表盘图表。
// 统计当前导师 grader_id 命中的 exam_answer 行数（即导师本人批阅题数）。
// days 仅允许 7 或 30，其他值统一回退为 7。
func (s *TutorService) GetGradingStats(tutorID, days int) *GradingStatsDTO {
	// 按天聚合当前导师已批阅题数（起点由 BuildDailySeries/dailySeriesStart 统一钳制与推导）
	start := dailySeriesStart(days)
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

	countByDay := make(map[string]int64, len(rows))
	for _, r := range rows {
		countByDay[r.Day] = r.Count
	}

	series := BuildDailySeries(days, countByDay)
	return &GradingStatsDTO{
		Days:       series.Days,
		Labels:     series.Labels,
		Data:       series.Data,
		TotalCount: series.Total,
		ActiveDays: series.ActiveDays,
	}
}

// GetCourseChapters 导师章节列表（含文件）。
func (s *TutorService) GetCourseChapters(courseID int) (*TutorCourseChaptersDTO, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	var chapters []model.Chapter
	s.db.Where("course_id = ?", courseID).Order("order_num").Find(&chapters)
	resultChapters := make([]ChapterDTO, 0, len(chapters))
	for i := range chapters {
		ch := &chapters[i]
		chDTO := chapterToDTO(ch)
		var files []model.ChapterFile
		s.db.Where("chapter_id = ?", ch.ChapterID).Order("created_at").Find(&files)
		fileList := make([]ChapterFileDTO, 0, len(files))
		if len(files) == 0 && ch.FileURL != "" {
			fileList = append(fileList, legacyFileEntry(ch))
		} else {
			for j := range files {
				fileList = append(fileList, chapterFileToDTO(&files[j]))
			}
		}
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
func (s *TutorService) GetChapterDetail(chapterID int) (*ChapterDetailDTO, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	// 同课程章节按 order_num 排序，计算上下章ID
	var chapters []model.Chapter
	s.db.Where("course_id = ?", chapter.CourseID).Order("order_num").Find(&chapters)
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
	// 文件列表
	var files []model.ChapterFile
	s.db.Where("chapter_id = ?", chapterID).Order("created_at").Find(&files)
	fileList := make([]ChapterFileDTO, 0, len(files))
	if len(files) == 0 && chapter.FileURL != "" {
		fileList = append(fileList, legacyFileEntry(&chapter))
	} else {
		for j := range files {
			fileList = append(fileList, chapterFileToDTO(&files[j]))
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
	return &ChapterDetailDTO{
		ChapterDTO:        result,
		Files:             fileList,
		PreviousChapterID: prevIDPtr,
		NextChapterID:     nextIDPtr,
	}, nil
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
	fileURL, err := s.fileService.SaveFile(fileContent, filename, "chapters")
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
	if contentType == "ppt" {
		slideURLs := s.fileService.ConvertPPTToImages(fileContent, chapterID)
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
	if s.fileService != nil {
		_ = s.fileService.DeleteFile(chapterFile.FileURL)
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
		if s.fileService != nil {
			_ = s.fileService.DeleteFile(chapterFile.FileURL)
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
