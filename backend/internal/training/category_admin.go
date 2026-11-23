package training

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"forklift-training/internal/model"
	"forklift-training/internal/sortorder"
)

var (
	errCategoryInput    = errors.New("培训分类信息无效")
	errCategoryNotFound = errors.New("培训分类不存在")
)

func (s *Service) listAdminCategories(ctx context.Context) (*TrainingCategoryListDTO, error) {
	items := make([]TrainingCategoryDTO, 0)
	err := s.db.WithContext(ctx).Model(&model.TrainingCategory{}).Select("id", "name", "status", "sort_order").
		Order("sort_order, id").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return &TrainingCategoryListDTO{Categories: items}, nil
}

func (s *Service) saveCategory(ctx context.Context, id int, input TrainingCategoryInput) (*TrainingCategoryDTO, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return nil, fmt.Errorf("%w，请填写 1–100 个字符的名称", errCategoryInput)
	}
	if err := validateStatus(input.Status); err != nil {
		return nil, fmt.Errorf("%w，%s", errCategoryInput, err.Error())
	}
	if input.SortOrder != nil && *input.SortOrder < 0 {
		return nil, fmt.Errorf("%w，排序值不能为负数", errCategoryInput)
	}
	category := model.TrainingCategory{Name: name, Status: 1}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if id > 0 {
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Take(&category, id).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errCategoryNotFound
			}
			if err != nil {
				return err
			}
			category.Name = name
		} else if input.SortOrder == nil {
			category.SortOrder = sortorder.NextValue(tx, "training_category", nil)
		}
		if input.SortOrder != nil {
			category.SortOrder = *input.SortOrder
		}
		if input.Status != nil {
			category.Status = *input.Status
		}
		if id > 0 {
			return tx.Save(&category).Error
		}
		status := category.Status
		if err := tx.Create(&category).Error; err != nil {
			return err
		}
		if category.Status != status {
			return tx.Model(&category).Update("status", status).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &TrainingCategoryDTO{ID: category.ID, Name: category.Name, Status: &category.Status, SortOrder: &category.SortOrder}, nil
}
