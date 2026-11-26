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
	ids := make([]int, len(items))
	for i := range items {
		ids[i] = items[i].CourseID
	}
	states, err := publicscope.LoadCourseStates(db, ids)
	if err != nil {
		return err
	}
	for i := range items {
		if state, ok := states[items[i].CourseID]; ok {
			items[i].Category = state.Category
			items[i].PublicEligibility = &state.Eligibility
		}
	}
	return nil
}
