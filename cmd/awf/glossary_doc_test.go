package main

import (
	"os"
	"strings"
	"testing"
)

// TestGlossaryTermsSorted holds the glossary part to its declared "Sorted by
// term." contract: the table's first-column terms must be in case-insensitive
// ascending order. Promoted from prose memory after the order broke twice in
// three days (restored by 2af0ca4, broken again the session after) — the
// retrospective's rung-2 gate-test promotion. The part is scanned rather than
// the rendered doc so the failure names the file to edit.
func TestGlossaryTermsSorted(t *testing.T) {
	src, err := os.ReadFile("../../.awf/docs/parts/glossary/terms.md")
	if err != nil {
		t.Fatalf("read glossary part: %v", err)
	}
	var prev string
	for _, line := range strings.Split(string(src), "\n") {
		if !strings.HasPrefix(line, "| ") || strings.HasPrefix(line, "| Term") || strings.HasPrefix(line, "|--") {
			continue
		}
		cells := strings.SplitN(line, "|", 3)
		if len(cells) < 3 {
			continue
		}
		term := strings.ToLower(strings.TrimSpace(cells[1]))
		if prev != "" && term < prev {
			t.Errorf("glossary term %q sorts before preceding %q — restore the table's sorted-by-term order in .awf/docs/parts/glossary/terms.md", term, prev)
		}
		prev = term
	}
	if prev == "" {
		t.Fatal("no glossary table rows parsed — did the table format change?")
	}
}
