package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// invariant: rendering/workflow-skill-templates:phase-transaction-ownership
func TestPhaseTransactionOwnershipAcrossWorkflowSurfaces(t *testing.T) {
	data := map[string]any{"prefix": "example", "vars": map[string]any{"gateCmd": "./x gate"}, "layout": testLayout(), "data": catalog.Standard.Agents["plan-reviewer"].Data}
	surfaces := map[string]string{
		"writer":   renderSkillGolden(t, "writing-plans", data),
		"reviewer": renderAgentGolden(t, "plan-reviewer", data),
		"inline":   renderSkillGolden(t, "executing-plans", data),
		"subagent": renderSkillGolden(t, "subagent-driven-development", data),
		"readme":   renderGolden(t, "plans-readme/README.md.tmpl", data),
		"template": renderGolden(t, "plans-template/template.md.tmpl", data),
	}
	for name, output := range surfaces {
		for _, forbidden := range []string{"<no value>", "{{", "coupled-phase", "coupled phase", "one commit per task"} {
			if strings.Contains(output, forbidden) {
				t.Errorf("%s retains forbidden %q", name, forbidden)
			}
		}
	}
	assertAll := func(name string, clauses ...string) {
		t.Helper()
		for _, clause := range clauses {
			if !strings.Contains(surfaces[name], clause) {
				t.Errorf("%s missing %q", name, clause)
			}
		}
	}
	assertAll("writer", "Execution mode", "inline", "subagent-driven", "ordered steps", "one independently green coherent implementation transaction", "exhaustive", "path-disjoint", "shared files", "command-confined", "dead-code escape")
	assertAll("reviewer", "ownership", "task-level transaction boundaries", "incomplete/overlapping partitions", "helper-owned shared files", "unconfined commands")
	assertAll("inline", "Iterate phases", "parent owns", "commit-disabled helpers", "report-only phase review", "Review settles before checkpointing", "completed and remaining work", "restore the known green baseline")
	assertAll("subagent", "allowCommits: true", "known clean and green baseline", "complete phase", "one implementation child alone", "report-only phase review", "blind successor")
	assertAll("readme", "Execution mode", "one independently green coherent implementation transaction", "exactly one helper", "shared files")
	assertAll("template", "**Execution mode: inline.**", "phase-closing", "awf check --staged", "closing commit")
	phase := surfaces["template"]
	assertOrderedPhrases(t, phase, "Task 1.1", "Task 1.2", "awf check --staged", "closing commit")
	if strings.Count(phase, "awf check --staged") != 1 {
		t.Errorf("representative phase has %d staged-check boundaries, want one", strings.Count(phase, "awf check --staged"))
	}
}
