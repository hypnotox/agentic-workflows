package plancheck

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/plan"
)

func terminalPlan(status, body string) plan.Plan {
	return plan.Plan{Filename: "p.md", Path: "docs/plans/p.md", Status: status, Source: []byte("---\nstatus: " + status + "\n---\n# Plan: P\n" + body), Notes: body, Phases: []plan.Phase{{Tasks: []plan.Task{{Fields: plan.TaskFields{Paths: []plan.PathEntry{{Kind: plan.PathLiteral, Value: "internal/a.go"}}}}}}}}
}

func TestTerminalTransitionFreezesBodiesAndRequiresSelectedReconciliation(t *testing.T) {
	implemented := terminalPlan("Implemented", "## Notes\nTouched paths: internal/a.go\nMaterial deviations: none\n")
	changed := implemented
	changed.Source = []byte(strings.Replace(string(changed.Source), "internal/a.go", "internal/b.go", 1))
	if err := TerminalTransition([]plan.Plan{implemented}, []plan.Plan{changed}, []string{"docs/plans/p.md"}); err == nil || !strings.Contains(err.Error(), "body changed") {
		t.Fatalf("Implemented body edit accepted: %v", err)
	}

	proposed := terminalPlan("Proposed", "## Notes\n")
	if err := TerminalTransition([]plan.Plan{proposed}, []plan.Plan{implemented}, []string{"docs/plans/p.md", "internal/a.go"}); err != nil {
		t.Fatalf("reconciled terminal transition refused: %v", err)
	}
	if err := TerminalTransition([]plan.Plan{proposed}, []plan.Plan{terminalPlan("Implemented", "## Notes\nTouched paths: internal/a.go\n")}, []string{"docs/plans/p.md", "internal/a.go"}); err == nil || !strings.Contains(err.Error(), "material-deviation") {
		t.Fatalf("unreconciled terminal transition accepted: %v", err)
	}

	amended := proposed
	amended.Source = []byte(strings.Replace(string(amended.Source), "## Notes", "## Notes\nordinary amendment", 1))
	if err := TerminalTransition([]plan.Plan{proposed}, []plan.Plan{amended}, []string{"docs/plans/p.md"}); err != nil {
		t.Fatalf("Proposed amendment refused: %v", err)
	}
}
