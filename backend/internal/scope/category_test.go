package scope

import (
	"errors"
	"testing"
)

func TestStaffCategoryFilterAcceptsLegacyBucket(t *testing.T) {
	filter, err := ParseStaffCategoryFilter("legacy", true)
	if err != nil {
		t.Fatal(err)
	}
	if !filter.Explicit() || !filter.Legacy() || !filter.Narrowed() {
		t.Fatalf("legacy staff filter = %+v", filter)
	}
}

func TestSharedCategoryFilterStillRejectsLegacyBucket(t *testing.T) {
	if _, err := ParseCategoryFilter("legacy", true); !errors.Is(err, ErrCategoryFilter) {
		t.Fatalf("shared category filter should not gain staff-only legacy semantics, got %v", err)
	}
}
