// Package service 管理端课程 CRUD。
package service

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// AdminCourseService 管理端课程服务。
type AdminCourseService struct {
	db      *gorm.DB
	fileSvc *FileService
}

// NewAdminCourseService 创建管理端课程服务实例。fileSvc 用于删除章节时清理幻灯片/图文图片（可 nil，nil 时跳过）。
func NewAdminCourseService(db *gorm.DB, fileSvc *FileService) *AdminCourseService {
	return &AdminCourseService{db: db, fileSvc: fileSvc}
}

// GetCourses 管理端课程列表。
func (s *AdminCourseService) GetCourses(page, pageSize int, keyword, category string, specialtyID, levelID *int) map[string]any {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := s.db.Model(&model.Course{})
	if keyword != "" {
		q = q.Where("name LIKE ?", "%"+keyword+"%")
	}
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if specialtyID != nil {
		q = q.Where("specialty_id = ?", *specialtyID)
	}
	if levelID != nil {
		q = q.Where("level_id = ?", *levelID)
	}
	var total int64
	q.Count(&total)
	var courses []model.Course
	q.Order("sort_order ASC, created_at DESC, course_id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&courses)
	items := make([]map[string]any, 0, len(courses))
	for i := range courses {
		item := courseToDict(&courses[i])
		var chapterCount int64
		s.db.Model(&model.Chapter{}).Where("course_id = ?", courses[i].CourseID).Count(&chapterCount)
		item["chapter_count"] = chapterCount
		items = append(items, item)
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return map[string]any{
		"total":   total,
		"page":    page,
		"pages":   pages,
		"courses": items,
	}
}

// GetCourseDetail 管理端课程详情。
func (s *AdminCourseService) GetCourseDetail(courseID int) (map[string]any, error) {
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
	detail := courseToDict(&course)
	detail["chapters"] = chapterList
	fillCourseMeta(s.db, &course, detail)
	return detail, nil
}

// CreateCourse 创建课程。
func (s *AdminCourseService) CreateCourse(data map[string]any) (map[string]any, error) {
	name, _ := data["name"].(string)
	category, _ := data["category"].(string)
	if name == "" {
		return nil, errors.New("课程名称不能为空")
	}
	if category == "" {
		return nil, errors.New("课程分类不能为空")
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
		Category:    category,
		Description: description,
		CoverImage:  coverImage,
		Duration:    duration,
		Status:      status,
		CreatedAt:   beijingNow(),
	}
	if err := applyCourseTrainingFields(s.db, &course, data); err != nil {
		return nil, err
	}
	if err := s.db.Create(&course).Error; err != nil {
		return nil, err
	}
	if err := replaceCoursePrerequisites(s.db, course.CourseID, prerequisiteIDsFromData(data)); err != nil {
		return nil, err
	}
	return courseToDict(&course), nil
}

// UpdateCourse 更新课程。
func (s *AdminCourseService) UpdateCourse(courseID int, data map[string]any) (map[string]any, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	if v, ok := data["name"].(string); ok && v != "" {
		course.Name = v
	}
	if v, ok := data["category"].(string); ok && v != "" {
		course.Category = v
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
	return courseToDict(&course), nil
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
func (s *AdminCourseService) CreateChapter(courseID int, data map[string]any) (map[string]any, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	title, _ := data["title"].(string)
	if title == "" {
		return nil, errors.New("章节标题不能为空")
	}
	content, _ := data["content"].(string)
	contentURL, _ := data["content_url"].(string)
	duration := toIntDefault(data["duration"], 0)

	var maxOrder int
	s.db.Model(&model.Chapter{}).Where("course_id = ?", courseID).
		Select("COALESCE(MAX(order_num), 0)").Scan(&maxOrder)

	chapter := model.Chapter{
		CourseID:   courseID,
		Title:      title,
		Content:    content,
		ContentURL: contentURL,
		Duration:   duration,
		OrderNum:   maxOrder + 1,
		CreatedAt:  beijingNow(),
	}
	if err := s.db.Create(&chapter).Error; err != nil {
		return nil, err
	}
	return chapterToDict(&chapter), nil
}

// UpdateChapter 更新章节。
func (s *AdminCourseService) UpdateChapter(chapterID int, data map[string]any) (map[string]any, error) {
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
	if v, ok := data["content_url"]; ok {
		chapter.ContentURL, _ = v.(string)
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
	return chapterToDict(&chapter), nil
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
