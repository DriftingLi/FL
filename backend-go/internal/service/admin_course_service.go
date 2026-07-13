// Package service 管理端课程 CRUD，对应 Python admin_course_service。
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/cache"
	"forklift-training/internal/model"
)

// AdminCourseService 管理端课程服务。
type AdminCourseService struct {
	db *gorm.DB
}

// NewAdminCourseService 创建管理端课程服务实例。
func NewAdminCourseService(db *gorm.DB) *AdminCourseService {
	return &AdminCourseService{db: db}
}

// GetCourses 管理端课程列表，对应 Python get_admin_courses。
func (s *AdminCourseService) GetCourses(page, pageSize int, keyword, category string) map[string]interface{} {
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

// GetCourseDetail 管理端课程详情，对应 Python get_admin_course_detail。
func (s *AdminCourseService) GetCourseDetail(courseID int) (map[string]interface{}, error) {
	cacheKey := cache.SafeKey("course", "detail", fmt.Sprintf("%d", courseID))
	var result map[string]interface{}
	err := cache.GetOrSetJSON(context.Background(), cacheKey, 10*time.Minute, &result, func() (interface{}, error) {
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
		detail := courseToDict(&course)
		detail["chapters"] = chapterList
		return detail, nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CreateCourse 创建课程，对应 Python create_course。
func (s *AdminCourseService) CreateCourse(data map[string]interface{}) (map[string]interface{}, error) {
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
	if err := s.db.Create(&course).Error; err != nil {
		return nil, err
	}
	_ = cache.InvalidatePattern(context.Background(), "course:list:*")
	_ = cache.InvalidatePattern(context.Background(), "course:detail:*")
	return courseToDict(&course), nil
}

// UpdateCourse 更新课程，对应 Python update_course。
func (s *AdminCourseService) UpdateCourse(courseID int, data map[string]interface{}) (map[string]interface{}, error) {
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
	if err := s.db.Save(&course).Error; err != nil {
		return nil, err
	}
	_ = cache.InvalidatePattern(context.Background(), "course:list:*")
	cache.InvalidatePattern(context.Background(), cache.SafeKey("course", "detail", fmt.Sprintf("%d", courseID)))
	return courseToDict(&course), nil
}

// DeleteCourse 删除课程，对应 Python delete_course。
func (s *AdminCourseService) DeleteCourse(courseID int) (map[string]interface{}, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	if err := s.db.Delete(&course).Error; err != nil {
		return nil, err
	}
	_ = cache.InvalidatePattern(context.Background(), "course:list:*")
	cache.InvalidatePattern(context.Background(), cache.SafeKey("course", "detail", fmt.Sprintf("%d", courseID)))
	return map[string]interface{}{"course_id": courseID}, nil
}

// CreateChapter 创建章节，对应 Python create_chapter。
func (s *AdminCourseService) CreateChapter(courseID int, data map[string]interface{}) (map[string]interface{}, error) {
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
	cache.InvalidatePattern(context.Background(), "course:detail:*")
	return chapterToDict(&chapter), nil
}

// UpdateChapter 更新章节，对应 Python update_chapter。
func (s *AdminCourseService) UpdateChapter(chapterID int, data map[string]interface{}) (map[string]interface{}, error) {
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
	cache.InvalidatePattern(context.Background(), "course:detail:*")
	return chapterToDict(&chapter), nil
}

// DeleteChapter 删除章节，对应 Python delete_chapter。
func (s *AdminCourseService) DeleteChapter(chapterID int) (map[string]interface{}, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	if err := s.db.Delete(&chapter).Error; err != nil {
		return nil, err
	}
	cache.InvalidatePattern(context.Background(), "course:detail:*")
	return map[string]interface{}{"chapter_id": chapterID}, nil
}
