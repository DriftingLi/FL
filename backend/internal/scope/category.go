package scope

import (
	"errors"
	"strconv"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

var (
	ErrCategoryFilter      = errors.New("培训分类筛选无效")
	ErrCategoryUnavailable = errors.New("请选择已启用的培训分类")
)

func RequireActiveCategory(db *gorm.DB, id int) error {
	if id <= 0 {
		return ErrCategoryUnavailable
	}
	var count int64
	if err := db.Model(&model.TrainingCategory{}).Where("id = ? AND status = 1", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrCategoryUnavailable
	}
	return nil
}

type CategoryFilter struct {
	explicit     bool
	unclassified bool
	id           int
}

func ParseCategoryFilter(raw string, provided bool) (CategoryFilter, error) {
	if !provided {
		return CategoryFilter{}, nil
	}
	switch raw {
	case "all":
		return CategoryFilter{explicit: true}, nil
	case "unclassified":
		return CategoryFilter{explicit: true, unclassified: true}, nil
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return CategoryFilter{}, ErrCategoryFilter
	}
	return CategoryFilter{explicit: true, id: id}, nil
}

func (f CategoryFilter) Explicit() bool { return f.explicit }

func (f CategoryFilter) Narrowed() bool { return f.id > 0 || f.unclassified }

func (f CategoryFilter) Unclassified() bool { return f.unclassified }

func (f CategoryFilter) Apply(q *gorm.DB, column string) *gorm.DB {
	if f.unclassified {
		return q.Where(column + " IS NULL")
	}
	if f.id > 0 {
		return q.Where(column+" = ?", f.id)
	}
	return q
}
