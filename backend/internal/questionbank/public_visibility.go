package questionbank

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/publicscope"
)

func (s *Service) setPublicVisibility(ctx context.Context, id int, visible bool) (*publicscope.Eligibility, error) {
	db := s.db.WithContext(ctx)
	updated := db.Table("question").Where("id = ?", id).UpdateColumn("public_hidden", !visible)
	if updated.Error != nil {
		return nil, updated.Error
	}
	if updated.RowsAffected == 0 {
		return nil, ErrQuestionNotFound
	}
	var record model.Question
	if err := db.First(&record, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuestionNotFound
		}
		return nil, err
	}
	item, err := s.decorateEditorQuestion(&record, false)
	if err != nil {
		return nil, err
	}
	return item.PublicEligibility, nil
}
