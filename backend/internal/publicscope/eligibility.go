package publicscope

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type CategoryColumn string

const (
	CourseCategoryColumn   CategoryColumn = "course.category_id"
	QuestionCategoryColumn CategoryColumn = "question.category_id"
)

type Reason string

const (
	ReasonManuallyHidden     Reason = "manually_hidden"
	ReasonNotPublished       Reason = "not_published"
	ReasonCategoryMissing    Reason = "category_missing"
	ReasonCategoryDisabled   Reason = "category_disabled"
	ReasonCategoryUnmapped   Reason = "category_unmapped"
	ReasonCredentialMissing  Reason = "credential_missing"
	ReasonCredentialDisabled Reason = "credential_disabled"
	ReasonSourceTagExcluded  Reason = "source_tag_excluded"
)

type VisibilityInput struct {
	Visible *bool `json:"visible" binding:"required"`
}

type Eligibility struct {
	Eligible bool     `json:"eligible"`
	Reasons  []Reason `json:"reasons" nullability:"nonnil"`
}

type CategoryRef struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status int16  `json:"status"`
}

func categoryColumnSQL(column CategoryColumn) string {
	if column != CourseCategoryColumn && column != QuestionCategoryColumn {
		panic("publicscope: unsupported category column")
	}
	return string(column)
}

func LegacyCategoryFilter(db *gorm.DB, column CategoryColumn) *gorm.DB {
	return db.Where("NOT EXISTS (SELECT 1 FROM training_category_public_eligibility WHERE id = " + categoryColumnSQL(column) + " AND blocker_codes = '')")
}

const (
	CourseView   = "course_public_state"
	QuestionView = "question_public_state"
)

// ErrPublicStateMissing 表示内容行存在，但公开资格视图没有对应读模型行。
var ErrPublicStateMissing = errors.New("内容公开状态缺失")

type PublicState struct {
	PublicVisibility bool         `json:"public_visibility"`
	Category         *CategoryRef `json:"category,omitempty" extensions:"x-optional"`
	Eligibility      Eligibility  `json:"eligibility"`
}

func LoadCourseStates(db *gorm.DB, ids []int) (map[int]PublicState, error) {
	return loadStates(db, CourseView, "course_id", ids)
}

func LoadQuestionStates(db *gorm.DB, ids []int) (map[int]PublicState, error) {
	return loadStates(db, QuestionView, "id", ids)
}

func loadStates(db *gorm.DB, view, idColumn string, ids []int) (map[int]PublicState, error) {
	states := make(map[int]PublicState, len(ids))
	if len(ids) == 0 {
		return states, nil
	}
	var rows []struct {
		ID               int
		CategoryID       *int
		CategoryName     string
		CategoryStatus   int16
		Eligible         bool
		PublicVisibility bool
		BlockerCodes     string
	}
	if err := db.Table(view).
		Select(idColumn+" AS id, category_id, category_name, category_status, eligible, public_visibility, blocker_codes").
		Where(idColumn+" IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		state := PublicState{
			PublicVisibility: row.PublicVisibility,
			Eligibility:      Eligibility{Eligible: row.Eligible, Reasons: make([]Reason, 0)},
		}
		if row.CategoryID != nil {
			state.Category = &CategoryRef{ID: *row.CategoryID, Name: row.CategoryName, Status: row.CategoryStatus}
		}
		for code := range strings.SplitSeq(row.BlockerCodes, ",") {
			if code != "" {
				state.Eligibility.Reasons = append(state.Eligibility.Reasons, Reason(code))
			}
		}
		states[row.ID] = state
	}
	missing := make([]int, 0)
	for _, id := range ids {
		if _, ok := states[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: ids=%v", ErrPublicStateMissing, missing)
	}
	return states, nil
}
