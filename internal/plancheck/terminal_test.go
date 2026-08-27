package plancheck

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/plan"
)

func terminalPlan(status, body string) plan.Plan {
	reconciliation, err := plan.ParseTerminalReconciliation(body)
	if err != nil {
		panic(err)
	}
	return plan.Plan{Filename: "p.md", Path: "docs/plans/p.md", Status: status, Source: []byte("---\nstatus: " + status + "\n---\n# Plan: P\n" + body), Notes: body, TerminalReconciliation: reconciliation, Phases: []plan.Phase{{Tasks: []plan.Task{{Fields: plan.TaskFields{Paths: []plan.PathEntry{{Kind: plan.PathLiteral, Value: "internal/a.go"}}}}}}}}
}

func selected(paths ...string) map[string][]string { return map[string][]string{"p.md": paths} }

const terminalRange = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa..bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// invariant: adr-system/plan-artifacts:terminal-plan-history-frozen (TestTerminalTransitionFreezesBodiesAndRequiresSelectedReconciliation)
func TestTerminalTransitionFreezesBodiesAndRequiresSelectedReconciliation(t *testing.T) {
	implemented := terminalPlan("Implemented", "## Notes\n### Terminal reconciliation\nImplementation range: "+terminalRange+"\nTouched paths:\n- \"internal/a.go\"\nMaterial deviations:\n- none\n")
	changed := implemented
	changed.Source = []byte(strings.Replace(string(changed.Source), "internal/a.go", "internal/b.go", 1))
	if err := TerminalTransition([]plan.Plan{implemented}, []plan.Plan{changed}, selected("internal/a.go")); err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Fatalf("Implemented body edit accepted: %v", err)
	}

	proposed := terminalPlan("Proposed", "## Notes\n")
	if err := TerminalTransition([]plan.Plan{proposed}, []plan.Plan{implemented}, selected("internal/a.go")); err != nil {
		t.Fatalf("reconciled terminal transition refused: %v", err)
	}
	if err := TerminalTransition(nil, []plan.Plan{implemented}, selected("internal/a.go")); err != nil {
		t.Fatalf("new Implemented plan was mistaken for a transition: %v", err)
	}
	if err := TerminalTransition([]plan.Plan{proposed}, []plan.Plan{terminalPlan("Implemented", "## Notes\nTouched paths: internal/a.go\n")}, selected("internal/a.go")); err == nil || !strings.Contains(err.Error(), "parsed reconciliation") {
		t.Fatalf("unreconciled terminal transition accepted: %v", err)
	}
	if err := TerminalTransition([]plan.Plan{proposed}, []plan.Plan{implemented}, nil); err == nil || !strings.Contains(err.Error(), "unavailable touched-path evidence") {
		t.Fatalf("missing selected evidence accepted: %v", err)
	}
	if err := TerminalTransition([]plan.Plan{proposed}, []plan.Plan{implemented}, selected("internal/b.go")); err == nil || !strings.Contains(err.Error(), "does not equal") {
		t.Fatalf("mismatched selected evidence accepted: %v", err)
	}

	// A marker and a substring are not a complete reconciliation: paths must be
	// parsed as distinct entries and equal the selected implementation evidence.
	markerOnly := terminalPlan("Implemented", "## Notes\nTouched paths: internal/a.go, internal/unmentioned.go\nMaterial deviations: none\n")
	if err := TerminalTransition([]plan.Plan{proposed}, []plan.Plan{markerOnly}, selected("internal/a.go")); err == nil || !strings.Contains(err.Error(), "reconciliation") {
		t.Fatalf("substring reconciliation accepted: %v", err)
	}

	// Actual paths are historical evidence, not choreography: an unplanned
	// touched path is valid when it is completely reconciled.
	unplanned := terminalPlan("Implemented", "## Notes\n### Terminal reconciliation\nImplementation range: "+terminalRange+"\nTouched paths:\n- \"internal/actual.go\"\nMaterial deviations:\n- implementation used the existing owner\n")
	if err := TerminalTransition([]plan.Plan{proposed}, []plan.Plan{unplanned}, selected("internal/actual.go")); err != nil {
		t.Fatalf("complete non-choreographic reconciliation refused: %v", err)
	}

	reversed := terminalPlan("Implemented", "## Notes\n### Terminal reconciliation\nImplementation range: "+terminalRange+"\nTouched paths:\n- \"internal/b.go\"\n- \"internal/a.go\"\nMaterial deviations:\n- none\n")
	if err := TerminalTransition([]plan.Plan{proposed}, []plan.Plan{reversed}, selected("internal/a.go", "internal/b.go")); err != nil {
		t.Fatalf("reversed complete path set refused: %v", err)
	}
	for name, evidence := range map[string]map[string][]string{
		"different length":        selected("internal/a.go", "internal/b.go", "internal/c.go"),
		"duplicate selected path": selected("internal/a.go", "internal/a.go"),
		"missing reconciled path": selected("internal/a.go", "internal/c.go"),
	} {
		t.Run(name, func(t *testing.T) {
			if err := TerminalTransition([]plan.Plan{proposed}, []plan.Plan{reversed}, evidence); err == nil || !strings.Contains(err.Error(), "does not equal") {
				t.Fatalf("invalid path set accepted: %v", err)
			}
		})
	}

	regressed := implemented
	regressed.Status = "Proposed"
	regressed.Source = []byte(strings.Replace(string(regressed.Source), "status: Implemented", "status: Proposed", 1))
	if err := TerminalTransition([]plan.Plan{implemented}, []plan.Plan{regressed}, selected("internal/a.go")); err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Fatalf("Implemented status regression accepted: %v", err)
	}

	amended := proposed
	amended.Source = []byte(strings.Replace(string(amended.Source), "## Notes", "## Notes\nordinary amendment", 1))
	if err := TerminalTransition([]plan.Plan{proposed}, []plan.Plan{amended}, nil); err != nil {
		t.Fatalf("Proposed amendment refused: %v", err)
	}
}
