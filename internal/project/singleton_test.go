package project

import (
	"strings"
	"testing"
)

// The former singleton-kind-single-source and mandatory-docs-not-in-docs-catalog
// backing tests are retired with those ADR-0043 invariants (ADR-0061): with
// SingletonKinds and plainSingletons both derived from the one doc collection,
// their drift guards are subsumed by unified-doc-model / mandatory-doc-pool-exclusion
// (backed in unified_doc_model_test.go).

// agentsDocContent renders the tree and returns AGENTS.md's content.
func agentsDocContent(t *testing.T, configYAML string) string {
	t.Helper()
	p, err := Open(scaffold(t, configYAML))
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "AGENTS.md" {
			return f.Content
		}
	}
	t.Fatal("AGENTS.md not rendered")
	return ""
}

// With no commands data and no command vars, the Commands section still names
// the drift check, resolved runner-aware (./awf with the runner singleton
// enabled, the generic awf otherwise; ADR-0156 Decision 4); identical command
// values render once.
func TestAgentsDocCommandsDefaultAndDedupe(t *testing.T) {
	empty := agentsDocContent(t, "prefix: example\nvars: {}\nskills: []\nagents: []\n")
	if !strings.Contains(empty, "- `awf check`: check rendered files for drift") {
		t.Errorf("empty Commands section missing the generic awf check default:\n%s", empty)
	}
	runner := agentsDocContent(t, "prefix: example\nvars: {}\nskills: []\nagents: []\nrunner:\n  enabled: true\n")
	if !strings.Contains(runner, "- `./awf check`: check rendered files for drift") {
		t.Errorf("runner-enabled Commands section missing the ./awf check default:\n%s", runner)
	}
	dup := agentsDocContent(t, "prefix: example\nvars:\n  testCmd: make test\n  gateCmd: make test\n  checkCmd: make check\nskills: []\nagents: []\n")
	if got := strings.Count(dup, "- `make test`: run the test suite"); got != 1 {
		t.Errorf("testCmd line rendered %d times, want 1:\n%s", got, dup)
	}
	if strings.Contains(dup, ": run the gate before committing") {
		t.Errorf("gateCmd identical to testCmd must not render its own Commands line:\n%s", dup)
	}
	if !strings.Contains(dup, "`make check`: check rendered files for drift") {
		t.Errorf("distinct checkCmd line missing:\n%s", dup)
	}
}
