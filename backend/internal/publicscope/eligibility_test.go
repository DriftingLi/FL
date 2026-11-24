package publicscope

import (
	"reflect"
	"testing"
)

func intPtr(value int) *int       { return &value }
func int16Ptr(value int16) *int16 { return &value }

func TestCourseEligibilityReportsAllBlockers(t *testing.T) {
	state := CategoryState{ID: 1, Status: 0}
	got := CourseEligibility(0, state, true)
	want := []Reason{ReasonNotPublished, ReasonCategoryDisabled, ReasonCategoryUnmapped}
	if got.Eligible || !reflect.DeepEqual(got.Reasons, want) {
		t.Fatalf("CourseEligibility() = %#v, want reasons %v", got, want)
	}
}

func TestCourseEligibilityUsesCategoryCredentialMapping(t *testing.T) {
	state := CategoryState{ID: 1, Status: 1, LegacyCredentialID: intPtr(9), CredentialStatus: int16Ptr(0)}
	got := CourseEligibility(1, state, true)
	want := []Reason{ReasonCredentialDisabled}
	if got.Eligible || !reflect.DeepEqual(got.Reasons, want) {
		t.Fatalf("CourseEligibility() = %#v, want reasons %v", got, want)
	}
}

func TestQuestionEligibilityIncludesWorkflowAndSourceTag(t *testing.T) {
	state := CategoryState{ID: 1, Status: 1, LegacyCredentialID: intPtr(9), CredentialStatus: int16Ptr(1)}
	got := QuestionEligibility("pending", true, state, true)
	want := []Reason{ReasonNotPublished, ReasonSourceTagExcluded}
	if got.Eligible || !reflect.DeepEqual(got.Reasons, want) {
		t.Fatalf("QuestionEligibility() = %#v, want reasons %v", got, want)
	}
}

func TestEligibilityReturnsEmptyReasonSliceWhenEligible(t *testing.T) {
	state := CategoryState{ID: 1, Status: 1, LegacyCredentialID: intPtr(9), CredentialStatus: int16Ptr(1)}
	got := QuestionEligibility("published", false, state, true)
	if !got.Eligible || got.Reasons == nil || len(got.Reasons) != 0 {
		t.Fatalf("QuestionEligibility() = %#v, want eligible with non-nil empty reasons", got)
	}
}

func TestCategoryReasonsDistinguishesMissingAndUnmapped(t *testing.T) {
	if got := CategoryReasons(CategoryState{}, false); !reflect.DeepEqual(got, []Reason{ReasonCategoryMissing}) {
		t.Fatalf("missing category reasons = %v", got)
	}
	state := CategoryState{ID: 1, Status: 1}
	if got := CategoryReasons(state, true); !reflect.DeepEqual(got, []Reason{ReasonCategoryUnmapped}) {
		t.Fatalf("unmapped category reasons = %v", got)
	}
	state.LegacyCredentialID = intPtr(9)
	if got := CategoryReasons(state, true); !reflect.DeepEqual(got, []Reason{ReasonCredentialMissing}) {
		t.Fatalf("missing credential reasons = %v", got)
	}
}
