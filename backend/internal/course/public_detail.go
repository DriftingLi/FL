package course

import (
	"context"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

type PublicCourseCategoryDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type PublicCourseChapterDTO struct {
	ChapterID   int    `json:"chapter_id"`
	Title       string `json:"title"`
	OrderNum    int    `json:"order_num"`
	ContentType string `json:"content_type"`
	Duration    int    `json:"duration"`
}

type PublicCourseDetailDTO struct {
	CourseID     int                      `json:"course_id"`
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
	CoverImage   string                   `json:"cover_image"`
	Category     PublicCourseCategoryDTO  `json:"category"`
	PointsPrice  *int                     `json:"points_price,omitempty" extensions:"x-optional"`
	ChapterCount int                      `json:"chapter_count"`
	Chapters     []PublicCourseChapterDTO `json:"chapters" nullability:"nonnil"`
}

type PublicCourseSummaryDTO struct {
	CourseID    int                     `json:"course_id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	CoverImage  string                  `json:"cover_image"`
	Category    PublicCourseCategoryDTO `json:"category" gorm:"embedded;embeddedPrefix:category_"`
	PointsPrice *int                    `json:"points_price,omitempty" extensions:"x-optional"`
}

type PublicCoursePageDTO struct {
	Courses []PublicCourseSummaryDTO `json:"courses" nullability:"nonnil"`
	Page    int                      `json:"page"`
	Pages   int                      `json:"pages"`
	Total   int64                    `json:"total"`
}

type publicCourseListRequest struct {
	page       int
	pageSize   int
	categoryID *int
	filter     string
}

func (s *Service) ListPublicCourses(ctx context.Context, req publicCourseListRequest) (*PublicCoursePageDTO, error) {
	items, total, page, pageSize, err := paging.QueryWithScan[PublicCourseSummaryDTO](
		s.db.WithContext(ctx), req.page, req.pageSize, 12, 60, "course.sort_order, course.created_at DESC, course.course_id DESC",
		func(db *gorm.DB) *gorm.DB {
			query := PublishedCategoryCourseScope(db.Table("course")).
				Joins("JOIN training_category AS category ON category.id = course.category_id").
				Select("course.course_id, course.name, course.description, course.cover_image, course.points_price, category.id AS category_id, category.name AS category_name")
			if req.categoryID != nil {
				query = query.Where("course.category_id = ?", *req.categoryID)
			}
			return filterCourseHighlights(query, req.filter)
		},
	)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []PublicCourseSummaryDTO{}
	}
	return &PublicCoursePageDTO{Courses: items, Page: page, Pages: response.PageCount(total, pageSize), Total: total}, nil
}

func (s *Service) GetPublicCourseDetail(ctx context.Context, courseID int) (*PublicCourseDetailDTO, error) {
	db := s.db.WithContext(ctx)
	var item model.Course
	query := PublishedCategoryCourseScope(db.Model(&model.Course{})).
		Select("course_id", "name", "description", "cover_image", "category_id", "points_price").
		Where("course_id = ?", courseID)
	if err := fetchRow(query, &item, model.ErrCourseNotFound); err != nil {
		return nil, err
	}
	var category PublicCourseCategoryDTO
	if err := fetchRow(db.Model(&model.TrainingCategory{}).Select("id", "name").
		Where("id = ?", item.CategoryID), &category, model.ErrCourseNotFound); err != nil {
		return nil, err
	}
	chapters := make([]PublicCourseChapterDTO, 0)
	if err := db.Model(&model.Chapter{}).
		Select("chapter.chapter_id, chapter.title, chapter.order_num, published.content_type, chapter.duration").
		Joins("JOIN chapter_content_slot AS published ON published.chapter_id = chapter.chapter_id AND published.slot = ?", model.ChapterReviewPublished).
		Where("chapter.course_id = ?", courseID).
		Order("chapter.order_num, chapter.chapter_id").Find(&chapters).Error; err != nil {
		return nil, err
	}
	return &PublicCourseDetailDTO{
		CourseID: item.CourseID, Name: item.Name, Description: item.Description, CoverImage: item.CoverImage,
		Category: category, PointsPrice: item.PointsPrice, ChapterCount: len(chapters), Chapters: chapters,
	}, nil
}
