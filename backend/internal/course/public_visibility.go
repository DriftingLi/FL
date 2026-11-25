package course

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/publicscope"
)

func (s *AdminService) setPublicVisibility(ctx context.Context, id int, visible bool) (*publicscope.Eligibility, error) {
	db := s.db.WithContext(ctx)
	updated := db.Table("course").Where("course_id = ?", id).UpdateColumn("public_hidden", !visible)
	if updated.Error != nil {
		return nil, updated.Error
	}
	if updated.RowsAffected == 0 {
		return nil, model.ErrCourseNotFound
	}
	var record model.Course
	if err := db.First(&record, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrCourseNotFound
		}
		return nil, err
	}
	item := CourseToDTO(&record)
	if err := FillCoursePublicState(db, &item); err != nil {
		return nil, err
	}
	return item.PublicEligibility, nil
}
