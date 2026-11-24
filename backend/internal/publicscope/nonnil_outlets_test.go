package publicscope

import (
	"testing"

	"forklift-training/internal/testutil"
)

var nonnilOutlets = map[string]func(t *testing.T) any{
	"publicscope.Eligibility.reasons": outletEligibleCourse,
}

func outletEligibleCourse(t *testing.T) any {
	t.Helper()
	credentialStatus := int16(1)
	credentialID := 1
	state := CategoryState{ID: 1, Status: 1, LegacyCredentialID: &credentialID, CredentialStatus: &credentialStatus}
	return CourseEligibility(1, state, true)
}

func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutlets})
}
