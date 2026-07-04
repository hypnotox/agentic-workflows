package evals

import (
	"os"
	"strings"
	"testing"
)

// read reads path or fails the test.
func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// assertHandoff asserts a cross-artifact seam: the rendered `from` skill names
// the prefixed `to` skill AND the `to` skill is itself present in the rendered
// set. Neither spine_test.go (single-template render, no target-existence
// check) nor ADR-0046 (reference to an *absent* skill) covers "handoff to a
// present skill" — that seam is this suite's mandate (ADR-0053).
func assertHandoff(t *testing.T, root, from, to string) {
	t.Helper()
	body := read(t, skillPath(root, from))
	want := evalPrefix + "-" + to
	if !strings.Contains(body, want) {
		t.Errorf("skill %q does not hand off to %q", from, want)
	}
	if _, err := os.Stat(skillPath(root, to)); err != nil {
		t.Errorf("handoff target %q not present in rendered set: %v", want, err)
	}
}

// TestWorkflowChainHandoffs asserts each load-bearing chain handoff resolves to
// a skill present in the same full-catalog render.
func TestWorkflowChainHandoffs(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalog(t, cat)
	for _, tc := range []struct{ from, to string }{
		{"brainstorming", "proposing-adr"},
		{"brainstorming", "writing-plans"},
		{"proposing-adr", "reviewing-adr"},
		{"writing-plans", "reviewing-plan"},
		{"bugfix", "reviewing-impl"},
	} {
		t.Run(tc.from+"_to_"+tc.to, func(t *testing.T) {
			assertHandoff(t, root, tc.from, tc.to)
		})
	}
}

// assertDispatch asserts a skill->agent->partial seam: the rendered reviewing
// `skill` names the reviewer `agent`, and that agent carries the shared
// review-spine partial (ADR-0052) identified by spineToken. This spans three
// artifacts no single existing test composes.
func assertDispatch(t *testing.T, root, skill, agent, spineToken string) {
	t.Helper()
	if body := read(t, skillPath(root, skill)); !strings.Contains(body, agent) {
		t.Errorf("skill %q does not dispatch agent %q", skill, agent)
	}
	if agentBody := read(t, agentPath(root, agent)); !strings.Contains(agentBody, spineToken) {
		t.Errorf("agent %q missing spine partial token %q", agent, spineToken)
	}
}

// TestReviewerDispatchCarriesSpine asserts each reviewing skill dispatches its
// reviewer agent and that agent carries the review-spine partial.
func TestReviewerDispatchCarriesSpine(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalog(t, cat)
	for _, tc := range []struct{ skill, agent string }{
		{"reviewing-impl", "code-reviewer"},
		{"reviewing-adr", "adr-reviewer"},
		{"reviewing-plan", "plan-reviewer"},
	} {
		t.Run(tc.skill, func(t *testing.T) {
			assertDispatch(t, root, tc.skill, tc.agent, "## Classification rules")
		})
	}
}
