package questionbank

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/publicscope"
)

func (s *Service) sourceTaggedByQuestion(questionIDs []int) (map[int]bool, error) {
	result := make(map[int]bool)
	if len(questionIDs) == 0 {
		return result, nil
	}
	var ids []int
	if err := s.db.Table("question_tag_relation AS qtr").
		Select("DISTINCT qtr.question_id").
		Joins("JOIN question_tag AS qt ON qt.id = qtr.tag_id").
		Where("qt.is_source_tag = ? AND qtr.question_id IN ?", true, questionIDs).
		Pluck("qtr.question_id", &ids).Error; err != nil {
		return nil, err
	}
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

func FillQuestionPublicStates(db *gorm.DB, items []QuestionDTO, sourceTagged map[int]bool) error {
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
		eligibility := publicscope.QuestionEligibility(items[i].Status, sourceTagged[items[i].ID], state, exists).WithVisibility(items[i].publicHidden)
		items[i].PublicEligibility = &eligibility
	}
	return nil
}

func (s *Service) decorateEditorQuestion(q *model.Question, withTags bool) (QuestionDTO, error) {
	d := NewQuestionDTO(q, true)
	if withTags {
		d.Tags = s.loadTagsByQuestion(q.ID)
	}
	sourceTagged, err := s.sourceTaggedByQuestion([]int{q.ID})
	if err != nil {
		return QuestionDTO{}, err
	}
	items := []QuestionDTO{d}
	if err := FillQuestionPublicStates(s.db, items, sourceTagged); err != nil {
		return QuestionDTO{}, err
	}
	return items[0], nil
}
