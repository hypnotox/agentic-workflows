package adr

import (
	"strings"
	"testing"
	"time"
)

// SetNowForTest overrides the now seam for a test and returns the previous
// value, so the caller can restore it. It lives in an in-package _test.go file
// (package adr) so the external adr_test package can reach it without the seam
// shipping in the production binary (ADR-0063).
func SetNowForTest(fn func() time.Time) (prev func() time.Time) {
	prev = now
	now = fn
	return prev
}

func TestValidateV2HistoryRejectsImplementingWithoutAppliedOperations(t *testing.T) {
	digest := ContentDigest(nil)
	record := ADR{
		Status: "Implementing",
		History: []HistoryEvent{
			{Kind: HistoryStatus, Date: "2026-08-04", Status: "Proposed"},
			{Kind: HistoryStatus, Date: "2026-08-04", Status: "Implementing", Digest: digest},
			{Kind: HistoryApplied, Date: "2026-08-04"},
		},
	}
	if err := validateV2History(record); err == nil || !strings.Contains(err.Error(), "requires at least one applied operation") {
		t.Fatalf("Implementing without applied operations error = %v", err)
	}
}
