package course

import (
	"gorm.io/gorm"

	"forklift-training/internal/publicscope"
)

func FillCoursePublicState(db *gorm.DB, item *CourseDTO) error {
	items := []CourseDTO{*item}
	if err := FillCoursePublicStates(db, items); err != nil {
		return err
	}
	*item = items[0]
	return nil
}

func FillCoursePublicStates(db *gorm.DB, items []CourseDTO) error {
	categoryIDs := make([]*int, 0, len(items))
	for i := range items {
		categoryIDs = append(categoryIDs, items[i].CategoryID)
	}
	states, err := publicscope.LoadCategoryStates(db, publicscope.KeepExistingCategories(categoryIDs))
	if err != nil {
		return err
	}
	for i := range items {
		var state publicscope.CategoryState
		exists := false
		if items[i].CategoryID != nil {
			state, exists = states[*items[i].CategoryID]
		}
		items[i].Category = state.Ref()
		eligibility := publicscope.CourseEligibility(items[i].Status, state, exists).WithVisibility(items[i].publicHidden)
		items[i].PublicEligibility = &eligibility
	}
	return nil
}
