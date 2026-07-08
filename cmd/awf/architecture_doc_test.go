package main

import (
	"os"
	"strings"
	"testing"
)

// TestArchitectureDocNamesEveryCmd guards the checker-cmd enumeration in
// docs/architecture.md: cmd/releasecheck (ADR-0078) shipped without a mention
// and cmd/pincheck (ADR-0079) nearly did, each caught only by review. Every
// directory under cmd/ must be named in the architecture doc; the fix path is
// .awf/docs/parts/architecture/components.md + ./x sync (2026-07-08
// retrospective promotion).
func TestArchitectureDocNamesEveryCmd(t *testing.T) {
	doc, err := os.ReadFile("../../docs/architecture.md")
	if err != nil {
		t.Fatalf("read architecture doc: %v", err)
	}
	entries, err := os.ReadDir("../../cmd")
	if err != nil {
		t.Fatalf("read cmd/: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// No closing backtick: the doc writes `cmd/awf/` with a trailing slash
		// but `cmd/covercheck` without one.
		if needle := "`cmd/" + e.Name(); !strings.Contains(string(doc), needle) {
			t.Errorf("docs/architecture.md does not name %s` — add it via .awf/docs/parts/architecture/components.md and ./x sync", needle)
		}
	}
}
