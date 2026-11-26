package questionbank

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/publicscope"
)

func fillQuestionPublicStates(db *gorm.DB, items []QuestionDTO) error {
	ids := make([]int, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	states, err := publicscope.LoadQuestionStates(db, ids)
	if err != nil {
		return err
	}
	for i := range items {
		if state, ok := states[items[i].ID]; ok {
			items[i].Category = state.Category
			items[i].PublicEligibility = &state.Eligibility
		}
	}
	return nil
}

func (s *Service) decorateEditorQuestion(q *model.Question, withTags bool) (QuestionDTO, error) {
	d := NewQuestionDTO(q, true)
	if withTags {
		d.Tags = s.loadTagsByQuestion(q.ID)
	}
	items := []QuestionDTO{d}
	if err := fillQuestionPublicStates(s.db, items); err != nil {
		return QuestionDTO{}, err
	}
	return items[0], nil
}
