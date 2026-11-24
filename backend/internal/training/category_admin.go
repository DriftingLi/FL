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
	"forklift-training/internal/scope"
	"forklift-training/internal/sortorder"
)

var (
	errCategoryInput              = errors.New("培训分类信息无效")
	errCategoryNotFound           = errors.New("培训分类不存在")
	errCategoryCredentialNeeded   = errors.New("请选择培训分类映射的证件")
	errCategoryCredentialNotFound = errors.New("培训分类映射的证件不存在")
	errCategoryCredentialUsed     = errors.New("该证件已映射到其他培训分类")
)

func (s *Service) listAdminCategories(ctx context.Context) (*AdminTrainingCategoryListDTO, error) {
	items := make([]AdminTrainingCategoryDTO, 0)
	err := s.db.WithContext(ctx).Model(&model.TrainingCategory{}).Select("id", "name", "status", "sort_order", "legacy_credential_id").
		Order("sort_order, id").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return &AdminTrainingCategoryListDTO{Categories: items}, nil
}

func (s *Service) saveCategory(ctx context.Context, id int, input TrainingCategoryInput) (*AdminTrainingCategoryDTO, error) {
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
	if id == 0 && input.LegacyCredentialID == nil {
		return nil, errCategoryCredentialNeeded
	}
	if input.LegacyCredentialID != nil && *input.LegacyCredentialID <= 0 {
		return nil, fmt.Errorf("%w，映射证件无效", errCategoryInput)
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
		if input.LegacyCredentialID != nil {
			credentialID := *input.LegacyCredentialID
			var count int64
			if err := tx.Model(&model.Credential{}).Where("id = ?", credentialID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errCategoryCredentialNotFound
			}
			mappingQuery := tx.Model(&model.TrainingCategory{})
			if err := scope.UnusedCredentialMapping(mappingQuery, credentialID, id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errCategoryCredentialUsed
			}
			category.LegacyCredentialID = &credentialID
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
	return &AdminTrainingCategoryDTO{ID: category.ID, Name: category.Name, Status: &category.Status, SortOrder: &category.SortOrder, LegacyCredentialID: category.LegacyCredentialID}, nil
}
