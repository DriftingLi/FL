// Package service 管理端课程 CRUD。
package service

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// AdminCourseService 管理端课程服务。
type AdminCourseService struct {
	db      *gorm.DB
	fileSvc *FileService

	logger *zap.Logger
}

// NewAdminCourseService 创建管理端课程服务实例。fileSvc 用于删除章节时清理幻灯片/图文图片（可 nil，nil 时跳过）。
func NewAdminCourseService(db *gorm.DB, fileSvc *FileService, logger *zap.Logger) *AdminCourseService {
	return &AdminCourseService{db: db, fileSvc: fileSvc, logger: logger}
}

// GetCourses 管理端课程列表。
func (s *AdminCourseService) GetCourses(page, pageSize int, keyword string, specialtyID, levelID *int) CoursePageResult {
	return ListCourses(s.db, page, pageSize, CourseListOptions{
		Keyword: keyword, SpecialtyID: specialtyID, LevelID: levelID, DefaultPageSize: 10,
	})
}

// GetCourseDetail 管理端课程详情。
func (s *AdminCourseService) GetCourseDetail(courseID int) (*AdminCourseDetailDTO, error) {
	course, chapterList, err := loadCourseWithChapters(s.db, courseID)
	if err != nil {
		return nil, err
	}
	detail := courseToDTO(course)
	fillCourseMeta(s.db, course, &detail)
	return &AdminCourseDetailDTO{
		CourseDTO: detail,
		Chapters:  chapterList,
	}, nil
}

// CreateCourse 创建课程。专业方向与课程等级必填（挂载不变式，旧 category 已退役）。
func (s *AdminCourseService) CreateCourse(in *CourseInput) (*CourseDTO, error) {
	if in == nil || in.Name == nil || *in.Name == "" {
		return nil, errors.New("课程名称不能为空")
	}
	// 挂载不变式：创建必填方向与等级
	if in.SpecialtyID == nil || *in.SpecialtyID <= 0 {
		return nil, errors.New("专业方向不能为空")
	}
	if in.LevelID == nil || *in.LevelID <= 0 {
		return nil, errors.New("课程等级不能为空")
	}
	status := int16(1)
	if in.Status != nil {
		status = *in.Status
	}
	course := model.Course{
		Name:      *in.Name,
		Status:    status,
		CreatedAt: beijingNow(),
	}
	if in.Description != nil {
		course.Description = *in.Description
	}
	if in.CoverImage != nil {
		course.CoverImage = *in.CoverImage
	}
	if in.Duration != nil {
		course.Duration = *in.Duration
	}
	if err := applyCourseTrainingFields(s.db, &course, in); err != nil {
		return nil, err
	}
	// 未显式传 sort_order 时，自动排到所属方向+等级组的末尾（max+1）
	if in.SortOrder == nil && course.SpecialtyID != nil && course.LevelID != nil {
		course.SortOrder = nextSortOrderValue(s.db, "course",
			map[string]any{"specialty_id": *course.SpecialtyID, "level_id": *course.LevelID})
	}
	if err := s.db.Create(&course).Error; err != nil {
		return nil, err
	}
	if err := replaceCoursePrerequisites(s.db, course.CourseID, in.PrerequisiteCourseIDs); err != nil {
		return nil, err
	}
	result := courseToDTO(&course)
	fillChapterCount(s.db, course.CourseID, &result)
	fillPrereqIDs(s.db, course.CourseID, &result)
	return &result, nil
}

