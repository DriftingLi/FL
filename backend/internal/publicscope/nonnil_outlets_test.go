package publicscope

import (
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

var nonnilOutlets = map[string]func(t *testing.T) any{
	"publicscope.Eligibility.reasons": outletEligibleCourse,
}

func outletEligibleCourse(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	credential := model.Credential{Code: "eligibility", Name: "公开资格证件", Status: 1}
	category := model.TrainingCategory{Name: "公开资格分类", Status: 1, LegacyCredentialID: &credential.ID}
	course := model.Course{Name: "公开课程", Status: 1, CategoryID: &category.ID}
	for _, record := range []any{&credential, &category, &course} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	states, err := LoadCourseStates(db, []int{course.CourseID})
	if err != nil {
		t.Fatal(err)
	}
	return states[course.CourseID].Eligibility
}

func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutlets})
}
