package commitpolicy

import (
	"errors"
	"strings"
	"testing"
)

func TestOperationalOutcomeContracts(t *testing.T) {
	cause := errors.New("native failure")
	categories := []Category{ConfigFailure, BaselineFailure, RevisionFailure, TagPeelFailure, LinkedWorktreeFailure, TrustFileFailure, SignatureProcessFailure}
	for _, category := range categories {
		t.Run(string(category), func(t *testing.T) {
			refusal := &Refusal{Category: category, Observed: "observed", RefsChanged: false, IndexChanged: true, Actions: []string{"reconcile", "rerun"}, Cause: cause}
			outcome := Outcome{Refusal: refusal}
			if outcome.OK() || !errors.Is(refusal.Cause, cause) || refusal.Category != category {
				t.Fatalf("refusal contract = %#v", outcome)
			}
			text := renderOutcome(t, Policy{}, outcome)
			for _, want := range []string{"cause: native failure", "refs: false", "index: true", "step 1: reconcile", "step 2: rerun"} {
				if !strings.Contains(text, want) {
					t.Fatalf("rendered refusal %q missing %q", text, want)
				}
			}
		})
	}
	if (Outcome{Violations: []Violation{{Commit: "a"}}}).OK() {
		t.Fatal("violating outcome reported success")
	}
}
