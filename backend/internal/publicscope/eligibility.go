package publicscope

import (
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
	switch column {
	case CourseCategoryColumn:
		return string(CourseCategoryColumn)
	case QuestionCategoryColumn:
		return string(QuestionCategoryColumn)
	default:
		panic("publicscope: unsupported category column")
	}
}

func LegacyCategoryFilter(db *gorm.DB, column CategoryColumn) *gorm.DB {
	return db.Where("NOT EXISTS (SELECT 1 FROM training_category_public_eligibility WHERE id = " + categoryColumnSQL(column) + " AND blocker_codes = '')")
}

const (
	CourseView   = "course_public_eligibility"
	QuestionView = "question_public_eligibility"
)

type PublicState struct {
	Category    *CategoryRef
	Eligibility Eligibility
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
		ID             int
		CategoryID     *int
		CategoryName   string
		CategoryStatus int16
		Eligible       bool
		BlockerCodes   string
	}
	if err := db.Table(view).
		Select(idColumn+" AS id, category_id, category_name, category_status, eligible, blocker_codes").
		Where(idColumn+" IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		state := PublicState{Eligibility: Eligibility{Eligible: row.Eligible, Reasons: make([]Reason, 0)}}
		if row.CategoryID != nil {
			state.Category = &CategoryRef{ID: *row.CategoryID, Name: row.CategoryName, Status: row.CategoryStatus}
		}
		for code := range strings.SplitSeq(strings.TrimSuffix(row.BlockerCodes, ","), ",") {
			if code != "" {
				state.Eligibility.Reasons = append(state.Eligibility.Reasons, Reason(code))
			}
		}
		states[row.ID] = state
	}
	return states, nil
}
