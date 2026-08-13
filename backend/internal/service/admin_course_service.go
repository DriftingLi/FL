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
func (s *AdminCourseService) CreateCourse(data map[string]any) (*CourseDTO, error) {
	name, _ := data["name"].(string)
	if name == "" {
		return nil, errors.New("课程名称不能为空")
	}
	if err := validateMountedCourseInput(data, false); err != nil {
		return nil, err
	}
	description, _ := data["description"].(string)
	coverImage, _ := data["cover_image"].(string)
	duration := toIntDefault(data["duration"], 0)
	status := int16(1)
	if v, ok := data["status"]; ok {
		status = int16(toIntDefault(v, 1))
	}
	course := model.Course{
		Name:        name,
		Description: description,
		CoverImage:  coverImage,
		Duration:    duration,
		Status:      status,
		CreatedAt:   beijingNow(),
	}
	if err := applyCourseTrainingFields(s.db, &course, data); err != nil {
		return nil, err
	}
	// 未显式传 sort_order 时，自动排到所属方向+等级组的末尾（max+1）
	if _, ok := data["sort_order"]; !ok && course.SpecialtyID != nil && course.LevelID != nil {
		course.SortOrder = nextSortOrderValue(s.db, "course",
			map[string]any{"specialty_id": *course.SpecialtyID, "level_id": *course.LevelID})
	}
	if err := s.db.Create(&course).Error; err != nil {
		return nil, err
	}
	if err := replaceCoursePrerequisites(s.db, course.CourseID, prerequisiteIDsFromData(data)); err != nil {
		return nil, err
	}
	result := courseToDTO(&course)
	fillChapterCount(s.db, course.CourseID, &result)
	fillPrereqIDs(s.db, course.CourseID, &result)
	return &result, nil
}

// UpdateCourse 更新课程。
func (s *AdminCourseService) UpdateCourse(courseID int, data map[string]any) (*CourseDTO, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	if v, ok := data["name"].(string); ok && v != "" {
		course.Name = v
	}
	if v, ok := data["description"]; ok {
		course.Description, _ = v.(string)
	}
	if v, ok := data["cover_image"]; ok {
		course.CoverImage, _ = v.(string)
	}
	if v, ok := data["duration"]; ok {
		course.Duration = toIntDefault(v, course.Duration)
	}
	if v, ok := data["status"]; ok {
		course.Status = int16(toIntDefault(v, int(course.Status)))
	}
	// 编辑时若携带方向/等级字段则不允许清空（挂载不变式；DB 列保持可空以兼容存量）
	if err := validateMountedCourseInput(data, true); err != nil {
		return nil, err
	}
	if err := applyCourseTrainingFields(s.db, &course, data); err != nil {
		return nil, err
	}
	if err := s.db.Save(&course).Error; err != nil {
		return nil, err
	}
	if prereqIDs, ok := data["prerequisite_course_ids"]; ok {
		if err := replaceCoursePrerequisites(s.db, courseID, toIntSlice(prereqIDs)); err != nil {
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
func (s *AdminCourseService) DeleteCourse(courseID int) (map[string]any, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	if err := s.db.Delete(&course).Error; err != nil {
		return nil, err
	}
	return map[string]any{"course_id": courseID}, nil
}

// CreateChapter 创建章节。
func (s *AdminCourseService) CreateChapter(courseID int, data map[string]any) (*ChapterDTO, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	title, _ := data["title"].(string)
	if title == "" {
		return nil, errors.New("章节标题不能为空")
	}
	content, _ := data["content"].(string)
	duration := toIntDefault(data["duration"], 0)

	var maxOrder int
	s.db.Model(&model.Chapter{}).Where("course_id = ?", courseID).
		Select("COALESCE(MAX(order_num), 0)").Scan(&maxOrder)

	chapter := model.Chapter{
		CourseID:  courseID,
		Title:     title,
		Content:   content,
		Duration:  duration,
		OrderNum:  maxOrder + 1,
		CreatedAt: beijingNow(),
	}
	if err := s.db.Create(&chapter).Error; err != nil {
		return nil, err
	}
	d := chapterToDTO(&chapter)
	return &d, nil
}

// UpdateChapter 更新章节。
func (s *AdminCourseService) UpdateChapter(chapterID int, data map[string]any) (*ChapterDTO, error) {
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
	if err := s.db.Save(&chapter).Error; err != nil {
		return nil, err
	}
	d := chapterToDTO(&chapter)
	return &d, nil
}

// DeleteChapter 删除章节，并清理章节关联的存储文件：
// PPT 幻灯片（slides/<chapterID>/ 前缀）与图文 Markdown 图片（images/chapters/<chapterID>/ 前缀）。
func (s *AdminCourseService) DeleteChapter(chapterID int) (map[string]any, error) {
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
	return map[string]any{"chapter_id": chapterID}, nil
}
