package adr_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestRenderIndexMDPartitionsInFlightAndHistory locks the INDEX.md contract
// (ADR-0135 item 8): Proposed and Accepted ADRs render under "In flight",
// Implemented and Abandoned under a compact "History", each section
// number-sorted, In flight before History.
func TestRenderIndexMDPartitionsInFlightAndHistory(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"0002-a-proposal.md": testsupport.ADR("Proposed",
			testsupport.WithTitle("0002: A Proposal"), testsupport.WithBody("## Context\nx\n")),
		"0004-accepted.md": testsupport.ADR("Accepted",
			testsupport.WithTitle("0004: Accepted"), testsupport.WithBody("## Context\nx\n")),
		"0001-shipped.md": testsupport.ADR("Implemented",
			testsupport.WithTitle("0001: Shipped"), testsupport.WithBody("## Context\nx\n")),
		"0003-abandoned.md": testsupport.ADR("Abandoned",
			testsupport.WithTitle("0003: Abandoned"), testsupport.WithBody("## Context\nx\n")),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	got := adr.RenderIndexMD(mustCorpus(t, dir))

	inflightPos := strings.Index(got, "## In flight")
	historyPos := strings.Index(got, "## History")
	if inflightPos < 0 || historyPos < 0 {
		t.Fatalf("both sections must render; got:\n%s", got)
	}
	if inflightPos > historyPos {
		t.Errorf("In flight (%d) must come before History (%d)", inflightPos, historyPos)
	}

	inflight := got[inflightPos:historyPos]
	history := got[historyPos:]

	for _, want := range []string{"ADR-0002: A Proposal", "ADR-0004: Accepted"} {
		if !strings.Contains(inflight, want) {
			t.Errorf("In flight missing %q; section:\n%s", want, inflight)
		}
	}
	for _, want := range []string{"ADR-0001: Shipped", "ADR-0003: Abandoned"} {
		if !strings.Contains(history, want) {
			t.Errorf("History missing %q; section:\n%s", want, history)
		}
	}
	// A terminal ADR never appears in flight, and an in-flight ADR never in history.
	if strings.Contains(inflight, "Shipped") || strings.Contains(inflight, "Abandoned") {
		t.Errorf("terminal ADR leaked into In flight:\n%s", inflight)
	}
	if strings.Contains(history, "Proposal") || strings.Contains(history, "0004: Accepted") {
		t.Errorf("in-flight ADR leaked into History:\n%s", history)
	}
	// Entries carry the status suffix and the file link.
	if !strings.Contains(inflight, "(0002-a-proposal.md) (Proposed)") {
		t.Errorf("In flight entry missing link/status suffix:\n%s", inflight)
	}
	if !strings.Contains(history, "(0003-abandoned.md) (Abandoned)") {
		t.Errorf("History entry missing link/status suffix:\n%s", history)
	}
	// Within History, 0001 sorts before 0003.
	if strings.Index(history, "0001-shipped.md") > strings.Index(history, "0003-abandoned.md") {
		t.Errorf("History not number-sorted:\n%s", history)
	}
}

// TestRenderIndexMDPlaceholdersWhenEmpty proves both sections render a
// placeholder line for an empty corpus, so INDEX.md is never blank and its
// document-map link resolves.
func TestRenderIndexMDPlaceholdersWhenEmpty(t *testing.T) {
	got := adr.RenderIndexMD(mustCorpus(t, t.TempDir()))
	for _, want := range []string{
		"## In flight\n\n_No decisions are in flight._\n",
		"## History\n\n_No decisions recorded yet._\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing placeholder %q; got:\n%s", want, got)
		}
	}
}