// UpdateCourse 更新课程。
func (s *AdminCourseService) UpdateCourse(courseID int, in *CourseInput) (*CourseDTO, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	if in == nil {
		in = &CourseInput{}
	}
	if in.Name != nil && *in.Name != "" {
		course.Name = *in.Name
	}
	if in.Description != nil {
		course.Description = *in.Description
	}
	if in.CoverImage != nil {
		course.CoverImage = *in.CoverImage
	}
	if in.Duration != nil {
		course.Duration = *in.Duration
	}
	if in.Status != nil {
		course.Status = *in.Status
	}
	// 编辑时若携带方向/等级字段则不允许清空（挂载不变式；DB 列保持可空以兼容存量）
	if err := validateMountedCourseInputUpdate(in); err != nil {
		return nil, err
	}
	if err := applyCourseTrainingFields(s.db, &course, in); err != nil {
		return nil, err
	}
	if err := s.db.Save(&course).Error; err != nil {
		return nil, err
	}
	if in.PrerequisiteCourseIDs != nil {
		if err := replaceCoursePrerequisites(s.db, courseID, in.PrerequisiteCourseIDs); err != nil {
			return nil, err
		}
	}
	result := courseToDTO(&course)
	fillChapterCount(s.db, courseID, &result)
	fillPrereqIDs(s.db, courseID, &result)
	return &result, nil
}

// SwapCourseSort 交换两门课程的排序位置（限制在同一方向+等级组内，真实生效含同值默认）。
func (s *AdminCourseService) SwapCourseSort(a, b int) error {
	var ca, cb model.Course
	if err := s.db.First(&ca, a).Error; err != nil {
		return errors.New("课程不存在")
	}
	if err := s.db.First(&cb, b).Error; err != nil {
		return errors.New("课程不存在")
	}
	if !courseMounted(ca.SpecialtyID, ca.LevelID) || !courseMounted(cb.SpecialtyID, cb.LevelID) {
		return errors.New("未挂载方向/等级的课程不能参与排序")
	}
	if *ca.SpecialtyID != *cb.SpecialtyID || *ca.LevelID != *cb.LevelID {
		return errors.New("只能交换同一方向+等级组内的课程")
	}
	return swapGroupPositions(s.db, &model.Course{}, "course_id", a, b,
		map[string]any{"specialty_id": *ca.SpecialtyID, "level_id": *ca.LevelID})
}

// DeleteCourse 删除课程。
func (s *AdminCourseService) DeleteCourse(courseID int) (*DeleteCourseResult, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	if err := s.db.Delete(&course).Error; err != nil {
		return nil, err
	}
	return &DeleteCourseResult{CourseID: courseID}, nil
}

// CreateChapter 创建章节。
func (s *AdminCourseService) CreateChapter(courseID int, in *ChapterInput) (*ChapterDTO, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	if in == nil || in.Title == nil || *in.Title == "" {
		return nil, errors.New("章节标题不能为空")
	}

	var maxOrder int
	s.db.Model(&model.Chapter{}).Where("course_id = ?", courseID).
		Select("COALESCE(MAX(order_num), 0)").Scan(&maxOrder)

	chapter := model.Chapter{
		CourseID:  courseID,
		Title:     *in.Title,
		OrderNum:  maxOrder + 1,
		CreatedAt: beijingNow(),
	}
	if in.Content != nil {
		chapter.Content = *in.Content
	}
	if in.Duration != nil {
		chapter.Duration = *in.Duration
	}
	if err := s.db.Create(&chapter).Error; err != nil {
		return nil, err
	}
	d := chapterToDTO(&chapter)
	return &d, nil
}

// UpdateChapter 更新章节。
func (s *AdminCourseService) UpdateChapter(chapterID int, in *ChapterInput) (*ChapterDTO, error) {
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
	if err := s.db.Save(&chapter).Error; err != nil {
		return nil, err
	}
	d := chapterToDTO(&chapter)
	return &d, nil
}

// DeleteChapter 删除章节，并清理章节关联的存储文件：
// PPT 幻灯片（slides/<chapterID>/ 前缀）与图文 Markdown 图片（images/chapters/<chapterID>/ 前缀）。
func (s *AdminCourseService) DeleteChapter(chapterID int) (*DeleteChapterResult, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	if err := s.db.Delete(&chapter).Error; err != nil {
		return nil, err
	}
	if s.fileSvc != nil {
		s.fileSvc.DeleteFiles(s.fileSvc.ListFiles(fmt.Sprintf("slides/%d", chapterID)))
		s.fileSvc.DeleteFiles(s.fileSvc.ListFiles(fmt.Sprintf("images/chapters/%d", chapterID)))
	}
	return &DeleteChapterResult{ChapterID: chapterID}, nil
}
