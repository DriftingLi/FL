package publicscope

import "gorm.io/gorm"

type CategoryColumn string

const (
	CourseCategoryColumn   CategoryColumn = "course.category_id"
	QuestionCategoryColumn CategoryColumn = "question.category_id"
)

type Reason string

const (
	ReasonNotPublished       Reason = "not_published"
	ReasonCategoryMissing    Reason = "category_missing"
	ReasonCategoryDisabled   Reason = "category_disabled"
	ReasonCategoryUnmapped   Reason = "category_unmapped"
	ReasonCredentialMissing  Reason = "credential_missing"
	ReasonCredentialDisabled Reason = "credential_disabled"
	ReasonSourceTagExcluded  Reason = "source_tag_excluded"
)

type Eligibility struct {
	Eligible bool     `json:"eligible"`
	Reasons  []Reason `json:"reasons" nullability:"nonnil"`
}

type CategoryRef struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status int16  `json:"status"`
}

type CategoryState struct {
	ID                 int
	Name               string
	Status             int16
	LegacyCredentialID *int
	CredentialStatus   *int16
}

func (s CategoryState) Ref() *CategoryRef {
	if s.ID == 0 {
		return nil
	}
	return &CategoryRef{ID: s.ID, Name: s.Name, Status: s.Status}
}

func CategoryReasons(state CategoryState, exists bool) []Reason {
	if !exists {
		return []Reason{ReasonCategoryMissing}
	}
	reasons := make([]Reason, 0, 3)
	if state.Status != 1 {
		reasons = append(reasons, ReasonCategoryDisabled)
	}
	if state.LegacyCredentialID == nil {
		reasons = append(reasons, ReasonCategoryUnmapped)
	} else if state.CredentialStatus == nil {
		reasons = append(reasons, ReasonCredentialMissing)
	} else if *state.CredentialStatus != 1 {
		reasons = append(reasons, ReasonCredentialDisabled)
	}
	return reasons
}

func CourseEligibility(status int16, state CategoryState, categoryExists bool) Eligibility {
	reasons := make([]Reason, 0, 4)
	if status != 1 {
		reasons = append(reasons, ReasonNotPublished)
	}
	reasons = append(reasons, CategoryReasons(state, categoryExists)...)
	return Eligibility{Eligible: len(reasons) == 0, Reasons: reasons}
}

func QuestionEligibility(status string, sourceTagged bool, state CategoryState, categoryExists bool) Eligibility {
	reasons := make([]Reason, 0, 4)
	if status != "published" {
		reasons = append(reasons, ReasonNotPublished)
	}
	if sourceTagged {
		reasons = append(reasons, ReasonSourceTagExcluded)
	}
	reasons = append(reasons, CategoryReasons(state, categoryExists)...)
	return Eligibility{Eligible: len(reasons) == 0, Reasons: reasons}
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

func ActiveCategoryScope(db *gorm.DB, column CategoryColumn) *gorm.DB {
	categoryColumn := categoryColumnSQL(column)
	return db.
		Joins("JOIN training_category ON training_category.id = "+categoryColumn).
		Joins("JOIN credential ON credential.id = training_category.legacy_credential_id").
		Where("training_category.status = ?", 1).
		Where("credential.status = ?", 1)
}

func LegacyCategoryFilter(db *gorm.DB, column CategoryColumn) *gorm.DB {
	categoryColumn := categoryColumnSQL(column)
	return db.Where("NOT EXISTS (SELECT 1 FROM training_category JOIN credential ON credential.id = training_category.legacy_credential_id WHERE training_category.id = "+categoryColumn+" AND training_category.status = ? AND credential.status = ?)", 1, 1)
}

func LoadCategoryStates(db *gorm.DB, ids []int) (map[int]CategoryState, error) {
	states := make(map[int]CategoryState)
	if len(ids) == 0 {
		return states, nil
	}
	type row struct {
		ID                 int
		Name               string
		Status             int16
		LegacyCredentialID *int
		CredentialStatus   *int16
	}
	var rows []row
	if err := db.Table("training_category").
		Select("training_category.id, training_category.name, training_category.status, training_category.legacy_credential_id, credential.status AS credential_status").
		Joins("LEFT JOIN credential ON credential.id = training_category.legacy_credential_id").
		Where("training_category.id IN ?", ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		states[item.ID] = CategoryState(item)
	}
	return states, nil
}

func KeepExistingCategories(categoryIDs []*int) []int {
	seen := make(map[int]struct{}, len(categoryIDs))
	ids := make([]int, 0, len(categoryIDs))
	for _, categoryID := range categoryIDs {
		if categoryID == nil {
			continue
		}
		if _, ok := seen[*categoryID]; ok {
			continue
		}
		seen[*categoryID] = struct{}{}
		ids = append(ids, *categoryID)
	}
	return ids
}
